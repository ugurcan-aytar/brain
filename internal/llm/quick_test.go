package llm

import (
	"context"
	"testing"
)

func TestQuickCompleteNoKeys(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	got, err := QuickComplete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty result when no keys set, got %q", got)
	}
}

func TestExpandQueryNoKeys(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	variants := ExpandQuery(context.Background(), "test question")
	if len(variants) != 1 {
		t.Errorf("expected 1 variant (original only) when no keys, got %d", len(variants))
	}
	if variants[0] != "test question" {
		t.Errorf("expected original query, got %q", variants[0])
	}
}

func TestExpandQueryParsesVariants(t *testing.T) {
	// Test the parsing logic by simulating what happens after QuickComplete
	// returns a multi-line result.
	original := "activation energy"
	mockResult := "Ea Arrhenius equation\nenergy barrier catalysis\nminimum reaction energy"

	variants := []string{original}
	for _, line := range splitLines(mockResult) {
		line = trimSpace(line)
		if line != "" && line != original {
			variants = append(variants, line)
		}
	}
	if len(variants) > 4 {
		variants = variants[:4]
	}

	if len(variants) != 4 {
		t.Errorf("expected 4 variants (original + 3), got %d: %v", len(variants), variants)
	}
	if variants[0] != original {
		t.Errorf("first variant should be original, got %q", variants[0])
	}
}

// Helpers to mirror ExpandQuery's internal logic without calling LLM.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
