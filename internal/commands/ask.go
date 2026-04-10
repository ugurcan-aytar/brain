package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh/spinner"
	"github.com/ugurcan-aytar/brain/internal/history"
	"github.com/ugurcan-aytar/brain/internal/llm"
	"github.com/ugurcan-aytar/brain/internal/picker"
	"github.com/ugurcan-aytar/brain/internal/prompt"
	"github.com/ugurcan-aytar/brain/internal/retriever"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// AskOptions are the flags that ride alongside `brain ask`.
type AskOptions struct {
	Collection string // single collection shortcut; skips the picker
	Model      string // alias or full model ID
	Mode       string // one of prompt.ValidModes (auto|recall|…)
}

// Ask runs the one-shot Q&A path: pick collections → retrieve → grounding
// gate → classify mode → stream answer → print sources → save history.
// SIGINT cancels the in-flight stage and returns cleanly.
func Ask(parent context.Context, question string, opts AskOptions) error {
	ctx, stopSignal := withSignalCancel(parent)
	defer stopSignal()

	collections, err := resolveCollections(ctx, opts.Collection)
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			fmt.Println()
			return nil
		}
		if errors.Is(err, picker.ErrNoCollections) {
			fmt.Println(ui.Yellow.Render("No collections found. Add one with: brain add <path>"))
			return nil
		}
		return err
	}

	if collections != nil {
		fmt.Println(ui.Dim.Render("  Collections: " + strings.Join(collections, ", ")))
		fmt.Println()
	}

	var (
		chunks     []retriever.Chunk
		retrieveErr error
	)
	retrieveAction := func() {
		chunks, retrieveErr = retriever.Retrieve(ctx, question, retriever.Options{
			Collections: collections,
		})
	}
	if err := spinner.New().Title("Searching your notes…").Action(retrieveAction).Run(); err != nil {
		return err
	}

	if ctx.Err() != nil {
		fmt.Println(ui.Dim.Render("\n  Cancelled."))
		return nil
	}
	if retrieveErr != nil {
		if errors.Is(retrieveErr, retriever.ErrQmdMissing) {
			printQmdMissing()
			return nil
		}
		return retrieveErr
	}
	if !retriever.GroundingGate(chunks) {
		return nil
	}

	var modeOverride prompt.QueryType
	if opts.Mode != "" && opts.Mode != "auto" && prompt.IsValidMode(opts.Mode) {
		modeOverride = prompt.QueryType(opts.Mode)
	}

	detected := prompt.Classify(question)
	active := detected
	modeSuffix := " - auto"
	if modeOverride != "" {
		active = modeOverride
		modeSuffix = ""
	}
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  [%s%s]", active, modeSuffix)))
	fmt.Println()

	systemPrompt := prompt.BuildSystemPrompt(chunks, question, modeOverride)

	answer, err := llm.Stream(ctx, systemPrompt, []llm.Message{
		{Role: llm.RoleUser, Content: question},
	}, llm.Options{Model: opts.Model})
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		fmt.Println(ui.Dim.Render("\n  Cancelled."))
		return nil
	}

	ui.PrintSources(chunks, "")
	ui.PrintLogo()

	if _, err := history.Save(question, answer, chunks, "ask"); err != nil {
		fmt.Println(ui.Dim.Render("  (history not saved: " + err.Error() + ")"))
	}
	return nil
}

// resolveCollections returns either a single-collection slice (when the user
// passed --collection), or the user's picker selection. A nil slice with nil
// error means "all collections".
func resolveCollections(ctx context.Context, flag string) ([]string, error) {
	if flag != "" {
		return []string{flag}, nil
	}
	return picker.Pick(ctx, picker.PickOptions{})
}
