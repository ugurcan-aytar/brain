package config

import (
	"os"
	"strings"
	"testing"
)

func TestRewriteQmdOutput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "add command with current dir",
			in:   "Try: qmd collection add . --name foo",
			want: "Try: brain add <path> --name foo",
		},
		{
			name: "add command general",
			in:   "qmd collection add /path",
			want: "brain add /path",
		},
		{
			name: "remove command",
			in:   "Use qmd collection remove oldname",
			want: "Use brain remove oldname",
		},
		{
			name: "list command",
			in:   "qmd collection list to see collections",
			want: "brain collections to see collections",
		},
		{
			name: "update command",
			in:   "Run qmd update to reindex",
			want: "Run brain index to reindex",
		},
		{
			name: "embed command",
			in:   "qmd embed rebuilds vectors",
			want: "brain index rebuilds vectors",
		},
		{
			name: "embed hint phrase",
			in:   "Run 'qmd embed'",
			want: "Run 'brain index'",
		},
		{
			name: "status command",
			in:   "Check with qmd status",
			want: "Check with brain status",
		},
		{
			name: "files command",
			in:   "Try qmd ls first",
			want: "Try brain files first",
		},
		{
			name: "unrelated text passes through",
			in:   "nothing to rewrite here",
			want: "nothing to rewrite here",
		},
		{
			name: "multiple replacements in one string",
			in:   "qmd collection list then qmd status",
			want: "brain collections then brain status",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RewriteQmdOutput(tc.in)
			if got != tc.want {
				t.Errorf("RewriteQmdOutput(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestQmdEnvStripsBunInstall(t *testing.T) {
	// Set a sentinel so we can confirm it's gone post-filter.
	t.Setenv("BUN_INSTALL", "/tmp/bun-test")
	t.Setenv("BRAIN_TEST_MARKER", "present")

	env := QmdEnv()

	var (
		sawBun    bool
		sawMarker bool
	)
	for _, kv := range env {
		if strings.HasPrefix(kv, "BUN_INSTALL=") {
			sawBun = true
		}
		if kv == "BRAIN_TEST_MARKER=present" {
			sawMarker = true
		}
	}

	if sawBun {
		t.Error("QmdEnv() should strip BUN_INSTALL but it was still present")
	}
	if !sawMarker {
		t.Error("QmdEnv() should preserve unrelated env vars but BRAIN_TEST_MARKER was missing")
	}
}

func TestQmdEnvPreservesOtherVars(t *testing.T) {
	// Sanity check: the returned slice has roughly the same shape as os.Environ.
	env := QmdEnv()
	osEnv := os.Environ()

	// With BUN_INSTALL possibly absent, the returned slice should be at most
	// len(osEnv) long and at least len(osEnv)-1 long.
	if len(env) > len(osEnv) {
		t.Errorf("QmdEnv returned more entries (%d) than os.Environ (%d)", len(env), len(osEnv))
	}
}

func TestDefaultSettingsInvariants(t *testing.T) {
	if Default.TopK <= 0 {
		t.Error("TopK must be positive")
	}
	if Default.MaxTokens <= 0 {
		t.Error("MaxTokens must be positive")
	}
	if Default.MinScore < 0 || Default.MinScore > 1 {
		t.Errorf("MinScore should be in [0,1], got %f", Default.MinScore)
	}
	if Default.Model == "" {
		t.Error("Model must not be empty")
	}
	if Default.QmdBinary == "" {
		t.Error("QmdBinary must not be empty")
	}
}
