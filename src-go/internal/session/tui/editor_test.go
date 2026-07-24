package tui

import (
	"io"
	"strings"
	"testing"

	sessionrepl "github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorPreservesSpacesAndCJKOnSubmit(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("你好")})
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeySpace})
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world 世界")})
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	event := m.Event()
	if event.Kind != sessionrepl.EventLine || event.Line != "你好 world 世界" || event.Err != nil {
		t.Fatalf("submitted event = %#v, want exact unicode text", event)
	}
}

func TestEditorCtrlJInsertsNewlineAndEnterSubmits(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("line 1"))
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = updateEditor(t, m, keyRunes("第二行"))
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.Event().Line; got != "line 1\n第二行" {
		t.Fatalf("submitted line = %q, want exact multiline text", got)
	}
}

func TestEditorShowsAndAppliesCommandCompletion(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("/he"))
	if view := m.View(); !strings.Contains(view, "/help") || !strings.Contains(view, "List enabled commands") {
		t.Fatalf("completion view missing help candidate:\n%s", view)
	}

	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got := m.Value(); got != "/help" {
		t.Fatalf("completed editor value = %q, want /help", got)
	}
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.Event().Line; got != "/help" {
		t.Fatalf("submitted completion = %q, want /help", got)
	}
}

func TestEditorCompletionPreservesArguments(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("/sw  功能 分支"))
	for range []rune("  功能 分支") {
		m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	}
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyTab})

	if got := m.Value(); got != "/switch  功能 分支" {
		t.Fatalf("completed value = %q, want exact arguments", got)
	}
}

func TestEditorResizeReflowsWithoutChangingValue(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("这是一个用于验证终端宽度变化时输入内容保持不变的长句子"))
	m = updateEditor(t, m, tea.WindowSizeMsg{Width: 60, Height: 16})

	if got := m.Value(); got != "这是一个用于验证终端宽度变化时输入内容保持不变的长句子" {
		t.Fatalf("resize changed value: %q", got)
	}
	for _, line := range strings.Split(m.View(), "\n") {
		if got := ansi.StringWidth(line); got > 60 {
			t.Fatalf("resized editor line width = %d, want <= 60: %q", got, line)
		}
	}
}

func TestEditorControlEvents(t *testing.T) {
	t.Run("transcript", func(t *testing.T) {
		m := NewEditorModel("> ")
		m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyCtrlT})
		if event := m.Event(); event.Kind != sessionrepl.EventTranscript || event.Err != nil {
			t.Fatalf("Ctrl+T event = %#v", event)
		}
	})

	t.Run("eof", func(t *testing.T) {
		m := NewEditorModel("> ")
		m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
		if event := m.Event(); event.Kind != sessionrepl.EventLine || event.Err != io.EOF {
			t.Fatalf("Ctrl+D event = %#v", event)
		}
	})
}

func TestEditorVimNormalChangeVisualYankAndPaste(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("alpha beta"))
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Mode() != vimNormal {
		t.Fatalf("mode after Esc = %s, want NORMAL", m.Mode())
	}

	m = updateEditor(t, m, keyRunes("b"))
	m = updateEditor(t, m, keyRunes("c"))
	m = updateEditor(t, m, keyRunes("w"))
	if m.Mode() != vimInsert || m.Value() != "alpha " {
		t.Fatalf("after bcw mode=%s value=%q", m.Mode(), m.Value())
	}
	m = updateEditor(t, m, keyRunes("gamma"))
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = updateEditor(t, m, keyRunes("0"))
	m = updateEditor(t, m, keyRunes("v"))
	m = updateEditor(t, m, keyRunes("e"))
	if m.Mode() != vimVisual {
		t.Fatalf("mode after v = %s, want VISUAL", m.Mode())
	}
	m = updateEditor(t, m, keyRunes("y"))
	m = updateEditor(t, m, keyRunes("$"))
	m = updateEditor(t, m, keyRunes("p"))

	if got := m.Value(); got != "alpha gammaalpha" {
		t.Fatalf("edited value = %q, want %q", got, "alpha gammaalpha")
	}
}

func TestEditorShowsModeAndVisualSelection(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("你好 world"))
	if view := m.View(); !strings.Contains(view, "-- INSERT --") {
		t.Fatalf("insert view missing mode label:\n%s", view)
	}

	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if view := m.View(); !strings.Contains(view, "-- NORMAL --") {
		t.Fatalf("normal view missing mode label:\n%s", view)
	}
	m = updateEditor(t, m, keyRunes("0"))
	m = updateEditor(t, m, keyRunes("v"))
	m = updateEditor(t, m, keyRunes("e"))
	view := m.View()
	if !strings.Contains(view, "-- VISUAL --") {
		t.Fatalf("visual view missing mode label:\n%s", view)
	}
	if selected := visualSelectionStyle.Render("你"); !strings.Contains(view, selected) {
		t.Fatalf("visual view missing selected rune %q:\n%s", selected, view)
	}
}

func TestEditorVimUndoRestoresInsertSession(t *testing.T) {
	m := NewEditorModel("> ")
	m = updateEditor(t, m, keyRunes("temporary"))
	m = updateEditor(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = updateEditor(t, m, keyRunes("u"))

	if got := m.Value(); got != "" {
		t.Fatalf("value after undoing insert = %q, want empty", got)
	}
}

func updateEditor(t *testing.T, m EditorModel, msg tea.Msg) EditorModel {
	t.Helper()
	next, _ := m.Update(msg)
	updated, ok := next.(EditorModel)
	if !ok {
		t.Fatalf("Update returned %T, want EditorModel", next)
	}
	return updated
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
