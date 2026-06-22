package tui

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func keyMsg(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }
func runeKey(r rune) tea.KeyMsg       { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
func tabKey() tea.KeyMsg              { return tea.KeyMsg{Type: tea.KeyTab} }
func enterKey() tea.KeyMsg            { return tea.KeyMsg{Type: tea.KeyEnter} }

func entries(n int) []displayEntry {
	out := make([]displayEntry, n)
	for i := 0; i < n; i++ {
		txt := "ENTRY" + string(rune('A'+i))
		out[i] = displayEntry{role: "user", raw: txt, lines: []string{"user:", txt}}
	}
	return out
}

func send(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestModelBounds(t *testing.T) {
	m := New(Config{History: entries(5), Width: 80, Height: 10})
	if m.cursor != 0 {
		t.Fatalf("fresh cursor = %d, want 0", m.cursor)
	}
	m, _ = send(m, keyMsg(tea.KeyUp))
	if m.cursor != 0 {
		t.Fatalf("Up below 0: cursor = %d, want 0", m.cursor)
	}
	m, _ = send(m, keyMsg(tea.KeyEnd))
	if m.cursor != 4 {
		t.Fatalf("End cursor = %d, want 4", m.cursor)
	}
	m, _ = send(m, keyMsg(tea.KeyDown))
	if m.cursor != 4 {
		t.Fatalf("Down past last: cursor = %d, want 4", m.cursor)
	}
	m, _ = send(m, keyMsg(tea.KeyHome))
	if m.cursor != 0 {
		t.Fatalf("Home cursor = %d, want 0", m.cursor)
	}
}

func TestModelDownUp(t *testing.T) {
	m := New(Config{History: entries(5), Width: 80, Height: 10})
	m, _ = send(m, keyMsg(tea.KeyDown))
	if m.cursor != 1 {
		t.Fatalf("Down cursor = %d, want 1", m.cursor)
	}
	m, _ = send(m, keyMsg(tea.KeyUp))
	if m.cursor != 0 {
		t.Fatalf("Up cursor = %d, want 0", m.cursor)
	}
}

func TestModelPaging(t *testing.T) {
	m := New(Config{History: entries(20), Width: 80, Height: 10})
	page := m.page()
	if page < 1 {
		t.Fatalf("page = %d, want >= 1", page)
	}
	m, _ = send(m, keyMsg(tea.KeyPgDown))
	if m.cursor != page {
		t.Fatalf("PgDn cursor = %d, want %d", m.cursor, page)
	}
	m, _ = send(m, keyMsg(tea.KeyEnd))
	m, _ = send(m, keyMsg(tea.KeyPgDown))
	if m.cursor != 19 {
		t.Fatalf("PgDn clamp cursor = %d, want 19", m.cursor)
	}
	m, _ = send(m, keyMsg(tea.KeyHome))
	m, _ = send(m, keyMsg(tea.KeyPgUp))
	if m.cursor != 0 {
		t.Fatalf("PgUp clamp cursor = %d, want 0", m.cursor)
	}
}

func TestModelEmptyHistory(t *testing.T) {
	m := New(Config{History: nil, Width: 80, Height: 10})
	m, _ = send(m, keyMsg(tea.KeyDown))
	if m.cursor != 0 {
		t.Fatalf("empty history nav: cursor = %d, want 0", m.cursor)
	}
	_ = m.View()
}

func TestModelLayout(t *testing.T) {
	m := New(Config{History: entries(3), Current: "main", Width: 80, Height: 10})
	v := m.View()
	if !strings.Contains(v, "session") {
		t.Fatalf("View missing header 'session': %q", v)
	}
	if !strings.Contains(v, "main") {
		t.Fatalf("View missing current branch 'main'")
	}
	if !strings.Contains(v, "ENTRYA") {
		t.Fatalf("View missing selected entry text 'ENTRYA'")
	}
	if !strings.Contains(v, "quit") {
		t.Fatalf("View missing footer hint 'quit'")
	}
	bodyHeight := m.bodyHeight()
	body := m.renderBody()
	got := len(strings.Split(body, "\n"))
	if got > bodyHeight {
		t.Fatalf("body line count = %d, want <= %d", got, bodyHeight)
	}
}

func TestModelScrollKeepsSelectionVisible(t *testing.T) {
	m := New(Config{History: entries(20), Width: 80, Height: 8})
	m, _ = send(m, keyMsg(tea.KeyEnd))
	v := m.View()
	last := "ENTRY" + string(rune('A'+19))
	if !strings.Contains(v, last) {
		t.Fatalf("last entry %q not visible after End: %q", last, v)
	}
	if strings.Contains(v, "ENTRYA") {
		t.Fatalf("first entry should have scrolled off, but found ENTRYA")
	}
}

func TestModelTallEntryPinsFirstLine(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "LINE" + string(rune('A'+i))
	}
	tall := displayEntry{role: "user", raw: "tall", lines: lines}
	m := New(Config{History: []displayEntry{tall}, Width: 80, Height: 6})
	body := m.renderBody()
	if !strings.Contains(body, lines[0]) {
		t.Fatalf("renderBody missing first line %q: %q", lines[0], body)
	}
	bodyHeight := m.bodyHeight()
	got := len(strings.Split(body, "\n"))
	if got > bodyHeight {
		t.Fatalf("body line count = %d, want <= %d", got, bodyHeight)
	}
}

func TestModelQuit(t *testing.T) {
	for _, msg := range []tea.Msg{keyMsg(tea.KeyEsc), keyMsg(tea.KeyCtrlT), runeKey('q')} {
		m := New(Config{History: entries(3), Width: 80, Height: 10})
		next, cmd := send(m, msg)
		if cmd == nil {
			t.Fatalf("quit msg %v returned nil cmd", msg)
		}
		if !next.quitting {
			t.Fatalf("quit msg %v: model not quitting", msg)
		}
		if cmd() != tea.Quit() {
			t.Fatalf("quit msg %v: cmd did not produce QuitMsg", msg)
		}
	}
}

func TestModelBranchPanelPreviewIsSwitchIsolated(t *testing.T) {
	loads := make([]string, 0, 1)
	m := New(Config{
		History: entries(1),
		Branches: []graph.Branch{
			{Name: "main", HeadID: "main-head"},
			{Name: "explore", HeadID: "explore-head", Parent: "main"},
		},
		Current: "main",
		Width:   80,
		Height:  10,
		Load: func(headID string) ([]displayEntry, error) {
			loads = append(loads, headID)
			return []displayEntry{{role: "assistant", raw: "explore-only", lines: []string{"assistant:", "explore-only"}}}, nil
		},
	})

	m, _ = send(m, tabKey())
	panel := m.View()
	if !strings.Contains(panel, "main") || !strings.Contains(panel, "explore") {
		t.Fatalf("branch panel missing branch names: %q", panel)
	}
	if !strings.Contains(panel, "* main") {
		t.Fatalf("branch panel did not mark current branch: %q", panel)
	}

	m, _ = send(m, keyMsg(tea.KeyDown))
	m, _ = send(m, enterKey())
	view := m.View()
	if len(loads) != 1 || loads[0] != "explore-head" {
		t.Fatalf("Load calls = %#v, want [explore-head]", loads)
	}
	if !strings.Contains(view, "explore-only") {
		t.Fatalf("preview missing explore content: %q", view)
	}
	if strings.Contains(view, "ENTRYA") {
		t.Fatalf("preview should hide main-only content, view: %q", view)
	}
}

func TestModelClipboardCopiesSelectedRawDialog(t *testing.T) {
	var clip bytes.Buffer
	m := New(Config{
		History: []displayEntry{
			{role: "user", raw: "first raw", lines: []string{"user:", "first visible"}},
			{role: "assistant", raw: "second **raw**", lines: []string{"assistant:", "second visible"}},
		},
		Clip:   &clip,
		Width:  80,
		Height: 10,
	})

	m, _ = send(m, keyMsg(tea.KeyDown))
	_, cmd := send(m, runeKey('y'))
	if cmd == nil {
		t.Fatalf("copy key returned nil command")
	}
	_ = cmd()

	out := clip.String()
	if !strings.Contains(out, "\x1b]52;c;") {
		t.Fatalf("clipboard output missing OSC 52 system clipboard prefix: %q", out)
	}
	want := base64.StdEncoding.EncodeToString([]byte("second **raw**"))
	if !strings.Contains(out, want) {
		t.Fatalf("clipboard output missing selected raw base64 %q: %q", want, out)
	}
	if strings.Contains(out, base64.StdEncoding.EncodeToString([]byte("first raw"))) {
		t.Fatalf("clipboard output copied unselected entry: %q", out)
	}
}
