package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/llm"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewDoctorCmd wires the Doctor handler into a Cobra command.
func NewDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check required dependencies and configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Doctor(cmd.Context())
		},
	}
}

// Doctor checks that every external dependency brain relies on is in place:
// the qmd retrieval engine, plus at least one LLM backend from the three
// brain supports (Anthropic API, OpenAI-compatible, or the Claude CLI).
// Prints a human-readable report and exits non-zero if anything required is
// missing so CI and install scripts can gate on it.
func Doctor(ctx context.Context) error {
	fmt.Println(ui.Bold.Render("brain doctor"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  %s/%s, Go runtime %s", runtime.GOOS, runtime.GOARCH, runtime.Version())))
	fmt.Println()

	var failures int

	// qmd is required. Without it, retrieval cannot run and every command path
	// that touches the index will fail.
	if qmdPath, err := exec.LookPath(config.Default.QmdBinary); err == nil {
		version := qmdVersion(ctx)
		ok(fmt.Sprintf("search engine found at %s%s", qmdPath, version))
	} else {
		fail("search engine (qmd) not found in PATH")
		hint("Install: npm install -g @tobilu/qmd")
		failures++
	}

	// LLM backends: report each slot independently, then show which one is
	// actually active based on brain's priority order.
	active := llm.Select()
	reportBackend := func(b llm.Backend, msg string) {
		if b == active {
			ok(msg + ui.Green.Render("  ← active"))
		} else {
			ok(msg)
		}
	}

	hasAnthropic := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")) != ""
	if hasAnthropic {
		reportBackend(llm.BackendAnthropicAPI, "ANTHROPIC_API_KEY set")
	}

	hasOpenAI := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
	if hasOpenAI {
		base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
		if model == "" {
			model = "gpt-4o (default)"
		}
		reportBackend(llm.BackendOpenAI, fmt.Sprintf("OPENAI_API_KEY set (%s, model=%s)", base, model))
	}

	claudeBin := "claude"
	if override := strings.TrimSpace(os.Getenv("BRAIN_CLAUDE_BIN")); override != "" {
		claudeBin = override
	}
	if claudePath, err := exec.LookPath(claudeBin); err == nil {
		reportBackend(llm.BackendClaudeCLI, fmt.Sprintf("%s CLI found at %s", claudeBin, claudePath))
	} else if !hasAnthropic && !hasOpenAI {
		fail("no LLM backend configured")
		fmt.Println()
		hint("Add one of the following to your shell profile (~/.zshrc or ~/.bashrc):")
		fmt.Println()
		hint("  # Option 1: Anthropic API (recommended, best quality)")
		hint("  # Get a key at https://console.anthropic.com/settings/keys")
		hint("  export ANTHROPIC_API_KEY=sk-ant-...")
		fmt.Println()
		hint("  # Option 2: OpenAI")
		hint("  export OPENAI_API_KEY=sk-...")
		fmt.Println()
		hint("  # Option 2b: Ollama (local, free, offline-capable)")
		hint("  # Install Ollama first: https://ollama.com")
		hint("  export OPENAI_API_KEY=ollama")
		hint("  export OPENAI_BASE_URL=http://localhost:11434/v1")
		hint("  export OPENAI_MODEL=llama3.1")
		fmt.Println()
		hint("  # Option 2c: OpenRouter (one key, every model)")
		hint("  export OPENAI_API_KEY=sk-or-...")
		hint("  export OPENAI_BASE_URL=https://openrouter.ai/api/v1")
		fmt.Println()
		hint("  # Option 3: Claude Code CLI (uses your Claude subscription)")
		hint("  # Install: https://claude.ai/download")
		hint("  # Then:    claude login")
		fmt.Println()
		hint("After adding, restart your terminal and re-run `brain doctor`.")
		failures++
	}

	// Collection context: warn if no context is set on any collection.
	if _, err := exec.LookPath(config.Default.QmdBinary); err == nil {
		checkQmdContext(ctx)
	}

	// qmd pipeline health: run a tiny vector search to confirm vec0 + embeddings work.
	// This catches the "vec0 module missing" crash that silently degrades brain
	// to BM25-only without telling the user.
	if _, err := exec.LookPath(config.Default.QmdBinary); err == nil {
		checkQmdPipeline(ctx)
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

func checkQmdContext(ctx context.Context) {
	res, err := runQmd(ctx, "context", "list")
	if err != nil || res.exitCode != 0 {
		return
	}
	stdout := strings.TrimSpace(res.stdout)
	if stdout == "" || strings.Contains(stdout, "No context") || stdout == "[]" {
		warn("no collection context set — search quality may be degraded")
		hint("Add context to help brain understand your collections:")
		hint("  brain add <path> --context \"description of what these notes contain\"")
	} else {
		ok("collection context configured")
	}
}

func checkQmdPipeline(ctx context.Context) {
	// Quick probe: run `qmd search "test" -n 1` (BM25 only, no vec0 needed)
	// to confirm basic search works.
	res, err := runQmd(ctx, "search", "test", "-n", "1", "--json")
	if err != nil || res.exitCode != 0 {
		warn("search probe failed — index may be empty or corrupted")
		hint("Try: brain index")
		return
	}
	ok("keyword search working")

	// Now probe vector search — this exercises the vec0 SQLite extension.
	vres, verr := runQmd(ctx, "vsearch", "test", "-n", "1", "--json")
	if verr != nil {
		warn("vector search probe failed — search engine may need reinstalling")
		hint("Try: npm install -g @tobilu/qmd && brain index")
		return
	}
	if vres.exitCode != 0 {
		stderr := strings.TrimSpace(vres.stderr)
		if strings.Contains(stderr, "vec0") || strings.Contains(stderr, "no such module") {
			warn("vector search broken — only keyword match is working")
			hint("This significantly degrades retrieval quality.")
			hint("Fix: npm install -g @tobilu/qmd && brain index")
		} else {
			warn("vector search returned an error")
			if stderr != "" {
				hint(stderr)
			}
		}
		return
	}
	ok("vector search working")
}

func ok(msg string)   { fmt.Println(ui.Green.Render("  ✓ ") + msg) }
func fail(msg string) { fmt.Println(ui.Red.Render("  ✗ ") + msg) }
func warn(msg string) { fmt.Println(ui.Yellow.Render("  ! ") + msg) }
func hint(msg string) { fmt.Println(ui.Dim.Render("      " + msg)) }
