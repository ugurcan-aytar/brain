package ui

import (
	"fmt"

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

// logoStyle renders the ASCII logo in a single warm parchment tone. We
// deliberately avoid a gradient — the cyan→purple gradient brain shipped
// with looked generic-AI, and a single warm color leans into the
// "reading notes by lamplight" metaphor that fits the product.
var logoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#fde68a"))

func PrintLogo() {
	fmt.Println()
	for _, line := range logoRows {
		fmt.Printf("  %s\n", logoStyle.Render(line))
	}
	fmt.Println()
}
