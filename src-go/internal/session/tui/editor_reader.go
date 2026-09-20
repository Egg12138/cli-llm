package tui

import (
	"fmt"
	"io"

	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	tea "github.com/charmbracelet/bubbletea"
)

type EditorReader struct {
	in  io.Reader
	out io.Writer
}

func NewEditorReader(in io.Reader, out io.Writer) *EditorReader {
	return &EditorReader{in: in, out: out}
}

func (r *EditorReader) ReadLine(prompt string) (string, error) {
	event := r.ReadEvent(prompt)
	return event.Line, event.Err
}

func (r *EditorReader) ReadEvent(prompt string) sessionrepl.InputEvent {
	program := tea.NewProgram(
		NewEditorModel(prompt),
		tea.WithInput(r.in),
		tea.WithOutput(r.out),
	)
	result, err := program.Run()
	if err != nil {
		return sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Err: err}
	}
	editor, ok := result.(EditorModel)
	if !ok {
		return sessionrepl.InputEvent{Kind: sessionrepl.EventLine, Err: fmt.Errorf("unexpected editor model %T", result)}
	}
	event := editor.Event()
	return event
}

var _ sessionrepl.InputEventReader = (*EditorReader)(nil)
