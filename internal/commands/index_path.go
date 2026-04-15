// Shared --index flag surface for every subcommand.
//
// brain inherits recall's named-index convention (v0.2.6+): each
// named index lives at ~/.recall/indexes/<name>.db, isolated from
// the default ~/.recall/index.db. We expose the flag as a persistent
// root-level flag (wired in cmd/brain/main.go) so `brain --index
// work ask "..."` works on every subcommand without each having to
// re-declare it.

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/ugurcan-aytar/recall/pkg/recall"

	"github.com/ugurcan-aytar/brain/internal/engine"
)

// IndexName is set from the `--index` persistent flag on the root
// command. Empty = use recall's default path (~/.recall/index.db or
// $RECALL_DB_PATH). Non-empty = open ~/.recall/indexes/<name>.db
// via recall.ResolveIndexPath.
//
// The package-level var mirrors how `dbPath` is wired in recall's
// own cmd tree — every subcommand ends up calling OpenEngine() and
// the flag propagates transparently.
var IndexName string

// OpenEngine opens the brain engine against either the default
// recall DB (when IndexName is empty) or the named index. Prints a
// one-shot stderr warning if --index collides with $RECALL_DB_PATH
// — path env wins, matching recall's own precedence.
func OpenEngine() (*engine.Engine, error) {
	name := strings.TrimSpace(IndexName)
	if name == "" {
		return engine.Open()
	}
	if envPath := os.Getenv("RECALL_DB_PATH"); envPath != "" {
		fmt.Fprintf(os.Stderr,
			"warning: --index %q ignored because $RECALL_DB_PATH is set\n", name)
		return engine.Open()
	}
	path, err := recall.ResolveIndexPath(name)
	if err != nil {
		return nil, fmt.Errorf("resolve --index %q: %w", name, err)
	}
	return engine.Open(recall.WithDBPath(path))
}
