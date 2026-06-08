package gui

import "github.com/go-gui-org/go-glyph"

// renderLayout walks the layout tree and emits RenderCmd entries
// into window.renderers. Clip rectangles bracket clipped children.
func renderLayout(layout *Layout, bgColor Color, clip drawClip, w *Window) {
	// Emit filter bracket when ColorFilter is set (containers only).
	fx := layout.Shape.fx
	hasColorFilter := fx != nil && fx.ColorFilter != nil && !w.inFilter
	if hasColorFilter {
		w.inFilter = true
		emitRenderer(RenderCmd{
			Kind:        RenderFilterBegin,
			BlurRadius:  fx.BlurRadius,
			Layers:      1,
			ColorMatrix: &fx.ColorFilter.Matrix,
		}, w)
	}

	renderShape(layout.Shape, bgColor, clip, w)

	shapeClip := clip
	if layout.Shape.OverDraw {
		shapeClip = layout.Shape.shapeClip
		if layout.Shape.ScrollbarOrientation == ScrollbarVertical {
			shapeClip.Y = clip.Y
			shapeClip.Height = clip.Height
		}
		if layout.Shape.ScrollbarOrientation == ScrollbarHorizontal {
			shapeClip.X = clip.X
			shapeClip.Width = clip.Width
		}
		emitClipCmd(shapeClip, w)
	} else if layout.Shape.Clip {
		sc := layout.Shape.shapeClip
		isRTL := effectiveTextDir(layout.Shape) == TextDirRTL
		var padX float32
		if isRTL {
			padX = layout.Shape.Padding.Right + layout.Shape.SizeBorder
		} else {
			padX = layout.Shape.PaddingLeft()
		}
		shapeClip = drawClip{
			X:      sc.X + padX,
			Y:      sc.Y + layout.Shape.PaddingTop(),
			Width:  f32Max(0, sc.Width-layout.Shape.paddingWidth()),
			Height: f32Max(0, sc.Height-layout.Shape.paddingHeight()),
		}
		emitClipCmd(shapeClip, w)
	}

	// Emit stencil clip bracket before children.
	didIncrement := false
	if layout.Shape.ClipContents {
		if w.stencilDepth < 255 {
			w.stencilDepth++
			didIncrement = true
		}
		emitRenderer(RenderCmd{
			Kind:         RenderStencilBegin,
			X:            layout.Shape.X,
			Y:            layout.Shape.Y,
			W:            layout.Shape.Width,
			H:            layout.Shape.Height,
			Radius:       layout.Shape.Radius,
			StencilDepth: w.stencilDepth,
		}, w)
		// Also apply scissor clip as optimization (avoids
		// rasterizing fragments outside bounding rect).
		if !layout.Shape.Clip && !layout.Shape.OverDraw {
			shapeClip = layout.Shape.shapeClip
			emitClipCmd(shapeClip, w)
		}
	}

	// Propagate rounded clip radius to child images.
	savedClipRadius := w.clipRadius
	w.clipRadius = resolveClipRadius(savedClipRadius, layout.Shape)

	// Emit rotation bracket before children.
	if turns := layout.Shape.QuarterTurns; turns > 0 {
		cx := layout.Shape.X + layout.Shape.Width/2
		cy := layout.Shape.Y + layout.Shape.Height/2
		emitRenderer(RenderCmd{
			Kind:     RenderRotateBegin,
			RotAngle: float32(turns) * 90,
			RotCX:    cx,
			RotCY:    cy,
		}, w)
	}

	color := bgColor
	if layout.Shape.Color != ColorTransparent {
		color = layout.Shape.Color
	}
	for i := range layout.Children {
		renderLayout(&layout.Children[i], color, shapeClip, w)
	}

	if layout.Shape.QuarterTurns > 0 {
		emitRenderer(RenderCmd{Kind: RenderRotateEnd}, w)
	}

	w.clipRadius = savedClipRadius

	if layout.Shape.ClipContents {
		// Restore scissor if we pushed one.
		if !layout.Shape.Clip && !layout.Shape.OverDraw {
			emitClipCmd(clip, w)
		}
		emitRenderer(RenderCmd{
			Kind:         RenderStencilEnd,
			X:            layout.Shape.X,
			Y:            layout.Shape.Y,
			W:            layout.Shape.Width,
			H:            layout.Shape.Height,
			Radius:       layout.Shape.Radius,
			StencilDepth: w.stencilDepth,
		}, w)
		if didIncrement {
			w.stencilDepth--
		}
	}

	if layout.Shape.Clip || layout.Shape.OverDraw {
		emitClipCmd(clip, w)
	}

	if hasColorFilter {
		emitRenderer(RenderCmd{Kind: RenderFilterEnd}, w)
		w.inFilter = false
	}
}

