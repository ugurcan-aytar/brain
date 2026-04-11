package llm

import "strings"

// Models maps short aliases used across the CLI (and in /model, -m flags)
// to the canonical Anthropic model IDs we pass to the API.
var Models = map[string]string{
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-6",
	"haiku":  "claude-haiku-4-5",
}

// ModelChoice is the metadata shown in interactive model pickers.
type ModelChoice struct {
	Alias       string
	ResolvedID  string
	Description string
}

var ModelChoices = []ModelChoice{
	{"sonnet", "claude-sonnet-4-6", "balanced, default"},
	{"opus", "claude-opus-4-6", "deepest reasoning"},
	{"haiku", "claude-haiku-4-5", "fastest"},
}

// ResolveModel maps an alias to its canonical Anthropic model ID. Unknown
// names pass through so users can supply full IDs directly.
func ResolveModel(name string) string {
	if name == "" {
		return Models["sonnet"]
	}
	if id, ok := Models[name]; ok {
		return id
	}
	return name
}

// IsValidModel returns true if the name is a known alias or a plausible
// raw Anthropic model ID.
func IsValidModel(name string) bool {
	if _, ok := Models[name]; ok {
		return true
	}
	return strings.HasPrefix(name, "claude-")
}

// Display returns the "alias (resolved)" form for status/prompt lines.
func Display(name string) string {
	if id, ok := Models[name]; ok {
		return name + " (" + id + ")"
	}
	return name
}
