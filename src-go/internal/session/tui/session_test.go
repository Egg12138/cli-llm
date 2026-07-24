package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
)

func cast(m tea.Model) SessionModel {
	return m.(SessionModel)
}

func TestSessionModelInitialView(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State:       graph.NewState(nil),
		SessionName: "test-session",
		ModelName:   "test-model",
	})
	v := m.View()

	if !strings.Contains(v, "test-session") {
		t.Fatalf("header missing session name: %q", v)
	}
	if !strings.Contains(v, "main") {
		t.Fatalf("header missing branch 'main': %q", v)
	}
	if !strings.Contains(v, "test-model") {
		t.Fatalf("status bar missing model name: %q", v)
	}
}

func TestSessionModelTextInput(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m = cast(next)
	if cmd != nil {
		t.Fatalf("expected nil cmd after typing, got %v", cmd)
	}
	if m.input.Value() != "hello" {
		t.Fatalf("input = %q, want 'hello'", m.input.Value())
	}

	next, _ = m.updateFromKey(tea.KeyBackspace)
	m = cast(next)
	next, _ = m.updateFromKey(tea.KeyBackspace)
	m = cast(next)
	if m.input.Value() != "hel" {
		t.Fatalf("after 2 backspaces, input = %q, want 'hel'", m.input.Value())
	}
}

func TestSessionModelExit(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	next, _ := m.updateFromString("/exit")
	m = cast(next)
	if !m.quitting {
		t.Fatal("expected quitting after /exit")
	}
}

func TestSessionModelCtrlC(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = cast(next)
	if !m.quitting {
		t.Fatal("expected quitting after Ctrl+C")
	}
}

func TestSessionModelStreamChunkAppendsToViewport(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	m.streamCh = make(chan string, 10)

	next, cmd := m.Update(streamChunkMsg("Hello"))
	m = cast(next)
	if cmd == nil {
		t.Fatalf("expected non-nil cmd after stream chunk")
	}
	v := m.View()
	if !strings.Contains(v, "Hello") {
		t.Fatalf("viewport missing chunk: %q", v)
	}
}

func TestSessionModelStreamMultipleChunks(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	m.streamCh = make(chan string, 10)

	for _, s := range []string{"Hello ", "world", "!"} {
		var next tea.Model
		next, _ = m.Update(streamChunkMsg(s))
		m = cast(next)
	}
	v := m.View()
	if !strings.Contains(v, "Hello world!") {
		t.Fatalf("viewport missing all chunks: %q", v)
	}
}

func TestSessionModelStreamFinishedClearsStatus(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	m.busy = true
	m.streamBuf.WriteString("done")

	next, _ := m.Update(streamFinishedMsg{})
	m = cast(next)
	if m.busy {
		t.Fatal("expected busy=false after stream finished")
	}
}

func TestSessionModelWindowResize(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})

	if m.width != defaultWidth || m.height != defaultHeight {
		t.Fatalf("default: %dx%d, want %dx%d", m.width, m.height, defaultWidth, defaultHeight)
	}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = cast(next)
	if m.width != 100 || m.height != 30 {
		t.Fatalf("after resize: %dx%d, want 100x30", m.width, m.height)
	}
	if m.vpHeight != 27 {
		t.Fatalf("vpHeight = %d, want 27", m.vpHeight)
	}
}

func TestSessionModelSpaceKey(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	m = cast(next)
	if cmd != nil {
		t.Fatalf("expected nil cmd after typing, got %v", cmd)
	}
	next, _ = m.updateFromKey(tea.KeySpace)
	m = cast(next)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")})
	m = cast(next)
	if m.input.Value() != "hello world" {
		t.Fatalf("input = %q, want 'hello world'", m.input.Value())
	}
}

func TestSessionModelCtrlJInsertsNewline(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line1")})
	m = cast(next)
	next, _ = m.updateFromKey(tea.KeyCtrlJ)
	m = cast(next)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line2")})
	m = cast(next)
	if m.input.Value() != "line1\nline2" {
		t.Fatalf("input = %q, want 'line1\\nline2'", m.input.Value())
	}
	if m.input.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", m.input.LineCount())
	}
}

func TestSessionModelSpinnerDoesNotAccumulate(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	m.busy = true
	m.statusLabel = "Thinking"
	m.statusText = spinnerFrames[0] + " Thinking"

	// Simulate several ticks
	for i := 0; i < 5; i++ {
		next, _ := m.Update(tickMsg{})
		m = cast(next)
	}

	// statusText should only have one spinner frame + label, not accumulated frames
	if !strings.Contains(m.statusText, "Thinking") {
		t.Fatalf("statusText missing 'Thinking': %q", m.statusText)
	}
	// Count how many spinner frames appear in statusText
	frameCount := 0
	for _, f := range spinnerFrames {
		frameCount += strings.Count(m.statusText, f)
	}
	if frameCount != 1 {
		t.Fatalf("statusText has %d spinner frames, want 1: %q", frameCount, m.statusText)
	}
}

func TestSessionModelBranchesCommand(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	next, _ := m.updateFromString("/branches")
	m = cast(next)
	v := m.View()
	if !strings.Contains(v, "main") {
		t.Fatalf("branches output missing 'main': %q", v)
	}
}

func TestSessionModelViewHasLayout(t *testing.T) {
	m := NewSessionModel(SessionConfig{
		State: graph.NewState(nil),
	})
	v := m.View()

	if !strings.Contains(v, "Ready") {
		t.Fatalf("status bar missing 'Ready': %q", v)
	}
	lines := strings.Split(v, "\n")
	if len(lines) < 3 {
		t.Fatalf("view has %d lines, want at least 3 (header/input/status)", len(lines))
	}
}

// ── helpers ────────────────────────────────────────────────────

func (m SessionModel) updateFromKey(kt tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: kt})
}

func (m SessionModel) updateFromString(s string) (tea.Model, tea.Cmd) {
	for _, r := range s {
		var next tea.Model
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = cast(next)
	}
	return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func init() {
	var _ tea.Model = SessionModel{}
}
