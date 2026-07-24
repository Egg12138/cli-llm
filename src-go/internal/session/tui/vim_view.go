package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (e vimEditor) View(prompt string, width, maxHeight int) string {
	contentWidth := width - ansi.StringWidth(prompt)
	if contentWidth < 1 {
		contentWidth = 1
	}
	if maxHeight < 1 {
		maxHeight = 1
	}

	selectionStart, selectionEnd, hasSelection := e.SelectionRange()
	lines := make([]string, 0, e.lineCount())
	cursorLine := 0
	lineStart := 0
	for {
		lineEnd := lineStart
		for lineEnd < len(e.buffer) && e.buffer[lineEnd] != '\n' {
			lineEnd++
		}
		lineRunes := e.buffer[lineStart:lineEnd]
		styled := strings.Builder{}
		for offset, r := range lineRunes {
			position := lineStart + offset
			style := lipgloss.NewStyle()
			if hasSelection && position >= selectionStart && position < selectionEnd {
				style = visualSelectionStyle
			}
			if position == e.cursor {
				style = e.cursorStyle()
			}
			styled.WriteString(style.Render(string(r)))
		}

		cursorHere := e.cursor >= lineStart && e.cursor <= lineEnd
		if cursorHere && (e.cursor == lineEnd || len(lineRunes) == 0) {
			styled.WriteString(e.cursorStyle().Render(" "))
		}
		wrapped := ansi.Hardwrap(styled.String(), contentWidth, true)
		physical := strings.Split(wrapped, "\n")
		if len(physical) == 0 {
			physical = []string{""}
		}

		if cursorHere {
			local := clampInt(e.cursor-lineStart, 0, len(lineRunes))
			probe := string(lineRunes[:local])
			if local < len(lineRunes) {
				probe += string(lineRunes[local])
			} else {
				probe += " "
			}
			cursorLine = len(lines) + len(strings.Split(ansi.Hardwrap(probe, contentWidth, true), "\n")) - 1
		}

		for _, line := range physical {
			lines = append(lines, prompt+line)
		}
		if lineEnd >= len(e.buffer) {
			break
		}
		lineStart = lineEnd + 1
	}

	if len(lines) <= maxHeight {
		return strings.Join(lines, "\n")
	}
	start := cursorLine - maxHeight + 1
	if start < 0 {
		start = 0
	}
	if start+maxHeight > len(lines) {
		start = len(lines) - maxHeight
	}
	return strings.Join(lines[start:start+maxHeight], "\n")
}

func (e vimEditor) cursorStyle() lipgloss.Style {
	switch e.mode {
	case vimNormal:
		return normalCursorStyle
	case vimVisual:
		return visualCursorStyle
	default:
		return insertCursorStyle
	}
}

func (e vimEditor) lineCount() int {
	count := 1
	for _, r := range e.buffer {
		if r == '\n' {
			count++
		}
	}
	return count
}
