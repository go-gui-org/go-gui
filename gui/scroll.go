package gui

import "github.com/go-gui-org/go-glyph"

// findScrollLayout returns the layout for the scroll id, or false
// if the layout tree is not yet built or the id is not found.
func findScrollLayout(w *Window, id string) (*Layout, bool) {
	if w.layout.Shape == nil {
		return nil, false
	}
	return FindLayoutByScrollID(&w.layout, id)
}

// fireOnScroll fires the OnScroll callback if set.
func fireOnScroll(ly *Layout, w *Window) {
	if ly.Shape.hasEvents() && ly.Shape.events.OnScroll != nil {
		ly.Shape.events.OnScroll(ly, w)
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

// inputScrollCursorIntoView adjusts the vertical scroll of a
// multiline input so the cursor remains visible.
// layout must be the outer scroll container (Column with a scroll ID).
func inputScrollCursorIntoView(
	id string, text string, layout *Layout, w *Window,
) {
	if id == "" || w.textMeasurer == nil {
		return
	}
	if len(layout.Children) == 0 {
		return
	}
	inner := &layout.Children[0]
	if len(inner.Children) == 0 {
		return
	}
	txtShape := inner.Children[0].Shape
	if txtShape == nil || txtShape.TC == nil {
		return
	}
	style := textStyleOrDefault(txtShape)
	gl, ok := inputGlyphLayout(text, txtShape, style, w)
	if !ok {
		return
	}

	is := StateReadOr(w, nsInput,
		layout.Shape.ID, InputState{})
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	byteIdx := runeToByteIndex(text, pos)

	cp, ok := gl.GetCursorPos(byteIdx)
	if !ok {
		return
	}
	adjustCursorTrailing(&cp, gl.Lines, byteIdx, is.CursorTrailing)

	sy := w.scrollY()
	scrollOffset, _ := sy.Get(id)
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
	scrollID := scrollParent.Shape.ID

	text := shape.TC.Text
	style := textStyleOrDefault(shape)
	gl, ok := inputGlyphLayout(text, shape, style, w)
	if !ok {
		return
	}

	is := StateReadOr(
		w, nsInput, shape.ID, InputState{})
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	byteIdx := runeToByteIndex(text, pos)

	cp, ok := gl.GetCursorPos(byteIdx)
	if !ok {
		return
	}
	adjustCursorTrailing(&cp, gl.Lines, byteIdx, is.CursorTrailing)

	sy := w.scrollY()
	scrollOffset, _ := sy.Get(scrollID)
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
	id := layout.Shape.ID
	if !layout.Shape.Scrollable || id == "" ||
		layout.Shape.ScrollMode == ScrollVerticalOnly {
		return false
	}
	maxOffset := scrollMaxOffsetX(layout)
	sx := w.scrollX()
	old, _ := sx.Get(id)
	clamped := f32Clamp(
		old+delta*guiTheme.ScrollMultiplier, maxOffset, 0)
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
	id := layout.Shape.ID
	if !layout.Shape.Scrollable || id == "" ||
		layout.Shape.ScrollMode == ScrollHorizontalOnly {
		return false
	}
	maxOffset := scrollMaxOffsetY(layout)
	sy := w.scrollY()
	old, _ := sy.Get(id)
	clamped := f32Clamp(
		old+delta*guiTheme.ScrollMultiplier, maxOffset, 0)
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
func (w *Window) ScrollToView(id string) {
	target, ok := w.layout.FindByID(id)
	if !ok {
		return
	}
	p := target
	for p.Parent != nil {
		p = p.Parent
		if p.Shape.Scrollable {
			scrollID := p.Shape.ID
			sy := w.scrollY()
			current, _ := sy.Get(scrollID)
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

// ScrollHorizontalBy scrolls the given scrollable by delta. id is
// the scrollable's scroll key (see the Scrollable doc on each Cfg).
func (w *Window) ScrollHorizontalBy(id string, delta float32) {
	scrollSmoothCancel(w, id, scrollAxisX)
	sx := w.scrollX()
	current, _ := sx.Get(id)
	newVal := current + delta
	if ly, ok := findScrollLayout(w, id); ok {
		maxOffset := scrollMaxOffsetX(ly)
		newVal = f32Clamp(newVal, maxOffset, 0)
		sx.Set(id, newVal)
		fireOnScroll(ly, w)
		return
	}
	sx.Set(id, newVal)
}

// ScrollHorizontalTo scrolls the given scrollable to offset
// (negative).
func (w *Window) ScrollHorizontalTo(id string, offset float32) {
	scrollSmoothCancel(w, id, scrollAxisX)
	sx := w.scrollX()
	if ly, ok := findScrollLayout(w, id); ok {
		maxOffset := scrollMaxOffsetX(ly)
		sx.Set(id, f32Clamp(offset, maxOffset, 0))
		fireOnScroll(ly, w)
		return
	}
	sx.Set(id, offset)
}

// ScrollHorizontalToPct scrolls to a horizontal percentage.
// pct: 0.0 = left, 1.0 = right. Clamped to [0, 1].
// No-op if the scroll id is not found or content fits viewport.
func (w *Window) ScrollHorizontalToPct(id string, pct float32) {
	ly, ok := FindLayoutByScrollID(&w.layout, id)
	if !ok {
		return
	}
	maxOffset := scrollMaxOffsetX(ly)
	if maxOffset == 0 {
		return
	}
	sx := w.scrollX()
	sx.Set(id, maxOffset*f32Clamp(pct, 0, 1))
	scrollSmoothCancel(w, id, scrollAxisX)
}

// ScrollHorizontalPct returns the current horizontal scroll
// position as a percentage (0.0 = left, 1.0 = right).
// Returns 0 if not found or content fits viewport.
func (w *Window) ScrollHorizontalPct(id string) float32 {
	ly, ok := FindLayoutByScrollID(&w.layout, id)
	if !ok {
		return 0
	}
	maxOffset := scrollMaxOffsetX(ly)
	if maxOffset == 0 {
		return 0
	}
	sx := w.scrollX()
	current, _ := sx.Get(id)
	return f32Clamp(current/maxOffset, 0, 1)
}

// ScrollVerticalBy scrolls the given scrollable by delta. id is
// the scrollable's scroll key (see the Scrollable doc on each Cfg).
func (w *Window) ScrollVerticalBy(id string, delta float32) {
	scrollSmoothCancel(w, id, scrollAxisY)
	sy := w.scrollY()
	current, _ := sy.Get(id)
	newVal := current + delta
	if ly, ok := findScrollLayout(w, id); ok {
		maxOffset := scrollMaxOffsetY(ly)
		newVal = f32Clamp(newVal, maxOffset, 0)
		sy.Set(id, newVal)
		fireOnScroll(ly, w)
		return
	}
	sy.Set(id, newVal)
}

// ScrollVerticalTo scrolls the given scrollable to offset
// (negative).
func (w *Window) ScrollVerticalTo(id string, offset float32) {
	scrollSmoothCancel(w, id, scrollAxisY)
	sy := w.scrollY()
	if ly, ok := findScrollLayout(w, id); ok {
		maxOffset := scrollMaxOffsetY(ly)
		sy.Set(id, f32Clamp(offset, maxOffset, 0))
		fireOnScroll(ly, w)
		return
	}
	sy.Set(id, offset)
}

// ScrollVerticalToPct scrolls to a vertical percentage.
// pct: 0.0 = top, 1.0 = bottom. Clamped to [0, 1].
// No-op if the scroll id is not found or content fits viewport.
func (w *Window) ScrollVerticalToPct(id string, pct float32) {
	ly, ok := FindLayoutByScrollID(&w.layout, id)
	if !ok {
		return
	}
	maxOffset := scrollMaxOffsetY(ly)
	if maxOffset == 0 {
		return
	}
	sy := w.scrollY()
	sy.Set(id, maxOffset*f32Clamp(pct, 0, 1))
	scrollSmoothCancel(w, id, scrollAxisY)
}

// ScrollVerticalPct returns the current vertical scroll
// position as a percentage (0.0 = top, 1.0 = bottom).
// Returns 0 if not found or content fits viewport.
func (w *Window) ScrollVerticalPct(id string) float32 {
	ly, ok := FindLayoutByScrollID(&w.layout, id)
	if !ok {
		return 0
	}
	maxOffset := scrollMaxOffsetY(ly)
	if maxOffset == 0 {
		return 0
	}
	sy := w.scrollY()
	current, _ := sy.Get(id)
	return f32Clamp(current/maxOffset, 0, 1)
}
