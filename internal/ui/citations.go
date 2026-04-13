package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ugurcan-aytar/brain/internal/retriever"
)

var reCitation = regexp.MustCompile(`\[([^\[\]]+\.(?:md|txt))\]`)

// VerifyCitations scans the LLM answer for [filename.md] citations and
// warns about any that don't match a retrieved source. This catches the
// most embarrassing class of hallucination — the model inventing a
// filename to look authoritative — without an extra LLM call.
func VerifyCitations(answer string, chunks []retriever.Chunk) {
	known := make(map[string]bool, len(chunks))
	for _, c := range chunks {
		known[c.DisplayPath] = true
		// Also allow bare filename without directory prefix.
		if idx := strings.LastIndex(c.DisplayPath, "/"); idx >= 0 {
			known[c.DisplayPath[idx+1:]] = true
		}
	}

	matches := reCitation.FindAllStringSubmatch(answer, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		cited := m[1]
		if seen[cited] || known[cited] {
			continue
		}
		seen[cited] = true
		fmt.Println(Dim.Render(fmt.Sprintf("  ⚠ cited [%s] but that source wasn't in the retrieved set", cited)))
	}
}
