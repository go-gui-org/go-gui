package gui

import (
	"strings"

	"github.com/go-gui-org/go-glyph"
)

// Text rendering functions. Extracted from render_layout.go to keep
// both files under 800 lines.

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
		var sb strings.Builder
		sb.Grow(len(text) + len(compText))
		sb.WriteString(string(runes[:compInsertPos]))
		sb.WriteString(compText)
		sb.WriteString(string(runes[compInsertPos:]))
		text = sb.String()
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
		(shape.IDFocus > 0 && shape.IDFocus == w.IDFocus() && w.inputCursorOn()) ||
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
	if !w.inputCursorOn() {
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
