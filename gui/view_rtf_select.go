package gui

import (
	"time"
)

// --- RTF standalone text selection ---

// rtfSelectAmendLayout copies InputState selection into the shape's
// TextSelBeg/TextSelEnd for rendering and calls rtfAmendTooltip.
func rtfSelectAmendLayout(ctx EventCtx) {
	rtfAmendTooltip(ctx)
	if ctx.Layout.Shape.ID == "" || !ctx.Layout.Shape.Focusable || ctx.Layout.Shape.TC == nil {
		return
	}
	is := StateReadOr(ctx.Window, nsInput, ctx.Layout.Shape.idKey(), inputState{})
	ctx.Layout.Shape.TC.textSelBeg = is.selectBeg
	ctx.Layout.Shape.TC.textSelEnd = is.selectEnd
}

// rtfMarkdownAmendLayout calls rtfAmendTooltip and the markdown block
// selection handler. The markdown block handler is defined in markdown_select.go.
func rtfMarkdownAmendLayout(ctx EventCtx) {
	rtfAmendTooltip(ctx)
	markdownBlockAmendSel(ctx.Layout, ctx.Window)
}

// rtfSelectOnClick handles clicks for an RTF widget with selection enabled.
// Link navigation (rtfOnClick) runs first; selection state is always updated.
func rtfSelectOnClick(ctx EventCtx) {
	rtfOnClick(ctx)
	if ctx.Event.MouseButton == MouseRight {
		return
	}
	shape := ctx.Layout.Shape
	if shape.TC == nil || !shape.hasRtfLayout() || shape.ID == "" || !shape.Focusable {
		return
	}
	ctx.Window.SetFocus(shape.idKey())

	gl := shape.TC.rTFLayout
	flatText := shape.TC.rTFFlatText

	// The glyph layout's char rects are in the same space as the click
	// coordinates: OnClick arrives through callRelative, which already
	// translated the event to shape-local coordinates (scroll offsets
	// included — the shape's post-scroll position is subtracted). The
	// drag path below handles its own translation because MouseLock
	// callbacks receive window coordinates.
	byteIdx := gl.GetClosestOffset(ctx.Event.MouseX, ctx.Event.MouseY)
	runePos := byteToRuneIndex(flatText, byteIdx)

	focusID := shape.idKey()
	imap := StateMap[string, inputState](ctx.Window, nsInput, capMany)
	// Default InputState{}: zero value seeds initial selection/cursor state.
	is := imap.GetOr(focusID, inputState{})

	now := time.Now().UnixMilli()
	doubleClick := is.LastClickTime > 0 &&
		now-is.LastClickTime <= doubleClickThresholdMs
	is.LastClickTime = now

	if doubleClick {
		bBeg, bEnd := gl.GetWordAtIndex(byteIdx)
		beg := byteToRuneIndex(flatText, bBeg)
		end := byteToRuneIndex(flatText, bEnd)
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
	ctx.Consume()

	anchorPos := is.selectBeg
	anchorEnd := is.selectEnd
	dragShapeX := shape.X
	dragShapeY := shape.Y

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
			// Default 0: unscrolled container before first scroll event.
			dragScrollY0 = sy.GetOr(scrollID, 0)
			sp := p.Shape
			viewTop = sp.Y + sp.Padding.Top
			viewH := sp.Height - sp.paddingHeight()
			viewBot = viewTop + viewH
			maxScrollNeg = f32Min(0, viewH-contentHeight(p))
			break
		}
	}

	computeRunePos := func(mx, my float32, w *Window) int {
		scrollDelta := float32(0)
		if scrollID != "" {
			sy := w.scrollY()
			// Default 0: unscrolled position when no offset recorded yet.
			sNow := sy.GetOr(scrollID, 0)
			scrollDelta = sNow - dragScrollY0
		}
		rx := mx - dragShapeX
		ry := my - (dragShapeY + scrollDelta)
		bi := gl.GetClosestOffset(rx, ry)
		return byteToRuneIndex(flatText, bi)
	}

	updateDrag := func(rp int, w *Window) {
		dim := StateMap[string, inputState](w, nsInput, capMany)
		// Default InputState{}: zero value seeds initial drag-edit state.
		dis := dim.GetOr(focusID, inputState{})
		if doubleClick {
			bi := runeToByteIndex(flatText, rp)
			bBeg, bEnd := gl.GetWordAtIndex(bi)
			wb := byteToRuneIndex(flatText, bBeg)
			we := byteToRuneIndex(flatText, bEnd)
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
		dim.Set(focusID, dis)
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
		newScroll := f32Clamp(cur+delta, maxScrollNeg, 0)
		if newScroll == cur {
			return
		}
		sy.Set(scrollID, newScroll)
		rp := computeRunePos(lastMouseX, lastMouseY, w)
		updateDrag(rp, w)
	}

	ctx.Window.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {
			lastMouseX = ctx.Event.MouseX
			lastMouseY = ctx.Event.MouseY
			rp := computeRunePos(ctx.Event.MouseX, ctx.Event.MouseY, ctx.Window)
			updateDrag(rp, ctx.Window)
			if scrollID != "" {
				outside := ctx.Event.MouseY < viewTop || ctx.Event.MouseY > viewBot
				if outside && !ctx.Window.HasAnimation(animIDTextDragScroll) {
					ctx.Window.AnimationAdd(&Animate{
						AnimID:   animIDTextDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: dragScrollCB,
					})
				} else if !outside {
					ctx.Window.AnimationRemove(animIDTextDragScroll)
				}
			}
		},
		MouseUp: func(ctx EventCtx) {
			ctx.Window.AnimationRemove(animIDTextDragScroll)
			ctx.Window.MouseUnlock()
		},
		Cancel: func(w *Window) {
			w.AnimationRemove(animIDTextDragScroll)
			// Zero the partial selection the drag was mutating
			// (nsInput keyed by this RTF's own ID, issue #281) —
			// capture loss must not leave a stuck highlight, while
			// a normal release still commits.
			dim := StateMap[string, inputState](w, nsInput, capMany)
			dis := dim.GetOr(focusID, inputState{})
			dis.selectBeg = 0
			dis.selectEnd = 0
			dim.Set(focusID, dis)
		},
	})
}

