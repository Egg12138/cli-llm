package tui

import (
	"io"
	"strings"

	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const maxEditorHeight = 8

type EditorModel struct {
	editor              vimEditor
	prompt              string
	matches             []sessionrepl.Completion
	selected            int
	width               int
	height              int
	done                bool
	completionDismissed bool
	event               sessionrepl.InputEvent
}

func NewEditorModel(prompt string) EditorModel {
	m := EditorModel{
		editor: newVimEditor(),
		prompt: prompt,
		width:  defaultWidth,
		height: defaultHeight,
	}
	m.refreshCompletions()
	return m
}

func (m EditorModel) Init() tea.Cmd {
	return nil
}

func (m EditorModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m EditorModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.editor.Value() == "" {
			return m, nil
		}
		m.done = true
		m.event = sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Line: m.editor.Value()}
		return m, tea.Quit
	case tea.KeyCtrlJ:
		m.enterInsertForNewline()
		m.editor.InsertUserText("\n")
		m.completionDismissed = false
		m.refreshCompletions()
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
		if m.editor.Value() == "" {
			m.done = true
			m.event = sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Err: io.EOF}
			return m, tea.Quit
		}
		if m.editor.Mode() == vimInsert {
			m.editor.DeleteAtCursor()
			m.refreshAfterEdit()
		}
		return m, nil
	case tea.KeyTab:
		if m.editor.Mode() == vimInsert && len(m.matches) > 0 {
			line, cursor := sessionrepl.ApplyCompletion(m.editor.Value(), m.cursor(), m.matches[m.selected])
			m.editor.SetValue(line)
			m.editor.SetCursor(cursor)
			m.matches = nil
			m.selected = 0
			m.completionDismissed = true
			return m, nil
		}
	case tea.KeyUp:
		if m.editor.Mode() == vimInsert && len(m.matches) > 0 {
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.matches) - 1
			}
			return m, nil
		}
	case tea.KeyDown:
		if m.editor.Mode() == vimInsert && len(m.matches) > 0 {
			m.selected = (m.selected + 1) % len(m.matches)
			return m, nil
		}
	case tea.KeyEsc:
		m.matches = nil
		m.selected = 0
		m.completionDismissed = true
		m.editor.Handle("esc")
		return m, nil
	}

	before := m.editor.Value()
	beforeMode := m.editor.Mode()
	m.handleEditingKey(msg)
	if m.editor.Value() != before || m.editor.Mode() == vimInsert && beforeMode != vimInsert {
		m.completionDismissed = false
	}
	m.refreshCompletions()
	return m, nil
}

func (m *EditorModel) handleEditingKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeySpace:
		if m.editor.Mode() == vimInsert {
			m.editor.InsertUserText(" ")
		}
	case tea.KeyBackspace:
		m.editor.DeleteBeforeCursor()
	case tea.KeyDelete:
		if m.editor.Mode() == vimInsert {
			m.editor.DeleteAtCursor()
		} else {
			m.editor.Handle("x")
		}
	case tea.KeyLeft:
		m.moveCursor("left")
	case tea.KeyRight:
		m.moveCursor("right")
	case tea.KeyUp:
		m.moveCursor("up")
	case tea.KeyDown:
		m.moveCursor("down")
	case tea.KeyHome:
		m.moveCursor("home")
	case tea.KeyEnd:
		m.moveCursor("end")
	case tea.KeyRunes:
		if m.editor.Mode() == vimInsert {
			m.editor.InsertUserText(string(msg.Runes))
			return
		}
		for _, r := range msg.Runes {
			m.editor.Handle(string(r))
		}
	}
}

func (m *EditorModel) moveCursor(key string) {
	if m.editor.Mode() == vimInsert {
		m.editor.MoveInsertCursor(key)
		return
	}
	m.editor.Handle(key)
}

func (m *EditorModel) enterInsertForNewline() {
	switch m.editor.Mode() {
	case vimNormal:
		m.editor.Handle("a")
	case vimVisual:
		m.editor.Handle("c")
	}
}

func (m *EditorModel) refreshAfterEdit() {
	m.completionDismissed = false
	m.refreshCompletions()
}

func (m EditorModel) View() string {
	if m.done {
		return ""
	}
	parts := []string{m.editor.View(m.prompt, m.width, maxEditorHeight)}
	if completions := renderCompletions(m.matches, m.selected, m.width); completions != "" {
		parts = append(parts, completions)
	}
	parts = append(parts, footerStyle.Render(ansi.Truncate(m.modeHint(), m.width, "…")))
	return strings.Join(parts, "\n")
}

func (m EditorModel) modeHint() string {
	label := "-- " + m.editor.Mode().String() + " --"
	if m.editor.pending != 0 {
		label += " " + string(m.editor.pending)
	}
	switch m.editor.Mode() {
	case vimNormal:
		return label + " · i insert · v visual · y/d/c + motion · p paste · Enter send"
	case vimVisual:
		return label + " · y yank · d delete · c change · Esc normal · Enter send"
	default:
		return label + " · Enter send · Esc normal · Ctrl+J newline · Ctrl+T transcript"
	}
}

func (m EditorModel) Event() sessionrepl.InputEvent {
	return m.event
}

func (m EditorModel) Value() string {
	return m.editor.Value()
}

func (m EditorModel) Mode() vimMode {
	return m.editor.Mode()
}

func (m *EditorModel) refreshCompletions() {
	if m.editor.Mode() != vimInsert || m.completionDismissed {
		m.matches = nil
		m.selected = 0
		return
	}
	m.matches = sessionrepl.CompleteCommand(m.editor.Value(), m.cursor())
	if len(m.matches) == 0 {
		m.selected = 0
	} else if m.selected >= len(m.matches) {
		m.selected = len(m.matches) - 1
	}
}

func (m EditorModel) cursor() int {
	return m.editor.Cursor()
}

var _ tea.Model = EditorModel{}
