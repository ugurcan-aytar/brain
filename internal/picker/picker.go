// Package picker wraps qmd's "list collections" output and presents an
// interactive multi-select backed by charmbracelet/huh.
package picker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/ugurcan-aytar/brain/internal/config"
)

// ErrCancelled is returned when the user hits Ctrl+C or Esc during picking.
var ErrCancelled = errors.New("picker cancelled")

// ErrNoCollections is returned when qmd has no registered collections yet —
// the caller is expected to show a helpful "run brain add" message.
var ErrNoCollections = errors.New("no collections registered")

// sentinel used inside huh to signal "all collections"; translated to a nil
// return value in Pick() so callers can distinguish "all" from "none".
const allSentinel = "__ALL__"

var collectionLineRegex = regexp.MustCompile(`^(\S+)\s+\(qmd://`)

// Names returns the list of collection names qmd knows about. It parses the
// human-readable `qmd collection list` output — qmd has a JSON flag for
// query results but not for this command.
func Names(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, config.Default.QmdBinary, "collection", "list")
	cmd.Env = config.QmdEnv()
	out, err := cmd.Output()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, fmt.Errorf("qmd is not installed or not found in PATH")
		}
		return nil, err
	}

	var names []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if m := collectionLineRegex.FindStringSubmatch(line); m != nil {
			names = append(names, m[1])
		}
	}
	return names, nil
}

// PickOptions customizes the interactive prompt.
type PickOptions struct {
	Title string
}

// Pick prompts the user to select one or more collections. A nil return
// value (with nil error) means "all collections" (the user chose the
// aggregate option). An empty error return after ErrNoCollections / ErrCancelled
// allows callers to distinguish between states.
func Pick(ctx context.Context, opts PickOptions) ([]string, error) {
	names, err := Names(ctx)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, ErrNoCollections
	}

	title := opts.Title
	if title == "" {
		title = "Select collections"
	}

	options := []huh.Option[string]{
		huh.NewOption("All collections", allSentinel),
	}
	for _, n := range names {
		options = append(options, huh.NewOption(n, n))
	}

	for {
		var selected []string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(title).
					Description("space: toggle    enter: confirm    ctrl+c: cancel").
					Options(options...).
					Value(&selected),
			),
		).WithShowHelp(false).WithShowErrors(false)

		if err := form.Run(); err != nil {
			return nil, ErrCancelled
		}

		if len(selected) == 0 {
			fmt.Println("  Please select at least one collection.")
			continue
		}

		for _, s := range selected {
			if s == allSentinel {
				return nil, nil
			}
		}
		return selected, nil
	}
}
