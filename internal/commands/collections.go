package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewCollectionsCmd wires the Collections handler into a Cobra command.
func NewCollectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "collections",
		Short: "List all registered collections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Collections(cmd.Context())
		},
	}
}

// Collections prints every registered recall collection plus its doc
// count and (when set) its context blurb.
func Collections(ctx context.Context) error {
	_ = ctx

	eng, err := OpenEngine()
	if err != nil {
		return err
	}
	defer eng.Close()

	cols, err := eng.Recall().ListCollections()
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	if len(cols) == 0 {
		fmt.Println(ui.Yellow.Render("No collections registered."))
		fmt.Println(ui.Dim.Render("Add one with: brain add <path>"))
		return nil
	}

	fmt.Println(ui.Bold.Render("Registered collections:"))
	fmt.Println()
	for _, c := range cols {
		fmt.Printf("  %s\n", c.Name)
		fmt.Println(ui.Dim.Render(fmt.Sprintf("    %s", c.Path)))
		if c.Context != "" {
			fmt.Println(ui.Dim.Render("    Context: " + c.Context))
		}
	}
	return nil
}
