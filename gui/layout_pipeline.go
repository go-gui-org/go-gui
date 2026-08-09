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
	for i := range layout.Children {
		layoutAmend(&layout.Children[i], w)
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
		if layoutHover(&layout.Children[i], w) {
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
		MouseButton: MouseInvalid,
	}
	shape.events.OnHover(EventCtx{layout, &w.scratch.hoverEvent, w})
	return true
}

// layoutMouseLeave walks the entire layout tree, firing OnMouseLeave on any
// shape whose hover state transitioned inside→outside this frame. shape.ID
// must be non-empty; shapes with an empty ID are silently skipped.
func layoutMouseLeave(layout *Layout, w *Window) {
	if w.mouseIsLocked() {
		return
	}
	savedX, savedY := w.viewState.mousePosX, w.viewState.mousePosY
	if layout.Shape.QuarterTurns > 0 {
		w.viewState.mousePosX, w.viewState.mousePosY =
			rotateCoordsInverse(layout.Shape, savedX, savedY)
	}
	for i := range layout.Children {
		layoutMouseLeave(&layout.Children[i], w)
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
	for i := range layout.Children {
		layoutWrapTextWalk(&layout.Children[i], w)
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
		layoutPlainText(shape, tc, style, w)
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
	themeName := guiTheme.Name
	if vs.rtfLayoutCache != nil && vs.rtfLayoutTheme != themeName {
		vs.rtfLayoutCache.Clear()
		vs.rtfLayoutTheme = themeName
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
		vs.rtfLayoutTheme = themeName
	}
	vs.rtfLayoutCache.Set(cacheKey, rtfLayoutEntry{
		Layout: l,
	})
}

// layoutPlainText computes final text dimensions after sizing.
// Mirrors the initial estimate in view_text.go:GenerateLayout.
func layoutPlainText(
	shape *Shape,
	tc *shapeTextConfig,
	style TextStyle,
	w *Window,
) {
	if len(tc.Text) == 0 {
		return
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
		return
	}
	if tc.TextStyle == nil {
		return
	}
	l, ok := plainTextLayoutResolved(tc.Text, shape, style, w)
	if !ok {
		return
	}
	shape.Height = l.Height
	if tc.TextMode == TextModeMultiline &&
		shape.Sizing.Width != sizingFixed && l.Width > 0 {
		shape.Width = l.Width
	}
}
