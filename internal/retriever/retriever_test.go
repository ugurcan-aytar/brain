package retriever_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/retriever"
)

// newRetrieverEngine builds an engine against a temp SQLite, registers a
// small collection of markdown files, indexes + embeds with the mock
// embedder, and returns it ready for SearchHybrid / SearchBM25 calls.
func newRetrieverEngine(t *testing.T, files map[string]string) *engine.Engine {
	t.Helper()

	tmp := t.TempDir()
	collectionDir := filepath.Join(tmp, "notes")
	if err := os.MkdirAll(collectionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		p := filepath.Join(collectionDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	eng, err := engine.Open(recall.WithDBPath(filepath.Join(tmp, "idx.db")))
	if err != nil {
		t.Fatalf("engine.Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	if _, err := eng.Recall().AddCollection("notes", collectionDir, "", ""); err != nil {
		t.Fatalf("AddCollection: %v", err)
	}
	if _, err := eng.Recall().Index(); err != nil {
		t.Fatalf("Index: %v", err)
	}

	mock := recall.NewMockEmbedder(recall.EmbeddingDimensions)
	eng.SetEmbedder(mock)
	if _, err := eng.Recall().Embed(mock, false); err != nil {
		t.Fatalf("Embed: %v", err)
	}

	return eng
}

func TestRetrieveReturnsScoredChunks(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{
		"auth.md":    "# Auth\nThe authentication flow handles JWT tokens.",
		"rate.md":    "# Rate Limiter\nDiscussion of rate limiting algorithms.",
		"weather.md": "# Misc\nUnrelated content about clouds and weather.",
	})

	chunks, err := retriever.Retrieve(context.Background(), eng, "authentication", retriever.Options{TopK: 5})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("Retrieve returned no results")
	}
	// The auth note must surface — BM25 alone is enough for this.
	var found bool
	for _, c := range chunks {
		if c.DisplayPath == "notes/auth.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("auth.md missing from results: %+v", chunks)
	}
}

func TestRetrieveHonoursMinScore(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{
		"a.md": "alpha content",
		"b.md": "beta content",
	})

	high := 10.0 // so high nothing survives
	chunks, err := retriever.Retrieve(context.Background(), eng, "alpha", retriever.Options{
		TopK:     5,
		MinScore: &high,
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("min_score=10 should drop everything; got %+v", chunks)
	}
}

func TestRawSearchReturnsBM25Hits(t *testing.T) {
	eng := newRetrieverEngine(t, map[string]string{
		"zebras.md": "zebras eat grass",
		"lions.md":  "lions hunt at night",
	})

	chunks, err := retriever.RawSearch(context.Background(), eng, "zebras", retriever.Options{})
	if err != nil {
		t.Fatalf("RawSearch: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("RawSearch returned no results")
	}
	if chunks[0].DisplayPath != "notes/zebras.md" {
		t.Errorf("top hit = %s, want notes/zebras.md", chunks[0].DisplayPath)
	}
}

func TestGroundingGateEmpty(t *testing.T) {
	if retriever.GroundingGate(nil) {
		t.Error("GroundingGate(nil) = true, want false")
	}
}

func TestGroundingGateWithChunks(t *testing.T) {
	if !retriever.GroundingGate([]retriever.Chunk{{Score: 0.5}}) {
		t.Error("GroundingGate with 1 chunk should return true (with warning)")
	}
}
