// Command brain is a CLI that turns a folder of markdown/txt notes into a
// conversational knowledge base. See `brain --help` for subcommands.
//
// This file is intentionally thin: each subcommand's Cobra wiring lives
// next to its handler in internal/commands/<name>.go, so all you find here
// is the entry point, terminal cleanup, signal handling, and a root-command
// constructor that stitches the registered subcommands together.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/ugurcan-aytar/brain/internal/commands"
	"github.com/ugurcan-aytar/brain/internal/version"
	"golang.org/x/term"
)

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
    brain doctor                 Check required dependencies and config
    brain upgrade                Show how to upgrade to the latest release
    brain history                Browse saved Q&A history`

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

	// Fire a background "is there a newer release?" check. No-op when
	// stdout isn't a terminal or BRAIN_NO_UPDATE_CHECK is set. Safe to
	// call unconditionally — the check is rate-limited to once per day
	// and runs in a goroutine.
	commands.CheckForUpdate()

	if err := newRootCmd().ExecuteContext(context.Background()); err != nil {
		restore()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// snapshotTerminal captures the current TTY state (if stdin is a terminal)
// and returns a best-effort restore closure. No-op when stdin isn't a TTY
// — e.g. when brain is piped or run under CI.
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

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "brain",
		Short:         "Conversational knowledge base over your local notes",
		Long:          longDescription,
		Version:       version.Current,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		commands.NewAskCmd(),
		commands.NewChatCmd(),
		commands.NewSearchCmd(),
		commands.NewAddCmd(),
		commands.NewRemoveCmd(),
		commands.NewCollectionsCmd(),
		commands.NewStatusCmd(),
		commands.NewIndexCmd(),
		commands.NewFilesCmd(),
		commands.NewDoctorCmd(),
		commands.NewUpgradeCmd(),
		commands.NewHistoryCmd(),
	)
	return root
}
