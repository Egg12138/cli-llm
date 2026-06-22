package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func TestTranscriptWritesAlternateScreenSequences(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	viewer := NewTranscriptViewer(&out, 4)
	if err := viewer.Open([]string{"one", "two"}, []KeyEvent{{Key: KeyEsc}}); err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "\x1b[?1049h") {
		t.Fatalf("missing enter alternate screen sequence: %q", output)
	}
	if !strings.Contains(output, "\x1b[?1007h") {
		t.Fatalf("missing enable alternate scroll sequence: %q", output)
	}
	if strings.Index(output, "\x1b[?1007l") > strings.Index(output, "\x1b[?1049l") {
		t.Fatalf("alternate scroll should be disabled before leaving screen: %q", output)
	}
}

func TestTranscriptViewportNavigation(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	viewer := NewTranscriptViewer(&out, 2)
	lines := []string{"one", "two", "three", "four", "five"}
	err := viewer.Open(lines, []KeyEvent{
		{Key: KeyEnd},
		{Key: KeyArrowUp},
		{Key: KeyPageUp},
		{Key: KeyArrowDown},
		{Key: KeyPageDown},
		{Key: KeyHome},
		{Key: KeyEsc},
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if viewer.Offset() != 0 {
		t.Fatalf("expected viewport to end at top after Home, got %d", viewer.Offset())
	}
	output := out.String()
	for _, want := range []string{"one", "two", "five"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected rendered output to include %q: %q", want, output)
		}
	}
}

func TestTranscriptCleanupAfterPanic(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	viewer := NewTranscriptViewer(&out, 2)
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatalf("expected panic")
			}
		}()
		_ = viewer.Open([]string{"one"}, []KeyEvent{{Key: KeyPanic}})
	}()

	output := out.String()
	if !strings.Contains(output, "\x1b[?1007l") || !strings.Contains(output, "\x1b[?1049l") {
		t.Fatalf("cleanup did not restore terminal modes: %q", output)
	}
}
