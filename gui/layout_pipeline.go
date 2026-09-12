package gui

import (
	"math"
	"slices"

	"github.com/go-gui-org/go-glyph"
)

// layoutPipeline runs all layout passes in order on a single
// layout tree.
func layoutPipeline(layout *Layout, w *Window) {
	// Identities are already stamped: layoutArrange resolves the whole
	// tree before it splits the floats out, and each injected overlay as
	// it is generated. Resolving here instead would see a float as its
	// own root and strip the scope its ancestors gave it.

	// Width passes.
	layoutWidths(layout)
	w.scratch.beginFillPass()
	layoutFillWidths(layout, &w.scratch)
	layoutWrapContainers(layout, w)
	layoutOverflow(layout, w)
	layoutWrapText(layout, w)

	// Height passes.
	layoutHeights(layout)
	layoutFillHeights(layout, &w.scratch)
	layoutRotationSwap(layout)

	// Position passes.
	layoutAdjustScrollOffsets(layout, w)
	fx, fy := floatAttachLayout(layout, w.windowRect())
	layoutPositions(layout, fx, fy, w)
	layoutApplyScrollAnchors(layout, w)
	layoutApplyVirtualScrolls(layout, w)
	layoutDisables(layout, false)

	// Post-position passes.
	layoutAmend(layout, w)
	applyLayoutTransition(layout, w)
	applyHeroTransition(layout, w)
	layoutSetShapeClips(layout, w.windowRect())
}

// layoutAmend walks the layout tree children-first, firing
// AmendLayout callbacks. Not for size changes — post-position
// only.
func layoutAmend(layout *Layout, w *Window) {
	layoutAmendDepth(layout, w, 0)
}

func layoutAmendDepth(layout *Layout, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	for i := range layout.Children {
		layoutAmendDepth(&layout.Children[i], w, depth+1)
	}
	if layout.Shape.hasEvents() &&
		layout.Shape.events.AmendLayout != nil {
		layout.Shape.events.AmendLayout(EventCtx{layout, nil, w})
	}
}

// layoutHover walks the layout tree depth-first, firing OnHover
// callbacks when the mouse is inside a shape. Returns true if
// any hover was handled.
// SAFETY: mutates w.viewState.mousePosX/Y to compensate for child
// rotation. Called from layoutArrange, which runs under w.mu.
func layoutHover(layout *Layout, w *Window) bool {
	return layoutHoverDepth(layout, w, 0)
}

func layoutHoverDepth(layout *Layout, w *Window, depth int) bool {
	if overMaxDepth(depth) {
		return false
	}
	if w.mouseIsLocked() {
		return false
	}
	// Apply inverse rotation for children of rotated containers.
	savedX, savedY := w.viewState.mousePosX, w.viewState.mousePosY
	if layout.Shape.QuarterTurns > 0 {
		w.viewState.mousePosX, w.viewState.mousePosY =
			rotateCoordsInverse(layout.Shape, savedX, savedY)
	}
	for i := range slices.Backward(layout.Children) {
		if layoutHoverDepth(&layout.Children[i], w, depth+1) {
			w.viewState.mousePosX, w.viewState.mousePosY = savedX, savedY
			return true
		}
	}
	w.viewState.mousePosX, w.viewState.mousePosY = savedX, savedY
	shape := layout.Shape
	if shape.Disabled {
		return false
	}
	if !shape.hasEvents() || shape.events.OnHover == nil {
		return false
	}
	if !shape.PointInShape(w.viewState.mousePosX,
		w.viewState.mousePosY) {
		return false
	}
	if w.dialogCfg.visible &&
		!layoutInDialogLayout(layout) {
		return false
	}
	w.scratch.hoverEvent = Event{
		MouseX:      w.viewState.mousePosX,
		MouseY:      w.viewState.mousePosY,
		Type:        EventMouseMove,
		MouseButton: w.viewState.mouseButtonHeld,
	}
	shape.events.OnHover(EventCtx{layout, &w.scratch.hoverEvent, w})
	return true
}

// layoutMouseLeave walks the entire layout tree, firing OnMouseLeave on any
// shape whose hover state transitioned inside→outside this frame. shape.ID
// must be non-empty; shapes with an empty ID are silently skipped.
func layoutMouseLeave(layout *Layout, w *Window) {
	layoutMouseLeaveDepth(layout, w, 0)
}

