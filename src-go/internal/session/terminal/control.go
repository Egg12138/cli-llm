package terminal

import (
	"fmt"
	"io"
)

type Control struct {
	out io.Writer
}

func NewControl(out io.Writer) Control {
	return Control{out: out}
}

func (c Control) EnterAltScreen() {
	fmt.Fprint(c.out, "\x1b[?1049h")
}

func (c Control) LeaveAltScreen() {
	fmt.Fprint(c.out, "\x1b[?1049l")
}

func (c Control) EnableAltScroll() {
	fmt.Fprint(c.out, "\x1b[?1007h")
}

func (c Control) DisableAltScroll() {
	fmt.Fprint(c.out, "\x1b[?1007l")
}

func (c Control) Clear() {
	fmt.Fprint(c.out, "\x1b[2J")
}

func (c Control) MoveCursor(row, col int) {
	fmt.Fprintf(c.out, "\x1b[%d;%dH", row, col)
}
