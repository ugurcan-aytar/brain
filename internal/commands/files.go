package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// Files lists every indexed file, optionally scoped to a single collection.
func Files(ctx context.Context, collection string) error {
	args := []string{"ls"}
	if collection != "" {
		args = append(args, collection)
	}

	res, err := runQmd(ctx, args...)
	if err != nil {
		if isMissing(err) {
			printQmdMissing()
			return nil
		}
		return err
	}
	if res.exitCode != 0 {
		fmt.Println(ui.Red.Render("Failed to list files: " + strings.TrimSpace(res.stderr)))
		return nil
	}

	output := strings.TrimSpace(res.stdout)
	if output == "" {
		fmt.Println(ui.Yellow.Render("No indexed files found."))
		fmt.Println(ui.Dim.Render("Run `brain add <path>` then `brain index` to get started."))
		return nil
	}

	fmt.Println(config.RewriteQmdOutput(output))
	return nil
}
