package graph

import "fmt"

func (s *State) Switch(target string) error {
	if target == "" {
		return fmt.Errorf("switch target cannot be empty")
	}
	if branch, ok := s.Branches[target]; ok {
		s.CurrentBranch = branch.Name
		s.HeadID = branch.HeadID
		s.Detached = false
		return nil
	}
	if _, ok := s.Entries[target]; ok {
		s.CurrentBranch = ""
		s.HeadID = target
		s.Detached = true
		return nil
	}

	parent := s.CurrentBranch
	if s.Detached {
		parent = ""
	}
	s.Branches[target] = Branch{Name: target, HeadID: s.HeadID, Parent: parent}
	s.CurrentBranch = target
	s.Detached = false
	return nil
}
