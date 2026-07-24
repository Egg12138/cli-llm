package acceptance

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func TestSessionPTY(t *testing.T) {
	h := newPTYHarness(t, 80, 24)
	h.WaitRawContains("Enter send")

	h.Send("help", []byte("/help\r"))
	h.WaitRawContains("Available commands:")
	for _, command := range []string{"/help", "/exit", "/transcript", "/branches", "/switch <target>", "/checkpoint <name>", "/t"} {
		if !strings.Contains(h.Raw(), command) {
			t.Fatalf("help output missing %q", command)
		}
	}
	if requests := h.mock.Requests(); len(requests) != 0 {
		t.Fatalf("help contacted provider: %#v", requests)
	}

	h.Send("completion-prefix", []byte("/he"))
	h.WaitScreenContains("/help")
	h.Send("completion-tab", []byte{'\t'})
	h.WaitScreenContains("> /help")
	h.Send("completion-submit", []byte{'\r'})
	h.waitUntil(func() bool { return strings.Count(h.Raw(), "Available commands:") >= 2 }, "completed /help execution")

	h.Send("switch-prefix-and-argument", []byte("/sw feature"))
	h.Send("cursor-to-command-token", []byte(strings.Repeat("\x1b[D", len(" feature"))))
	h.Send("switch-completion-tab", []byte{'\t'})
	h.WaitScreenContains("/switch feature")
	switchStart := len(h.Raw())
	h.Send("switch-submit", []byte{'\r'})
	h.WaitRawSequenceAfter(switchStart, "switched to feature", "Enter send")

	cjkStart := len(h.Raw())
	h.Send("cjk-spaces", []byte("你好 世界\r"))
	h.waitUntil(func() bool {
		return containsBrailleSpinner(h.Raw()[cjkStart:])
	}, "visible CJK turn spinner")
	screen, scrollback, _ := h.Snapshot()
	if terminalHistory := scrollback + "\n" + screen; !strings.Contains(terminalHistory, "你好 世界") {
		t.Fatalf("submitted input disappeared while waiting for response\nscreen:\n%s\nscrollback:\n%s", screen, scrollback)
	}
	h.WaitRawSequenceAfter(cjkStart, "CJK_ACK", "Enter send")
	h.WaitRequestCount(2)
	if users := h.mock.StreamingUsers(); len(users) != 1 || users[0] != "你好 世界" {
		t.Fatalf("streaming users after CJK turn = %#v", users)
	}

	responseStart := len(h.Raw())
	h.Send("multiline-first", []byte("第一行"))
	h.Send("multiline-newline", []byte{'\n'})
	h.Send("multiline-second-and-submit", []byte("second line\r"))
	h.WaitRawSequenceAfter(responseStart, "MULTILINE_ACK", "Enter send")
	h.WaitRequestCount(3)
	users := h.mock.StreamingUsers()
	if got := users[len(users)-1]; got != "第一行\nsecond line" {
		t.Fatalf("multiline provider input = %q", got)
	}
	screen, scrollback, _ = h.Snapshot()
	terminalHistory := scrollback + "\n" + screen
	for _, marker := range []string{"LONG-LINE-00", "LONG-LINE-18", "LONG-LINE-35", "MULTILINE_ACK"} {
		if !strings.Contains(terminalHistory, marker) {
			t.Fatalf("terminal history missing %q\nscreen:\n%s\nscrollback:\n%s", marker, screen, scrollback)
		}
	}
	responseRaw := h.Raw()[responseStart:]
	firstContent := strings.Index(responseRaw, "LONG-LINE-00")
	if firstContent < 0 {
		t.Fatal("long response first marker missing from raw ANSI")
	}
	if !strings.Contains(responseRaw[:firstContent], "\r\x1b[2K") {
		t.Fatalf("spinner did not use CR + erase before response: %q", responseRaw[:firstContent])
	}
	if containsBrailleSpinner(responseRaw[firstContent:strings.Index(responseRaw, "MULTILINE_ACK")]) {
		t.Fatalf("spinner frame appeared after response content began: %q", responseRaw[firstContent:])
	}

	resizeStart := len(h.Raw())
	h.Send("resize-input", []byte("resize 保留"))
	h.Resize(60, 16)
	h.WaitScreenContains("resize 保留")
	h.Send("resize-submit", []byte{'\r'})
	h.WaitRawSequenceAfter(resizeStart, "RESIZE_ACK", "Enter send")

	cancelStart := len(h.Raw())
	h.Send("cancel-turn", []byte("cancel me\r"))
	h.waitUntil(func() bool {
		return containsBrailleSpinner(h.Raw()[cancelStart:])
	}, "visible cancellation spinner")
	h.Send("cancel-sigint", []byte{0x03})
	h.WaitRawSequenceAfter(cancelStart, "cancelled", "Enter send")
	cancelSegment := h.Raw()[cancelStart:]
	if strings.Count(cancelSegment, "cancelled") != 1 {
		t.Fatalf("cancellation message count = %d, want 1: %q", strings.Count(cancelSegment, "cancelled"), cancelSegment)
	}

	h.Send("open-transcript", []byte{0x14})
	h.WaitAltScreen(true)
	if !strings.Contains(h.Raw(), "\x1b[?1049h") {
		t.Fatalf("overlay did not emit alternate-screen enter: %q", tail(h.Raw(), 1000))
	}
	h.WaitScreenContains("session")
	overlayCloseStart := len(h.Raw())
	h.Send("close-transcript", []byte{0x1b})
	h.WaitAltScreen(false)
	h.WaitRawSequenceAfter(overlayCloseStart, "\x1b[?1049l", "Enter send")
	h.WaitScreenContains("LONG-LINE-18")

	h.Exit()
	assertPersistedInputs(t, h.home, []string{"你好 世界", "第一行\nsecond line", "resize 保留", "cancel me"})
}

func containsBrailleSpinner(value string) bool {
	return strings.ContainsAny(value, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
}

func assertPersistedInputs(t *testing.T, home string, expected []string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(home, ".cli-llm", "sessions", "*.jsonl"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("session files = %#v, err = %v", paths, err)
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatalf("open session JSONL: %v", err)
	}
	defer file.Close()

	var users []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry model.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("decode session entry: %v", err)
		}
		if entry.Type != model.EntryTypeMessage {
			continue
		}
		message, err := entry.MessageData()
		if err != nil {
			t.Fatalf("decode message entry: %v", err)
		}
		if message.Role == "user" {
			users = append(users, message.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan session JSONL: %v", err)
	}
	for _, want := range expected {
		found := false
		for _, got := range users {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("persisted users %#v missing %q", users, want)
		}
	}
}

func TestSessionPTYHarnessHasBoundedTimeout(t *testing.T) {
	if acceptanceTimeout <= 0 || acceptanceTimeout > 30*time.Second {
		t.Fatalf("acceptance timeout = %s, want a positive bounded timeout", acceptanceTimeout)
	}
}
