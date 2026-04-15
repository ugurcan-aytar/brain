package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/history"
	"github.com/ugurcan-aytar/brain/internal/llm"
	"github.com/ugurcan-aytar/brain/internal/picker"
	"github.com/ugurcan-aytar/brain/internal/prompt"
	"github.com/ugurcan-aytar/brain/internal/retriever"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// AskOptions are the flags that ride alongside `brain ask`.
//
// --expand / --rerank / --hyde are orthogonal enhancements that
// trade a one-time subprocess boot (~1-3 s for expansion, ~400 ms
// for reranker) for better retrieval quality — combine freely.
// --deep is the pre-existing LLM chunk-filter pass; it sits AFTER
// retrieval and is independent of the three new flags.
type AskOptions struct {
	Collection string // single-collection shortcut; skips the picker
	Model      string // alias or full model ID
	Mode       string // one of prompt.ValidModes (auto|recall|…)
	Deep       bool   // post-retrieval LLM chunk filter (20 → 8-10)
	Expand     bool   // query expansion via recall's local LLM
	Rerank     bool   // cross-encoder rerank via recall's bge-reranker
	Hyde       bool   // hypothetical-doc embedding
	Explain    bool   // show per-chunk score trace
	TopK       int    // -n; 0 → config default
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
	cmd.Flags().BoolVar(&opts.Deep, "deep", false, "Post-retrieval LLM chunk filter (20 → 8-10). Independent of --expand/--rerank/--hyde.")
	cmd.Flags().BoolVar(&opts.Expand, "expand", false, "Query expansion — run an LLM over the query to produce lex/vec variants + HyDE passages")
	cmd.Flags().BoolVar(&opts.Rerank, "rerank", false, "Cross-encoder rerank the top-30 candidates via bge-reranker-v2-m3")
	cmd.Flags().BoolVar(&opts.Hyde, "hyde", false, "HyDE — embed LLM-generated hypothetical answers as extra vector probes")
	cmd.Flags().BoolVar(&opts.Explain, "explain", false, "Annotate retrieved chunks with a short score trace")
	cmd.Flags().IntVarP(&opts.TopK, "top", "n", 0, "Candidate count (default: config.TopK)")
	return cmd
}

// Ask runs the one-shot Q&A path: pick collections → retrieve → grounding
// gate → classify mode → stream answer → print sources → save history.
// SIGINT cancels the in-flight stage and returns cleanly.
func Ask(parent context.Context, question string, opts AskOptions) error {
	ctx, stopSignal := withSignalCancel(parent)
	defer stopSignal()

	// Fail fast before the picker takes over the TTY — nothing else brain
	// does is useful without a working backend.
	if llm.Select() == llm.BackendNone {
		printNoBackend()
		return nil
	}

	eng, err := OpenEngine()
	if err != nil {
		return err
	}
	defer eng.Close()

	collections, err := resolveCollections(ctx, eng, opts.Collection)
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
		chunks      []retriever.Chunk
		retrieveErr error
	)
	searchStart := time.Now()
	retrieveAction := func() {
		chunks, retrieveErr = retriever.Retrieve(ctx, eng, question, retriever.Options{
			Collections: collections,
			TopK:        opts.TopK,
			Expand:      opts.Expand,
			Rerank:      opts.Rerank,
			Hyde:        opts.Hyde,
			Explain:     opts.Explain,
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
		return retrieveErr
	}
	if !retriever.GroundingGate(chunks) {
		return nil
	}

	// Enrich top results with full document bodies so the LLM sees
	// complete source content, not just the winning chunk.
	chunks = retriever.EnrichTopChunks(ctx, eng, chunks, 3)

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

// resolveCollections returns either a single-collection slice (when the
// user passed --collection), or the user's picker selection. A nil
// slice with nil error means "all collections".
func resolveCollections(ctx context.Context, eng *engine.Engine, flag string) ([]string, error) {
	if flag != "" {
		return []string{flag}, nil
	}
	names, err := collectionNames(eng)
	if err != nil {
		return nil, err
	}
	return picker.Pick(ctx, names, picker.PickOptions{})
}

// collectionNames is a small helper so ask/chat don't duplicate the
// "fetch names for picker" logic.
func collectionNames(eng *engine.Engine) ([]string, error) {
	cols, err := eng.Recall().ListCollections()
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c.Name)
	}
	return names, nil
}
