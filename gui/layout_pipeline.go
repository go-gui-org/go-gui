package gui

import (
	"math"
	"slices"

	"github.com/go-gui-org/go-glyph"
)

// layoutPipeline runs all layout passes in order on a single
// layout tree.
func layoutPipeline(layout *Layout, w *Window) {
	// Width passes.
	layoutWidths(layout)
	layoutFillWidths(layout)
	layoutWrapContainers(layout, w)
	layoutOverflow(layout, w)
	layoutWrapText(layout, w)

	// Height passes.
	layoutHeights(layout)
	layoutFillHeights(layout)
	layoutRotationSwap(layout)

	// Position passes.
	layoutAdjustScrollOffsets(layout, w)
	fx, fy := floatAttachLayout(layout, w.WindowRect())
	layoutPositions(layout, fx, fy, w)
	layoutDisables(layout, false)
	layoutScrollContainers(layout, 0)

	// Post-position passes.
	layoutAmend(layout, w)
	applyLayoutTransition(layout, w)
	applyHeroTransition(layout, w)
	layoutSetShapeClips(layout, w.WindowRect())
}

// layoutAmend walks the layout tree children-first, firing
// AmendLayout callbacks. Not for size changes — post-position
// only.
func layoutAmend(layout *Layout, w *Window) {
	for i := range layout.Children {
		layoutAmend(&layout.Children[i], w)
	}
	if layout.Shape.HasEvents() &&
		layout.Shape.Events.AmendLayout != nil {
		layout.Shape.Events.AmendLayout(layout, w)
	}
}

// layoutHover walks the layout tree depth-first, firing OnHover
// callbacks when the mouse is inside a shape. Returns true if
// any hover was handled.
func layoutHover(layout *Layout, w *Window) bool {
	if w.MouseIsLocked() {
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
	if !shape.HasEvents() || shape.Events.OnHover == nil {
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
	shape.Events.OnHover(layout, &w.scratch.hoverEvent, w)
	return true
}

// layoutMouseLeave walks the entire layout tree, firing OnMouseLeave on any
// shape whose hover state transitioned inside→outside this frame. shape.ID
// must be non-empty; shapes with an empty ID are silently skipped.
func layoutMouseLeave(layout *Layout, w *Window) {
	if w.MouseIsLocked() {
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
	if shape == nil || shape.Disabled || shape.Events == nil ||
		shape.Events.OnMouseLeave == nil || shape.ID == "" {
		return
	}
	sm := StateMap[string, bool](w, nsHoverInside, capModerate)
	inside := shape.PointInShape(w.viewState.mousePosX, w.viewState.mousePosY)
	wasInside, _ := sm.Get(shape.ID)
	if wasInside && !inside {
		w.scratch.hoverEvent = Event{
			MouseX:      w.viewState.mousePosX,
			MouseY:      w.viewState.mousePosY,
			Type:        EventMouseMove,
			MouseButton: MouseInvalid,
		}
		shape.Events.OnMouseLeave(layout, &w.scratch.hoverEvent, w)
	}
	if inside {
		sm.Set(shape.ID, true)
	} else {
		sm.Delete(shape.ID)
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
	switch shape.ShapeType {
	case ShapeRTF:
		if tc.TextMode != TextModeWrap &&
			tc.TextMode != TextModeWrapKeepSpaces {
			return
		}
		if shape.Width <= 0 {
			return
		}
		layoutWrapRTF(shape, tc, w)
	case ShapeText:
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

func layoutWrapRTF(shape *Shape, tc *ShapeTextConfig, w *Window) {
	if tc.RtfRuns == nil {
		return
	}
	if tc.wrapCacheValid &&
		f32AreClose(tc.wrapCacheWidth, shape.Width) &&
		tc.RtfLayout != nil {
		shape.Height = tc.wrapCacheHeight
		return
	}

	// Cross-frame cache: content hash XOR'd with width and
	// base style bits so different styles don't collide. Math
	// cache state mixed in so layout invalidates when an inline
	// math fetch transitions Loading→Ready (different glyph
	// runs: raw LaTeX text vs InlineObject placeholder).
	contentKey := rtfRunsKey(tc.RtfRuns)
	styleKey := rtfStyleKey(tc.RtfBaseStyle)
	mathKey := rtfMathStateKey(tc.RtfRuns, w.viewState.diagramCache)
	cacheKey := contentKey ^ styleKey ^ mathKey ^
		uint64(math.Float32bits(shape.Width))
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
			tc.RtfLayout = &entry.Layout
			shape.Height = entry.Layout.Height
			tc.wrapCacheWidth = shape.Width
			tc.wrapCacheHeight = entry.Layout.Height
			tc.wrapCacheValid = true
			if tc.RtfFlatText == "" {
				tc.RtfFlatText = rtfFlatTextFromRuns(tc.RtfRuns)
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
		vgRT, mh = tc.RtfRuns.toGlyphRichTextWithMath(
			w.viewState.diagramCache)
		tc.rtfMathHashes = mh
	}
	cfg := glyph.TextConfig{
		Style: tc.RtfBaseStyle,
		Block: glyph.BlockStyle{
			Wrap:   glyph.WrapWord,
			Width:  shape.Width,
			Indent: -tc.HangingIndent,
		},
	}
	l, err := tm.LayoutRichText(vgRT, cfg)
	if err != nil {
		return
	}
	rtfSuppressInlineObjectGlyphs(&l)
	tc.RtfLayout = &l
	shape.Height = l.Height
	tc.wrapCacheWidth = shape.Width
	tc.wrapCacheHeight = l.Height
	tc.wrapCacheValid = true
	if tc.RtfFlatText == "" {
		tc.RtfFlatText = rtfFlatTextFromRuns(tc.RtfRuns)
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
	tc *ShapeTextConfig,
	style TextStyle,
	w *Window,
) {
	if w.textMeasurer == nil || tc.TextStyle == nil {
		return
	}
	if len(tc.Text) == 0 {
		return
	}
	l, ok := plainTextLayoutResolved(tc.Text, shape, style, w)
	if !ok {
		return
	}
	shape.Height = l.Height
	if tc.TextMode == TextModeMultiline &&
		shape.Sizing.Width != SizingFixed && l.Width > 0 {
		shape.Width = l.Width
	}
}
