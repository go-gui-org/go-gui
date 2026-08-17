package gui

import "testing"

// iconProbeRune stands in for an icon face's glyph: a private-use
// codepoint, which is where icon fonts put their symbols.
const iconProbeRune = "\ue000"

// runInkMeasurer reports a different ink box per run. That is what makes
// the three bands distinguishable at all: with one box for every string
// the run, cap and figure bands agree, and a test asserting which one a
// widget took would pass whatever the widget did — which is exactly the
// hole the golden harness has, since it runs with no measurer.
type runInkMeasurer struct {
	stubTextMeasurer
	ink map[string]InkBounds
}

func (m *runInkMeasurer) TextInkBounds(text string, _ TextStyle) (
	InkBounds, bool,
) {
	b, ok := m.ink[text]
	return b, ok
}

// bandTestWindow returns a window whose face is 20 tall and whose bands
// disagree by construction:
//
//	"H" (the cap probe)  ink 2..14, centre  8 → cap offset   +2
//	"0" (the digit probe) ink 4..14, centre  9 → digit offset +1
//	an icon glyph         ink 6..16, centre 11 → run offset    0 (clamped)
//
// The icon is the case the marker exists for: its own ink already sits
// below the box centre, so the right correction for it is none, while
// the cap band would push it down by two.
func bandTestWindow() *Window {
	w := newTestWindow()
	w.SetTextMeasurer(&runInkMeasurer{
		stubTextMeasurer: stubTextMeasurer{charWidth: 10, fontHeight: 20},
		ink: map[string]InkBounds{
			opticalProbe:      {Y: 2, Height: 12, Width: 10},
			opticalDigitProbe: {Y: 4, Height: 10, Width: 10},
			"Save":            {Y: 2, Height: 12, Width: 40},
			iconProbeRune:     {Y: 6, Height: 10, Width: 10},
		},
	})
	return w
}

// bandTestFrame is a container holding one text child at a known Y, so a
// hook run can be read as a displacement.
func bandTestFrame(text string, style TextStyle) Layout {
	frame := Layout{Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 20}}
	frame.Children = []Layout{{Shape: &Shape{
		shapeType: shapeText,
		X:         0, Y: 0, Width: 40, Height: 20,
		TC: &shapeTextConfig{Text: text, TextStyle: &style},
	}}}
	return frame
}

// An icon glyph has no cap band to speak of, so a container correcting
// its children on the face's caps must leave the glyph on its own ink —
// the answer centerGlyphOnInk gives a checkbox's check. Without the
// style's glyph role there is no way to tell an arrow from a word:
// either can sit inside the cap band (issue #346).
func TestOpticalCapBandLeavesGlyphsOnTheirOwnInk(t *testing.T) {
	w := bandTestWindow()

	iconStyle := TextStyle{Size: 16, glyphRole: true}
	frame := bandTestFrame(iconProbeRune, iconStyle)
	opticalCenterChildren(EventCtx{&frame, nil, w}, opticalBandCap)
	if got := frame.Children[0].Shape.Y; got != 0 {
		t.Errorf("icon glyph moved by %v, want 0 (its ink is already low)",
			got)
	}

	// The same container, same band, a label rather than a glyph: this
	// one takes the cap band, or the test above would pass with the
	// correction switched off entirely.
	labelFrame := bandTestFrame("Save", TextStyle{Size: 16})
	opticalCenterChildren(EventCtx{&labelFrame, nil, w}, opticalBandCap)
	if got := labelFrame.Children[0].Shape.Y; got != 2 {
		t.Errorf("label moved by %v, want the cap offset 2", got)
	}
}

// A blank run keeps taking a content-free correction: an empty digit
// field still shows a caret, positioned from this shape, and declining
// to move it would step the caret as soon as a character arrived. The
// measured band is the one that declines — there the question is about
// ink, and a blank run has none.
func TestOpticalCenterBlankRunFollowsTheBand(t *testing.T) {
	w := bandTestWindow()

	frame := bandTestFrame(" ", TextStyle{Size: 16})
	opticalCenterChildren(EventCtx{&frame, nil, w}, opticalBandDigit)
	if got := frame.Children[0].Shape.Y; got != 1 {
		t.Errorf("blank run on the figure band moved by %v, want 1", got)
	}

	runFrame := bandTestFrame(" ", TextStyle{Size: 16})
	opticalCenterChildren(EventCtx{&runFrame, nil, w}, opticalBandRun)
	if got := runFrame.Children[0].Shape.Y; got != 0 {
		t.Errorf("blank run on the measured band moved by %v, want 0", got)
	}
}

// A button whose label is digits by construction takes the figure band:
// figures measure shorter than caps, so the cap band would land them as
// far low as an uncorrected label sits high.
func TestButtonDigitLabelTakesFigureBand(t *testing.T) {
	w := bandTestWindow()
	frame := bandTestFrame("0", TextStyle{Size: 16})
	frame.Shape.bc = &shapeButtonColors{opticalDigits: true}
	buttonAmendLayout(EventCtx{&frame, nil, w})
	if got := frame.Children[0].Shape.Y; got != 1 {
		t.Errorf("digit label moved by %v, want the figure offset 1", got)
	}

	// The same button without the flag reads the cap band, which is the
	// default for every other label.
	capFrame := bandTestFrame("Save", TextStyle{Size: 16})
	capFrame.Shape.bc = &shapeButtonColors{}
	buttonAmendLayout(EventCtx{&capFrame, nil, w})
	if got := capFrame.Children[0].Shape.Y; got != 2 {
		t.Errorf("label moved by %v, want the cap offset 2", got)
	}
}
