package commands

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/retriever"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// SearchOptions are the flags for `brain search`. Mirror the ask
// enhancements so users can eyeball the exact candidate set brain
// would feed the LLM on an equivalent `brain ask`.
type SearchOptions struct {
	Collection string
	TopK       int
	Expand     bool
	Rerank     bool
	Hyde       bool
	Explain    bool
}

// NewSearchCmd wires the Search handler into a Cobra command with its flags.
func NewSearchCmd() *cobra.Command {
	var opts SearchOptions
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search your notes without LLM (raw retrieval results)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Search(cmd.Context(), args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Collection, "collection", "c", "", "Scope search to a specific collection")
	cmd.Flags().IntVarP(&opts.TopK, "top", "n", 10, "Number of results")
	cmd.Flags().BoolVar(&opts.Expand, "expand", false, "Query expansion — lex/vec variants for recall")
	cmd.Flags().BoolVar(&opts.Rerank, "rerank", false, "Cross-encoder rerank the top candidates")
	cmd.Flags().BoolVar(&opts.Hyde, "hyde", false, "HyDE — embed LLM-generated hypothetical answers as extra vector probes")
	cmd.Flags().BoolVar(&opts.Explain, "explain", false, "Annotate retrieved chunks with a score trace")
	return cmd
}

// Search runs retrieval (BM25 baseline; hybrid+expand/rerank/hyde
// when any of those flags is set) and prints the results with
// confidence bars — useful for verifying indexing and tuning flags
// before you burn tokens on `ask`.
func Search(parent context.Context, query string, opts SearchOptions) error {
	ctx, stopSignal := withSignalCancel(parent)
	defer stopSignal()

	eng, err := engine.Open()
	if err != nil {
		return err
	}
	defer eng.Close()

	var (
		results []retriever.Chunk
		runErr  error
	)
	action := func() {
		if opts.Expand || opts.Rerank || opts.Hyde {
			// Enhanced path — returns scored, adaptively-filtered,
			// rerank-blended results. Matches `brain ask`'s candidate
			// set so users can tune flags against a concrete output.
			results, runErr = retriever.Retrieve(ctx, eng, query, retriever.Options{
				Collection: opts.Collection,
				TopK:       opts.TopK,
				Expand:     opts.Expand,
				Rerank:     opts.Rerank,
				Hyde:       opts.Hyde,
				Explain:    opts.Explain,
			})
			return
		}
		// Baseline: raw BM25, no min-score floor, no subprocess
		// boot. Keeps `brain search "<term>"` snappy and free.
		results, runErr = retriever.RawSearch(ctx, eng, query, retriever.Options{
			Collection: opts.Collection,
			TopK:       opts.TopK,
		})
	}
	if spinErr := spinner.New().Title("Searching…").Action(action).Run(); spinErr != nil {
		return spinErr
	}
	if runErr != nil {
		return runErr
	}

	if len(results) == 0 {
		fmt.Println(ui.Yellow.Render("No results found."))
		fmt.Println(ui.Dim.Render("Try different keywords, or run `brain index` to re-index."))
		return nil
	}

	fmt.Println()
	fmt.Println(ui.Bold.Render(fmt.Sprintf("%d result(s) found:", len(results))))
	fmt.Println()
	for i, r := range results {
		ui.PrintSearchResult(r, i)
		if opts.Explain && r.Explain != "" {
			fmt.Println(ui.Dim.Render("     trace: " + r.Explain))
		}
	}
	PrintUpdateBanner()
	return nil
}
