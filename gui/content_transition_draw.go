package gui

import "github.com/go-gui-org/go-glyph"

// The draws of one content transition frame: the fade, the numeric roll
// and the Replace scale, and the commands they emit. Split from
// content_transition.go to keep that file under the 800-line gate
// (tools/largefiles).

// contentTransitionFrame is what the draws of one transition frame
// share. It lives on the stack of renderContentTransition.
type contentTransitionFrame struct {
	shape *Shape
	run   *contentTransitionRun
	w     *Window
	style TextStyle
	// p is the eased progress, 0 to 1.
	p float32
	// baseX, baseY, boxW and boxH are the current content box.
	baseX, baseY, boxW, boxH float32
	// c is the paint color with the shape's opacity and disabled state
	// applied. gradDim tells if a gradient fill is dimmed for the
	// disabled state.
	c       Color
	gradDim bool
}

// gradient is the style's gradient fill at alpha times the shape's
// opacity, as renderText dims it, or nil when the style has none.
func (f *contentTransitionFrame) gradient(alpha float32) *glyph.GradientConfig {
	return dimmedTextGradient(f.style.Gradient, f.shape.Opacity*alpha,
		f.gradDim, &f.w.scratch)
}

// oldContentW is the content width of the old box.
func (f *contentTransitionFrame) oldContentW() float32 {
	return f.run.fromShapeW - f.shape.paddingWidth()
}

// fade draws the old text at 1-p of its alpha and the new text at p,
// each the way renderText draws a plain text: through a glyph layout
// where renderText uses one, so that each line wraps and aligns as it
// will when the fade ends, and as a plain text draw elsewhere. The old
// text is drawn in the box it was drawn in.
func (f *contentTransitionFrame) fade(clip drawClip) {
	run := f.run
	tc := f.shape.TC
	// The test reads the new text; an old one past one chunk takes the
	// layouts too, or a single-line plain text faded to a short value
	// would hand the backend all of the old one as one string.
	if f.w.textMeasurer != nil &&
		(plainTextNeedsGlyphLayout(f.shape, tc, f.style) ||
			len(run.from) > textChunkBytes) && f.fadeLayouts(clip) {
		return
	}
	// The plain path draws at the box's left edge, and wraps at the
	// shape's width.
	var fromWrap, toWrap float32
	if tc.TextMode == TextModeWrap || tc.TextMode == TextModeWrapKeepSpaces {
		fromWrap, toWrap = run.fromShapeW, f.shape.Width
	}
	f.emitText(run.from, f.baseX+run.fromDX, f.baseY, fromWrap, 1-f.p)
	f.emitText(run.to, f.baseX, f.baseY, toWrap, f.p)
}

// fadeLayouts draws a fade from glyph layouts. The new text uses the
// shape's own cached layout. The old text is shaped once for the run,
// with the width argument the plain path gave it in the old box. It
// reports false when there is no block, or when the shaper refuses the
// old text of a single-layout fade; a refused chunk of a block draws
// nothing and the fade still reports true.
func (f *contentTransitionFrame) fadeLayouts(clip drawClip) bool {
	run := f.run
	toBlk, ok := inputGlyphLayoutResolved(run.to, f.shape, f.style, f.w, true)
	if !ok {
		return false
	}
	// A text past one chunk has no single layout to fade, and the plain
	// path would hand the backend all of it as one string, which the
	// shaper refuses past its budget (#1190): fade chunk by chunk.
	toLayout, ok := toBlk.single()
	if !ok || len(run.from) > textChunkBytes {
		f.fadeBlocks(toBlk, clip)
		return true
	}
	if run.fromLayout == nil {
		l, err := f.w.textMeasurer.LayoutText(run.from, f.style,
			f.oldWidthArg())
		if err != nil {
			return false
		}
		run.fromLayout = &l
	}
	f.emitLayout(*run.fromLayout, run.from, f.baseX+run.fromDX, 1-f.p)
	f.emitLayout(toLayout, run.to, f.baseX, f.p)
	return true
}

