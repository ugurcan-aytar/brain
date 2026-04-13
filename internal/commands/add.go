package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// AddOptions controls how a collection is registered.
type AddOptions struct {
	Name    string
	Mask    string
	Context string
}

// NewAddCmd wires the Add handler into a Cobra command with its flags.
func NewAddCmd() *cobra.Command {
	var opts AddOptions
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a new collection of notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Add(cmd.Context(), args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.Name, "name", "", "Collection name (default: folder basename)")
	cmd.Flags().StringVar(&opts.Mask, "mask", "", "File glob mask (default: **/*.{txt,md})")
	cmd.Flags().StringVar(&opts.Context, "context", "", "Description of what this collection contains (improves search quality)")
	return cmd
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

	// Set collection context if provided — this tells qmd's HyDE and
	// reranker what the collection is about, dramatically improving
	// search quality for domain-specific content.
	if opts.Context != "" {
		qmdPath := fmt.Sprintf("qmd://%s/", name)
		cres, cerr := runQmd(ctx, "context", "add", qmdPath, opts.Context)
		if cerr == nil && cres.exitCode == 0 {
			fmt.Println(ui.Dim.Render("  Context: " + opts.Context))
		}
	} else {
		fmt.Println(ui.Dim.Render("  Tip: add --context \"description\" to improve search quality"))
	}
	fmt.Println()

	return Index(ctx)
}
