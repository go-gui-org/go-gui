package gui

import (
	"math"
	"testing"
)

// inkStubMeasurer is a stubTextMeasurer that also reports an ink box, so
// the ink-centring path can be exercised without a real font backend.
// The ink box is inset from the advance box by the given margins.
type inkStubMeasurer struct {
	stubTextMeasurer
	left, top, right, bottom float32
	unavailable              bool
}

func (m *inkStubMeasurer) TextInkBounds(text string, style TextStyle) (
	InkBounds, bool,
) {
	if m.unavailable {
		return InkBounds{}, false
	}
	return InkBounds{
		X:      m.left,
		Y:      m.top,
		Width:  m.TextWidth(text, style) - m.left - m.right,
		Height: m.FontHeight(style) - m.top - m.bottom,
	}, true
}

// inkTestFrame builds a 40x40 frame holding one 10x20 text child placed
// where advance-box centring would put it, so a hook run can be compared
// against the uncorrected position.
func inkTestFrame() Layout {
	frame := Layout{Shape: &Shape{X: 100, Y: 200, Width: 40, Height: 40}}
	frame.Children = []Layout{{Shape: &Shape{
		shapeType: shapeText,
		X:         115, Y: 210, Width: 10, Height: 20,
	}}}
	return frame
}

// TestCenterGlyphOnInk asserts the hook lands the ink centre — not the
// advance-box centre — on the frame's centre, which is the whole point of
// the correction.
func TestCenterGlyphOnInk(t *testing.T) {
	w := newTestWindow()
	// 10px per char, 20px line box: "x" occupies 10x20. Ink insets of
	// 2 left / 6 top / 4 right / 2 bottom put the ink centre 1px left
	// of and 2px above the advance-box centre.
	w.SetTextMeasurer(&inkStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
		left:             2, top: 6, right: 4, bottom: 2,
	})

	frame := inkTestFrame()
	centerGlyphOnInk("x", TextStyle{Size: 12})(EventCtx{&frame, nil, w})

	glyph := frame.Children[0].Shape
	inkCX := glyph.X + 2 + (10-2-4)/2
	inkCY := glyph.Y + 6 + (20-6-2)/2
	if !floatNear(inkCX, 120, 0.01) || !floatNear(inkCY, 220, 0.01) {
		t.Errorf("ink centre (%v, %v), want frame centre (120, 220)",
			inkCX, inkCY)
	}
}

// TestCenterGlyphOnInkUnavailable covers every path that must leave the
// glyph on advance-box centring: no measurer, a measurer without the ink
// capability, and a measurer that cannot read the glyph.
func TestCenterGlyphOnInkUnavailable(t *testing.T) {
	cases := []struct {
		name string
		tm   TextMeasurer
	}{
		{"no measurer", nil},
		{"no ink capability",
			&stubTextMeasurer{charWidth: 10, fontHeight: 20}},
		{"unmeasurable glyph", &inkStubMeasurer{
			stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
			unavailable:      true,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWindow()
			if tc.tm != nil {
				w.SetTextMeasurer(tc.tm)
			}
			frame := inkTestFrame()
			centerGlyphOnInk("x", TextStyle{Size: 12})(
				EventCtx{&frame, nil, w})

			glyph := frame.Children[0].Shape
			if !floatNear(glyph.X, 115, 0.01) ||
				!floatNear(glyph.Y, 210, 0.01) {
				t.Errorf("glyph moved to (%v, %v), want (115, 210)",
					glyph.X, glyph.Y)
			}
		})
	}
}

// TestCenterGlyphOnInkNoText asserts a frame with no text child is a
// no-op rather than a panic — mdTaskCheckbox builds an empty frame for an
// unchecked item.
func TestCenterGlyphOnInkNoText(t *testing.T) {
	w := newTestWindow()
	w.SetTextMeasurer(&inkStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
	})
	frame := Layout{Shape: &Shape{Width: 40, Height: 40}}
	centerGlyphOnInk("x", TextStyle{Size: 12})(EventCtx{&frame, nil, w})
}

