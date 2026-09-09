package gui

import "github.com/go-gui-org/go-glyph"

// findScrollLayout returns the layout for the scroll id, or false
// if the layout tree is not yet built or the id is not found.
func findScrollLayout(w *Window, effectiveID string) (*Layout, bool) {
	if w.layout.Shape == nil {
		return nil, false
	}
	return findLayoutByScrollID(&w.layout, effectiveID)
}

// fireOnScroll fires the OnScroll callback if set.
func fireOnScroll(ly *Layout, w *Window) {
	if ly.Shape.hasEvents() && ly.Shape.events.onScroll != nil {
		ly.Shape.events.onScroll(EventCtx{ly, nil, w})
	}
}

// adjustCursorTrailing adjusts cursor position to the end of
// the previous line when CursorTrailing is set and the byte
// index matches the start of a later line.
func adjustCursorTrailing(
	cp *glyph.CursorPosition, lines []glyph.Line,
	byteIdx int, trailing bool,
) {
	if !trailing {
		return
	}
	for i, line := range lines {
		if i > 0 && byteIdx == line.StartIndex {
			prev := lines[i-1]
			cp.X = prev.Rect.X + prev.Rect.Width
			cp.Y = prev.Rect.Y
			cp.Height = prev.Rect.Height
			return
		}
	}
}

// inputCaretW is the painted width of the text caret. Shared with
// renderInputCursor so the horizontal follow below scrolls the whole
// caret into view, not just the edge the glyph layout reports.
const inputCaretW = float32(1.5)

// inputScrollCursorIntoView adjusts an input's scroll so the cursor
// stays visible: vertically for a multiline input, and horizontally in
// both modes.
// layout must be the outer field container (the Column holding cfg.ID).
func inputScrollCursorIntoView(
	id string, text string, layout *Layout, w *Window,
) {
	if id == "" || w.textMeasurer == nil {
		return
	}
	txtShape := inputTextShapeFromLayout(layout)
	if txtShape == nil {
		return
	}
	style := textStyleOrDefault(txtShape)
	gl, ok := inputGlyphLayout(text, txtShape, style, w)
	if !ok {
		return
	}

	is := StateReadOr(w, nsInput,
		layout.Shape.idKey(), inputState{})
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	// The glyph layout above was built from the masked text for a
	// password field (inputGlyphLayoutResolved masks internally), so the
	// byte index has to be taken against that same string. A bullet and
	// the rune it hides are different byte lengths, so indexing the raw
	// text lands the caret in the wrong place.
	idxText := text
	if txtShape.TC.textIsPassword {
		idxText = maskPassword(text)
		pos = min(pos, utf8RuneCount(idxText))
	}
	byteIdx := runeToByteIndex(idxText, pos)

	cp, ok := gl.GetCursorPos(byteIdx)
	if !ok {
		return
	}
	adjustCursorTrailing(&cp, gl.Lines, byteIdx, is.cursorTrailing)

	inputScrollCursorIntoViewX(id, layout, txtShape, cp, w)

	// Default 0: unscrolled position when no offset recorded yet.
	sy := w.scrollY()
	scrollOffset := sy.GetOr(id, 0)
	viewportH := layout.Shape.Height - layout.Shape.paddingHeight()

	cursorTop := cp.Y
	cursorBot := cp.Y + cp.Height
	visibleTop := -scrollOffset
	visibleBot := visibleTop + viewportH

	if cursorTop < visibleTop {
		sy.Set(id, -cursorTop)
		scrollSmoothCancel(w, id, scrollAxisY)
	} else if cursorBot > visibleBot {
		sy.Set(id, -(cursorBot - viewportH))
		scrollSmoothCancel(w, id, scrollAxisY)
	}
}

