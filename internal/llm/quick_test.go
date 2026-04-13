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
