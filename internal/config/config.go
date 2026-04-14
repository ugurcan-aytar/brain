// Package config holds brain's runtime defaults. It centralises the
// knobs callers tweak (model, token budget, TopK, min-score floor,
// grounding-gate threshold, default file mask).
package config

// Settings collects the runtime knobs brain consults on every ask /
// chat call. The zero value is NOT useful; callers should start from
// Default and override fields as needed.
type Settings struct {
	Model                string
	MaxTokens            int
	TopK                 int
	MinScore             float64
	MinChunksToCallLLM   int
	MaxConversationTurns int
	DefaultMask          string
}

// Default is the settings snapshot used by every command unless a flag
// or env var overrides a specific field. Tuned against the brain test
// corpus; see CHANGELOG entries for v0.2.5 (TopK 7→20) and v0.2.6
// (adaptive min-score) for the history behind the current values.
var Default = Settings{
	Model:                "claude-sonnet-4-6",
	MaxTokens:            16384,
	TopK:                 20,
	MinScore:             0.05,
	MinChunksToCallLLM:   1,
	MaxConversationTurns: 10,
	DefaultMask:          "**/*.{txt,md}",
}