// inputScrollCursorIntoViewX is the horizontal half of the follow. It
// runs for both modes: single-line text never wraps, and a multiline
// run with no break opportunity overflows the wrap width, so either can
// put the caret outside the field.
//
// Unlike the vertical branch, the offset is clamped here at the write.
// A single-line field is deliberately not Scrollable (see
// inputAmendLayout), so layoutAdjustScrollOffsets never sees it and
// would not clamp a stale offset for it. For a multiline field this is
// the same clamp the pipeline applies, so doing it twice is harmless.
func inputScrollCursorIntoViewX(
	id string, layout *Layout, txtShape *Shape,
	cp glyph.CursorPosition, w *Window,
) {
	if layout == nil || layout.Shape == nil || txtShape == nil {
		return
	}
	viewportW := layout.Shape.Width - layout.Shape.paddingWidth()
	if viewportW <= 0 || !f32IsFinite(viewportW) {
		return
	}
	if !f32IsFinite(cp.X) {
		return
	}
	// The caret is painted ink, so the extent the offset is clamped
	// against has to include it. That is not pedantry: this runs from a
	// key handler, so layout still holds the arrangement of the frame
	// before the keystroke and inputScrollContentW is one character
	// short, while cp comes from a glyph layout built on the new text.
	// Clamping to the stale extent alone parks the caret exactly one
	// character past the right edge and keeps it there.
	maxNegX := f32Min(0, viewportW-f32Max(
		inputScrollContentW(layout, txtShape), cp.X+inputCaretW))

	// Default 0: unscrolled position when no offset recorded yet.
	sx := w.scrollX()
	offset := sx.GetOr(id, 0)
	cursorLeft := cp.X
	cursorRight := cp.X + inputCaretW
	visibleLeft := -offset
	visibleRight := visibleLeft + viewportW

	switch {
	case cursorLeft < visibleLeft:
		sx.Set(id, f32Clamp(-cursorLeft, maxNegX, 0))
		scrollSmoothCancel(w, id, scrollAxisX)
	case cursorRight > visibleRight:
		sx.Set(id, f32Clamp(-(cursorRight-viewportW), maxNegX, 0))
		scrollSmoothCancel(w, id, scrollAxisX)
	}
}

// textScrollCursorIntoView adjusts the vertical scroll of the
// nearest scroll ancestor so the text cursor stays visible.
// Used by the read-only text widget's keyboard handler.
func textScrollCursorIntoView(layout *Layout, w *Window) {
	shape := layout.Shape
	if shape == nil || shape.TC == nil ||
		!shape.Focusable || shape.ID == "" || w.textMeasurer == nil {
		return
	}

	// Find nearest scroll ancestor.
	var scrollParent *Layout
	for p := layout.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Scrollable {
			scrollParent = p
			break
		}
	}
	if scrollParent == nil {
		return
	}
	scrollID := scrollParent.Shape.idKey()

	text := shape.TC.Text
	style := textStyleOrDefault(shape)
	gl, ok := inputGlyphLayout(text, shape, style, w)
	if !ok {
		return
	}

	is := StateReadOr(
		w, nsInput, shape.idKey(), inputState{})
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	byteIdx := runeToByteIndex(text, pos)

	cp, ok := gl.GetCursorPos(byteIdx)
	if !ok {
		return
	}
	adjustCursorTrailing(&cp, gl.Lines, byteIdx, is.cursorTrailing)

	// Default 0: unscrolled position when no offset recorded yet.
	sy := w.scrollY()
	scrollOffset := sy.GetOr(scrollID, 0)
	sp := scrollParent.Shape
	viewportH := sp.Height - sp.paddingHeight()
	viewTop := sp.Y + sp.Padding.Top
	viewBot := viewTop + viewportH

	cursorAbsTop := shape.Y + cp.Y
	cursorAbsBot := cursorAbsTop + cp.Height

	maxScrollNeg := f32Min(0,
		viewportH-contentHeight(scrollParent))
	if cursorAbsTop < viewTop {
		newScroll := scrollOffset +
			(viewTop - cursorAbsTop)
		sy.Set(scrollID,
			f32Clamp(newScroll, maxScrollNeg, 0))
		scrollSmoothCancel(w, scrollID, scrollAxisY)
	} else if cursorAbsBot > viewBot {
		newScroll := scrollOffset -
			(cursorAbsBot - viewBot)
		sy.Set(scrollID,
			f32Clamp(newScroll, maxScrollNeg, 0))
		scrollSmoothCancel(w, scrollID, scrollAxisY)
	}
}

