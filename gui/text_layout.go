package gui

import "github.com/go-gui-org/go-glyph"

func plainTextNeedsGlyphLayout(
	shape *Shape,
	tc *ShapeTextConfig,
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
		style.HasTextTransform()
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
		tc.TextLayout != nil {
		return *tc.TextLayout, true
	}
	layout, err := w.textMeasurer.LayoutText(text, style, widthArg)
	if err != nil {
		return glyph.Layout{}, false
	}
	tc.TextLayout = &layout
	tc.textLayoutWidth = widthArg
	tc.textLayoutText = text
	tc.textLayoutStyle = style
	tc.textLayoutMode = tc.TextMode
	tc.textLayoutValid = true
	return layout, true
}
