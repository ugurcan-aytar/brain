package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// AddOptions controls how a collection is registered.
type AddOptions struct {
	Name string
	Mask string
}

// Add validates the path exists, registers it as a collection with qmd,
// and then kicks off an index+embed pass so the new content is immediately
// queryable.
func Add(ctx context.Context, path string, opts AddOptions) error {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		fmt.Println(ui.Red.Render("Path not found or not a directory: " + resolved))
		fmt.Println(ui.Dim.Render("Make sure the directory exists before adding it."))
		return nil
	}

	name := opts.Name
	if name == "" {
		name = filepath.Base(resolved)
	}
	mask := opts.Mask
	if mask == "" {
		mask = config.Default.DefaultMask
	}

	res, err := runQmd(ctx, "collection", "add", resolved, "--name", name, "--mask", mask)
	if err != nil {
		if isMissing(err) {
			printQmdMissing()
			return nil
		}
		return err
	}
	if res.exitCode != 0 {
		fmt.Println(ui.Red.Render("Failed to add collection: " + config.RewriteQmdOutput(strings.TrimSpace(res.stderr))))
		return nil
	}

	if stdout := strings.TrimSpace(res.stdout); stdout != "" {
		fmt.Println(config.RewriteQmdOutput(stdout))
	}

	fmt.Println(ui.Green.Render(fmt.Sprintf("✓ Collection \"%s\" added from %s", name, path)))
	fmt.Println(ui.Dim.Render("  Mask: " + mask))
	fmt.Println()

	return Index(ctx)
}