// renderShape dispatches to the type-specific renderer, applying
// opacity when needed.
func renderShape(shape *Shape, parentColor Color, clip drawClip, w *Window) {
	// Degrade safely if a text-like shape is missing text config.
	if (shape.shapeType == shapeText || shape.shapeType == shapeRTF) &&
		shape.TC == nil {
		return
	}

	if shape.Opacity < 1.0 {
		origColor := shape.Color
		origBorder := shape.ColorBorder
		shape.Color = shape.Color.WithOpacity(shape.Opacity)
		shape.ColorBorder = shape.ColorBorder.WithOpacity(shape.Opacity)
		renderShapeInner(shape, parentColor, clip, w)
		shape.Color = origColor
		shape.ColorBorder = origBorder
	} else {
		renderShapeInner(shape, parentColor, clip, w)
	}
}

// renderShapeInner dispatches to the type-specific renderer after
// visibility checks.
func renderShapeInner(shape *Shape, parentColor Color, clip drawClip, w *Window) {
	hasBorder := shape.SizeBorder > 0 && shape.ColorBorder != ColorTransparent
	hasText := shape.shapeType == shapeText && shape.TC != nil
	isImage := shape.shapeType == shapeImage
	isSvg := shape.shapeType == shapeSVG
	isCanvas := shape.shapeType == shapeDrawCanvas
	hasFX := shape.fx != nil && (shape.fx.Gradient != nil ||
		shape.fx.BorderGradient != nil)

	isRTF := shape.shapeType == shapeRTF

	if shape.Color == ColorTransparent && !hasFX && !hasBorder &&
		!hasText && !isImage && !isSvg && !isCanvas && !isRTF {
		return
	}

	switch shape.shapeType {
	case shapeRectangle:
		renderContainer(shape, parentColor, clip, w)
	case shapeText:
		renderText(shape, clip, w)
	case shapeImage:
		renderImage(shape, clip, w)
	case shapeCircle:
		renderCircle(shape, clip, w)
	case shapeRTF:
		renderRtf(shape, clip, w)
	case shapeSVG:
		renderSvg(shape, clip, w)
	case shapeDrawCanvas:
		renderDrawCanvas(shape, clip, w)
	case shapeNone:
		// no-op
	}
}

