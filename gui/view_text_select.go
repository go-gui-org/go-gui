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
func textOnClick(ctx EventCtx) {
	shape := ctx.Layout.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable {
		return
	}
	ctx.Window.SetFocus(shape.idKey())

	text := shape.TC.Text
	style := textStyleOrDefault(shape)
	gl, glOK := inputGlyphLayout(text, shape, style, ctx.Window)

	// e.MouseX/Y are already relative to shape via
	// eventRelativeTo in executeMouseCallback.
	relX := ctx.Event.MouseX
	relY := ctx.Event.MouseY

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

	focusID := shape.idKey()
	imap := StateMap[string, inputState](
		ctx.Window, nsInput, capMany,
	)
	// Default InputState{}: zero value seeds initial selection/cursor state.
	is := imap.GetOr(focusID, inputState{})

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
		is.selectBeg = uint32(beg)
		is.selectEnd = uint32(end)
	} else {
		is.CursorPos = runePos
		is.selectBeg = uint32(runePos)
		is.selectEnd = uint32(runePos)
	}
	is.cursorOffset = -1
	imap.Set(focusID, is)
	resetBlinkCursorVisible(ctx.Window)
	ctx.Consume()

	// Drag-to-select via MouseLock.
	anchorPos := is.selectBeg
	anchorEnd := is.selectEnd
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
	for p := ctx.Layout.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Scrollable {
			scrollID = p.Shape.idKey()
			sy := ctx.Window.scrollY()
			// Default 0: unscrolled position when no offset recorded yet.
			dragScrollY0 = sy.GetOr(scrollID, 0)
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
				// Default 0: unscrolled position when no offset recorded yet.
				sNow := sy.GetOr(scrollID, 0)
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
		dim := StateMap[string, inputState](
			w, nsInput, capMany,
		)
		// Default InputState{}: zero value seeds initial drag-edit state.
		dis := dim.GetOr(dragFocusID, inputState{})
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
				dis.selectBeg = anchorEnd
				dis.selectEnd = uint32(wb)
				dis.CursorPos = wb
			} else {
				dis.selectBeg = anchorPos
				dis.selectEnd = uint32(we)
				dis.CursorPos = we
			}
		} else {
			dis.CursorPos = rp
			dis.selectBeg = anchorPos
			dis.selectEnd = uint32(rp)
		}
		dis.cursorOffset = -1
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
		// Default 0: unscrolled position when no offset recorded yet.
		cur := sy.GetOr(scrollID, 0)
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

	ctx.Window.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {
			lastMouseX = ctx.Event.MouseX
			lastMouseY = ctx.Event.MouseY
			rp := computeRunePos(
				ctx.Event.MouseX, ctx.Event.MouseY, ctx.Window)
			updateDragSelection(rp, ctx.Window)
			if scrollID != "" {
				outside := ctx.Event.MouseY < viewTop ||
					ctx.Event.MouseY > viewBot
				if outside && !ctx.Window.HasAnimation(
					animIDTextDragScroll) {
					ctx.Window.AnimationAdd(&Animate{
						AnimID:   animIDTextDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: dragScrollCB,
					})
				} else if !outside {
					ctx.Window.AnimationRemove(
						animIDTextDragScroll)
				}
			}
		},
		MouseUp: func(ctx EventCtx) {
			ctx.Window.AnimationRemove(animIDTextDragScroll)
			ctx.Window.MouseUnlock()
		},
		Cancel: func(w *Window) {
			// Stop the edge-scroll animation. Nothing else unwinds
			// it once the lock is gone, so it would keep scrolling
			// and extending the selection on its own.
			w.AnimationRemove(animIDTextDragScroll)
		},
	})
}

// textOnKeyDown is a read-only key handler for text navigation
// and copy. No editing keys (paste, cut, delete).
func textOnKeyDown(ctx EventCtx) {
	shape := ctx.Layout.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable ||
		!ctx.Window.IsFocus(shape.idKey()) {
		return
	}
	id := shape.idKey()
	text := shape.TC.Text
	imap := StateMap[string, inputState](
		ctx.Window, nsInput, capMany,
	)
	// Default InputState{}: zero value seeds initial keyboard-nav state.
	is := imap.GetOr(id, inputState{})
	savedOffset := is.cursorOffset
	savedTrailing := is.cursorTrailing
	is.cursorOffset = -1
	is.cursorTrailing = false
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	isShift := ctx.Event.Modifiers.Has(ModShift)
	isWordMod := ctx.Event.Modifiers.HasAny(
		ModCtrl, ModAlt, ModSuper,
	)
	handled := true

	gl, glOK := inputGlyphLayout(
		text, shape, textStyleOrDefault(shape), ctx.Window,
	)

	switch ctx.Event.KeyCode {
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
		if ctx.Event.Modifiers.HasAny(ModCtrl, ModSuper) {
			inputSelectAll(text, id, ctx.Window)
		} else {
			handled = false
		}
	case KeyC:
		handled = inputKeyCopy(
			text, id, shape.TC.textIsPassword, ctx.Event, ctx.Window)
	default:
		handled = false
	}

	if handled {
		resetBlinkCursorVisible(ctx.Window)
		textScrollCursorIntoView(ctx.Layout, ctx.Window)
		ctx.Consume()
	}
}

// textKeyVertical handles KeyUp/KeyDown for text selection.
// Returns false when the key is unhandled (single-line mode).
func textKeyVertical(
	imap *BoundedMap[string, inputState], id string, is inputState,
	text string, pos int, isShift bool,
	savedOffset float32, up bool, mode textMode,
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
		is.cursorOffset = px
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
func textAmendLayout(ctx EventCtx) {
	if ctx.Layout.Shape.ID == "" || !ctx.Layout.Shape.Focusable || ctx.Layout.Shape.TC == nil {
		return
	}
	is := StateReadOr(
		ctx.Window, nsInput, ctx.Layout.Shape.idKey(), inputState{},
	)
	ctx.Layout.Shape.TC.textSelBeg = is.selectBeg
	ctx.Layout.Shape.TC.textSelEnd = is.selectEnd
}
