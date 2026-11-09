package gui

import "strings"

// Group-box titles (#1238): ContainerCfg.Title draws a label on the
// container's top border, like an HTML fieldset legend. The label is
// not a child. It hangs off Shape.fx.groupTitle and the container draws
// it itself, after its children and in its parent's clip, so it sits in
// the main layer under every popup and is clipped like its box. As two
// Float children it was drawn over any popup earlier in the tree and
// escaped every clip. See docs/specs/group-box-title.md.

// A group-box title starts groupTitleInset from the box's leading edge
// (the right edge in RTL), and its eraser keeps groupTitlePad clear on
// each side of the text.
const (
	groupTitleInset float32 = 20
	groupTitlePad   float32 = 5
)

// addGroupBoxTitle gives a titled container its label and reserves the
// room it takes. The label's first line is centred on the top border.
// The half above the border is a band at the top of the box: the border,
// fill and shadow are drawn that much lower (containerBody). The rest of
// the label lies where the top padding is, and a top padding smaller
// than that grows to it, so neither the label nor its eraser covers the
// first child. Both amounts follow the label's measured height, so a
// larger title font, or a title with more lines, reserves more. The
// label is then wholly inside the box, where a scroll viewport, a
// clipping parent or a RotatedBox keeps it whole.
func addGroupBoxTitle(title string, titleBG, colorBorder Color,
	w *Window, shape *Shape) {
	if len(title) == 0 {
		return
	}
	// A group-box title is a heading role and takes a title role
	// (visual-refresh §2.2); the border color below is the only
	// restyle.
	ts := guiTheme.TextStyleTitleSmall

	// Every renderer breaks a line at a newline, so a title with one is
	// measured whole; the second measurement is paid only then.
	var textWidth, lineHeight, textHeight float32
	if w != nil && w.textMeasurer != nil {
		textWidth = w.textMeasurer.TextWidth(title, ts)
		lineHeight = w.textMeasurer.FontHeight(ts)
		textHeight = lineHeight
		if strings.IndexByte(title, '\n') >= 0 {
			textHeight = max(lineHeight, w.textMeasurer.TextHeight(title, ts))
		}
	} else {
		// Fallback for tests without a text measurer.
		textWidth = float32(len(title)) * 8
		lineHeight = 16
		textHeight = lineHeight * float32(strings.Count(title, "\n")+1)
	}
	// Undimmed: renderGroupBoxTitle stamps the container's Disabled,
	// inherited or its own, and renderText dims once (issue #341).
	ts.Color = colorBorder
	// Position, opacity and Disabled are the container's, copied at
	// render, so a render-only frame follows a scroll or a fade.
	text := w.allocShape(Shape{
		shapeType: shapeText,
		Width:     textWidth,
		Height:    textHeight,
		Color:     colorBorder,
		Opacity:   1.0,
		// Both from the view-phase arena: a literal here and &ts each
		// moved to the heap, two allocations per titled container per
		// frame.
		TC: w.allocTextConfig(ShapeTextConfig{
			Text:      title,
			TextStyle: w.allocTextStyle(ts),
		}),
	})
	if shape.fx == nil {
		shape.fx = w.allocEffects(shapeEffects{})
	}
	shape.fx.groupTitle = text
	shape.fx.titleBG = titleBG
	half := lineHeight / 2
	shape.fx.titleBand = half
	sb := shape.SizeBorder
	if !f32IsFinite(sb) || sb < 0 {
		sb = 0
	}
	shape.Padding.Top = half + max(shape.Padding.Top, textHeight-half-sb)
}

// groupTitleOf returns the title a container draws, or nil. A container
// an overflowing row hid (hideOverflowChild makes it a shapeNone) draws
// none, nor does a title whose ink is invisible: a ColorBorder of
// ColorTransparent beside a BorderGradient leaves nothing to show, and
// an eraser would only cut a gap in the border.
func groupTitleOf(shape *Shape) *Shape {
	fx := shape.fx
	if fx == nil || fx.groupTitle == nil || shape.shapeType == shapeNone {
		return nil
	}
	if fx.groupTitle.TC.TextStyle.Color.A == 0 {
		return nil
	}
	return fx.groupTitle
}

// groupTitleBand returns the height of the band a group-box title
// reserves above the border: half the title's first line, capped at the
// box's height. Zero for a container with no title.
func groupTitleBand(shape *Shape) float32 {
	fx := shape.fx
	if fx == nil || fx.groupTitle == nil {
		return 0
	}
	return max(0, min(fx.titleBand, shape.Height))
}

// groupTitleLead returns how far a title's eraser starts from the box's
// leading edge: groupTitleInset, or a larger rounded corner's radius, so
// the title and its eraser sit on the straight part of the top border.
// Over the curve an eraser in the box's fill would square off the
// corner, and the border would run through the title.
func groupTitleLead(s *Shape) float32 {
	if s.shapeType != shapeRectangle || !(s.Radius > groupTitleInset) {
		return groupTitleInset
	}
	body := containerBody(s)
	// The renderers clamp a radius to half the shorter side.
	return max(groupTitleInset, min(s.Radius, body.Width/2, body.Height/2))
}

// containerBody returns the rectangle a container paints its fill,
// shadow and border over, and its rounded stencil follows: its bounds,
// less the band a group-box title reserves above the border.
func containerBody(shape *Shape) drawClip {
	b := shapeBounds(shape)
	band := groupTitleBand(shape)
	b.Y += band
	b.Height -= band
	return b
}

