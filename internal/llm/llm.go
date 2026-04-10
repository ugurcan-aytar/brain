// Package llm wraps the three ways brain can reach a model:
//
//  1. Anthropic REST API          -- when ANTHROPIC_API_KEY is set
//  2. OpenAI-compatible REST API  -- when OPENAI_API_KEY is set
//     (covers OpenAI, Ollama, OpenRouter, LM Studio, LiteLLM, Groq, etc.)
//  3. Claude CLI fallback         -- pipes through `claude` (or $BRAIN_CLAUDE_BIN)
//     for users with a Claude Code subscription but no API key
//
// All three stream tokens into a markdown.Renderer so the user sees output
// as it arrives, and all three honor ctx cancellation for Ctrl+C.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/markdown"
)

// Role is the sender of a conversation turn.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn in a chat history.
type Message struct {
	Role    Role
	Content string
}

// Options controls a streaming LLM call.
type Options struct {
	Model string // alias or full model ID; empty defaults to opus
}

// Stream sends the conversation to the active backend and streams the
// response into a markdown.Renderer, returning the full text. If ctx is
// cancelled the call unwinds gracefully and returns whatever was received
// so far (no error). If no backend is configured at all, ErrNoBackend is
// returned immediately so callers can render install guidance.
func Stream(ctx context.Context, systemPrompt string, messages []Message, opts Options) (string, error) {
	renderer := markdown.New()

	var (
		full string
		err  error
	)

	switch Select() {
	case BackendAnthropicAPI:
		fmt.Println()
		full, err = streamViaSDK(ctx, systemPrompt, messages, ResolveModel(opts.Model), renderer)
	case BackendOpenAI:
		fmt.Println()
		full, err = streamViaOpenAI(ctx, systemPrompt, messages, ResolveOpenAIModel(opts.Model), renderer)
	case BackendClaudeCLI:
		fmt.Println()
		full, err = streamViaCLI(ctx, systemPrompt, messages, ResolveModel(opts.Model), renderer)
	default:
		return "", ErrNoBackend
	}

	if err != nil {
		if ctx.Err() != nil {
			renderer.Flush()
			return full, nil
		}
		return full, err
	}

	renderer.Flush()
	return full, nil
}

// buildConversationPrompt flattens a multi-turn history into a single user
// prompt — used only by the CLI backend, which doesn't accept structured
// messages the way the SDK does.
func buildConversationPrompt(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	if len(messages) == 1 {
		return messages[0].Content
	}
	var parts []string
	for _, m := range messages[:len(messages)-1] {
		label := "User"
		if m.Role == RoleAssistant {
			label = "Assistant"
		}
		parts = append(parts, fmt.Sprintf("%s: %s", label, m.Content))
	}
	last := messages[len(messages)-1]
	parts = append(parts, fmt.Sprintf("User: %s", last.Content))
	parts = append(parts, "\nContinue the conversation. Respond to the latest User message only.")
	return strings.Join(parts, "\n\n")
}

// stripAnsiRemnants scrubs broken escape sequences that can leak through
// when the upstream claude CLI flushes a partial ANSI code. Harmless to
// SDK output but cheap to run.
func stripAnsiRemnants(text string) string {
	text = strings.ReplaceAll(text, "\x1b[0m", "")
	// Also handle the bare `[Nm` remnants that sometimes slip through.
	var b strings.Builder
	i := 0
	for i < len(text) {
		if text[i] == '[' {
			end := i + 1
			for end < len(text) && (text[end] == ';' || (text[end] >= '0' && text[end] <= '9')) {
				end++
			}
			if end < len(text) && text[end] == 'm' && end > i+1 {
				i = end + 1
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// maxTokens exposes the configured max_tokens to the SDK and CLI backends.
func maxTokens() int {
	return config.Default.MaxTokens
}
