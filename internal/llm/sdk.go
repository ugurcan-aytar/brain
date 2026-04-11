package llm

// SDK backend: talks directly to https://api.anthropic.com/v1/messages over
// HTTP + Server-Sent Events. No third-party dependencies — the Anthropic Go
// SDK is still in beta and its API shape shifts between releases, so we stay
// on the stable REST surface instead.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/markdown"
)

const (
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
)

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

// sseEvent is the subset of each Server-Sent Event we care about — the
// Anthropic stream reuses HTTP SSE format with `event:` and `data:` lines.
// Two delta types matter: text_delta (the visible response) and
// thinking_delta (the extended-thinking phase opus + sonnet-thinking
// fire before emitting text). We only use thinking_delta to drive a
// dim progress indicator — the reasoning text itself is never printed.
type sseEvent struct {
	Type string `json:"type"`
	// For content_block_delta events, the delta we care about lives here.
	Delta struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	} `json:"delta"`
	// message_start carries usage info etc. — not parsed.
	Index int `json:"index,omitempty"`
}

type anthropicError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func streamViaSDK(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	model string,
	renderer *markdown.Renderer,
) (string, error) {
	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens(),
		System:    systemPrompt,
		Stream:    true,
		Messages:  make([]anthropicMessage, 0, len(messages)),
	}
	for _, m := range messages {
		reqBody.Messages = append(reqBody.Messages, anthropicMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", os.Getenv("ANTHROPIC_API_KEY"))
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", nil
		}
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		var apiErr anthropicError
		if err := json.Unmarshal(errBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("anthropic API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var full strings.Builder
	var thinking thinkingIndicator

	for scanner.Scan() {
		if ctx.Err() != nil {
			return full.String(), nil
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var ev sseEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if ev.Type != "content_block_delta" {
			continue
		}
		switch ev.Delta.Type {
		case "thinking_delta":
			thinking.Note()
		case "text_delta":
			thinking.Finalize()
			if ev.Delta.Text != "" {
				full.WriteString(ev.Delta.Text)
				renderer.Write(ev.Delta.Text)
			}
		}
	}
	thinking.Finalize()

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return full.String(), nil
		}
		return full.String(), fmt.Errorf("stream read failed: %w", err)
	}

	return full.String(), nil
}
