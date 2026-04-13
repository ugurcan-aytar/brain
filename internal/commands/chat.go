package commands

// The chat REPL is the most feature-dense surface in brain. It handles:
//
//   - Interactive multi-turn Q&A with rolling history
//   - Slash commands with unique-prefix matching and Tab completion
//   - Cross-reference (/challenge) against a second collection set
//   - Mid-response cancellation via Ctrl+C (double-tap to exit)
//   - Mode and model switching mid-session
//
// All of the REPL-specific terminal quirks are encapsulated here so the
// other command files stay straightforward.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/history"
	"github.com/ugurcan-aytar/brain/internal/llm"
	"github.com/ugurcan-aytar/brain/internal/picker"
	"github.com/ugurcan-aytar/brain/internal/prompt"
	"github.com/ugurcan-aytar/brain/internal/retriever"
	"github.com/ugurcan-aytar/brain/internal/ui"
)

// ChatOptions are the flags that ride alongside `brain chat`.
type ChatOptions struct {
	Model      string
	Collection string // single-collection shortcut; skips the picker
}

// NewChatCmd wires the Chat REPL into a Cobra command with its flags.
func NewChatCmd() *cobra.Command {
	var opts ChatOptions
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interactive REPL chat with your notes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Chat(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Model, "model", "m", "sonnet", "Claude model (sonnet, opus, haiku)")
	cmd.Flags().StringVarP(&opts.Collection, "collection", "c", "", "Scope chat to a specific collection (skips the picker)")
	return cmd
}

var slashCommands = []slashCommand{
	{"/help", "Show this message"},
	{"/mode", "Set thinking mode (auto, recall, analysis, decision, synthesis)"},
	{"/model", "Switch Claude model"},
	{"/collections", "Re-pick active collections"},
	{"/sources", "Show sources from last answer"},
	{"/challenge", "Cross-reference last Q&A against different sources"},
	{"/clear", "Reset conversation history"},
	{"/quit", "Exit chat"},
}

