package llm

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestResolveModel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"sonnet", "claude-sonnet-4-6"},
		{"opus", "claude-opus-4-6"},
		{"haiku", "claude-haiku-4-5"},
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20241022"}, // passes through
		{"", "claude-opus-4-6"}, // empty defaults to opus
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := ResolveModel(tc.in); got != tc.want {
				t.Errorf("ResolveModel(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsValidModel(t *testing.T) {
	valid := []string{
		"sonnet", "opus", "haiku",
		"claude-opus-4-6",
		"claude-anything-even-weird",
	}
	for _, v := range valid {
		if !IsValidModel(v) {
			t.Errorf("IsValidModel(%q) = false, want true", v)
		}
	}

	invalid := []string{
		"", "gpt-4", "random-string", "SONNET", "clauude-opus",
	}
	for _, v := range invalid {
		if IsValidModel(v) {
			t.Errorf("IsValidModel(%q) = true, want false", v)
		}
	}
}

func TestDisplay(t *testing.T) {
	if got := Display("sonnet"); got != "sonnet (claude-sonnet-4-6)" {
		t.Errorf("Display(\"sonnet\") = %q", got)
	}
	if got := Display("claude-opus-4-6"); got != "claude-opus-4-6" {
		t.Errorf("Display with full id should pass through, got %q", got)
	}
	if got := Display(""); got != "" {
		t.Errorf("Display(\"\") = %q", got)
	}
}

func TestBuildConversationPrompt(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := buildConversationPrompt(nil); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("single user message", func(t *testing.T) {
		msgs := []Message{{Role: RoleUser, Content: "hello"}}
		if got := buildConversationPrompt(msgs); got != "hello" {
			t.Errorf("single-message path should return raw content, got %q", got)
		}
	})

	t.Run("multi-turn includes labels and continuation instruction", func(t *testing.T) {
		msgs := []Message{
			{Role: RoleUser, Content: "first"},
			{Role: RoleAssistant, Content: "reply"},
			{Role: RoleUser, Content: "followup"},
		}
		got := buildConversationPrompt(msgs)
		for _, want := range []string{
			"User: first",
			"Assistant: reply",
			"User: followup",
			"Continue the conversation",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("multi-turn prompt missing %q", want)
			}
		}
	})
}

func TestStripAnsiRemnants(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain text", "plain text"},
		{"\x1b[0mtext", "text"},
		{"[31mred[0m", "red"},
		{"[1;32mbold green", "bold green"},
		{"no remnants [like] this", "no remnants [like] this"},
		{"[abcm not an escape", "[abcm not an escape"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := stripAnsiRemnants(tc.in); got != tc.want {
				t.Errorf("stripAnsiRemnants(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSelectPriority(t *testing.T) {
	// Stub lookPath so tests don't depend on whether `claude` is actually
	// installed on the box running them.
	origLookPath := lookPath
	t.Cleanup(func() { lookPath = origLookPath })

	claudeAvailable := func(name string) (string, error) { return "/fake/bin/" + name, nil }
	claudeMissing := func(name string) (string, error) { return "", &exec.Error{Name: name, Err: errors.New("not found")} }

	cases := []struct {
		name      string
		anthropic string
		openai    string
		claudeBin string
		lookPath  func(string) (string, error)
		want      Backend
	}{
		{"nothing set", "", "", "", claudeMissing, BackendNone},
		{"only claude CLI", "", "", "", claudeAvailable, BackendClaudeCLI},
		{"openai beats claude CLI", "", "sk-openai", "", claudeAvailable, BackendOpenAI},
		{"anthropic beats everything", "sk-ant", "sk-openai", "", claudeAvailable, BackendAnthropicAPI},
		{"anthropic alone", "sk-ant", "", "", claudeMissing, BackendAnthropicAPI},
		{"openai alone", "", "sk-openai", "", claudeMissing, BackendOpenAI},
		{"BRAIN_CLAUDE_BIN custom name", "", "", "opencode", claudeAvailable, BackendClaudeCLI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ANTHROPIC_API_KEY", tc.anthropic)
			t.Setenv("OPENAI_API_KEY", tc.openai)
			t.Setenv("BRAIN_CLAUDE_BIN", tc.claudeBin)
			lookPath = tc.lookPath

			if got := Select(); got != tc.want {
				t.Errorf("Select() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClaudeBinary(t *testing.T) {
	t.Setenv("BRAIN_CLAUDE_BIN", "")
	if got := claudeBinary(); got != "claude" {
		t.Errorf("default claudeBinary = %q, want %q", got, "claude")
	}
	t.Setenv("BRAIN_CLAUDE_BIN", "opencode")
	if got := claudeBinary(); got != "opencode" {
		t.Errorf("override claudeBinary = %q, want %q", got, "opencode")
	}
	// Whitespace-only overrides should be ignored — otherwise a user with
	// `export BRAIN_CLAUDE_BIN= ` in their rc would break the fallback.
	t.Setenv("BRAIN_CLAUDE_BIN", "   ")
	if got := claudeBinary(); got != "claude" {
		t.Errorf("whitespace override should fall back, got %q", got)
	}
}

func TestResolveOpenAIModel(t *testing.T) {
	cases := []struct {
		name  string
		flag  string
		env   string
		want  string
	}{
		{"default when nothing set", "", "", "gpt-4o"},
		{"env var fills default", "", "gpt-5-mini", "gpt-5-mini"},
		{"claude alias ignored, env wins", "opus", "gpt-5", "gpt-5"},
		{"claude alias ignored, default wins", "sonnet", "", "gpt-4o"},
		{"claude full id ignored", "claude-opus-4-6", "gpt-5", "gpt-5"},
		{"raw openai id passes through", "gpt-4o-mini", "gpt-5", "gpt-4o-mini"},
		{"openrouter slash id passes through", "meta-llama/llama-3.1-70b-instruct", "", "meta-llama/llama-3.1-70b-instruct"},
		{"ollama model passes through", "llama3.1", "", "llama3.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OPENAI_MODEL", tc.env)
			if got := ResolveOpenAIModel(tc.flag); got != tc.want {
				t.Errorf("ResolveOpenAIModel(%q) with OPENAI_MODEL=%q = %q, want %q",
					tc.flag, tc.env, got, tc.want)
			}
		})
	}
}

func TestOpenAIBaseURL(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "")
	if got := openAIBaseURL(); got != "https://api.openai.com/v1" {
		t.Errorf("default openAIBaseURL = %q", got)
	}
	t.Setenv("OPENAI_BASE_URL", "http://localhost:11434/v1/")
	if got := openAIBaseURL(); got != "http://localhost:11434/v1" {
		t.Errorf("trailing slash should be trimmed, got %q", got)
	}
}

func TestModelChoicesCoverAllAliases(t *testing.T) {
	// Every alias in Models should appear in ModelChoices and vice versa —
	// otherwise the /model picker drifts out of sync with what ResolveModel
	// accepts.
	inChoices := map[string]bool{}
	for _, c := range ModelChoices {
		inChoices[c.Alias] = true
	}
	for alias := range Models {
		if !inChoices[alias] {
			t.Errorf("alias %q in Models but missing from ModelChoices", alias)
		}
	}
	for _, c := range ModelChoices {
		if _, ok := Models[c.Alias]; !ok {
			t.Errorf("ModelChoice %q not registered in Models", c.Alias)
		}
		if c.ResolvedID == "" {
			t.Errorf("ModelChoice %q has empty ResolvedID", c.Alias)
		}
	}
}
