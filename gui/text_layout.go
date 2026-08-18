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
		style.hasTextTransform()
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
		tc.textLayoutStyle == style &&
		tc.textLayoutMode == tc.TextMode &&
		tc.textLayout != nil {
		return *tc.textLayout, true
	}
	layout, err := w.textMeasurer.LayoutText(text, style, widthArg)
	if err != nil {
		return glyph.Layout{}, false
	}
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
	// Guard a face (or a LineSpacing) whose line box is tighter than
	// ascent+descent: growing the shape here would be a regression.
	lineH := l.Height / float32(lines)
	if fh >= lineH {
		return l.Height
	}
	return float32(lines-1)*lineH + fh
}
