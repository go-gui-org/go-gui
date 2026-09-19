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
// wrap path (plainTextLayoutResolved) needs a measurer and bails.
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
	tc *shapeTextConfig,
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
	tc *shapeTextConfig,
	style TextStyle,
) bool {
	if shape == nil || tc == nil {
		return false
	}
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
	tc *shapeTextConfig,
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

func plainTextLayoutResolved(
	text string,
	shape *Shape,
	style TextStyle,
	w *Window,
) (glyph.Layout, bool) {
	if w == nil || w.textMeasurer == nil ||
		shape == nil || shape.TC == nil {
		return glyph.Layout{}, false
	}
	tc := shape.TC
	widthArg := plainTextLayoutWidthArg(shape, tc, style)
	if tc.textLayoutValid &&
		f32AreClose(tc.textLayoutWidth, widthArg) &&
		tc.textLayoutText == text &&
		textLayoutStyleEqual(tc.textLayoutStyle, style) &&
		tc.textLayoutMode == tc.TextMode &&
		tc.textLayout != nil {
		l := *tc.textLayout
		// A hit on geometry with different paint colours is the render
		// pass of a faded, pulsing or disabled text: renderText adjusts
		// Color, BgColor and StrokeColor after the layout pass shaped
		// with the plain ones. The glyphs are the same, so recolour a
		// copy of the items instead of shaping the text again.
		if !textPaintEqual(tc.textLayoutStyle, style) {
			l.Items = recolorLayoutItems(l.Items, style, w)
		}
		return l, true
	}
	shaped, err := w.textMeasurer.LayoutText(text, style, widthArg)
	if err != nil {
		// The shaper refused the text — past its byte budget, for
		// example — so the frame falls back to approximate metrics.
		// Report once per identity: without this the caret,
		// selection and grapheme delete silently lose precision,
		// and nothing else in the frame says why.
		//
		// Gated here as well as inside debugWarn: a refused text is
		// refused again every frame, and the variadic args would
		// allocate each time even with the check turned off.
		if DebugCategory(debugMask.Load())&DebugGlyphLayoutFallback != 0 {
			w.debugWarn(debugCheckGlyphLayoutFallback, shape.idKey(),
				"glyph layout failed for %q (%d bytes): %v; "+
					"using approximate metrics with degraded caret, "+
					"selection and delete precision",
				shape.idKey(), len(text), err)
		}
		return glyph.Layout{}, false
	}
	// Copied into its own variable here, on the success path only:
	// taking the address of the call's result moved it to the heap on
	// every call, a refused text included.
	layout := shaped
	tc.textLayout = &layout
	tc.textLayoutWidth = widthArg
	tc.textLayoutText = text
	tc.textLayoutStyle = style
	tc.textLayoutMode = tc.TextMode
	tc.textLayoutValid = true
	return layout, true
}

// plainTextBoxHeight converts glyph's layout height into the box
// go-gui sizes text with.
//
// glyph reports Height as n whole line boxes, and a line box is the
// baseline-to-baseline advance: ascent + descent + leading, floored at
// 1.15em (go-glyph linespacing.go). Rendering, though, puts the first
// baseline at FontAscent below the shape top and advances by that same
// line height, so the leading below the *last* baseline is space
// nothing ever paints into. Kept in the shape, it is padding only at
// the bottom: a centred or padded row then reads visibly top-biased —
// 2.4px at size 16, which is what a ListBox row showed.
//
// Dropping the trailing leading makes a one-line shape exactly
// FontHeight, the same box the single-line path in view_text.go
// assigns, so the two paths agree and the optical-centring model in
// text_optical.go ("a text shape is sized to FontHeight") holds for
// multi-line text too. Inter-line spacing is untouched: only the tail
// goes.
func plainTextBoxHeight(
	l glyph.Layout, style TextStyle, w *Window,
) float32 {
	lines := len(l.Lines)
	if lines == 0 || l.Height <= 0 || !f32IsFinite(l.Height) {
		return l.Height
	}
	fh := fontHeight(style, w)
	// A non-finite face height (a corrupt measurer) must not poison
	// the box: the shape would move to NaN and never paint again.
	if !f32IsFinite(fh) {
		return l.Height
	}
	// The trailing leading is the last line's own box minus the face
	// height. Read it from the last line, not from an average: glyph
	// adds LineSpacing after every line but the last, and a fallback
	// face can make one line taller than the rest, so Height/lines is
	// not the last line's height. An average made the box short by
	// (n-1)*LineSpacing/n, and the last line's descenders hung out of
	// it. A layout without line rects (a host-built one) falls back
	// to the average, which is exact when the lines are even.
	lastH := l.Lines[lines-1].Rect.Height
	if lastH <= 0 || !f32IsFinite(lastH) {
		lastH = l.Height / float32(lines)
	}
	// Guard a face whose line box is tighter than ascent+descent:
	// growing the shape here would be a regression.
	if fh >= lastH {
		return l.Height
	}
	return l.Height - (lastH - fh)
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
