package prompt

import (
	"strings"
	"testing"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		query string
		want  QueryType
	}{
		// Decision
		{"Should I use Postgres or MySQL?", Decision},
		{"Yapmalı mıyım bu değişikliği?", Decision},
		{"What is the best approach here?", Decision},
		{"Pros and cons of microservices", Decision},
		{"Recommend a testing framework", Decision},
		{"Is it worth it to refactor?", Decision},
		{"Trade-off between speed and safety", Decision},
		{"Go vs Rust", Decision},

		// Synthesis
		{"Build me a roadmap for Q1", Synthesis},
		{"How can I scale my startup?", Synthesis},
		{"Give me an action plan", Synthesis},
		{"Strateji ne olmalı?", Synthesis},
		{"How do I launch this product", Synthesis},

		// Recall
		{"What did I write about caching?", Recall},
		{"Find notes about authentication", Recall},
		{"List my ideas", Recall},
		{"What is a monad?", Recall},
		{"Show me the notes on kubernetes", Recall},
		{"Nedir bu kavram?", Recall},

		// Analysis
		{"Why did the deploy fail?", Analysis},
		{"Compare REST and GraphQL", Analysis},
		{"Explain the observer pattern", Analysis},
		{"What connects these ideas?", Analysis},
		{"Neden böyle oldu?", Analysis},

		// Fallback → Analysis (no pattern matches)
		{"hello world", Analysis},
		{"foo bar baz", Analysis},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			got := Classify(tc.query)
			if got != tc.want {
				t.Errorf("Classify(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestIsValidMode(t *testing.T) {
	valid := []string{"auto", "recall", "analysis", "decision", "synthesis"}
	for _, m := range valid {
		if !IsValidMode(m) {
			t.Errorf("IsValidMode(%q) = false, want true", m)
		}
	}

	invalid := []string{"", "sonnet", "opus", "random", "ANALYSIS", "recall "}
	for _, m := range invalid {
		if IsValidMode(m) {
			t.Errorf("IsValidMode(%q) = true, want false", m)
		}
	}
}

func makeChunks(count int, minScore float64) []retriever.Chunk {
	chunks := make([]retriever.Chunk, count)
	for i := 0; i < count; i++ {
		chunks[i] = retriever.Chunk{
			DisplayPath: "note.md",
			Title:       "Note",
			Score:       minScore + float64(i)*0.01,
			Snippet:     "snippet content",
		}
	}
	return chunks
}

func TestBuildSystemPromptContainsCoreBlocks(t *testing.T) {
	chunks := makeChunks(3, 0.5)
	got := BuildSystemPrompt(chunks, "explain this", "")

	mustContain := []string{
		"GROUNDING RULES",
		"SOURCE ANALYSIS PROTOCOL",
		"DEEP EXTRACTION RULES",
		"SYNTHESIS RULES",
		"HARD BOUNDARIES",
		"STRUCTURE YOUR RESPONSE",
		"Context from personal knowledge base:",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("system prompt missing required block %q", s)
		}
	}
}

func TestBuildSystemPromptModeSelection(t *testing.T) {
	chunks := makeChunks(3, 0.5)

	t.Run("auto-classifies from query", func(t *testing.T) {
		p := BuildSystemPrompt(chunks, "Why did this fail?", "")
		if !strings.Contains(p, "KEY FINDINGS") {
			t.Error("expected analysis mode directive when classifier picks analysis")
		}
	})

	t.Run("override wins over classifier", func(t *testing.T) {
		// Query would classify as Decision, but we override to Recall.
		p := BuildSystemPrompt(chunks, "Should I refactor?", Recall)
		if !strings.Contains(p, "DIRECT ANSWER") {
			t.Error("expected recall mode directive when override is Recall")
		}
		if strings.Contains(p, "RELEVANT FRAMEWORKS") {
			t.Error("decision directive should not appear when override is Recall")
		}
	})

	t.Run("no query + no override defaults to analysis", func(t *testing.T) {
		p := BuildSystemPrompt(chunks, "", "")
		if !strings.Contains(p, "KEY FINDINGS") {
			t.Error("expected analysis directive as default")
		}
	})
}

func TestBuildSystemPromptSourceQuality(t *testing.T) {
	t.Run("rich material", func(t *testing.T) {
		// 5+ chunks with 3+ high-relevance (≥0.7)
		chunks := []retriever.Chunk{
			{Score: 0.9, DisplayPath: "a.md", Snippet: "x"},
			{Score: 0.85, DisplayPath: "b.md", Snippet: "x"},
			{Score: 0.75, DisplayPath: "c.md", Snippet: "x"},
			{Score: 0.5, DisplayPath: "d.md", Snippet: "x"},
			{Score: 0.3, DisplayPath: "e.md", Snippet: "x"},
		}
		p := BuildSystemPrompt(chunks, "", "")
		if !strings.Contains(p, "rich material") {
			t.Error("expected rich material description")
		}
	})

	t.Run("moderate material", func(t *testing.T) {
		chunks := makeChunks(3, 0.3)
		p := BuildSystemPrompt(chunks, "", "")
		if !strings.Contains(p, "moderate material") {
			t.Error("expected moderate material description")
		}
	})

	t.Run("limited material", func(t *testing.T) {
		chunks := makeChunks(1, 0.3)
		p := BuildSystemPrompt(chunks, "", "")
		if !strings.Contains(p, "limited material") {
			t.Error("expected limited material description")
		}
	})
}

func TestBuildSystemPromptEmbedsChunks(t *testing.T) {
	chunks := []retriever.Chunk{
		{DisplayPath: "alpha.md", Title: "Alpha", Score: 0.8, Snippet: "alpha snippet body"},
		{DisplayPath: "beta.md", Title: "Beta", Score: 0.3, Snippet: "beta snippet body"},
	}
	p := BuildSystemPrompt(chunks, "explain", "")

	for _, want := range []string{
		"[alpha.md]",
		"alpha snippet body",
		"80% — HIGH RELEVANCE",
		"[beta.md]",
		"beta snippet body",
		"30% — LOW RELEVANCE",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestBuildChallengePromptTruncatesLongAnswer(t *testing.T) {
	longAnswer := strings.Repeat("word ", 1000) // ~5000 chars
	origSources := []retriever.Chunk{{DisplayPath: "a.md", Score: 0.7, Snippet: "s"}}
	challengeSources := []retriever.Chunk{{DisplayPath: "b.md", Score: 0.8, Snippet: "s"}}

	p := BuildChallengePrompt("question", longAnswer, origSources, challengeSources)

	if !strings.Contains(p, "[truncated]") {
		t.Error("expected [truncated] marker on long answer")
	}
	if strings.Count(p, "word ") > 700 {
		// Rough upper bound: we only truncate at 3000 chars, so word count ≤ 600.
		t.Error("answer appears untruncated")
	}
}

func TestBuildChallengePromptKeepsShortAnswer(t *testing.T) {
	shortAnswer := "brief answer"
	origSources := []retriever.Chunk{{DisplayPath: "a.md", Score: 0.7, Snippet: "s"}}
	challengeSources := []retriever.Chunk{{DisplayPath: "b.md", Score: 0.8, Snippet: "s"}}

	p := BuildChallengePrompt("question", shortAnswer, origSources, challengeSources)

	if strings.Contains(p, "[truncated]") {
		t.Error("short answer should not be truncated")
	}
	if !strings.Contains(p, shortAnswer) {
		t.Error("short answer should appear verbatim")
	}
	if !strings.Contains(p, "CHALLENGE review") {
		t.Error("challenge prompt header missing")
	}
}

func TestClassifyEmptyStringDefaultsToAnalysis(t *testing.T) {
	if got := Classify(""); got != Analysis {
		t.Errorf("Classify(\"\") = %q, want %q", got, Analysis)
	}
}

func TestStaticDirectivesStableAcrossCalls(t *testing.T) {
	a := StaticDirectives()
	b := StaticDirectives()
	if a != b {
		t.Error("StaticDirectives should return the same string on every call")
	}
}

func TestStaticDirectivesContainsAllModes(t *testing.T) {
	sd := StaticDirectives()
	for _, mode := range []string{"RECALL", "ANALYSIS", "DECISION", "SYNTHESIS"} {
		if !strings.Contains(sd, "["+mode+"]") {
			t.Errorf("StaticDirectives missing mode section [%s]", mode)
		}
	}
	for _, block := range []string{"GROUNDING RULES", "SOURCE ANALYSIS PROTOCOL", "DEEP EXTRACTION", "SYNTHESIS RULES", "HARD BOUNDARIES"} {
		if !strings.Contains(sd, block) {
			t.Errorf("StaticDirectives missing %q", block)
		}
	}
}

func TestStaticDirectivesNoChunks(t *testing.T) {
	sd := StaticDirectives()
	if strings.Contains(sd, "Context from personal knowledge base:") {
		t.Error("StaticDirectives should NOT contain chunks")
	}
}

func TestContextBlockContainsMode(t *testing.T) {
	chunks := makeChunks(2, 0.5)
	cb := ContextBlock(chunks, "Why did this happen?", "")
	if !strings.Contains(cb, "[Active mode: analysis]") {
		t.Error("ContextBlock should contain auto-detected mode label")
	}
}

func TestContextBlockOverrideMode(t *testing.T) {
	chunks := makeChunks(2, 0.5)
	cb := ContextBlock(chunks, "anything", Recall)
	if !strings.Contains(cb, "[Active mode: recall]") {
		t.Errorf("ContextBlock should use override mode, got:\n%s", cb[:200])
	}
}

func TestContextBlockContainsChunks(t *testing.T) {
	chunks := []retriever.Chunk{
		{DisplayPath: "test/note.md", Score: 0.7, Snippet: "unique snippet content"},
	}
	cb := ContextBlock(chunks, "query", "")
	if !strings.Contains(cb, "unique snippet content") {
		t.Error("ContextBlock should embed chunk snippets")
	}
	if !strings.Contains(cb, "[test/note.md]") {
		t.Error("ContextBlock should embed chunk display paths")
	}
}

func TestBuildSystemPromptStillWorks(t *testing.T) {
	chunks := makeChunks(2, 0.5)
	combined := BuildSystemPrompt(chunks, "Why?", "")
	if !strings.Contains(combined, "Context from personal knowledge base:") {
		t.Error("BuildSystemPrompt should still contain chunks for non-caching backends")
	}
	if !strings.Contains(combined, "GROUNDING RULES") {
		t.Error("BuildSystemPrompt should still contain directives")
	}
}
