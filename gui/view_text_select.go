package gui

import (
	"time"

	"github.com/go-gui-org/go-glyph"
)

const (
	animIDTextDragScroll   = "text-drag-scroll"
	doubleClickThresholdMs = 400
)

// textOnClick handles click events for text selection.
// Single-click places cursor, double-click selects word,
// drag-to-select via MouseLock.
func textOnClick(layout *Layout, e *Event, w *Window) {
	shape := layout.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable {
		return
	}
	w.SetFocus(shape.ID)

	text := shape.TC.Text
	style := textStyleOrDefault(shape)
	gl, glOK := inputGlyphLayout(text, shape, style, w)

	// e.MouseX/Y are already relative to shape via
	// eventRelativeTo in executeMouseCallback.
	relX := e.MouseX
	relY := e.MouseY

	var runePos int
	if glOK {
		byteIdx := gl.GetClosestOffset(relX, relY)
		runePos = byteToRuneIndex(text, byteIdx)
	} else {
		// Fallback for tests (no glyph backend).
		charWidth := style.Size * 0.6
		if charWidth <= 0 {
			charWidth = 14 * 0.6
		}
		runePos = int(relX / charWidth)
		runeLen := utf8RuneCount(text)
		runePos = intClamp(runePos, 0, runeLen)
	}

	focusID := shape.ID
	imap := StateMap[string, InputState](
		w, nsInput, capMany,
	)
	is, _ := imap.Get(focusID)

	// Double-click detection.
	now := time.Now().UnixMilli()
	doubleClick := is.LastClickTime > 0 &&
		now-is.LastClickTime <= doubleClickThresholdMs
	is.LastClickTime = now

	if doubleClick {
		var beg, end int
		if glOK {
			byteIdx := runeToByteIndex(text, runePos)
			bBeg, bEnd := gl.GetWordAtIndex(byteIdx)
			beg = byteToRuneIndex(text, bBeg)
			end = byteToRuneIndex(text, bEnd)
		} else {
			beg, end = wordBoundsAt(
				[]rune(text), runePos)
		}
		is.CursorPos = end
		is.SelectBeg = uint32(beg)
		is.SelectEnd = uint32(end)
	} else {
		is.CursorPos = runePos
		is.SelectBeg = uint32(runePos)
		is.SelectEnd = uint32(runePos)
	}
	is.CursorOffset = -1
	imap.Set(focusID, is)
	resetBlinkCursorVisible(w)
	e.IsHandled = true

	// Drag-to-select via MouseLock.
	anchorPos := is.SelectBeg
	anchorEnd := is.SelectEnd
	dragGL := gl
	dragGLOK := glOK
	dragFocusID := focusID
	dragShapeX := shape.X
	dragShapeY := shape.Y

	// Find nearest scroll ancestor.
	var lastMouseX, lastMouseY float32
	scrollID := ""
	dragScrollY0 := float32(0)
	viewTop := float32(0)
	viewBot := float32(0)
	maxScrollNeg := float32(0)
	for p := layout.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Scrollable {
			scrollID = p.Shape.ID
			sy := w.scrollY()
			dragScrollY0, _ = sy.Get(scrollID)
			sp := p.Shape
			viewTop = sp.Y + sp.Padding.Top
			viewH := sp.Height - sp.paddingHeight()
			viewBot = viewTop + viewH
			maxScrollNeg = f32Min(0,
				viewH-contentHeight(p))
			break
		}
	}

	computeRunePos := func(mx, my float32,
		w *Window,
	) int {
		if dragGLOK {
			scrollDelta := float32(0)
			if scrollID != "" {
				sy := w.scrollY()
				sNow, _ := sy.Get(scrollID)
				scrollDelta = sNow - dragScrollY0
			}
			rx := mx - dragShapeX
			ry := my - (dragShapeY + scrollDelta)
			bi := dragGL.GetClosestOffset(rx, ry)
			return byteToRuneIndex(text, bi)
		}
		cw := style.Size * 0.6
		if cw <= 0 {
			cw = 14 * 0.6
		}
		rp := int((mx - dragShapeX) / cw)
		rl := utf8RuneCount(text)
		rp = intClamp(rp, 0, rl)
		return rp
	}

	runes := []rune(text)
	updateDragSelection := func(rp int, w *Window) {
		dim := StateMap[string, InputState](
			w, nsInput, capMany,
		)
		dis, _ := dim.Get(dragFocusID)
		if doubleClick {
			var wb, we int
			if dragGLOK {
				bi := runeToByteIndex(text, rp)
				bBeg, bEnd := dragGL.GetWordAtIndex(bi)
				wb = byteToRuneIndex(text, bBeg)
				we = byteToRuneIndex(text, bEnd)
			} else {
				wb, we = wordBoundsAt(runes, rp)
			}
			if rp < int(anchorPos) {
				dis.SelectBeg = anchorEnd
				dis.SelectEnd = uint32(wb)
				dis.CursorPos = wb
			} else {
				dis.SelectBeg = anchorPos
				dis.SelectEnd = uint32(we)
				dis.CursorPos = we
			}
		} else {
			dis.CursorPos = rp
			dis.SelectBeg = anchorPos
			dis.SelectEnd = uint32(rp)
		}
		dis.CursorOffset = -1
		dim.Set(dragFocusID, dis)
		resetBlinkCursorVisible(w)
	}

	dragScrollCB := func(_ *Animate, w *Window) {
		var delta float32
		if lastMouseY < viewTop {
			delta = (viewTop - lastMouseY) * 0.3
		} else if lastMouseY > viewBot {
			delta = -((lastMouseY - viewBot) * 0.3)
		} else {
			w.AnimationRemove(animIDTextDragScroll)
			return
		}
		sy := w.scrollY()
		cur, _ := sy.Get(scrollID)
		newScroll := f32Clamp(
			cur+delta, maxScrollNeg, 0)
		if newScroll == cur {
			return
		}
		sy.Set(scrollID, newScroll)
		rp := computeRunePos(
			lastMouseX, lastMouseY, w)
		updateDragSelection(rp, w)
	}

	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			lastMouseX = e.MouseX
			lastMouseY = e.MouseY
			rp := computeRunePos(
				e.MouseX, e.MouseY, w)
			updateDragSelection(rp, w)
			if scrollID != "" {
				outside := e.MouseY < viewTop ||
					e.MouseY > viewBot
				if outside && !w.HasAnimation(
					animIDTextDragScroll) {
					w.AnimationAdd(&Animate{
						AnimID:   animIDTextDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: dragScrollCB,
					})
				} else if !outside {
					w.AnimationRemove(
						animIDTextDragScroll)
				}
			}
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.AnimationRemove(animIDTextDragScroll)
			w.MouseUnlock()
		},
	})
}