func layoutMouseLeaveDepth(layout *Layout, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	if w.mouseIsLocked() {
		return
	}
	savedX, savedY := w.viewState.mousePosX, w.viewState.mousePosY
	if layout.Shape.QuarterTurns > 0 {
		w.viewState.mousePosX, w.viewState.mousePosY =
			rotateCoordsInverse(layout.Shape, savedX, savedY)
	}
	for i := range layout.Children {
		layoutMouseLeaveDepth(&layout.Children[i], w, depth+1)
	}
	w.viewState.mousePosX, w.viewState.mousePosY = savedX, savedY

	shape := layout.Shape
	if shape == nil || shape.Disabled || shape.events == nil ||
		shape.events.OnMouseLeave == nil || shape.ID == "" {
		return
	}
	sm := w.hoverInside()
	key := shape.idKey()
	inside := shape.PointInShape(w.viewState.mousePosX, w.viewState.mousePosY)
	// Default false: absent entry means cursor was not previously in shape.
	wasInside := sm.GetOr(key, false)
	if wasInside && !inside {
		w.scratch.hoverEvent = Event{
			MouseX:      w.viewState.mousePosX,
			MouseY:      w.viewState.mousePosY,
			Type:        EventMouseMove,
			MouseButton: MouseInvalid,
		}
		shape.events.OnMouseLeave(EventCtx{layout, &w.scratch.hoverEvent, w})
	}
	if inside {
		sm.Set(key, true)
	} else {
		sm.Delete(key)
	}
}

// layoutInDialogLayout walks the parent chain checking if any
// ancestor has ID == reservedDialogID.
func layoutInDialogLayout(layout *Layout) bool {
	for p := layout; p != nil; p = p.Parent {
		if p.Shape.ID == reservedDialogID {
			return true
		}
	}
	return false
}

// layoutWrapText re-layouts text and RTF shapes whose height depends on
// glyph layout. Called after fill-widths so actual widths are known.
func layoutWrapText(layout *Layout, w *Window) {
	if layout == nil || w == nil {
		return
	}
	layoutWrapTextWalk(layout, w)
}

func layoutWrapTextWalk(layout *Layout, w *Window) {
	layoutWrapTextWalkDepth(layout, w, 0)
}

func layoutWrapTextWalkDepth(layout *Layout, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	for i := range layout.Children {
		layoutWrapTextWalkDepth(&layout.Children[i], w, depth+1)
	}
	shape := layout.Shape
	tc := shape.TC
	if tc == nil {
		return
	}
	switch shape.shapeType {
	case shapeRTF:
		if tc.TextMode != TextModeWrap &&
			tc.TextMode != TextModeWrapKeepSpaces {
			return
		}
		if shape.Width <= 0 {
			return
		}
		layoutWrapRTF(shape, tc, w)
	case shapeText:
		style := textStyleOrDefault(shape)
		if !plainTextNeedsGlyphLayout(shape, tc, style) {
			return
		}
		if layoutPlainText(
			shape, tc, style, wrapParentHAlign(layout), w,
		) {
			// The box gave width back, so the parent's contentW cache
			// — taken in the fill pass, before the glyph layout existed
			// — is now too wide. Every reader of that cache is a scroll
			// one (the clamp in layoutAdjustScrollOffsets, the
			// scrollbar thumb), so only a viewport needs the refresh;
			// recomputing for every wrapped child of a plain column
			// would be quadratic in the child count for a number
			// nothing reads. Only the immediate parent changes: every
			// ancestor above it reads the parent's Width, which the
			// shrink leaves alone.
			if p := layout.Parent; p != nil && p.Shape != nil &&
				(p.Shape.Scrollable || p.Shape.Clip) {
				p.Shape.contentW = computeContentWidth(p)
			}
		}
		if shape.inkOverflowW > 0 {
			propagateInkOverflow(layout)
		}
	}
}

// propagateInkOverflow tells the ancestors of an overflowing text shape
// how far its ink actually reaches, so a scroll container above it can
// scroll to the ink instead of stopping at the box.
//
// Two things happen per ancestor. Its contentW cache is refreshed,
// because layoutFillWidths cached it one pass before the glyph layout
// existed (refitFitAncestors in layout_wrap.go does the same for the
// wrap pass). And the overflow is carried one level further up, unless
// this ancestor is a viewport — a Clip or Scrollable node is where the
// ink stops being visible, so it takes the refreshed contentW (that is
// what makes scrolling possible) but does not leak the overflow to its
// own parent, which would widen the whole page.
//
// Ordering: called from layoutWrapTextWalk, which runs after the fill
// widths and before layoutAdjustScrollOffsets, so a width discovered
// here still reaches the scroll clamp, positioning and the scrollbar
// thumb in the same frame. Parent pointers are set by layoutParents at
// the top of layoutArrange, and shapes are rebuilt every frame, so
// inkOverflowW needs no reset.
func propagateInkOverflow(node *Layout) {
	if node == nil || node.Shape == nil {
		return
	}
	ink := node.Shape.inkOverflowW
	if !f32IsFinite(ink) || ink <= 0 {
		return
	}
	for p := node.Parent; p != nil; p = p.Parent {
		if p.Shape == nil {
			return
		}
		p.Shape.contentW = computeContentWidth(p)
		if p.Shape.Clip || p.Shape.Scrollable {
			return
		}
		p.Shape.inkOverflowW = f32Max(p.Shape.inkOverflowW, ink)
	}
}

