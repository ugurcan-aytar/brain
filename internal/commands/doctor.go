package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// Doctor checks that every external dependency brain relies on is in place:
// the qmd binary (retrieval engine), and at least one LLM backend — either
// ANTHROPIC_API_KEY or the `claude` CLI. It prints a human-readable report
// and exits non-zero if anything required is missing, so CI and install
// scripts can gate on it.
func Doctor(ctx context.Context) error {
	fmt.Println(ui.Bold.Render("brain doctor"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  %s/%s, Go runtime %s", runtime.GOOS, runtime.GOARCH, runtime.Version())))
	fmt.Println()

	var failures int

	// qmd is required. Without it, retrieval cannot run and every command path
	// that touches the index will fail.
	if qmdPath, err := exec.LookPath(config.Default.QmdBinary); err == nil {
		version := qmdVersion(ctx)
		ok(fmt.Sprintf("qmd found at %s%s", qmdPath, version))
	} else {
		fail("qmd not found in PATH")
		hint("Install with: npm install -g @tobilu/qmd")
		failures++
	}

	// LLM backend: either the API key or the claude CLI is enough. Both is
	// fine too — the API key takes priority at runtime.
	hasKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
	claudePath, claudeErr := exec.LookPath("claude")
	hasClaude := claudeErr == nil

	switch {
	case hasKey && hasClaude:
		ok("ANTHROPIC_API_KEY set (will take priority)")
		ok(fmt.Sprintf("claude CLI found at %s (fallback)", claudePath))
	case hasKey:
		ok("ANTHROPIC_API_KEY set")
	case hasClaude:
		ok(fmt.Sprintf("claude CLI found at %s", claudePath))
	default:
		fail("no LLM backend configured")
		hint("Set ANTHROPIC_API_KEY, or install the Claude Code CLI from https://claude.ai/download")
		failures++
	}

	// History directory is best-effort — warn if the override points somewhere
	// we can't write, but don't count it as a failure.
	if dir := os.Getenv("BRAIN_HISTORY_DIR"); dir != "" {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			warn(fmt.Sprintf("BRAIN_HISTORY_DIR=%s is not a writable directory", dir))
		} else {
			ok(fmt.Sprintf("BRAIN_HISTORY_DIR=%s", dir))
		}
	}

	fmt.Println()
	if failures > 0 {
		fmt.Println(ui.Red.Render(fmt.Sprintf("%d check(s) failed.", failures)))
		return fmt.Errorf("doctor: %d check(s) failed", failures)
	}
	fmt.Println(ui.Green.Render("All checks passed. You're ready to run `brain add <path>`."))
	return nil
}

func qmdVersion(ctx context.Context) string {
	res, err := runQmd(ctx, "--version")
	if err != nil || res.exitCode != 0 {
		return ""
	}
	v := strings.TrimSpace(res.stdout)
	if v == "" {
		return ""
	}
	return " (" + v + ")"
}

func ok(msg string)   { fmt.Println(ui.Green.Render("  ✓ ") + msg) }
func fail(msg string) { fmt.Println(ui.Red.Render("  ✗ ") + msg) }
func warn(msg string) { fmt.Println(ui.Yellow.Render("  ! ") + msg) }
func hint(msg string) { fmt.Println(ui.Dim.Render("      " + msg)) }