// TestCenterGlyphOnInkSnapToPixelGrid asserts the baseline snap lands
// the glyph's baseline on a whole physical pixel at a fractional scale,
// so the backend's own whole-pixel baseline rounding cannot fight the
// ink centring.
func TestCenterGlyphOnInkSnapToPixelGrid(t *testing.T) {
	w := newTestWindow()
	w.SetTextMeasurer(&inkStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
		left:             2, top: 6, right: 4, bottom: 2,
	})
	// Retina 2x. The ink-centring math lands the baseline at an
	// odd quarter-pixel (…217.6 at 1x in TestCenterGlyphOnInk);
	// the snap must pull it onto the grid.
	w.BackingScale = 2

	frame := inkTestFrame()
	centerGlyphOnInk("x", TextStyle{Size: 12})(EventCtx{&frame, nil, w})

	glyph := frame.Children[0].Shape
	baseline := glyph.Y + 12*0.8
	snapped := baseline * 2
	if !floatNear(snapped, float32(math.Round(float64(snapped))), 0.001) {
		t.Errorf("baseline %v is off the 2x pixel grid (snapped: %v)",
			baseline, snapped)
	}
}

// TestCenterGlyphOnInkNonFiniteScale asserts a non-finite BackingScale
// skips the snap entirely, leaving the ink-centred position alone — the
// scale comes from the backend each frame and render defends against a
// corrupted value, so the snap must not poison the glyph with NaN.
func TestCenterGlyphOnInkNonFiniteScale(t *testing.T) {
	for _, scale := range []float32{
		float32(math.NaN()), float32(math.Inf(1)),
	} {
		w := newTestWindow()
		w.SetTextMeasurer(&inkStubMeasurer{
			stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
			left:             2, top: 6, right: 4, bottom: 2,
		})
		w.BackingScale = scale

		frame := inkTestFrame()
		centerGlyphOnInk("x", TextStyle{Size: 12})(EventCtx{&frame, nil, w})

		glyph := frame.Children[0].Shape
		if math.IsNaN(float64(glyph.Y)) || math.IsInf(float64(glyph.Y), 0) {
			t.Errorf("scale %v: glyph.Y = %v, must stay finite",
				scale, glyph.Y)
		}
	}
}

// TestToggleCheckCentersOnInk asserts the rendered check ends up centred
// on its ink inside the box, which is the whole point of the correction:
// the glyph's own centre, not its advance box's centre, lands on the box
// centre.
func TestToggleCheckCentersOnInk(t *testing.T) {
	w := &Window{windowWidth: 200, windowHeight: 100}
	w.SetTextMeasurer(&inkStubMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
		left:             2, top: 6, right: 4, bottom: 2,
	})

	root := Toggle(ToggleCfg{
		ID: "t", Selected: true, Size: SomeF(40),
	}).GenerateLayout(w)
	layoutArrange(&root, w)

	box := root.Children[0]
	text := findShapeType(&box, shapeText)
	if text == nil {
		t.Fatal("no text shape under the toggle box")
	}

	// Ink centre of the text shape, and the centre of the box it sits in.
	inkCX := text.Shape.X + 2 + (text.Shape.Width-2-4)/2
	inkCY := text.Shape.Y + 6 + (text.Shape.Height-6-2)/2
	boxCX := box.Shape.X + box.Shape.Width/2
	boxCY := box.Shape.Y + box.Shape.Height/2

	if !floatNear(inkCX, boxCX, 0.01) || !floatNear(inkCY, boxCY, 0.01) {
		t.Errorf("ink centre (%v, %v), box centre (%v, %v)",
			inkCX, inkCY, boxCX, boxCY)
	}
}

// TestToggleCheckFallsBackWithoutInk asserts the no-capability path
// leaves the check exactly where advance-box centring put it.
func TestToggleCheckFallsBackWithoutInk(t *testing.T) {
	w := &Window{windowWidth: 200, windowHeight: 100}
	w.SetTextMeasurer(&stubTextMeasurer{charWidth: 10, fontHeight: 20})

	root := Toggle(ToggleCfg{
		ID: "t", Selected: true, Size: SomeF(40),
	}).GenerateLayout(w)
	layoutArrange(&root, w)

	box := root.Children[0]
	text := box.Children[0]
	// Advance box centred: 30x20 text in a 40x40 box.
	if !floatNear(text.Shape.X, box.Shape.X+5, 0.01) ||
		!floatNear(text.Shape.Y, box.Shape.Y+10, 0.01) {
		t.Errorf("text at (%v, %v), want advance-centred (%v, %v)",
			text.Shape.X, text.Shape.Y, box.Shape.X+5, box.Shape.Y+10)
	}
}

func floatNear(a, b, tol float32) bool {
	d := a - b
	return d <= tol && d >= -tol
}

// findShapeType returns the first descendant (depth-first) with the given
// shape type, or nil.
func findShapeType(l *Layout, st shapeType) *Layout {
	if l == nil || l.Shape == nil {
		return nil
	}
	if l.Shape.shapeType == st {
		return l
	}
	for i := range l.Children {
		if found := findShapeType(&l.Children[i], st); found != nil {
			return found
		}
	}
	return nil
}
