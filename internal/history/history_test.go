package history

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ascii words", "what is this", "what-is-this"},
		{"uppercase folded", "Hello World", "hello-world"},
		{"punctuation collapsed", "what?! really...", "what-really"},
		{"leading/trailing separators trimmed", "   hi   ", "hi"},
		{"empty string", "", ""},
		{"turkish chars preserved", "günaydın dünya", "günaydın-dünya"},
		{"cyrillic preserved", "привет мир", "привет-мир"},
		{"numbers kept", "plan 2026 q1", "plan-2026-q1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := slugify(tc.in)
			if got != tc.want {
				t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlugifyTruncatesToFifty(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := slugify(long)
	if len(got) != 50 {
		t.Errorf("expected slug length 50, got %d", len(got))
	}
}

func TestTimestampFormat(t *testing.T) {
	// Use a fixed time so the test is deterministic regardless of TZ.
	fixed := time.Date(2026, 4, 10, 15, 4, 5, 0, time.UTC)
	got := timestamp(fixed)
	want := "2026-04-10_15-04-05"
	if got != want {
		t.Errorf("timestamp(fixed) = %q, want %q", got, want)
	}
}

func TestDirectoryFromEnvOverride(t *testing.T) {
	t.Setenv("BRAIN_HISTORY_DIR", "/tmp/brain-custom-history")
	if got := Directory(); got != "/tmp/brain-custom-history" {
		t.Errorf("Directory() = %q, want override", got)
	}
}

func TestDirectoryFromXDGStateHome(t *testing.T) {
	t.Setenv("BRAIN_HISTORY_DIR", "")
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg")
	got := Directory()
	want := filepath.Join("/tmp/xdg", "brain", "history")
	if got != want {
		t.Errorf("Directory() = %q, want %q", got, want)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BRAIN_HISTORY_DIR", tmp)
	// Make sure XDG doesn't override the test dir.
	t.Setenv("XDG_STATE_HOME", "")

	chunks := []retriever.Chunk{
		{DisplayPath: "notes/a.md", Score: 0.85, Snippet: "x"},
		{DisplayPath: "notes/b.md", Score: 0.42, Snippet: "y"},
	}

	path, err := Save("What is attention?", "Attention is all you need.", chunks, "ask")
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if !strings.HasPrefix(path, tmp) {
		t.Errorf("Save wrote outside BRAIN_HISTORY_DIR: %q", path)
	}
	if !strings.HasSuffix(path, ".md") {
		t.Errorf("history file should end in .md, got %q", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	text := string(content)

	mustContain := []string{
		"# What is attention?",
		"Attention is all you need.",
		"**Mode:** ask",
		"notes/a.md (85%)",
		"notes/b.md (42%)",
		"## Sources",
	}
	for _, s := range mustContain {
		if !strings.Contains(text, s) {
			t.Errorf("saved history missing %q\n---\n%s", s, text)
		}
	}

	// Filename pattern: <timestamp>_<slug>.md
	base := filepath.Base(path)
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}_what-is-attention\.md$`)
	if !re.MatchString(base) {
		t.Errorf("filename %q does not match expected pattern", base)
	}
}

func TestSaveHandlesEmptySources(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("BRAIN_HISTORY_DIR", tmp)
	t.Setenv("XDG_STATE_HOME", "")

	_, err := Save("quick q", "quick a", nil, "chat")
	if err != nil {
		t.Fatalf("Save with empty sources should not error: %v", err)
	}
}