// oldWidthArg is the width argument the plain path gave the old text in
// the old box. A copy of the shape, for its width only: the argument
// depends on the mode, the alignment and the width.
func (f *contentTransitionFrame) oldWidthArg() float32 {
	old := *f.shape
	old.Width = f.run.fromShapeW
	return plainTextLayoutWidthArg(&old, f.shape.TC, f.style)
}

// fadeBlocks fades texts that do not both fit one chunk: each is drawn
// as renderTextBlock draws a text, chunk by chunk within clip, at its
// alpha. The new text uses the shape's own block; the old one gets a
// block for the run, laid out as the fade lays it out in the old box.
// Like emitLayout, it turns through the style's transform and not the
// animation's motion, which waits until the transition ends.
func (f *contentTransitionFrame) fadeBlocks(toBlk *textBlock, clip drawClip) {
	run := f.run
	if run.fromBlock == nil {
		run.fromBlock = newTextBlock(run.from, f.oldWidthArg(), f.style,
			f.shape.TC.TextMode, f.w)
	}
	f.emitBlock(run.fromBlock, f.baseX+run.fromDX, 1-f.p, clip)
	f.emitBlock(toBlk, f.baseX, f.p, clip)
}

// emitBlock draws blk's chunks in view at (x, baseY) at alpha.
func (f *contentTransitionFrame) emitBlock(
	blk *textBlock, x, alpha float32, clip drawClip,
) {
	c := f.c.WithOpacity(alpha)
	if c.A == 0 {
		return
	}
	style := f.style
	style.Color = c
	renderTextBlock(f.shape, x, f.baseY, blk, style, f.gradient(alpha),
		false, style.effectiveTextTransform(), style.hasTextTransform(),
		clip, f.w)
}

// roll draws a numeric roll. A column that did not change is drawn in
// place. In a column that changed, the old grapheme leaves by one line
// height and the new one arrives from the other side, each fading as it
// goes. The content box clips both, so a digit is never seen outside
// the line. The columns follow the new text: the old digits roll out
// of them, wherever the old box was.
func (f *contentTransitionFrame) roll(clip drawClip) {
	run := f.run
	box := drawClip{X: f.baseX, Y: f.baseY, Width: f.boxW, Height: f.boxH}
	// No overlap clips the text away entirely, which is the safe
	// direction: a dropped RenderClip would leave the parent's wider
	// scissor in force.
	inner, _ := rectIntersection(box, clip)
	emitClipCmd(inner, f.w)

	// Up: the old digit rises out of the top and the new one rises in
	// from below. Down mirrors it.
	sign := float32(1)
	if run.down {
		sign = -1
	}
	fromDY := -sign * f.p * f.boxH
	toDY := sign * (1 - f.p) * f.boxH

	x0 := f.baseX + contentTransitionAlignX(f.style.Align, f.boxW, run.toW)
	for i := range run.cols {
		col := &run.cols[i]
		x := x0 + col.x
		if !col.changed {
			f.emitText(run.to[col.toStart:col.toEnd], x, f.baseY, 0, 1)
			continue
		}
		if col.fromStart >= 0 {
			f.emitText(run.from[col.fromStart:col.fromEnd],
				x, f.baseY+fromDY, 0, 1-f.p)
		}
		if col.toStart >= 0 {
			f.emitText(run.to[col.toStart:col.toEnd],
				x, f.baseY+toDY, 0, f.p)
		}
	}
	emitClipCmd(clip, f.w)
}