// resolveCommand maps an input line to a slash command via exact match or
// unique prefix. Returns "" if the input doesn't disambiguate to exactly one
// command.
func resolveCommand(input string) string {
	head := strings.Fields(input)
	if len(head) == 0 || !strings.HasPrefix(head[0], "/") {
		return ""
	}
	cmd := strings.ToLower(head[0])
	for _, c := range slashCommands {
		if c.name == cmd {
			return cmd
		}
	}
	var matches []string
	for _, c := range slashCommands {
		if strings.HasPrefix(c.name, cmd) {
			matches = append(matches, c.name)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

// Chat runs the interactive REPL until the user exits.
func Chat(ctx context.Context, opts ChatOptions) error {
	currentModel := opts.Model
	if currentModel == "" {
		currentModel = "sonnet"
	}

	ui.PrintLogo()

	// Same pre-check as Ask — bail before the picker so users see friendly
	// guidance instead of getting stuck in a REPL with a broken backend.
	if llm.Select() == llm.BackendNone {
		printNoBackend()
		return nil
	}

	// --collection / -c lets users skip the picker when they already know
	// which collection they want to talk to — same shortcut `brain ask`
	// supports. Primary use is scripting/demos, but it's also just nicer
	// for power users who work mostly in one collection.
	var activeCollections []string
	if opts.Collection != "" {
		activeCollections = []string{opts.Collection}
	} else {
		picked, err := picker.Pick(ctx, picker.PickOptions{})
		if err != nil {
			if errors.Is(err, picker.ErrCancelled) || errors.Is(err, picker.ErrNoCollections) {
				if errors.Is(err, picker.ErrNoCollections) {
					fmt.Println(ui.Yellow.Render("No collections found. Add one with: brain add <path>"))
					return nil
				}
				fmt.Println(ui.Dim.Render("\nGoodbye."))
				return nil
			}
			return err
		}
		activeCollections = picked
	}

	currentMode := "auto"

	modeDisplay := func() string { return currentMode }
	collectionsDisplay := func() string {
		if activeCollections == nil {
			return "All"
		}
		return strings.Join(activeCollections, ", ")
	}

	printHelp := func() {
		fmt.Println(ui.Dim.Render(fmt.Sprintf(
			"  Model: %s  Collections: %s  Mode: %s",
			llm.Display(currentModel),
			collectionsDisplay(),
			modeDisplay(),
		)))
		fmt.Println()
		for _, c := range slashCommands {
			fmt.Println(ui.Dim.Render(fmt.Sprintf("  %-16s %s", c.name, c.help)))
		}
		fmt.Println()
	}

	printHelp()
	PrintUpdateBanner()

	var (
		historyMessages []llm.Message
		lastChunks      []retriever.Chunk
		lastCtrlC       time.Time
	)

	for {
		line, result, readErr := readChatInput(slashCommands)
		if readErr != nil {
			return readErr
		}
		if result == chatInputEOF {
			fmt.Println(ui.Dim.Render("Goodbye."))
			return nil
		}
		if result == chatInputInterrupted {
			// Double-tap Ctrl+C within 2s exits. Single tap prints a hint
			// and clears whatever was partially typed.
			if time.Since(lastCtrlC) < 2*time.Second {
				fmt.Println(ui.Dim.Render("Goodbye."))
				return nil
			}
			lastCtrlC = time.Now()
			fmt.Println()
			fmt.Println(ui.Dim.Render("  Press Ctrl+C again to exit."))
			fmt.Println()
			continue
		}

		lastCtrlC = time.Time{}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		// Re-print the submitted question so it lives in persistent
		// scrollback — the bubbletea textinput returns "" on submit
		// and the subsequent spinner would otherwise overwrite the
		// last-frame question below it.
		fmt.Println(ui.Cyan.Render("❯ ") + input)

		resolved := resolveCommand(input)

		if resolved == "/help" || input == "/" {
			printHelp()
			continue
		}

		if resolved == "/quit" {
			fmt.Println(ui.Dim.Render("Goodbye."))
			return nil
		}

		if resolved == "/clear" {
			historyMessages = nil
			lastChunks = nil
			fmt.Println(ui.Dim.Render("Conversation cleared."))
			fmt.Println()
			continue
		}

		if resolved == "/sources" {
			if len(lastChunks) > 0 {
				ui.PrintSources(lastChunks, "")
			} else {
				fmt.Println(ui.Dim.Render("No sources from a previous answer."))
				fmt.Println()
			}
			continue
		}

		if resolved == "/collections" {
			picked, perr := picker.Pick(ctx, picker.PickOptions{})
			if perr != nil && !errors.Is(perr, picker.ErrCancelled) {
				fmt.Println(ui.Red.Render("  " + perr.Error()))
				fmt.Println()
				continue
			}
			if perr == nil {
				activeCollections = picked
				fmt.Println(ui.Dim.Render("  Collections: " + collectionsDisplay()))
				fmt.Println()
			}
			continue
		}

		if resolved == "/model" {
			rest := strings.TrimSpace(strings.TrimPrefix(input, "/model"))
			if rest == "" {
				picked, perr := modelPicker(currentModel)
				if perr == nil {
					currentModel = picked
					fmt.Println(ui.Dim.Render("  Switched to " + llm.Display(currentModel)))
					fmt.Println()
				} else {
					fmt.Println()
				}
			} else if !llm.IsValidModel(rest) {
				fmt.Println(ui.Yellow.Render("  Unknown model: " + rest))
				fmt.Println(ui.Dim.Render("  Available: sonnet (default), opus, haiku"))
				fmt.Println(ui.Dim.Render("  Or use a full model ID like claude-sonnet-4-6"))
				fmt.Println()
			} else {
				currentModel = rest
				fmt.Println(ui.Dim.Render("  Switched to " + llm.Display(currentModel)))
				fmt.Println()
			}
			continue
		}

		if resolved == "/mode" {
			arg := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(input, "/mode")))
			if arg == "" {
				fmt.Println(ui.Dim.Render("  Current mode: " + modeDisplay()))
				fmt.Println(ui.Dim.Render("  Available: " + strings.Join(prompt.ValidModes, ", ")))
				fmt.Println(ui.Dim.Render("  auto = detect from query | recall = direct lookup | analysis = cross-reference"))
				fmt.Println(ui.Dim.Render("  decision = frameworks + recommendation | synthesis = action plan"))
				fmt.Println()
			} else if prompt.IsValidMode(arg) {
				currentMode = arg
				fmt.Println(ui.Dim.Render("  Mode: " + modeDisplay()))
				fmt.Println()
			} else {
				fmt.Println(ui.Yellow.Render("  Unknown mode: " + arg))
				fmt.Println(ui.Dim.Render("  Available: " + strings.Join(prompt.ValidModes, ", ")))
				fmt.Println()
			}
			continue
		}

		if resolved == "/challenge" {
			if err := runChallenge(ctx, &historyMessages, &lastChunks, currentModel); err != nil {
				return err
			}
			continue
		}

		// Unrecognized slash command → print suggestions.
		if strings.HasPrefix(input, "/") {
			head := strings.ToLower(strings.Fields(input)[0])
			var matches []string
			for _, c := range slashCommands {
				if strings.HasPrefix(c.name, head) {
					matches = append(matches, c.name)
				}
			}
			if len(matches) > 1 {
				fmt.Println(ui.Yellow.Render("  Ambiguous: " + strings.Join(matches, ", ")))
			} else {
				fmt.Println(ui.Yellow.Render("  Unknown command: " + head))
			}
			fmt.Println(ui.Dim.Render("  Type /help to see available commands."))
			fmt.Println()
			continue
		}

		// ── Regular question: search + stream with Ctrl+C cancellation ──
		streamCtx, cancel := context.WithCancel(ctx)
		done := watchSIGINT(streamCtx, cancel)

		var (
			chunks     []retriever.Chunk
			retrErr    error
		)
		searchAction := func() {
			chunks, retrErr = retriever.Retrieve(streamCtx, input, retriever.Options{
				Collections: activeCollections,
			})
		}
		if err := spinner.New().Title("Searching…").Action(searchAction).Run(); err != nil {
			cancel()
			<-done
			return err
		}

		if streamCtx.Err() != nil {
			cancel()
			<-done
			fmt.Println(ui.Dim.Render("  Cancelled."))
			fmt.Println()
			continue
		}
		if retrErr != nil {
			cancel()
			<-done
			if errors.Is(retrErr, retriever.ErrQmdMissing) {
				printQmdMissing()
				continue
			}
			return retrErr
		}

		if !retriever.GroundingGate(chunks) {
			cancel()
			<-done
			continue
		}

		lastChunks = chunks
		detected := prompt.Classify(input)
		activeModeLabel := detected
		modeSuffix := " - auto"
		var modeOverride prompt.QueryType
		if currentMode != "auto" {
			modeOverride = prompt.QueryType(currentMode)
			activeModeLabel = modeOverride
			modeSuffix = ""
		}
		fmt.Println(ui.Dim.Render(fmt.Sprintf("  [%s%s]", activeModeLabel, modeSuffix)))
		fmt.Println()

		var systemPrompt, chunkContext string
		if llm.Select() == llm.BackendAnthropicAPI {
			systemPrompt = prompt.StaticDirectives()
			chunkContext = prompt.ContextBlock(chunks, input, modeOverride)
		} else {
			systemPrompt = prompt.BuildSystemPrompt(chunks, input, modeOverride)
		}

		historyMessages = append(historyMessages, llm.Message{Role: llm.RoleUser, Content: input})
		historyMessages = trimHistory(historyMessages)

		streamStart := time.Now()
		response, streamErr := llm.Stream(streamCtx, systemPrompt, historyMessages, llm.Options{Model: currentModel, ChunkContext: chunkContext})
		streamElapsed := time.Since(streamStart)
		// Capture user-cancellation BEFORE we run our own cleanup cancel() —
		// otherwise streamCtx.Err() below would always be non-nil and we'd
		// silently drop every successful response from the history, which
		// breaks both multi-turn conversations and /challenge.
		userCancelled := streamCtx.Err() != nil
		cancel()
		<-done

		if streamErr != nil {
			fmt.Println(ui.Red.Render("  " + streamErr.Error()))
			historyMessages = historyMessages[:len(historyMessages)-1]
			continue
		}
		if userCancelled {
			fmt.Println(ui.Dim.Render("\n  Cancelled."))
			fmt.Println()
			historyMessages = historyMessages[:len(historyMessages)-1]
			continue
		}

		historyMessages = append(historyMessages, llm.Message{Role: llm.RoleAssistant, Content: response})
		fmt.Println()
		fmt.Println(ui.Dim.Render(fmt.Sprintf("  responded in %s", formatElapsed(streamElapsed))))
		ui.PrintSources(chunks, "")
		if _, err := history.Save(history.Entry{
			Question:    input,
			Answer:      response,
			Sources:     chunks,
			Mode:        "chat",
			Thinking:    string(activeModeLabel),
			Model:       llm.Display(currentModel),
			Collections: activeCollections,
			Elapsed:     streamElapsed,
		}); err != nil {
			fmt.Println(ui.Dim.Render("  (history not saved: " + err.Error() + ")"))
		}
	}
}

// watchSIGINT installs a SIGINT handler scoped to one streaming call. When
// the signal fires, the context is cancelled and the LLM/retriever stages
// unwind. Returning a `done` channel lets the caller wait for the handler
// goroutine to exit before reinstalling the normal REPL handler.
func watchSIGINT(ctx context.Context, cancel context.CancelFunc) <-chan struct{} {
	done := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		defer close(done)
		defer signal.Stop(sigs)
		select {
		case <-ctx.Done():
		case <-sigs:
			cancel()
		}
	}()
	return done
}

