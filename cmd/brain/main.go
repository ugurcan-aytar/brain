// Command brain is a CLI that turns a folder of markdown/txt notes into a
// conversational knowledge base. See `brain --help` for subcommands.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/commands"
	"golang.org/x/term"
)

const version = "0.1.1"

func main() {
	// Restore terminal state on any exit path — readline + huh both put the
	// TTY into raw mode, and a panic or signal can otherwise leave the shell
	// unable to echo input. golang.org/x/term is cross-platform (macOS, Linux,
	// Windows) so we don't need a stty subprocess.
	restore := snapshotTerminal()
	defer restore()

	// SIGTERM always exits hard (no handler can recover meaningfully). SIGINT
	// is handled by each subcommand itself — chat wants first Ctrl+C to cancel
	// the in-flight stream without tearing down the REPL, while ask/search
	// install local handlers scoped to a single request.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	go func() {
		<-sigs
		restore()
		os.Exit(143)
	}()

	ctx := context.Background()
	root := newRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		restore()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// snapshotTerminal captures the current TTY state (if stdin is a terminal)
// and returns a best-effort restore closure. It's a no-op when stdin isn't a
// TTY — e.g. when brain is piped or run under CI.
func snapshotTerminal() func() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return func() {}
	}
	state, err := term.GetState(fd)
	if err != nil {
		return func() {}
	}
	return func() { _ = term.Restore(fd, state) }
}

const longDescription = `Second Brain CLI -- conversational knowledge base over your local notes

  Query:
    brain ask "<question>"       One-shot Q&A with cited sources
    brain chat                   Interactive multi-turn conversation
    brain search "<query>"       Raw retrieval results (no LLM)

  Collections:
    brain add <path>             Register a folder of notes
    brain remove <name>          Remove a collection and clean up index
    brain collections            List all registered collections
    brain files [-c name]        List all indexed files

  Maintenance:
    brain index                  Re-index and generate embeddings
    brain status                 Show index health and config
    brain doctor                 Check required dependencies and config`

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "brain",
		Short:         "Conversational knowledge base over your local notes",
		Long:          longDescription,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newAskCmd(),
		newChatCmd(),
		newSearchCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newCollectionsCmd(),
		newStatusCmd(),
		newIndexCmd(),
		newFilesCmd(),
		newDoctorCmd(),
	)
	return root
}

func newAskCmd() *cobra.Command {
	var opts commands.AskOptions
	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "One-shot Q&A against your notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Ask(cmd.Context(), args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Collection, "collection", "c", "", "Scope search to a specific collection")
	cmd.Flags().StringVarP(&opts.Model, "model", "m", "opus", "Claude model (opus, sonnet, haiku)")
	cmd.Flags().StringVarP(&opts.Mode, "mode", "M", "auto", "Thinking mode (auto, recall, analysis, decision, synthesis)")
	return cmd
}

func newChatCmd() *cobra.Command {
	var opts commands.ChatOptions
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interactive REPL chat with your notes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Chat(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Model, "model", "m", "opus", "Claude model (opus, sonnet, haiku)")
	cmd.Flags().StringVarP(&opts.Collection, "collection", "c", "", "Scope chat to a specific collection (skips the picker)")
	return cmd
}

func newSearchCmd() *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search your notes without LLM (raw retrieval results)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Search(cmd.Context(), args[0], collection)
		},
	}
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Scope search to a specific collection")
	return cmd
}

func newAddCmd() *cobra.Command {
	var opts commands.AddOptions
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a new collection of notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Add(cmd.Context(), args[0], opts)
		},
	}
	cmd.Flags().StringVar(&opts.Name, "name", "", "Collection name (default: folder basename)")
	cmd.Flags().StringVar(&opts.Mask, "mask", "", "File glob mask (default: **/*.{txt,md})")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a collection and clean up its index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Remove(cmd.Context(), args[0])
		},
	}
}

func newCollectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "collections",
		Short: "List all registered collections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Collections(cmd.Context())
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show index status and brain config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Status(cmd.Context())
		},
	}
}

func newIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Re-index and embed all collections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Index(cmd.Context())
		},
	}
}

func newFilesCmd() *cobra.Command {
	var collection string
	cmd := &cobra.Command{
		Use:   "files",
		Short: "List all indexed files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Files(cmd.Context(), collection)
		},
	}
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Filter by collection")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check required dependencies and configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Doctor(cmd.Context())
		},
	}
}
