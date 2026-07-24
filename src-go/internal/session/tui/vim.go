package tui

import "unicode"

type vimMode uint8

const (
	vimInsert vimMode = iota
	vimNormal
	vimVisual
)

func (m vimMode) String() string {
	switch m {
	case vimNormal:
		return "NORMAL"
	case vimVisual:
		return "VISUAL"
	default:
		return "INSERT"
	}
}

type vimEditor struct {
	buffer          []rune
	cursor          int
	mode            vimMode
	pending         rune
	preferredColumn int
	hasPreferredCol bool
	visualAnchor    int
	register        vimRegister
	undo            []vimSnapshot
	insertUndoSaved bool
}

type vimRegister struct {
	text     []rune
	linewise bool
}

type vimSnapshot struct {
	buffer []rune
	cursor int
}

func newVimEditor() vimEditor {
	return vimEditor{mode: vimInsert}
}

func (e *vimEditor) InsertText(value string) {
	for _, r := range []rune(value) {
		e.insertRune(r)
	}
}

func (e *vimEditor) InsertUserText(value string) {
	if e.mode != vimInsert || value == "" {
		return
	}
	e.beginInsertChange()
	e.InsertText(value)
}

func (e *vimEditor) DeleteBeforeCursor() {
	if e.mode != vimInsert || e.cursor <= 0 {
		return
	}
	e.beginInsertChange()
	copy(e.buffer[e.cursor-1:], e.buffer[e.cursor:])
	e.buffer = e.buffer[:len(e.buffer)-1]
	e.cursor--
	e.resetPreferredColumn()
}

func (e *vimEditor) DeleteAtCursor() {
	if e.mode != vimInsert || e.cursor >= len(e.buffer) {
		return
	}
	e.beginInsertChange()
	copy(e.buffer[e.cursor:], e.buffer[e.cursor+1:])
	e.buffer = e.buffer[:len(e.buffer)-1]
	e.resetPreferredColumn()
}

func (e *vimEditor) MoveInsertCursor(key string) {
	if e.mode != vimInsert {
		return
	}
	switch key {
	case "left":
		if e.cursor > 0 {
			e.cursor--
		}
		e.resetPreferredColumn()
	case "right":
		if e.cursor < len(e.buffer) {
			e.cursor++
		}
		e.resetPreferredColumn()
	case "up":
		e.moveInsertVertical(-1)
	case "down":
		e.moveInsertVertical(1)
	case "home":
		e.cursor = e.lineStart(e.cursor)
		e.resetPreferredColumn()
	case "end":
		e.cursor = e.lineEnd(e.cursor)
		e.resetPreferredColumn()
	}
}

func (e *vimEditor) SetValue(value string) {
	e.buffer = []rune(value)
	e.cursor = len(e.buffer)
	e.resetPreferredColumn()
}

func (e *vimEditor) SetCursor(position int) {
	e.cursor = clampInt(position, 0, len(e.buffer))
	e.resetPreferredColumn()
}

func (e *vimEditor) beginInsertChange() {
	if e.insertUndoSaved {
		return
	}
	e.saveUndo()
	e.insertUndoSaved = true
}

func (e *vimEditor) Handle(key string) {
	if e.mode == vimInsert {
		if key == "esc" {
			e.mode = vimNormal
			e.insertUndoSaved = false
			if e.cursor > 0 {
				e.cursor--
			}
			e.clampNormalCursor()
		}
		return
	}
	if e.mode == vimVisual {
		e.handleVisualKey(key)
		return
	}

	e.handleNormalKey(key)
}

