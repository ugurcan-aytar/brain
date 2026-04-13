package prompt

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

// QueryType is the classifier output — each bucket maps to a different
// response structure directive in the system prompt.
type QueryType string

const (
	Recall    QueryType = "recall"
	Analysis  QueryType = "analysis"
	Decision  QueryType = "decision"
	Synthesis QueryType = "synthesis"
)

// ValidModes lists every mode a user can explicitly pass to `brain ask -M`
// or the `/mode` slash command, plus "auto" for classifier-detected.
var ValidModes = []string{"auto", "recall", "analysis", "decision", "synthesis"}

func IsValidMode(mode string) bool {
	for _, m := range ValidModes {
		if m == mode {
			return true
		}
	}
	return false
}

// Pattern order matters — most specific intent first. A decision question
// can trivially match generic "how" patterns, so we check decision/synthesis
// before recall/analysis.
var queryPatterns = []struct {
	kind     QueryType
	patterns []*regexp.Regexp
}{
	{Decision, mustCompileAll(
		`(?i)\b(should i|should we|yapmalı mıyım|yapalım mı)\b`,
		`(?i)\b(best approach|en iyi yaklaşım|which option|hangi seçenek)\b`,
		`(?i)\b(pros and cons|artıları ve eksileri)\b`,
		`(?i)\b(decide|karar|choose|seç)\b`,
		`(?i)\b(recommend|öner|tavsiye)\b`,
		`(?i)\b(worth it|değer mi|risk)\b`,
		`(?i)\b(trade-?off|versus|vs\.?)\b`,
	)},
	{Synthesis, mustCompileAll(
		`(?i)\b(plan|planla|strateji|strategy)\b`,
		`(?i)\b(build|oluştur|create|yarat)\b.*(plan|strategy|framework|roadmap)`,
		`(?i)\b(how can i|how do i|nasıl yapabilirim|nasıl)\b.*(grow|scale|launch|build|improve|start)`,
		`(?i)\b(apply|uygula|implement|adapt)\b`,
		`(?i)\b(action|aksiyon|step|adım)\b`,
		`(?i)\b(roadmap|playbook|blueprint|çerçeve)\b`,
	)},
	{Recall, mustCompileAll(
		`(?i)\b(what|ne)\b.*(say|wrote|write|note|yazdı|yazmış|diyor)`,
		`(?i)\b(find|bul|ara)\b.*(note|not|yazı)`,
		`(?i)\b(list|listele|sırala)\b`,
		`(?i)\b(what is|nedir|ne demek)\b`,
		`(?i)\b(show me|göster)\b`,
		`(?i)\b(summarize|özetle)\b.*(note|not)`,
	)},
	{Analysis, mustCompileAll(
		`(?i)\b(why|neden|niçin)\b`,
		`(?i)\b(how does|nasıl)\b.*(relate|connect|work|ilişki|bağlantı)`,
		`(?i)\b(compare|karşılaştır|contrast|fark)\b`,
		`(?i)\b(explain|açıkla|analyze|analiz)\b`,
		`(?i)\b(pattern|örüntü|theme|tema|trend)\b`,
		`(?i)\b(what connects|ne bağlıyor|relationship|ilişki)\b`,
	)},
}

func mustCompileAll(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		out[i] = regexp.MustCompile(p)
	}
	return out
}

// Classify returns the detected QueryType, defaulting to Analysis when no
// pattern matches — analysis is the most generally useful fallback.
func Classify(query string) QueryType {
	for _, group := range queryPatterns {
		for _, p := range group.patterns {
			if p.MatchString(query) {
				return group.kind
			}
		}
	}
	return Analysis
}

func confidenceBand(score float64) string {
	switch {
	case score >= 0.7:
		return "HIGH RELEVANCE"
	case score >= 0.4:
		return "MODERATE RELEVANCE"
	default:
		return "LOW RELEVANCE"
	}
}

func formatChunks(chunks []retriever.Chunk) string {
	parts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		pct := int(c.Score*100 + 0.5)
		parts = append(parts, fmt.Sprintf("[%s] (%d%% — %s)\n%s", c.DisplayPath, pct, confidenceBand(c.Score), c.Snippet))
	}
	return strings.Join(parts, "\n---\n")
}

