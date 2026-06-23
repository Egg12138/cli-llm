package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	einomodel "github.com/cloudwego/eino/components/model"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	sessionruntime "github.com/Egg12138/cli-llm/src-go/internal/session/runtime"
)

// ── message types ──────────────────────────────────────────────

type streamChunkMsg string
type streamFinishedMsg struct{}
type streamErrorMsg struct{ err error }
type tickMsg time.Time

// ── session config ─────────────────────────────────────────────

type SessionConfig struct {
	State       *graph.State
	Store       interface{ Append(model.Entry) error }
	Model       einomodel.BaseChatModel
	TitleModel  einomodel.BaseChatModel
	ModelName   string
	SessionName string
}

// ── SessionModel ───────────────────────────────────────────────

type SessionModel struct {
	state       *graph.State
	store       interface{ Append(model.Entry) error }
	chatModel   einomodel.BaseChatModel
	titleModel  einomodel.BaseChatModel
	modelName   string
	sessionName string

	input textInput
	vp    viewport

	width, height int
	vpHeight      int

	busy       bool
	streamCh   chan string
	streamBuf  strings.Builder
	cancel     context.CancelFunc
	statusText string
	spinnerPos int

	quitting bool
}

func NewSessionModel(cfg SessionConfig) SessionModel {
	m := SessionModel{
		state:       cfg.State,
		store:       cfg.Store,
		chatModel:   cfg.Model,
		titleModel:  cfg.TitleModel,
		modelName:   cfg.ModelName,
		sessionName: cfg.SessionName,
		width:       defaultWidth,
		height:      defaultHeight,
		vpHeight:    defaultHeight - 3,
	}
	m.vp.height = m.vpHeight
	return m
}

func (m SessionModel) Init() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ── Update ─────────────────────────────────────────────────────

func (m SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vpHeight = m.height - 3
		if m.vpHeight < 1 {
			m.vpHeight = 1
		}
		m.vp.height = m.vpHeight

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case streamChunkMsg:
		m.streamBuf.WriteString(string(msg))
		m.appendStreamContent(string(msg))
		return m, m.watchStreams()

	case streamFinishedMsg:
		m.busy = false
		m.statusText = ""
		// Re-render display from state
		m.refreshViewport()
		m.streamBuf.Reset()

	case streamErrorMsg:
		m.busy = false
		m.statusText = fmt.Sprintf("Error: %v", msg.err)
		m.vp.AppendLine(fmt.Sprintf("Error: %v", msg.err))

	case tickMsg:
		if m.busy {
			m.spinnerPos = (m.spinnerPos + 1) % len(spinnerFrames)
			label := m.statusText
			if label == "" {
				label = "Thinking"
			}
			m.statusText = fmt.Sprintf("%s %s", spinnerFrames[m.spinnerPos], label)
		}
		return m, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}

	return m, nil
}

func (m SessionModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.busy {
		if msg.Type == tea.KeyCtrlC && m.cancel != nil {
			m.cancel()
			m.cancel = nil
			m.busy = false
			m.statusText = "cancelled"
			m.vp.AppendLine("-- cancelled --")
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		line := m.input.Value()
		m.input.Reset()
		if line == "" {
			return m, nil
		}
		return m.dispatch(line)

	case tea.KeyBackspace:
		m.input.DeleteBeforeCursor()
	case tea.KeyDelete:
		m.input.DeleteAtCursor()
	case tea.KeyLeft:
		m.input.MoveLeft()
	case tea.KeyRight:
		m.input.MoveRight()
	case tea.KeyHome:
		m.input.MoveHome()
	case tea.KeyEnd:
		m.input.MoveEnd()
	case tea.KeyUp:
		m.vp.ScrollUp(1)
	case tea.KeyDown:
		m.vp.ScrollDown(1)
	case tea.KeyPgUp:
		m.vp.ScrollUp(m.vpHeight)
	case tea.KeyPgDown:
		m.vp.ScrollDown(m.vpHeight)
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEscape:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.input.Insert(r)
		}
	}

	return m, nil
}