// renderContainer draws a rectangle (possibly with shadow, gradient,
// blur, or border).
func renderContainer(shape *Shape, _ Color, clip drawClip, w *Window) {
	fx := shape.fx
	hasFX := fx != nil

	// Shadow
	if hasFX && fx.Shadow != nil &&
		fx.Shadow.Color.A > 0 &&
		(fx.Shadow.BlurRadius > 0 || fx.Shadow.OffsetX != 0 || fx.Shadow.OffsetY != 0) {
		emitRenderer(RenderCmd{
			Kind:       RenderShadow,
			X:          shape.X,
			Y:          shape.Y,
			W:          shape.Width,
			H:          shape.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.Shadow.BlurRadius,
			Color:      fx.Shadow.Color,
			OffsetX:    fx.Shadow.OffsetX,
			OffsetY:    fx.Shadow.OffsetY,
		}, w)
	}

	// Custom shader
	if hasFX && fx.Shader != nil {
		emitRenderer(RenderCmd{
			Kind:   RenderCustomShader,
			X:      shape.X,
			Y:      shape.Y,
			W:      shape.Width,
			H:      shape.Height,
			Radius: shape.Radius,
			Color:  shape.Color,
			Shader: fx.Shader,
		}, w)
	} else

	// Gradient fill
	if hasFX && fx.Gradient != nil {
		emitRenderer(RenderCmd{
			Kind:     RenderGradient,
			X:        shape.X,
			Y:        shape.Y,
			W:        shape.Width,
			H:        shape.Height,
			Radius:   shape.Radius,
			Gradient: fx.Gradient,
		}, w)
	} else if hasFX && fx.BlurRadius > 0 && shape.Color.A > 0 &&
		fx.ColorFilter == nil {
		// SDF blur (skipped when ColorFilter is set; FBO blur
		// handles it via the filter bracket pipeline).
		c := shape.Color
		if shape.Disabled {
			c = dimAlpha(c)
		}
		emitRenderer(RenderCmd{
			Kind:       RenderBlur,
			X:          shape.X,
			Y:          shape.Y,
			W:          shape.Width,
			H:          shape.Height,
			Radius:     shape.Radius,
			BlurRadius: fx.BlurRadius,
			Color:      c,
		}, w)
	} else {
		// Border gradient or plain rectangle
		if hasFX && fx.BorderGradient != nil {
			emitRenderer(RenderCmd{
				Kind:      RenderGradientBorder,
				X:         shape.X,
				Y:         shape.Y,
				W:         shape.Width,
				H:         shape.Height,
				Radius:    shape.Radius,
				Thickness: shape.SizeBorder,
				Gradient:  fx.BorderGradient,
			}, w)
		} else {
			renderRectangle(shape, clip, w)
		}
	}
}

// renderRectangle draws a shape as a filled rectangle with optional
// stroke border.
func renderRectangle(shape *Shape, clip drawClip, w *Window) {
	dr := shapeBounds(shape)
	c := shape.Color
	if shape.Disabled {
		c = dimAlpha(c)
	}

	if rectsOverlap(dr, clip) {
		// Fill
		if c.A > 0 {
			emitRenderer(RenderCmd{
				Kind:   RenderRect,
				X:      dr.X,
				Y:      dr.Y,
				W:      dr.Width,
				H:      dr.Height,
				Color:  c,
				Fill:   true,
				Radius: shape.Radius,
			}, w)
		}
		// Border
		if shape.SizeBorder > 0 {
			cb := shape.ColorBorder
			if shape.Disabled {
				cb = dimAlpha(cb)
			}
			if cb.A > 0 {
				emitRenderer(RenderCmd{
					Kind:      RenderStrokeRect,
					X:         dr.X,
					Y:         dr.Y,
					W:         dr.Width,
					H:         dr.Height,
					Color:     cb,
					Radius:    shape.Radius,
					Thickness: shape.SizeBorder,
				}, w)
			}
		}
	}
}

// renderCircle draws a shape as a circle in the middle of the
// shape's rectangular region.
func renderCircle(shape *Shape, clip drawClip, w *Window) {
	dr := shapeBounds(shape)
	c := shape.Color
	if shape.Disabled {
		c = dimAlpha(c)
	}

	if rectsOverlap(dr, clip) {
		radius := f32Min(shape.Width, shape.Height) / 2
		cx := shape.X + shape.Width/2
		cy := shape.Y + shape.Height/2

		if c.A > 0 {
			emitRenderer(RenderCmd{
				Kind:   RenderCircle,
				X:      cx,
				Y:      cy,
				Radius: radius,
				Fill:   true,
				Color:  c,
			}, w)
		}

		// Border
		fx := shape.fx
		if fx != nil && fx.BorderGradient != nil && shape.SizeBorder > 0 {
			emitRenderer(RenderCmd{
				Kind:      RenderGradientBorder,
				X:         dr.X,
				Y:         dr.Y,
				W:         dr.Width,
				H:         dr.Height,
				Radius:    radius,
				Thickness: shape.SizeBorder,
				Gradient:  fx.BorderGradient,
			}, w)
		} else if shape.SizeBorder > 0 {
			cb := shape.ColorBorder
			if shape.Disabled {
				cb = dimAlpha(cb)
			}
			if cb.A > 0 {
				emitRenderer(RenderCmd{
					Kind:      RenderStrokeRect,
					X:         dr.X,
					Y:         dr.Y,
					W:         dr.Width,
					H:         dr.Height,
					Color:     cb,
					Radius:    radius,
					Thickness: shape.SizeBorder,
				}, w)
			}
		}
	}
}

