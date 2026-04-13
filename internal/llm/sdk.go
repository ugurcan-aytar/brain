package llm

// SDK backend: talks directly to https://api.anthropic.com/v1/messages over
// HTTP + Server-Sent Events. Uses content-block arrays for system and
// messages so the Anthropic prompt caching layer can cache the stable
// system directives and the growing conversation prefix between turns.

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

type cacheControl struct {
	Type string `json:"type"`
}

type contentBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type anthropicMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    []contentBlock     `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

// sseEvent is the subset of each Server-Sent Event we care about.
type sseEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	} `json:"delta"`
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
	systemBlocks := buildSystemBlocks(systemPrompt)
	apiMessages := buildAPIMessages(messages)

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens(),
		System:    systemBlocks,
		Stream:    true,
		Messages:  apiMessages,
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

// buildSystemBlocks converts the system prompt into content blocks.
// The full prompt is placed in a single block with cache_control so
// Anthropic caches the entire system prefix. When the same system
// prompt is sent again within the TTL window (e.g. ask retries,
// chat turns with identical chunks), the cached version is reused
// at 10% of the input token cost.
func buildSystemBlocks(systemPrompt string) []contentBlock {
	return []contentBlock{
		{
			Type:         "text",
			Text:         systemPrompt,
			CacheControl: &cacheControl{Type: "ephemeral"},
		},
	}
}

// buildAPIMessages converts internal messages to the Anthropic content-block
// format and places a cache_control breakpoint on the second-to-last message
// so the conversation prefix is cached between turns.
func buildAPIMessages(messages []Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(messages))
	for i, m := range messages {
		block := contentBlock{Type: "text", Text: m.Content}
		// Mark the second-to-last message so everything up to (and
		// including) it is cached for the next turn.
		if len(messages) > 2 && i == len(messages)-2 {
			block.CacheControl = &cacheControl{Type: "ephemeral"}
		}
		out = append(out, anthropicMessage{
			Role:    string(m.Role),
			Content: []contentBlock{block},
		})
	}
	return out
}
