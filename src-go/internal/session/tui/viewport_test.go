package tui

import (
	"strings"
	"testing"
)

func TestViewportEmpty(t *testing.T) {
	var vp viewport
	if got := vp.View(); got != "" {
		t.Fatalf("empty viewport should return empty string, got %q", got)
	}
}

func TestViewportSetLines(t *testing.T) {
	var vp viewport
	vp.height = 3
	vp.SetLines([]string{"a", "b", "c", "d", "e"})
	if vp.offset != 2 { // at bottom: len(5)-3=2
		t.Fatalf("offset = %d, want 2", vp.offset)
	}
	v := vp.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 3 || lines[0] != "c" || lines[2] != "e" {
		t.Fatalf("expected c/d/e, got %q", v)
	}
}

func TestViewportAutoScrollOnAppend(t *testing.T) {
	var vp viewport
	vp.height = 2
	vp.SetLines([]string{"a", "b"})
	vp.AppendLine("c")
	if vp.offset != 1 {
		t.Fatalf("after append at bottom, offset = %d, want 1", vp.offset)
	}
}

func TestViewportNoAutoScrollWhenScrolledUp(t *testing.T) {
	var vp viewport
	vp.height = 2
	vp.SetLines([]string{"a", "b", "c", "d"})
	vp.ScrollUp(1) // offset 2->1
	vp.AppendLine("e")
	if vp.offset != 1 {
		t.Fatalf("when scrolled up, offset should stay 1, got %d", vp.offset)
	}
}

func TestViewportScrollUpDown(t *testing.T) {
	var vp viewport
	vp.height = 2
	vp.SetLines([]string{"a", "b", "c", "d"})
	vp.ScrollToTop()
	if vp.offset != 0 {
		t.Fatalf("top offset = %d, want 0", vp.offset)
	}
	vp.ScrollDown(1)
	if vp.offset != 1 {
		t.Fatalf("down 1 offset = %d, want 1", vp.offset)
	}
	vp.ScrollDown(10) // clamped
	if vp.offset != 2 {
		t.Fatalf("down 10 offset = %d, want 2", vp.offset)
	}
}

func TestViewportContentFits(t *testing.T) {
	var vp viewport
	vp.height = 10
	vp.SetLines([]string{"a", "b"})
	if vp.offset != 0 {
		t.Fatalf("when content fits, offset = %d, want 0", vp.offset)
	}
}

func TestViewportAtBottom(t *testing.T) {
	var vp viewport
	vp.height = 2
	vp.SetLines([]string{"a", "b", "c"})
	if !vp.AtBottom() {
		t.Fatal("should be at bottom after SetLines")
	}
	vp.ScrollUp(1)
	if vp.AtBottom() {
		t.Fatal("should NOT be at bottom after scrolling up")
	}
}

func TestViewportViewHeightClamping(t *testing.T) {
	var vp viewport
	vp.height = 3
	vp.SetLines([]string{"a"})
	v := vp.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 1 {
		t.Fatalf("should show 1 line when only 1 exists, got %d lines: %q", len(lines), v)
	}
}

func TestViewportAppendAtBottomKeepsAtBottom(t *testing.T) {
	var vp viewport
	vp.height = 2
	vp.SetLines([]string{"a", "b", "c"}) // offset=1, at bottom
	vp.ScrollUp(1)                         // offset=0, NOT at bottom
	vp.AppendLine("d")
	// Should NOT auto-scroll since not at bottom
	if vp.offset != 0 {
		t.Fatalf("offset should stay 0 when not at bottom, got %d", vp.offset)
	}
}
