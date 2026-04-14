package retriever

import (
	"context"

	"github.com/ugurcan-aytar/brain/internal/engine"
)

// EnrichTopChunks replaces the Snippet of the top N results with the
// full document body via recall.Engine.Get. qmd's old per-document dedup
// meant the snippet only carried the winning chunk; recall doesn't have
// that problem, but enriching still helps for long transcripts where
// relevant detail lives outside the top chunk.
//
// Only enriches chunks whose File path is populated. Falls back to the
// original snippet on any error. Caps at 5 regardless of topN so we
// don't balloon the token budget.
func EnrichTopChunks(ctx context.Context, eng *engine.Engine, chunks []Chunk, topN int) []Chunk {
	_ = ctx // reserved — recall.Engine.Get is not ctx-aware yet.

	if topN <= 0 || len(chunks) == 0 || eng == nil {
		return chunks
	}
	if topN > len(chunks) {
		topN = len(chunks)
	}
	if topN > 5 {
		topN = 5
	}

	out := make([]Chunk, len(chunks))
	copy(out, chunks)

	for i := 0; i < topN; i++ {
		if out[i].File == "" {
			continue
		}
		doc, err := eng.Recall().Get(out[i].File)
		if err != nil || doc == nil {
			continue
		}
		body := doc.Content
		// Truncate very long documents to cap the context budget. 30K
		// chars ≈ ~8K tokens — enough to capture detail from long
		// transcripts without blowing up the prompt.
		const maxLen = 30000
		if len(body) > maxLen {
			body = body[:maxLen] + "\n[… truncated — full document is longer]"
		}
		if body != "" {
			out[i].Snippet = body
		}
	}
	return out
}
