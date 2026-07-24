package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestEnabledCommandsAreCompleteAndUnique(t *testing.T) {
	commands := EnabledCommands()
	if len(commands) != 6 {
		t.Fatalf("EnabledCommands returned %d commands, want 6", len(commands))
	}

	seen := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		if command.Name == "" || command.Usage == "" || command.Description == "" || command.Kind == "" {
			t.Fatalf("registry contains incomplete command: %#v", command)
		}
		if _, exists := seen[command.Name]; exists {
			t.Fatalf("duplicate command name %q", command.Name)
		}
		seen[command.Name] = struct{}{}
	}
}

func TestLookupCommandResolvesAliases(t *testing.T) {
	canonical, ok := LookupCommand("transcript")
	if !ok {
		t.Fatal("canonical transcript command not found")
	}
	alias, ok := LookupCommand("t")
	if !ok {
		t.Fatal("transcript alias not found")
	}
	if canonical.Kind != CommandTranscript || alias.Kind != canonical.Kind {
		t.Fatalf("alias resolved to wrong kind: canonical=%q alias=%q", canonical.Kind, alias.Kind)
	}
}

func TestHelpComesFromEnabledCommandRegistry(t *testing.T) {
	var out bytes.Buffer
	WriteHelp(&out)
	text := out.String()

	for _, command := range EnabledCommands() {
		if count := strings.Count(text, command.Usage); count != 1 {
			t.Fatalf("help contains usage %q %d times, want 1:\n%s", command.Usage, count, text)
		}
		if !strings.Contains(text, command.Description) {
			t.Fatalf("help missing description %q:\n%s", command.Description, text)
		}
	}
	if !strings.Contains(text, "/t") {
		t.Fatalf("help missing transcript alias /t:\n%s", text)
	}
	for _, deferred := range []string{"/export", "/new", "/rename"} {
		if strings.Contains(text, deferred) {
			t.Fatalf("help contains deferred command %q:\n%s", deferred, text)
		}
	}
}

func TestHelpCommandParsesAndContinues(t *testing.T) {
	command := ParseLine("/help")
	if command.Kind != CommandHelp {
		t.Fatalf("ParseLine(/help) kind = %q, want %q", command.Kind, CommandHelp)
	}

	var out bytes.Buffer
	action, err := ExecuteCommand(nil, command, &out)
	if err != nil {
		t.Fatalf("ExecuteCommand(/help) returned error: %v", err)
	}
	if action != ActionContinue {
		t.Fatalf("ExecuteCommand(/help) action = %q, want %q", action, ActionContinue)
	}
	if !strings.Contains(out.String(), "/help") {
		t.Fatalf("help output missing /help: %q", out.String())
	}
}