func describeSourceQuality(chunks []retriever.Chunk) string {
	high := 0
	for _, c := range chunks {
		if c.Score >= 0.7 {
			high++
		}
	}
	total := len(chunks)
	switch {
	case total >= 5 && high >= 3:
		return "You have rich material to work with. Prioritize depth, cross-referencing between sources, and surfacing non-obvious connections."
	case total >= 3:
		return "You have moderate material. Be thorough with what exists and clearly flag where gaps appear."
	default:
		return "You have limited material. Extract maximum value from what exists, be transparent about limitations, and do not overreach."
	}
}

const groundingRules = `GROUNDING RULES (non-negotiable):
- Every fact, framework, principle, data point, or claim MUST come from the provided context chunks.
- Every factual claim MUST cite its source using [filename].
- You MUST NOT introduce new facts, statistics, market data, or domain knowledge from outside the notes.
- If the notes don't contain relevant knowledge, say so explicitly.
- If multiple documents conflict, present both versions with their sources.`

const sourceProtocol = `SOURCE ANALYSIS PROTOCOL:
Before answering, silently assess the provided sources:
- Which chunks are directly relevant vs. tangentially related?
- Do any chunks contradict each other? If so, address the tension explicitly.
- What themes or patterns emerge across multiple chunks?
- What does the knowledge base NOT cover that the question touches on?
Lead with the strongest-grounded claims and flag weaker ones.`

const deepExtraction = `DEEP EXTRACTION RULES:
- Do not merely summarize chunks. Extract implications, second-order effects, and non-obvious connections.
- If a chunk contains a framework or principle, APPLY it to the user's specific question — don't just cite it.
- If two chunks approach the same topic differently, explain why the difference matters.
- Treat the absence of information as information: note what the knowledge base is silent about.
- Prefer specificity: if a note contains a concrete example, metric, or case study, surface it rather than abstracting it away.`

const crossSourceTension = `CROSS-SOURCE TENSION (mandatory):
Before synthesizing, identify at least one pair of sources that disagree,
contradict, or approach the topic from fundamentally different angles.
Explain the disagreement and why it matters for the user's question.
If every source genuinely agrees, say so explicitly — but look hard first.
Do NOT skip this step. Shallow agreement across many sources often hides
real tensions that matter.`

const synthesisRules = `SYNTHESIS RULES:
- You ARE allowed to reason, connect, and apply the grounded knowledge to the user's specific situation.
- You CAN draw parallels, map frameworks to new contexts, and combine insights from multiple notes.
- You MUST clearly separate what the notes say from your reasoning about it.
- Use phrases like "Your notes say X [source]. Applying this…" or "Combining [a] and [b], this suggests…"
- NEVER present synthesis as if it were a fact from the notes. Always make the leap visible.`

const hardBoundaries = `HARD BOUNDARIES:
- Zero relevant knowledge → say nothing was found. Do NOT synthesize from nothing.
- Partially relevant → synthesize from what exists and flag the gaps explicitly.
- No filler, no hedging, no preamble.
- Respond in the same language the user writes in (Turkish or English).`

var modeDirectives = map[QueryType]string{
	Recall: `STRUCTURE YOUR RESPONSE:
1. **DIRECT ANSWER**: What your notes explicitly say about this, with citations.
2. **RELATED CONTEXT**: Other relevant details from nearby notes that add useful context.

Keep it tight. The user wants facts from their notes, not a dissertation.`,

	Analysis: `STRUCTURE YOUR RESPONSE:
1. **KEY FINDINGS**: The most relevant insights from the notes, cited.
2. **CONNECTIONS**: Patterns, tensions, or complementary ideas across different sources.
3. **GAPS**: What the notes don't cover that would be important for this question.
4. **SYNTHESIS**: Your integrated understanding combining all sources.`,

	Decision: `STRUCTURE YOUR RESPONSE:
1. **RELEVANT FRAMEWORKS**: What decision frameworks, mental models, or principles the notes contain that apply here.
2. **ARGUMENTS**: What the notes suggest for and against each option, cited.
3. **BLIND SPOTS**: What the notes don't address that could affect this decision.
4. **RECOMMENDATION**: Based solely on what the notes contain, what direction they point toward — and how confident you are given the available material.`,

	Synthesis: `STRUCTURE YOUR RESPONSE:
1. **BUILDING BLOCKS**: The key ideas, frameworks, and data points from the notes that are relevant.
2. **INTEGRATION**: How these pieces fit together to address the question — connect the dots explicitly.
3. **ACTION PLAN**: Concrete steps, outputs, or next moves derived from the knowledge base.
4. **ASSUMPTIONS & GAPS**: What this plan assumes that the notes don't explicitly confirm.`,
}

