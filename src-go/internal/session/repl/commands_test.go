package repl

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func TestSlashCommands(t *testing.T) {
	t.Parallel()

	root := mustReplMessage(t, "", "user", "root")
	head := mustReplMessage(t, root.ID, "assistant", "head")
	state := graph.NewState([]model.Entry{root, head})
	state.SetHead(head.ID)

	tests := []struct {
		name       string
		line       string
		wantKind   CommandKind
		wantAction Action
	}{
		{name: "exit", line: "/exit", wantKind: CommandExit, wantAction: ActionExit},
		{name: "branches", line: "/branches", wantKind: CommandBranches, wantAction: ActionContinue},
		{name: "switch", line: "/switch main", wantKind: CommandSwitch, wantAction: ActionContinue},
		{name: "checkpoint", line: "/checkpoint explore", wantKind: CommandCheckpoint, wantAction: ActionContinue},
		{name: "unknown", line: "/wat", wantKind: CommandUnknown, wantAction: ActionContinue},
		{name: "chat", line: "hello", wantKind: CommandChat, wantAction: ActionChat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := ParseLine(tt.line)
			if command.Kind != tt.wantKind {
				t.Fatalf("expected kind %q, got %#v", tt.wantKind, command)
			}
			var out bytes.Buffer
			action, err := ExecuteCommand(state, command, &out)
			if err != nil {
				t.Fatalf("ExecuteCommand returned error: %v", err)
			}
			if action != tt.wantAction {
				t.Fatalf("expected action %q, got %q", tt.wantAction, action)
			}
		})
	}
}

func mustReplMessage(t *testing.T, parentID, role, content string) model.Entry {
	t.Helper()
	entry, err := model.NewMessage(parentID, role, content, time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	return entry
}

func TestSlashCommandEffects(t *testing.T) {
	t.Parallel()

	root := mustReplMessage(t, "", "user", "root")
	head := mustReplMessage(t, root.ID, "assistant", "head")
	old := mustReplMessage(t, root.ID, "assistant", "old")
	state := graph.NewState([]model.Entry{root, head, old})
	state.SetHead(head.ID)
	state.Branches["old"] = graph.Branch{Name: "old", HeadID: old.ID, Parent: "main"}

	var out bytes.Buffer
	if _, err := ExecuteCommand(state, ParseLine("/branches"), &out); err != nil {
		t.Fatalf("branches: %v", err)
	}
	if output := out.String(); !strings.Contains(output, "main") || !strings.Contains(output, head.ID) {
		t.Fatalf("branches output missing main head: %q", output)
	}

	if _, err := ExecuteCommand(state, ParseLine("/switch old"), &out); err != nil {
		t.Fatalf("switch old: %v", err)
	}
	if state.CurrentBranch != "old" || state.HeadID != old.ID {
		t.Fatalf("expected old branch at old head, got branch=%q head=%q", state.CurrentBranch, state.HeadID)
	}

	if _, err := ExecuteCommand(state, ParseLine("/switch "+root.ID), &out); err != nil {
		t.Fatalf("switch hash: %v", err)
	}
	if !state.Detached || state.HeadID != root.ID {
		t.Fatalf("expected detached root, got branch=%q head=%q detached=%v", state.CurrentBranch, state.HeadID, state.Detached)
	}

	if _, err := ExecuteCommand(state, ParseLine("/switch newbranch"), &out); err != nil {
		t.Fatalf("switch newbranch: %v", err)
	}
	if state.CurrentBranch != "newbranch" || state.Branches["newbranch"].HeadID != root.ID {
		t.Fatalf("expected newbranch from root, got %#v", state.Branches["newbranch"])
	}

	if _, err := ExecuteCommand(state, ParseLine("/checkpoint named"), &out); err != nil {
		t.Fatalf("checkpoint named: %v", err)
	}
	if state.Branches["named"].HeadID != root.ID {
		t.Fatalf("expected named checkpoint at root, got %#v", state.Branches["named"])
	}

	out.Reset()
	if _, err := ExecuteCommand(state, ParseLine("/unknown"), &out); err != nil {
		t.Fatalf("unknown: %v", err)
	}
	if !strings.Contains(out.String(), "unknown command") {
		t.Fatalf("expected user-visible unknown command error, got %q", out.String())
	}
}
