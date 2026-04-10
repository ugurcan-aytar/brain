package llm

// Fallback backend: pipes a prompt through the local `claude` CLI when
// no ANTHROPIC_API_KEY is set. The CLI streams back newline-delimited
// JSON events in the same shape as the API SSE stream, so parsing is
// nearly identical to sdk.go.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/markdown"
)

// claudeCLIEvent matches the shape emitted by `claude --output-format stream-json --verbose`.
type claudeCLIEvent struct {
	Type  string `json:"type"`
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
}

// ErrClaudeCLIMissing is returned when the CLI fallback was selected but the
// binary isn't on PATH at exec time. In practice Select() guards against this
// so the error only surfaces in race conditions (PATH changed mid-run) or
// when tests force the CLI path directly.
var ErrClaudeCLIMissing = errors.New("claude CLI is not installed or not found in PATH")

func streamViaCLI(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	model string,
	renderer *markdown.Renderer,
) (string, error) {
	userPrompt := buildConversationPrompt(messages)

	args := []string{
		"-p",
		"--system-prompt", systemPrompt,
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--no-session-persistence",
		"--model", model,
		userPrompt,
	}

	cmd := exec.CommandContext(ctx, claudeBinary(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = nil // discard claude's stderr chatter

	if err := cmd.Start(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return "", ErrClaudeCLIMissing
		}
		return "", fmt.Errorf("start claude: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var full strings.Builder
	for scanner.Scan() {
		if ctx.Err() != nil {
			_ = cmd.Process.Kill()
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev claudeCLIEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Type == "stream_event" && ev.Event.Type == "content_block_delta" && ev.Event.Delta.Type == "text_delta" {
			text := stripAnsiRemnants(ev.Event.Delta.Text)
			if text != "" {
				full.WriteString(text)
				renderer.Write(text)
			}
		}
	}

	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return full.String(), nil
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			// Non-zero exit from claude → bubble up only if we got nothing.
			if full.Len() == 0 {
				return "", fmt.Errorf("claude CLI exited with code %d", exitErr.ExitCode())
			}
		}
	}
	return full.String(), nil
}
