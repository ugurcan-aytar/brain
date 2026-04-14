package engine_test

import (
	"path/filepath"
	"testing"

	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/engine"
)

func newTestEngine(t *testing.T) *engine.Engine {
	t.Helper()
	dir := t.TempDir()
	eng, err := engine.Open(recall.WithDBPath(filepath.Join(dir, "idx.db")))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func TestOpenAndClose(t *testing.T) {
	eng := newTestEngine(t)
	if eng.Recall() == nil {
		t.Fatal("Recall() returned nil")
	}
	cols, err := eng.Recall().ListCollections()
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("fresh db has %d collections, want 0", len(cols))
	}
}

func TestEmbedderSetOverrideWins(t *testing.T) {
	eng := newTestEngine(t)
	mock := recall.NewMockEmbedder(recall.EmbeddingDimensions)
	eng.SetEmbedder(mock)

	got, err := eng.Embedder()
	if err != nil {
		t.Fatalf("Embedder: %v", err)
	}
	if got == nil {
		t.Fatal("expected injected mock, got nil")
	}
	if got.Dimensions() != recall.EmbeddingDimensions {
		t.Errorf("injected embedder dims = %d, want %d", got.Dimensions(), recall.EmbeddingDimensions)
	}
}

func TestEmbedderGracefulOnStubBuild(t *testing.T) {
	if recall.LocalEmbedderAvailable() {
		t.Skip("built with embed_llama — graceful-fallback path not reachable")
	}
	t.Setenv("RECALL_EMBED_PROVIDER", "")

	eng := newTestEngine(t)
	emb, err := eng.Embedder()
	if err != nil {
		t.Fatalf("Embedder should fall back silently on stub build, got err: %v", err)
	}
	if emb != nil {
		t.Errorf("expected nil embedder on stub build, got %T", emb)
	}
}

func TestEmbedderSurfacesConfigError(t *testing.T) {
	t.Setenv("RECALL_EMBED_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "") // force the "key missing" error path

	eng := newTestEngine(t)
	_, err := eng.Embedder()
	if err == nil {
		t.Fatal("expected config error when openai provider lacks API key")
	}
}
