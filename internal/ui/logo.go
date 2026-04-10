package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var logoRows = []string{
	"██████╗ ██████╗  █████╗ ██╗███╗   ██╗",
	"██╔══██╗██╔══██╗██╔══██╗██║████╗  ██║",
	"██████╔╝██████╔╝███████║██║██╔██╗ ██║",
	"██╔══██╗██╔══██╗██╔══██║██║██║╚██╗██║",
	"██████╔╝██║  ██║██║  ██║██║██║ ╚████║",
	"╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝",
}

// gradientText colors each non-space rune along a cyan→purple gradient.
func gradientText(text string) string {
	start := [3]int{34, 211, 238} // cyan
	end := [3]int{168, 85, 247}   // purple

	runes := []rune(text)
	solid := 0
	for _, r := range runes {
		if r != ' ' {
			solid++
		}
	}

	var b strings.Builder
	idx := 0
	for _, r := range runes {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		var t float64
		if solid > 1 {
			t = float64(idx) / float64(solid-1)
		}
		rc := int(float64(start[0]) + (float64(end[0])-float64(start[0]))*t + 0.5)
		gc := int(float64(start[1]) + (float64(end[1])-float64(start[1]))*t + 0.5)
		bc := int(float64(start[2]) + (float64(end[2])-float64(start[2]))*t + 0.5)
		hex := fmt.Sprintf("#%02x%02x%02x", rc, gc, bc)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		b.WriteString(style.Render(string(r)))
		idx++
	}
	return b.String()
}

func PrintLogo() {
	fmt.Println()
	for _, line := range logoRows {
		fmt.Printf("  %s\n", gradientText(line))
	}
	fmt.Println()
}
