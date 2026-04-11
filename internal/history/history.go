// Package history persists each Q&A exchange as a timestamped markdown file
// under ~/.brain/history. Filenames use local time (not UTC) so users scanning
// their own history aren't confused by "yesterday" answers labeled "tomorrow".
package history

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// Entry is the metadata captured alongside a Q&A exchange. All fields are
// optional; empty values are omitted from the persisted header so legacy
// files parse cleanly.
type Entry struct {
	Question    string
	Answer      string
	Sources     []retriever.Chunk
	Mode        string // "ask" or "chat"
	Thinking    string // prompt.QueryType label (auto, recall, analysis, …)
	Model       string // llm.Display() of the active model
	Collections []string
	Elapsed     time.Duration
}

// Save writes a Q&A exchange to the history directory and returns the path.
func Save(e Entry) (string, error) {
	dir := Directory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create history dir: %w", err)
	}

	now := time.Now()
	filename := fmt.Sprintf("%s_%s.md", timestamp(now), slugify(e.Question))
	path := filepath.Join(dir, filename)

	var header strings.Builder
	fmt.Fprintf(&header, "> **Date:** %s\n", now.Format("2006-01-02 15:04:05"))
	if e.Mode != "" {
		fmt.Fprintf(&header, "> **Mode:** %s\n", e.Mode)
	}
	if e.Thinking != "" {
		fmt.Fprintf(&header, "> **Thinking:** %s\n", e.Thinking)
	}
	if e.Model != "" {
		fmt.Fprintf(&header, "> **Model:** %s\n", e.Model)
	}
	if len(e.Collections) > 0 {
		fmt.Fprintf(&header, "> **Collections:** %s\n", strings.Join(e.Collections, ", "))
	}
	if e.Elapsed > 0 {
		fmt.Fprintf(&header, "> **Elapsed:** %s\n", formatDuration(e.Elapsed))
	}

	var srcBlock strings.Builder
	for _, c := range e.Sources {
		pct := int(c.Score*100 + 0.5)
		fmt.Fprintf(&srcBlock, "- %s (%d%%)\n", c.DisplayPath, pct)
	}

	content := fmt.Sprintf(`# %s

%s
---

%s

---

## Sources

%s`, e.Question, header.String(), e.Answer, srcBlock.String())

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write history: %w", err)
	}
	return path, nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	mins := int(d / time.Minute)
	secs := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%ds", mins, secs)
}

// Record is a lightweight summary parsed from a persisted history file. It
// carries just enough to render list/search output without loading the full
// answer body. Call Load() to hydrate the full content.
type Record struct {
	Path      string    // absolute path on disk
	Filename  string    // basename (no dir)
	Timestamp time.Time // parsed from filename prefix; zero if parse failed
	Question  string    // from the `# …` H1 line
	Mode      string
	Thinking  string
	Model     string
	Collections string
	Elapsed   string
}

var filenameRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2})_(.+)\.md$`)

// List returns persisted history records, sorted newest-first by filename
// timestamp. Files that don't match the expected naming convention are
// skipped so a stray `.md` in the directory doesn't break the command.
func List() ([]Record, error) {
	dir := Directory()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read history dir: %w", err)
	}

	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		m := filenameRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		ts, _ := time.ParseInLocation("2006-01-02_15-04-05", m[1], time.Local)
		rec := Record{
			Path:      filepath.Join(dir, entry.Name()),
			Filename:  entry.Name(),
			Timestamp: ts,
		}
		readSummary(&rec)
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Filename > records[j].Filename
	})
	return records, nil
}

// readSummary fills in the Question/Mode/etc fields by scanning the first
// handful of lines of the file. Failures are swallowed — a record with a
// missing question is still useful.
func readSummary(rec *Record) {
	f, err := os.Open(rec.Path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := 0
	for sc.Scan() {
		line := sc.Text()
		lines++
		if lines > 20 || strings.HasPrefix(line, "---") {
			break
		}
		switch {
		case rec.Question == "" && strings.HasPrefix(line, "# "):
			rec.Question = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		case strings.HasPrefix(line, "> **Mode:**"):
			rec.Mode = strings.TrimSpace(strings.TrimPrefix(line, "> **Mode:**"))
		case strings.HasPrefix(line, "> **Thinking:**"):
			rec.Thinking = strings.TrimSpace(strings.TrimPrefix(line, "> **Thinking:**"))
		case strings.HasPrefix(line, "> **Model:**"):
			rec.Model = strings.TrimSpace(strings.TrimPrefix(line, "> **Model:**"))
		case strings.HasPrefix(line, "> **Collections:**"):
			rec.Collections = strings.TrimSpace(strings.TrimPrefix(line, "> **Collections:**"))
		case strings.HasPrefix(line, "> **Elapsed:**"):
			rec.Elapsed = strings.TrimSpace(strings.TrimPrefix(line, "> **Elapsed:**"))
		}
	}
	if rec.Question == "" {
		// Fall back to the slug in the filename so list output is never blank.
		if m := filenameRe.FindStringSubmatch(rec.Filename); m != nil {
			rec.Question = strings.ReplaceAll(m[2], "-", " ")
		}
	}
}

// Load returns the full file contents for a record.
func Load(rec Record) (string, error) {
	b, err := os.ReadFile(rec.Path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Delete removes a history file.
func Delete(rec Record) error {
	return os.Remove(rec.Path)
}

// Search returns records whose file content matches the (case-insensitive)
// query. Matches on question, answer, or source block. The match is a plain
// substring — good enough for the scale of a single user's Q&A archive.
func Search(query string) ([]Record, error) {
	all, err := List()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	var out []Record
	for _, rec := range all {
		content, err := os.ReadFile(rec.Path)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(content)), needle) {
			out = append(out, rec)
		}
	}
	return out, nil
}
