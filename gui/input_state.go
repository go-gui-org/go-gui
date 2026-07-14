package gui

const inputMaxInsertRunes = 65_536

// InputState manages cursor, selection, and undo/redo for
// an input field. Stored in StateRegistry keyed by ID.
type InputState struct {
	Undo           *BoundedStack[InputMemento]
	Redo           *BoundedStack[InputMemento]
	CursorPos      int
	LastClickTime  int64
	SelectBeg      uint32
	SelectEnd      uint32
	CursorOffset   float32
	CursorTrailing bool // prefer end-of-previous-line at wrap boundaries
}

// InputMemento stores a snapshot for undo/redo.
type InputMemento struct {
	Text         string
	CursorPos    int
	SelectBeg    uint32
	SelectEnd    uint32
	CursorOffset float32
}

// InputMode selects single-line or multiline behavior.
type InputMode uint8

// InputMode constants.
const (
	InputSingleLine InputMode = iota
	InputMultiline
)

// InputCommitReason identifies why text was committed.
type InputCommitReason uint8

// InputCommitReason constants.
const (
	CommitEnter InputCommitReason = iota
	CommitBlur
)

const undoMaxSize = 50

func inputStateOrDefault(focusID string, w *Window) InputState {
	m := StateMap[string, InputState](w, nsInput, capMany)
	if v, ok := m.Get(focusID); ok {
		return v
	}
	return InputState{}
}

func inputMementoFromState(text string, is InputState) InputMemento {
	return InputMemento{
		Text:         text,
		CursorPos:    is.CursorPos,
		SelectBeg:    is.SelectBeg,
		SelectEnd:    is.SelectEnd,
		CursorOffset: is.CursorOffset,
	}
}

func inputPushUndo(is InputState, text string) *BoundedStack[InputMemento] {
	stack := is.Undo
	if stack == nil {
		stack = NewBoundedStack[InputMemento](undoMaxSize)
	}
	stack.Push(inputMementoFromState(text, is))
	return stack
}

func inputStateFromMemento(m InputMemento, undo, redo *BoundedStack[InputMemento]) InputState {
	return InputState{
		CursorPos:    m.CursorPos,
		SelectBeg:    m.SelectBeg,
		SelectEnd:    m.SelectEnd,
		CursorOffset: m.CursorOffset,
		Undo:         undo,
		Redo:         redo,
	}
}

// inputProposedText returns the text that would result from
// inserting insertText at the cursor without modifying state.
func inputProposedText(text, insertText string, focusID string, w *Window) string {
	if len(insertText) == 0 {
		return text
	}
	insertRunes := []rune(insertText)
	if len(insertRunes) > inputMaxInsertRunes {
		insertRunes = insertRunes[:inputMaxInsertRunes]
	}
	runes := []rune(text)
	is := inputStateOrDefault(focusID, w)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		return text + string(insertRunes)
	}
	if is.SelectBeg != is.SelectEnd {
		beg, end := u32Sort(is.SelectBeg, is.SelectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text
		}
		result := make([]rune, 0, int(beg)+len(insertRunes)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, insertRunes...)
		result = append(result, runes[end:]...)
		return string(result)
	}
	result := make([]rune, 0, len(runes)+len(insertRunes))
	result = append(result, runes[:cursorPos]...)
	result = append(result, insertRunes...)
	result = append(result, runes[cursorPos:]...)
	return string(result)
}

// inputInsert inserts text at cursor or replaces selection.
// Returns resulting text.
func inputInsert(text string, insertText string, focusID string, w *Window) string {
	if len(insertText) == 0 {
		return text
	}
	insertRunes := []rune(insertText)
	if len(insertRunes) > inputMaxInsertRunes {
		insertRunes = insertRunes[:inputMaxInsertRunes]
	}

	runes := []rune(text)
	is := inputStateOrDefault(focusID, w)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		runes = append([]rune(text), insertRunes...)
		cursorPos = len(runes)
	} else if is.SelectBeg != is.SelectEnd {
		beg, end := u32Sort(is.SelectBeg, is.SelectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text
		}
		result := make([]rune, 0, int(beg)+len(insertRunes)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, insertRunes...)
		result = append(result, runes[end:]...)
		runes = result
		cursorPos = min(int(beg)+len(insertRunes), len(runes))
	} else {
		result := make([]rune, 0, cursorPos+len(insertRunes)+(len(runes)-cursorPos))
		result = append(result, runes[:cursorPos]...)
		result = append(result, insertRunes...)
		result = append(result, runes[cursorPos:]...)
		runes = result
		cursorPos = min(cursorPos+len(insertRunes), len(runes))
	}

	nextText := string(runes)
	undo := inputPushUndo(is, text)
	imap := StateMap[string, InputState](w, nsInput, capMany)
	imap.Set(focusID, InputState{
		CursorPos:    cursorPos,
		CursorOffset: -1,
		Undo:         undo,
	})
	return nextText
}

