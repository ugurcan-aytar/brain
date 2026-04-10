package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// Status runs `qmd status` and appends the brain-specific config block so
// users can see the model, token limits, and retrieval thresholds in one
// place.
func Status(ctx context.Context) error {
	res, err := runQmd(ctx, "status")
	if err != nil {
		if isMissing(err) {
			printQmdMissing()
			return nil
		}
		return err
	}
	if res.exitCode != 0 {
		fmt.Println(ui.Red.Render("qmd status failed: " + strings.TrimSpace(res.stderr)))
		return nil
	}

	fmt.Println(config.RewriteQmdOutput(strings.TrimSpace(res.stdout)))
	fmt.Println()
	fmt.Println(ui.Dim.Render("── Brain Config ─────────────────────"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Model:      %s", config.Default.Model)))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Max Tokens: %d", config.Default.MaxTokens)))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Top-K:      %d", config.Default.TopK)))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Min Score:  %g", config.Default.MinScore)))
	return nil
}
