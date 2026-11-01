package gui

import (
	"strings"

	"github.com/go-gui-org/go-glyph"
)

// fallbackLineHeight is the height of one line of text when no
// TextMeasurer is injected — the same 1.4em approximation the initial
// estimate in view_text.go uses. Shared so the estimate and the
// post-sizing fallback below cannot drift apart.
func fallbackLineHeight(style TextStyle) float32 {
	return style.Size * 1.4
}

// plainTextHeightNoMeasurer approximates the height of a text shape
// when w.textMeasurer is nil, i.e. in headless tests.
//
// Without this the shape keeps the single-line estimate assigned in
// view_text.go no matter how much text it holds, because the real
// wrap path (plainTextBlock) needs a measurer and bails.
// Wrapped text then reports one line, a scroll container around it
// never overflows, and TestScroll comes back ErrTestUnhandled for a
// reason that has nothing to do with the widget under test.
//
// Line count is derived from Window.TextWidth, which has its own
// no-measurer approximation (0.6em per rune). The result is an
// estimate of an estimate — right for "does this overflow", wrong for
// any pixel assertion, which is already the documented contract of a
// window built by NewTestWindow.
//
// Empty text reserves one line: the shape still holds a caret, and the
// single-line estimate in view_text.go sizes it the same way.
func plainTextHeightNoMeasurer(
	shape *Shape,
	tc *ShapeTextConfig,
	style TextStyle,
	w *Window,
) float32 {
	lineH := fallbackLineHeight(style)
	if style.LineSpacing != 0 {
		lineH += style.LineSpacing
	}
	wraps := tc.TextMode == TextModeWrap ||
		tc.TextMode == TextModeWrapKeepSpaces
	// Only wrap against a width that sizing has actually resolved; a
	// zero or negative width would divide every line into infinity.
	avail := shape.Width

	// Walk the hard lines with Cut rather than strings.Split: this runs
	// inside the layout walk, once per text shape, and Split would put
	// a slice on the heap for every one of them.
	lines := 0
	rest := tc.Text
	for {
		line, tail, more := strings.Cut(rest, "\n")
		rest = tail
		n := 1
		if wraps && avail > 0 {
			ln := w.TextWidth(line, style)
			// Ceiling division without math.Ceil: one line per full
			// avail, plus one for any remainder.
			n = int(ln / avail)
			if ln > float32(n)*avail {
				n++
			}
			n = max(n, 1)
		}
		lines += n
		if !more {
			break
		}
	}
	return float32(lines) * lineH
}

func plainTextNeedsGlyphLayout(
	shape *Shape,
	tc *ShapeTextConfig,
	style TextStyle,
) bool {
	if shape == nil || tc == nil {
		return false
	}
	// Past one chunk the text is drawn chunk by chunk: a single
	// RenderText command would hand the backend a string the shaper
	// refuses (#1190).
	return plainTextStyleNeedsGlyphLayout(tc, style) ||
		len(tc.Text) > textChunkBytes
}

// plainTextStyleNeedsGlyphLayout reports whether a text shape needs a
// glyph layout for its mode or style, whatever its length. A single-line
// text that needs one only for its length keeps the one-line box
// GenerateLayout gives it (layoutPlainText).
func plainTextStyleNeedsGlyphLayout(tc *ShapeTextConfig, style TextStyle) bool {
	return tc.TextMode != TextModeSingleLine ||
		style.Align != TextAlignLeft ||
		style.LineSpacing != 0 ||
		style.BgColor.A > 0 ||
		style.Features != nil ||
		style.Gradient != nil ||
		style.hasTextTransform() ||
		// A text animation that moves, reveals or shimmers paints
		// through a glyph layout: RenderText has no transform, no
		// per-glyph mask and no gradient. See textAnimRender.
		tc.anim.needsGlyphLayout()
}

func plainTextLayoutWidthArg(
	shape *Shape,
	tc *ShapeTextConfig,
	style TextStyle,
) float32 {
	if shape == nil || tc == nil || shape.Width <= 0 {
		return 0
	}
	if tc.TextMode == TextModeWrap ||
		tc.TextMode == TextModeWrapKeepSpaces {
		return shape.Width
	}
	if style.Align != TextAlignLeft {
		return -shape.Width
	}
	return 0
}

