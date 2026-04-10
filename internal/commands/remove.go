package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh/spinner"
	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// Remove unregisters a collection and re-runs qmd update so the embeddings
// for the removed files drop out of the index. We intentionally do NOT rerun
// `embed` here — dropping rows doesn't require re-embedding anything.
func Remove(ctx context.Context, name string) error {
	var removeRes qmdResult
	var removeErr error
	removeAction := func() {
		removeRes, removeErr = runQmd(ctx, "collection", "remove", name)
	}
	if err := spinner.New().Title(fmt.Sprintf("Removing collection %q…", name)).Action(removeAction).Run(); err != nil {
		return err
	}

	if removeErr != nil {
		if isMissing(removeErr) {
			printQmdMissing()
			return nil
		}
		return removeErr
	}
	if removeRes.exitCode != 0 {
		fmt.Println(ui.Red.Render(fmt.Sprintf("Failed to remove %q", name)))
		fmt.Println(ui.Red.Render(config.RewriteQmdOutput(strings.TrimSpace(removeRes.stderr))))
		return nil
	}

	fmt.Println(ui.Green.Render(fmt.Sprintf("✓ Removed collection %q", name)))
	if stdout := strings.TrimSpace(removeRes.stdout); stdout != "" {
		fmt.Println(ui.Dim.Render(config.RewriteQmdOutput(stdout)))
	}

	var updateRes qmdResult
	var updateErr error
	updateAction := func() {
		updateRes, updateErr = runQmd(ctx, "update")
	}
	if err := spinner.New().Title("Cleaning up index…").Action(updateAction).Run(); err != nil {
		return err
	}
	if updateErr != nil || updateRes.exitCode != 0 {
		fmt.Println(ui.Red.Render("Index cleanup failed"))
		return nil
	}
	fmt.Println(ui.Green.Render("✓ Index updated"))

	fmt.Println()
	fmt.Println(ui.Green.Render(fmt.Sprintf("Done. Collection %q fully removed.", name)))
	return nil
}
