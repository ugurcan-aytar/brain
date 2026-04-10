package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh/spinner"
	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// Index runs `qmd update` followed by `qmd embed`, showing a spinner for
// each step. Re-running is safe — qmd handles dedupe on its side.
func Index(ctx context.Context) error {
	fmt.Println(ui.Bold.Render("Indexing your knowledge base…"))
	fmt.Println()

	if ok, err := runStep(ctx, "Updating file index", []string{"update"}); err != nil {
		return err
	} else if !ok {
		return nil
	}

	if ok, err := runStep(ctx, "Generating embeddings", []string{"embed"}); err != nil {
		return err
	} else if !ok {
		return nil
	}

	fmt.Println()
	fmt.Println(ui.Green.Render("✓ Indexing complete. Your brain is up to date."))
	return nil
}

// runStep wraps a single qmd invocation in a spinner. Returns (success, error).
// A false success with nil error means "we printed the problem and the caller
// should just stop cleanly".
func runStep(ctx context.Context, label string, args []string) (bool, error) {
	var res qmdResult
	var runErr error
	action := func() {
		res, runErr = runQmd(ctx, args...)
	}
	if err := spinner.New().Title(label + "…").Action(action).Run(); err != nil {
		return false, err
	}

	if runErr != nil {
		if isMissing(runErr) {
			printQmdMissing()
			return false, nil
		}
		return false, runErr
	}

	if res.exitCode != 0 {
		fmt.Println(ui.Red.Render(label + " — failed"))
		fmt.Println(ui.Red.Render(config.RewriteQmdOutput(strings.TrimSpace(res.stderr))))
		return false, nil
	}

	fmt.Println(ui.Green.Render("✓ " + label))
	if stdout := strings.TrimSpace(res.stdout); stdout != "" {
		fmt.Println(ui.Dim.Render(config.RewriteQmdOutput(stdout)))
	}
	return true, nil
}
