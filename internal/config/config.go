// Package config holds brain's runtime defaults and environment helpers.
// It centralises the knobs that callers tweak (model, token budget, TopK,
// min-score floor, grounding-gate threshold, default file mask) and owns
// the qmd-subprocess environment scrubber and CLI output rewriter so the
// underlying retrieval tool stays invisible to end users.
package config

import (
	"os"
	"strings"
)

// Settings collects the runtime knobs brain consults on every ask / chat
// call. The zero value is NOT useful; callers should start from Default
// and override fields as needed.
type Settings struct {
	Model                string
	MaxTokens            int
	TopK                 int
	MinScore             float64
	MinChunksToCallLLM   int
	MaxConversationTurns int
	QmdBinary            string
	DefaultMask          string
}

// Default is the settings snapshot used by every command unless a flag or
// env var overrides a specific field. Tuned against the brain test corpus;
// see CHANGELOG entries for v0.2.5 (TopK 7→20) and v0.2.6 (adaptive
// min-score) for the history behind the current values.
var Default = Settings{
	Model:                "claude-sonnet-4-6",
	MaxTokens:            16384,
	TopK:                 20,
	MinScore:             0.05,
	MinChunksToCallLLM:   1,
	MaxConversationTurns: 10,
	QmdBinary:            "qmd",
	DefaultMask:          "**/*.{txt,md}",
}

// QmdEnv returns a clean environment for qmd subprocesses. The Bun-era parent
// used to unset BUN_INSTALL here to force qmd onto Node; we no longer run under
// Bun but the cleanup is harmless and keeps behavior identical for users who
// still have the var exported.
func QmdEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "BUN_INSTALL=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// RewriteQmdOutput replaces qmd CLI references in qmd's own output with brain
// equivalents, so users never see the underlying tool name.
func RewriteQmdOutput(text string) string {
	replacements := [][2]string{
		{"qmd collection add .", "brain add <path>"},
		{"qmd collection add", "brain add"},
		{"qmd collection remove", "brain remove"},
		{"qmd collection list", "brain collections"},
		{"qmd update", "brain index"},
		{"qmd embed", "brain index"},
		{"'qmd embed' to update embeddings", "'brain index' to update embeddings"},
		{"Run 'qmd embed'", "Run 'brain index'"},
		{"qmd status", "brain status"},
		{"qmd ls", "brain files"},
	}
	for _, pair := range replacements {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}
	return text
}
