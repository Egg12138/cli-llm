package repl

import (
	"reflect"
	"testing"
)

func TestCompleteCommandMatchesPrefix(t *testing.T) {
	matches := CompleteCommand("/h", 2)
	if len(matches) != 1 || matches[0].Spec.Name != "help" {
		t.Fatalf("CompleteCommand(/h) = %#v, want help", matches)
	}
}

func TestCompleteCommandDeduplicatesAliasAndCanonicalMatch(t *testing.T) {
	matches := CompleteCommand("/t", 2)
	if len(matches) != 1 {
		t.Fatalf("CompleteCommand(/t) returned %d matches, want 1: %#v", len(matches), matches)
	}
	if matches[0].Spec.Name != "transcript" {
		t.Fatalf("CompleteCommand(/t) = %#v, want transcript", matches[0])
	}
}

func TestCompleteCommandIsOnlyActiveInLeadingCommandToken(t *testing.T) {
	tests := []struct {
		line   string
		cursor int
	}{
		{line: "hello /sw", cursor: 9},
		{line: "/sw feature", cursor: 5},
		{line: " /sw", cursor: 4},
	}
	for _, tt := range tests {
		if matches := CompleteCommand(tt.line, tt.cursor); len(matches) != 0 {
			t.Fatalf("CompleteCommand(%q, %d) = %#v, want no matches", tt.line, tt.cursor, matches)
		}
	}
}

func TestCompleteCommandReturnsDeterministicCanonicalOrder(t *testing.T) {
	matches := CompleteCommand("/", 1)
	names := make([]string, len(matches))
	for i, match := range matches {
		names[i] = match.Spec.Name
	}
	want := []string{"branches", "checkpoint", "exit", "help", "switch", "transcript"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("command order = %#v, want %#v", names, want)
	}
}

func TestApplyCompletionPreservesArgumentsExactly(t *testing.T) {
	matches := CompleteCommand("/sw  功能 分支", 3)
	if len(matches) != 1 {
		t.Fatalf("CompleteCommand returned %#v, want one match", matches)
	}

	next, cursor := ApplyCompletion("/sw  功能 分支", 3, matches[0])
	if next != "/switch  功能 分支" {
		t.Fatalf("ApplyCompletion result = %q, want exact argument preservation", next)
	}
	if cursor != len([]rune("/switch")) {
		t.Fatalf("ApplyCompletion cursor = %d, want %d", cursor, len([]rune("/switch")))
	}
}

func TestApplyCompletionAddsSpaceForArgumentCommand(t *testing.T) {
	matches := CompleteCommand("/chec", 5)
	if len(matches) != 1 {
		t.Fatalf("CompleteCommand returned %#v, want one match", matches)
	}

	next, cursor := ApplyCompletion("/chec", 5, matches[0])
	if next != "/checkpoint " {
		t.Fatalf("ApplyCompletion result = %q, want %q", next, "/checkpoint ")
	}
	if cursor != len([]rune(next)) {
		t.Fatalf("ApplyCompletion cursor = %d, want %d", cursor, len([]rune(next)))
	}
}
