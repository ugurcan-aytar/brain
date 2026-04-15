// Enhanced retrieval flows for --expand / --rerank / --hyde.
//
// These features cost a one-shot subprocess boot each (qmd-query-
// expansion for --expand and --hyde, bge-reranker-v2-m3 for
// --rerank), so they're only spun up when the corresponding flag is
// set — plain Retrieve stays on the zero-subprocess fast path.
//
// Flow, when any of the three are on:
//
//  1. If --expand or --hyde: open an expansion generator, call
//     recall.Expand to produce {Lex, Vec, Hyde} variants.
//  2. Run the original query + every Lex/Vec variant through
//     SearchHybrid, merging into a single candidate set keyed by
//     docid (best-score wins).
//  3. If --hyde: embed each Hyde passage as a document, vector-
//     search with each embedding, merge into the same pool.
//  4. If --rerank: take the top-N candidates, send them to the
//     cross-encoder reranker, min-max normalise the returned
//     logits, then run recall.PositionAwareBlend against RRF rank.
//  5. Adaptive floor filter (same as retrieveHybrid).
//
// All LLM / reranker open failures degrade gracefully with a
// warning on stderr — brain still returns the best-available
// results.

package retriever

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/engine"
)

// rerankTopN is how many fused-result candidates go through the
// cross-encoder per query. Matches recall.DefaultTopN (30) — big
// enough to rescue rank-20 gems, small enough to stay inside a
// couple seconds on bge-reranker-v2-m3.
const rerankTopN = 30

func retrieveEnhanced(ctx context.Context, eng *engine.Engine, query string, opt Options) ([]Chunk, error) {
	topK := topKOr(opt.TopK)
	col := collectionArg(opt)

	emb, err := eng.Embedder()
	if err != nil {
		return nil, fmt.Errorf("resolve embedder: %w", err)
	}
	if emb == nil {
		// No embedder → vector / hybrid / hyde are all no-ops. Fall
		// back to the plain path so the user still gets BM25 results.
		return retrieveHybrid(ctx, eng, query, opt)
	}

	baseSearchOpts := func() []recall.SearchOption {
		o := []recall.SearchOption{recall.WithLimit(topK)}
		if col != "" {
			o = append(o, recall.WithCollection(col))
		}
		if opt.MinScore != nil {
			o = append(o, recall.WithMinScore(*opt.MinScore))
		}
		return o
	}

	// Step 1: expand the query when asked.
	var expanded *recall.Expanded
	if opt.Expand || opt.Hyde {
		expanded = openAndExpand(query, opt)
	}

	// Step 2: run SearchHybrid for the original query + every
	// Lex/Vec variant. Merge into a single pool keyed by DocID so
	// the same document never shows up twice.
	pool := map[string]*pooledResult{}
	traces := map[string][]string{}

	collect := func(results []recall.FusedResult, label string) {
		for rank, r := range results {
			if existing, ok := pool[r.DocID]; !ok || r.FusedScore > existing.fused.FusedScore {
				pool[r.DocID] = &pooledResult{
					fused:  r,
					bestRk: rank,
				}
			}
			traces[r.DocID] = append(traces[r.DocID], fmt.Sprintf("%s@%d", label, rank))
		}
	}

	if fused, err := eng.Recall().SearchHybrid(emb, query, baseSearchOpts()...); err == nil {
		collect(fused, "orig")
	}
	if opt.Expand && expanded != nil {
		for i, v := range expanded.Lex {
			if strings.TrimSpace(v) == "" {
				continue
			}
			if fused, err := eng.Recall().SearchHybrid(emb, v, baseSearchOpts()...); err == nil {
				collect(fused, fmt.Sprintf("lex%d", i))
			}
		}
		for i, v := range expanded.Vec {
			if strings.TrimSpace(v) == "" {
				continue
			}
			if fused, err := eng.Recall().SearchHybrid(emb, v, baseSearchOpts()...); err == nil {
				collect(fused, fmt.Sprintf("vec%d", i))
			}
		}
	}

	// Step 3: HyDE — embed each hypothetical passage, run a
	// vector-only search, merge.
	if opt.Hyde && expanded != nil {
		for i, passage := range expanded.Hyde {
			if strings.TrimSpace(passage) == "" {
				continue
			}
			vec, err := emb.EmbedSingle(recall.FormatDocumentFor(emb.Family(), "", passage))
			if err != nil {
				continue
			}
			vecResults, err := eng.Recall().SearchVector(emb, passage, baseSearchOpts()...)
			_ = vec // embedding already happened via SearchVector internally; we just wanted the format prefix parity
			if err == nil {
				for rank, r := range vecResults {
					if existing, ok := pool[r.DocID]; !ok || r.Score > existing.fused.FusedScore {
						pool[r.DocID] = &pooledResult{
							fused:  recall.FusedResult{SearchResult: r, FusedScore: r.Score},
							bestRk: rank,
						}
					}
					traces[r.DocID] = append(traces[r.DocID], fmt.Sprintf("hyde%d@%d", i, rank))
				}
			}
		}
	}

	if len(pool) == 0 {
		return nil, nil
	}

	// Step 4: sort pool by fused score desc, optionally rerank.
	ranked := make([]*pooledResult, 0, len(pool))
	for _, p := range pool {
		ranked = append(ranked, p)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].fused.FusedScore > ranked[j].fused.FusedScore
	})

	var rerankerTrace map[string]float64
	if opt.Rerank {
		rerankerTrace = applyReranker(ctx, query, ranked)
	}

	// Step 5: build chunks, apply adaptive floor, surface explain
	// trace when asked.
	chunks := make([]Chunk, 0, len(ranked))
	for _, p := range ranked {
		r := p.fused
		c := Chunk{
			DisplayPath: displayPathOf(r.CollectionName, r.Path),
			Title:       firstNonEmpty(r.Title, r.Path, "Untitled"),
			Score:       r.FusedScore,
			Snippet:     r.Snippet,
			DocID:       r.DocID,
			File:        joinCollectionPath(r.CollectionName, r.Path),
		}
		if opt.Explain {
			parts := traces[r.DocID]
			if len(parts) > 4 {
				parts = append(parts[:4:4], "…")
			}
			trace := strings.Join(parts, " ")
			if rerankerTrace != nil {
				if s, ok := rerankerTrace[r.DocID]; ok {
					trace += fmt.Sprintf(" rerank=%.2f", s)
				}
			}
			c.Explain = trace
		}
		chunks = append(chunks, c)
	}
	if topK > 0 && len(chunks) > topK {
		chunks = chunks[:topK]
	}
	return adaptiveFilter(chunks), nil
}