// scrollMaxOffsetX returns the most-negative horizontal offset a
// layout can scroll to (0 when content fits).
func scrollMaxOffsetX(layout *Layout) float32 {
	return f32Min(0,
		layout.Shape.Width-layout.Shape.paddingWidth()-
			contentWidth(layout))
}

// scrollMaxOffsetY returns the most-negative vertical offset a
// layout can scroll to (0 when content fits).
func scrollMaxOffsetY(layout *Layout) float32 {
	return f32Min(0,
		layout.Shape.Height-layout.Shape.paddingHeight()-
			contentHeight(layout))
}

// scrollHorizontal adjusts the horizontal scroll offset of a
// scrollable layout. Returns true if offset was adjusted. Instant:
// used by the precise/trackpad and keyboard paths. The discrete
// mouse-wheel path eases via scrollSmoothBy instead.
func scrollHorizontal(layout *Layout, delta float32, w *Window) bool {
	id := layout.Shape.idKey()
	if !layout.Shape.Scrollable || id == "" ||
		layout.Shape.ScrollMode == ScrollVerticalOnly {
		return false
	}
	maxOffset := scrollMaxOffsetX(layout)
	sx := w.scrollX()
	// Default 0: unscrolled position when no offset recorded yet.
	old := sx.GetOr(id, 0)
	// Post-generation read: this window's theme, not the frame cache.
	clamped := f32Clamp(
		old+delta*w.themeRef().ScrollMultiplier, maxOffset, 0)
	if old == clamped {
		return false
	}
	sx.Set(id, clamped)
	scrollSmoothCancel(w, id, scrollAxisX)
	fireOnScroll(layout, w)
	return true
}

// scrollVertical adjusts the vertical scroll offset of a
// scrollable layout. Returns true if offset was adjusted. Instant:
// used by the precise/trackpad and keyboard paths. The discrete
// mouse-wheel path eases via scrollSmoothBy instead.
func scrollVertical(layout *Layout, delta float32, w *Window) bool {
	id := layout.Shape.idKey()
	if !layout.Shape.Scrollable || id == "" ||
		layout.Shape.ScrollMode == ScrollHorizontalOnly {
		return false
	}
	maxOffset := scrollMaxOffsetY(layout)
	sy := w.scrollY()
	// Default 0: unscrolled position when no offset recorded yet.
	old := sy.GetOr(id, 0)
	// Post-generation read: this window's theme, not the frame cache.
	clamped := f32Clamp(
		old+delta*w.themeRef().ScrollMultiplier, maxOffset, 0)
	if old == clamped {
		return false
	}
	sy.Set(id, clamped)
	scrollSmoothCancel(w, id, scrollAxisY)
	fireOnScroll(layout, w)
	return true
}

// ScrollToView scrolls the parent scroll container to make
// the view with the given id visible.
func (w *Window) scrollToView(effectiveID string) {
	target, ok := w.layout.FindByID(effectiveID)
	if !ok {
		return
	}
	p := target
	for p.Parent != nil {
		p = p.Parent
		if p.Shape.Scrollable {
			scrollID := p.Shape.idKey()
			// Default 0: unscrolled position when no offset
			// recorded yet.
			sy := w.scrollY()
			current := sy.GetOr(scrollID, 0)
			baseY := p.Shape.Y + p.Shape.Padding.Top
			newScroll := baseY - target.Shape.Y + current
			maxScrollNeg := scrollMaxOffsetY(p)
			sy.Set(scrollID,
				f32Clamp(newScroll, maxScrollNeg, 0))
			scrollSmoothCancel(w, scrollID, scrollAxisY)
			w.UpdateWindow()
			return
		}
	}
}

// ScrollHorizontalBy scrolls the given scrollable by delta. effectiveID is
// the scrollable's scroll key (see the Scrollable doc on each Cfg).
func (w *Window) scrollHorizontalBy(effectiveID string, delta float32) {
	scrollSmoothCancel(w, effectiveID, scrollAxisX)
	sx := w.scrollX()
	// Default 0: unscrolled position when no offset recorded yet.
	current := sx.GetOr(effectiveID, 0)
	newVal := current + delta
	if ly, ok := findScrollLayout(w, effectiveID); ok {
		maxOffset := scrollMaxOffsetX(ly)
		newVal = f32Clamp(newVal, maxOffset, 0)
		sx.Set(effectiveID, newVal)
		fireOnScroll(ly, w)
		return
	}
	sx.Set(effectiveID, newVal)
}

