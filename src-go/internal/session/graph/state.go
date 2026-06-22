package graph

import (
	"fmt"
	"sort"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

type Branch struct {
	Name   string
	HeadID string
	Parent string
}

type State struct {
	Entries       map[string]model.Entry
	Order         []string
	Branches      map[string]Branch
	CurrentBranch string
	HeadID        string
	Detached      bool
}

func NewState(entries []model.Entry) *State {
	state := &State{
		Entries:       make(map[string]model.Entry, len(entries)),
		Order:         make([]string, 0, len(entries)),
		Branches:      map[string]Branch{"main": {Name: "main"}},
		CurrentBranch: "main",
	}
	for _, entry := range entries {
		_ = state.AddEntry(entry)
		state.HeadID = entry.ID
	}
	state.Branches["main"] = Branch{Name: "main", HeadID: state.HeadID}
	return state
}

func (s *State) AddEntry(entry model.Entry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry ID cannot be empty")
	}
	if _, exists := s.Entries[entry.ID]; exists {
		return nil
	}
	s.Entries[entry.ID] = entry
	s.Order = append(s.Order, entry.ID)
	return nil
}

func (s *State) SetHead(entryID string) {
	s.HeadID = entryID
	if s.CurrentBranch != "" && !s.Detached {
		branch := s.Branches[s.CurrentBranch]
		branch.Name = s.CurrentBranch
		branch.HeadID = entryID
		s.Branches[s.CurrentBranch] = branch
	}
}

func (s *State) Reparent(entryID, parentID string) {
	entry, ok := s.Entries[entryID]
	if !ok {
		return
	}
	entry.ParentID = parentID
	s.Entries[entryID] = entry
}

func (s *State) AutoCheckpoint(entry model.Entry) {
	s.HeadID = entry.ID
	if s.Detached || s.CurrentBranch == "" {
		return
	}
	branch := s.Branches[s.CurrentBranch]
	branch.Name = s.CurrentBranch
	branch.HeadID = entry.ID
	s.Branches[s.CurrentBranch] = branch
}

func (s *State) LabelCheckpoint(name string) error {
	if name == "" {
		return fmt.Errorf("checkpoint name cannot be empty")
	}
	parent := s.CurrentBranch
	if s.Detached {
		parent = ""
	}
	s.Branches[name] = Branch{Name: name, HeadID: s.HeadID, Parent: parent}
	return nil
}

func (s *State) ListBranches() []Branch {
	names := make([]string, 0, len(s.Branches))
	for name := range s.Branches {
		names = append(names, name)
	}
	sort.Strings(names)
	branches := make([]Branch, 0, len(names))
	for _, name := range names {
		branches = append(branches, s.Branches[name])
	}
	return branches
}

func (s *State) ReachableHistory() ([]model.Entry, error) {
	if s.HeadID == "" {
		return []model.Entry{}, nil
	}
	var reversed []model.Entry
	seen := map[string]struct{}{}
	for id := s.HeadID; id != ""; {
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("cycle in parent chain at %s", id)
		}
		seen[id] = struct{}{}
		entry, ok := s.Entries[id]
		if !ok {
			return nil, fmt.Errorf("missing entry %s", id)
		}
		reversed = append(reversed, entry)
		id = entry.ParentID
	}

	history := make([]model.Entry, len(reversed))
	for i := range reversed {
		history[len(reversed)-1-i] = reversed[i]
	}
	return history, nil
}
