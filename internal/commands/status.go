package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewStatusCmd wires the Status handler into a Cobra command.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show index status and brain config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Status(cmd.Context())
		},
	}
}

// Status shows recall-powered index stats (collection count, document
// count, embedding count) followed by the brain-specific config block.
func Status(ctx context.Context) error {
	_ = ctx

	eng, err := engine.Open()
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer eng.Close()

	cols, err := eng.Recall().ListCollections()
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	docs, err := eng.Recall().MultiGet("**")
	if err != nil {
		return fmt.Errorf("count documents: %w", err)
	}
	embCount, err := eng.Recall().Store().EmbeddingCount()
	if err != nil {
		return fmt.Errorf("count embeddings: %w", err)
	}

	fmt.Println(ui.Bold.Render("recall index"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Collections: %d", len(cols))))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Documents:   %d", len(docs))))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Embeddings:  %d", embCount)))
	if len(cols) > 0 {
		fmt.Println()
		for _, c := range cols {
			fmt.Println(ui.Dim.Render(fmt.Sprintf("  - %-20s %s", c.Name, c.Path)))
		}
	}

	fmt.Println()
	fmt.Println(ui.Dim.Render("── Brain Config ─────────────────────"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Model:      %s", config.Default.Model)))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Max Tokens: %d", config.Default.MaxTokens)))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Top-K:      %d", config.Default.TopK)))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Min Score:  %g", config.Default.MinScore)))
	return nil
}
