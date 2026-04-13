package llm

import (
	"encoding/json"
	"testing"
)

func TestBuildSystemBlocksHasCacheControl(t *testing.T) {
	blocks := buildSystemBlocks("test system prompt")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(blocks))
	}
	if blocks[0].Text != "test system prompt" {
		t.Errorf("block text = %q", blocks[0].Text)
	}
	if blocks[0].CacheControl == nil || blocks[0].CacheControl.Type != "ephemeral" {
		t.Error("system block missing cache_control ephemeral")
	}
}

func TestBuildAPIMessagesCacheControlPlacement(t *testing.T) {
	t.Run("3+ messages: cache on second-to-last", func(t *testing.T) {
		msgs := []Message{
			{Role: RoleUser, Content: "q1"},
			{Role: RoleAssistant, Content: "a1"},
			{Role: RoleUser, Content: "q2"},
		}
		api := buildAPIMessages(msgs)
		if len(api) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(api))
		}
		// msg[0] (user q1): no cache_control
		if api[0].Content[0].CacheControl != nil {
			t.Error("first message should not have cache_control")
		}
		// msg[1] (assistant a1): cache_control (second-to-last)
		if api[1].Content[0].CacheControl == nil {
			t.Error("second-to-last message should have cache_control")
		}
		// msg[2] (user q2): no cache_control (latest)
		if api[2].Content[0].CacheControl != nil {
			t.Error("last message should not have cache_control")
		}
	})

	t.Run("1 message: no cache_control", func(t *testing.T) {
		msgs := []Message{{Role: RoleUser, Content: "q1"}}
		api := buildAPIMessages(msgs)
		if api[0].Content[0].CacheControl != nil {
			t.Error("single message should not have cache_control")
		}
	})

	t.Run("2 messages: no cache_control", func(t *testing.T) {
		msgs := []Message{
			{Role: RoleUser, Content: "q1"},
			{Role: RoleAssistant, Content: "a1"},
		}
		api := buildAPIMessages(msgs)
		for i, m := range api {
			if m.Content[0].CacheControl != nil {
				t.Errorf("message %d should not have cache_control (need >2 messages)", i)
			}
		}
	})
}

func TestBuildAPIMessagesJSON(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: "hello"},
		{Role: RoleAssistant, Content: "world"},
		{Role: RoleUser, Content: "again"},
	}
	api := buildAPIMessages(msgs)
	b, err := json.Marshal(api)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// Verify it's valid JSON and contains content blocks.
	var parsed []struct {
		Role    string `json:"role"`
		Content []struct {
			Type         string `json:"type"`
			Text         string `json:"text"`
			CacheControl *struct {
				Type string `json:"type"`
			} `json:"cache_control,omitempty"`
		} `json:"content"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("round-trip JSON parse failed: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(parsed))
	}
	if parsed[0].Content[0].Text != "hello" {
		t.Errorf("first message text = %q", parsed[0].Content[0].Text)
	}
	if parsed[1].Content[0].CacheControl == nil || parsed[1].Content[0].CacheControl.Type != "ephemeral" {
		t.Error("second message should have cache_control in JSON")
	}
}

func TestInjectChunkContext(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: "first question"},
		{Role: RoleAssistant, Content: "first answer"},
		{Role: RoleUser, Content: "second question"},
	}

	result := injectChunkContext(msgs, "CHUNK CONTEXT HERE")

	// Original should not be mutated.
	if msgs[2].Content != "second question" {
		t.Error("injectChunkContext mutated the original slice")
	}

	// Last user message should have context prepended.
	if result[2].Content != "CHUNK CONTEXT HERE\n\nsecond question" {
		t.Errorf("last user message = %q", result[2].Content)
	}

	// Earlier messages should be unchanged.
	if result[0].Content != "first question" {
		t.Errorf("first message should be unchanged, got %q", result[0].Content)
	}
	if result[1].Content != "first answer" {
		t.Errorf("assistant message should be unchanged, got %q", result[1].Content)
	}
}

func TestInjectChunkContextSingleMessage(t *testing.T) {
	msgs := []Message{{Role: RoleUser, Content: "only question"}}
	result := injectChunkContext(msgs, "CHUNKS")
	if result[0].Content != "CHUNKS\n\nonly question" {
		t.Errorf("got %q", result[0].Content)
	}
}

func TestInjectChunkContextEmptyContext(t *testing.T) {
	msgs := []Message{{Role: RoleUser, Content: "q"}}
	result := injectChunkContext(msgs, "")
	if result[0].Content != "\n\nq" {
		t.Errorf("empty context should still prepend separator, got %q", result[0].Content)
	}
}
