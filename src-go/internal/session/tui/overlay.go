package tui

import (
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
)

type Overlay struct{}

func NewOverlay() Overlay {
	return Overlay{}
}

func (o Overlay) Open(state *graph.State, out io.Writer) error {
	m, err := newModelFromState(state, out)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithInput(os.Stdin), tea.WithOutput(out), tea.WithAltScreen()).Run()
	return err
}

func newModelFromState(state *graph.State, clip io.Writer) (Model, error) {
	history, err := state.ReachableHistory()
	if err != nil {
		return Model{}, err
	}
	display, err := renderEntries(history, renderOptions{})
	if err != nil {
		return Model{}, err
	}
	return New(Config{
		History:  display,
		Branches: state.ListBranches(),
		Current:  state.CurrentBranch,
		Load: func(headID string) ([]displayEntry, error) {
			entries, err := state.ReachableFrom(headID)
			if err != nil {
				return nil, err
			}
			return renderEntries(entries, renderOptions{})
		},
		Clip: clip,
	}), nil
}
