package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/recall/pkg/recall"

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

// Doctor prints a health report: recall index state (DB reachable,
// collection + embedding counts, model available), at least one LLM
// backend configured, and any BRAIN_HISTORY_DIR override. Exits
// non-zero when something required is missing so install scripts / CI
// can gate on it.
func Doctor(ctx context.Context) error {
	_ = ctx

	fmt.Println(ui.Bold.Render("brain doctor"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  %s/%s, Go runtime %s", runtime.GOOS, runtime.GOARCH, runtime.Version())))
	fmt.Println()

	var failures int

	// ── recall index ──────────────────────────────────────────────────
	eng, engErr := OpenEngine()
	if engErr != nil {
		fail("recall index not reachable")
		hint(engErr.Error())
		hint("Try: brain add <path> to create the index, or check RECALL_DB_PATH.")
		failures++
	} else {
		defer eng.Close()
		cols, err := eng.Recall().ListCollections()
		if err != nil {
			fail("recall index open but unreadable")
			hint(err.Error())
			failures++
		} else {
			ok(fmt.Sprintf("recall index reachable (%d collection(s))", len(cols)))
			if len(cols) == 0 {
				hint("Add your first collection: brain add <path>")
			}
		}

		// Embedder health — no failure here, only info. Brain works on
		// BM25 alone; a missing embedder just means hybrid degrades.
		emb, embErr := eng.Embedder()
		switch {
		case embErr != nil:
			warn("embedder configuration error: " + embErr.Error())
			hint("Hybrid / vector search disabled until resolved.")
		case emb == nil:
			if recall.LocalEmbedderAvailable() {
				warn("local GGUF backend compiled in, but model not available")
				hint("Run: recall models download     (or rebuild brain with embed_llama and the patched gollama)")
			} else {
				warn("hybrid search uses keyword (BM25) only on this build")
				hint("Set RECALL_EMBED_PROVIDER=openai|voyage for vector search, or build from source with -tags embed_llama.")
			}
		default:
			ok(fmt.Sprintf("embedder: %s (%d dims)", emb.ModelName(), emb.Dimensions()))
		}
	}

	// ── LLM backends ──────────────────────────────────────────────────
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
		hint("  export OPENAI_API_KEY=ollama")
		hint("  export OPENAI_BASE_URL=http://localhost:11434/v1")
		hint("  export OPENAI_MODEL=llama3.1")
		fmt.Println()
		hint("  # Option 3: Claude Code CLI (uses your Claude subscription)")
		hint("  # Install: https://claude.ai/download")
		hint("  # Then:    claude login")
		fmt.Println()
		hint("After adding, restart your terminal and re-run `brain doctor`.")
		failures++
	}

	// ── History directory (best-effort) ───────────────────────────────
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

func ok(msg string)   { fmt.Println(ui.Green.Render("  ✓ ") + msg) }
func fail(msg string) { fmt.Println(ui.Red.Render("  ✗ ") + msg) }
func warn(msg string) { fmt.Println(ui.Yellow.Render("  ! ") + msg) }
func hint(msg string) { fmt.Println(ui.Dim.Render("      " + msg)) }
