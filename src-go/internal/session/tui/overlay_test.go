package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

func TestOverlayNewModelFromStateUsesCurrentReachableHistoryAndBranches(t *testing.T) {
	now := time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC)
	root, err := model.NewMessage("", "user", "common root", now)
	if err != nil {
		t.Fatalf("NewMessage root: %v", err)
	}
	mainLeaf, err := model.NewMessage(root.ID, "assistant", "main-only", now.Add(time.Second))
	if err != nil {
		t.Fatalf("NewMessage mainLeaf: %v", err)
	}
	exploreLeaf, err := model.NewMessage(root.ID, "assistant", "explore-only", now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("NewMessage exploreLeaf: %v", err)
	}

	state := graph.NewState([]model.Entry{root, mainLeaf, exploreLeaf})
	state.Branches = map[string]graph.Branch{
		"main":    {Name: "main", HeadID: mainLeaf.ID},
		"explore": {Name: "explore", HeadID: exploreLeaf.ID, Parent: "main"},
	}
	state.CurrentBranch = "main"
	state.HeadID = mainLeaf.ID

	var clip bytes.Buffer
	m, err := newModelFromState(state, &clip)
	if err != nil {
		t.Fatalf("newModelFromState: %v", err)
	}

	view := m.View()
	if !strings.Contains(view, "main-only") {
		t.Fatalf("current branch view missing main content: %q", view)
	}
	if strings.Contains(view, "explore-only") {
		t.Fatalf("current branch view leaked explore content: %q", view)
	}
	if len(m.cfg.Branches) != 2 {
		t.Fatalf("branches = %d, want 2", len(m.cfg.Branches))
	}

	m, _ = send(m, tabKey())
	m, _ = send(m, keyMsg(tea.KeyHome))
	m, _ = send(m, enterKey())
	preview := m.View()
	if !strings.Contains(preview, "explore-only") {
		t.Fatalf("preview missing explore content: %q", preview)
	}
	if strings.Contains(preview, "main-only") {
		t.Fatalf("preview leaked main-only content: %q", preview)
	}
}
