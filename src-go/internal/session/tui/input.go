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
	if !unicode.IsPrint(r) {
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
	prompt := "> "
	avail := width - lipgloss.Width(prompt)
	if avail < 2 {
		avail = 2
	}

	var visible []rune
	cursorOffset := 0
	if len(t.buffer) <= avail-1 {
		visible = t.buffer
		cursorOffset = len(prompt) + len(t.buffer)
	} else {
		// Scroll visible window so cursor is visible
		end := t.cursor + (avail - 1)
		if end > len(t.buffer) {
			end = len(t.buffer)
		}
		start := end - (avail - 1)
		if start < 0 {
			start = 0
			end = start + avail - 1
			if end > len(t.buffer) {
				end = len(t.buffer)
			}
		}
		visible = t.buffer[start:end]
		cursorOffset = len(prompt) + (t.cursor - start)
	}

	text := string(visible)
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	cursorChar := " "

	before := text[:cursorOffset-len(prompt)]
	after := text[cursorOffset-len(prompt):]

	return prompt + before + cursorStyle.Render(cursorChar) + after
}

func (t *textInput) Len() int {
	return len(t.buffer)
}
