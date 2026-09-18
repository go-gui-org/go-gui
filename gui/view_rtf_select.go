package gui

import (
	"github.com/go-gui-org/go-glyph"
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
// rtfDragFrame is the layout input for one step of an RTF selection
// drag: where the block sits, what it shaped to, and the scroll
// viewport around it. A drag holds one press-time frame as its
// fallback and resolves a fresh one per step.
//
// scrollAnchor is the scroll offset shY is already expressed in, and
// it travels with shY rather than being fixed at the press. A child
// of a scroll container has the offset baked into Shape.Y by arrange
// (layoutChildStartPos, gui/layout_position.go), so a re-read shY
// already carries every scroll up to the last arrange; measuring it
// against the press-time offset as well would count the scroll since
// the press twice and drag the selection away by that distance.
type rtfDragFrame struct {
	gl           *glyph.Layout
	flat         string
	shX          float32
	shY          float32
	top          float32
	bot          float32
	vTop         float32
	vBot         float32
	maxNeg       float32
	scrollAnchor float32
}

// resolve returns the frame for the tree as it stands: the live shape
// when it is still there, else the receiver — the press-time frame.
// Geometry that moves under the drag (shape position, viewport, text
// band) is re-read whenever the shape resolves, and otherwise keeps
// its press-time value; a zero viewport would read as "pointer
// outside" for every Y and drive the edge-scroll to clamp the
// container back to the top.
func (snap rtfDragFrame) resolve(
	w *Window, root *Layout, focusID, scrollID string,
) rtfDragFrame {
	f := snap
	node := root
	// findByID, not FindByID: a miss here is legitimate (the shape is
	// briefly out of the tree mid-drag), and the public form would
	// report it as a misspelling through the DebugUnknownLookup gate.
	if found, ok := root.findByID(focusID); ok &&
		found.Shape != nil && found.Shape.TC != nil &&
		found.Shape.TC.rTFLayout != nil {
		node = found
		f.gl = found.Shape.TC.rTFLayout
		f.flat = found.Shape.TC.rTFFlatText
		f.shX, f.shY = found.Shape.X, found.Shape.Y
		f.top, f.bot = glyphTextBand(f.gl)
		if scrollID != "" {
			// The tree this shY came from was arranged at the offset
			// standing now. A caller that then moves the scroll
			// before mapping a pointer — dragScrollCB — resolves the
			// frame first, so the difference it introduces is exactly
			// what computeRunePos corrects for.
			// Default 0: unscrolled before any scroll event.
			f.scrollAnchor = w.scrollY().GetOr(scrollID, 0)
		}
	}
	for p := node.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Scrollable {
			f.vTop, f.vBot, f.maxNeg = dragViewport(p)
			break
		}
	}
	return f
}

func rtfSelectOnClick(ctx EventCtx) {
	// A link click still collapses the selection under the pointer, the
	// way a browser drops the old highlight — but it must not arm the
	// drag below. See the linkHit guard before MouseLock.
	linkHit := rtfClickLink(ctx)
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

	now := doubleClickNowMs()
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

	// The click activated a link, so it is spent: navigation owns it.
	// Arming the drag here would lock the mouse for a release that
	// never reaches this widget — the link opened a browser, scrolled
	// the view away, or raised a context menu over the pointer — and
	// the pointer would then keep extending the selection with no
	// button held.
	if linkHit {
		return
	}

	anchorPos := is.selectBeg
	anchorEnd := is.selectEnd

	// Press-time drag frame. The MouseLock callbacks below run on
	// later frames, when the shape tree may have been rebuilt around
	// them (a resize re-wraps, a math fetch flips Loading→Ready and
	// with it the flat text). Each callback re-resolves the shape by
	// effective ID first and falls back to this copy only when the
	// shape is briefly gone. The glyph value copy is safe across
	// frames: shaped layouts are never mutated once stored (see
	// rtfLayoutEntry).
	snapGL := *gl
	snap := rtfDragFrame{
		gl: &snapGL, flat: flatText,
		shX: shape.X, shY: shape.Y,
	}
	snap.top, snap.bot = glyphTextBand(gl)

	var lastMouseX, lastMouseY float32
	scrollID := ""
	for p := ctx.Layout.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Scrollable {
			scrollID = p.Shape.idKey()
			// Default 0: unscrolled container before first scroll event.
			snap.scrollAnchor = ctx.Window.scrollY().GetOr(scrollID, 0)
			snap.vTop, snap.vBot, snap.maxNeg = dragViewport(p)
			break
		}
	}

	computeRunePos := func(
		mx, my float32, w *Window, f rtfDragFrame,
	) int {
		scrollDelta := float32(0)
		if scrollID != "" {
			sy := w.scrollY()
			// Default 0: unscrolled position when no offset recorded yet.
			sNow := sy.GetOr(scrollID, 0)
			scrollDelta = sNow - f.scrollAnchor
		}
		ry := my - (f.shY + scrollDelta)
		rx := textDragEdgeX(mx-f.shX, ry, f.top, f.bot)
		bi := f.gl.GetClosestOffset(rx, ry)
		return byteToRuneIndex(f.flat, bi)
	}

	updateDrag := func(rp int, w *Window, f rtfDragFrame) {
		dim := StateMap[string, inputState](w, nsInput, capMany)
		// Default InputState{}: zero value seeds initial drag-edit state.
		dis := dim.GetOr(focusID, inputState{})
		if doubleClick {
			bi := runeToByteIndex(f.flat, rp)
			bBeg, bEnd := f.gl.GetWordAtIndex(bi)
			wb := byteToRuneIndex(f.flat, bBeg)
			we := byteToRuneIndex(f.flat, bEnd)
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
		f := snap.resolve(w, &w.layout, focusID, scrollID)
		var delta float32
		if lastMouseY < f.vTop {
			delta = (f.vTop - lastMouseY) * textDragEdgeScrollFactor
		} else if lastMouseY > f.vBot {
			delta = -((lastMouseY - f.vBot) * textDragEdgeScrollFactor)
		} else {
			w.AnimationRemove(animIDTextDragScroll)
			return
		}
		sy := w.scrollY()
		// Default 0: unscrolled position when no offset recorded yet.
		cur := sy.GetOr(scrollID, 0)
		newScroll := f32Clamp(cur+delta, f.maxNeg, 0)
		if newScroll == cur {
			return
		}
		sy.Set(scrollID, newScroll)
		rp := computeRunePos(lastMouseX, lastMouseY, w, f)
		updateDrag(rp, w, f)
	}

	ctx.Window.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {
			lastMouseX = ctx.Event.MouseX
			lastMouseY = ctx.Event.MouseY
			f := snap.resolve(
				ctx.Window, ctx.Layout, focusID, scrollID)
			rp := computeRunePos(
				ctx.Event.MouseX, ctx.Event.MouseY, ctx.Window, f)
			updateDrag(rp, ctx.Window, f)
			if scrollID != "" {
				outside := ctx.Event.MouseY < f.vTop ||
					ctx.Event.MouseY > f.vBot
				if outside && !ctx.Window.HasAnimation(animIDTextDragScroll) {
					ctx.Window.AnimationAdd(&Animate{
						AnimID:   animIDTextDragScroll,
						Delay:    textDragScrollInterval,
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
		inputKeyRight(imap, id, is, flatText, pos,
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
