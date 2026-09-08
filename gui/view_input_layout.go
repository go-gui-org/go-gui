package gui

import (
	"strings"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// inputTextFromLayout extracts the current text from the input's
// inner layout structure (Column → Row → Text).
func inputTextFromLayout(layout *Layout) string {
	txt := inputTextShapeFromLayout(layout)
	if txt == nil {
		return ""
	}
	if txt.TC.textIsPlaceholder {
		return ""
	}
	return txt.TC.Text
}

// inputTextShapeFromLayout returns an input's inner text shape
// (Column -> Row/Column -> Text), or nil when the tree is not the shape
// this expects. Every caller that reaches the text leaf has to descend
// the same way and move together if the widget's structure changes;
// one helper is what keeps them together.
func inputTextShapeFromLayout(layout *Layout) *Shape {
	if layout == nil || len(layout.Children) == 0 {
		return nil
	}
	row := &layout.Children[0]
	if len(row.Children) == 0 {
		return nil
	}
	txt := row.Children[0].Shape
	if txt == nil || txt.TC == nil {
		return nil
	}
	return txt
}

// inputSetTextInLayout writes mutated text back into the input's
// inner text shape (Column → Row/Column → Text). Backends drain
// every pending event against the same layout tree before the next
// frame rebuild, so without this write-back the second key event in
// a batch reads the pre-mutation text via inputTextFromLayout and
// silently discards the earlier keystroke (dropped characters when
// typing fast; backspaces that do not stick). The tree is rebuilt
// from app state before the next render, so this echo only feeds
// intra-batch event reads — it never reaches the screen.
func inputSetTextInLayout(layout *Layout, text string) {
	txt := inputTextShapeFromLayout(layout)
	if txt == nil {
		return
	}
	txt.TC.Text = text
	// Clear the placeholder flag so a follow-up read returns the
	// typed text rather than treating it as placeholder content.
	txt.TC.textIsPlaceholder = false
}

// inputGlyphLayoutFor navigates to the inner text shape of an
// input layout and returns a glyph Layout for cursor navigation.
func inputGlyphLayoutFor(layout *Layout, w *Window) (glyph.Layout, bool) {
	return inputGlyphLayoutWithText(
		inputTextFromLayout(layout), layout, w,
	)
}

// inputGlyphLayoutWithText returns a glyph Layout using
// pre-extracted text, avoiding redundant layout traversal.
func inputGlyphLayoutWithText(
	text string, layout *Layout, w *Window,
) (glyph.Layout, bool) {
	if w.textMeasurer == nil {
		return glyph.Layout{}, false
	}
	txt := inputTextShapeFromLayout(layout)
	if txt == nil {
		return glyph.Layout{}, false
	}
	style := textStyleOrDefault(txt)
	return inputGlyphLayout(text, txt, style, w)
}

// trailingLineStart returns the start of the previous visual line
// when byteIdx is at a soft-wrap boundary and CursorTrailing is set.
// Falls back to fallback if no boundary match is found.
func trailingLineStart(lines []glyph.Line, byteIdx, fallback int) int {
	for i, line := range lines {
		if i > 0 && byteIdx == line.StartIndex {
			return lines[i-1].StartIndex
		}
	}
	return fallback
}

// trailingLineEnd returns the end of the previous visual line
// when byteIdx is at a soft-wrap boundary and CursorTrailing is set.
// Falls back to fallback if no boundary match is found.
func trailingLineEnd(lines []glyph.Line, byteIdx, fallback int) int {
	for i, line := range lines {
		if i > 0 && byteIdx == line.StartIndex {
			return lines[i-1].StartIndex + lines[i-1].Length
		}
	}
	return fallback
}

// inputDeleteGrapheme deletes a grapheme cluster at cursor using
// glyph when available, falling back to rune-based inputDelete.
func inputDeleteGrapheme(
	text string, focusID string, forward bool,
	layout *Layout, w *Window,
) (string, bool) {
	gl, glOK := inputGlyphLayoutFor(layout, w)
	if !glOK {
		newText, _ := inputDelete(text, focusID, forward, w)
		return newText, newText != text
	}
	is := inputStateOrDefault(focusID, w)
	if is.selectBeg != is.selectEnd {
		newText, _ := inputDelete(text, focusID, forward, w)
		return newText, newText != text
	}
	pos := min(is.CursorPos, utf8RuneCount(text))
	byteIdx := runeToByteIndex(text, pos)
	var res glyph.MutationResult
	if forward {
		res = glyph.DeleteForward(text, gl, byteIdx)
	} else {
		res = glyph.DeleteBackward(text, gl, byteIdx)
	}
	if res.NewText == text {
		return text, false
	}
	newPos := byteToRuneIndex(res.NewText, res.CursorPos)
	undo := inputPushUndo(is, text, inputOpDelete)
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set(focusID, inputState{
		CursorPos:    newPos,
		cursorOffset: -1,
		Undo:         undo,
		lastEditOp:   inputOpDelete,
	})
	return res.NewText, true
}

// passwordMask replaces each rune with a bullet character.
// Uses a stack-local buffer for short strings to avoid heap
// allocation from strings.Repeat.
func passwordMask(text string) string {
	n := utf8RuneCount(text)
	// "•" (U+2022) = 3 bytes UTF-8: 0xE2 0x80 0xA2
	const bLen = 3
	if n <= 64 {
		var buf [64 * bLen]byte
		for i := range n {
			buf[i*bLen] = 0xe2
			buf[i*bLen+1] = 0x80
			buf[i*bLen+2] = 0xa2
		}
		return string(buf[:n*bLen])
	}
	return strings.Repeat("•", n)
}

// inputDragState holds state for drag-to-select in an input.
// Replaces ~10 closure-captured locals with explicit fields;
// wordSelect is set iff the drag started from a double-click.
type inputDragState struct {
	displayText            string
	wordSelect             bool // drag extends by whole words
	gl                     glyph.Layout
	anchorPos, anchorEnd   uint32
	txtOffX, txtOffY       float32
	focusID                string
	scrollID               string
	lastMouseX, lastMouseY float32
	scrollY0               float32
	viewTop, viewBot       float32
	maxScrollNeg           float32
	// Horizontal mirror of the four fields above. The offset can move
	// mid-drag on either axis, so both are captured at drag start and
	// re-read as a delta in computeRunePos.
	scrollX0            float32
	viewLeft, viewRight float32
	maxScrollNegX       float32
}

func (d *inputDragState) computeRunePos(
	mx, my float32, w *Window,
) int {
	scrollDelta, scrollDeltaX := float32(0), float32(0)
	if d.scrollID != "" {
		sy := w.scrollY()
		// Default 0: absent entry means unscrolled initial position.
		sNow := sy.GetOr(d.scrollID, 0)
		scrollDelta = sNow - d.scrollY0
		sNowX := w.scrollX().GetOr(d.scrollID, 0)
		scrollDeltaX = sNowX - d.scrollX0
	}
	relY := my - (d.txtOffY + scrollDelta)
	// A drag carried above or below the text selects to the line's
	// edge rather than stopping at the pointer's column; see
	// textDragEdgeX.
	top, bot := glyphTextBand(&d.gl)
	relX := textDragEdgeX(mx-(d.txtOffX+scrollDeltaX), relY, top, bot)
	byteIdx := d.gl.GetClosestOffset(relX, relY)
	return byteToRuneIndex(d.displayText, byteIdx)
}

func (d *inputDragState) updateSelection(rp int, w *Window) {
	imap := StateMap[string, inputState](w, nsInput, capMany)
	// Default InputState{}: zero value seeds initial drag-selection state.
	is := imap.GetOr(d.focusID, inputState{})
	if d.wordSelect {
		wb, we := wordBoundsAt(d.displayText, rp)
		if rp < int(d.anchorPos) {
			is.selectBeg = d.anchorEnd
			is.selectEnd = uint32(wb)
			is.CursorPos = wb
		} else {
			is.selectBeg = d.anchorPos
			is.selectEnd = uint32(we)
			is.CursorPos = we
		}
	} else {
		is.CursorPos = rp
		is.selectBeg = d.anchorPos
		is.selectEnd = uint32(rp)
	}
	is.cursorOffset = -1
	// Drag selection is caret motion: break any undo run so a
	// subsequent edit starts a fresh undo step (issue #328).
	is.lastEditOp = inputOpNone
	imap.Set(d.focusID, is)
	resetBlinkCursorVisible(w)
}

func (d *inputDragState) scrollCallback(
	_ *Animate, w *Window,
) {
	// Both axes are computed before anything is decided. Returning as
	// soon as one axis is inside its band would stop the auto-scroll
	// while the pointer is still dragging past the other edge.
	delta := dragScrollDelta(d.lastMouseY, d.viewTop, d.viewBot)
	deltaX := float32(0)
	if d.viewRight > d.viewLeft {
		// An unseeded X band (both zero) would read as "outside" for
		// every pointer position; only a real band takes part.
		deltaX = dragScrollDelta(d.lastMouseX, d.viewLeft, d.viewRight)
	}
	if delta == 0 && deltaX == 0 {
		w.AnimationRemove(animIDDragScroll)
		return
	}
	// Both calls run: || would skip the second axis once the first has
	// moved, and the drag has to advance on each independently.
	movedY := applyDragScroll(
		w.scrollY(), d.scrollID, delta, d.maxScrollNeg)
	movedX := applyDragScroll(
		w.scrollX(), d.scrollID, deltaX, d.maxScrollNegX)
	if !movedY && !movedX {
		return
	}
	rp := d.computeRunePos(d.lastMouseX, d.lastMouseY, w)
	d.updateSelection(rp, w)
}

// applyDragScroll advances one axis of a drag auto-scroll by delta and
// reports whether the stored offset actually moved. It does not move
// when the axis is already scrolled as far as it goes, which is what
// stops a drag held past the end from re-selecting every frame.
func applyDragScroll(
	m *BoundedMap[string, float32], id string, delta, maxNeg float32,
) bool {
	if delta == 0 || !f32IsFinite(delta) {
		return false
	}
	if !f32IsFinite(maxNeg) || maxNeg > 0 {
		maxNeg = 0
	}
	// Default 0: unscrolled position when no offset recorded yet.
	cur := m.GetOr(id, 0)
	next := f32Clamp(cur+delta, maxNeg, 0)
	if next == cur {
		return false
	}
	m.Set(id, next)
	return true
}

// dragScrollDelta returns the auto-scroll step for a pointer dragged
// past one edge of the viewport band, or 0 while it is inside. The
// step is proportional to the overshoot, so the further out the drag
// is carried the faster the field scrolls.
func dragScrollDelta(pos, lo, hi float32) float32 {
	switch {
	case pos < lo:
		return (lo - pos) * 0.3
	case pos > hi:
		return -((pos - hi) * 0.3)
	}
	return 0
}

// startInputDrag sets up MouseLock drag-to-select for an input.
func startInputDrag(d *inputDragState, w *Window) {
	w.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {
			// No originating event: no pointer position, so decline.
			if ctx.Event == nil {
				return
			}
			d.lastMouseX = ctx.Event.MouseX
			d.lastMouseY = ctx.Event.MouseY
			rp := d.computeRunePos(ctx.Event.MouseX, ctx.Event.MouseY, ctx.Window)
			d.updateSelection(rp, ctx.Window)
			if d.scrollID != "" {
				outside := ctx.Event.MouseY < d.viewTop ||
					ctx.Event.MouseY > d.viewBot ||
					(d.viewRight > d.viewLeft &&
						(ctx.Event.MouseX < d.viewLeft ||
							ctx.Event.MouseX > d.viewRight))
				if outside && !ctx.Window.HasAnimation(
					animIDDragScroll) {
					ctx.Window.AnimationAdd(&Animate{
						AnimID:   animIDDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: d.scrollCallback,
					})
				} else if !outside {
					ctx.Window.AnimationRemove(animIDDragScroll)
				}
			}
		},
		MouseUp: func(ctx EventCtx) {
			ctx.Window.AnimationRemove(animIDDragScroll)
			ctx.Window.MouseUnlock()
		},
		Cancel: func(w *Window) {
			w.AnimationRemove(animIDDragScroll)
			// The drag was mutating this input's own nsInput state:
			// zero the partial selection so a cancelled drag never
			// leaves a stuck highlight (issue #281). Other widgets'
			// selections are untouched — the key is d.focusID.
			imap := StateMap[string, inputState](w, nsInput, capMany)
			is := imap.GetOr(d.focusID, inputState{})
			is.selectBeg = 0
			is.selectEnd = 0
			imap.Set(d.focusID, is)
		},
	})
}

