package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	historypkg "github.com/ugurcan-aytar/brain/internal/history"
	"github.com/ugurcan-aytar/brain/internal/markdown"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewHistoryCmd builds the `brain history` command tree. Default action
// lists recent entries; subcommands handle search / view / rm / path.
func NewHistoryCmd() *cobra.Command {
	var listLimit int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Browse the Q&A history saved on disk",
		Long: `Browse the timestamped Q&A archive brain writes after every answer.

  brain history                 List recent entries (newest first)
  brain history search <query>  Find entries containing <query>
  brain history view <id>       Show the full markdown of an entry
  brain history rm <id>         Delete an entry
  brain history path            Print the history directory`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryList(listLimit)
		},
	}
	cmd.Flags().IntVarP(&listLimit, "limit", "n", 10, "Number of entries to show")

	cmd.AddCommand(newHistorySearchCmd())
	cmd.AddCommand(newHistoryViewCmd())
	cmd.AddCommand(newHistoryRmCmd())
	cmd.AddCommand(newHistoryPathCmd())
	return cmd
}

func newHistorySearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Find history entries containing <query>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistorySearch(args[0])
		},
	}
}

func newHistoryViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "Show the full markdown of an entry (id from `brain history`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryView(args[0])
		},
	}
}

func newHistoryRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a history entry (id from `brain history`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryRm(args[0])
		},
	}
}

func newHistoryPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the history directory path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(historypkg.Directory())
			return nil
		},
	}
}

func runHistoryList(limit int) error {
	recs, err := historypkg.List()
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		fmt.Println(ui.Dim.Render("  No history entries yet. Ask a question with `brain ask`."))
		fmt.Println(ui.Dim.Render("  History directory: " + historypkg.Directory()))
		return nil
	}
	if limit > 0 && len(recs) > limit {
		recs = recs[:limit]
	}
	renderHistoryTable(recs)
	fmt.Println()
	fmt.Println(ui.Dim.Render("  View an entry: brain history view <id>"))
	fmt.Println(ui.Dim.Render("  History dir:   " + historypkg.Directory()))
	return nil
}

func runHistorySearch(query string) error {
	matches, err := historypkg.Search(query)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Println(ui.Dim.Render("  No matches for: " + query))
		return nil
	}
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  %d match(es) for %q", len(matches), query)))
	fmt.Println()
	renderHistoryTable(matches)
	return nil
}

func runHistoryView(id string) error {
	rec, err := resolveHistoryID(id)
	if err != nil {
		return err
	}
	content, err := historypkg.Load(rec)
	if err != nil {
		return fmt.Errorf("load history entry: %w", err)
	}
	// Reuse the streaming markdown renderer so `brain history view` looks
	// the same as a live answer — headings, code, lists all colored.
	r := markdown.New()
	r.Write(content)
	r.Flush()
	fmt.Println()
	fmt.Println(ui.Dim.Render("  Path: " + rec.Path))
	return nil
}

func runHistoryRm(id string) error {
	rec, err := resolveHistoryID(id)
	if err != nil {
		return err
	}
	if err := historypkg.Delete(rec); err != nil {
		return fmt.Errorf("delete history entry: %w", err)
	}
	fmt.Println(ui.Dim.Render("  Deleted: " + rec.Filename))
	return nil
}

// resolveHistoryID accepts either a 1-based index (as shown by
// `brain history`) or a filename / filename prefix. Numeric wins when the
// input parses cleanly.
func resolveHistoryID(id string) (historypkg.Record, error) {
	recs, err := historypkg.List()
	if err != nil {
		return historypkg.Record{}, err
	}
	if len(recs) == 0 {
		return historypkg.Record{}, fmt.Errorf("no history entries")
	}

	if n, err := strconv.Atoi(id); err == nil {
		if n < 1 || n > len(recs) {
			return historypkg.Record{}, fmt.Errorf("id %d out of range (have %d entries)", n, len(recs))
		}
		return recs[n-1], nil
	}

	// Filename match: exact, then suffix/prefix.
	for _, rec := range recs {
		if rec.Filename == id || strings.TrimSuffix(rec.Filename, ".md") == id {
			return rec, nil
		}
	}
	var partial []historypkg.Record
	for _, rec := range recs {
		if strings.Contains(rec.Filename, id) {
			partial = append(partial, rec)
		}
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	if len(partial) > 1 {
		return historypkg.Record{}, fmt.Errorf("ambiguous id %q: %d matches", id, len(partial))
	}
	return historypkg.Record{}, fmt.Errorf("no history entry matches %q", id)
}

// renderHistoryTable prints a compact numbered list. Columns are chosen to
// fit comfortably in an 80-col terminal while still showing the model /
// collections metadata when present.
func renderHistoryTable(recs []historypkg.Record) {
	width := terminalWidth()
	for i, rec := range recs {
		idx := fmt.Sprintf("%3d", i+1)
		date := rec.Timestamp.Format("2006-01-02 15:04")
		question := rec.Question
		// Reserve room for prefix + date + padding.
		maxQ := width - 30
		if maxQ < 20 {
			maxQ = 20
		}
		if len(question) > maxQ {
			question = question[:maxQ-1] + "…"
		}
		fmt.Printf("  %s  %s  %s\n",
			ui.Cyan.Render(idx),
			ui.Dim.Render(date),
			question,
		)
		var meta []string
		if rec.Mode != "" {
			meta = append(meta, rec.Mode)
		}
		if rec.Model != "" {
			meta = append(meta, rec.Model)
		}
		if rec.Thinking != "" && rec.Thinking != rec.Mode {
			meta = append(meta, rec.Thinking)
		}
		if rec.Collections != "" {
			meta = append(meta, "["+rec.Collections+"]")
		}
		if rec.Elapsed != "" {
			meta = append(meta, rec.Elapsed)
		}
		if len(meta) > 0 {
			fmt.Println(ui.Dim.Render("       " + strings.Join(meta, "  ")))
		}
	}
}

// terminalWidth returns the current stdout column count, or 80 when the
// terminal size can't be determined (non-TTY, CI, etc.).
func terminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}
