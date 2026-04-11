package config

import (
	"os"
	"strings"
)

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

var Default = Settings{
	Model:                "claude-sonnet-4-6",
	MaxTokens:            16384,
	TopK:                 7,
	MinScore:             0.2,
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
