package llm

// OpenAI-compatible backend: talks to any server that speaks the
// /v1/chat/completions SSE protocol. That covers OpenAI proper plus Ollama
// (`http://localhost:11434/v1`), OpenRouter (`https://openrouter.ai/api/v1`),
// LM Studio, LiteLLM proxies, Groq, Together, Fireworks, and friends.
//
// We intentionally don't pull in the official openai-go SDK — it's a big
// dependency for what is ultimately a single streaming POST, and the tiny
// payload shape we use here is stable across all the providers above.

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
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "gpt-4o"
)

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

// openAIStreamChunk matches the subset of `chat.completion.chunk` events we
// need to assemble the streaming response. Providers vary in which fields
// they populate, but `choices[0].delta.content` is the universal contract.
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ResolveOpenAIModel picks the model name to send to the OpenAI-compatible
// endpoint. Priority:
//
//  1. A non-Claude `-m` flag value (passes through as-is, so things like
//     `-m meta-llama/llama-3.1-70b-instruct` work on OpenRouter)
//  2. $OPENAI_MODEL
//  3. Package default (gpt-4o)
//
// Claude aliases like `opus`/`sonnet`/`haiku` are ignored here — they're
// meaningful only to the Anthropic backends, and silently reinterpreting
// them as GPT models would be more confusing than picking the default.
func ResolveOpenAIModel(flag string) string {
	trimmed := strings.TrimSpace(flag)
	if trimmed != "" && !isClaudeAlias(trimmed) && !strings.HasPrefix(trimmed, "claude-") {
		return trimmed
	}
	if env := strings.TrimSpace(os.Getenv("OPENAI_MODEL")); env != "" {
		return env
	}
	return defaultOpenAIModel
}

func isClaudeAlias(name string) bool {
	switch name {
	case "opus", "sonnet", "haiku":
		return true
	}
	return false
}

func openAIBaseURL() string {
	if base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); base != "" {
		return strings.TrimRight(base, "/")
	}
	return defaultOpenAIBaseURL
}

func streamViaOpenAI(
	ctx context.Context,
	systemPrompt string,
	messages []Message,
	model string,
	renderer *markdown.Renderer,
) (string, error) {
	// OpenAI-style chat completions take `system` as just another message
	// rather than a dedicated field like Anthropic. Prepend it.
	payload := openAIRequest{
		Model:     model,
		Stream:    true,
		MaxTokens: maxTokens(),
		Messages:  make([]openAIMessage, 0, len(messages)+1),
	}
	if strings.TrimSpace(systemPrompt) != "" {
		payload.Messages = append(payload.Messages, openAIMessage{Role: "system", Content: systemPrompt})
	}
	for _, m := range messages {
		payload.Messages = append(payload.Messages, openAIMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	endpoint := openAIBaseURL() + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

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
		var apiErr openAIError
		if err := json.Unmarshal(errBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("openai API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("openai API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var full strings.Builder
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
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content == "" {
				continue
			}
			full.WriteString(c.Delta.Content)
			renderer.Write(c.Delta.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return full.String(), nil
		}
		return full.String(), fmt.Errorf("stream read failed: %w", err)
	}
	return full.String(), nil
}
