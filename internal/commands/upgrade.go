package commands

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ugurcan-aytar/brain/internal/ui"
	"github.com/ugurcan-aytar/brain/internal/version"
)

// Upgrade detects how brain was installed and prints the matching
// update command. It intentionally does NOT download and swap the
// binary — package managers know how to do that properly with the
// right permissions, signatures, and rollback semantics. We just
// surface the right invocation so users don't have to remember it.
func Upgrade(ctx context.Context) error {
	fmt.Println(ui.Bold.Render("brain upgrade"))
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  Current version:  v%s", version.Current)))

	// Synchronous fetch so we can show the latest version number in
	// the output. Ignore errors — we still want to print the install
	// instructions even if GitHub is unreachable.
	latestCtx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()
	latestTag, err := fetchLatestTag(latestCtx)

	if err == nil && latestTag != "" {
		if isNewerTag(latestTag, version.Current) {
			fmt.Println(ui.Dim.Render(fmt.Sprintf("  Latest release:   %s  ", latestTag)) + ui.Green.Render("(new)"))
			// Refresh the cache so the per-command banner stops
			// nagging once the user has seen this screen.
			if path, perr := updateCachePath(); perr == nil {
				refreshUpdateCache(path)
			}
		} else {
			fmt.Println(ui.Dim.Render(fmt.Sprintf("  Latest release:   %s  ", latestTag)) + ui.Green.Render("(up to date)"))
			fmt.Println()
			fmt.Println(ui.Green.Render("You're on the latest version. Nothing to do."))
			return nil
		}
	} else {
		fmt.Println(ui.Dim.Render("  Latest release:   unknown (couldn't reach github.com)"))
	}

	fmt.Println()
	fmt.Println(ui.Bold.Render("Run one of these, matching how you installed brain:"))
	fmt.Println()

	exe, _ := os.Executable()
	switch {
	case strings.Contains(exe, "/opt/homebrew/") || strings.Contains(exe, "linuxbrew"):
		fmt.Println("  brew upgrade brain")
		fmt.Println(ui.Dim.Render("    # detected: Homebrew install at " + exe))
	case strings.Contains(exe, "/go/bin/") || strings.Contains(exe, "go/pkg/mod/"):
		fmt.Println("  go install github.com/ugurcan-aytar/brain/cmd/brain@latest")
		fmt.Println(ui.Dim.Render("    # detected: go install at " + exe))
	default:
		fmt.Println("  brew upgrade brain")
		fmt.Println(ui.Dim.Render("    # if you installed via Homebrew"))
		fmt.Println()
		fmt.Println("  go install github.com/ugurcan-aytar/brain/cmd/brain@latest")
		fmt.Println(ui.Dim.Render("    # if you installed via `go install`"))
		fmt.Println()
		fmt.Println("  curl -sSfL https://raw.githubusercontent.com/ugurcan-aytar/brain/main/install.sh | sh")
		fmt.Println(ui.Dim.Render("    # if you installed via the shell script"))
	}

	if runtime.GOOS == "linux" {
		fmt.Println()
		fmt.Println(ui.Dim.Render("  Linux package downloads (.deb / .rpm / .apk) are on the Releases page:"))
		fmt.Println(ui.Dim.Render("    https://github.com/ugurcan-aytar/brain/releases/latest"))
	}

	fmt.Println()
	fmt.Println(ui.Dim.Render("  To silence the update banner entirely: export BRAIN_NO_UPDATE_CHECK=1"))
	return nil
}

// NewUpgradeCmd wires the Upgrade handler into a Cobra command.
func NewUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Check for a newer release and print the install command",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Upgrade(cmd.Context())
		},
	}
}
