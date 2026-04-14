// Package picker presents an interactive multi-select over a
// pre-fetched list of collection names. Callers resolve the name list
// from recall.Engine.ListCollections and hand it in — picker stays
// stateless / testable and has no engine dependency.
package picker

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// ErrCancelled is returned when the user hits Ctrl+C or Esc during picking.
var ErrCancelled = errors.New("picker cancelled")

// ErrNoCollections is returned when there are no collections to offer.
// The caller is expected to show a "run brain add" message.
var ErrNoCollections = errors.New("no collections registered")

// sentinel used inside huh to signal "all collections"; translated to a
// nil return value in Pick() so callers can distinguish "all" from "none".
const allSentinel = "__ALL__"

// PickOptions customizes the interactive prompt.
type PickOptions struct {
	Title string
}

// Pick prompts the user to select one or more collections from names.
// A nil return value (with nil error) means "all collections" (user
// chose the aggregate option). ctx is accepted for future
// cancellability; huh consumes its own signal handler today.
func Pick(ctx context.Context, names []string, opts PickOptions) ([]string, error) {
	_ = ctx

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
