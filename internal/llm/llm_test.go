package llm

import (
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
