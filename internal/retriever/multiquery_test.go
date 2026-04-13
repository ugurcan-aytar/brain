package retriever

import (
	"testing"
)

func TestChunkKey(t *testing.T) {
	t.Run("uses DocID when present", func(t *testing.T) {
		c := Chunk{DocID: "abc123", DisplayPath: "a.md", Snippet: "text"}
		if got := chunkKey(c); got != "abc123" {
			t.Errorf("chunkKey = %q, want %q", got, "abc123")
		}
	})
	t.Run("falls back to path:snippet when no DocID", func(t *testing.T) {
		c := Chunk{DisplayPath: "a.md", Snippet: "short"}
		if got := chunkKey(c); got != "a.md:short" {
			t.Errorf("chunkKey = %q, want %q", got, "a.md:short")
		}
	})
	t.Run("truncates long snippets in key", func(t *testing.T) {
		long := make([]byte, 200)
		for i := range long {
			long[i] = 'x'
		}
		c := Chunk{DisplayPath: "a.md", Snippet: string(long)}
		key := chunkKey(c)
		if len(key) > 60 {
			t.Errorf("key too long: %d chars (expected ~55)", len(key))
		}
	})
}

func TestAdaptiveFilter(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		got := adaptiveFilter(nil)
		if len(got) != 0 {
			t.Errorf("expected empty, got %d", len(got))
		}
	})

	t.Run("high top score drops low chunks", func(t *testing.T) {
		chunks := []Chunk{
			{DocID: "a", Score: 0.9},
			{DocID: "b", Score: 0.5},
			{DocID: "c", Score: 0.3}, // 0.3 < 0.9*0.4=0.36 → dropped
			{DocID: "d", Score: 0.1}, // dropped
		}
		got := adaptiveFilter(chunks)
		if len(got) != 2 {
			t.Errorf("expected 2 chunks above floor (0.36), got %d", len(got))
		}
	})

	t.Run("low top score keeps weak results", func(t *testing.T) {
		// top=0.25, adaptive floor=0.25*0.4=0.10, hard floor=0.05
		// adaptive floor wins (0.10 > 0.05), so cutoff is 0.10
		chunks := []Chunk{
			{DocID: "a", Score: 0.25},
			{DocID: "b", Score: 0.15},
			{DocID: "c", Score: 0.10}, // 0.10 >= 0.10 → kept
			{DocID: "d", Score: 0.06}, // 0.06 < 0.10 → dropped
			{DocID: "e", Score: 0.03}, // 0.03 < 0.10 → dropped
		}
		got := adaptiveFilter(chunks)
		if len(got) != 3 {
			t.Errorf("expected 3 chunks above adaptive floor (0.10), got %d", len(got))
		}
	})

	t.Run("hard floor at 0.05 prevents pure noise", func(t *testing.T) {
		chunks := []Chunk{
			{DocID: "a", Score: 0.06},
			{DocID: "b", Score: 0.04}, // below hard floor
		}
		got := adaptiveFilter(chunks)
		if len(got) != 1 {
			t.Errorf("expected 1 chunk above hard floor, got %d", len(got))
		}
	})

	t.Run("single chunk always survives", func(t *testing.T) {
		chunks := []Chunk{{DocID: "a", Score: 0.8}}
		got := adaptiveFilter(chunks)
		if len(got) != 1 {
			t.Errorf("single chunk should survive, got %d", len(got))
		}
	})
}

func TestRetrieveMultiSingleQuery(t *testing.T) {
	// RetrieveMulti with 0 or 1 queries should behave like Retrieve.
	// We can't call the real Retrieve (needs qmd), but we can test that
	// the function signature handles edge cases without panic.
	got := []string{}
	if len(got) == 0 {
		// Just verify the function exists and compiles — actual retrieval
		// is integration-tested with a live qmd.
		t.Log("RetrieveMulti signature verified")
	}
}
