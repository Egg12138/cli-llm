package tui

import (
	"strings"
	"testing"

	"github.com/Egg12138/cli-llm/src-go/internal/session/repl"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderCompletionsShowsUsageAndDescriptions(t *testing.T) {
	matches := repl.CompleteCommand("/", 1)
	view := renderCompletions(matches, 1, 80)

	if !strings.Contains(view, "/checkpoint <name>") || !strings.Contains(view, "Label the current position") {
		t.Fatalf("wide completion view missing usage or description:\n%s", view)
	}
	if !strings.Contains(view, "> /checkpoint <name>") {
		t.Fatalf("wide completion view missing selected marker:\n%s", view)
	}
	assertCompletionWidth(t, view, 80)
}

func TestRenderCompletionsNarrowShowsSelectedDescription(t *testing.T) {
	matches := repl.CompleteCommand("/", 1)
	view := renderCompletions(matches, 4, 40)

	if !strings.Contains(view, "> /switch <target>") {
		t.Fatalf("narrow completion view missing selected usage:\n%s", view)
	}
	if !strings.Contains(view, "Switch to a branch or checkpoint") {
		t.Fatalf("narrow completion view missing selected description:\n%s", view)
	}
	assertCompletionWidth(t, view, 40)
}

func TestRenderCompletionsHandlesEmptyAndClampedSelection(t *testing.T) {
	if view := renderCompletions(nil, 0, 80); view != "" {
		t.Fatalf("empty completion view = %q, want empty", view)
	}

	matches := repl.CompleteCommand("/h", 2)
	view := renderCompletions(matches, 99, 20)
	if !strings.Contains(view, "> /help") {
		t.Fatalf("clamped completion view missing help selection: %q", view)
	}
	assertCompletionWidth(t, view, 20)
}

func assertCompletionWidth(t *testing.T, view string, width int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("completion line width = %d, want <= %d: %q", got, width, line)
		}
	}
}
