package config

import "testing"

func TestDefaultSettingsInvariants(t *testing.T) {
	if Default.TopK <= 0 {
		t.Error("TopK must be positive")
	}
	if Default.MaxTokens <= 0 {
		t.Error("MaxTokens must be positive")
	}
	if Default.MinScore < 0 || Default.MinScore > 1 {
		t.Errorf("MinScore should be in [0,1], got %f", Default.MinScore)
	}
	if Default.Model == "" {
		t.Error("Model must not be empty")
	}
	if Default.DefaultMask == "" {
		t.Error("DefaultMask must not be empty")
	}
	if Default.MinChunksToCallLLM < 0 {
		t.Error("MinChunksToCallLLM must not be negative")
	}
	if Default.MaxConversationTurns <= 0 {
		t.Error("MaxConversationTurns must be positive")
	}
}
