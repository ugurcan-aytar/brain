package retriever

import (
	"context"
	"math"
	"sort"
	"sync"
)

// RetrieveMulti runs multiple queries in parallel and merges the results
// using Reciprocal Rank Fusion (RRF). A chunk that appears in multiple
// query results gets a higher fused score than one that scores high on
// a single query — this is what catches the "ücra köşedeki" chunks that
// score medium on several reformulations.
func RetrieveMulti(ctx context.Context, queries []string, opt Options) ([]Chunk, error) {
	if len(queries) <= 1 {
		q := ""
		if len(queries) == 1 {
			q = queries[0]
		}
		return Retrieve(ctx, q, opt)
	}

	type queryResult struct {
		chunks []Chunk
		err    error
	}
	results := make([]queryResult, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			chunks, err := Retrieve(ctx, q, opt)
			results[i] = queryResult{chunks, err}
		}(i, q)
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, nil
	}

	// Collect ranked lists per query for RRF.
	const k = 60.0 // standard RRF constant
	scores := map[string]float64{}
	best := map[string]Chunk{}

	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		for rank, c := range r.chunks {
			key := chunkKey(c)
			scores[key] += 1.0 / (k + float64(rank+1))
			if existing, ok := best[key]; !ok || c.Score > existing.Score {
				best[key] = c
			}
		}
	}

	out := make([]Chunk, 0, len(best))
	for key, c := range best {
		c.Score = math.Min(scores[key]*k, 1.0) // normalize to [0,1] range
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })

	topK := topKOr(opt.TopK)
	if len(out) > topK {
		out = out[:topK]
	}
	return out, nil
}

func chunkKey(c Chunk) string {
	if c.DocID != "" {
		return c.DocID
	}
	prefix := c.Snippet
	if len(prefix) > 50 {
		prefix = prefix[:50]
	}
	return c.DisplayPath + ":" + prefix
}
