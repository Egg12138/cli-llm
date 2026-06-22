package graph

import (
	"testing"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func TestStateInitialSessionCreatesMainBranch(t *testing.T) {
	t.Parallel()

	state := NewState(nil)

	if state.CurrentBranch != "main" {
		t.Fatalf("expected current branch main, got %q", state.CurrentBranch)
	}
	if state.Detached {
		t.Fatalf("new state should not be detached")
	}
	if branch, ok := state.Branches["main"]; !ok || branch.Name != "main" {
		t.Fatalf("expected main branch, got %#v", state.Branches)
	}
}

func TestStateAutoCheckpointAdvancesCurrentBranchHead(t *testing.T) {
	t.Parallel()

	state := NewState(nil)
	assistant := mustMessage(t, "", "assistant", "hello")
	checkpoint := mustCheckpoint(t, assistant.ID, "", assistant.ID)

	if err := state.AddEntry(assistant); err != nil {
		t.Fatalf("AddEntry assistant: %v", err)
	}
	if err := state.AddEntry(checkpoint); err != nil {
		t.Fatalf("AddEntry checkpoint: %v", err)
	}
	state.AutoCheckpoint(checkpoint)

	if state.HeadID != checkpoint.ID {
		t.Fatalf("expected head %q, got %q", checkpoint.ID, state.HeadID)
	}
	if state.Branches["main"].HeadID != checkpoint.ID {
		t.Fatalf("expected main head %q, got %q", checkpoint.ID, state.Branches["main"].HeadID)
	}
}

func TestStateReloadMainHeadIgnoresNamedBranchMetadata(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	user := mustMessage(t, "", "user", "hello")
	assistant := mustMessage(t, user.ID, "assistant", "hi")
	mainCheckpoint, err := model.NewCheckpoint(assistant.ID, "", assistant.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint main returned error: %v", err)
	}
	branchPointer, err := model.NewCheckpoint(mainCheckpoint.ID, "experiment", mainCheckpoint.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint branch returned error: %v", err)
	}

	state := NewState([]model.Entry{user, assistant, mainCheckpoint, branchPointer})

	if state.Branches["main"].HeadID != mainCheckpoint.ID {
		t.Fatalf("expected main head %q, got %#v", mainCheckpoint.ID, state.Branches["main"])
	}
	if state.HeadID != mainCheckpoint.ID {
		t.Fatalf("expected current head %q, got %q", mainCheckpoint.ID, state.HeadID)
	}
	if state.Branches["experiment"].HeadID != mainCheckpoint.ID {
		t.Fatalf("expected experiment head %q, got %#v", mainCheckpoint.ID, state.Branches["experiment"])
	}
}

func TestStateReloadUsesNamedMainPointerWhenLaterBranchHasConversation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	root := mustMessage(t, "", "user", "root")
	mainAssistant := mustMessage(t, root.ID, "assistant", "main")
	mainCheckpoint, err := model.NewCheckpoint(mainAssistant.ID, "", mainAssistant.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint main returned error: %v", err)
	}
	mainPointer, err := model.NewCheckpoint(mainCheckpoint.ID, "main", mainCheckpoint.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint main pointer returned error: %v", err)
	}
	branchUser := mustMessage(t, mainCheckpoint.ID, "user", "branch")
	branchAssistant := mustMessage(t, branchUser.ID, "assistant", "experiment")
	branchCheckpoint, err := model.NewCheckpoint(branchAssistant.ID, "", branchAssistant.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint branch returned error: %v", err)
	}
	branchPointer, err := model.NewCheckpoint(branchCheckpoint.ID, "experiment", branchCheckpoint.ID, now)
	if err != nil {
		t.Fatalf("NewCheckpoint branch pointer returned error: %v", err)
	}

	state := NewState([]model.Entry{root, mainAssistant, mainCheckpoint, mainPointer, branchUser, branchAssistant, branchCheckpoint, branchPointer})

	if state.Branches["main"].HeadID != mainCheckpoint.ID {
		t.Fatalf("expected main head %q, got %#v", mainCheckpoint.ID, state.Branches["main"])
	}
	if state.Branches["experiment"].HeadID != branchCheckpoint.ID {
		t.Fatalf("expected experiment head %q, got %#v", branchCheckpoint.ID, state.Branches["experiment"])
	}
}

