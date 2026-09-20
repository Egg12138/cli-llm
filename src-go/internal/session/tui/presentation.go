package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	userLabel      = "You ›"
	assistantLabel = "Assistant ›"
)

// Presentation owns display-only styling for the normal-buffer session UI.
// It never changes persisted message content.
type Presentation struct {
	out            io.Writer
	userLabel      string
	assistantLabel string
	color          bool
}

func NewPresentation(out io.Writer, color bool) Presentation {
	if out == nil {
		out = io.Discard
	}
	p := Presentation{
		out:            out,
		userLabel:      userLabel,
		assistantLabel: assistantLabel,
		color:          color,
	}
	if !color {
		return p
	}

	renderer := lipgloss.NewRenderer(out)
	renderer.SetColorProfile(termenv.ANSI)
	p.userLabel = renderer.NewStyle().Bold(true).Foreground(roleUserColor).Render(userLabel)
	p.assistantLabel = renderer.NewStyle().Bold(true).Foreground(roleAssistantColor).Render(assistantLabel)
	return p
}

func (p Presentation) UserLabel() string {
	return p.userLabel
}

func (p Presentation) WriteUser(input string) error {
	if _, err := fmt.Fprintln(p.out, p.userLabel); err != nil {
		return err
	}
	if _, err := fmt.Fprint(p.out, input); err != nil {
		return err
	}
	if len(input) > 0 && input[len(input)-1] == '\n' {
		_, err := fmt.Fprintln(p.out)
		return err
	}
	_, err := fmt.Fprint(p.out, "\n\n")
	return err
}

func (p Presentation) NewAssistantWriter() *AssistantWriter {
	return &AssistantWriter{out: p.out, label: p.assistantLabel}
}

func (p Presentation) CommandWriter() io.Writer {
	if !p.color {
		return p.out
	}
	return styledWriter{out: p.out, prefix: "\x1b[2m", suffix: "\x1b[0m"}
}

type AssistantWriter struct {
	out              io.Writer
	label            string
	started          bool
	finished         bool
	trailingNewlines int
}

func (w *AssistantWriter) Write(content []byte) (int, error) {
	if len(content) == 0 {
		return 0, nil
	}
	if !w.started {
		if _, err := fmt.Fprintln(w.out, w.label); err != nil {
			return 0, err
		}
		w.started = true
	}
	n, err := w.out.Write(content)
	w.trackTrailingNewlines(content[:n])
	return n, err
}

// Finish ensures one blank line separates the response from the next prompt.
func (w *AssistantWriter) Finish() error {
	if w.finished || !w.started {
		return nil
	}
	w.finished = true
	needed := 2 - w.trailingNewlines
	if needed <= 0 {
		return nil
	}
	_, err := fmt.Fprint(w.out, strings.Repeat("\n", needed))
	return err
}

func (w *AssistantWriter) trackTrailingNewlines(content []byte) {
	if len(content) == 0 {
		return
	}
	if content[len(content)-1] != '\n' {
		w.trailingNewlines = 0
		return
	}
	count := 0
	for i := len(content) - 1; i >= 0 && content[i] == '\n'; i-- {
		count++
	}
	if count == len(content) {
		w.trailingNewlines += count
		return
	}
	w.trailingNewlines = count
}

type styledWriter struct {
	out            io.Writer
	prefix, suffix string
}

func (w styledWriter) Write(content []byte) (int, error) {
	if len(content) == 0 {
		return 0, nil
	}
	if _, err := fmt.Fprintf(w.out, "%s%s%s", w.prefix, content, w.suffix); err != nil {
		return 0, err
	}
	return len(content), nil
}