func (e *vimEditor) handleNormalKey(key string) {
	if e.pending == 'g' {
		e.pending = 0
		if key == "g" {
			e.cursor = 0
			e.resetPreferredColumn()
			return
		}
	}
	if e.pending == 'd' || e.pending == 'c' || e.pending == 'y' {
		operator := e.pending
		e.pending = 0
		if key == string(operator) {
			e.applyLineOperator(operator)
		} else {
			e.applyOperator(operator, key)
		}
		return
	}
	if e.pending == 'r' {
		e.pending = 0
		e.replaceWith(key)
		return
	}

	switch key {
	case "esc":
		e.pending = 0
	case "h", "left":
		e.moveLeft()
	case "l", "right":
		e.moveRight()
	case "j", "down":
		e.moveVertical(1)
	case "k", "up":
		e.moveVertical(-1)
	case "0", "home":
		e.cursor = e.lineStart(e.cursor)
		e.resetPreferredColumn()
	case "^":
		e.cursor = e.firstNonBlank(e.cursor)
		e.resetPreferredColumn()
	case "$", "end":
		e.cursor = e.lineLastPosition(e.cursor)
		e.resetPreferredColumn()
	case "w":
		e.cursor = e.wordForward(e.cursor)
		e.resetPreferredColumn()
	case "b":
		e.cursor = e.wordBackward(e.cursor)
		e.resetPreferredColumn()
	case "e":
		e.cursor = e.wordEnd(e.cursor)
		e.resetPreferredColumn()
	case "g":
		e.pending = 'g'
	case "G":
		e.cursor = e.lastPosition()
		e.pending = 0
		e.resetPreferredColumn()
	case "i":
		e.mode = vimInsert
		e.insertUndoSaved = false
	case "I":
		e.cursor = e.firstNonBlank(e.cursor)
		e.mode = vimInsert
		e.insertUndoSaved = false
	case "a":
		if len(e.buffer) > 0 {
			e.cursor = min(e.cursor+1, len(e.buffer))
		}
		e.mode = vimInsert
		e.insertUndoSaved = false
	case "A":
		e.cursor = e.lineEnd(e.cursor)
		e.mode = vimInsert
		e.insertUndoSaved = false
	case "o":
		e.openLineBelow()
	case "O":
		e.openLineAbove()
	case "v":
		e.mode = vimVisual
		e.visualAnchor = e.cursor
	case "x":
		e.deleteCharacter()
	case "d", "c", "y", "r":
		e.pending = []rune(key)[0]
	case "p":
		e.paste(false)
	case "P":
		e.paste(true)
	case "u":
		e.restoreUndo()
	}

	if e.mode != vimInsert {
		e.clampNormalCursor()
	}
}

func (e *vimEditor) handleVisualKey(key string) {
	switch key {
	case "esc":
		e.mode = vimNormal
		e.pending = 0
	case "o":
		e.visualAnchor, e.cursor = e.cursor, e.visualAnchor
	case "y":
		e.applyVisualOperator('y')
	case "d", "x":
		e.applyVisualOperator('d')
	case "c":
		e.applyVisualOperator('c')
	default:
		e.handleNormalKey(key)
	}
}

func (e *vimEditor) applyOperator(operator rune, motion string) {
	start, end, ok := e.operatorRange(operator, motion)
	if !ok || start == end {
		return
	}
	e.setRegister(e.buffer[start:end], false)
	if operator == 'y' {
		return
	}
	e.saveUndo()
	if operator == 'c' {
		e.mode = vimInsert
		e.insertUndoSaved = true
	}
	e.deleteRange(start, end)
}

func (e vimEditor) operatorRange(operator rune, motion string) (int, int, bool) {
	start := e.cursor
	end := start
	switch motion {
	case "w":
		if operator == 'c' {
			end = e.wordEnd(start) + 1
		} else {
			end = e.wordForwardBoundary(start)
		}
	case "e":
		end = e.wordEnd(start) + 1
	case "l", "right":
		end = min(start+1, e.lineEnd(start))
	case "h", "left":
		start = max(e.lineStart(start), start-1)
		end = e.cursor
	case "$", "end":
		end = e.lineEnd(start)
	case "0", "home":
		start = e.lineStart(start)
		end = e.cursor
	case "b":
		start = e.wordBackward(start)
		end = e.cursor
	default:
		return 0, 0, false
	}
	if start > end {
		start, end = end, start
	}
	return clampInt(start, 0, len(e.buffer)), clampInt(end, 0, len(e.buffer)), true
}

