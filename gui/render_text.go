package gui

import "github.com/go-gui-org/go-glyph"

// Text rendering functions. Extracted from render_layout.go to keep
// both files under 800 lines.

// textShapeFocused reports whether the widget this text shape renders
// for currently holds focus. A standalone Text answers for itself via
// Focusable+ID; an input's inner text shape answers for the container
// named by focusOwner, which owns both the focus and the state.
func textShapeFocused(shape *Shape, w *Window) bool {
	if !shape.rendersFocusState() {
		return false
	}
	key := shape.focusKey()
	return key != "" && w.IsFocus(key)
}

// renderText emits a RenderText command for a text shape.
//
//nolint:gocyclo // text rendering options
func renderText(shape *Shape, clip drawClip, w *Window) {
	tc := shape.TC
	if tc == nil {
		return
	}
	// Resolved once: every text shape in the frame asks, and each ask
	// is an atomic load, an interface assertion and a string compare.
	// renderInputCursor and renderInputSelection re-derive it for
	// themselves because tests call them directly.
	focused := textShapeFocused(shape, w)
	if len(tc.Text) == 0 &&
		(!focused || !w.IMEComposing()) {
		// Empty text — still render cursor if focused.
		if focused {
			baseX := shape.X + shape.PaddingLeft()
			baseY := shape.Y + shape.PaddingTop()
			renderInputCursor(shape, "", baseX, baseY, nil, false, w)
		}
		return
	}
	// Cull against the ink, not just the box. A run that could not wrap
	// paints past its shape's right edge (see Shape.inkOverflowW), and
	// once the field is scrolled the box can sit entirely left of the
	// clip while the visible part of the text does not.
	bounds := textRenderBounds(shape, tc)
	if !rectsOverlap(bounds, clip) {
		return
	}
	c := tc.TextStyle.Color
	if shape.Opacity < 1.0 {
		c = c.WithOpacity(shape.Opacity)
	}
	// A style from the disabled role has already been quieted by the
	// theme; quieting it again is the double-dim of issue #335. Any
	// other text shape stamped Disabled takes the theme's disabled
	// amount (issue #341): dimAlpha halved the caller's color, which
	// matches the dark theme by construction and sits 22 alpha under
	// the light theme's contrast-matched role.
	if shape.Disabled && !tc.TextStyle.disabledRole {
		c = w.Theme().disabledTextColor(c)
	}
	if c.A == 0 {
		return
	}

	text := tc.Text
	if tc.textIsPassword {
		text = maskPassword(tc.Text)
	}

	// Insert IME preedit text at cursor position for display.
	// A read-only field stays Focusable (to keep selection and
	// cursor) but can never commit a composition, so its preedit
	// must not render — makeInputOnChar would swallow the commit.
	imeComposing := !tc.textReadOnly &&
		focused && w.IMEComposing()
	compText := ""
	compInsertPos := 0
	if imeComposing {
		compText = w.IMECompText()
		is := StateReadOr(w, nsInput, shape.focusKey(),
			inputState{}).in(tc.Text)
		base := text
		if tc.textIsPlaceholder {
			// The placeholder is a hint, not content: a composition
			// replaces it outright. Inserting into it would push the
			// hint out to the right of the preedit.
			base = ""
		}
		// The caret is an offset of the field's text; the preedit is
		// spliced into the text as drawn. Clamped both ways: the slices
		// below panic on a cursor outside the text, and the preedit is
		// drawn every frame.
		compInsertPos = displayOffset(tc.Text, base,
			min(max(is.CursorPos, 0), len(tc.Text)))
		if tc.textIsPlaceholder {
			compInsertPos = 0
		}
		text = base[:compInsertPos] + compText + base[compInsertPos:]
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

	// A content transition draws the old and the new text itself, from
	// the progress its driver wrote into the run. Render-only frames
	// reach here with the shape of the last layout, so this is where the
	// text moves.
	// One condition here: renderText sits at the cyclomatic limit, so
	// the nil, focus and selection tests live in
	// renderContentTransition.
	if renderContentTransition(shape, tc.ctrans, clip,
		c, renderStyle, w) {
		return
	}

	var blk *textBlock
	hasBlk := false
	needLayout := tc.textSelBeg != tc.textSelEnd ||
		(focused && w.inputCursorOn()) ||
		imeComposing ||
		spellCheckHasRanges(shape.focusKey(), w)
	renderWithLayout := plainTextNeedsGlyphLayout(shape, tc, renderStyle)
	if needLayout || renderWithLayout {
		blk, hasBlk = inputGlyphLayoutResolved(text, shape, renderStyle, w, true)
	}
	text = textRenderDisplay(text, shape, blk, hasBlk)

	// A gradient fill carries no color of its own, so it needs the
	// same treatment c got above — except under the disabled role,
	// whose theme style already expresses the state.
	gradDim := shape.Disabled && !tc.TextStyle.disabledRole
	gradient := renderStyle.Gradient
	// A shimmer is built here, from the colour the text ended up
	// with, not in the view pass: a filled button restamps its label's
	// colour after arrange, and a gradient built earlier kept the old
	// one. Undimmed base: dimmedTextGradient applies the opacity and
	// the disabled state to the stops, as for any gradient.
	if g := tc.anim.gradient(tc.TextStyle.Color, w); g != nil {
		gradient = g
	}
	textGradient := dimmedTextGradient(gradient, shape.Opacity, gradDim,
		&w.scratch)

	// A typewriter paints only its revealed prefix. The layout is of
	// the full text, so the painted part sits exactly where it will
	// once all of it is shown.
	reveal := tc.anim != nil && tc.anim.revealOn && !tc.textIsPassword &&
		!imeComposing
	if renderWithLayout && hasBlk {
		// Drawn chunk by chunk from the block the sizing pass shaped,
		// so a text past the shaper's budget still paints (#1190).
		xf, xfOn := textLayoutTransform(shape, renderStyle)
		renderTextBlock(shape, baseX, baseY, blk, renderStyle,
			textGradient, reveal, xf, xfOn, clip, w)
	} else {
		// No glyph layout (no measurer, or the shaper refused the
		// text): the reveal falls back to painting the prefix. Only
		// the painted string is cut; the caret and selection below
		// still index the full text.
		paintText := text
		if reveal && tc.anim.revealEnd <= len(text) {
			paintText = text[:tc.anim.revealEnd]
		}
		fontAscent := tc.TextStyle.Size * 0.8 // fallback
		var textWidth float32
		if w.textMeasurer != nil {
			fontAscent = w.textMeasurer.FontAscent(*tc.TextStyle)
			textWidth = w.textMeasurer.TextWidth(paintText, *tc.TextStyle)
		}
		cmd := RenderCmd{
			Kind:         RenderText,
			X:            baseX,
			Y:            baseY,
			Color:        c,
			Text:         paintText,
			FontName:     tc.TextStyle.Family,
			FontSize:     tc.TextStyle.Size,
			FontAscent:   fontAscent,
			TextWidth:    textWidth,
			TextStylePtr: w.scratch.renderTextStyles.alloc(renderStyle),
			TextGradient: textGradient,
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
		renderInputSelection(shape, text, baseX, baseY, blk, hasBlk, clip, w)
	}

	// Cursor (drawn after text).
	if !imeComposing {
		renderInputCursor(shape, text, baseX, baseY, blk, hasBlk, w)
	}

	// IME preedit underline.
	if imeComposing && hasBlk {
		renderIMEPreeditUnderline(shape, baseX, baseY,
			compInsertPos, compText,
			w.IMECompCursor(), w.IMECompSelLen(),
			blk, style, clip, w)
	}

	// Spell check underlines.
	if !imeComposing && hasBlk {
		renderSpellCheckUnderlines(shape, text, baseX, baseY, blk, clip, w)
	}
}

// textRenderBounds includes ink that can reach past a text shape's arranged
// box. A Fill-width single-line input keeps that box at the viewport width
// and records the full run as MinWidth. Once caret following shifts it left,
// later chunks still reach into the clip and need the chunk-level cull.
func textRenderBounds(shape *Shape, tc *ShapeTextConfig) drawClip {
	bounds := shapeBounds(shape)
	if tc.TextMode == TextModeSingleLine &&
		f32IsFinite(shape.MinWidth) && shape.MinWidth > bounds.Width {
		bounds.Width = shape.MinWidth
	}
	if f32IsFinite(shape.inkOverflowW) &&
		shape.inkOverflowW > bounds.Width {
		bounds.Width = shape.inkOverflowW
	}
	return bounds
}

// textRenderDisplay returns the exact string the frame shapes and paints.
// A block already owns it; only a short single-line input with no caret
// showing, selection, composition or spelling ranges can take the direct
// RenderText path without resolving one.
func textRenderDisplay(
	text string, shape *Shape, blk *textBlock, hasBlk bool,
) string {
	if hasBlk {
		return blk.text
	}
	if shape.focusOwner != "" && shape.TC.TextMode == TextModeSingleLine {
		return singleLineInputDisplayText(text)
	}
	return text
}

// textBlockVisibleBox maps the renderer's current clip into a text block's
// coordinates. Using the current clip, rather than just the window height,
// also bounds work inside an Input or a nested scroll view.
func textBlockVisibleBox(
	baseX, baseY float32, clip drawClip,
) (xMin, xMax, yMin, yMax float32) {
	xMin, yMin = clip.X-baseX, clip.Y-baseY
	xMax, yMax = xMin+clip.Width, yMin+clip.Height
	return xMin, xMax, yMin, yMax
}

// textLayoutTransform returns the transform a text shape's glyph layout
// draws through, about the text's origin, and whether there is one: the
// style's rotation or transform, after the animation's motion when it
// moves. The animation moves the text in its own frame, then the style's
// rotation applies to the result; replacing the style's transform
// dropped a caller's rotation on every frame the animation moved.
func textLayoutTransform(
	shape *Shape, style TextStyle,
) (glyph.AffineTransform, bool) {
	var anim *textAnimRender
	if shape.TC != nil {
		anim = shape.TC.anim
	}
	animT, animMoves := anim.transform(shape)
	if !style.hasTextTransform() && !animMoves {
		return glyph.AffineTransform{}, false
	}
	transform := style.effectiveTextTransform()
	if animMoves {
		transform = transform.Multiply(animT)
	}
	return transform, true
}

// emitTextLayoutCmd emits the glyph-layout draw of one chunk of a text
// block: the path for wrapped, aligned, decorated, transformed or
// animated text. The chunk sits at (dx, dy) in a block drawn at (baseX,
// baseY). glyph applies a draw's transform about the draw's origin, so
// under xf the draw keeps the block's origin and moves the chunk inside
// the transform: the block turns as one, where a chunk drawn at its own
// origin would turn about its own corner.
func emitTextLayoutCmd(
	text string, baseX, baseY, dx, dy float32,
	layout glyph.Layout, style TextStyle, gradient *glyph.GradientConfig,
	xf glyph.AffineTransform, xfOn bool, w *Window,
) {
	cmd := RenderCmd{
		Kind:         RenderLayout,
		X:            baseX + dx,
		Y:            baseY + dy,
		Text:         text,
		LayoutPtr:    w.scratch.renderGlyphLayouts.alloc(layout),
		TextStylePtr: w.scratch.renderTextStyles.alloc(style),
		TextGradient: gradient,
	}
	if xfOn {
		if dx != 0 || dy != 0 {
			xf = xf.Multiply(glyph.AffineTranslation(dx, dy))
		}
		cmd.Kind = RenderLayoutTransformed
		cmd.X, cmd.Y = baseX, baseY
		cmd.LayoutTransform = w.scratch.renderAffineTransforms.alloc(xf)
	}
	emitRenderer(cmd, w)
}

// renderInputCursor emits a thin rect for the text cursor when
// the shape is focused. Uses the glyph layout engine for precise
// character-boundary positioning. The rect is always emitted —
// transparent while the blink state is off — so a blink tick can
// toggle its visibility in place without rebuilding the render
// list (issue #404).
func renderInputCursor(shape *Shape, text string, baseX, baseY float32,
	blk *textBlock, hasBlk bool, w *Window) {
	if !textShapeFocused(shape, w) {
		return
	}
	if shape.TC != nil && shape.TC.textIsPlaceholder {
		text = ""
	}
	is := StateReadOr(w, nsInput, shape.focusKey(), inputState{})
	byteIdx := drawnOffset(shape.TC, text, is.CursorPos)

	style := textStyleOrDefault(shape)
	cursorW := inputCaretW

	if !hasBlk {
		blk, hasBlk = inputGlyphLayoutResolved(text, shape, style, w,
			shape.TC != nil && shape.TC.textIsPassword)
	}
	color := style.Color
	// A background window draws no caret (native macOS/Windows
	// behaviour) — syncBlinkCursor retires the blink animation for the
	// same reason. The rect is still emitted, transparent, so the
	// render list keeps one shape across focus states and the recorded
	// caretCmd slot stays valid (issue #404).
	if !w.inputCursorOn() || !w.hasFocus() {
		color = ColorTransparent
	}
	if hasBlk {
		if cp, ok := blk.cursorPosTrailing(byteIdx, is.cursorTrailing); ok {
			emitCaretCmd(RenderCmd{
				Kind:  RenderRect,
				X:     baseX + cp.X,
				Y:     baseY + cp.Y,
				W:     cursorW,
				H:     cp.Height,
				Color: color,
				Fill:  true,
			}, style.Color, w)
			return
		}
	}

	// Fallback for nil textMeasurer (tests).
	fh := fontHeight(style, w)
	cx := textWidthFallback(text, byteIdx, shape.TC, style, w, true)
	emitCaretCmd(RenderCmd{
		Kind:  RenderRect,
		X:     baseX + cx,
		Y:     baseY,
		W:     cursorW,
		H:     fh,
		Color: color,
		Fill:  true,
	}, style.Color, w)
}

// caretCmdState locates the focused caret's RenderCmd inside the
// window's flat render list so a blink tick can toggle it in place
// (issue #404). Recorded by emitCaretCmd during buildRenderers and
// reset at the start of every rebuild; the position is stable
// because the list only changes on a rebuild, which re-records.
type caretCmdState struct {
	idx   int   // index into w.renderers
	color Color // caret color when the blink state is on
	ok    bool  // a caret cmd is recorded this frame
}

// emitCaretCmd appends the caret rect and records its position for
// the in-place blink toggle. Only the first emission records — focus
// is exclusive, so there is at most one caret per window. The len
// check covers emitRenderer dropping an invalid rect: nothing to
// toggle in place then, and the next rebuild re-records.
func emitCaretCmd(r RenderCmd, caretColor Color, w *Window) {
	idx := len(w.renderers)
	emitRenderer(r, w)
	if idx < len(w.renderers) && !w.caretCmd.ok {
		w.caretCmd = caretCmdState{idx: idx, color: caretColor, ok: true}
	}
}

// commandToggleCaretBlink flips the recorded caret renderer's color
// to match the current blink state, without rebuilding the render
// list. Queued by BlinkCursorAnimation on each toggle and run by
// flushCommands on the main thread; sets renderersDirty so FrameFn
// presents the patched list.
func commandToggleCaretBlink(w *Window) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.caretCmd.ok {
		return
	}
	if w.caretCmd.idx >= len(w.renderers) ||
		w.renderers[w.caretCmd.idx].Kind != RenderRect {
		// Defensive: the recorded slot is no longer the caret.
		w.caretCmd = caretCmdState{}
		return
	}
	cmd := &w.renderers[w.caretCmd.idx]
	// Same gate as renderInputCursor: a Pulsar keeps the blink
	// animation registered while the window is unfocused, and its
	// toggles must not paint the caret back in.
	if w.inputCursorOn() && w.hasFocus() {
		cmd.Color = w.caretCmd.color
	} else {
		cmd.Color = ColorTransparent
	}
	w.renderersDirty = true
}

// drawnOffset maps a caret or selection offset of a shape's text to the
// text as drawn, clamped to it. They differ only for a password, whose
// bullets are wider in bytes than most of the runes they hide; every
// other drawn text is the text the offsets were taken in — the shape's
// own, or a rich text's flat text.
func drawnOffset(tc *ShapeTextConfig, drawn string, off int) int {
	if tc != nil && tc.textIsPassword && !tc.textIsPlaceholder {
		return displayOffset(tc.Text, drawn, off)
	}
	return runeStartAtOrBefore(drawn, max(0, min(off, len(drawn))))
}

// renderInputSelection emits highlight rectangles for the selected
// text range. Uses glyph layout for precise boundaries.
func renderInputSelection(shape *Shape, text string, baseX, baseY float32,
	blk *textBlock, hasBlk bool, clip drawClip, w *Window) {
	tc := shape.TC
	if tc == nil || tc.textSelBeg == tc.textSelEnd {
		return
	}
	beg, end := u32Sort(tc.textSelBeg, tc.textSelEnd)
	// The offsets are of the field's text. A password draws bullets,
	// which can be fewer bytes than the runes they hide; drawnOffset
	// maps the offsets onto them.
	limit := len(text)
	if tc.textIsPassword && !tc.textIsPlaceholder {
		limit = len(tc.Text)
	}
	if int(beg) > limit || int(end) > limit {
		return
	}

	style := textStyleOrDefault(shape)
	selColor := RGBA(51, 153, 255, 100)
	startByte := drawnOffset(tc, text, int(beg))
	endByte := drawnOffset(tc, text, int(end))

	if !hasBlk {
		blk, hasBlk = inputGlyphLayoutResolved(text, shape, style, w, tc.textIsPassword)
	}
	if hasBlk {
		xMin, xMax, yMin, yMax := textBlockVisibleBox(baseX, baseY, clip)
		rects := blk.selectionRectsVisible(startByte, endByte,
			xMin, xMax, yMin, yMax)
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
	x0 := textWidthFallback(text, startByte, tc, style, w, true)
	x1 := textWidthFallback(text, endByte, tc, style, w, true)
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
	shape *Shape, baseX, baseY float32,
	insertPos int, compText string, compCursor, compSelLen int,
	blk *textBlock, style TextStyle, clip drawClip, w *Window,
) {
	// insertPos is a byte offset into the drawn text; the input method
	// reports its caret and clause in runes of the preedit.
	startByte := insertPos
	endByte := insertPos + len(compText)

	c := style.Color
	if shape.Opacity < 1.0 {
		c = c.WithOpacity(shape.Opacity)
	}

	thinH := max(float32(1), style.Size/14)
	thickH := max(float32(2), style.Size/7)
	xMin, xMax, yMin, yMax := textBlockVisibleBox(baseX, baseY, clip)

	// Thin underline for the entire preedit region.
	rects := blk.selectionRectsVisible(startByte, endByte,
		xMin, xMax, yMin, yMax)
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
		sb := insertPos + runeToByteIndex(compText, compCursor)
		eb := insertPos + runeToByteIndex(compText, compCursor+compSelLen)
		selRects := blk.selectionRectsVisible(sb, eb,
			xMin, xMax, yMin, yMax)
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
	// Anchored to the caret rather than the preedit start, so a
	// mid-preedit cursor on a wrapped line puts the candidate on
	// the line being edited.
	caretByte := insertPos + runeToByteIndex(compText, compCursor)
	if cp, ok := blk.cursorPos(caretByte); ok {
		imeSetRectDrawn(drawClip{X: baseX + cp.X, Y: baseY + cp.Y,
			Width: inputCaretW, Height: cp.Height}, w)
	} else if len(rects) > 0 {
		r := rects[0]
		imeSetRectDrawn(drawClip{X: baseX + r.X, Y: baseY + r.Y,
			Width: r.Width, Height: r.Height}, w)
	}
}

// imeSetRectDrawn reports a caret rect, in the frame the text is drawn
// in, to the platform. Inside a view transform (#1019) it is mapped out
// to where the caret is drawn, so the candidate window sits by it. A
// quarter turn opens a frame on w.xfStack too (renderQuarterTurnBegin).
func imeSetRectDrawn(r drawClip, w *Window) {
	r = renderBoxToWindow(r, w)
	w.IMESetRect(r.X, r.Y, r.Width, r.Height)
}

// inputGlyphLayout returns the chunked layout of the input text,
// applying password masking and wrap width as needed.
func inputGlyphLayout(text string, shape *Shape, style TextStyle, w *Window) (*textBlock, bool) {
	return inputGlyphLayoutResolved(text, shape, style, w, false)
}

func inputGlyphLayoutResolved(text string, shape *Shape, style TextStyle, w *Window, textAlreadyMasked bool) (*textBlock, bool) {
	if w == nil || w.textMeasurer == nil {
		return nil, false
	}
	displayText := text
	if shape.TC != nil && shape.TC.textIsPassword && !textAlreadyMasked {
		displayText = maskPassword(text)
	}
	return plainTextBlock(displayText, shape, style, w)
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
//
// Without a measurer the fallback is fallbackLineHeight — the same
// 1.4em approximation view_text.go sizes new text with — so layout
// and optical centring agree on the box in headless tests.
func fontHeight(style TextStyle, w *Window) float32 {
	if w != nil && w.textMeasurer != nil {
		return w.textMeasurer.FontHeight(style)
	}
	if style.Size > 0 {
		return fallbackLineHeight(style)
	}
	return 16
}

// textWidthFallback approximates the width of text up to byte offset
// pos for tests (no glyph). textAlreadyMasked skips password masking:
// renderText's fallback sites pass text masked at the top of the frame,
// and masking bullets again is a wasted allocation for an identical
// string.
func textWidthFallback(text string, pos int, tc *ShapeTextConfig, style TextStyle, w *Window, textAlreadyMasked bool) float32 {
	prefix := text[:max(0, min(pos, len(text)))]
	if tc != nil && tc.textIsPassword && !textAlreadyMasked {
		prefix = maskPassword(prefix)
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
		LayoutPtr: shape.TC.rtf.rTFLayout,
	}, w)

	if flat := shape.TC.rtf.rTFFlatText; flat != "" &&
		shape.TC.textSelBeg != shape.TC.textSelEnd {
		// The selection path queries a block; this one wraps the shaped
		// rich-text layout and is never asked to shape. Built only for a
		// selection: a document of many blocks draws every frame.
		blk := newTextBlockFromLayout(flat, shape.TC.rtf.rTFLayout, 0,
			textStyleOrDefault(shape), shape.TC.TextMode, w)
		renderInputSelection(shape, flat, baseX, baseY, blk, true, clip, w)
	}

	// Emit RenderImage for inline math objects.
	// Use rtfMathHashes (populated by toGlyphRichTextWithMath)
	// instead of item.ObjectID, which is unreliable due to
	// Pango's shape attribute data pointer lacking null
	// termination on round-trip through C.GoString.
	cache := w.viewState.diagramCache
	hashes := shape.TC.rtf.rtfMathHashes
	if cache == nil || len(hashes) == 0 {
		return
	}
	objIdx := 0
	for i := range shape.TC.rtf.rTFLayout.Items {
		item := &shape.TC.rtf.rTFLayout.Items[i]
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
		if !ok || entry.State != diagramReady {
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
			Resource: entry.pNGPath,
			Opacity:  imageAlpha(shape.Opacity, shape.Disabled),
		}, w)
	}
}
