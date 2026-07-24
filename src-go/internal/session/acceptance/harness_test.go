package acceptance

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

const acceptanceTimeout = 8 * time.Second

type ptyHarness struct {
	t           *testing.T
	command     *exec.Cmd
	terminal    *os.File
	emulator    *vt.Emulator
	mock        *mockOpenAI
	home        string
	moduleRoot  string
	artifactDir string

	mu        sync.Mutex
	raw       bytes.Buffer
	trace     strings.Builder
	readDone  chan struct{}
	replyDone chan struct{}
	waitDone  chan error
}

func newPTYHarness(t *testing.T, width, height int) *ptyHarness {
	t.Helper()
	root := sessionModuleRoot(t)
	buildDir := t.TempDir()
	binary := filepath.Join(buildDir, "llm-session")
	build := exec.Command("go", "build", "-o", binary, "./cmd/llm-session")
	build.Dir = root
	build.Env = replaceEnvironment(os.Environ(), map[string]string{"GOCACHE": "/tmp/cli-llm-go-cache"})
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build llm-session: %v\n%s", err, output)
	}

	home := t.TempDir()
	mock := newMockOpenAI()
	command := exec.Command(binary)
	command.Dir = root
	command.Env = replaceEnvironment(os.Environ(), map[string]string{
		"HOME":            home,
		"OPENAI_API_KEY":  "test-key",
		"OPENAI_BASE_URL": mock.URL(),
		"OPENAI_MODEL":    "mock-model",
		"TERM":            "xterm-256color",
		"NO_COLOR":        "1",
	})
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
	if err != nil {
		mock.Close()
		t.Fatalf("start llm-session under PTY: %v", err)
	}

	h := &ptyHarness{
		t:           t,
		command:     command,
		terminal:    terminal,
		emulator:    vt.NewEmulator(width, height),
		mock:        mock,
		home:        home,
		moduleRoot:  root,
		artifactDir: artifactDirectory(root, t.Name()),
		readDone:    make(chan struct{}),
		replyDone:   make(chan struct{}),
		waitDone:    make(chan error, 1),
	}
	h.emulator.SetScrollbackSize(10000)
	go h.replyToTerminalQueries()
	go h.capture()
	go func() { h.waitDone <- command.Wait() }()
	t.Cleanup(h.cleanup)
	return h
}

func (h *ptyHarness) replyToTerminalQueries() {
	defer close(h.replyDone)
	buffer := make([]byte, 1024)
	for {
		n, err := h.emulator.Read(buffer)
		if n > 0 {
			_, _ = h.terminal.Write(buffer[:n])
		}
		if err != nil {
			return
		}
	}
}

func (h *ptyHarness) capture() {
	defer close(h.readDone)
	buffer := make([]byte, 4096)
	for {
		n, err := h.terminal.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			h.mu.Lock()
			_, _ = h.raw.Write(chunk)
			_, _ = h.emulator.Write(chunk)
			h.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (h *ptyHarness) Send(label string, data []byte) {
	h.t.Helper()
	h.mu.Lock()
	fmt.Fprintf(&h.trace, "%s\t%s\n", label, hex.EncodeToString(data))
	h.mu.Unlock()
	if _, err := h.terminal.Write(data); err != nil {
		h.t.Fatalf("send %s: %v", label, err)
	}
}

func (h *ptyHarness) Resize(width, height int) {
	h.t.Helper()
	if err := pty.Setsize(h.terminal, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)}); err != nil {
		h.t.Fatalf("resize PTY: %v", err)
	}
	h.mu.Lock()
	h.emulator.Resize(width, height)
	fmt.Fprintf(&h.trace, "resize\t%dx%d\n", width, height)
	h.mu.Unlock()
}

func (h *ptyHarness) WaitRawContains(text string) {
	h.t.Helper()
	h.waitUntil(func() bool { return strings.Contains(h.Raw(), text) }, "raw output containing "+fmt.Sprintf("%q", text))
}

func (h *ptyHarness) WaitRawSequenceAfter(offset int, values ...string) {
	h.t.Helper()
	h.waitUntil(func() bool {
		raw := h.Raw()
		if offset > len(raw) {
			return false
		}
		remaining := raw[offset:]
		for _, value := range values {
			index := strings.Index(remaining, value)
			if index < 0 {
				return false
			}
			remaining = remaining[index+len(value):]
		}
		return true
	}, "ordered raw output "+fmt.Sprintf("%q", values))
}

