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
	cfg       Config
	cursor    int
	mode      mode
	branchSel int
	quitting  bool
}

type mode int

const (
	modeScroll mode = iota
	modeBranches
)

func New(cfg Config) Model {
	if cfg.Width <= 0 {
		cfg.Width = defaultWidth
	}
	if cfg.Height <= 0 {
		cfg.Height = defaultHeight
	}
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
	if msg.Type == tea.KeyTab {
		m.toggleBranchPanel()
		return m, nil
	}
	if m.mode == modeBranches {
		return m.handleBranchKey(msg)
	}
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
		if len(msg.Runes) == 1 && msg.Runes[0] == 'y' {
			return m, m.copySelection()
		}
	}
	return m, nil
}

func (m *Model) toggleBranchPanel() {
	if m.mode == modeBranches {
		m.mode = modeScroll
		return
	}
	m.mode = modeBranches
	m.branchSel = m.selectedCurrentBranch()
}

func (m Model) handleBranchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.branchSel = m.clampBranch(m.branchSel - 1)
	case tea.KeyDown:
		m.branchSel = m.clampBranch(m.branchSel + 1)
	case tea.KeyHome:
		m.branchSel = 0
	case tea.KeyEnd:
		m.branchSel = m.clampBranch(len(m.cfg.Branches) - 1)
	case tea.KeyEnter:
		m.previewSelectedBranch()
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

func (m *Model) previewSelectedBranch() {
	if len(m.cfg.Branches) == 0 || m.cfg.Load == nil {
		m.mode = modeScroll
		return
	}
	branch := m.cfg.Branches[m.clampBranch(m.branchSel)]
	history, err := m.cfg.Load(branch.HeadID)
	if err == nil {
		m.cfg.History = history
		m.cfg.Current = branch.Name
		m.cursor = m.clamp(0)
	}
	m.mode = modeScroll
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

func (m Model) clampBranch(i int) int {
	if len(m.cfg.Branches) == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if last := len(m.cfg.Branches) - 1; i > last {
		return last
	}
	return i
}

func (m Model) selectedCurrentBranch() int {
	for i, branch := range m.cfg.Branches {
		if branch.Name == m.cfg.Current {
			return i
		}
	}
	return 0
}

func (m Model) bodyHeight() int {
	h := m.cfg.Height - headerLines - footerLines
	return max(h, 1)
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
	if m.mode == modeBranches {
		b.WriteByte('\n')
		b.WriteString(m.renderBranchPanel())
	}
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

	end := min(offset+bodyHeight, total)
	return strings.Join(lines[offset:end], "\n")
}

func (m Model) renderBranchPanel() string {
	if len(m.cfg.Branches) == 0 {
		return panelStyle.Render("branches\n(no branches)")
	}
	lines := []string{"branches"}
	for i, branch := range m.cfg.Branches {
		marker := "  "
		if branch.Name == m.cfg.Current {
			marker = "* "
		}
		line := marker + branch.Name
		if i == m.branchSel {
			line = selectedStyle.Render("> " + line)
		}
		lines = append(lines, line)
	}
	return panelStyle.Render(strings.Join(lines, "\n"))
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
	return max(0, total-h)
}

func clampOffset(o, mx int) int {
	return max(0, min(o, mx))
}
