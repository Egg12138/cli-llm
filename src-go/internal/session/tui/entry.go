package tui

import (
	"strings"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	"github.com/charmbracelet/glamour"
)

type displayEntry struct {
	role  string
	raw   string
	lines []string
}

type renderOptions struct {
	width int // 0 => deterministic plain rendering; >0 => glamour markdown wrap width
}

func renderEntries(entries []model.Entry, opts renderOptions) ([]displayEntry, error) {
	var out []displayEntry
	for _, e := range entries {
		if e.Type != model.EntryTypeMessage {
			continue
		}
		data, err := e.MessageData()
		if err != nil {
			return nil, err
		}
		out = append(out, displayEntry{
			role:  data.Role,
			raw:   data.Content,
			lines: renderLines(data.Role, data.Content, opts.width),
		})
	}
	return out, nil
}

func renderLines(role, content string, width int) []string {
	if width > 0 {
		if lines, ok := renderMarkdown(role, content, width); ok {
			return lines
		}
	}
	return plainLines(role, content)
}

// plainLines yields a deterministic "<role>:" header followed by each content line.
func plainLines(role, content string) []string {
	lines := []string{role + ":"}
	lines = append(lines, strings.Split(content, "\n")...)
	return lines
}

func renderMarkdown(role, content string, width int) ([]string, bool) {
	r, err := glamour.NewTermRenderer(glamour.WithWordWrap(width))
	if err != nil {
		return nil, false
	}
	rendered, err := r.Render(content)
	if err != nil {
		return nil, false
	}
	lines := []string{role + ":"}
	lines = append(lines, strings.Split(rendered, "\n")...)
	return lines, true
}
