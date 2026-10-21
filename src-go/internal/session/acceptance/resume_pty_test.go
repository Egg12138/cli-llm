package acceptance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func TestSessionResumePickerPTY(t *testing.T) {
	h := newPTYHarnessWith(t, 100, 30, ptyHarnessOptions{
		args: []string{"--resume"},
		setupHome: func(t *testing.T, home string) {
			seedResumeSession(t, home, "alpha", "ALPHA_HISTORY", time.Unix(2000, 0))
			seedResumeSession(t, home, "beta", "BETA_HISTORY", time.Unix(1000, 0))
		},
	})

	h.WaitRawContains("1. alpha")
	h.WaitRawContains("2. beta")
	h.WaitRawContains("Resume which session? [1-2, name, Enter to cancel]: ")

	h.Send("resume-invalid-number", []byte("9\r"))
	h.WaitRawContains("No session 9; choose a number between 1 and 2.")
	if strings.Contains(h.Raw(), "Enter send") {
		t.Fatalf("picker started the session before a valid selection: %q", h.Raw())
	}
	if requests := h.mock.Requests(); len(requests) != 0 {
		t.Fatalf("picker selection failure contacted the provider: %#v", requests)
	}

	resumeStart := len(h.Raw())
	h.Send("resume-select-second", []byte("2\r"))
	h.WaitRawSequenceAfter(resumeStart, "Enter send")

	turnStart := len(h.Raw())
	h.Send("resume-turn", []byte("after resume\r"))
	h.WaitRawSequenceAfter(turnStart, "after resume", "ACK", "Enter send")

	requests := h.mock.Requests()
	var resumed *mockRequest
	for i := range requests {
		if requests[i].Stream {
			resumed = &requests[i]
		}
	}
	if resumed == nil {
		t.Fatalf("resumed session made no streaming request: %#v", requests)
	}
	if got := lastUserMessage(resumed.Messages); got != "after resume" {
		t.Fatalf("provider last user message = %q, want %q", got, "after resume")
	}
	context := make([]string, 0, len(resumed.Messages))
	for _, message := range resumed.Messages {
		context = append(context, message.Content)
	}
	joined := strings.Join(context, "\n")
	if !strings.Contains(joined, "BETA_HISTORY") {
		t.Fatalf("resumed context missing the selected session history: %q", joined)
	}
	if strings.Contains(joined, "ALPHA_HISTORY") {
		t.Fatalf("resumed context leaked the unselected session history: %q", joined)
	}

	h.Exit()
	if !strings.Contains(readSessionFile(t, h.home, "beta"), "after resume") {
		t.Fatalf("selected session did not persist the new turn: %s", readSessionFile(t, h.home, "beta"))
	}
	if strings.Contains(readSessionFile(t, h.home, "alpha"), "after resume") {
		t.Fatalf("unselected session received the new turn: %s", readSessionFile(t, h.home, "alpha"))
	}
}

func TestSessionResumePickerCancelPTY(t *testing.T) {
	h := newPTYHarnessWith(t, 100, 30, ptyHarnessOptions{
		args: []string{"--resume"},
		setupHome: func(t *testing.T, home string) {
			seedResumeSession(t, home, "alpha", "ALPHA_HISTORY", time.Unix(1000, 0))
		},
	})

	h.WaitRawContains("Resume which session? [1-1, name, Enter to cancel]: ")
	h.Send("resume-cancel", []byte("\r"))
	select {
	case err := <-h.waitDone:
		if err != nil {
			t.Fatalf("cancelled resume exit: %v", err)
		}
	case <-time.After(acceptanceTimeout):
		t.Fatal("timed out waiting for a cancelled resume to exit")
	}
	if strings.Contains(h.Raw(), "Enter send") {
		t.Fatalf("cancelled resume started a session: %q", h.Raw())
	}
	if requests := h.mock.Requests(); len(requests) != 0 {
		t.Fatalf("cancelled resume contacted the provider: %#v", requests)
	}
}

func TestSessionResumeSkippedLinesPTY(t *testing.T) {
	h := newPTYHarnessWith(t, 100, 30, ptyHarnessOptions{
		args: []string{"--resume", "gamma"},
		setupHome: func(t *testing.T, home string) {
			seedResumeSession(t, home, "gamma", "GAMMA_HISTORY", time.Unix(5000, 0))
			path := filepath.Join(home, ".cli-llm", "sessions", "gamma.jsonl")
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				t.Fatalf("open gamma session: %v", err)
			}
			if _, err := file.WriteString("{\"id\":\"torn\"\n"); err != nil {
				_ = file.Close()
				t.Fatalf("write torn line: %v", err)
			}
			if err := file.Close(); err != nil {
				t.Fatalf("close gamma session: %v", err)
			}
		},
	})

	h.WaitRawContains("skipped unreadable line(s) [5]")
	h.WaitRawSequenceAfter(0, "resumed gamma (4 entries)", "Enter send")

	turnStart := len(h.Raw())
	h.Send("resumed-turn", []byte("after torn\r"))
	h.WaitRawSequenceAfter(turnStart, "after torn", "ACK", "Enter send")

	h.Exit()
	if !strings.Contains(readSessionFile(t, h.home, "gamma"), "after torn") {
		t.Fatalf("resumed session did not persist the new turn: %s", readSessionFile(t, h.home, "gamma"))
	}
}

func seedResumeSession(t *testing.T, home, name, history string, modified time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	info, err := model.NewSessionInfo("", model.SessionInfo{Title: name, Created: now, Model: "mock-model"}, now)
	if err != nil {
		t.Fatalf("create session info: %v", err)
	}
	question, err := model.NewMessage(info.ID, "user", name+" question", now)
	if err != nil {
		t.Fatalf("create user message: %v", err)
	}
	answer, err := model.NewMessage(question.ID, "assistant", history, now)
	if err != nil {
		t.Fatalf("create assistant message: %v", err)
	}
	checkpoint, err := model.NewCheckpoint(answer.ID, "main", answer.ID, now)
	if err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}

	dir := filepath.Join(home, ".cli-llm", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create session dir: %v", err)
	}
	var content bytes.Buffer
	for _, entry := range []model.Entry{info, question, answer, checkpoint} {
		encoded, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("encode session entry: %v", err)
		}
		content.Write(encoded)
		content.WriteByte('\n')
	}
	path := filepath.Join(dir, name+".jsonl")
	if err := os.WriteFile(path, content.Bytes(), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatalf("set session mtime: %v", err)
	}
}

func readSessionFile(t *testing.T, home, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(home, ".cli-llm", "sessions", name+".jsonl"))
	if err != nil {
		t.Fatalf("read %s session: %v", name, err)
	}
	return string(content)
}