// renderText emits a RenderText command for a text shape.
//
//nolint:gocyclo // text rendering options
func renderText(shape *Shape, clip drawClip, w *Window) {
	tc := shape.TC
	if tc == nil {
		return
	}
	if len(tc.Text) == 0 &&
		(shape.IDFocus <= 0 || shape.IDFocus != w.IDFocus() || !w.IMEComposing()) {
		// Empty text — still render cursor if focused.
		if shape.IDFocus > 0 && shape.IDFocus == w.IDFocus() {
			baseX := shape.X + shape.PaddingLeft()
			baseY := shape.Y + shape.PaddingTop()
			renderInputCursor(shape, "", baseX, baseY,
				glyph.Layout{}, false, w)
		}
		return
	}
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}
	c := tc.TextStyle.Color
	if shape.Opacity < 1.0 {
		c = c.WithOpacity(shape.Opacity)
	}
	if shape.Disabled {
		c = dimAlpha(c)
	}
	if c.A == 0 {
		return
	}

	text := tc.Text
	if tc.TextIsPassword {
		text = maskPassword(tc.Text)
	}

	// Insert IME preedit text at cursor position for display.
	imeComposing := shape.IDFocus > 0 &&
		shape.IDFocus == w.IDFocus() && w.IMEComposing()
	compText := ""
	compRuneLen := 0
	compInsertPos := 0
	if imeComposing {
		compText = w.IMECompText()
		compRunes := []rune(compText)
		compRuneLen = len(compRunes)
		is := StateReadOr(w, nsInput, shape.IDFocus,
			InputState{})
		runes := []rune(text)
		compInsertPos = min(is.CursorPos, len(runes))
		text = string(runes[:compInsertPos]) + compText +
			string(runes[compInsertPos:])
	}

	baseX := shape.X + shape.PaddingLeft()
	baseY := shape.Y + shape.PaddingTop()
	style := textStyleOrDefault(shape)
	renderStyle := style
	renderStyle.Color = c
	if renderStyle.BgColor.A > 0 {
		if shape.Opacity < 1.0 {
			renderStyle.BgColor = renderStyle.BgColor.WithOpacity(shape.Opacity)
		}
		if shape.Disabled {
			renderStyle.BgColor = dimAlpha(renderStyle.BgColor)
		}
	}
	if renderStyle.StrokeColor.A > 0 {
		if shape.Opacity < 1.0 {
			renderStyle.StrokeColor = renderStyle.StrokeColor.WithOpacity(shape.Opacity)
		}
		if shape.Disabled {
			renderStyle.StrokeColor = dimAlpha(renderStyle.StrokeColor)
		}
	}

	var preLayout glyph.Layout
	hasPreLayout := false
	needLayout := tc.TextSelBeg != tc.TextSelEnd ||
		(shape.IDFocus > 0 && shape.IDFocus == w.IDFocus() && w.InputCursorOn()) ||
		imeComposing ||
		spellCheckHasRanges(shape.IDFocus, w)
	renderWithLayout := plainTextNeedsGlyphLayout(shape, tc, renderStyle)
	if needLayout || renderWithLayout {
		preLayout, hasPreLayout = inputGlyphLayoutResolved(text, shape, renderStyle, w, true)
	}

	if renderWithLayout && hasPreLayout {
		cmd := RenderCmd{
			Kind:         RenderLayout,
			X:            baseX,
			Y:            baseY,
			Text:         text,
			LayoutPtr:    w.scratch.renderGlyphLayouts.alloc(preLayout),
			TextStylePtr: w.scratch.renderTextStyles.alloc(renderStyle),
			TextGradient: renderStyle.Gradient,
		}
		if renderStyle.HasTextTransform() {
			transform := renderStyle.EffectiveTextTransform()
			cmd.Kind = RenderLayoutTransformed
			cmd.LayoutTransform = w.scratch.renderAffineTransforms.alloc(transform)
		}
		emitRenderer(cmd, w)
	} else {
		fontAscent := tc.TextStyle.Size * 0.8 // fallback
		var textWidth float32
		if w.textMeasurer != nil {
			fontAscent = w.textMeasurer.FontAscent(*tc.TextStyle)
			textWidth = w.textMeasurer.TextWidth(text, *tc.TextStyle)
		}
		cmd := RenderCmd{
			Kind:         RenderText,
			X:            baseX,
			Y:            baseY,
			Color:        c,
			Text:         text,
			FontName:     tc.TextStyle.Family,
			FontSize:     tc.TextStyle.Size,
			FontAscent:   fontAscent,
			TextWidth:    textWidth,
			TextStylePtr: w.scratch.renderTextStyles.alloc(renderStyle),
			TextGradient: renderStyle.Gradient,
		}
		if tc.TextMode == TextModeWrap ||
			tc.TextMode == TextModeWrapKeepSpaces {
			cmd.W = shape.Width
		}
		emitRenderer(cmd, w)
	}

	// Selection highlight (drawn after text so BgColor does not
	// obscure it; semi-transparent overlay remains readable).
	if !imeComposing {
		renderInputSelection(shape, text, baseX, baseY,
			preLayout, hasPreLayout, w)
	}

	// Cursor (drawn after text).
	if !imeComposing {
		renderInputCursor(shape, text, baseX, baseY,
			preLayout, hasPreLayout, w)
	}

	// IME preedit underline.
	if imeComposing && hasPreLayout {
		renderIMEPreeditUnderline(shape, text, baseX, baseY,
			compInsertPos, compRuneLen,
			w.IMECompCursor(), w.IMECompSelLen(),
			preLayout, style, w)
	}

	// Spell check underlines.
	if !imeComposing && hasPreLayout {
		renderSpellCheckUnderlines(shape, text,
			baseX, baseY, preLayout, w)
	}
}

