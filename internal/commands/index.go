package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/engine"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// NewIndexCmd wires the Index handler into a Cobra command.
func NewIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Re-index and embed all collections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Index(cmd.Context())
		},
	}
}

// Index re-scans every registered collection and re-embeds new/changed
// chunks. Safe to re-run: recall skips unchanged files by content hash.
func Index(ctx context.Context) error {
	eng, err := engine.Open()
	if err != nil {
		return err
	}
	defer eng.Close()
	return indexWithEngine(ctx, eng)
}

// indexWithEngine is shared with `brain add` so we don't open a second
// engine when add→index runs back-to-back.
func indexWithEngine(ctx context.Context, eng *engine.Engine) error {
	_ = ctx
	fmt.Println(ui.Bold.Render("Indexing your knowledge base…"))
	fmt.Println()

	// Step 1: scan files + update document rows.
	var (
		idxRes *recall.IndexResult
		idxErr error
	)
	action := func() { idxRes, idxErr = eng.Recall().Index() }
	if err := spinner.New().Title("Scanning files…").Action(action).Run(); err != nil {
		return err
	}
	if idxErr != nil {
		fmt.Println(ui.Red.Render("Scan failed: " + idxErr.Error()))
		return nil
	}
	fmt.Println(ui.Green.Render("✓ Scan complete"))
	for name, stats := range idxRes.PerCollection {
		fmt.Println(ui.Dim.Render(fmt.Sprintf("  %s: +%d -%d ~%d =%d",
			name, stats.Indexed, stats.Removed, stats.Updated, stats.Unchanged)))
	}

	// Step 2: embed anything that's missing a vector. Graceful no-op
	// when the local GGUF backend isn't compiled in AND no API provider
	// is configured — BM25 still works.
	emb, embErr := eng.Embedder()
	if embErr != nil {
		fmt.Println(ui.Yellow.Render("! Embedder configuration error: " + embErr.Error()))
		fmt.Println(ui.Dim.Render("  Indexing completed; vector search is disabled until resolved."))
		return nil
	}
	if emb == nil {
		fmt.Println(ui.Dim.Render("  Local GGUF backend not available; skipping embedding step."))
		fmt.Println(ui.Dim.Render("  Set RECALL_EMBED_PROVIDER=openai|voyage for hybrid search, or rebuild with -tags embed_llama."))
		return nil
	}

	var (
		embRes *recall.EmbedResult
		embErr2 error
	)
	embAction := func() { embRes, embErr2 = eng.Recall().Embed(emb, false) }
	if err := spinner.New().Title("Generating embeddings…").Action(embAction).Run(); err != nil {
		return err
	}
	if embErr2 != nil {
		if errors.Is(embErr2, recall.ErrLocalEmbedderNotCompiled) {
			fmt.Println(ui.Dim.Render("  Local GGUF backend not compiled in; skipping embedding."))
			return nil
		}
		fmt.Println(ui.Red.Render("Embedding failed: " + embErr2.Error()))
		return nil
	}
	fmt.Println(ui.Green.Render(fmt.Sprintf("✓ Embedded %d chunk(s)", embRes.Embedded)))

	fmt.Println()
	fmt.Println(ui.Green.Render("✓ Indexing complete. Your brain is up to date."))
	return nil
}
