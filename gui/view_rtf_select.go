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
	is := StateReadOr(ctx.Window, nsInput, ctx.Layout.Shape.ID, InputState{})
	ctx.Layout.Shape.TC.TextSelBeg = is.SelectBeg
	ctx.Layout.Shape.TC.TextSelEnd = is.SelectEnd
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
	ctx.Window.SetFocus(shape.ID)

	gl := shape.TC.RTFLayout
	flatText := shape.TC.RTFFlatText

	byteIdx := gl.GetClosestOffset(ctx.Event.MouseX, ctx.Event.MouseY)
	runePos := byteToRuneIndex(flatText, byteIdx)

	focusID := shape.ID
	imap := StateMap[string, InputState](ctx.Window, nsInput, capMany)
	// Default InputState{}: zero value seeds initial selection/cursor state.
	is := imap.GetOr(focusID, InputState{})

	now := time.Now().UnixMilli()
	doubleClick := is.LastClickTime > 0 &&
		now-is.LastClickTime <= doubleClickThresholdMs
	is.LastClickTime = now

	if doubleClick {
		bBeg, bEnd := gl.GetWordAtIndex(byteIdx)
		beg := byteToRuneIndex(flatText, bBeg)
		end := byteToRuneIndex(flatText, bEnd)
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
	ctx.Consume()

	anchorPos := is.SelectBeg
	anchorEnd := is.SelectEnd
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
			scrollID = p.Shape.ID
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
		dim := StateMap[string, InputState](w, nsInput, capMany)
		// Default InputState{}: zero value seeds initial drag-edit state.
		dis := dim.GetOr(focusID, InputState{})
		if doubleClick {
			bi := runeToByteIndex(flatText, rp)
			bBeg, bEnd := gl.GetWordAtIndex(bi)
			wb := byteToRuneIndex(flatText, bBeg)
			we := byteToRuneIndex(flatText, bEnd)
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
	})
}

// rtfSelectOnKeyDown handles keyboard navigation and copy for selectable RTF.
func rtfSelectOnKeyDown(ctx EventCtx) {
	shape := ctx.Layout.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable ||
		!ctx.Window.IsFocus(shape.ID) {
		return
	}
	id := shape.ID
	flatText := shape.TC.RTFFlatText
	gl := *shape.TC.RTFLayout

	imap := StateMap[string, InputState](ctx.Window, nsInput, capMany)
	// Default InputState{}: zero value seeds initial keyboard-nav state.
	is := imap.GetOr(id, InputState{})
	savedOffset := is.CursorOffset
	savedTrailing := is.CursorTrailing
	is.CursorOffset = -1
	is.CursorTrailing = false
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
