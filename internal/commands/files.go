package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewFilesCmd wires the Files handler into a Cobra command with its flag.
func NewFilesCmd() *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "files",
		Short: "List all indexed files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Files(cmd.Context(), collection)
		},
	}
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Filter by collection")
	return cmd
}

// Files lists every indexed file, optionally scoped to a single
// collection. Uses recall.Engine.MultiGet with a matching glob so we
// only do one round-trip to the DB.
func Files(ctx context.Context, collection string) error {
	_ = ctx

	eng, err := OpenEngine()
	if err != nil {
		return err
	}
	defer eng.Close()

	pattern := "**"
	if collection != "" {
		pattern = collection + "/**"
	}

	docs, err := eng.Recall().MultiGet(pattern)
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}
	if len(docs) == 0 {
		fmt.Println(ui.Yellow.Render("No indexed files found."))
		fmt.Println(ui.Dim.Render("Run `brain add <path>` then `brain index` to get started."))
		return nil
	}

	for _, d := range docs {
		if d.CollectionName != "" {
			fmt.Printf("%s/%s\n", d.CollectionName, d.Path)
		} else {
			fmt.Println(d.Path)
		}
	}
	return nil
}