// ScrollHorizontalTo scrolls the given scrollable to offset
// (negative).
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
func (w *Window) ScrollHorizontalTo(effectiveID string, offset float32) {
	scrollSmoothCancel(w, effectiveID, scrollAxisX)
	sx := w.scrollX()
	if ly, ok := findScrollLayout(w, effectiveID); ok {
		maxOffset := scrollMaxOffsetX(ly)
		sx.Set(effectiveID, f32Clamp(offset, maxOffset, 0))
		fireOnScroll(ly, w)
		return
	}
	sx.Set(effectiveID, offset)
}

// ScrollHorizontalToSmooth eases the given scrollable to offset
// (negative) using the same exponential smoothing as discrete
// mouse-wheel scrolling. No-op if the scroll id is not found or the
// target equals the current offset. Use ScrollHorizontalTo for an
// instant jump.
func (w *Window) scrollHorizontalToSmooth(effectiveID string, offset float32) {
	if ly, ok := findScrollLayout(w, effectiveID); ok {
		scrollSmoothTo(w, ly, scrollAxisX, offset)
	}
}

// ScrollHorizontalToPct scrolls to a horizontal percentage.
// pct: 0.0 = left, 1.0 = right. Clamped to [0, 1].
// No-op if the scroll id is not found or content fits viewport.
func (w *Window) scrollHorizontalToPct(effectiveID string, pct float32) {
	// See ScrollHorizontalPct: the walk faults on an un-arranged tree.
	ly, ok := findScrollLayout(w, effectiveID)
	if !ok {
		return
	}
	maxOffset := scrollMaxOffsetX(ly)
	if maxOffset == 0 {
		return
	}
	sx := w.scrollX()
	sx.Set(effectiveID, maxOffset*f32Clamp(pct, 0, 1))
	scrollSmoothCancel(w, effectiveID, scrollAxisX)
}

// ScrollVerticalBy scrolls the given scrollable by delta. effectiveID is
// the scrollable's scroll key (see the Scrollable doc on each Cfg).
func (w *Window) scrollVerticalBy(effectiveID string, delta float32) {
	scrollSmoothCancel(w, effectiveID, scrollAxisY)
	sy := w.scrollY()
	// Default 0: unscrolled position when no offset recorded yet.
	current := sy.GetOr(effectiveID, 0)
	newVal := current + delta
	if ly, ok := findScrollLayout(w, effectiveID); ok {
		maxOffset := scrollMaxOffsetY(ly)
		newVal = f32Clamp(newVal, maxOffset, 0)
		sy.Set(effectiveID, newVal)
		fireOnScroll(ly, w)
		return
	}
	sy.Set(effectiveID, newVal)
}

// ScrollVerticalTo scrolls the given scrollable to offset
// (negative).
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
func (w *Window) ScrollVerticalTo(effectiveID string, offset float32) {
	scrollSmoothCancel(w, effectiveID, scrollAxisY)
	sy := w.scrollY()
	if ly, ok := findScrollLayout(w, effectiveID); ok {
		maxOffset := scrollMaxOffsetY(ly)
		sy.Set(effectiveID, f32Clamp(offset, maxOffset, 0))
		fireOnScroll(ly, w)
		return
	}
	// The offset is still recorded: setting one before the scrollable
	// is built is legitimate, and the next frame reads it. The gate
	// reports only the case that is not — a leaf spelled without the
	// scope the frame stamped it under.
	debugLookupMiss(&w.layout, "ScrollVerticalTo", effectiveID)
	sy.Set(effectiveID, offset)
}

// ScrollVerticalToSmooth eases the given scrollable to offset
// (negative) using the same exponential smoothing as discrete
// mouse-wheel scrolling. No-op if the scroll id is not found or the
// target equals the current offset. Use ScrollVerticalTo for an
// instant jump.
func (w *Window) scrollVerticalToSmooth(effectiveID string, offset float32) {
	if ly, ok := findScrollLayout(w, effectiveID); ok {
		scrollSmoothTo(w, ly, scrollAxisY, offset)
	}
}

