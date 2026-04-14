// Package retriever wraps recall.Engine with brain's adaptive scoring,
// grounding gate, and multi-collection helpers. Callers (ask, chat,
// search) hand in a *engine.Engine + Options and get back scored
// []Chunk results ready for prompt assembly.
package retriever

import (
	"context"
	"fmt"
	"strings"

	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/engine"
)

// Chunk is brain's normalized view of a retrieved note fragment. Kept
// stable across the qmd → recall migration so prompt / ui / history
// packages don't need to change.
type Chunk struct {
	DisplayPath string
	Title       string
	Score       float64
	Snippet     string
	DocID       string
	File        string // recall "path" — "<collection>/<relative-path>"
}

// Options configures a retrieval call. Collection scopes a single
// collection; Collections fans out to several (joined with commas and
// passed to recall, which handles multi-collection querying natively).
// TopK defaults to config.Default.TopK when zero; MinScore overrides the
// adaptive floor when non-nil.
type Options struct {
	Collection  string
	Collections []string
	TopK        int
	MinScore    *float64 // nil = use config default
}

func topKOr(n int) int {
	if n > 0 {
		return n
	}
	return config.Default.TopK
}

// collectionArg joins Collection + Collections into the single string
// recall's SearchOptions.Collection expects (comma-separated, "" for
// all).
func collectionArg(opt Options) string {
	names := map[string]struct{}{}
	if opt.Collection != "" {
		names[opt.Collection] = struct{}{}
	}
	for _, c := range opt.Collections {
		if c != "" {
			names[c] = struct{}{}
		}
	}
	if len(names) == 0 {
		return ""
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	return strings.Join(out, ",")
}

// Retrieve runs a hybrid (BM25 + vector + RRF) search through
// recall.Engine. When no embedder is available (stub build + no API
// provider), recall degrades to BM25 automatically — brain doesn't
// need to branch.
//
// Results are adaptively filtered (40% of top score floor) before
// return, matching the behaviour brain had against qmd.
func Retrieve(ctx context.Context, eng *engine.Engine, query string, opt Options) ([]Chunk, error) {
	_ = ctx // reserved — recall's Engine doesn't thread ctx yet; see recall#ctx.

	topK := topKOr(opt.TopK)
	col := collectionArg(opt)

	emb, err := eng.Embedder()
	if err != nil {
		return nil, fmt.Errorf("resolve embedder: %w", err)
	}

	searchOpts := []recall.SearchOption{recall.WithLimit(topK)}
	if col != "" {
		searchOpts = append(searchOpts, recall.WithCollection(col))
	}
	if opt.MinScore != nil {
		searchOpts = append(searchOpts, recall.WithMinScore(*opt.MinScore))
	}

	fused, err := eng.Recall().SearchHybrid(emb, query, searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}

	chunks := make([]Chunk, 0, len(fused))
	for _, r := range fused {
		chunks = append(chunks, Chunk{
			DisplayPath: displayPathOf(r.CollectionName, r.Path),
			Title:       firstNonEmpty(r.Title, r.Path, "Untitled"),
			Score:       r.FusedScore,
			Snippet:     r.Snippet,
			DocID:       r.DocID,
			File:        joinCollectionPath(r.CollectionName, r.Path),
		})
	}
	return adaptiveFilter(chunks), nil
}

// RawSearch runs a single BM25 query through recall.Engine with no
// min-score filter and a small default TopK. Used by `brain search`
// when the user wants to eyeball the raw retrieval surface.
func RawSearch(ctx context.Context, eng *engine.Engine, query string, opt Options) ([]Chunk, error) {
	_ = ctx

	topK := opt.TopK
	if topK == 0 {
		topK = 10
	}
	col := collectionArg(opt)

	searchOpts := []recall.SearchOption{recall.WithLimit(topK), recall.WithMinScore(0)}
	if col != "" {
		searchOpts = append(searchOpts, recall.WithCollection(col))
	}

	results, err := eng.Recall().SearchBM25(query, searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("bm25 search: %w", err)
	}

	out := make([]Chunk, 0, len(results))
	for _, r := range results {
		out = append(out, Chunk{
			DisplayPath: displayPathOf(r.CollectionName, r.Path),
			Title:       firstNonEmpty(r.Title, r.Path, "Untitled"),
			Score:       r.Score,
			Snippet:     r.Snippet,
			DocID:       r.DocID,
			File:        joinCollectionPath(r.CollectionName, r.Path),
		})
	}
	return out, nil
}

// adaptiveFilter drops results below 40% of the top score. qmd's scale
// kept BM25 scores well above 0.05 in practice, so the old brain
// defensively lifted the floor to that value — but recall's FTS5 bm25
// can return 0.0 on tiny corpora or single-term matches, and lifting
// the floor to 0.05 would drop the top result. We keep the noise floor
// but only apply it when the top score is already comfortably above
// it; otherwise the top chunk always survives.
func adaptiveFilter(chunks []Chunk) []Chunk {
	if len(chunks) == 0 {
		return chunks
	}
	topScore := chunks[0].Score
	floor := topScore * 0.4
	const noiseFloor = 0.05
	if topScore > noiseFloor && floor < noiseFloor {
		floor = noiseFloor
	}
	out := make([]Chunk, 0, len(chunks))
	for _, c := range chunks {
		if c.Score >= floor {
			out = append(out, c)
		}
	}
	return out
}

// GroundingGate reports whether there are enough relevant chunks to
// justify an LLM call. Also prints user-facing messages so callers can
// just early-return on false.
func GroundingGate(chunks []Chunk) bool {
	if len(chunks) == 0 {
		fmt.Println(yellow("No relevant notes found for this query."))
		fmt.Println(dim("Try different keywords, or run `brain index` to re-index."))
		return false
	}
	if len(chunks) <= 2 {
		fmt.Println(dim(fmt.Sprintf("⚠ Only %d relevant note(s) found — answer may be limited.\n", len(chunks))))
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// displayPathOf formats the collection + path pair the way ui expects.
// "notes/subdir/file.md" reads better than bare "subdir/file.md" when a
// user has several collections registered.
func displayPathOf(collectionName, path string) string {
	if collectionName == "" {
		return path
	}
	if path == "" {
		return collectionName
	}
	return collectionName + "/" + path
}

// joinCollectionPath produces the spec recall.Engine.Get accepts
// ("<collection>/<path>"). Stored on Chunk.File so enrichment can fetch
// the full document.
func joinCollectionPath(collectionName, path string) string {
	if collectionName == "" || path == "" {
		return ""
	}
	return collectionName + "/" + path
}

// tiny color helpers — avoids a ui import cycle (ui depends on retriever).
func yellow(s string) string { return "\x1b[33m" + s + "\x1b[0m" }
func dim(s string) string    { return "\x1b[2m" + s + "\x1b[0m" }
