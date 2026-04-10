package ui

import "github.com/charmbracelet/lipgloss"

var (
	Dim     = lipgloss.NewStyle().Faint(true)
	Bold    = lipgloss.NewStyle().Bold(true)
	Italic  = lipgloss.NewStyle().Italic(true)
	Red     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	Green   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	Yellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Blue    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	Magenta = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	Cyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	White   = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)
