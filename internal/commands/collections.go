package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// Collections lists every registered qmd collection.
func Collections(ctx context.Context) error {
	res, err := runQmd(ctx, "collection", "list")
	if err != nil {
		if isMissing(err) {
			printQmdMissing()
			return nil
		}
		return err
	}
	if res.exitCode != 0 {
		fmt.Println(ui.Red.Render("Failed to list collections: " + strings.TrimSpace(res.stderr)))
		return nil
	}

	output := strings.TrimSpace(res.stdout)
	if output == "" {
		fmt.Println(ui.Yellow.Render("No collections registered."))
		fmt.Println(ui.Dim.Render("Add one with: brain add <path>"))
		return nil
	}

	fmt.Println(ui.Bold.Render("Registered collections:"))
	fmt.Println()
	fmt.Println(config.RewriteQmdOutput(output))
	return nil
}
