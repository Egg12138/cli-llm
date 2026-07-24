package tui

import (
	"io"
	"strings"

	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const maxEditorHeight = 8

type EditorModel struct {
	textarea            textarea.Model
	matches             []sessionrepl.Completion
	selected            int
	width               int
	height              int
	done                bool
	completionDismissed bool
	event               sessionrepl.InputEvent
}

func NewEditorModel(prompt string) EditorModel {
	input := textarea.New()
	input.Prompt = prompt
	input.ShowLineNumbers = false
	input.SetWidth(defaultWidth)
	input.SetHeight(1)
	input.Focus()

	m := EditorModel{
		textarea: input,
		width:    defaultWidth,
		height:   defaultHeight,
	}
	m.refreshCompletions()
	return m
}

func (m EditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m EditorModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateSize()
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(message)
	m.recalculateSize()
	return m, cmd
}

func (m EditorModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.textarea.Value() == "" {
			return m, nil
		}
		m.done = true
		m.event = sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Line: m.textarea.Value()}
		return m, tea.Quit
	case tea.KeyCtrlJ:
		m.textarea.InsertString("\n")
		m.completionDismissed = false
		m.refreshCompletions()
		m.recalculateSize()
		return m, nil
	case tea.KeyCtrlT:
		m.done = true
		m.event = sessionrepl.InputEvent{Kind: sessionrepl.EventTranscript}
		return m, tea.Quit
	case tea.KeyCtrlC:
		m.done = true
		m.event = sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Err: sessionrepl.ErrInterrupted}
		return m, tea.Quit
	case tea.KeyCtrlD:
		if m.textarea.Value() == "" {
			m.done = true
			m.event = sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Err: io.EOF}
			return m, tea.Quit
		}
	case tea.KeyTab:
		if len(m.matches) > 0 {
			line, cursor := sessionrepl.ApplyCompletion(m.textarea.Value(), m.cursor(), m.matches[m.selected])
			m.textarea.SetValue(line)
			m.textarea.SetCursor(cursor)
			m.matches = nil
			m.selected = 0
			m.completionDismissed = true
			m.recalculateSize()
			return m, nil
		}
	case tea.KeyUp:
		if len(m.matches) > 0 {
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.matches) - 1
			}
			return m, nil
		}
	case tea.KeyDown:
		if len(m.matches) > 0 {
			m.selected = (m.selected + 1) % len(m.matches)
			return m, nil
		}
	case tea.KeyEsc:
		if len(m.matches) > 0 {
			m.matches = nil
			m.selected = 0
			m.completionDismissed = true
			return m, nil
		}
	case tea.KeySpace:
		m.textarea.InsertRune(' ')
		m.completionDismissed = false
		m.refreshCompletions()
		m.recalculateSize()
		return m, nil
	}

	before := m.textarea.Value()
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	if m.textarea.Value() != before {
		m.completionDismissed = false
	}
	m.refreshCompletions()
	m.recalculateSize()
	return m, cmd
}

func (m EditorModel) View() string {
	if m.done {
		return ""
	}
	parts := []string{m.textarea.View()}
	if completions := renderCompletions(m.matches, m.selected, m.width); completions != "" {
		parts = append(parts, completions)
	}
	hint := ansi.Truncate("Enter send · Ctrl+J newline · Ctrl+T transcript", m.width, "…")
	parts = append(parts, footerStyle.Render(hint))
	return strings.Join(parts, "\n")
}

func (m EditorModel) Event() sessionrepl.InputEvent {
	return m.event
}

func (m EditorModel) Value() string {
	return m.textarea.Value()
}

func (m *EditorModel) refreshCompletions() {
	if m.completionDismissed {
		return
	}
	m.matches = sessionrepl.CompleteCommand(m.textarea.Value(), m.cursor())
	if len(m.matches) == 0 {
		m.selected = 0
	} else if m.selected >= len(m.matches) {
		m.selected = len(m.matches) - 1
	}
}

func (m EditorModel) cursor() int {
	if m.textarea.Line() != 0 {
		return len([]rune(m.textarea.Value()))
	}
	info := m.textarea.LineInfo()
	return info.StartColumn + info.ColumnOffset
}

func (m *EditorModel) recalculateSize() {
	width := m.width
	if width < 1 {
		width = 1
	}
	m.textarea.SetWidth(width)
	rows := m.textarea.LineCount() - 1 + m.textarea.LineInfo().Height
	if rows < 1 {
		rows = 1
	}
	if rows > maxEditorHeight {
		rows = maxEditorHeight
	}
	m.textarea.SetHeight(rows)
}

var _ tea.Model = EditorModel{}