func (e *vimEditor) applyLineOperator(operator rune) {
	start := e.lineStart(e.cursor)
	end := e.lineEnd(e.cursor)
	e.setRegister(e.buffer[start:end], true)
	if operator == 'y' {
		return
	}

	e.saveUndo()
	if operator == 'c' {
		e.mode = vimInsert
		e.insertUndoSaved = true
		e.deleteRange(start, end)
		return
	}

	deleteStart, deleteEnd := start, end
	if deleteEnd < len(e.buffer) {
		deleteEnd++
	} else if deleteStart > 0 {
		deleteStart--
	}
	e.deleteRange(deleteStart, deleteEnd)
}

func (e *vimEditor) applyVisualOperator(operator rune) {
	start, end, ok := e.SelectionRange()
	if !ok {
		e.mode = vimNormal
		return
	}
	e.setRegister(e.buffer[start:end], false)
	if operator == 'y' {
		e.cursor = start
		e.mode = vimNormal
		return
	}
	e.saveUndo()
	if operator == 'c' {
		e.mode = vimInsert
		e.insertUndoSaved = true
	} else {
		e.mode = vimNormal
	}
	e.deleteRange(start, end)
}

func (e *vimEditor) deleteCharacter() {
	if len(e.buffer) == 0 || e.cursor >= len(e.buffer) || e.buffer[e.cursor] == '\n' {
		return
	}
	e.setRegister(e.buffer[e.cursor:e.cursor+1], false)
	e.saveUndo()
	e.deleteRange(e.cursor, e.cursor+1)
}

func (e *vimEditor) deleteRange(start, end int) {
	start = clampInt(start, 0, len(e.buffer))
	end = clampInt(end, start, len(e.buffer))
	copy(e.buffer[start:], e.buffer[end:])
	e.buffer = e.buffer[:len(e.buffer)-(end-start)]
	e.cursor = start
	if e.mode != vimInsert {
		e.clampNormalCursor()
	}
	e.resetPreferredColumn()
}

func (e *vimEditor) replaceWith(key string) {
	runes := []rune(key)
	if len(runes) != 1 || len(e.buffer) == 0 || e.cursor >= len(e.buffer) || e.buffer[e.cursor] == '\n' {
		return
	}
	e.saveUndo()
	e.buffer[e.cursor] = runes[0]
}

func (e *vimEditor) openLineBelow() {
	e.saveUndo()
	end := e.lineEnd(e.cursor)
	if end < len(e.buffer) {
		e.insertAt(end+1, []rune{'\n'})
		e.cursor = end + 1
	} else {
		e.insertAt(end, []rune{'\n'})
		e.cursor = end + 1
	}
	e.mode = vimInsert
	e.insertUndoSaved = true
}

func (e *vimEditor) openLineAbove() {
	e.saveUndo()
	start := e.lineStart(e.cursor)
	e.insertAt(start, []rune{'\n'})
	e.cursor = start
	e.mode = vimInsert
	e.insertUndoSaved = true
}

func (e *vimEditor) paste(before bool) {
	if len(e.register.text) == 0 && !e.register.linewise {
		return
	}
	e.saveUndo()
	if e.register.linewise {
		e.pasteLine(before)
		return
	}

	position := e.cursor
	if !before && len(e.buffer) > 0 {
		position++
	}
	position = clampInt(position, 0, len(e.buffer))
	e.insertAt(position, e.register.text)
	e.cursor = position + len(e.register.text) - 1
	e.clampNormalCursor()
}

