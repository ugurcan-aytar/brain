package retriever

import (
	"context"
	"os/exec"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
)

// EnrichTopChunks replaces the snippet of the top N results with the full
// document body via `qmd get`. This works around qmd's per-document dedup:
// when a long transcript has multiple relevant sections, the snippet only
// carries the highest-scoring chunk (often the intro), but the full body
// includes everything — so the LLM sees the complete source, not just the
// winning chunk.
//
// Only enriches chunks whose file path is known (non-empty). Falls back to
// the original snippet on any error.
func EnrichTopChunks(ctx context.Context, chunks []Chunk, topN int) []Chunk {
	if topN <= 0 || len(chunks) == 0 {
		return chunks
	}
	if topN > len(chunks) {
		topN = len(chunks)
	}
	// Cap to avoid fetching too many full documents — each is a subprocess.
	if topN > 5 {
		topN = 5
	}

	out := make([]Chunk, len(chunks))
	copy(out, chunks)

	for i := 0; i < topN; i++ {
		if out[i].File == "" {
			continue
		}
		body := fetchFullBody(ctx, out[i].File)
		if body != "" {
			out[i].Snippet = body
		}
	}
	return out
}

// fetchFullBody calls `qmd get <path>` and returns the full document text.
// Returns empty string on any error so callers keep the original snippet.
func fetchFullBody(ctx context.Context, filePath string) string {
	if !strings.HasPrefix(filePath, "qmd://") {
		return ""
	}
	path := filePath

	cmd := exec.CommandContext(ctx, config.Default.QmdBinary, "get", path)
	cmd.Env = config.QmdEnv()

	stdout, err := cmd.Output()
	if err != nil || len(stdout) == 0 {
		return ""
	}

	body := string(stdout)
	// Truncate very long documents to avoid blowing up the context window.
	// 30K chars ≈ ~8K tokens — enough to capture detail from long transcripts.
	const maxLen = 30000
	if len(body) > maxLen {
		body = body[:maxLen] + "\n[… truncated — full document is longer]"
	}
	return body
}