// rtfLayoutEntry caches a shaped RTF layout.
type rtfLayoutEntry struct {
	Layout glyph.Layout
}

func layoutWrapRTF(shape *Shape, tc *shapeTextConfig, w *Window) {
	if tc.rTFRuns == nil {
		return
	}
	if tc.wrapCacheValid &&
		f32AreClose(tc.wrapCacheWidth, shape.Width) &&
		tc.rTFLayout != nil {
		shape.Height = tc.wrapCacheHeight
		return
	}

	// Cross-frame cache: content hash XOR'd with width and
	// base style bits so different styles don't collide. Math
	// cache state mixed in so layout invalidates when an inline
	// math fetch transitions Loading→Ready (different glyph
	// runs: raw LaTeX text vs InlineObject placeholder).
	contentKey := rtfRunsKey(tc.rTFRuns)
	styleKey := rtfStyleKey(tc.rTFBaseStyle)
	mathKey := rtfMathStateKey(tc.rTFRuns, w.viewState.diagramCache)
	cacheKey := contentKey ^ styleKey ^ mathKey ^
		uint64(math.Float32bits(shape.Width)) ^
		(uint64(math.Float32bits(tc.rTFLineSpacing)) << 1)
	vs := &w.viewState

	// Invalidate on theme change.
	themeID := guiTheme.id
	if vs.rtfLayoutCache != nil && vs.rtfLayoutTheme != themeID {
		vs.rtfLayoutCache.Clear()
		vs.rtfLayoutTheme = themeID
	}

	// Check cross-frame cache.
	if vs.rtfLayoutCache != nil {
		if entry, ok := vs.rtfLayoutCache.Get(cacheKey); ok {
			tc.rTFLayout = &entry.Layout
			shape.Height = entry.Layout.Height
			tc.wrapCacheWidth = shape.Width
			tc.wrapCacheHeight = entry.Layout.Height
			tc.wrapCacheValid = true
			if tc.rTFFlatText == "" {
				tc.rTFFlatText = rtfFlatTextFromRuns(tc.rTFRuns)
			}
			return
		}
	}

	tm, ok := w.textMeasurer.(interface {
		LayoutRichText(glyph.RichText, glyph.TextConfig) (glyph.Layout, error)
	})
	if !ok {
		return
	}
	var vgRT glyph.RichText
	if tc.rtfGlyphRT != nil {
		vgRT = *tc.rtfGlyphRT
	} else {
		var mh []int64
		vgRT, mh = tc.rTFRuns.toGlyphRichTextWithMath(
			w.viewState.diagramCache)
		tc.rtfMathHashes = mh
	}
	cfg := glyph.TextConfig{
		Style: tc.rTFBaseStyle,
		Block: glyph.BlockStyle{
			Wrap:        glyph.WrapWord,
			Width:       shape.Width,
			Indent:      -tc.hangingIndent,
			LineSpacing: tc.rTFLineSpacing,
		},
	}
	l, err := tm.LayoutRichText(vgRT, cfg)
	if err != nil {
		return
	}
	rtfSuppressInlineObjectGlyphs(&l)
	tc.rTFLayout = &l
	shape.Height = l.Height
	tc.wrapCacheWidth = shape.Width
	tc.wrapCacheHeight = l.Height
	tc.wrapCacheValid = true
	if tc.rTFFlatText == "" {
		tc.rTFFlatText = rtfFlatTextFromRuns(tc.rTFRuns)
	}

	// Store in cross-frame cache.
	if vs.rtfLayoutCache == nil {
		vs.rtfLayoutCache = NewBoundedMap[uint64, rtfLayoutEntry](200)
		vs.rtfLayoutTheme = themeID
	}
	vs.rtfLayoutCache.Set(cacheKey, rtfLayoutEntry{
		Layout: l,
	})
}

// wrapParentHAlign reports the physical alignment the parent would give
// this child across a column's cross axis, or HAlignLeft when there is
// none to inherit.
//
// Only a column (axisTopToBottom) answers. In a row HAlign is the main
// axis: it places the children as a group, so narrowing one of them
// there would open a gap rather than move the text.
func wrapParentHAlign(layout *Layout) HorizontalAlign {
	parent := layout.Parent
	if parent == nil || parent.Shape == nil ||
		parent.Shape.Axis != axisTopToBottom {
		return HAlignLeft
	}
	isRTL := effectiveTextDir(parent.Shape) == TextDirRTL
	return resolveHAlign(parent.Shape.HAlign, isRTL)
}