// renderInputCursor emits a thin rect for the text cursor when
// the shape is focused and the blink state is on. Uses the glyph
// layout engine for precise character-boundary positioning.
func renderInputCursor(shape *Shape, text string, baseX, baseY float32,
	preLayout glyph.Layout, hasPreLayout bool, w *Window) {
	if shape.IDFocus == 0 || shape.IDFocus != w.IDFocus() {
		return
	}
	if !w.InputCursorOn() {
		return
	}
	if shape.TC != nil && shape.TC.TextIsPlaceholder {
		text = ""
	}
	is := StateReadOr(w, nsInput, shape.IDFocus, InputState{})
	runeLen := utf8RuneCount(text)
	pos := min(is.CursorPos, runeLen)

	style := textStyleOrDefault(shape)
	byteIdx := runeToByteIndex(text, pos)
	cursorW := float32(1.5)

	layout := preLayout
	ok := hasPreLayout
	if !ok {
		layout, ok = inputGlyphLayoutResolved(text, shape, style, w, shape.TC != nil && shape.TC.TextIsPassword)
	}
	if ok {
		cp, cpOK := layout.GetCursorPos(byteIdx)
		if !cpOK {
			// End-of-text fallback: use layout dimensions.
			cp.X = layout.VisualWidth
			cp.Y = 0
			cp.Height = fontHeight(style, w)
			if len(layout.Lines) > 0 {
				last := layout.Lines[len(layout.Lines)-1]
				cp.X = last.Rect.X + last.Rect.Width
				cp.Y = last.Rect.Y
				cp.Height = last.Rect.Height
			}
		}
		adjustCursorTrailing(
			&cp, layout.Lines, byteIdx, is.CursorTrailing)
		emitRenderer(RenderCmd{
			Kind:  RenderRect,
			X:     baseX + cp.X,
			Y:     baseY + cp.Y,
			W:     cursorW,
			H:     cp.Height,
			Color: style.Color,
			Fill:  true,
		}, w)
		return
	}

	// Fallback for nil textMeasurer (tests).
	fh := fontHeight(style, w)
	cx := textWidthFallback(text, pos, shape.TC, style, w)
	emitRenderer(RenderCmd{
		Kind:  RenderRect,
		X:     baseX + cx,
		Y:     baseY,
		W:     cursorW,
		H:     fh,
		Color: style.Color,
		Fill:  true,
	}, w)
}