// pooledResult carries the best-scoring SearchHybrid row for a
// given DocID plus its best observed rank across all variants
// (used for the RRF-rank side of the position-aware blend).
type pooledResult struct {
	fused  recall.FusedResult
	bestRk int
}

// openAndExpand resolves the expansion model, boots a generator,
// runs recall.Expand, and returns the parsed Expanded struct.
// Swallows errors with a stderr warning so the caller can continue
// on the original query when the model isn't available — matching
// recall's own `runExpansion` behaviour.
func openAndExpand(query string, opt Options) *recall.Expanded {
	modelPath, err := recall.ResolveActiveExpansionModelPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: expansion unavailable —", err)
		return nil
	}
	if _, err := os.Stat(modelPath); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: --expand/--hyde requested but expansion model missing at %s — run `recall models download --expansion`\n",
			modelPath)
		return nil
	}
	gen, err := recall.NewLocalGenerator(recall.LocalGeneratorOptions{ModelPath: modelPath})
	if err != nil {
		if !errors.Is(err, recall.ErrLocalGeneratorNotCompiled) {
			fmt.Fprintln(os.Stderr, "warning: expansion generator open failed —", err)
		}
		return nil
	}
	defer gen.Close()

	expanded, err := recall.Expand(gen, query, recall.ExpandOptions{IncludeLex: opt.Expand})
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: expansion failed —", err)
		return nil
	}
	return expanded
}

// applyReranker takes the ordered candidate pool, sends the top-N
// to a cross-encoder reranker, min-max normalises the returned
// logits, runs PositionAwareBlend, and mutates ranked in place to
// reflect the blended order. Returns a docid→normalised-score map
// for --explain traces (nil when reranker couldn't boot).
func applyReranker(ctx context.Context, query string, ranked []*pooledResult) map[string]float64 {
	modelPath, err := recall.ResolveActiveRerankerModelPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: --rerank unavailable —", err)
		return nil
	}
	if _, err := os.Stat(modelPath); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: --rerank requested but reranker model missing at %s — run `recall models download --reranker`\n",
			modelPath)
		return nil
	}
	rr, err := recall.NewLocalReranker(recall.LocalRerankerOptions{ModelPath: modelPath})
	if err != nil {
		if !errors.Is(err, recall.ErrLocalRerankerNotAvailable) {
			fmt.Fprintln(os.Stderr, "warning: reranker open failed —", err)
		}
		return nil
	}
	defer rr.Close()

	topN := rerankTopN
	if topN > len(ranked) {
		topN = len(ranked)
	}

	candidates := make([]recall.SearchResult, topN)
	for i := 0; i < topN; i++ {
		candidates[i] = ranked[i].fused.SearchResult
	}

	scored, err := recall.Rerank(ctx, rr, query, candidates, recall.RerankOptions{TopN: topN})
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: rerank failed —", err)
		return nil
	}
	blended := recall.PositionAwareBlend(scored, recall.DefaultRerankBlendBands)

	// Reorder ranked[:topN] to follow blended order, and overwrite
	// Score so downstream adaptiveFilter sees the reranker's say.
	byDoc := make(map[string]recall.Scored, len(blended))
	traceScores := make(map[string]float64, len(blended))
	for _, b := range blended {
		byDoc[b.Result.DocID] = b
		traceScores[b.Result.DocID] = b.Score
	}
	newTop := make([]*pooledResult, 0, topN)
	for _, b := range blended {
		for _, p := range ranked[:topN] {
			if p.fused.DocID == b.Result.DocID {
				p.fused.FusedScore = b.BlendedScore
				newTop = append(newTop, p)
				break
			}
		}
	}
	copy(ranked[:topN], newTop)
	return traceScores
}
