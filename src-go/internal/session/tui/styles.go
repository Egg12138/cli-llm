package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	footerStyle = lipgloss.NewStyle().Faint(true)
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().Bold(true)

	roleUserColor      = lipgloss.AdaptiveColor{Light: "4", Dark: "12"}
	roleAssistantColor = lipgloss.AdaptiveColor{Light: "2", Dark: "10"}
	roleSystemColor    = lipgloss.AdaptiveColor{Light: "8", Dark: "7"}

	roleStyles = map[string]lipgloss.Style{
		"user":      lipgloss.NewStyle().Foreground(roleUserColor),
		"assistant": lipgloss.NewStyle().Foreground(roleAssistantColor),
		"system":    lipgloss.NewStyle().Foreground(roleSystemColor),
	}
)
