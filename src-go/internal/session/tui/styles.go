package tui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	footerStyle = lipgloss.NewStyle().Faint(true)

	selectedStyle = lipgloss.NewStyle().Bold(true)

	roleStyles = map[string]lipgloss.Style{
		"user":      lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		"assistant": lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		"system":    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
)