// formatElapsed renders a Stream duration in a dev-friendly unit — under
// a minute it's just seconds ("43s"), over a minute it's minutes + seconds
// ("2m13s"). Keeps the post-response footer concise regardless of how
// slow the backend was.
func formatElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	mins := int(d / time.Minute)
	secs := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%ds", mins, secs)
}

// trimHistory bounds the conversation buffer at maxConversationTurns pairs
// — one user + one assistant per turn.
func trimHistory(msgs []llm.Message) []llm.Message {
	const maxTurns = 10
	max := maxTurns * 2
	if len(msgs) <= max {
		return msgs
	}
	return msgs[len(msgs)-max:]
}

// runChallenge handles the /challenge flow: pick a second set of collections,
// retrieve against them, and stream a re-scored answer. Mutates the caller's
// history and lastChunks on success so /sources reflects the challenge run.
func runChallenge(
	ctx context.Context,
	historyMessages *[]llm.Message,
	lastChunks *[]retriever.Chunk,
	currentModel string,
) error {
	msgs := *historyMessages
	var lastUser, lastAssistant *llm.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == llm.RoleUser && lastUser == nil {
			m := msgs[i]
			lastUser = &m
		}
		if msgs[i].Role == llm.RoleAssistant && lastAssistant == nil {
			m := msgs[i]
			lastAssistant = &m
		}
		if lastUser != nil && lastAssistant != nil {
			break
		}
	}
	if lastUser == nil || lastAssistant == nil {
		fmt.Println(ui.Yellow.Render("  Nothing to challenge yet. Ask a question first."))
		fmt.Println()
		return nil
	}

	challengeCols, err := picker.Pick(ctx, picker.PickOptions{Title: "Challenge with collections"})
	if err != nil {
		if !errors.Is(err, picker.ErrCancelled) {
			fmt.Println(ui.Red.Render("  " + err.Error()))
		}
		fmt.Println()
		return nil
	}

	challengeCtx, cancel := context.WithCancel(ctx)
	done := watchSIGINT(challengeCtx, cancel)
	defer func() { cancel(); <-done }()

	var (
		challengeChunks []retriever.Chunk
		retrErr         error
	)
	action := func() {
		challengeChunks, retrErr = retriever.Retrieve(challengeCtx, lastUser.Content, retriever.Options{
			Collections: challengeCols,
		})
	}
	if err := spinner.New().Title("Retrieving challenge sources…").Action(action).Run(); err != nil {
		return err
	}

	if challengeCtx.Err() != nil {
		fmt.Println(ui.Dim.Render("  Cancelled."))
		fmt.Println()
		return nil
	}
	if retrErr != nil {
		if errors.Is(retrErr, retriever.ErrQmdMissing) {
			printQmdMissing()
			return nil
		}
		return retrErr
	}

	if len(challengeChunks) == 0 {
		fmt.Println(ui.Yellow.Render("  No relevant notes found in challenged collections."))
		fmt.Println()
		return nil
	}

	challengePrompt := prompt.BuildChallengePrompt(lastUser.Content, lastAssistant.Content, *lastChunks, challengeChunks)

	streamStart := time.Now()
	response, streamErr := llm.Stream(challengeCtx, challengePrompt, []llm.Message{
		{Role: llm.RoleUser, Content: "Challenge the previous answer using these new sources."},
	}, llm.Options{Model: currentModel})
	streamElapsed := time.Since(streamStart)
	// Same userCancelled capture as the main loop: check the context
	// before the defer's cancel() runs, otherwise every successful stream
	// would be reported as cancelled and the challenge response would
	// never land in history.
	userCancelled := challengeCtx.Err() != nil

	if streamErr != nil {
		fmt.Println(ui.Red.Render("  " + streamErr.Error()))
		return nil
	}
	if userCancelled {
		fmt.Println(ui.Dim.Render("\n  Cancelled."))
		fmt.Println()
		return nil
	}

	*historyMessages = append(*historyMessages,
		llm.Message{Role: llm.RoleUser, Content: "[Challenge] " + lastUser.Content},
		llm.Message{Role: llm.RoleAssistant, Content: response},
	)
	*lastChunks = challengeChunks

	fmt.Println()
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  responded in %s", formatElapsed(streamElapsed))))
	ui.PrintSources(challengeChunks, "Challenge Sources")
	if _, err := history.Save(history.Entry{
		Question:    "[Challenge] " + lastUser.Content,
		Answer:      response,
		Sources:     challengeChunks,
		Mode:        "chat",
		Thinking:    "challenge",
		Model:       llm.Display(currentModel),
		Collections: challengeCols,
		Elapsed:     streamElapsed,
	}); err != nil {
		fmt.Println(ui.Dim.Render("  (history not saved: " + err.Error() + ")"))
	}
	return nil
}

// modelPicker prompts the user to select one of the known model aliases.
func modelPicker(current string) (string, error) {
	var picked string
	opts := []huh.Option[string]{}
	for _, mc := range llm.ModelChoices {
		label := fmt.Sprintf("%-8s -- %-22s (%s)", mc.Alias, mc.ResolvedID, mc.Description)
		if mc.Alias == current {
			label += "  <-- current"
		}
		opts = append(opts, huh.NewOption(label, mc.Alias))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Select model (current: %s)", current)).
				Options(opts...).
				Value(&picked),
		),
	).WithShowHelp(false).WithShowErrors(false)
	if err := form.Run(); err != nil {
		return "", err
	}
	return picked, nil
}

