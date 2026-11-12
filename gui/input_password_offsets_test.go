package gui

import "testing"

// A password's caret and selection are bytes of its text, and it draws
// bullets of another byte length. Every path between the two maps them.

// Four 4-byte emoji draw as four 3-byte bullets: a selection of the
// whole text still highlights, over all four.
func TestRenderInputSelectionPasswordWiderThanMask(t *testing.T) {
	w := makeWindow()
	style := TextStyle{Color: RGB(0, 0, 0), Size: 14}
	text := "😀😀😀😀"
	shape := &Shape{
		TC: &ShapeTextConfig{
			Text:           text,
			textSelBeg:     0,
			textSelEnd:     uint32(len(text)),
			textIsPassword: true,
			TextStyle:      &style,
		},
	}
	renderInputSelection(shape, maskPassword(text), 0, 0,
		nil, false, makeClip(0, 0, 800, 600), w)
	if len(w.renderers) != 1 {
		t.Fatalf("got %d renderers, want 1 selection rect", len(w.renderers))
	}
	if want := float32(4) * 14 * 0.6; !f32AreClose(w.renderers[0].W, want) {
		t.Errorf("selection width = %v, want %v (four bullets)",
			w.renderers[0].W, want)
	}
}

// A stale selection inside the characters of an app-set password still
// highlights: 2..4 of "😀😀" is the first emoji once both ends snap to
// rune starts. Counted unsnapped, the start mapped past the end.
func TestRenderInputSelectionPasswordStaleOffsets(t *testing.T) {
	w := makeWindow()
	style := TextStyle{Color: RGB(0, 0, 0), Size: 14}
	text := "😀😀"
	shape := &Shape{
		TC: &ShapeTextConfig{
			Text:           text,
			textSelBeg:     2,
			textSelEnd:     4,
			textIsPassword: true,
			TextStyle:      &style,
		},
	}
	renderInputSelection(shape, maskPassword(text), 0, 0,
		nil, false, makeClip(0, 0, 800, 600), w)
	if len(w.renderers) != 1 {
		t.Fatalf("got %d renderers, want 1 selection rect", len(w.renderers))
	}
	if want := float32(14) * 0.6; !f32AreClose(w.renderers[0].W, want) {
		t.Errorf("selection width = %v, want %v (the first bullet)",
			w.renderers[0].W, want)
	}
}
