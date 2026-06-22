package terminal

import (
	"fmt"
	"io"
)

type Key string

const (
	KeyArrowUp   Key = "arrow-up"
	KeyArrowDown Key = "arrow-down"
	KeyPageUp    Key = "page-up"
	KeyPageDown  Key = "page-down"
	KeyHome      Key = "home"
	KeyEnd       Key = "end"
	KeyEsc       Key = "esc"
	KeyCtrlT     Key = "ctrl-t"
	KeyPanic     Key = "panic"
)

type KeyEvent struct {
	Key Key
}

type TranscriptViewer struct {
	out    io.Writer
	height int
	offset int
}

func NewTranscriptViewer(out io.Writer, height int) *TranscriptViewer {
	if height <= 0 {
		height = 20
	}
	return &TranscriptViewer{out: out, height: height}
}

func (v *TranscriptViewer) Offset() int {
	return v.offset
}

func (v *TranscriptViewer) Open(lines []string, events []KeyEvent) error {
	control := NewControl(v.out)
	control.EnterAltScreen()
	control.EnableAltScroll()
	defer func() {
		control.DisableAltScroll()
		control.LeaveAltScreen()
	}()

	v.render(lines, control)
	for _, event := range events {
		switch event.Key {
		case KeyArrowUp:
			v.move(-1, len(lines))
		case KeyArrowDown:
			v.move(1, len(lines))
		case KeyPageUp:
			v.move(-v.height, len(lines))
		case KeyPageDown:
			v.move(v.height, len(lines))
		case KeyHome:
			v.offset = 0
		case KeyEnd:
			v.offset = maxOffset(len(lines), v.height)
		case KeyEsc, KeyCtrlT:
			v.render(lines, control)
			return nil
		case KeyPanic:
			panic("transcript panic")
		}
		v.render(lines, control)
	}
	return nil
}

func (v *TranscriptViewer) move(delta, total int) {
	v.offset += delta
	if v.offset < 0 {
		v.offset = 0
	}
	if max := maxOffset(total, v.height); v.offset > max {
		v.offset = max
	}
}

func (v *TranscriptViewer) render(lines []string, control Control) {
	control.Clear()
	control.MoveCursor(1, 1)
	end := v.offset + v.height
	if end > len(lines) {
		end = len(lines)
	}
	for _, line := range lines[v.offset:end] {
		fmt.Fprintln(v.out, line)
	}
}

func maxOffset(total, height int) int {
	if total <= height {
		return 0
	}
	return total - height
}
