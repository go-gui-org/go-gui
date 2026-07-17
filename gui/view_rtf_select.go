package gui

import (
	"time"
)

// --- RTF standalone text selection ---

// rtfSelectAmendLayout copies InputState selection into the shape's
// TextSelBeg/TextSelEnd for rendering and calls rtfAmendTooltip.
func rtfSelectAmendLayout(l *Layout, w *Window) {
	rtfAmendTooltip(l, w)
	if l.Shape.ID == "" || !l.Shape.Focusable || l.Shape.TC == nil {
		return
	}
	is := StateReadOr(w, nsInput, l.Shape.ID, InputState{})
	l.Shape.TC.TextSelBeg = is.SelectBeg
	l.Shape.TC.TextSelEnd = is.SelectEnd
}

// rtfMarkdownAmendLayout calls rtfAmendTooltip and the markdown block
// selection handler. The markdown block handler is defined in markdown_select.go.
func rtfMarkdownAmendLayout(l *Layout, w *Window) {
	rtfAmendTooltip(l, w)
	markdownBlockAmendSel(l, w)
}

// rtfSelectOnClick handles clicks for an RTF widget with selection enabled.
// Link navigation (rtfOnClick) runs first; selection state is always updated.
func rtfSelectOnClick(l *Layout, e *Event, w *Window) {
	rtfOnClick(l, e, w)
	if e.MouseButton == MouseRight {
		return
	}
	shape := l.Shape
	if shape.TC == nil || !shape.hasRtfLayout() || shape.ID == "" || !shape.Focusable {
		return
	}
	w.SetFocus(shape.ID)

	gl := shape.TC.RtfLayout
	flatText := shape.TC.RtfFlatText

	byteIdx := gl.GetClosestOffset(e.MouseX, e.MouseY)
	runePos := byteToRuneIndex(flatText, byteIdx)

	focusID := shape.ID
	imap := StateMap[string, InputState](w, nsInput, capMany)
	is, _ := imap.Get(focusID)

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
	e.IsHandled = true

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
	for p := l.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Scrollable {
			scrollID = p.Shape.ID
			sy := w.scrollY()
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
			sNow, _ := sy.Get(scrollID)
			scrollDelta = sNow - dragScrollY0
		}
		rx := mx - dragShapeX
		ry := my - (dragShapeY + scrollDelta)
		bi := gl.GetClosestOffset(rx, ry)
		return byteToRuneIndex(flatText, bi)
	}

	updateDrag := func(rp int, w *Window) {
		dim := StateMap[string, InputState](w, nsInput, capMany)
		dis, _ := dim.Get(focusID)
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
		cur, _ := sy.Get(scrollID)
		newScroll := f32Clamp(cur+delta, maxScrollNeg, 0)
		if newScroll == cur {
			return
		}
		sy.Set(scrollID, newScroll)
		rp := computeRunePos(lastMouseX, lastMouseY, w)
		updateDrag(rp, w)
	}

	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			lastMouseX = e.MouseX
			lastMouseY = e.MouseY
			rp := computeRunePos(e.MouseX, e.MouseY, w)
			updateDrag(rp, w)
			if scrollID != "" {
				outside := e.MouseY < viewTop || e.MouseY > viewBot
				if outside && !w.HasAnimation(animIDTextDragScroll) {
					w.AnimationAdd(&Animate{
						AnimID:   animIDTextDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: dragScrollCB,
					})
				} else if !outside {
					w.AnimationRemove(animIDTextDragScroll)
				}
			}
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.AnimationRemove(animIDTextDragScroll)
			w.MouseUnlock()
		},
	})
}

// rtfSelectOnKeyDown handles keyboard navigation and copy for selectable RTF.
func rtfSelectOnKeyDown(l *Layout, e *Event, w *Window) {
	shape := l.Shape
	if shape.TC == nil || shape.ID == "" || !shape.Focusable ||
		!w.IsFocus(shape.ID) {
		return
	}
	id := shape.ID
	flatText := shape.TC.RtfFlatText
	gl := *shape.TC.RtfLayout

	imap := StateMap[string, InputState](w, nsInput, capMany)
	is, _ := imap.Get(id)
	savedOffset := is.CursorOffset
	savedTrailing := is.CursorTrailing
	is.CursorOffset = -1
	is.CursorTrailing = false
	runeLen := utf8RuneCount(flatText)
	pos := min(is.CursorPos, runeLen)
	isShift := e.Modifiers.Has(ModShift)
	isWordMod := e.Modifiers.HasAny(ModCtrl, ModAlt, ModSuper)
	handled := true

	switch e.KeyCode {
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
		if e.Modifiers.HasAny(ModCtrl, ModSuper) {
			inputSelectAll(flatText, id, w)
		} else {
			handled = false
		}
	case KeyC:
		handled = inputKeyCopy(flatText, id, false, e, w)
	default:
		handled = false
	}

	if handled {
		e.IsHandled = true
	}
}