// inputSetTextAndCursorAtEnd pushes undo and places cursor at end
// of newText. Used when PreTextChange returns adjusted text where
// positional cursor mapping is unreliable.
func inputSetTextAndCursorAtEnd(oldText, newText string, focusID string, w *Window) {
	is := inputStateOrDefault(focusID, w)
	undo := inputPushUndo(is, oldText)
	imap := StateMap[string, InputState](w, nsInput, capMany)
	imap.Set(focusID, InputState{
		CursorPos:    utf8RuneCount(newText),
		CursorOffset: -1,
		Undo:         undo,
	})
}

// inputDelete removes text at cursor or selected range.
// forwardDelete=true for Delete key, false for Backspace.
func inputDelete(text string, focusID string, forwardDelete bool, w *Window) (string, bool) {
	runes := []rune(text)
	is := inputStateOrDefault(focusID, w)
	cursorPos := min(is.CursorPos, len(runes))
	if cursorPos < 0 {
		cursorPos = len(runes)
	}

	if is.SelectBeg != is.SelectEnd {
		beg, end := u32Sort(is.SelectBeg, is.SelectEnd)
		if int(beg) >= len(runes) || int(end) > len(runes) {
			return text, false
		}
		result := make([]rune, 0, int(beg)+(len(runes)-int(end)))
		result = append(result, runes[:beg]...)
		result = append(result, runes[end:]...)
		runes = result
		cursorPos = min(int(beg), len(runes))
	} else {
		if cursorPos == 0 && !forwardDelete {
			return text, true
		}
		if cursorPos == len(runes) && forwardDelete {
			return text, true
		}
		delPos := cursorPos
		if !forwardDelete {
			delPos = cursorPos - 1
		}
		if delPos < 0 || delPos >= len(runes) {
			return text, false
		}
		result := make([]rune, 0, len(runes)-1)
		result = append(result, runes[:delPos]...)
		result = append(result, runes[delPos+1:]...)
		runes = result
		if !forwardDelete {
			cursorPos--
		}
	}

	nextText := string(runes)
	undo := inputPushUndo(is, text)
	imap := StateMap[string, InputState](w, nsInput, capMany)
	imap.Set(focusID, InputState{
		CursorPos:    cursorPos,
		CursorOffset: -1,
		Undo:         undo,
	})
	return nextText, true
}

// inputCopy returns the selected text. Returns ("", false) if
// no selection or password mode.
func inputCopy(text string, focusID string, isPassword bool, w *Window) (string, bool) {
	if isPassword {
		return "", false
	}
	is := StateReadOr(w, nsInput, focusID, InputState{})
	if is.SelectBeg == is.SelectEnd {
		return "", false
	}
	beg, end := u32Sort(is.SelectBeg, is.SelectEnd)
	runeCount := utf8RuneCount(text)
	if int(beg) > runeCount || int(end) > runeCount || beg >= end {
		return "", false
	}
	begByte := runeToByteIndex(text, int(beg))
	endByte := runeToByteIndex(text, int(end))
	return text[begByte:endByte], true
}

// inputCut copies selected text then deletes it.
func inputCut(text string, focusID string, isPassword bool, w *Window) (string, string, bool) {
	if isPassword {
		return text, "", false
	}
	copied, ok := inputCopy(text, focusID, false, w)
	if !ok {
		return text, "", false
	}
	newText, _ := inputDelete(text, focusID, false, w)
	return newText, copied, true
}

// inputUndo reverts to previous state. Returns restored text.
func inputUndo(text string, focusID string, w *Window) string {
	imap := StateMap[string, InputState](w, nsInput, capMany)
	is, _ := imap.Get(focusID)
	if is.Undo == nil || is.Undo.IsEmpty() {
		return text
	}
	memento, ok := is.Undo.Pop()
	if !ok {
		return text
	}
	redo := is.Redo
	if redo == nil {
		redo = NewBoundedStack[InputMemento](undoMaxSize)
	}
	redo.Push(inputMementoFromState(text, is))
	imap.Set(focusID, inputStateFromMemento(memento, is.Undo, redo))
	return memento.Text
}

// inputRedo reapplies a previously undone operation.
func inputRedo(text string, focusID string, w *Window) string {
	imap := StateMap[string, InputState](w, nsInput, capMany)
	is, _ := imap.Get(focusID)
	if is.Redo == nil || is.Redo.IsEmpty() {
		return text
	}
	memento, ok := is.Redo.Pop()
	if !ok {
		return text
	}
	undo := is.Undo
	if undo == nil {
		undo = NewBoundedStack[InputMemento](undoMaxSize)
	}
	undo.Push(inputMementoFromState(text, is))
	imap.Set(focusID, inputStateFromMemento(memento, undo, is.Redo))
	return memento.Text
}

