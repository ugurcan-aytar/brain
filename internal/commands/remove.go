package commands

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewRemoveCmd wires the Remove handler into a Cobra command.
func NewRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a collection and clean up its index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Remove(cmd.Context(), args[0])
		},
	}
}

// Remove unregisters a collection. recall's RemoveCollection cascades
// to documents, chunks, and embeddings via foreign-key ON DELETE, so
// there's no follow-up index run to execute.
func Remove(ctx context.Context, name string) error {
	_ = ctx

	eng, err := engine.Open()
	if err != nil {
		return err
	}
	defer eng.Close()

	var removeErr error
	action := func() {
		removeErr = eng.Recall().RemoveCollection(name)
	}
	if err := spinner.New().Title(fmt.Sprintf("Removing collection %q…", name)).Action(action).Run(); err != nil {
		return err
	}

	if removeErr != nil {
		fmt.Println(ui.Red.Render(fmt.Sprintf("Failed to remove %q: %s", name, removeErr)))
		return nil
	}

	fmt.Println(ui.Green.Render(fmt.Sprintf("✓ Removed collection %q", name)))
	fmt.Println(ui.Dim.Render("  Documents, chunks, and embeddings cascaded automatically."))
	return nil
}
