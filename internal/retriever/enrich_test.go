package retriever

import (
	"context"
	"testing"
)

func TestEnrichTopChunksSkipsEmptyFile(t *testing.T) {
	chunks := []Chunk{
		{DisplayPath: "a.md", Title: "A", Score: 0.9, Snippet: "original", File: ""},
	}
	result := EnrichTopChunks(context.Background(), chunks, 3)
	if result[0].Snippet != "original" {
		t.Error("should keep original snippet when File is empty")
	}
}

func TestEnrichTopChunksSkipsNonQmdPath(t *testing.T) {
	chunks := []Chunk{
		{DisplayPath: "a.md", Title: "A", Score: 0.9, Snippet: "original", File: "not-a-qmd-path"},
	}
	result := EnrichTopChunks(context.Background(), chunks, 3)
	if result[0].Snippet != "original" {
		t.Error("should keep original snippet when File doesn't start with qmd://")
	}
}

func TestEnrichTopChunksDoesNotMutateOriginal(t *testing.T) {
	chunks := []Chunk{
		{DisplayPath: "a.md", Score: 0.9, Snippet: "original", File: "qmd://test/a.txt"},
		{DisplayPath: "b.md", Score: 0.5, Snippet: "second", File: ""},
	}
	_ = EnrichTopChunks(context.Background(), chunks, 1)
	if chunks[0].Snippet != "original" {
		t.Error("EnrichTopChunks should not mutate the input slice")
	}
}

func TestEnrichTopChunksRespectsCap(t *testing.T) {
	chunks := make([]Chunk, 10)
	for i := range chunks {
		chunks[i] = Chunk{
			DisplayPath: "x.md",
			Score:       float64(10-i) / 10,
			Snippet:     "snippet",
			File:        "qmd://test/x.txt",
		}
	}
	// topN=20 should be capped to 5 internally
	result := EnrichTopChunks(context.Background(), chunks, 20)
	if len(result) != 10 {
		t.Errorf("expected all 10 chunks returned, got %d", len(result))
	}
}

func TestEnrichTopChunksEmptyInput(t *testing.T) {
	result := EnrichTopChunks(context.Background(), nil, 3)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestEnrichTopChunksZeroN(t *testing.T) {
	chunks := []Chunk{{DisplayPath: "a.md", Score: 0.9, Snippet: "orig"}}
	result := EnrichTopChunks(context.Background(), chunks, 0)
	if result[0].Snippet != "orig" {
		t.Error("topN=0 should not enrich anything")
	}
}
