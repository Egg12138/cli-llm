package tui

import (
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
)

type Config struct {
	History       []displayEntry
	Branches      []graph.Branch
	Current       string
	Load          func(headID string) ([]displayEntry, error)
	Clip          io.Writer
	Width, Height int
}

type Model struct {
	cfg      Config
	cursor   int
	quitting bool
}

func New(cfg Config) Model {
	return Model{cfg: cfg}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.cfg.Width = msg.Width
		m.cfg.Height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.cursor = m.clamp(m.cursor - 1)
	case tea.KeyDown:
		m.cursor = m.clamp(m.cursor + 1)
	case tea.KeyPgUp:
		m.cursor = m.clamp(m.cursor - m.page())
	case tea.KeyPgDown:
		m.cursor = m.clamp(m.cursor + m.page())
	case tea.KeyHome:
		m.cursor = 0
	case tea.KeyEnd:
		m.cursor = m.clamp(len(m.cfg.History) - 1)
	case tea.KeyEsc, tea.KeyCtrlT:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) clamp(i int) int {
	if len(m.cfg.History) == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if last := len(m.cfg.History) - 1; i > last {
		return last
	}
	return i
}

func (m Model) bodyHeight() int {
	h := m.cfg.Height - headerLines - footerLines
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) page() int {
	if p := m.bodyHeight() / 3; p > 1 {
		return p
	}
	return 1
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')
	b.WriteString(m.renderBody())
	b.WriteByte('\n')
	b.WriteString(footerStyle.Render(footerHint))
	return b.String()
}

func (m Model) renderHeader() string {
	title := "session"
	if m.cfg.Current != "" {
		title += " · " + m.cfg.Current
	}
	return headerStyle.Render(title)
}

// renderBody flattens History to lines, derives a scroll offset that keeps the
// cursor entry visible, and renders the visible slice with the cursor entry
// highlighted.
func (m Model) renderBody() string {
	bodyHeight := m.bodyHeight()
	var lines []string
	cursorStart, cursorEnd := -1, -1
	for i, e := range m.cfg.History {
		if i == m.cursor {
			cursorStart = len(lines)
		}
		for _, ln := range e.lines {
			style := roleStyles[e.role]
			rendered := style.Render(ln)
			if i == m.cursor {
				rendered = selectedStyle.Render("> " + ln)
			}
			lines = append(lines, rendered)
		}
		if i == m.cursor {
			cursorEnd = len(lines)
		}
	}

	total := len(lines)
	offset := m.scrollOffset(cursorStart, cursorEnd, bodyHeight)
	offset = clampOffset(offset, maxOffset(total, bodyHeight))

	end := offset + bodyHeight
	if end > total {
		end = total
	}
	if offset > total {
		offset = total
	}
	return strings.Join(lines[offset:end], "\n")
}

// scrollOffset returns the top line offset so the cursor entry [start,end) is
// visible in a window of bodyHeight lines. A too-tall entry pins its first line.
func (m Model) scrollOffset(start, end, bodyHeight int) int {
	if start < 0 {
		return 0
	}
	offset := 0
	if end > bodyHeight {
		offset = end - bodyHeight // scroll down to reveal entry's last line
	}
	if start < offset {
		offset = start // entry taller than window, or above: pin first line
	}
	return offset
}

func maxOffset(total, h int) int {
	if total-h < 0 {
		return 0
	}
	return total - h
}

func clampOffset(o, max int) int {
	if o < 0 {
		return 0
	}
	if o > max {
		return max
	}
	return o
}
