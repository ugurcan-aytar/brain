// Package history persists each Q&A exchange as a timestamped markdown file
// under ~/.brain/history. Filenames use local time (not UTC) so users scanning
// their own history aren't confused by "yesterday" answers labeled "tomorrow".
package history

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

// Directory returns the absolute path where history files are written.
// By default this is `$XDG_STATE_HOME/brain/history`, falling back to
// `~/.brain/history` when the env var is unset — the same ~-relative
// convention the TypeScript version used, but ~/.brain/ rather than the
// project-relative history/ folder so a globally-installed binary can
// still write.
func Directory() string {
	if dir := os.Getenv("BRAIN_HISTORY_DIR"); dir != "" {
		return dir
	}
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "brain", "history")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "history"
	}
	return filepath.Join(home, ".brain", "history")
}

var slugReplacer = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func slugify(text string) string {
	lowered := strings.ToLower(text)
	replaced := slugReplacer.ReplaceAllString(lowered, "-")
	replaced = strings.Trim(replaced, "-")
	if len(replaced) > 50 {
		replaced = replaced[:50]
	}
	return replaced
}

func timestamp(now time.Time) string {
	return now.Format("2006-01-02_15-04-05")
}

// Save writes a Q&A exchange to the history directory and returns the path.
// Mode should be "ask" or "chat"; it's just recorded as metadata.
func Save(question, answer string, sources []retriever.Chunk, mode string) (string, error) {
	dir := Directory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create history dir: %w", err)
	}

	now := time.Now()
	filename := fmt.Sprintf("%s_%s.md", timestamp(now), slugify(question))
	path := filepath.Join(dir, filename)

	var srcBlock strings.Builder
	for _, c := range sources {
		pct := int(c.Score*100 + 0.5)
		fmt.Fprintf(&srcBlock, "- %s (%d%%)\n", c.DisplayPath, pct)
	}

	content := fmt.Sprintf(`# %s

> **Date:** %s
> **Mode:** %s

---

%s

---

## Sources

%s`, question, now.Format("2006-01-02 15:04:05"), mode, answer, srcBlock.String())

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write history: %w", err)
	}
	return path, nil
}
