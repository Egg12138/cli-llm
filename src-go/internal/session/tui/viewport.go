package tui

import (
	"strings"
)

type viewport struct {
	lines  []string
	offset int
	height int
}

func (vp *viewport) SetLines(lines []string) {
	if len(lines) == 0 {
		vp.lines = nil
		vp.offset = 0
		return
	}
	wasAtBottom := vp.AtBottom()
	vp.lines = make([]string, len(lines))
	copy(vp.lines, lines)
	if wasAtBottom {
		vp.ScrollToBottom()
	} else {
		vp.clampOffset()
	}
}

func (vp *viewport) AppendLine(line string) {
	wasAtBottom := vp.AtBottom()
	vp.lines = append(vp.lines, line)
	if wasAtBottom {
		vp.ScrollToBottom()
	}
}

func (vp *viewport) ScrollUp(n int) {
	vp.offset -= n
	if vp.offset < 0 {
		vp.offset = 0
	}
}

func (vp *viewport) ScrollDown(n int) {
	vp.offset += n
	vp.clampOffset()
}

func (vp *viewport) ScrollToBottom() {
	vp.offset = vp.maxOffset()
}

func (vp *viewport) ScrollToTop() {
	vp.offset = 0
}

func (vp *viewport) View() string {
	if vp.height <= 0 || len(vp.lines) == 0 {
		return ""
	}
	vp.clampOffset()
	end := vp.offset + vp.height
	if end > len(vp.lines) {
		end = len(vp.lines)
	}
	return strings.Join(vp.lines[vp.offset:end], "\n")
}

func (vp *viewport) AtBottom() bool {
	return vp.offset >= vp.maxOffset()
}

func (vp *viewport) maxOffset() int {
	if vp.height >= len(vp.lines) {
		return 0
	}
	return len(vp.lines) - vp.height
}

func (vp *viewport) LineCount() int {
	return len(vp.lines)
}

func (vp *viewport) AppendToLastLine(text string) {
	if len(vp.lines) == 0 {
		vp.lines = append(vp.lines, text)
		return
	}
	vp.lines[len(vp.lines)-1] += text
	if vp.AtBottom() {
		vp.ScrollToBottom()
	}
}

func (vp *viewport) clampOffset() {
	mx := vp.maxOffset()
	if vp.offset > mx {
		vp.offset = mx
	}
	if vp.offset < 0 {
		vp.offset = 0
	}
}
