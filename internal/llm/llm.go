// Package llm wraps the two ways brain can talk to Claude:
//
//  1. Directly via the Anthropic SDK if ANTHROPIC_API_KEY is set.
//  2. By piping a prompt through the local `claude` CLI (stream-json mode)
//     when no API key is available. This lets users with a Claude Code
//     subscription run brain without a separate API key.
//
// Both backends stream tokens into a markdown.Renderer so the user sees
// output as it arrives, and both honor ctx cancellation for Ctrl+C.
package llm

import (
	"context"
	"fmt"
	"os"
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

// Stream sends the conversation to Claude and streams the response into a
// markdown.Renderer, returning the full text. If ctx is cancelled the call
// unwinds gracefully and returns whatever was received so far (no error).
func Stream(ctx context.Context, systemPrompt string, messages []Message, opts Options) (string, error) {
	model := ResolveModel(opts.Model)
	renderer := markdown.New()
	fmt.Println()

	var (
		full string
		err  error
	)

	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		full, err = streamViaSDK(ctx, systemPrompt, messages, model, renderer)
	} else {
		full, err = streamViaCLI(ctx, systemPrompt, messages, model, renderer)
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