// rtfSelectOnKeyDown handles keyboard navigation and copy for selectable RTF.
func rtfSelectOnKeyDown(ctx EventCtx) {
	shape := ctx.Layout.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable ||
		!ctx.Window.IsFocus(shape.idKey()) {
		return
	}
	id := shape.idKey()
	flatText := shape.TC.rTFFlatText
	gl := *shape.TC.rTFLayout

	imap := StateMap[string, inputState](ctx.Window, nsInput, capMany)
	// Default InputState{}: zero value seeds initial keyboard-nav state.
	is := imap.GetOr(id, inputState{})
	savedOffset := is.cursorOffset
	savedTrailing := is.cursorTrailing
	is.cursorOffset = -1
	is.cursorTrailing = false
	runeLen := utf8RuneCount(flatText)
	pos := min(is.CursorPos, runeLen)
	isShift := ctx.Event.Modifiers.Has(ModShift)
	isWordMod := ctx.Event.Modifiers.HasAny(ModCtrl, ModAlt, ModSuper)
	handled := true

	switch ctx.Event.KeyCode {
	case KeyLeft:
		inputKeyLeft(imap, id, is, flatText, pos,
			isShift, isWordMod, gl, true)
	case KeyRight:
		inputKeyRight(imap, id, is, flatText, pos, runeLen,
			isShift, isWordMod, gl, true)
	case KeyHome:
		inputKeyHome(imap, id, is, flatText, pos,
			isShift, savedTrailing, gl, true)
	case KeyEnd:
		inputKeyEnd(imap, id, is, flatText, pos,
			isShift, savedTrailing, gl, true)
	case KeyUp:
		handled = textKeyVertical(imap, id, is, flatText,
			pos, isShift, savedOffset, true,
			shape.TC.TextMode, gl, true)
	case KeyDown:
		handled = textKeyVertical(imap, id, is, flatText,
			pos, isShift, savedOffset, false,
			shape.TC.TextMode, gl, true)
	case KeyEscape:
		inputKeyEscape(imap, id, is)
		handled = false
	case KeyA:
		if ctx.Event.Modifiers.HasAny(ModCtrl, ModSuper) {
			inputSelectAll(flatText, id, ctx.Window)
		} else {
			handled = false
		}
	case KeyC:
		handled = inputKeyCopy(flatText, id, false, ctx.Event, ctx.Window)
	default:
		handled = false
	}

	if handled {
		ctx.Consume()
	}
}