// renderInputSelection emits highlight rectangles for the selected
// text range. Uses glyph layout for precise boundaries.
func renderInputSelection(shape *Shape, text string, baseX, baseY float32,
	preLayout glyph.Layout, hasPreLayout bool, w *Window) {
	tc := shape.TC
	if tc == nil || tc.TextSelBeg == tc.TextSelEnd {
		return
	}
	beg, end := u32Sort(tc.TextSelBeg, tc.TextSelEnd)
	runeLen := utf8RuneCount(text)
	if int(beg) > runeLen || int(end) > runeLen {
		return
	}

	style := textStyleOrDefault(shape)
	selColor := RGBA(51, 153, 255, 100)
	startByte := runeToByteIndex(text, int(beg))
	endByte := runeToByteIndex(text, int(end))

	layout := preLayout
	ok := hasPreLayout
	if !ok {
		layout, ok = inputGlyphLayoutResolved(text, shape, style, w, tc.TextIsPassword)
	}
	if ok {
		rects := layout.GetSelectionRects(startByte, endByte)
		for _, r := range rects {
			emitRenderer(RenderCmd{
				Kind:  RenderRect,
				X:     baseX + r.X,
				Y:     baseY + r.Y,
				W:     r.Width,
				H:     r.Height,
				Color: selColor,
				Fill:  true,
			}, w)
		}
		return
	}

	// Fallback for nil textMeasurer (tests).
	fh := fontHeight(style, w)
	x0 := textWidthFallback(text, int(beg), tc, style, w)
	x1 := textWidthFallback(text, int(end), tc, style, w)
	emitRenderer(RenderCmd{
		Kind:  RenderRect,
		X:     baseX + x0,
		Y:     baseY,
		W:     x1 - x0,
		H:     fh,
		Color: selColor,
		Fill:  true,
	}, w)
}

// renderIMEPreeditUnderline draws underlines beneath the IME
// preedit region: thin for unconverted text, thick for the
// selected clause. Reports the cursor rect to the platform for
// candidate window positioning.
func renderIMEPreeditUnderline(
	shape *Shape, compositeText string,
	baseX, baseY float32,
	insertPos, compRuneLen, compCursor, compSelLen int,
	gl glyph.Layout, style TextStyle, w *Window,
) {
	startByte := runeToByteIndex(compositeText, insertPos)
	endByte := runeToByteIndex(compositeText,
		insertPos+compRuneLen)

	c := style.Color
	if shape.Opacity < 1.0 {
		c = c.WithOpacity(shape.Opacity)
	}

	thinH := max(float32(1), style.Size/14)
	thickH := max(float32(2), style.Size/7)

	// Thin underline for the entire preedit region.
	rects := gl.GetSelectionRects(startByte, endByte)
	for _, r := range rects {
		emitRenderer(RenderCmd{
			Kind:  RenderRect,
			X:     baseX + r.X,
			Y:     baseY + r.Y + r.Height - thinH,
			W:     r.Width,
			H:     thinH,
			Color: c,
			Fill:  true,
		}, w)
	}

	// Thick underline for the selected clause.
	if compSelLen > 0 {
		selStart := insertPos + compCursor
		selEnd := min(selStart+compSelLen, insertPos+compRuneLen)
		sb := runeToByteIndex(compositeText, selStart)
		eb := runeToByteIndex(compositeText, selEnd)
		selRects := gl.GetSelectionRects(sb, eb)
		for _, r := range selRects {
			emitRenderer(RenderCmd{
				Kind:  RenderRect,
				X:     baseX + r.X,
				Y:     baseY + r.Y + r.Height - thickH,
				W:     r.Width,
				H:     thickH,
				Color: c,
				Fill:  true,
			}, w)
		}
	}

	// Report cursor rect to platform for candidate window.
	if len(rects) > 0 {
		r := rects[0]
		w.IMESetRect(baseX+r.X, baseY+r.Y, r.Width, r.Height)
	}
}

// inputGlyphLayout creates a glyph layout for the input text,
// applying password masking and wrap width as needed.
func inputGlyphLayout(text string, shape *Shape, style TextStyle, w *Window) (glyph.Layout, bool) {
	return inputGlyphLayoutResolved(text, shape, style, w, false)
}