// plainTextBlock returns the chunked layout of a text shape's text at
// the shape's width: the block the frame already resolved for the
// shape, else the one cached under the shape's identity (an input's
// owner, a Text's ID, or for a large anonymous text the string's own
// memory), brought up to date, else a new one. A short anonymous label
// gets a block for the frame, which shapes it once as before.
func plainTextBlock(
	text string, shape *Shape, style TextStyle, w *Window,
) (*textBlock, bool) {
	if w == nil || w.textMeasurer == nil ||
		shape == nil || shape.TC == nil {
		return nil, false
	}
	tc := shape.TC
	widthArg := plainTextLayoutWidthArg(shape, tc, style)
	b := tc.block
	switch {
	case b != nil:
		b.update(text, widthArg, style, tc.TextMode, w)
	default:
		key := shape.focusKey()
		if key == "" && len(text) > textChunkBytes {
			key = textBlockAnonKey(text)
		}
		if key == "" {
			b = newTextBlock(text, widthArg, style, tc.TextMode, w)
			break
		}
		m := StateMap[string, *textBlock](w, nsTextBlock, capTextBlocks)
		var ok bool
		if b, ok = m.Get(key); ok && b != nil {
			b.update(text, widthArg, style, tc.TextMode, w)
		} else {
			b = newTextBlock(text, widthArg, style, tc.TextMode, w)
			m.Set(key, b)
		}
	}
	tc.block = b
	if b.failErr != nil {
		textBlockWarnRefused(shape, b, w)
	}
	return b, true
}

// textBlockWarnRefused reports a chunk the shaper refused, once per
// identity: the caret, selection and grapheme delete lose precision in
// that chunk, and nothing else in the frame says why. Gated here as
// well as inside debugWarn: a refused chunk is refused again every
// frame, and the variadic args would allocate each time even with the
// check turned off.
func textBlockWarnRefused(shape *Shape, b *textBlock, w *Window) {
	if DebugCategory(debugMask.Load())&DebugGlyphLayoutFallback == 0 {
		return
	}
	w.debugWarn(debugCheckGlyphLayoutFallback, shape.idKey(),
		"glyph layout failed for %q (%d bytes): %v; "+
			"using approximate metrics with degraded caret, "+
			"selection and delete precision",
		shape.idKey(), b.failBytes, b.failErr)
}

// textWidthEstimate approximates the advance of a text the shaper
// would refuse to measure whole (over glyph.MaxTextLength), from the
// average advance of a sample. The sizing pass replaces it with the
// chunked layout's width.
func textWidthEstimate(text string, style TextStyle, w *Window) float32 {
	avg := style.Size * 0.5
	if w != nil && w.textMeasurer != nil {
		if wd := w.textMeasurer.TextWidth(textBlockSample, style); wd > 0 &&
			f32IsFinite(wd) {
			avg = wd / float32(len(textBlockSample))
		}
	}
	return float32(utf8RuneCount(text)) * avg
}

// textLayoutStyleEqual reports whether two styles shape to the same
// glyph layout. It is == with the paint colours left out: they change
// the colour stamped on each item but no glyph position. Whether a
// background or a stroke is present does change the layout (the item
// carries HasBgColor and HasStroke), so presence still counts.
func textLayoutStyleEqual(a, b TextStyle) bool {
	if (a.BgColor.A > 0) != (b.BgColor.A > 0) ||
		(a.StrokeColor.A > 0) != (b.StrokeColor.A > 0) {
		return false
	}
	a.Color, b.Color = Color{}, Color{}
	a.BgColor, b.BgColor = Color{}, Color{}
	a.StrokeColor, b.StrokeColor = Color{}, Color{}
	return a == b
}

// textPaintEqual reports whether two styles paint in the same colours.
func textPaintEqual(a, b TextStyle) bool {
	return a.Color == b.Color && a.BgColor == b.BgColor &&
		a.StrokeColor == b.StrokeColor
}

// recolorLayoutItems returns a copy of items painted in style's
// colours. The copy comes from the render-phase arena, so the cached
// layout keeps the colours it was shaped with and the next layout pass
// still hits. A plain text is shaped from one style, so every item takes
// the same colours; an emoji item keeps its own (UseOriginalColor).
func recolorLayoutItems(
	items []glyph.Item, style TextStyle, w *Window,
) []glyph.Item {
	out := w.scratch.takeTextItems(len(items))
	copy(out, items)
	fg := colorToGlyph(style.Color)
	bg := colorToGlyph(style.BgColor)
	stroke := colorToGlyph(style.StrokeColor)
	for i := range out {
		if !out[i].UseOriginalColor {
			out[i].Color = fg
		}
		if out[i].HasBgColor {
			out[i].BgColor = bg
		}
		if out[i].HasStroke {
			out[i].StrokeColor = stroke
		}
	}
	return out
}