// ScrollVerticalToPct scrolls to a vertical percentage.
// pct: 0.0 = top, 1.0 = bottom. Clamped to [0, 1].
// No-op if the scroll id is not found or content fits viewport.
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
func (w *Window) ScrollVerticalToPct(effectiveID string, pct float32) {
	// See ScrollHorizontalPct: the walk faults on an un-arranged tree.
	ly, ok := findScrollLayout(w, effectiveID)
	if !ok {
		debugLookupMiss(&w.layout, "ScrollVerticalToPct", effectiveID)
		return
	}
	maxOffset := scrollMaxOffsetY(ly)
	if maxOffset == 0 {
		return
	}
	sy := w.scrollY()
	sy.Set(effectiveID, maxOffset*f32Clamp(pct, 0, 1))
	scrollSmoothCancel(w, effectiveID, scrollAxisY)
}

// ScrollVerticalOffset returns the current vertical scroll offset of
// the given scrollable: <= 0, where 0 is the top. Unknown ids read
// as 0 (unscrolled).
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
func (w *Window) ScrollVerticalOffset(effectiveID string) float32 {
	// Default 0: unscrolled position when no offset recorded yet.
	return w.scrollY().GetOr(effectiveID, 0)
}

// ScrollVerticalPct returns the current vertical scroll
// position as a percentage (0.0 = top, 1.0 = bottom).
// Returns 0 if not found or content fits viewport.
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
func (w *Window) ScrollVerticalPct(effectiveID string) float32 {
	// See ScrollHorizontalPct: guard the un-arranged tree before the
	// walk dereferences the root Shape.
	ly, ok := findScrollLayout(w, effectiveID)
	if !ok {
		return 0
	}
	maxOffset := scrollMaxOffsetY(ly)
	if maxOffset == 0 {
		return 0
	}
	sy := w.scrollY()
	// Default 0: unscrolled position when no offset recorded yet.
	current := sy.GetOr(effectiveID, 0)
	return f32Clamp(current/maxOffset, 0, 1)
}

// ScrollHorizontalOffset returns the current horizontal scroll offset
// of the given scrollable: <= 0, where 0 is the left edge. Unknown ids
// read as 0 (unscrolled).
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
//
// exportaudit:keep — caller-facing scroll query, axis twin of
// ScrollVerticalOffset (issue #546)
func (w *Window) ScrollHorizontalOffset(effectiveID string) float32 {
	// Read-only accessor: a query must not allocate the state map as a
	// side effect of being asked about a container that never scrolled.
	sx := w.scrollXRead()
	if sx == nil {
		return 0
	}
	// Default 0: unscrolled position when no offset recorded yet.
	return sx.GetOr(effectiveID, 0)
}

// ScrollHorizontalPct returns the current horizontal scroll
// position as a percentage (0.0 = left, 1.0 = right).
// Returns 0 if not found or content fits viewport.
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
//
// exportaudit:keep — caller-facing scroll query, axis twin of
// ScrollVerticalPct (issue #546)
func (w *Window) ScrollHorizontalPct(effectiveID string) float32 {
	// findScrollLayout, not findLayoutByScrollID: the walk reads
	// Shape.Scrollable, so a window whose first frame has not been
	// arranged yet has a nil root Shape and would fault.
	ly, ok := findScrollLayout(w, effectiveID)
	if !ok {
		debugLookupMiss(&w.layout, "ScrollHorizontalPct", effectiveID)
		return 0
	}
	maxOffset := scrollMaxOffsetX(ly)
	if maxOffset == 0 {
		return 0
	}
	sx := w.scrollX()
	// Default 0: unscrolled position when no offset recorded yet.
	current := sx.GetOr(effectiveID, 0)
	return f32Clamp(current/maxOffset, 0, 1)
}

