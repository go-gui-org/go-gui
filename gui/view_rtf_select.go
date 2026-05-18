package gui

// view_rtf_select.go implements text selection for RTF widgets.
// When an RTF widget has IDFocus set, it can be selected with
// mouse drag, keyboard shortcuts (Shift+arrows, Ctrl+A), and
// copied to clipboard with Ctrl+C.

import (
	"github.com/mike-ward/go-glyph"
)

// rtfSelectionEventHandlers returns event handlers for RTF widgets
// with text selection support (IDFocus > 0).
func rtfSelectionEventHandlers() *EventHandlers {
	return &EventHandlers{
		OnClick:     rtfSelectionOnClick,
		OnMouseMove: rtfSelectionOnMouseMove,
		OnKeyDown:   rtfSelectionOnKeyDown,
		AmendLayout: rtfSelectionAmendLayout,
	}
}

// rtfSelectionOnClick handles selection start/end and Ctrl+A for RTF.
func rtfSelectionOnClick(l *Layout, e *Event, w *Window) {
	if !l.Shape.HasRtfLayout() || l.Shape.IDFocus == 0 {
		return
	}
	idFocus := l.Shape.IDFocus
	rt := l.Shape.TC.RtfRuns
	if rt == nil {
		return
	}

	// Set focus when clicked.
	w.SetIDFocus(idFocus)

	// Convert click position to rune index.
	runIdx := rtfRunIndexAtPoint(l, e.MouseX, e.MouseY)
	if runIdx < 0 {
		return
	}

	// Single click: place cursor.
	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	is, _ := imap.Get(idFocus)
	is.CursorPos = runIdx
	is.SelectBeg = 0
	is.SelectEnd = 0
	imap.Set(idFocus, is)
	e.IsHandled = true
}

// rtfSelectionOnMouseMove handles mouse drag selection for RTF.
func rtfSelectionOnMouseMove(l *Layout, e *Event, w *Window) {
	if !l.Shape.HasRtfLayout() || l.Shape.IDFocus == 0 {
		return
	}
	idFocus := l.Shape.IDFocus

	// Only select if mouse button 1 is pressed (drag).
	if e.MouseButton != MouseButton1 {
		return
	}

	runIdx := rtfRunIndexAtPoint(l, e.MouseX, e.MouseY)
	if runIdx < 0 {
		return
	}

	// Update selection during drag.
	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	is, _ := imap.Get(idFocus)

	// If starting a new drag (no selection yet), set both anchor and end.
	if is.SelectBeg == is.SelectEnd {
		is.SelectBeg = uint32(is.CursorPos)
	}
	is.SelectEnd = uint32(runIdx)
	is.CursorPos = runIdx
	imap.Set(idFocus, is)
	e.IsHandled = true
}

// rtfSelectionOnKeyDown handles keyboard selection (Shift+arrows, Ctrl+A, Ctrl+C).
func rtfSelectionOnKeyDown(l *Layout, e *Event, w *Window) {
	if !l.Shape.HasRtfLayout() || l.Shape.IDFocus == 0 {
		return
	}
	idFocus := l.Shape.IDFocus
	rt := l.Shape.TC.RtfRuns
	if rt == nil {
		return
	}

	totalRunes := 0
	for _, run := range rt.Runs {
		totalRunes += len([]rune(run.Text))
	}

	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	is, _ := imap.Get(idFocus)
	isShift := e.Modifiers.Has(ModShift)
	isCtrl := e.Modifiers.HasAny(ModCtrl, ModSuper)

	handled := true
	switch e.KeyCode {
	case KeyLeft:
		newPos := is.CursorPos - 1
		if newPos < 0 {
			newPos = 0
		}
		readOnlyUpdateSelection(imap, idFocus, is, newPos, isShift)

	case KeyRight:
		newPos := is.CursorPos + 1
		if newPos > totalRunes {
			newPos = totalRunes
		}
		readOnlyUpdateSelection(imap, idFocus, is, newPos, isShift)

	case KeyHome:
		readOnlyUpdateSelection(imap, idFocus, is, 0, isShift)

	case KeyEnd:
		readOnlyUpdateSelection(imap, idFocus, is, totalRunes, isShift)

	case KeyA:
		if isCtrl {
			readOnlySelectAll(idFocus, totalRunes, w)
			handled = true
		} else {
			handled = false
		}

	case KeyC:
		if isCtrl {
			// Copy selected text to clipboard.
			readOnlySelectionCopy(idFocus, w, func(beg, end uint32) string {
				return rtfSelectedText(rt, beg, end)
			})
			handled = true
		} else {
			handled = false
		}

	default:
		handled = false
	}

	if handled {
		e.IsHandled = true
	}
}

// rtfSelectionAmendLayout copies InputState selection to shape.TC
// for rendering selection highlight.
func rtfSelectionAmendLayout(l *Layout, w *Window) {
	if l.Shape.IDFocus == 0 || l.Shape.TC == nil {
		return
	}
	is := StateReadOr(w, nsInput, l.Shape.IDFocus, InputState{})
	l.Shape.TC.TextSelBeg = is.SelectBeg
	l.Shape.TC.TextSelEnd = is.SelectEnd
}

// rtfRunIndexAtPoint finds the rune index at a screen coordinate
// by iterating through glyph layout items and hit-testing.
func rtfRunIndexAtPoint(l *Layout, x, y float32) int {
	if !l.Shape.HasRtfLayout() {
		return -1
	}

	layout := l.Shape.TC.RtfLayout
	relX := x - (l.Shape.X + l.Shape.PaddingLeft())
	relY := y - (l.Shape.Y + l.Shape.PaddingTop())

	// Find the glyph item containing this point.
	var bestItem *glyph.Item
	for i := range layout.Items {
		item := &layout.Items[i]
		if item.IsObject {
			continue
		}
		r := rtfRunRect(*item)
		if relX >= r.X && relY >= r.Y &&
			relX < r.X+r.Width && relY < r.Y+r.Height {
			bestItem = item
			break
		}
	}

	if bestItem == nil {
		// Not in any item; return end of text.
		rt := l.Shape.TC.RtfRuns
		if rt == nil {
			return 0
		}
		totalRunes := 0
		for _, run := range rt.Runs {
			totalRunes += len([]rune(run.Text))
		}
		return totalRunes
	}

	return bestItem.StartIndex
}
