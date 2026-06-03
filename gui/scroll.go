package gui

import "github.com/go-gui-org/go-glyph"

// findScrollLayout returns the layout for idScroll, or false
// if the layout tree is not yet built or idScroll is not found.
func findScrollLayout(w *Window, idScroll uint32) (*Layout, bool) {
	if w.layout.Shape == nil {
		return nil, false
	}
	return FindLayoutByIDScroll(&w.layout, idScroll)
}

// fireOnScroll fires the OnScroll callback if set.
func fireOnScroll(ly *Layout, w *Window) {
	if ly.Shape.HasEvents() && ly.Shape.Events.OnScroll != nil {
		ly.Shape.Events.OnScroll(ly, w)
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
// layout must be the outer scroll container (Column with IDScroll).
func inputScrollCursorIntoView(
	idScroll uint32, text string, layout *Layout, w *Window,
) {
	if idScroll == 0 || w.textMeasurer == nil {
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
		layout.Shape.IDFocus, InputState{})
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	byteIdx := runeToByteIndex(text, pos)

	cp, ok := gl.GetCursorPos(byteIdx)
	if !ok {
		return
	}
	adjustCursorTrailing(&cp, gl.Lines, byteIdx, is.CursorTrailing)

	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	scrollOffset, _ := sy.Get(idScroll)
	viewportH := layout.Shape.Height - layout.Shape.PaddingHeight()

	cursorTop := cp.Y
	cursorBot := cp.Y + cp.Height
	visibleTop := -scrollOffset
	visibleBot := visibleTop + viewportH

	if cursorTop < visibleTop {
		sy.Set(idScroll, -cursorTop)
	} else if cursorBot > visibleBot {
		sy.Set(idScroll, -(cursorBot - viewportH))
	}
}

// textScrollCursorIntoView adjusts the vertical scroll of the
// nearest scroll ancestor so the text cursor stays visible.
// Used by the read-only text widget's keyboard handler.
func textScrollCursorIntoView(layout *Layout, w *Window) {
	shape := layout.Shape
	if shape == nil || shape.TC == nil ||
		shape.IDFocus == 0 || w.textMeasurer == nil {
		return
	}

	// Find nearest scroll ancestor.
	var scrollParent *Layout
	for p := layout.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.IDScroll > 0 {
			scrollParent = p
			break
		}
	}
	if scrollParent == nil {
		return
	}
	scrollID := scrollParent.Shape.IDScroll

	text := shape.TC.Text
	style := textStyleOrDefault(shape)
	gl, ok := inputGlyphLayout(text, shape, style, w)
	if !ok {
		return
	}

	is := StateReadOr(
		w, nsInput, shape.IDFocus, InputState{})
	runeLen := utf8RuneCount(text)
	pos := is.CursorPos
	pos = min(pos, runeLen)
	byteIdx := runeToByteIndex(text, pos)

	cp, ok := gl.GetCursorPos(byteIdx)
	if !ok {
		return
	}
	adjustCursorTrailing(&cp, gl.Lines, byteIdx, is.CursorTrailing)

	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	scrollOffset, _ := sy.Get(scrollID)
	sp := scrollParent.Shape
	viewportH := sp.Height - sp.PaddingHeight()
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
	} else if cursorAbsBot > viewBot {
		newScroll := scrollOffset -
			(cursorAbsBot - viewBot)
		sy.Set(scrollID,
			f32Clamp(newScroll, maxScrollNeg, 0))
	}
}

// scrollHorizontal adjusts the horizontal scroll offset of a
// scrollable layout. Returns true if offset was adjusted.
func scrollHorizontal(layout *Layout, delta float32, w *Window) bool {
	idScroll := layout.Shape.IDScroll
	if idScroll == 0 ||
		layout.Shape.ScrollMode == ScrollVerticalOnly {
		return false
	}
	maxOffset := f32Min(0,
		layout.Shape.Width-layout.Shape.PaddingWidth()-
			contentWidth(layout))
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	old, _ := sx.Get(idScroll)
	clamped := f32Clamp(
		old+delta*guiTheme.ScrollMultiplier, maxOffset, 0)
	if old == clamped {
		return false
	}
	sx.Set(idScroll, clamped)
	fireOnScroll(layout, w)
	return true
}

// scrollVertical adjusts the vertical scroll offset of a
// scrollable layout. Returns true if offset was adjusted.
func scrollVertical(layout *Layout, delta float32, w *Window) bool {
	idScroll := layout.Shape.IDScroll
	if idScroll == 0 ||
		layout.Shape.ScrollMode == ScrollHorizontalOnly {
		return false
	}
	maxOffset := f32Min(0,
		layout.Shape.Height-layout.Shape.PaddingHeight()-
			contentHeight(layout))
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	old, _ := sy.Get(idScroll)
	clamped := f32Clamp(
		old+delta*guiTheme.ScrollMultiplier, maxOffset, 0)
	if old == clamped {
		return false
	}
	sy.Set(idScroll, clamped)
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
		if p.Shape.IDScroll > 0 {
			scrollID := p.Shape.IDScroll
			sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
			current, _ := sy.Get(scrollID)
			baseY := p.Shape.Y + p.Shape.Padding.Top
			newScroll := baseY - target.Shape.Y + current
			maxScrollNeg := f32Min(0,
				p.Shape.Height-p.Shape.PaddingHeight()-
					contentHeight(p))
			sy.Set(scrollID,
				f32Clamp(newScroll, maxScrollNeg, 0))
			w.UpdateWindow()
			return
		}
	}
}

