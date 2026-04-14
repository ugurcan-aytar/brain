package engine_test

// End-to-end tests across the brain→recall boundary. These prove the
// brain wrappers (engine.Engine, retriever.Retrieve, retriever.RawSearch)
// sit on top of recall.Engine correctly. Mock embedder is used
// throughout so CI doesn't need a GGUF model or an API key.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/commands"
	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/retriever"
)

func seedCollection(t *testing.T, eng *engine.Engine, name string, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for fname, content := range files {
		p := filepath.Join(dir, fname)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if _, err := eng.Recall().AddCollection(name, dir, "", ""); err != nil {
		t.Fatalf("AddCollection: %v", err)
	}
	if _, err := eng.Recall().Index(); err != nil {
		t.Fatalf("Index: %v", err)
	}
}

// TestBrainRecallIntegration exercises the full add → index → embed →
// hybrid retrieve pipeline through brain's wrappers.
func TestBrainRecallIntegration(t *testing.T) {
	dir := t.TempDir()
	eng, err := engine.Open(recall.WithDBPath(filepath.Join(dir, "idx.db")))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer eng.Close()

	seedCollection(t, eng, "notes", map[string]string{
		"auth.md":    "# Authentication\nThe auth flow validates JWT tokens.",
		"payments.md": "# Payment Retry\nThe payment retry logic needs a circuit breaker.",
	})

	mock := recall.NewMockEmbedder(recall.EmbeddingDimensions)
	eng.SetEmbedder(mock)
	if _, err := eng.Recall().Embed(mock, false); err != nil {
		t.Fatalf("Embed: %v", err)
	}

	chunks, err := retriever.Retrieve(context.Background(), eng, "authentication JWT",
		retriever.Options{TopK: 5})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("hybrid pipeline returned nothing")
	}
	var sawAuth bool
	for _, c := range chunks {
		if strings.HasSuffix(c.DisplayPath, "auth.md") {
			sawAuth = true
			break
		}
	}
	if !sawAuth {
		t.Errorf("auth.md missing from results: %+v", chunks)
	}

	// retriever.GroundingGate must accept the real-world output.
	if !retriever.GroundingGate(chunks) {
		t.Error("GroundingGate rejected valid chunks")
	}

	// Enrichment should replace snippets with the full body.
	enriched := retriever.EnrichTopChunks(context.Background(), eng, chunks, 3)
	if len(enriched) != len(chunks) {
		t.Errorf("enriched len = %d, want %d", len(enriched), len(chunks))
	}
}

// TestBrainRecallFallbackBM25 ensures brain still returns results when
// no embeddings exist — recall.Engine.SearchHybrid degrades silently
// when the embedder is nil.
func TestBrainRecallFallbackBM25(t *testing.T) {
	dir := t.TempDir()
	eng, err := engine.Open(recall.WithDBPath(filepath.Join(dir, "idx.db")))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer eng.Close()

	seedCollection(t, eng, "notes", map[string]string{
		"rate.md": "# Rate Limiting\nToken bucket vs leaky bucket vs sliding window.",
	})

	// Skip Embed intentionally.
	chunks, err := retriever.Retrieve(context.Background(), eng, "token bucket",
		retriever.Options{TopK: 5})
	if err != nil {
		// Stub-build / no-API case: ResolveEmbedder returns an error for
		// openai/voyage misconfig, and nil,nil for "no local backend".
		// In test, the env var is untouched so we expect nil,nil.
		if strings.Contains(err.Error(), "resolve embedder") {
			t.Fatalf("embedder resolve should soft-fail, got: %v", err)
		}
		t.Fatalf("Retrieve: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("BM25-only fallback returned nothing")
	}
}

// TestBrainDoctorNoQmdMentions runs the Doctor handler and proves its
// output never mentions qmd or npm. Catches accidental reintroduction
// of the old pre-B1 hints.
func TestBrainDoctorNoQmdMentions(t *testing.T) {
	// Route stdout to a pipe so we can capture what Doctor prints. No
	// assertions on exit status — Doctor returns non-zero when no LLM
	// backend is configured, which is the CI default.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	// Isolate to a temp DB so Doctor doesn't prompt about a real one.
	t.Setenv("RECALL_DB_PATH", filepath.Join(t.TempDir(), "idx.db"))
	// Prevent the graceful fallback path from exposing env noise.
	t.Setenv("RECALL_EMBED_PROVIDER", "")

	_ = commands.Doctor(context.Background())

	_ = w.Close()
	os.Stdout = oldStdout
	out := <-done

	banned := []string{"qmd", "npm", "@tobilu", "Node.js"}
	lower := strings.ToLower(out)
	for _, b := range banned {
		if strings.Contains(lower, strings.ToLower(b)) {
			t.Errorf("brain doctor output mentions banned token %q (full output below):\n%s", b, out)
		}
	}
}
