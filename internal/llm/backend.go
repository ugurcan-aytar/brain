package llm

// Backend selection. brain can talk to Claude three ways plus any
// OpenAI-compatible endpoint (OpenAI proper, Ollama, OpenRouter, LM Studio,
// LiteLLM, Groq, Together, Fireworks, …). We pick one at call time based on
// which credentials / binaries are actually available, in this order:
//
//	1. Anthropic API      -- ANTHROPIC_API_KEY is set (native, cheapest path)
//	2. OpenAI-compatible  -- OPENAI_API_KEY is set (covers Ollama/OpenRouter/…)
//	3. Claude CLI         -- the `claude` binary (or $BRAIN_CLAUDE_BIN) is in PATH
//
// Explicit env vars beat implicit binary presence so users can steer brain
// without uninstalling anything.

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Backend is the active LLM transport for a single Stream call.
type Backend int

const (
	BackendNone Backend = iota
	BackendAnthropicAPI
	BackendOpenAI
	BackendClaudeCLI
)

// ErrNoBackend is returned from Stream when no LLM backend is configured.
// Command-layer code intercepts this and prints install guidance instead of
// letting the raw error bubble up to the user.
var ErrNoBackend = errors.New("no LLM backend configured")

// lookPath is a seam so tests can stub out PATH resolution without shelling
// out. Production code should never reassign this.
var lookPath = exec.LookPath

// Select returns the backend brain will use right now. It's cheap enough to
// call on every request — the only I/O is a PATH lookup for the Claude CLI
// fallback, and that only runs when neither API key is set.
func Select() Backend {
	if strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != "" {
		return BackendAnthropicAPI
	}
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		return BackendOpenAI
	}
	if _, err := lookPath(claudeBinary()); err == nil {
		return BackendClaudeCLI
	}
	return BackendNone
}

// claudeBinary returns the binary name brain will use for the CLI fallback.
// Defaults to `claude`; override via BRAIN_CLAUDE_BIN for forks like opencode
// or custom wrappers that speak the same stream-json protocol.
func claudeBinary() string {
	if name := strings.TrimSpace(os.Getenv("BRAIN_CLAUDE_BIN")); name != "" {
		return name
	}
	return "claude"
}

// Describe returns a one-line human-readable summary of a backend for
// `brain doctor` and similar diagnostics.
func (b Backend) Describe() string {
	switch b {
	case BackendAnthropicAPI:
		return "Anthropic API (ANTHROPIC_API_KEY)"
	case BackendOpenAI:
		base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
		if base == "" {
			base = defaultOpenAIBaseURL
		}
		return "OpenAI-compatible (" + base + ")"
	case BackendClaudeCLI:
		return "Claude CLI (" + claudeBinary() + ")"
	default:
		return "none"
	}
}
