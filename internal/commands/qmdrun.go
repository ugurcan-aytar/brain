package commands

// Shared helpers for commands that shell out to qmd. Centralizing error
// messaging here means "qmd is not installed" prints identically everywhere
// and we never leak the bare exec error path to the user.

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

type qmdResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runQmd(ctx context.Context, args ...string) (qmdResult, error) {
	cmd := exec.CommandContext(ctx, config.Default.QmdBinary, args...)
	cmd.Env = config.QmdEnv()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := qmdResult{stdout: stdout.String(), stderr: stderr.String()}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.exitCode = exitErr.ExitCode()
			return res, nil // non-zero exit is not a Go error; callers inspect exitCode
		}
		var notFound *exec.Error
		if errors.As(err, &notFound) {
			return res, errQmdMissing
		}
		return res, err
	}
	return res, nil
}

var errQmdMissing = errors.New("qmd is not installed or not found in PATH")

// printQmdMissing renders the standard "please install qmd" message. All
// command entry points call this from their err path, so the install hint
// only lives in one place.
func printQmdMissing() {
	fmt.Println(ui.Red.Render("Error: search engine is not installed or not found in PATH."))
	fmt.Println(ui.Dim.Render("Install: npm install -g @tobilu/qmd"))
	fmt.Println(ui.Dim.Render("Then run: brain doctor"))
}

// isMissing checks whether an error is our sentinel for "qmd not in PATH".
func isMissing(err error) bool { return errors.Is(err, errQmdMissing) }

// printNoBackend renders the "no LLM backend configured" guidance. Called
// from Ask/Chat before they kick off the retrieval pipeline, so users get
// actionable next steps instead of a bare "no backend" error after the
// picker has already run.
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