// textOnKeyDown is a read-only key handler for text navigation
// and copy. No editing keys (paste, cut, delete).
func textOnKeyDown(layout *Layout, e *Event, w *Window) {
	shape := layout.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable ||
		!w.IsFocus(shape.ID) {
		return
	}
	id := shape.ID
	text := shape.TC.Text
	imap := StateMap[string, InputState](
		w, nsInput, capMany,
	)
	is, _ := imap.Get(id)
	savedOffset := is.CursorOffset
	savedTrailing := is.CursorTrailing
	is.CursorOffset = -1
	is.CursorTrailing = false
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	isShift := e.Modifiers.Has(ModShift)
	isWordMod := e.Modifiers.HasAny(
		ModCtrl, ModAlt, ModSuper,
	)
	handled := true

	gl, glOK := inputGlyphLayout(
		text, shape, textStyleOrDefault(shape), w,
	)

	switch e.KeyCode {
	case KeyLeft:
		inputKeyLeft(imap, id, is, text, pos,
			isShift, isWordMod, gl, glOK)
	case KeyRight:
		inputKeyRight(imap, id, is, text, pos, runeLen,
			isShift, isWordMod, gl, glOK)
	case KeyHome:
		inputKeyHome(imap, id, is, text, pos,
			isShift, savedTrailing, gl, glOK)
	case KeyEnd:
		inputKeyEnd(imap, id, is, text, pos,
			isShift, savedTrailing, gl, glOK)
	case KeyUp:
		handled = textKeyVertical(imap, id, is, text,
			pos, isShift, savedOffset, true,
			shape.TC.TextMode, gl, glOK)
	case KeyDown:
		handled = textKeyVertical(imap, id, is, text,
			pos, isShift, savedOffset, false,
			shape.TC.TextMode, gl, glOK)
	case KeyEscape:
		inputKeyEscape(imap, id, is)
		handled = false
	case KeyA:
		if e.Modifiers.HasAny(ModCtrl, ModSuper) {
			inputSelectAll(text, id, w)
		} else {
			handled = false
		}
	case KeyC:
		handled = inputKeyCopy(
			text, id, shape.TC.TextIsPassword, e, w)
	default:
		handled = false
	}

	if handled {
		resetBlinkCursorVisible(w)
		textScrollCursorIntoView(layout, w)
		e.IsHandled = true
	}
}

// textKeyVertical handles KeyUp/KeyDown for text selection.
// Returns false when the key is unhandled (single-line mode).
func textKeyVertical(
	imap *BoundedMap[string, InputState], id string, is InputState,
	text string, pos int, isShift bool,
	savedOffset float32, up bool, mode TextMode,
	gl glyph.Layout, glOK bool,
) bool {
	if mode == TextModeSingleLine {
		return false
	}
	var newPos int
	if glOK {
		bi := runeToByteIndex(text, pos)
		px := savedOffset
		if px < 0 {
			if cp, ok := gl.GetCursorPos(bi); ok {
				px = cp.X
			}
		}
		is.CursorOffset = px
		if up {
			newPos = byteToRuneIndex(text,
				gl.MoveCursorUp(bi, px))
		} else {
			newPos = byteToRuneIndex(text,
				gl.MoveCursorDown(bi, px))
		}
	} else {
		if up {
			newPos = moveCursorUp([]rune(text), pos)
		} else {
			newPos = moveCursorDown([]rune(text), pos)
		}
	}
	updateCursorAndSelection(imap, id, is, newPos, isShift)
	return true
}

// textAmendLayout copies InputState selection to the shape's
// TextSelBeg/TextSelEnd for rendering. Unlike input's nested
// structure, text is a flat shape — no child traversal needed.
func textAmendLayout(layout *Layout, w *Window) {
	if layout.Shape.ID == "" || !layout.Shape.Focusable || layout.Shape.TC == nil {
		return
	}
	is := StateReadOr(
		w, nsInput, layout.Shape.ID, InputState{},
	)
	layout.Shape.TC.TextSelBeg = is.SelectBeg
	layout.Shape.TC.TextSelEnd = is.SelectEnd
}
