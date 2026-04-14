package commands

// Shared CLI message for "no LLM backend configured". Used by ask / chat
// at entry so users get actionable setup guidance instead of hitting a
// bare error after the collection picker has already run.

import (
	"fmt"

	"github.com/ugurcan-aytar/brain/internal/ui"
)

// printNoBackend renders the "no LLM backend configured" guidance.
func printNoBackend() {
	fmt.Println(ui.Red.Render("Error: no LLM backend is configured."))
	fmt.Println()
	fmt.Println(ui.Bold.Render("  Pick one of the options below:"))
	fmt.Println()
	fmt.Println(ui.Cyan.Render("  Option 1 — Anthropic API key (recommended)"))
	fmt.Println(ui.Dim.Render("  Get a key at https://console.anthropic.com/settings/keys"))
	fmt.Println(ui.Dim.Render("  Then add to your shell profile (~/.zshrc or ~/.bashrc):"))
	fmt.Println()
	fmt.Println(ui.Dim.Render("    export ANTHROPIC_API_KEY=sk-ant-..."))
	fmt.Println()
	fmt.Println(ui.Cyan.Render("  Option 2 — OpenAI or any compatible provider"))
	fmt.Println(ui.Dim.Render("  Works with OpenAI, Ollama (local/free), OpenRouter, LM Studio, Groq, etc."))
	fmt.Println(ui.Dim.Render("  Add to your shell profile:"))
	fmt.Println()
	fmt.Println(ui.Dim.Render("    # OpenAI:"))
	fmt.Println(ui.Dim.Render("    export OPENAI_API_KEY=sk-..."))
	fmt.Println()
	fmt.Println(ui.Dim.Render("    # Ollama (local, free, no key needed):"))
	fmt.Println(ui.Dim.Render("    export OPENAI_API_KEY=ollama"))
	fmt.Println(ui.Dim.Render("    export OPENAI_BASE_URL=http://localhost:11434/v1"))
	fmt.Println()
	fmt.Println(ui.Cyan.Render("  Option 3 — Claude Code CLI (no API key needed)"))
	fmt.Println(ui.Dim.Render("  If you have a Claude subscription:"))
	fmt.Println(ui.Dim.Render("    1. Install: https://claude.ai/download"))
	fmt.Println(ui.Dim.Render("    2. Sign in: claude login"))
	fmt.Println()
	fmt.Println(ui.Dim.Render("  After setting a key, restart your terminal and run `brain doctor` to verify."))
}
