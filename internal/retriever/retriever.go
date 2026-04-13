package retriever

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/ugurcan-aytar/brain/internal/config"
)

// Chunk is a single retrieved note fragment returned by qmd.
type Chunk struct {
	DisplayPath string
	Title       string
	Score       float64
	Snippet     string
	DocID       string
}

// rawResult is the untyped shape of a qmd JSON entry — qmd uses a few
// different field names across versions, so we normalize in parse().
type rawResult struct {
	DocID       string  `json:"docid"`
	DisplayPath string  `json:"displayPath"`
	Title       string  `json:"title"`
	Score       float64 `json:"score"`
	Snippet     string  `json:"snippet"`
	Content     string  `json:"content"`
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\[\?[0-9]*[a-zA-Z]`)

// extractJSON strips ANSI escapes from qmd stdout and slices the first top-level
// JSON array out of the noise. qmd sometimes prints banner lines before/after.
func extractJSON(raw string) string {
	clean := ansiRegexp.ReplaceAllString(raw, "")
	start := strings.Index(clean, "[")
	end := strings.LastIndex(clean, "]")
	if start == -1 || end == -1 || end <= start {
		return "[]"
	}
	return clean[start : end+1]
}

// ErrQmdMissing is returned when the qmd binary is not in PATH.
var ErrQmdMissing = errors.New("qmd is not installed or not found in PATH")

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

func buildQmdArgs(query string, opt Options, minScore float64) []string {
	args := []string{"query", query, "--json", "-n", fmt.Sprintf("%d", topKOr(opt.TopK)), "--min-score", fmt.Sprintf("%g", minScore)}
	if opt.Collection != "" {
		args = append(args, "-c", opt.Collection)
	}
	return args
}

// runSingleQuery invokes qmd once and parses the JSON array out of stdout.
// Returns an empty slice (no error) if qmd exits 130 (SIGINT) so that cancelled
// requests don't surface as errors.
func runSingleQuery(ctx context.Context, query string, opt Options) ([]Chunk, error) {
	minScore := config.Default.MinScore
	if opt.MinScore != nil {
		minScore = *opt.MinScore
	}
	args := buildQmdArgs(query, opt, minScore)

	cmd := exec.CommandContext(ctx, config.Default.QmdBinary, args...)
	cmd.Env = config.QmdEnv()

	stdout, err := cmd.Output()
	if err != nil {
		// exec.Command returns an error wrapping exec.ExitError on non-zero exit.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 {
				return nil, nil // cancelled — not an error
			}
			return nil, fmt.Errorf("qmd exited with code %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		// Context cancelled → return empty cleanly.
		if ctx.Err() != nil {
			return nil, nil
		}
		// Missing binary
		var notFound *exec.Error
		if errors.As(err, &notFound) {
			return nil, ErrQmdMissing
		}
		return nil, err
	}

	var parsed []rawResult
	if err := json.Unmarshal([]byte(extractJSON(string(stdout))), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse qmd output: %w", err)
	}

	out := make([]Chunk, 0, len(parsed))
	for _, r := range parsed {
		c := Chunk{
			DisplayPath: firstNonEmpty(r.DisplayPath, r.Title, "unknown"),
			Title:       firstNonEmpty(r.Title, r.DisplayPath, "Untitled"),
			Score:       r.Score,
			Snippet:     firstNonEmpty(r.Snippet, r.Content),
			DocID:       r.DocID,
		}
		if c.Score >= minScore {
			out = append(out, c)
		}
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

type collectionResult struct {
	chunks []Chunk
	err    error
}

// Retrieve fans out across collections (if provided), merges results by docid
// keeping the highest score per document, applies adaptive minimum-score
// filtering, and returns them sorted descending.
func Retrieve(ctx context.Context, query string, opt Options) ([]Chunk, error) {
	if len(opt.Collections) == 0 {
		chunks, err := runSingleQuery(ctx, query, opt)
		if err != nil {
			return nil, err
		}
		return adaptiveFilter(chunks), nil
	}

	results := make([]collectionResult, len(opt.Collections))
	var wg sync.WaitGroup
	for i, c := range opt.Collections {
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			perCall := opt
			perCall.Collection = c
			perCall.Collections = nil
			chunks, err := runSingleQuery(ctx, query, perCall)
			results[i] = collectionResult{chunks, err}
		}(i, c)
	}
	wg.Wait()

	seen := map[string]Chunk{}
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
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
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return adaptiveFilter(out), nil
}

// adaptiveFilter replaces the old hard MinScore cutoff. Instead of a fixed
// 0.2 threshold that silently drops chunks on difficult queries, we use
// 40% of the top score as the floor. When the top chunk scores 0.9 the
// floor is 0.36 (junk is dropped). When the top chunk scores 0.3 the
// floor is 0.12 (weak-but-best results survive).
func adaptiveFilter(chunks []Chunk) []Chunk {
	if len(chunks) == 0 {
		return chunks
	}
	topScore := chunks[0].Score
	floor := topScore * 0.4
	hardFloor := 0.05 // absolute minimum — never surface pure noise
	if floor < hardFloor {
		floor = hardFloor
	}
	out := make([]Chunk, 0, len(chunks))
	for _, c := range chunks {
		if c.Score >= floor {
			out = append(out, c)
		}
	}
	return out
}

// RawSearch is like Retrieve but with topK=10 and no minimum-score filter.
// Used by `brain search`.
func RawSearch(ctx context.Context, query string, opt Options) ([]Chunk, error) {
	zero := 0.0
	opt.MinScore = &zero
	if opt.TopK == 0 {
		opt.TopK = 10
	}
	return Retrieve(ctx, query, opt)
}

// GroundingGate reports whether we have enough chunks to call the LLM.
// It also prints user-facing warnings so callers can simply bail on false.
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

// tiny color helpers — avoids a ui import cycle (ui depends on retriever).
func yellow(s string) string { return "\x1b[33m" + s + "\x1b[0m" }
func dim(s string) string    { return "\x1b[2m" + s + "\x1b[0m" }