func inputGlyphLayoutResolved(text string, shape *Shape, style TextStyle, w *Window, textAlreadyMasked bool) (glyph.Layout, bool) {
	if w.textMeasurer == nil {
		return glyph.Layout{}, false
	}
	displayText := text
	if shape.TC != nil && shape.TC.TextIsPassword && !textAlreadyMasked {
		displayText = maskPassword(text)
	}
	return plainTextLayoutResolved(displayText, shape, style, w)
}

// textStyleOrDefault returns the TextStyle from shape.TC or a
// fallback default.
func textStyleOrDefault(shape *Shape) TextStyle {
	if shape.TC != nil && shape.TC.TextStyle != nil {
		return *shape.TC.TextStyle
	}
	return DefaultTextStyle
}

// fontHeight returns the font height, or a fallback.
func fontHeight(style TextStyle, w *Window) float32 {
	if w.textMeasurer != nil {
		return w.textMeasurer.FontHeight(style)
	}
	if style.Size > 0 {
		return style.Size * 1.2
	}
	return 16
}

// textWidthFallback approximates text width for tests (no glyph).
func textWidthFallback(text string, runePos int, tc *ShapeTextConfig, style TextStyle, w *Window) float32 {
	runeLen := utf8RuneCount(text)
	runePos = min(runePos, runeLen)
	prefix := text[:runeToByteIndex(text, runePos)]
	if tc != nil && tc.TextIsPassword {
		prefix = passwordMask(prefix)
	}
	if w.textMeasurer != nil {
		return w.textMeasurer.TextWidth(prefix, style)
	}
	sz := style.Size
	if sz <= 0 {
		sz = 14
	}
	return float32(utf8RuneCount(prefix)) * sz * 0.6
}

// renderRtf emits a RenderRTF command for pre-shaped rich text.
func renderRtf(shape *Shape, clip drawClip, w *Window) {
	if !shape.hasRtfLayout() {
		return
	}
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}
	baseX := shape.X + shape.PaddingLeft()
	baseY := shape.Y + shape.PaddingTop()
	emitRenderer(RenderCmd{
		Kind:      RenderRTF,
		X:         baseX,
		Y:         baseY,
		LayoutPtr: shape.TC.RtfLayout,
	}, w)

	if shape.TC.RtfFlatText != "" {
		renderInputSelection(shape, shape.TC.RtfFlatText,
			baseX, baseY, *shape.TC.RtfLayout, true, w)
	}

	// Emit RenderImage for inline math objects.
	// Use rtfMathHashes (populated by toGlyphRichTextWithMath)
	// instead of item.ObjectID, which is unreliable due to
	// Pango's shape attribute data pointer lacking null
	// termination on round-trip through C.GoString.
	cache := w.viewState.diagramCache
	hashes := shape.TC.rtfMathHashes
	if cache == nil || len(hashes) == 0 {
		return
	}
	objIdx := 0
	for i := range shape.TC.RtfLayout.Items {
		item := &shape.TC.RtfLayout.Items[i]
		if !item.IsObject {
			continue
		}
		if objIdx >= len(hashes) {
			objIdx++
			continue
		}
		hash := hashes[objIdx]
		objIdx++
		entry, ok := cache.Get(hash)
		if !ok || entry.State != DiagramReady {
			continue
		}
		// Prefer the InlineObject's own height/offset so tall math
		// (fractions, integrals) keeps its true aspect ratio. Fall
		// back to line-height when Object is missing (legacy path).
		h := float32(item.Ascent + item.Descent)
		y := float32(item.Y - item.Ascent)
		if obj := item.Style.Object; obj != nil && obj.Height > 0 {
			h = obj.Height
			// Offset positions image bottom relative to baseline
			// (matches Pango logicalRect: y = -h - offset).
			y = float32(item.Y) - obj.Height - obj.Offset
		}
		emitRenderer(RenderCmd{
			Kind:     RenderImage,
			X:        baseX + float32(item.X),
			Y:        baseY + y,
			W:        float32(item.Width),
			H:        h,
			Resource: entry.PNGPath,
		}, w)
	}
}
