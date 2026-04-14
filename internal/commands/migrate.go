package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewMigrateCmd wires the Migrate helper into a Cobra command.
func NewMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Migration helper for users coming from the qmd-era (v0.2.x) brain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Migrate(cmd.Context())
		},
	}
}

// Migrate is an advisory helper, not an automated importer. brain's
// retrieval backend changed from qmd (external npm subprocess) to
// recall (in-process Go library) — the two stores have different
// schemas, chunkers, and embedders, so auto-imports risk wrong docids,
// stale hashes, and bad provenance. Instead we:
//
//   - detect the qmd era (either ~/.qmd/ on disk OR the qmd binary on
//     PATH)
//   - list the qmd collections the user had (when we can)
//   - print the exact `brain add` commands to run for each
//   - point at `npm uninstall -g @tobilu/qmd` once they're happy
//
// Honest > magic. The user stays in control of which paths carry over.
func Migrate(ctx context.Context) error {
	fmt.Println(ui.Bold.Render("brain migrate"))
	fmt.Println()

	qmdHome := qmdHomeDir()
	qmdBin, _ := exec.LookPath("qmd")

	haveQmdHome := qmdHome != ""
	haveQmdBin := qmdBin != ""

	if !haveQmdHome && !haveQmdBin {
		fmt.Println(ui.Green.Render("Nothing to migrate."))
		fmt.Println(ui.Dim.Render("  No ~/.qmd/ directory and no qmd binary on PATH — you're already clean."))
		return nil
	}

	fmt.Println(ui.Yellow.Render("  brain's retrieval layer has moved from qmd to recall."))
	fmt.Println(ui.Dim.Render("  qmd (npm) is no longer invoked at runtime; the new engine is an in-process"))
	fmt.Println(ui.Dim.Render("  Go library. That means your old ~/.qmd/ index isn't read anymore."))
	fmt.Println()

	// Advisory (not automatic) re-add guidance. When qmd is on PATH, we
	// shell out once to list collection names so the user knows what
	// they had; we intentionally do NOT try to parse qmd's SQLite for
	// paths — we ask the user instead.
	if haveQmdBin {
		fmt.Println(ui.Bold.Render("  Your qmd collections:"))
		if names, err := qmdCollectionNames(ctx, qmdBin); err == nil && len(names) > 0 {
			for _, n := range names {
				fmt.Println(ui.Dim.Render("    • " + n))
			}
		} else {
			fmt.Println(ui.Dim.Render("    (couldn't list them — run `qmd collection list` yourself to see the set)"))
		}
		fmt.Println()
	}

	fmt.Println(ui.Bold.Render("  To carry them over:"))
	fmt.Println(ui.Dim.Render("    1. For each collection, re-register its folder with brain:"))
	fmt.Println()
	fmt.Println(ui.Cyan.Render("       brain add <path> --name <name>"))
	fmt.Println()
	fmt.Println(ui.Dim.Render("    2. Optional but recommended — describe the collection so search quality"))
	fmt.Println(ui.Dim.Render("       on domain-specific content doesn't regress:"))
	fmt.Println()
	fmt.Println(ui.Cyan.Render("       brain add <path> --name <name> --context \"<what's in this folder>\""))
	fmt.Println()
	fmt.Println(ui.Dim.Render("    3. brain add auto-runs the index + embed pass, so you're queryable"))
	fmt.Println(ui.Dim.Render("       immediately after the last add."))
	fmt.Println()

	if haveQmdHome {
		fmt.Println(ui.Dim.Render(fmt.Sprintf("  Your old qmd store (%s) is left intact. Delete it when you're satisfied:", qmdHome)))
		fmt.Println(ui.Cyan.Render("    rm -rf " + qmdHome))
		fmt.Println()
	}
	if haveQmdBin {
		fmt.Println(ui.Dim.Render("  Once every collection is re-added and `brain ask` works, you can"))
		fmt.Println(ui.Dim.Render("  uninstall qmd (brain no longer invokes it):"))
		fmt.Println(ui.Cyan.Render("    npm uninstall -g @tobilu/qmd"))
		fmt.Println()
	}

	fmt.Println(ui.Green.Render("  Migration plan printed. Run `brain doctor` after re-adding your collections."))
	return nil
}

// qmdHomeDir returns the qmd-era data directory if it exists, or "".
// Honours $QMD_HOME first, falls back to ~/.qmd.
func qmdHomeDir() string {
	if v := os.Getenv("QMD_HOME"); v != "" {
		if info, err := os.Stat(v); err == nil && info.IsDir() {
			return v
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	def := filepath.Join(home, ".qmd")
	if info, err := os.Stat(def); err == nil && info.IsDir() {
		return def
	}
	return ""
}

// qmdCollectionNames best-effort lists collections from qmd's text output.
// Returns an empty slice (no error) when qmd produces nothing parseable —
// the caller handles that gracefully.
func qmdCollectionNames(ctx context.Context, bin string) ([]string, error) {
	cmd := exec.CommandContext(ctx, bin, "collection", "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// qmd's list format is "<name>  (qmd://...)" per line. We grab the
	// leading token up to the first whitespace, keeping anything that
	// looks like an identifier.
	var names []string
	for _, line := range splitLines(string(out)) {
		line = trimLeadingSpace(line)
		if line == "" {
			continue
		}
		end := firstWhitespace(line)
		if end <= 0 {
			continue
		}
		token := line[:end]
		// Skip obvious headers / banners qmd might print.
		if token == "Collections" || token == "No" || token == "Registered" {
			continue
		}
		names = append(names, token)
	}
	return names, nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimLeadingSpace(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return s[i:]
		}
	}
	return ""
}

func firstWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return i
		}
	}
	return -1
}
