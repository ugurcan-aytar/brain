package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ugurcan-aytar/brain/internal/retriever"
)

func scoreBar(score float64) string {
	const width = 12
	filled := int(score*float64(width) + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	var color lipgloss.Style
	switch {
	case score >= 0.7:
		color = Green
	case score >= 0.4:
		color = Yellow
	default:
		color = Red
	}

	return color.Render(strings.Repeat("█", filled)) + Dim.Render(strings.Repeat("░", empty))
}

// PrintSources renders a titled block of retrieved sources with colored
// confidence bars, one per line.
func PrintSources(chunks []retriever.Chunk, header string) {
	if len(chunks) == 0 {
		return
	}
	label := header
	if label == "" {
		label = "Sources"
	}

	fmt.Println()
	fmt.Println(Dim.Render("  ── " + label + " ─────────────────────────────────────"))
	fmt.Println()
	for _, c := range chunks {
		pct := int(c.Score*100 + 0.5)
		fmt.Printf(
			"  %s %s  %s\n",
			scoreBar(c.Score),
			Dim.Render(fmt.Sprintf("%3d%%", pct)),
			White.Render(c.DisplayPath),
		)
	}
	fmt.Println()
}

// PrintSearchResult renders a single entry in the raw `brain search` output.
func PrintSearchResult(chunk retriever.Chunk, index int) {
	pct := int(chunk.Score*100 + 0.5)
	preview := strings.ReplaceAll(chunk.Snippet, "\n", " ")
	truncated := false
	if len(preview) > 120 {
		preview = preview[:120]
		truncated = true
	}

	fmt.Printf(
		"  %s %s %s  %s\n",
		Dim.Render(fmt.Sprintf("%d.", index+1)),
		scoreBar(chunk.Score),
		Dim.Render(fmt.Sprintf("%d%%", pct)),
		Bold.Render(chunk.Title),
	)
	fmt.Println(Dim.Render("     " + chunk.DisplayPath))
	if preview != "" {
		tail := ""
		if truncated {
			tail = "…"
		}
		fmt.Println(Dim.Render("     " + preview + tail))
	}
	fmt.Println()
}
