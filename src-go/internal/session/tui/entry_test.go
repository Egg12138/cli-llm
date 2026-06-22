package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func mustMessage(t *testing.T, parentID, role, content string) model.Entry {
	t.Helper()
	e, err := model.NewMessage(parentID, role, content, time.Now())
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	return e
}

func mustCheckpoint(t *testing.T, parentID, name, returnTo string) model.Entry {
	t.Helper()
	e, err := model.NewCheckpoint(parentID, name, returnTo, time.Now())
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	return e
}

func TestRenderEntriesSkipsNonMessages(t *testing.T) {
	entries := []model.Entry{
		mustMessage(t, "", "user", "hello"),
		mustCheckpoint(t, "", "cp", ""),
		mustMessage(t, "", "assistant", "hi there"),
	}

	got, err := renderEntries(entries, renderOptions{})
	if err != nil {
		t.Fatalf("renderEntries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 display entries, got %d", len(got))
	}
	if got[0].role != "user" || got[0].raw != "hello" {
		t.Errorf("entry 0 mismatch: role=%q raw=%q", got[0].role, got[0].raw)
	}
	if got[1].role != "assistant" || got[1].raw != "hi there" {
		t.Errorf("entry 1 mismatch: role=%q raw=%q", got[1].role, got[1].raw)
	}
}

func TestRenderEntriesRolesAppearInLines(t *testing.T) {
	entries := []model.Entry{
		mustMessage(t, "", "user", "ask"),
		mustMessage(t, "", "assistant", "answer"),
	}
	got, err := renderEntries(entries, renderOptions{})
	if err != nil {
		t.Fatalf("renderEntries: %v", err)
	}
	joined0 := strings.Join(got[0].lines, "\n")
	joined1 := strings.Join(got[1].lines, "\n")
	if !strings.Contains(joined0, "user") {
		t.Errorf("user role not in lines: %q", joined0)
	}
	if !strings.Contains(joined1, "assistant") {
		t.Errorf("assistant role not in lines: %q", joined1)
	}
	if joined0 == joined1 {
		t.Errorf("user and assistant rendered identically: %q", joined0)
	}
}

func TestRenderEntriesPreservesMultilineContent(t *testing.T) {
	content := "line one\nline two\nline three"
	entries := []model.Entry{mustMessage(t, "", "user", content)}
	got, err := renderEntries(entries, renderOptions{})
	if err != nil {
		t.Fatalf("renderEntries: %v", err)
	}
	joined := strings.Join(got[0].lines, "\n")
	for _, want := range []string{"line one", "line two", "line three"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing content line %q in %q", want, joined)
		}
	}
}

func TestRenderEntriesRawIsVerbatim(t *testing.T) {
	content := "  weird\tspacing\n## markdown\n\n- item  "
	entries := []model.Entry{mustMessage(t, "", "assistant", content)}
	got, err := renderEntries(entries, renderOptions{})
	if err != nil {
		t.Fatalf("renderEntries: %v", err)
	}
	if got[0].raw != content {
		t.Errorf("raw not byte-identical:\nwant %q\ngot  %q", content, got[0].raw)
	}
}