// layoutPlainText computes final text dimensions after sizing.
// Mirrors the initial estimate in view_text.go:GenerateLayout.
// It reports whether shrinkWrapToInk narrowed the box, which the
// caller needs so the parent's content-width cache can follow.
func layoutPlainText(
	shape *Shape,
	tc *shapeTextConfig,
	style TextStyle,
	parentHAlign HorizontalAlign,
	w *Window,
) bool {
	if len(tc.Text) == 0 {
		return false
	}
	if w.textMeasurer == nil {
		// Headless: approximate rather than leave the single-line
		// estimate standing. Only the height, and only when it grows —
		// the estimate already covers the one-line case, and shrinking
		// it here would fight sizing over a number this path cannot
		// know accurately.
		if h := plainTextHeightNoMeasurer(shape, tc, style, w); h > shape.Height {
			shape.Height = h
		}
		return false
	}
	if tc.TextStyle == nil {
		return false
	}
	l, ok := plainTextLayoutResolved(tc.Text, shape, style, w)
	if !ok {
		return false
	}
	shape.Height = plainTextBoxHeight(l, style, w)
	if tc.TextMode == TextModeMultiline &&
		shape.Sizing.Width != sizingFixed && l.Width > 0 {
		shape.Width = l.Width
	}
	shrank := shrinkWrapToInk(shape, tc, style, l, parentHAlign)
	// A wrapped run with no break opportunity is wider than the width it
	// was wrapped to. Record the excess rather than growing the shape:
	// the shape's width IS the wrap width, so growing it would re-wrap
	// the text and undo the overflow that is being measured.
	if tc.overflowScrollX && f32IsFinite(l.Width) &&
		l.Width > shape.Width+f32Tolerance &&
		(tc.TextMode == TextModeWrap ||
			tc.TextMode == TextModeWrapKeepSpaces) {
		// Plus the caret: it is painted ink too, and the extent recorded
		// here is what layoutAdjustScrollOffsets clamps the field's
		// horizontal offset against. Without it the pipeline clamps the
		// caret's own width back off and the caret at the end of the run
		// is never quite reachable. Only Input's text sets
		// overflowScrollX, so no other shape pays for the caret.
		shape.inkOverflowW = l.Width + inputCaretW
	}
	return shrank
}

// shrinkWrapToInk pulls a wrapped text box back to its longest line so
// its container's HAlign has somewhere to move it (#577).
//
// A wrap box fills the axis because wrapping needs a width to wrap to.
// That leaves childCrossAxisHAlign with remaining == 0, so a centered
// column looks like it ignores its wrapped child. Shrinking the box to
// the ink it actually holds gives the existing alignment code its slack
// back, and the lines inside the box do not move: they are laid out
// from the box's left edge, and no line is wider than the longest one.
//
// Re-wrapping cannot follow from this. The layout l is already shaped at
// the old width, and next frame layoutFillWidths resets the box to the
// parent's content width before this pass runs again, so the width fed
// to the shaper is the same every frame.
func shrinkWrapToInk(
	shape *Shape,
	tc *shapeTextConfig,
	style TextStyle,
	l glyph.Layout,
	parentHAlign HorizontalAlign,
) bool {
	switch {
	case parentHAlign == HAlignLeft:
		// Nothing to move it to. Leaving the box full width keeps a
		// background, border or hit box at the extent it has always
		// had, which is every wrapped text that did not ask for this.
		return false
	case tc.TextMode != TextModeWrap &&
		tc.TextMode != TextModeWrapKeepSpaces:
		return false
	case !tc.wrapSizingDefault:
		// The caller named the Sizing. That is an instruction.
		return false
	case tc.overflowScrollX:
		// Input's text shape: its width is the viewport the caret
		// scrolls within, not a measurement of the text.
		return false
	case shape.Float:
		// A float is placed by its anchor, not by the alignment of the
		// container it was declared in, and layoutRemoveFloatingLayouts
		// leaves Parent pointing at that container. Its HAlign says
		// nothing about where this box goes.
		return false
	case shape.Sizing.Width == sizingFixed:
		return false
	case style.Align != TextAlignLeft:
		// glyph centred the lines against the wrap width already.
		// Moving the box now would double the offset.
		return false
	case !f32IsFinite(l.Width) || l.Width <= 0 ||
		l.Width >= shape.Width-f32Tolerance:
		return false
	}
	shape.Width = l.Width
	// Keep the shaped layout current for the new width. The cache key is
	// the width handed to the shaper, and re-wrapping at the longest line
	// yields the same breaks — every line already fits it, and a word
	// that did not fit the wider box does not fit this one — so the
	// render pass can reuse what this pass shaped instead of shaping the
	// same text a second time.
	tc.textLayoutWidth = shape.Width
	return true
}