// StaticDirectives returns the stable portion of the system prompt — the
// grounding rules, extraction guidelines, synthesis rules, all four mode
// templates, and hard boundaries. This block is identical across turns,
// which allows LLM backends with prompt caching to cache it once and
// reuse it cheaply.
func StaticDirectives() string {
	var allModes strings.Builder
	allModes.WriteString("RESPONSE MODES (the active mode is specified in the user context block):\n\n")
	for _, mode := range []QueryType{Recall, Analysis, Decision, Synthesis} {
		allModes.WriteString(fmt.Sprintf("[%s]\n%s\n\n", strings.ToUpper(string(mode)), modeDirectives[mode]))
	}
	return fmt.Sprintf(`You are a knowledge-grounded reasoning assistant.

%s

%s

%s

%s

%s

%s

%s`,
		groundingRules,
		sourceProtocol,
		deepExtraction,
		crossSourceTension,
		synthesisRules,
		allModes.String(),
		hardBoundaries,
	)
}

// ContextBlock formats the per-turn dynamic context: which mode is active,
// a quality assessment, and the retrieved chunks. This gets injected into
// the user message (not the system prompt) so the system prompt stays
// stable across turns — a requirement for prompt caching to work.
func ContextBlock(chunks []retriever.Chunk, query string, modeOverride QueryType) string {
	mode := modeOverride
	if mode == "" {
		if query != "" {
			mode = Classify(query)
		} else {
			mode = Analysis
		}
	}
	quality := describeSourceQuality(chunks)
	return fmt.Sprintf(`[Active mode: %s]

%s

Context from personal knowledge base:
---
%s
---`, mode, quality, formatChunks(chunks))
}

// BuildSystemPrompt produces the combined system prompt for backends that
// don't support prompt caching (OpenAI-compat, Claude CLI). For the
// Anthropic SDK backend, use StaticDirectives() + ContextBlock() instead.
func BuildSystemPrompt(chunks []retriever.Chunk, query string, modeOverride QueryType) string {
	mode := modeOverride
	if mode == "" {
		if query != "" {
			mode = Classify(query)
		} else {
			mode = Analysis
		}
	}
	quality := describeSourceQuality(chunks)

	return fmt.Sprintf(`You are a knowledge-grounded reasoning assistant. %s

%s

%s

%s

%s

%s

%s

%s

Context from personal knowledge base:
---
%s
---`,
		quality,
		groundingRules,
		sourceProtocol,
		deepExtraction,
		crossSourceTension,
		synthesisRules,
		modeDirectives[mode],
		hardBoundaries,
		formatChunks(chunks),
	)
}

// BuildChallengePrompt produces the `/challenge` prompt that asks the model
// to cross-reference a prior answer against a different set of sources.
func BuildChallengePrompt(origQuestion, origAnswer string, origSources, challengeChunks []retriever.Chunk) string {
	sourceNames := make([]string, 0, len(origSources))
	for _, c := range origSources {
		sourceNames = append(sourceNames, c.DisplayPath)
	}
	origSourceList := strings.Join(sourceNames, ", ")

	answer := origAnswer
	if len(answer) > 3000 {
		answer = answer[:3000] + "\n[truncated]"
	}

	return fmt.Sprintf(`You are a knowledge-grounded reasoning assistant performing a CHALLENGE review.

A user previously asked a question and received an answer based on one set of sources.
You are now reviewing that answer using a DIFFERENT set of sources to cross-reference, challenge, validate, or add nuance.

ORIGINAL QUESTION:
%s

ORIGINAL ANSWER (from sources: %s):
%s

YOUR TASK:
1. VALIDATE: Which parts of the original answer are confirmed by these new sources? Cite the confirming sources.
2. CHALLENGE: Which parts are contradicted, incomplete, or presented differently in these sources? Cite the contradicting sources.
3. ADD NUANCE: What new perspectives, details, or frameworks do these sources add that the original answer missed?
4. SYNTHESIZE: Provide a refined, more complete answer that integrates insights from both source sets.

RULES:
- Every claim MUST cite its source using [filename.txt].
- Clearly distinguish what the new sources say vs. your synthesis.
- If the new sources have nothing relevant to add, say so explicitly.
- Be concise and direct. No filler.
- Respond in the same language the user writes in (Turkish or English).

New sources for challenge:
---
%s
---`, origQuestion, origSourceList, answer, formatChunks(challengeChunks))
}
