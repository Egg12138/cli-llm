package repl

import (
	"sort"
	"strings"
	"unicode"
)

type Completion struct {
	Spec        CommandSpec
	MatchedName string
}

func CompleteCommand(line string, cursor int) []Completion {
	runes := []rune(line)
	if len(runes) == 0 || runes[0] != '/' || cursor < 1 || cursor > len(runes) {
		return nil
	}
	tokenEnd := commandTokenEnd(runes)
	if cursor > tokenEnd {
		return nil
	}

	prefix := string(runes[1:tokenEnd])
	matches := make([]Completion, 0, len(enabledCommands))
	for _, command := range EnabledCommands() {
		matchedName := ""
		if strings.HasPrefix(command.Name, prefix) {
			matchedName = command.Name
		} else {
			for _, alias := range command.Aliases {
				if strings.HasPrefix(alias, prefix) {
					matchedName = alias
					break
				}
			}
		}
		if matchedName != "" {
			matches = append(matches, Completion{Spec: command, MatchedName: matchedName})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Spec.Name < matches[j].Spec.Name
	})
	return matches
}

func ApplyCompletion(line string, cursor int, completion Completion) (string, int) {
	runes := []rune(line)
	if len(runes) == 0 || runes[0] != '/' || cursor < 1 || cursor > len(runes) {
		return line, cursor
	}
	tokenEnd := commandTokenEnd(runes)
	if cursor > tokenEnd {
		return line, cursor
	}

	replacement := []rune("/" + completion.Spec.Name)
	suffix := runes[tokenEnd:]
	if len(suffix) == 0 && strings.Contains(completion.Spec.Usage, "<") {
		replacement = append(replacement, ' ')
	}
	next := append(append([]rune(nil), replacement...), suffix...)
	return string(next), len(replacement)
}

func commandTokenEnd(runes []rune) int {
	for i, r := range runes {
		if unicode.IsSpace(r) {
			return i
		}
	}
	return len(runes)
}
