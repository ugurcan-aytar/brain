package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/retriever"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewSearchCmd wires the Search handler into a Cobra command with its flags.
func NewSearchCmd() *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search your notes without LLM (raw retrieval results)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Search(cmd.Context(), args[0], collection)
		},
	}
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Scope search to a specific collection")
	return cmd
}

// Search runs a raw retrieval (no LLM) and prints the top results with
// confidence bars — useful for verifying indexing is behaving before
// you burn tokens on `ask`.
func Search(parent context.Context, query string, collection string) error {
	ctx, stopSignal := withSignalCancel(parent)
	defer stopSignal()

	var (
		results []retriever.Chunk
		err     error
	)

	action := func() {
		results, err = retriever.RawSearch(ctx, query, retriever.Options{
			Collection: collection,
			TopK:       10,
		})
	}
	if spinErr := spinner.New().Title("Searching…").Action(action).Run(); spinErr != nil {
		return spinErr
	}

	if err != nil {
		if errors.Is(err, retriever.ErrQmdMissing) {
			printQmdMissing()
			return nil
		}
		return err
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
	}
	return nil
}
