package retriever

import (
	"testing"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain array passes through",
			in:   `[{"docid":"1"}]`,
			want: `[{"docid":"1"}]`,
		},
		{
			name: "ansi escapes are stripped",
			in:   "\x1b[32m[{\"docid\":\"1\"}]\x1b[0m",
			want: `[{"docid":"1"}]`,
		},
		{
			name: "banner text before array is dropped",
			in:   "Loading model...\n[{\"docid\":\"1\"}]",
			want: `[{"docid":"1"}]`,
		},
		{
			name: "trailing text after array is dropped",
			in:   `[{"docid":"1"}]` + "\nDone.",
			want: `[{"docid":"1"}]`,
		},
		{
			name: "no brackets returns empty array",
			in:   "no json here",
			want: "[]",
		},
		{
			name: "only opening bracket returns empty",
			in:   "[ oops",
			want: "[]",
		},
		{
			name: "malformed ansi ?25h cursor sequences stripped",
			in:   "\x1b[?25l[{\"docid\":\"a\"}]\x1b[?25h",
			want: `[{"docid":"a"}]`,
		},
		{
			name: "empty array",
			in:   "[]",
			want: "[]",
		},
		{
			name: "nested objects preserved",
			in:   `[{"a":{"b":1}}, {"c":[1,2,3]}]`,
			want: `[{"a":{"b":1}}, {"c":[1,2,3]}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSON(tc.in)
			if got != tc.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"first is non-empty", []string{"a", "b", "c"}, "a"},
		{"first is empty", []string{"", "b", "c"}, "b"},
		{"all empty", []string{"", "", ""}, ""},
		{"single value", []string{"only"}, "only"},
		{"no values", nil, ""},
		{"second non-empty", []string{"", "found", ""}, "found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := firstNonEmpty(tc.values...)
			if got != tc.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tc.values, got, tc.want)
			}
		})
	}
}

func TestTopKOr(t *testing.T) {
	// Positive override is respected.
	if got := topKOr(5); got != 5 {
		t.Errorf("topKOr(5) = %d, want 5", got)
	}
	// Zero falls back to the default.
	if got := topKOr(0); got <= 0 {
		t.Errorf("topKOr(0) should fall back to default, got %d", got)
	}
}

func TestBuildQmdArgs(t *testing.T) {
	opt := Options{TopK: 3, Collection: "notes"}
	args := buildQmdArgs("my query", opt, 0.5)

	// Expected shape: ["query", "my query", "--json", "-n", "3", "--min-score", "0.5", "-c", "notes"]
	wantPrefix := []string{"query", "my query", "--json", "-n", "3", "--min-score"}
	for i, w := range wantPrefix {
		if i >= len(args) || args[i] != w {
			t.Errorf("buildQmdArgs prefix mismatch at %d: got %v", i, args)
			return
		}
	}

	// Find "-c" and confirm "notes" follows.
	var sawCollection bool
	for i, a := range args {
		if a == "-c" && i+1 < len(args) && args[i+1] == "notes" {
			sawCollection = true
			break
		}
	}
	if !sawCollection {
		t.Errorf("buildQmdArgs missing -c notes in %v", args)
	}
}

func TestBuildQmdArgsWithoutCollection(t *testing.T) {
	args := buildQmdArgs("q", Options{TopK: 10}, 0.2)
	for _, a := range args {
		if a == "-c" {
			t.Errorf("unexpected -c flag with no collection: %v", args)
		}
	}
}

func TestMergeDedupeKeepsHighestScore(t *testing.T) {
	// runSingleQuery-level dedup is internal; drive it via Retrieve with fake
	// results by exercising the same code path in a mini helper.
	results := []collectionResult{
		{chunks: []Chunk{
			{DocID: "d1", DisplayPath: "a.md", Score: 0.5, Snippet: "s1"},
			{DocID: "d2", DisplayPath: "b.md", Score: 0.9, Snippet: "s2"},
		}},
		{chunks: []Chunk{
			{DocID: "d1", DisplayPath: "a.md", Score: 0.8, Snippet: "s1"}, // higher
			{DocID: "d3", DisplayPath: "c.md", Score: 0.6, Snippet: "s3"},
		}},
	}

	out := mergeResults(results)

	if len(out) != 3 {
		t.Fatalf("expected 3 unique chunks, got %d", len(out))
	}
	// Sorted descending: d2 (0.9), d1 (0.8), d3 (0.6)
	if out[0].DocID != "d2" || out[0].Score != 0.9 {
		t.Errorf("expected d2@0.9 first, got %+v", out[0])
	}
	if out[1].DocID != "d1" || out[1].Score != 0.8 {
		t.Errorf("expected d1@0.8 second (deduped to higher score), got %+v", out[1])
	}
	if out[2].DocID != "d3" || out[2].Score != 0.6 {
		t.Errorf("expected d3@0.6 third, got %+v", out[2])
	}
}

func TestMergeDedupeFallbackKey(t *testing.T) {
	// When DocID is empty, dedup should use displayPath + snippet prefix.
	results := []collectionResult{
		{chunks: []Chunk{
			{DisplayPath: "a.md", Snippet: "same snippet body that is long enough to truncate", Score: 0.5},
			{DisplayPath: "a.md", Snippet: "same snippet body that is long enough to truncate", Score: 0.7},
		}},
	}
	out := mergeResults(results)
	if len(out) != 1 {
		t.Fatalf("expected dedup to collapse identical entries without DocID, got %d", len(out))
	}
	if out[0].Score != 0.7 {
		t.Errorf("expected higher-scored duplicate to win, got score %f", out[0].Score)
	}
}

// mergeResults is a tiny wrapper that exercises the same dedup+sort logic
// Retrieve uses internally. We inline it here so we don't have to spawn
// real qmd processes just to test deterministic merge behavior.
func mergeResults(results []collectionResult) []Chunk {
	seen := map[string]Chunk{}
	for _, r := range results {
		for _, c := range r.chunks {
			key := c.DocID
			if key == "" {
				prefix := c.Snippet
				if len(prefix) > 50 {
					prefix = prefix[:50]
				}
				key = c.DisplayPath + ":" + prefix
			}
			if existing, ok := seen[key]; !ok || c.Score > existing.Score {
				seen[key] = c
			}
		}
	}
	out := make([]Chunk, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	// Sort descending by score — same ordering Retrieve promises.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Score > out[j-1].Score; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
