package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
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
	Deep       bool   // two-pass retrieval: LLM filters 20 chunks down to 8-10
}

// NewAskCmd wires the Ask handler into a Cobra command with its flags.
func NewAskCmd() *cobra.Command {
	var opts AskOptions
	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "One-shot Q&A against your notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Ask(cmd.Context(), args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Collection, "collection", "c", "", "Scope search to a specific collection")
	cmd.Flags().StringVarP(&opts.Model, "model", "m", "sonnet", "Claude model (sonnet, opus, haiku)")
	cmd.Flags().StringVarP(&opts.Mode, "mode", "M", "auto", "Thinking mode (auto, recall, analysis, decision, synthesis)")
	cmd.Flags().BoolVar(&opts.Deep, "deep", false, "Two-pass retrieval: LLM filters chunks for deeper analysis")
	return cmd
}

// Ask runs the one-shot Q&A path: pick collections → retrieve → grounding
// gate → classify mode → stream answer → print sources → save history.
// SIGINT cancels the in-flight stage and returns cleanly.
func Ask(parent context.Context, question string, opts AskOptions) error {
	ctx, stopSignal := withSignalCancel(parent)
	defer stopSignal()

	// Fail fast before the picker takes over the TTY — nothing else brain
	// does is useful without a working backend, and we don't want to make
	// the user click through collections just to hit an error.
	if llm.Select() == llm.BackendNone {
		printNoBackend()
		return nil
	}

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
	searchStart := time.Now()
	retrieveAction := func() {
		chunks, retrieveErr = retriever.Retrieve(ctx, question, retriever.Options{
			Collections: collections,
		})
	}
	if err := spinner.New().Title("Searching your notes…").Action(retrieveAction).Run(); err != nil {
		return err
	}
	searchElapsed := time.Since(searchStart)

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

	if opts.Deep {
		chunks = retriever.DeepFilter(ctx, chunks, question, llm.QuickComplete)
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
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  [%s%s]  searched in %s", active, modeSuffix, formatElapsed(searchElapsed))))
	fmt.Println()

	var systemPrompt, chunkContext string
	if llm.Select() == llm.BackendAnthropicAPI {
		systemPrompt = prompt.StaticDirectives()
		chunkContext = prompt.ContextBlock(chunks, question, modeOverride)
	} else {
		systemPrompt = prompt.BuildSystemPrompt(chunks, question, modeOverride)
	}

	streamStart := time.Now()
	answer, err := llm.Stream(ctx, systemPrompt, []llm.Message{
		{Role: llm.RoleUser, Content: question},
	}, llm.Options{Model: opts.Model, ChunkContext: chunkContext})
	elapsed := time.Since(streamStart)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		fmt.Println(ui.Dim.Render("\n  Cancelled."))
		return nil
	}

	ui.VerifyCitations(answer, chunks)
	ui.PrintSources(chunks, "")
	ui.PrintLogo()

	if _, err := history.Save(history.Entry{
		Question:    question,
		Answer:      answer,
		Sources:     chunks,
		Mode:        "ask",
		Thinking:    string(active),
		Model:       llm.Display(opts.Model),
		Collections: collections,
		Elapsed:     elapsed,
	}); err != nil {
		fmt.Println(ui.Dim.Render("  (history not saved: " + err.Error() + ")"))
	}
	PrintUpdateBanner()
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