// inputSelectAll selects all text.
func inputSelectAll(text string, focusID string, w *Window) {
	runeCount := utf8RuneCount(text)
	imap := StateMap[string, InputState](w, nsInput, capMany)
	is, _ := imap.Get(focusID)
	is.SelectBeg = 0
	is.SelectEnd = uint32(runeCount)
	is.CursorPos = runeCount
	imap.Set(focusID, is)
}

// updateCursorAndSelection moves cursor to newPos, extending
// or resetting selection based on shift modifier.
func updateCursorAndSelection(
	imap *BoundedMap[string, InputState],
	focusID string,
	is InputState,
	newPos int,
	isShift bool,
) {
	if isShift {
		if is.SelectBeg == is.SelectEnd {
			// Start new selection from current cursor.
			is.SelectBeg = uint32(is.CursorPos)
			is.SelectEnd = uint32(newPos)
		} else {
			// Extend: move the end that matches current cursor.
			if uint32(is.CursorPos) == is.SelectEnd {
				is.SelectEnd = uint32(newPos)
			} else {
				is.SelectBeg = uint32(newPos)
			}
		}
	} else {
		is.SelectBeg = 0
		is.SelectEnd = 0
	}
	is.CursorPos = newPos
	imap.Set(focusID, is)
}

// moveCursorWordLeft scans backwards to the previous word boundary.
func moveCursorWordLeft(runes []rune, pos int) int {
	if pos <= 0 {
		return 0
	}
	i := pos - 1
	// Skip whitespace.
	for i > 0 && isWordSep(runes[i]) {
		i--
	}
	// Skip word characters.
	for i > 0 && !isWordSep(runes[i-1]) {
		i--
	}
	return i
}

// moveCursorWordRight scans forward to the next word boundary.
func moveCursorWordRight(runes []rune, pos int) int {
	n := len(runes)
	if pos >= n {
		return n
	}
	i := pos
	// Skip word characters.
	for i < n && !isWordSep(runes[i]) {
		i++
	}
	// Skip whitespace.
	for i < n && isWordSep(runes[i]) {
		i++
	}
	return i
}

func isWordSep(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// wordBoundsAt returns the start and end rune indices of the word
// surrounding pos. If pos is on a separator, selects the separator run.
func wordBoundsAt(runes []rune, pos int) (int, int) {
	n := len(runes)
	if n == 0 {
		return 0, 0
	}
	if pos >= n {
		pos = n - 1
	}
	pos = max(pos, 0)
	sep := isWordSep(runes[pos])
	beg := pos
	for beg > 0 && isWordSep(runes[beg-1]) == sep {
		beg--
	}
	end := pos + 1
	for end < n && isWordSep(runes[end]) == sep {
		end++
	}
	return beg, end
}

// moveCursorUp moves cursor up one line in multiline text.
func moveCursorUp(runes []rune, pos int) int {
	// Find start of current line.
	lineStart := pos
	for lineStart > 0 && runes[lineStart-1] != '\n' {
		lineStart--
	}
	if lineStart == 0 {
		return 0 // Already on first line.
	}
	col := pos - lineStart
	// Find start of previous line.
	prevLineEnd := lineStart - 1
	prevLineStart := prevLineEnd
	for prevLineStart > 0 && runes[prevLineStart-1] != '\n' {
		prevLineStart--
	}
	prevLineLen := prevLineEnd - prevLineStart
	col = min(col, prevLineLen)
	return prevLineStart + col
}

// moveCursorDown moves cursor down one line in multiline text.
func moveCursorDown(runes []rune, pos int) int {
	n := len(runes)
	// Find start of current line.
	lineStart := pos
	for lineStart > 0 && runes[lineStart-1] != '\n' {
		lineStart--
	}
	col := pos - lineStart
	// Find end of current line (next \n).
	lineEnd := pos
	for lineEnd < n && runes[lineEnd] != '\n' {
		lineEnd++
	}
	if lineEnd >= n {
		return n // Already on last line.
	}
	// Next line starts after \n.
	nextLineStart := lineEnd + 1
	nextLineEnd := nextLineStart
	for nextLineEnd < n && runes[nextLineEnd] != '\n' {
		nextLineEnd++
	}
	nextLineLen := nextLineEnd - nextLineStart
	col = min(col, nextLineLen)
	return nextLineStart + col
}

// moveCursorLineStart returns the start of the current line.
func moveCursorLineStart(runes []rune, pos int) int {
	for pos > 0 && runes[pos-1] != '\n' {
		pos--
	}
	return pos
}

// moveCursorLineEnd returns the end of the current line.
func moveCursorLineEnd(runes []rune, pos int) int {
	n := len(runes)
	for pos < n && runes[pos] != '\n' {
		pos++
	}
	return pos
}
