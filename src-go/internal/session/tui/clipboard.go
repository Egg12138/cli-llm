package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) copySelection() tea.Cmd {
	if len(m.cfg.History) == 0 || m.cfg.Clip == nil {
		return nil
	}
	raw := m.cfg.History[m.clamp(m.cursor)].raw
	return func() tea.Msg {
		_, _ = fmt.Fprint(m.cfg.Clip, ansi.SetClipboard(ansi.SystemClipboard, raw))
		return nil
	}
}
