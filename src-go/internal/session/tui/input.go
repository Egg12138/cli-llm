package tui

import (
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

type textInput struct {
	buffer []rune
	cursor int
}

func (t *textInput) Insert(r rune) {
	if !unicode.IsPrint(r) && r != '\n' {
		return
	}
	t.buffer = append(t.buffer[:t.cursor], append([]rune{r}, t.buffer[t.cursor:]...)...)
	t.cursor++
}

func (t *textInput) DeleteBeforeCursor() {
	if t.cursor <= 0 {
		return
	}
	t.buffer = append(t.buffer[:t.cursor-1], t.buffer[t.cursor:]...)
	t.cursor--
}

func (t *textInput) DeleteAtCursor() {
	if t.cursor >= len(t.buffer) {
		return
	}
	t.buffer = append(t.buffer[:t.cursor], t.buffer[t.cursor+1:]...)
}

func (t *textInput) MoveLeft() {
	if t.cursor > 0 {
		t.cursor--
	}
}

func (t *textInput) MoveRight() {
	if t.cursor < len(t.buffer) {
		t.cursor++
	}
}

func (t *textInput) MoveHome() {
	t.cursor = 0
}

func (t *textInput) MoveEnd() {
	t.cursor = len(t.buffer)
}

func (t *textInput) Value() string {
	return string(t.buffer)
}

func (t *textInput) Reset() {
	t.buffer = nil
	t.cursor = 0
}

func (t *textInput) View(width int) string {
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	firstPrompt := "> "
	contPrompt := "  "
	avail := width - lipgloss.Width(firstPrompt)
	if avail < 2 {
		avail = 2
	}

	// Split buffer into logical lines on '\n', tracking cursor position
	var lines []string
	cursorLine, cursorCol := -1, -1

	start := 0
	for i, r := range t.buffer {
		if r == '\n' {
			lines = append(lines, string(t.buffer[start:i]))
			if cursorLine < 0 && t.cursor <= i {
				cursorLine = len(lines) - 1
				cursorCol = t.cursor - start
			}
			start = i + 1
		}
	}
	// Last line
	lines = append(lines, string(t.buffer[start:]))
	if cursorLine < 0 {
		cursorLine = len(lines) - 1
		cursorCol = t.cursor - start
	}

	// Render each line, placing the cursor on the correct one
	var result string
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		prompt := firstPrompt
		if i > 0 {
			prompt = contPrompt
		}
		result += prompt
		if i == cursorLine {
			// Render with cursor at cursorCol position
			if cursorCol >= len([]rune(line)) {
				result += line + cursorStyle.Render(" ")
			} else {
				runes := []rune(line)
				result += string(runes[:cursorCol]) + cursorStyle.Render(string(runes[cursorCol])) + string(runes[cursorCol+1:])
			}
		} else {
			result += line
		}
	}
	return result
}

// LineCount returns the number of display lines (based on '\n' in buffer).
func (t *textInput) LineCount() int {
	if len(t.buffer) == 0 {
		return 1
	}
	count := 1
	for _, r := range t.buffer {
		if r == '\n' {
			count++
		}
	}
	return count
}

func (t *textInput) Len() int {
	return len(t.buffer)
}