func TestStateCheckpointLabelsCurrentHeadWithBranch(t *testing.T) {
	t.Parallel()

	state := NewState(nil)
	entry := mustMessage(t, "", "assistant", "answer")
	if err := state.AddEntry(entry); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	state.SetHead(entry.ID)

	if err := state.LabelCheckpoint("explore"); err != nil {
		t.Fatalf("LabelCheckpoint returned error: %v", err)
	}

	branch := state.Branches["explore"]
	if branch.HeadID != entry.ID {
		t.Fatalf("expected explore head %q, got %q", entry.ID, branch.HeadID)
	}
	if branch.Parent != "main" {
		t.Fatalf("expected parent main, got %q", branch.Parent)
	}
}

func TestStateSwitchBranchHashAndUnknownBranch(t *testing.T) {
	t.Parallel()

	mainHead := mustMessage(t, "", "assistant", "main")
	detached := mustMessage(t, mainHead.ID, "assistant", "old")
	state := NewState([]model.Entry{mainHead, detached})
	state.SetHead(mainHead.ID)
	state.Branches["main"] = Branch{Name: "main", HeadID: mainHead.ID}

	if err := state.Switch("main"); err != nil {
		t.Fatalf("switch main: %v", err)
	}
	if state.HeadID != mainHead.ID || state.Detached {
		t.Fatalf("expected attached main at %q, got head=%q detached=%v", mainHead.ID, state.HeadID, state.Detached)
	}

	if err := state.Switch(detached.ID); err != nil {
		t.Fatalf("switch hash: %v", err)
	}
	if state.HeadID != detached.ID || !state.Detached || state.CurrentBranch != "" {
		t.Fatalf("expected detached at %q, got head=%q branch=%q detached=%v", detached.ID, state.HeadID, state.CurrentBranch, state.Detached)
	}

	if err := state.Switch("newname"); err != nil {
		t.Fatalf("switch new branch: %v", err)
	}
	if state.CurrentBranch != "newname" || state.Detached {
		t.Fatalf("expected attached newname, got branch=%q detached=%v", state.CurrentBranch, state.Detached)
	}
	if state.Branches["newname"].HeadID != detached.ID {
		t.Fatalf("expected new branch from detached head %q, got %q", detached.ID, state.Branches["newname"].HeadID)
	}
}

func TestStateBranchListAndReachableHistory(t *testing.T) {
	t.Parallel()

	root := mustMessage(t, "", "user", "root")
	main := mustMessage(t, root.ID, "assistant", "main")
	side := mustMessage(t, root.ID, "assistant", "side")
	state := NewState([]model.Entry{root, main, side})
	state.Branches["main"] = Branch{Name: "main", HeadID: main.ID}
	state.Branches["side"] = Branch{Name: "side", HeadID: side.ID, Parent: "main"}
	state.CurrentBranch = "side"
	state.HeadID = side.ID

	branches := state.ListBranches()
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %#v", branches)
	}
	if branches[0].Name != "main" || branches[0].HeadID != main.ID {
		t.Fatalf("expected main branch first, got %#v", branches)
	}
	if branches[1].Name != "side" || branches[1].Parent != "main" {
		t.Fatalf("expected side parent main, got %#v", branches)
	}

	history, err := state.ReachableHistory()
	if err != nil {
		t.Fatalf("ReachableHistory returned error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected root and side, got %#v", history)
	}
	if history[0].ID != root.ID || history[1].ID != side.ID {
		t.Fatalf("expected reachable chain root->side, got %#v", history)
	}
}

func mustMessage(t *testing.T, parentID, role, content string) model.Entry {
	t.Helper()
	entry, err := model.NewMessage(parentID, role, content, time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	return entry
}

func mustCheckpoint(t *testing.T, parentID, name, returnTo string) model.Entry {
	t.Helper()
	entry, err := model.NewCheckpoint(parentID, name, returnTo, time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewCheckpoint returned error: %v", err)
	}
	return entry
}
