package tui

import (
	"fmt"
	"io"
	"strings"

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
	if event.Err == nil && event.Kind == sessionrepl.EventLine && sessionrepl.ParseLine(event.Line).Kind == sessionrepl.CommandChat {
		if err := writeSubmittedInput(r.out, prompt, event.Line); err != nil {
			event.Err = err
		}
	}
	return event
}

func writeSubmittedInput(out io.Writer, prompt, input string) error {
	for _, line := range strings.Split(input, "\n") {
		if _, err := fmt.Fprintf(out, "%s%s\n", prompt, line); err != nil {
			return err
		}
	}
	return nil
}

var _ sessionrepl.InputEventReader = (*EditorReader)(nil)
