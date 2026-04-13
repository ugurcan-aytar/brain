package retriever

import (
	"context"
	"fmt"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
)

// DeepFilterFunc is the signature for an LLM-based chunk filter. It receives
// the question and a formatted list of chunks, and returns the indices
// (0-based) of the most relevant ones. Injected by the commands layer to
// avoid an import cycle between retriever and llm.
type DeepFilterFunc func(ctx context.Context, question, chunkList string) (string, error)

// DeepFilter takes 20 retrieved chunks and uses an LLM call to select the
// most relevant 8-10 for deeper synthesis. Returns the filtered subset in
// score order. Falls back to the original set on any error.
func DeepFilter(ctx context.Context, chunks []Chunk, question string, llmCall DeepFilterFunc) []Chunk {
	if len(chunks) <= 10 || llmCall == nil {
		return chunks
	}

	var listing strings.Builder
	for i, c := range chunks {
		pct := int(c.Score*100 + 0.5)
		snippet := c.Snippet
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		fmt.Fprintf(&listing, "[%d] %s (%d%%) — %s\n", i, c.DisplayPath, pct, snippet)
	}

	prompt := fmt.Sprintf(`Given this question: %q

Here are %d retrieved chunks from a personal knowledge base:

%s
Select the 8-10 chunks that are MOST relevant for answering the question in depth.
Return ONLY the chunk numbers as a comma-separated list, e.g.: 0,2,5,7,9,11,14,16
No explanations, just the numbers.`, question, len(chunks), listing.String())

	result, err := llmCall(ctx, "You select the most relevant chunks for deep analysis. Return only comma-separated chunk indices.", prompt)
	if err != nil || result == "" {
		return chunks
	}

	selected := parseIndices(result, len(chunks))
	if len(selected) < 3 {
		return chunks
	}

	out := make([]Chunk, 0, len(selected))
	for _, idx := range selected {
		out = append(out, chunks[idx])
	}
	return out
}

func parseIndices(raw string, max int) []int {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err == nil && n >= 0 && n < max && !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	topK := config.Default.TopK
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}
