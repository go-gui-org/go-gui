package gui

// inputOnClick handles a click on the field: the container that claims
// the ID, holding the inner row that wraps the text shape. It sits on
// the field rather than the row so the whole viewport takes the press
// (issue #1186). focusID is the field's ID — the key for focus and
// for the nsInput state — and canFocus is false for a FocusDisabled
// input, which still tracks a cursor but never takes focus. Both are
// passed in rather than read off the text shape, which carries no ID
// of its own (see Shape.focusOwner). hcfg is the field's handler
// config, which the context menu acts through; nil turns the menu off.
func inputOnClick(
	leafID, leafScrollID string, canFocus bool, hcfg *inputHandlerCfg,
) func(EventCtx) {
	return func(ctx EventCtx) {
		// Cursor placement needs the click coordinates and the inner
		// tree; without either there is nothing to place.
		if ctx.Event == nil || ctx.Layout == nil {
			return
		}
		// A screen reader's activate action on a text field means "edit
		// this". It has no pointer position, so it must not run the press
		// path below: that would move the caret to the field's corner,
		// drop the selection and take a drag lock no button release ends.
		if ctx.Event.a11yPress {
			if focusID := ctx.EffID(leafID); canFocus && focusID != "" {
				ctx.Window.SetFocus(focusID)
				ctx.Consume()
			}
			return
		}
		// A press on the field's own scrollbar is the scrollbar's. Its
		// parts take only the left button, so a right press would fall
		// back to the field and open the edit menu over a scrollbar;
		// decline it so it travels on, as it did before the handler
		// moved to the field.
		if dragPanOnScrollbar(ctx.Layout,
			ctx.Layout.Shape.X+ctx.Event.MouseX,
			ctx.Layout.Shape.Y+ctx.Event.MouseY) {
			return
		}
		// Only the left button places the caret. The field takes the
		// other buttons only when the context menu is on (OnAnyClick in
		// Input); a right press then opens it, and the rest travel on.
		switch ctx.Event.MouseButton {
		case MouseLeft:
		case MouseRight:
			if canFocus && hcfg != nil {
				inputMenuOnRightClick(ctx, leafID, leafScrollID, hcfg)
			}
			return
		default:
			return
		}
		field := ctx.Layout
		if len(field.Children) < 1 || len(field.Children[0].Children) < 1 {
			// No inner text shape: not ours
			return
		}
		// Input builds its tree eagerly, with no Window in hand, so the
		// captured IDs are leaves. Resolve them here: the owning
		// container is this shape, and by dispatch time every shape
		// carries its effective identity.
		focusID := ctx.EffID(leafID)
		scrollID := ctx.EffID(leafScrollID)
		inner := &field.Children[0]
		ly := inner.Children[0]
		if canFocus && focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		if ly.Shape.TC == nil {
			// No text config: not ours
			return
		}
		if ly.Shape.TC.textIsPlaceholder {
			imap := StateMap[string, inputState](
				ctx.Window, nsInput, capMany,
			)
			// Default InputState{}: zero value seeds initial state.
			is := imap.GetOr(focusID, inputState{})
			is.CursorPos = 0
			is.selectBeg = 0
			is.selectEnd = 0
			is.cursorOffset = -1
			imap.Set(focusID, is)
			resetBlinkCursorVisible(ctx.Window)
			ctx.Consume()
			return
		}
		text := ly.Shape.TC.Text
		style := textStyleOrDefault(ly.Shape)
		gl, ok := inputGlyphLayout(
			text, ly.Shape, style, ctx.Window,
		)
		if !ok {
			// No glyph layout: cannot place a cursor
			return
		}
		// MouseX/Y are local to the field. A press outside the text
		// (padding, below the last line) resolves to the closest offset.
		relX := ctx.Event.MouseX - (ly.Shape.X - field.Shape.X)
		relY := ctx.Event.MouseY - (ly.Shape.Y - field.Shape.Y)
		byteIdx := gl.GetClosestOffset(relX, relY)
		displayText := text
		if ly.Shape.TC.textIsPassword {
			displayText = maskPassword(text)
		}
		runePos := byteToRuneIndex(displayText, byteIdx)
		imap := StateMap[string, inputState](
			ctx.Window, nsInput, capMany,
		)
		// Default InputState{}: zero LastClickTime safely gates
		// double-click (the > 0 check below prevents false match).
		is := imap.GetOr(focusID, inputState{})

		// Double-click selects word.
		now := doubleClickNowMs()
		doubleClick := is.LastClickTime > 0 &&
			now-is.LastClickTime <= doubleClickThresholdMs
		is.LastClickTime = now

		if doubleClick {
			beg, end := wordBoundsAt(displayText, runePos)
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
		if scrollID != "" {
			inputScrollCursorIntoView(
				scrollID, text, field, ctx.Window,
			)
		}
		ctx.Consume()

		// Drag-to-select via MouseLock. The lock delivers mouse moves in
		// window-absolute coordinates (event_handlers.go), so the drag
		// offset must be the text shape's absolute position — subtracting
		// the box-relative offset (txt.X - box.X) would leave the drag
		// shifted by the input's window X. The click path derives the
		// same text-relative frame from the box-relative event.
		ds := &inputDragState{
			anchorPos:   is.selectBeg,
			anchorEnd:   is.selectEnd,
			gl:          gl,
			displayText: displayText,
			wordSelect:  doubleClick,
			txtOffX:     ly.Shape.X,
			txtOffY:     ly.Shape.Y,
			focusID:     focusID,
			scrollID:    scrollID,
		}
		if scrollID != "" {
			sy := ctx.Window.scrollY()
			// Default 0: unscrolled input before first scroll event.
			ds.scrollY0 = sy.GetOr(scrollID, 0)
			p := field.Shape
			viewLeft, viewTop, viewW, viewH := inputDragViewportEdges(p)
			ds.viewTop = viewTop
			ds.viewBot = ds.viewTop + viewH
			ds.maxScrollNeg = f32Min(0,
				viewH-inner.Shape.Height)

			// Same frame on X, clamped against the same extent the
			// follow writes against (see inputScrollContentW).
			sx := ctx.Window.scrollX()
			ds.scrollX0 = sx.GetOr(scrollID, 0)
			ds.viewLeft = viewLeft
			ds.viewRight = ds.viewLeft + viewW
			ds.maxScrollNegX = f32Min(0, viewW-inputScrollContentW(
				field, ly.Shape))
		}
		startInputDrag(ds, ctx.Window)
	}
}
