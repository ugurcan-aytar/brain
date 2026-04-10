package commands

// Shared helpers for commands that shell out to qmd. Centralizing error
// messaging here means "qmd is not installed" prints identically everywhere
// and we never leak the bare exec error path to the user.

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/config"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

type qmdResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runQmd(ctx context.Context, args ...string) (qmdResult, error) {
	cmd := exec.CommandContext(ctx, config.Default.QmdBinary, args...)
	cmd.Env = config.QmdEnv()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := qmdResult{stdout: stdout.String(), stderr: stderr.String()}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.exitCode = exitErr.ExitCode()
			return res, nil // non-zero exit is not a Go error; callers inspect exitCode
		}
		var notFound *exec.Error
		if errors.As(err, &notFound) {
			return res, errQmdMissing
		}
		return res, err
	}
	return res, nil
}

var errQmdMissing = errors.New("qmd is not installed or not found in PATH")

// printQmdMissing renders the standard "please install qmd" message. All
// command entry points call this from their err path, so the install hint
// only lives in one place.
func printQmdMissing() {
	fmt.Println(ui.Red.Render("Error: qmd is not installed or not found in PATH."))
	fmt.Println(ui.Dim.Render("Install it with: npm install -g @tobilu/qmd"))
}

// isMissing checks whether an error is our sentinel for "qmd not in PATH".
func isMissing(err error) bool { return errors.Is(err, errQmdMissing) }
