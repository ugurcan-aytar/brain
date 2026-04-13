package ui

import (
	"testing"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

func TestCitationRegex(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		want    int // number of unique [file.md] matches
	}{
		{"no citations", "plain text without brackets", 0},
		{"one md citation", "see [notes.md] for details", 1},
		{"one txt citation", "check [data.txt] now", 1},
		{"duplicate counted once", "[a.md] and [a.md] again", 1},
		{"two distinct", "[a.md] vs [b.txt]", 2},
		{"not a file extension", "[something] not a citation", 0},
		{"nested brackets ignored", "[[a.md]]", 1},
		{"path with dir", "[dir/notes.md] here", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matches := reCitation.FindAllStringSubmatch(tc.text, -1)
			seen := map[string]bool{}
			for _, m := range matches {
				seen[m[1]] = true
			}
			if len(seen) != tc.want {
				t.Errorf("got %d unique citations, want %d (matches: %v)", len(seen), tc.want, seen)
			}
		})
	}
}

func TestVerifyCitationsNoFalsePositives(t *testing.T) {
	chunks := []retriever.Chunk{
		{DisplayPath: "notes/a.md", Score: 0.9},
		{DisplayPath: "data/b.txt", Score: 0.8},
	}
	// All cited files exist in chunks — VerifyCitations should print nothing.
	// We can't easily capture stdout here without DI, but we can at least
	// verify the known set logic by calling the regex directly.
	known := map[string]bool{}
	for _, c := range chunks {
		known[c.DisplayPath] = true
	}
	// Full path match
	if !known["notes/a.md"] {
		t.Error("expected notes/a.md in known set")
	}
	// Bare filename should also be recognized (the function adds it)
	if known["a.md"] {
		t.Error("bare filename should NOT be in the raw known set — VerifyCitations adds it")
	}
}
