package retriever_test

import (
	"context"
	"testing"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

func TestEnrichTopChunksFetchesFullBody(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{
		"long.md": "# Long Doc\nThis is a reasonably long body that the snippet truncation should surface in full when enriched.",
	})

	// Simulate what Retrieve would produce: one chunk pointing at long.md.
	chunks := []retriever.Chunk{
		{DisplayPath: "notes/long.md", Title: "Long Doc", Score: 0.9,
			Snippet: "tiny snippet", File: "notes/long.md"},
	}
	out := retriever.EnrichTopChunks(context.Background(), eng, chunks, 3)
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Snippet == "tiny snippet" {
		t.Error("EnrichTopChunks should have replaced snippet with full body")
	}
}

func TestEnrichTopChunksSkipsEmptyFile(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{"x.md": "x"})
	chunks := []retriever.Chunk{
		{DisplayPath: "notes/x.md", Score: 0.9, Snippet: "original", File: ""},
	}
	out := retriever.EnrichTopChunks(context.Background(), eng, chunks, 3)
	if out[0].Snippet != "original" {
		t.Error("EnrichTopChunks should keep original snippet when File is empty")
	}
}

func TestEnrichTopChunksDoesNotMutateInput(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{"x.md": "actual body"})
	chunks := []retriever.Chunk{
		{DisplayPath: "notes/x.md", Score: 0.9, Snippet: "original", File: "notes/x.md"},
	}
	_ = retriever.EnrichTopChunks(context.Background(), eng, chunks, 1)
	if chunks[0].Snippet != "original" {
		t.Error("EnrichTopChunks mutated the input slice")
	}
}

func TestEnrichTopChunksZeroN(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{"x.md": "x"})
	chunks := []retriever.Chunk{{DisplayPath: "notes/x.md", Score: 0.9, Snippet: "orig"}}
	out := retriever.EnrichTopChunks(context.Background(), nil, chunks, 0)
	if out[0].Snippet != "orig" {
		t.Error("topN=0 should not enrich anything")
	}
	_ = eng
}

func TestEnrichTopChunksNilEngineIsSafe(t *testing.T) {
	chunks := []retriever.Chunk{{DisplayPath: "notes/x.md", Score: 0.9, Snippet: "orig", File: "notes/x.md"}}
	out := retriever.EnrichTopChunks(context.Background(), nil, chunks, 3)
	if out[0].Snippet != "orig" {
		t.Error("nil engine should no-op, not crash")
	}
}

func TestEnrichTopChunksEmptyInput(t *testing.T) {
	out := retriever.EnrichTopChunks(context.Background(), nil, nil, 3)
	if len(out) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(out))
	}
}