// inputApplyScrollX shifts an input's text shape by its horizontal
// scroll offset, which is what makes the field follow the caret.
// A Scrollable field is skipped: it is a real scroll container, so
// layoutPositions has already folded the offset into its children.
//
// The offset lives in w.scrollX() under the field's own identity even
// though such a field is deliberately NOT Scrollable — marking a
// single-line field scrollable would trip two sizing rules that treat
// any Scrollable Fill container as elastic (layout_sizing.go,
// layoutFillCrossAxis and the MinHeight floor), resizing Inputs in a
// Row and collapsing data grid cells. Sharing the storage instead keeps
// one follow function for every input.
//
// Only the text leaf moves. The inner row keeps its position, so the
// click target still covers the whole field, and the caret, hit test
// and drag all read the text shape's own X, so they move with it.
// Running from AmendLayout puts this before layoutSetShapeClips and
// before event dispatch: the shifted ink is still clipped to the field
// and hit testing sees the shifted rect.
func inputApplyScrollX(
	hcfg inputHandlerCfg, layout *Layout, w *Window,
) {
	if layout == nil || layout.Shape == nil {
		return
	}
	if layout.Shape.Scrollable || hcfg.scrollID == "" {
		return
	}
	key := layout.Shape.idKey()
	if key == "" {
		return
	}
	txt := inputTextShapeFromLayout(layout)
	if txt == nil || txt.TC.textIsPlaceholder {
		// A placeholder is never scrolled: it is not the user's text
		// and the caret sits at its start.
		return
	}
	// Default 0: no entry means the field has never scrolled.
	offset := w.scrollX().GetOr(key, 0)
	if offset == 0 {
		return
	}
	if !f32IsFinite(offset) {
		w.scrollX().Set(key, 0)
		return
	}
	// Re-clamp against this frame's geometry. The offset was written
	// against the previous frame, so a field that grew or text that
	// shrank can leave it stale, and nothing else clamps it: the field
	// is not Scrollable, so layoutAdjustScrollOffsets never sees it.
	viewW := layout.Shape.Width - layout.Shape.paddingWidth()
	if viewW <= 0 || !f32IsFinite(viewW) {
		// Degenerate geometry: a field sized to nothing this frame (a
		// collapsed pane, a hidden tab). Clamping against it would be
		// arithmetic on a negative viewport, so leave the stored offset
		// alone and paint unshifted until the field has a real width.
		// Matches the same guard at the write site.
		return
	}
	// Same extent the write site clamps against.
	clamped := f32Clamp(offset,
		f32Min(0, viewW-inputScrollContentW(layout, txt)), 0)
	if clamped != offset {
		// Store it back, the way layoutAdjustScrollOffsets does for a
		// real scroll container. Painting the clamped value while the
		// map keeps the stale one would leave every reader of the
		// offset (ScrollHorizontalPct, the next follow, a drag seed)
		// disagreeing with what is on screen.
		w.scrollX().Set(key, clamped)
	}
	txt.X += clamped
}