func (e *vimEditor) pasteLine(before bool) {
	text := append([]rune(nil), e.register.text...)
	if len(e.buffer) == 0 {
		e.insertAt(0, text)
		e.cursor = 0
		return
	}
	if before {
		position := e.lineStart(e.cursor)
		text = append(text, '\n')
		e.insertAt(position, text)
		e.cursor = position
		return
	}

	end := e.lineEnd(e.cursor)
	if end < len(e.buffer) {
		position := end + 1
		text = append(text, '\n')
		e.insertAt(position, text)
		e.cursor = position
		return
	}
	text = append([]rune{'\n'}, text...)
	e.insertAt(end, text)
	e.cursor = end + 1
}

func (e *vimEditor) insertAt(position int, value []rune) {
	position = clampInt(position, 0, len(e.buffer))
	inserted := append([]rune(nil), value...)
	e.buffer = append(e.buffer, make([]rune, len(inserted))...)
	copy(e.buffer[position+len(inserted):], e.buffer[position:len(e.buffer)-len(inserted)])
	copy(e.buffer[position:], inserted)
}

func (e *vimEditor) setRegister(value []rune, linewise bool) {
	e.register = vimRegister{text: append([]rune(nil), value...), linewise: linewise}
}

func (e *vimEditor) saveUndo() {
	e.undo = append(e.undo, vimSnapshot{
		buffer: append([]rune(nil), e.buffer...),
		cursor: e.cursor,
	})
}

func (e *vimEditor) restoreUndo() {
	if len(e.undo) == 0 {
		return
	}
	snapshot := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.buffer = append([]rune(nil), snapshot.buffer...)
	e.cursor = snapshot.cursor
	e.mode = vimNormal
	e.pending = 0
	e.insertUndoSaved = false
	e.clampNormalCursor()
}

func (e *vimEditor) insertRune(r rune) {
	e.cursor = clampInt(e.cursor, 0, len(e.buffer))
	e.buffer = append(e.buffer, 0)
	copy(e.buffer[e.cursor+1:], e.buffer[e.cursor:])
	e.buffer[e.cursor] = r
	e.cursor++
}

func (e *vimEditor) moveLeft() {
	start := e.lineStart(e.cursor)
	if e.cursor > start {
		e.cursor--
	}
	e.resetPreferredColumn()
}

func (e *vimEditor) moveRight() {
	last := e.lineLastPosition(e.cursor)
	if e.cursor < last {
		e.cursor++
	}
	e.resetPreferredColumn()
}

func (e *vimEditor) moveVertical(delta int) {
	start := e.lineStart(e.cursor)
	if !e.hasPreferredCol {
		e.preferredColumn = e.cursor - start
		e.hasPreferredCol = true
	}

	targetStart := start
	if delta < 0 {
		if start == 0 {
			return
		}
		targetStart = e.lineStart(start - 1)
	} else {
		end := e.lineEnd(e.cursor)
		if end >= len(e.buffer) {
			return
		}
		targetStart = end + 1
	}

	targetLast := e.lineLastPosition(targetStart)
	e.cursor = min(targetStart+e.preferredColumn, targetLast)
}

func (e *vimEditor) moveInsertVertical(delta int) {
	start := e.lineStart(e.cursor)
	if !e.hasPreferredCol {
		e.preferredColumn = e.cursor - start
		e.hasPreferredCol = true
	}

	targetStart := start
	if delta < 0 {
		if start == 0 {
			return
		}
		targetStart = e.lineStart(start - 1)
	} else {
		end := e.lineEnd(e.cursor)
		if end >= len(e.buffer) {
			return
		}
		targetStart = end + 1
	}
	e.cursor = min(targetStart+e.preferredColumn, e.lineEnd(targetStart))
}

func (e *vimEditor) resetPreferredColumn() {
	e.hasPreferredCol = false
}

func (e vimEditor) lineStart(position int) int {
	position = clampInt(position, 0, len(e.buffer))
	for position > 0 && e.buffer[position-1] != '\n' {
		position--
	}
	return position
}