// scale draws a Replace: the old text shrinks to nothing about its
// center while the new one grows from nothing. It needs glyph layouts
// to carry the transform, so it reports false without a text measurer
// or when the shaper refuses a text, and the caller fades instead.
//
// Both layouts are shaped with no width, so each is placed by
// contentTransitionAlignX in its own box and scales about its own
// center: the old one in the old box, the new one in the current box.
func (f *contentTransitionFrame) scale() bool {
	run := f.run
	if f.w.textMeasurer == nil {
		return false
	}
	if run.toLayout == nil {
		l, err := f.w.textMeasurer.LayoutText(run.to, f.style, 0)
		if err != nil {
			return false
		}
		run.toLayout = &l
	}
	if run.fromLayout == nil {
		l, err := f.w.textMeasurer.LayoutText(run.from, f.style, 0)
		if err != nil {
			return false
		}
		run.fromLayout = &l
	}
	fromX := f.baseX + run.fromDX +
		contentTransitionAlignX(f.style.Align, f.oldContentW(), run.fromW)
	toX := f.baseX + contentTransitionAlignX(f.style.Align, f.boxW, run.toW)
	f.emitScaled(*run.fromLayout, run.from, fromX, 1-f.p, run.fromW)
	f.emitScaled(*run.toLayout, run.to, toX, f.p, run.toW)
	return true
}

// emitLayout emits one glyph-layout draw at (x, baseY) at alpha, as
// emitTextLayoutCmd draws a plain text: with the style's gradient and,
// when the style has one, its transform.
func (f *contentTransitionFrame) emitLayout(
	l glyph.Layout, text string, x, alpha float32,
) {
	c := f.c.WithOpacity(alpha)
	if c.A == 0 {
		return
	}
	style := f.style
	style.Color = c
	l.Items = recolorLayoutItems(l.Items, style, f.w)
	cmd := RenderCmd{
		Kind:         RenderLayout,
		X:            x,
		Y:            f.baseY,
		Text:         text,
		LayoutPtr:    f.w.scratch.renderGlyphLayouts.alloc(l),
		TextStylePtr: f.w.scratch.renderTextStyles.alloc(style),
		TextGradient: f.gradient(alpha),
	}
	if style.hasTextTransform() {
		cmd.Kind = RenderLayoutTransformed
		cmd.LayoutTransform = f.w.scratch.renderAffineTransforms.alloc(
			style.effectiveTextTransform())
	}
	emitRenderer(cmd, f.w)
}

// emitScaled emits one layout draw at (x, baseY) scaled by s about the
// center of a textW by boxH box, at alpha s.
func (f *contentTransitionFrame) emitScaled(
	l glyph.Layout, text string, x, s, textW float32,
) {
	c := f.c.WithOpacity(s)
	if c.A == 0 || s <= 0 {
		return
	}
	style := f.style
	style.Color = c
	l.Items = recolorLayoutItems(l.Items, style, f.w)
	xf := textAnimTransform(s, 0, 0, 0, textW/2, f.boxH/2)
	emitRenderer(RenderCmd{
		Kind:            RenderLayoutTransformed,
		X:               x,
		Y:               f.baseY,
		Text:            text,
		LayoutPtr:       f.w.scratch.renderGlyphLayouts.alloc(l),
		TextStylePtr:    f.w.scratch.renderTextStyles.alloc(style),
		TextGradient:    f.gradient(s),
		LayoutTransform: f.w.scratch.renderAffineTransforms.alloc(xf),
	}, f.w)
}

// emitText emits one plain text draw at (x, y) at alpha, wrapped at
// wrapW when it is above 0. A fully transparent one is skipped, as
// renderText skips its own.
func (f *contentTransitionFrame) emitText(
	text string, x, y, wrapW, alpha float32,
) {
	c := f.c.WithOpacity(alpha)
	if c.A == 0 || text == "" {
		return
	}
	style := f.style
	style.Color = c
	fontAscent := style.Size * 0.8 // fallback, as in renderText
	var textWidth float32
	if f.w.textMeasurer != nil {
		fontAscent = f.w.textMeasurer.FontAscent(style)
		textWidth = f.w.textMeasurer.TextWidth(text, style)
	}
	emitRenderer(RenderCmd{
		Kind:         RenderText,
		X:            x,
		Y:            y,
		Color:        c,
		Text:         text,
		FontName:     style.Family,
		FontSize:     style.Size,
		FontAscent:   fontAscent,
		TextWidth:    textWidth,
		W:            wrapW,
		TextStylePtr: f.w.scratch.renderTextStyles.alloc(style),
		TextGradient: f.gradient(alpha),
	}, f.w)
}