// renderGroupBoxTitle draws a titled container's label over an eraser
// that hides the border line behind it. clip is the parent's: the label
// is inside the box's bounds but outside its content box, so the box's
// own clip would cut it. RTL mirrors the label to the right edge with
// the same inset.
func renderGroupBoxTitle(layout *Layout, clip drawClip, w *Window) {
	s := layout.Shape
	text := s.fx.groupTitle
	eraserW := text.Width + 2*groupTitlePad - 1
	lead := groupTitleLead(s)
	eraserX := s.X + lead
	if effectiveTextDir(s) == TextDirRTL {
		eraserX = s.X + s.Width - lead - eraserW
	}
	// Assigned, not accumulated: a render-only frame draws the same
	// shape again.
	text.X = eraserX + groupTitlePad
	text.Y = s.Y
	text.TextDir = s.TextDir
	text.Opacity = s.Opacity
	text.Disabled = s.Disabled
	if r, c, ok := groupTitleEraser(layout, eraserX, eraserW, w); ok &&
		rectsOverlap(r, clip) {
		emitRenderer(RenderCmd{
			Kind:  RenderRect,
			X:     r.X,
			Y:     r.Y,
			W:     r.Width,
			H:     r.Height,
			Color: c,
			Fill:  true,
		}, w)
	}
	renderText(text, clip, w)
}

// groupTitleEraser returns the patch that hides the border line behind
// a group-box title, its color, and false when none is drawn. x and
// width place it across the label.
//
// An explicit TitleBG is a patch behind the whole label, painted as
// given, never dimmed: it stands for the ground behind the box, which a
// disabled box does not dim. ColorTransparent turns it off.
//
// Unset, the patch is drawn only where there is a border to hide, over
// the label below the border: the border lies at the top of the box's
// body (containerBody), and the band above shows what is behind the box
// anyway. It starts half a border width higher, because the web and
// PDF backends centre a stroke on its path, but never above the box,
// which a border thicker than the label would reach; and it ends at the
// body's bottom in a box shorter than the label. Its color is what is
// drawn there (backdropColor), when that is one opaque color; where
// none is, the border shows through as it did before #1238.
func groupTitleEraser(layout *Layout, x, width float32, w *Window) (drawClip, Color, bool) {
	s := layout.Shape
	text := s.fx.groupTitle
	r := drawClip{X: x, Y: text.Y, Width: width, Height: text.Height}
	if bg := s.fx.titleBG; bg.IsSet() {
		return r, bg, bg.A > 0
	}
	if !(s.SizeBorder > 0) ||
		(s.ColorBorder.A == 0 && s.fx.BorderGradient == nil) {
		return r, Color{}, false
	}
	body := containerBody(s)
	top := max(body.Y-s.SizeBorder/2, s.Y)
	bottom := min(text.Y+text.Height, body.Y+body.Height)
	r.Y, r.Height = top, bottom-top
	c, ok := backdropColor(layout, w)
	if !ok || c.A < 255 || !(r.Height > 0) {
		return r, Color{}, false
	}
	return r, c, true
}

// backdropColor composites the fills drawn under the body of layout,
// its own first and then its ancestors', as renderContainer paints them
// (opacity, the disabled dim, a hover fill already applied in arrange),
// down to the first opaque one or else the window's clear color.
//
// It reports false when no one color matches what is drawn there: a
// gradient or shader fill; the box's own blurred fill, whose soft top
// edge lies under the patch; an ancestor that is not a container (an
// image, a canvas); whatever shows through a float that is not opaque;
// and fills outside a ColorFilter bracket the patch is drawn inside,
// which the filter changes for the patch but not for them. Siblings
// drawn underneath, as in a Stack, are not seen: the patch takes the
// container's color over them, and TitleBG names the right one.
func backdropColor(layout *Layout, w *Window) (Color, bool) {
	acc := Color{}
	depth := 0
	for p := layout; p != nil; p = p.Parent {
		if overMaxDepth(depth) {
			return Color{}, false
		}
		depth++
		s := p.Shape
		if s == nil {
			continue
		}
		switch s.shapeType {
		case shapeRectangle:
		case shapeNone, shapeText, shapeRTF:
			continue
		default:
			return Color{}, false
		}
		fx := s.fx
		if fx != nil && (fx.Gradient != nil || fx.Shader != nil) {
			return Color{}, false
		}
		if p == layout && fx != nil && fx.BlurRadius > 0 &&
			fx.ColorFilter == nil && s.Color.A > 0 {
			return Color{}, false
		}
		fill := s.Color
		if s.Opacity < 1 {
			fill = fill.WithOpacity(s.Opacity)
		}
		if fillDims(s) {
			fill = dimAlpha(fill)
		}
		acc = acc.Over(fill)
		if acc.A == 255 {
			return acc, true
		}
		// The patch is drawn inside this shape's filter bracket and its
		// own fill too; the fills further out are not.
		if fx != nil && fx.ColorFilter != nil {
			return Color{}, false
		}
		// A lifted float keeps its Parent pointer into the tree it
		// hangs from (layoutRemoveFloatingLayouts), but it is drawn in
		// a layer of its own, over whatever lies at its position. Past
		// it the walk would blend the anchor's ancestors instead.
		if s.Float {
			return Color{}, false
		}
	}
	return acc.Over(w.FrameBackground()), true
}