func (e vimEditor) lineEnd(position int) int {
	position = clampInt(position, 0, len(e.buffer))
	for position < len(e.buffer) && e.buffer[position] != '\n' {
		position++
	}
	return position
}

func (e vimEditor) lineLastPosition(position int) int {
	start := e.lineStart(position)
	end := e.lineEnd(position)
	if end == start {
		return start
	}
	return end - 1
}

func (e vimEditor) firstNonBlank(position int) int {
	start := e.lineStart(position)
	end := e.lineEnd(position)
	for start < end && unicode.IsSpace(e.buffer[start]) {
		start++
	}
	return start
}

func (e vimEditor) wordForward(position int) int {
	if len(e.buffer) == 0 {
		return 0
	}
	position = e.wordForwardBoundary(position)
	if position >= len(e.buffer) {
		return e.lastPosition()
	}
	return position
}

func (e vimEditor) wordForwardBoundary(position int) int {
	if len(e.buffer) == 0 {
		return 0
	}
	position = clampInt(position, 0, len(e.buffer)-1)
	class := vimRuneClass(e.buffer[position])
	for position < len(e.buffer) && vimRuneClass(e.buffer[position]) == class {
		position++
	}
	for position < len(e.buffer) && vimRuneClass(e.buffer[position]) == vimSpace {
		position++
	}
	return position
}

func (e vimEditor) wordBackward(position int) int {
	if len(e.buffer) == 0 || position <= 0 {
		return 0
	}
	position = min(position-1, len(e.buffer)-1)
	for position > 0 && vimRuneClass(e.buffer[position]) == vimSpace {
		position--
	}
	class := vimRuneClass(e.buffer[position])
	for position > 0 && vimRuneClass(e.buffer[position-1]) == class {
		position--
	}
	return position
}

func (e vimEditor) wordEnd(position int) int {
	if len(e.buffer) == 0 {
		return 0
	}
	position = clampInt(position, 0, len(e.buffer)-1)
	class := vimRuneClass(e.buffer[position])
	if class != vimSpace && position < len(e.buffer)-1 && vimRuneClass(e.buffer[position+1]) == class {
		for position < len(e.buffer)-1 && vimRuneClass(e.buffer[position+1]) == class {
			position++
		}
		return position
	}
	if position < len(e.buffer)-1 {
		position++
	}
	for position < len(e.buffer)-1 && vimRuneClass(e.buffer[position]) == vimSpace {
		position++
	}
	class = vimRuneClass(e.buffer[position])
	for position < len(e.buffer)-1 && vimRuneClass(e.buffer[position+1]) == class {
		position++
	}
	return position
}

func (e *vimEditor) clampNormalCursor() {
	if len(e.buffer) == 0 {
		e.cursor = 0
		return
	}
	e.cursor = clampInt(e.cursor, 0, len(e.buffer)-1)
}

func (e vimEditor) lastPosition() int {
	if len(e.buffer) == 0 {
		return 0
	}
	return len(e.buffer) - 1
}

func (e vimEditor) Value() string {
	return string(e.buffer)
}

func (e vimEditor) Cursor() int {
	return e.cursor
}

func (e vimEditor) Mode() vimMode {
	return e.mode
}

func (e vimEditor) SelectionRange() (int, int, bool) {
	if e.mode != vimVisual || len(e.buffer) == 0 {
		return 0, 0, false
	}
	start, end := e.visualAnchor, e.cursor
	if start > end {
		start, end = end, start
	}
	end++
	return clampInt(start, 0, len(e.buffer)), clampInt(end, 0, len(e.buffer)), true
}

type vimRuneKind uint8

const (
	vimSpace vimRuneKind = iota
	vimKeyword
	vimPunctuation
)

func vimRuneClass(r rune) vimRuneKind {
	if unicode.IsSpace(r) {
		return vimSpace
	}
	if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) {
		return vimKeyword
	}
	return vimPunctuation
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