func (m SessionModel) dispatch(line string) (tea.Model, tea.Cmd) {
	cmd := sessionrepl.ParseLine(line)

	switch cmd.Kind {
	case sessionrepl.CommandChat:
		return m.startChat(line)

	case sessionrepl.CommandExit:
		m.quitting = true
		return m, tea.Quit

	case sessionrepl.CommandTranscript:
		// Already in TUI, ignore or show help
		m.vp.AppendLine("Already in transcript view.")

	case sessionrepl.CommandBranches:
		var out strings.Builder
		if _, err := sessionrepl.ExecuteCommand(m.state, cmd, &out); err != nil {
			m.vp.AppendLine(fmt.Sprintf("Error: %v", err))
		} else {
			for _, l := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
				m.vp.AppendLine(l)
			}
		}

	default:
		// /switch, /checkpoint, /unknown
		var out strings.Builder
		if _, err := sessionrepl.ExecuteCommandWithStore(m.state, cmd, &out, m.store); err != nil {
			m.vp.AppendLine(fmt.Sprintf("Error: %v", err))
		} else {
			for _, l := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
				m.vp.AppendLine(l)
			}
			// Refresh header (branch may have changed)
		}
	}

	return m, nil
}

func (m SessionModel) startChat(input string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.streamCh = make(chan string, 128)
	m.statusText = spinnerFrames[0] + " Thinking"
	m.spinnerPos = 0
	m.vp.AppendLine("> " + input)

	ch := m.streamCh

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	go func() {
		defer close(ch)
		defer cancel()

		pw := &chanWriter{ch: ch}
		sr := &chanStatusReporter{}

		err := sessionruntime.RunChatTurn(ctx, sessionruntime.ChatTurnRequest{
			Input:       input,
			State:       m.state,
			Store:       m.store,
			Writer:      pw,
			Model:       m.chatModel,
			TitleModel:  m.titleModel,
			SessionName: m.sessionName,
			ModelName:   m.modelName,
			Status:      sr,
			Now:         time.Now,
		})
		if err != nil && err != context.Canceled {
			ch <- fmt.Sprintf("\nError: %v", err)
		}
	}()

	return m, m.watchStreams()
}

func (m SessionModel) watchStreams() tea.Cmd {
	ch := m.streamCh
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return streamFinishedMsg{}
		}
		return streamChunkMsg(chunk)
	}
}

func (m *SessionModel) appendStreamContent(s string) {
	parts := strings.Split(s, "\n")
	for i, part := range parts {
		if i > 0 {
			m.vp.AppendLine("") // newline creates a new line in viewport
		}
		if part != "" || i == len(parts)-1 {
			// Append text to the last line (or add as new line if \n was hit)
			if m.vp.LineCount() > 0 && i == 0 && part != "" {
				m.vp.AppendToLastLine(part)
			} else if part != "" {
				m.vp.AppendLine(part)
			}
		}
	}
}

func (m *SessionModel) refreshViewport() {
	entries, err := m.state.ReachableHistory()
	if err != nil {
		m.vp.AppendLine(fmt.Sprintf("Error loading history: %v", err))
		return
	}
	display, err := renderEntries(entries, renderOptions{})
	if err != nil {
		m.vp.AppendLine(fmt.Sprintf("Error rendering: %v", err))
		return
	}
	var lines []string
	for _, e := range display {
		for _, ln := range e.lines {
			lines = append(lines, ln)
		}
	}
	if len(lines) > 0 {
		m.vp.SetLines(lines)
	} else {
		m.vp.AppendLine("(no messages)")
	}
}

// ── View ────────────────────────────────────────────────────────

func (m SessionModel) View() string {
	if m.quitting {
		return ""
	}
	parts := []string{
		m.renderHeader(),
		m.vp.View(),
		m.renderInputBar(),
		m.renderStatusBar(),
	}
	return strings.Join(parts, "\n")
}

func (m SessionModel) renderHeader() string {
	left := fmt.Sprintf(" %s · %s · %s ", m.sessionName, m.state.CurrentBranch, m.modelName)
	pad := m.width - lipgloss.Width(left)
	if pad < 0 {
		pad = 0
	}
	return headerStyle.Width(m.width).Render(left + strings.Repeat(" ", pad))
}

func (m SessionModel) renderInputBar() string {
	if m.busy {
		return inputDisabledStyle.Width(m.width).Render(" (streaming... Ctrl+C to cancel) ")
	}
	view := m.input.View(m.width)
	return inputStyle.Width(m.width).Render(view)
}

func (m SessionModel) renderStatusBar() string {
	left := m.statusText
	if left == "" {
		left = "Ready"
	}
	right := m.modelName
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if pad < 1 {
		pad = 1
	}
	return statusStyle.Width(m.width).Render(" " + left + strings.Repeat(" ", pad) + right + " ")
}

// ── helper types ───────────────────────────────────────────────

type chanWriter struct {
	ch chan<- string
}

func (w *chanWriter) Write(p []byte) (int, error) {
	w.ch <- string(p)
	return len(p), nil
}

type chanStatusReporter struct{}

func (chanStatusReporter) Set(s sessionruntime.Status) {}
func (chanStatusReporter) Clear()                      {}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
