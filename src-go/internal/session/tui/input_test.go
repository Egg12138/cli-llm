package tui

import (
	"strings"
	"testing"
)

func TestInputInsert(t *testing.T) {
	var in textInput
	in.Insert('h')
	in.Insert('i')
	if in.Value() != "hi" {
		t.Fatalf("expected 'hi', got %q", in.Value())
	}
	if in.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", in.cursor)
	}
}

func TestInputInsertAtPosition(t *testing.T) {
	var in textInput
	in.Insert('a')
	in.Insert('c')
	in.MoveLeft()
	in.Insert('b')
	if in.Value() != "abc" {
		t.Fatalf("expected 'abc', got %q", in.Value())
	}
	if in.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", in.cursor)
	}
}

func TestInputInsertNonPrintable(t *testing.T) {
	var in textInput
	in.Insert(0x00)
	in.Insert('\t')
	if in.Len() != 0 {
		t.Fatalf("expected empty buffer for control chars, got %d", in.Len())
	}
}

func TestInputInsertNewline(t *testing.T) {
	var in textInput
	in.Insert('a')
	in.Insert('\n')
	in.Insert('b')
	if in.Value() != "a\nb" {
		t.Fatalf("expected 'a\\nb', got %q", in.Value())
	}
	if in.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", in.LineCount())
	}
}

func TestInputDeleteBeforeCursor(t *testing.T) {
	var in textInput
	in.Insert('a')
	in.Insert('b')
	in.DeleteBeforeCursor() // delete 'b'
	if in.Value() != "a" {
		t.Fatalf("expected 'a', got %q", in.Value())
	}
	if in.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", in.cursor)
	}
}

func TestInputDeleteBeforeCursorAtStart(t *testing.T) {
	var in textInput
	in.Insert('a')
	in.MoveHome()
	in.DeleteBeforeCursor() // no-op
	if in.Value() != "a" {
		t.Fatalf("expected 'a', got %q", in.Value())
	}
}

func TestInputDeleteAtCursor(t *testing.T) {
	var in textInput
	in.Insert('a')
	in.Insert('b')
	in.MoveHome()
	in.DeleteAtCursor() // delete 'a'
	if in.Value() != "b" {
		t.Fatalf("expected 'b', got %q", in.Value())
	}
}

func TestInputMoveBoundaries(t *testing.T) {
	var in textInput
	in.MoveLeft() // no-op
	if in.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", in.cursor)
	}
	in.MoveRight() // no-op (buffer empty)
	if in.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", in.cursor)
	}
	in.Insert('a')
	in.MoveLeft()
	if in.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", in.cursor)
	}
	in.MoveRight()
	if in.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", in.cursor)
	}
	in.MoveRight() // clamped
	if in.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", in.cursor)
	}
}

func TestInputHomeEnd(t *testing.T) {
	var in textInput
	for _, r := range "hello" {
		in.Insert(r)
	}
	in.MoveHome()
	if in.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", in.cursor)
	}
	in.MoveEnd()
	if in.cursor != 5 {
		t.Fatalf("cursor = %d, want 5", in.cursor)
	}
}

func TestInputReset(t *testing.T) {
	var in textInput
	in.Insert('x')
	in.Reset()
	if in.Len() != 0 {
		t.Fatalf("expected empty after reset")
	}
	if in.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", in.cursor)
	}
}

func TestInputViewContainsPrompt(t *testing.T) {
	var in textInput
	for _, r := range "hello" {
		in.Insert(r)
	}
	v := in.View(80)
	if !strings.HasPrefix(v, "> ") {
		t.Fatalf("view missing prompt: %q", v)
	}
	if !strings.Contains(v, "hello") {
		t.Fatalf("view missing text: %q", v)
	}
}

func TestInputViewNarrow(t *testing.T) {
	var in textInput
	for _, r := range "this is a long message" {
		in.Insert(r)
	}
	// Move cursor to end, width=15 should be narrower than text
	v := in.View(15)
	// View should contain prompt and some text
	if !strings.HasPrefix(v, "> ") {
		t.Fatalf("view missing prompt in narrow mode: %q", v)
	}
}