// ScrollOverflowX reports how much content the given scrollable hides
// on the X axis, as a positive width in pixels, and whether the
// scrollable was found. The width is 0 when the content fits.
//
// ok is false before the first frame is arranged and for an id
// nothing stamped. It is reported separately because the width alone
// cannot carry it: a miss and a scrollable with nothing to scroll both
// read 0, and a caller sizing a reservation from that would silently
// reserve nothing. Callers that only ask "is there overflow" may
// discard it.
//
// This answers "is there anything to scroll", which no other getter
// does: ScrollHorizontalPct reads 0 both at the left edge with content
// still to the right and when there is nothing to scroll at all.
//
// It does not report whether a scrollbar is drawn. A scrollbar's
// visibility also depends on ScrollbarCfg.Overflow, and
// ScrollbarVisible paints a bar over content that fits.
//
// The figure describes the frame that was last arranged. Sizing a
// reservation from it therefore feeds back into the measurement that
// produced it: reserve space, the content narrows, the overflow
// changes, and the layout can flip between the two states every frame.
// Reserve on an axis unconditionally, or hold the reservation once
// taken, rather than tracking this value directly.
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
//
// exportaudit:keep — caller-facing overflow query (issue #546)
func (w *Window) ScrollOverflowX(effectiveID string) (float32, bool) {
	ly, ok := findScrollLayout(w, effectiveID)
	if !ok {
		// A leaf spelled without its scope reaches here, and ok alone
		// does not say which mistake it was. Name the spelling the
		// frame stamped.
		debugLookupMiss(&w.layout, "ScrollOverflowX", effectiveID)
		return 0, false
	}
	// scrollMaxOffsetX is the most-negative reachable offset, so its
	// magnitude is the hidden width.
	return -scrollMaxOffsetX(ly), true
}

// ScrollOverflowY reports how much content the given scrollable hides
// on the Y axis, as a positive height in pixels, and whether the
// scrollable was found. The height is 0 when the content fits.
//
// The ok result, the reservation feedback loop and the
// scrollbar-visibility caveat are all as described on
// [Window.ScrollOverflowX].
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Read it back with [Window.ResolveID].
//
// exportaudit:keep — caller-facing overflow query (issue #546)
func (w *Window) ScrollOverflowY(effectiveID string) (float32, bool) {
	ly, ok := findScrollLayout(w, effectiveID)
	if !ok {
		// See ScrollOverflowX.
		debugLookupMiss(&w.layout, "ScrollOverflowY", effectiveID)
		return 0, false
	}
	// scrollMaxOffsetY is the most-negative reachable offset, so its
	// magnitude is the hidden height.
	return -scrollMaxOffsetY(ly), true
}

// inputScrollContentW returns how far an input's content reaches on the
// X axis, which is what its horizontal offset is clamped against.
//
// The two modes reach past the viewport by opposite routes, and each
// one's measure is the other's noise, so this picks rather than maxes.
//
// Multiline wraps, so its text shape is pinned to the wrap width and is
// always exactly the viewport — it says nothing. What reaches past is
// the ink recorded for a run that could not wrap, carried up to the
// field by propagateInkOverflow and read back through contentWidth.
// That figure already includes the caret (see layoutPlainText).
//
// Single-line never wraps, so the full text width is its intrinsic
// width — but read that from MinWidth, not Width. Text pins its
// measurement as a minimum and its Fill parent then stretches it, so
// for text that fits, Width is the viewport and says nothing, exactly
// like the container's content width. Plus the caret, painted just past
// the last glyph: without it a caret at the end of the text sits
// exactly on the clamp and renders half-clipped.
//
// Taking the max of every candidate instead would floor every input at
// the viewport plus a caret, so even a field whose text fits would
// report 1.5px of scrollable range and could paint itself shifted.
func inputScrollContentW(layout *Layout, txtShape *Shape) float32 {
	if layout == nil || txtShape == nil {
		return 0
	}
	if txtShape.TC != nil && txtShape.TC.overflowScrollX {
		content := contentWidth(layout)
		ink := txtShape.inkOverflowW
		if !f32IsFinite(content) {
			content = 0
		}
		if !f32IsFinite(ink) {
			ink = 0
		}
		return f32Max(content, ink)
	}
	intrinsic := txtShape.MinWidth
	if intrinsic <= 0 {
		intrinsic = txtShape.Width
	}
	if !f32IsFinite(intrinsic) || intrinsic < 0 {
		intrinsic = 0
	}
	return intrinsic + inputCaretW
}
