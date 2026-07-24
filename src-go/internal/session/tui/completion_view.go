package tui

import (
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	"github.com/charmbracelet/x/ansi"
)

func renderCompletions(matches []repl.Completion, selected, width int) string {
	if len(matches) == 0 || width <= 0 {
		return ""
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(matches) {
		selected = len(matches) - 1
	}

	usageWidth := 0
	for _, match := range matches {
		if w := ansi.StringWidth(match.Spec.Usage); w > usageWidth {
			usageWidth = w
		}
	}

	lines := make([]string, 0, len(matches)+1)
	for i, match := range matches {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		line := marker + match.Spec.Usage
		if width >= 60 {
			line += strings.Repeat(" ", usageWidth-ansi.StringWidth(match.Spec.Usage)+2)
			line += match.Spec.Description
		}
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	if width < 60 {
		description := "  " + matches[selected].Spec.Description
		lines = append(lines, ansi.Truncate(description, width, "…"))
	}
	return strings.Join(lines, "\n")
}