// ScrollHorizontalBy scrolls the given scrollable by delta.
func (w *Window) ScrollHorizontalBy(idScroll uint32, delta float32) {
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	current, _ := sx.Get(idScroll)
	newVal := current + delta
	if ly, ok := findScrollLayout(w, idScroll); ok {
		maxOffset := f32Min(0,
			ly.Shape.Width-ly.Shape.PaddingWidth()-
				contentWidth(ly))
		newVal = f32Clamp(newVal, maxOffset, 0)
		sx.Set(idScroll, newVal)
		fireOnScroll(ly, w)
		return
	}
	sx.Set(idScroll, newVal)
}

// ScrollHorizontalTo scrolls the given scrollable to offset
// (negative).
func (w *Window) ScrollHorizontalTo(idScroll uint32, offset float32) {
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	if ly, ok := findScrollLayout(w, idScroll); ok {
		maxOffset := f32Min(0,
			ly.Shape.Width-ly.Shape.PaddingWidth()-
				contentWidth(ly))
		sx.Set(idScroll, f32Clamp(offset, maxOffset, 0))
		fireOnScroll(ly, w)
		return
	}
	sx.Set(idScroll, offset)
}

// ScrollHorizontalToPct scrolls to a horizontal percentage.
// pct: 0.0 = left, 1.0 = right. Clamped to [0, 1].
// No-op if id_scroll not found or content fits viewport.
func (w *Window) ScrollHorizontalToPct(idScroll uint32, pct float32) {
	ly, ok := FindLayoutByIDScroll(&w.layout, idScroll)
	if !ok {
		return
	}
	maxOffset := f32Min(0,
		ly.Shape.Width-ly.Shape.PaddingWidth()-
			contentWidth(ly))
	if maxOffset == 0 {
		return
	}
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	sx.Set(idScroll, maxOffset*f32Clamp(pct, 0, 1))
}

// ScrollHorizontalPct returns the current horizontal scroll
// position as a percentage (0.0 = left, 1.0 = right).
// Returns 0 if not found or content fits viewport.
func (w *Window) ScrollHorizontalPct(idScroll uint32) float32 {
	ly, ok := FindLayoutByIDScroll(&w.layout, idScroll)
	if !ok {
		return 0
	}
	maxOffset := f32Min(0,
		ly.Shape.Width-ly.Shape.PaddingWidth()-
			contentWidth(ly))
	if maxOffset == 0 {
		return 0
	}
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	current, _ := sx.Get(idScroll)
	return f32Clamp(current/maxOffset, 0, 1)
}

// ScrollVerticalBy scrolls the given scrollable by delta.
func (w *Window) ScrollVerticalBy(idScroll uint32, delta float32) {
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	current, _ := sy.Get(idScroll)
	newVal := current + delta
	if ly, ok := findScrollLayout(w, idScroll); ok {
		maxOffset := f32Min(0,
			ly.Shape.Height-ly.Shape.PaddingHeight()-
				contentHeight(ly))
		newVal = f32Clamp(newVal, maxOffset, 0)
		sy.Set(idScroll, newVal)
		fireOnScroll(ly, w)
		return
	}
	sy.Set(idScroll, newVal)
}

// ScrollVerticalTo scrolls the given scrollable to offset
// (negative).
func (w *Window) ScrollVerticalTo(idScroll uint32, offset float32) {
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	if ly, ok := findScrollLayout(w, idScroll); ok {
		maxOffset := f32Min(0,
			ly.Shape.Height-ly.Shape.PaddingHeight()-
				contentHeight(ly))
		sy.Set(idScroll, f32Clamp(offset, maxOffset, 0))
		fireOnScroll(ly, w)
		return
	}
	sy.Set(idScroll, offset)
}

// ScrollVerticalToPct scrolls to a vertical percentage.
// pct: 0.0 = top, 1.0 = bottom. Clamped to [0, 1].
// No-op if id_scroll not found or content fits viewport.
func (w *Window) ScrollVerticalToPct(idScroll uint32, pct float32) {
	ly, ok := FindLayoutByIDScroll(&w.layout, idScroll)
	if !ok {
		return
	}
	maxOffset := f32Min(0,
		ly.Shape.Height-ly.Shape.PaddingHeight()-
			contentHeight(ly))
	if maxOffset == 0 {
		return
	}
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	sy.Set(idScroll, maxOffset*f32Clamp(pct, 0, 1))
}

// ScrollVerticalPct returns the current vertical scroll
// position as a percentage (0.0 = top, 1.0 = bottom).
// Returns 0 if not found or content fits viewport.
func (w *Window) ScrollVerticalPct(idScroll uint32) float32 {
	ly, ok := FindLayoutByIDScroll(&w.layout, idScroll)
	if !ok {
		return 0
	}
	maxOffset := f32Min(0,
		ly.Shape.Height-ly.Shape.PaddingHeight()-
			contentHeight(ly))
	if maxOffset == 0 {
		return 0
	}
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	current, _ := sy.Get(idScroll)
	return f32Clamp(current/maxOffset, 0, 1)
}