func (h *ptyHarness) WaitScreenContains(text string) {
	h.t.Helper()
	h.waitUntil(func() bool {
		screen, scrollback, _ := h.Snapshot()
		return strings.Contains(screen+"\n"+scrollback, text)
	}, "terminal screen containing "+fmt.Sprintf("%q", text))
}

func (h *ptyHarness) WaitRequestCount(count int) {
	h.t.Helper()
	h.waitUntil(func() bool { return len(h.mock.Requests()) >= count }, fmt.Sprintf("%d provider requests", count))
}

func (h *ptyHarness) WaitAltScreen(active bool) {
	h.t.Helper()
	h.waitUntil(func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.emulator.IsAltScreen() == active
	}, fmt.Sprintf("alternate screen active=%t", active))
}

func (h *ptyHarness) waitUntil(condition func() bool, description string) {
	h.t.Helper()
	deadline := time.Now().Add(acceptanceTimeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for %s\nraw tail: %q", description, tail(h.Raw(), 1200))
}

func (h *ptyHarness) Raw() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.raw.String()
}

func (h *ptyHarness) Snapshot() (screen string, scrollback string, alt bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	screen = h.emulator.String()
	lines := make([]string, 0, h.emulator.ScrollbackLen())
	for y := 0; y < h.emulator.ScrollbackLen(); y++ {
		var line strings.Builder
		for x := 0; x < h.emulator.Width(); x++ {
			cell := h.emulator.ScrollbackCellAt(x, y)
			if cell != nil {
				line.WriteString(cell.Content)
			}
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return screen, strings.Join(lines, "\n"), h.emulator.IsAltScreen()
}

func (h *ptyHarness) Exit() {
	h.t.Helper()
	h.Send("exit", []byte("/exit\r"))
	select {
	case err := <-h.waitDone:
		if err != nil {
			h.t.Fatalf("llm-session exit: %v", err)
		}
	case <-time.After(acceptanceTimeout):
		h.t.Fatal("timed out waiting for llm-session to exit")
	}
}

func (h *ptyHarness) cleanup() {
	failed := h.t.Failed()
	if h.command.Process != nil {
		_ = h.command.Process.Kill()
	}
	_ = h.terminal.Close()
	_ = h.emulator.Close()
	select {
	case <-h.readDone:
	case <-time.After(time.Second):
	}
	select {
	case <-h.replyDone:
	case <-time.After(time.Second):
	}
	if failed {
		h.writeArtifacts()
	}
	h.mock.Close()
}

func (h *ptyHarness) writeArtifacts() {
	_ = os.MkdirAll(h.artifactDir, 0o755)
	screen, scrollback, _ := h.Snapshot()
	h.mu.Lock()
	raw := append([]byte(nil), h.raw.Bytes()...)
	trace := h.trace.String()
	h.mu.Unlock()
	_ = os.WriteFile(filepath.Join(h.artifactDir, "raw.ansi"), raw, 0o600)
	_ = os.WriteFile(filepath.Join(h.artifactDir, "input.trace"), []byte(trace), 0o600)
	_ = os.WriteFile(filepath.Join(h.artifactDir, "screen.txt"), []byte(screen), 0o600)
	_ = os.WriteFile(filepath.Join(h.artifactDir, "scrollback.txt"), []byte(scrollback), 0o600)
	var requests bytes.Buffer
	for _, request := range h.mock.Requests() {
		encoded, _ := json.Marshal(request)
		requests.Write(encoded)
		requests.WriteByte('\n')
	}
	_ = os.WriteFile(filepath.Join(h.artifactDir, "requests.jsonl"), requests.Bytes(), 0o600)
	h.t.Logf("PTY failure artifacts: %s", h.artifactDir)
}

func sessionModuleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve acceptance source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func artifactDirectory(root, testName string) string {
	base := os.Getenv("SESSION_PTY_ARTIFACT_DIR")
	if base == "" {
		base = filepath.Join(root, "test-artifacts", "session-pty")
	}
	name := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(testName)
	return filepath.Join(base, name)
}

func replaceEnvironment(current []string, replacements map[string]string) []string {
	environment := make([]string, 0, len(current)+len(replacements))
	for _, entry := range current {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := replacements[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range replacements {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func tail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
