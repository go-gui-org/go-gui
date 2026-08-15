package gui

import "testing"

// disclosureArrowLayout generates the helper's view tree and returns
// the root Row plus its Text child, which the arrow glyph lives in.
func disclosureArrowLayout(open bool, style TextStyle) (Layout, *Shape) {
	root := generateViewLayout(disclosureArrow(open, style), &Window{})
	if len(root.Children) != 1 {
		return root, nil
	}
	return root, root.Children[0].Shape
}

// TestDisclosureArrowGlyph pins the open/closed glyph selection shared
// by ExpandPanel, Combobox and Select.
func TestDisclosureArrowGlyph(t *testing.T) {
	_, closed := disclosureArrowLayout(false, DefaultTextStyle)
	if closed.TC.Text != "▼" {
		t.Errorf("closed: glyph %q, want ▼", closed.TC.Text)
	}
	_, open := disclosureArrowLayout(true, DefaultTextStyle)
	if open.TC.Text != "▲" {
		t.Errorf("open: glyph %q, want ▲", open.TC.Text)
	}
}

// TestDisclosureArrowScale pins the size reduction and the optical
// nudge padding: the glyph renders at 0.6x the body size and the
// wrapper carries nudge*scaledSize of top padding.
func TestDisclosureArrowScale(t *testing.T) {
	root, glyph := disclosureArrowLayout(false, DefaultTextStyle)
	wantSize := DefaultTextStyle.Size * disclosureArrowScale
	if got := glyph.TC.TextStyle.Size; got != wantSize {
		t.Errorf("size: got %v, want %v", got, wantSize)
	}
	wantPad := wantSize * disclosureArrowNudge
	if got := root.Shape.Padding.Top; got != wantPad {
		t.Errorf("top padding: got %v, want %v", got, wantPad)
	}
}

// TestDisclosureArrowZeroStyle pins that a fully zero style inherits
// DefaultTextStyle (family, color and size) and a style with fields but
// zero Size only gets the size, both before the scale applies.
func TestDisclosureArrowZeroStyle(t *testing.T) {
	_, zero := disclosureArrowLayout(false, TextStyle{})
	if zero.TC.TextStyle.Family != DefaultTextStyle.Family {
		t.Error("zero style: family not inherited from DefaultTextStyle")
	}
	if zero.TC.TextStyle.Color != DefaultTextStyle.Color {
		t.Error("zero style: color not inherited from DefaultTextStyle")
	}

	c := RGBA(10, 20, 30, 255)
	_, partial := disclosureArrowLayout(false, TextStyle{Color: c})
	if partial.TC.TextStyle.Color != c {
		t.Error("partial style: color not preserved")
	}
	if got := partial.TC.TextStyle.Size; got != sizeTextMedium*disclosureArrowScale {
		t.Errorf("partial style: size = %v, want %v", got, sizeTextMedium*disclosureArrowScale)
	}
}
