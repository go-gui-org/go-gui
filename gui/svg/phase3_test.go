package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestPhase3SplineParsingAnimateAttribute verifies keySplines is
// captured when calcMode="spline" on attribute animation.
func TestPhase3SplineParsingAnimateAttribute(t *testing.T) {
	elem := `<animate attributeName="cy" calcMode="spline" dur="0.6s" ` +
		`values="12;6;12" ` +
		`keySplines=".33,.66,.66,1;.33,0,.66,.33"/>`
	a, ok := parseAnimateAttributeElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if len(a.KeySplines) != 8 {
		t.Fatalf("want 8 floats (2 segments), got %d", len(a.KeySplines))
	}
	want := []float32{.33, .66, .66, 1, .33, 0, .66, .33}
	for i, v := range want {
		if a.KeySplines[i] != v {
			t.Fatalf("idx %d: want %g got %g", i, v, a.KeySplines[i])
		}
	}
}

// TestPhase3NonSplineCalcModeIgnoresKeySplines confirms keySplines
// is ignored when calcMode != "spline". Linear stays linear.
func TestPhase3NonSplineCalcModeIgnoresKeySplines(t *testing.T) {
	elem := `<animate attributeName="cy" calcMode="linear" dur="0.6s" ` +
		`values="12;6;12" keySplines=".33,.66,.66,1;.33,0,.66,.33"/>`
	a, ok := parseAnimateAttributeElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.KeySplines != nil {
		t.Fatalf("want nil splines for calcMode=linear, got %v",
			a.KeySplines)
	}
}

// TestPhase3MissingCalcModeIgnoresKeySplines — default calcMode is
// linear per SMIL; keySplines must be ignored when calcMode absent.
func TestPhase3MissingCalcModeIgnoresKeySplines(t *testing.T) {
	elem := `<animate attributeName="cy" dur="0.6s" ` +
		`values="12;6;12" keySplines=".33,.66,.66,1;.33,0,.66,.33"/>`
	a, ok := parseAnimateAttributeElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.KeySplines != nil {
		t.Fatal("want nil splines when calcMode missing")
	}
}

// TestPhase3SegmentCountMismatchDropsSplines falls back to linear
// when keySplines count doesn't match segment count.
func TestPhase3SegmentCountMismatchDropsSplines(t *testing.T) {
	// 3 values → 2 segments; only 1 spline supplied.
	elem := `<animate attributeName="cy" calcMode="spline" dur="0.6s" ` +
		`values="12;6;12" keySplines=".33,.66,.66,1"/>`
	a, ok := parseAnimateAttributeElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.KeySplines != nil {
		t.Fatalf("want nil splines on count mismatch, got %v",
			a.KeySplines)
	}
}

// TestPhase3SplineParsingOpacity confirms the opacity parse path
// also reads keySplines.
func TestPhase3SplineParsingOpacity(t *testing.T) {
	elem := `<animate attributeName="opacity" calcMode="spline" ` +
		`dur="1s" values="0;1;0" ` +
		`keySplines=".4,0,.6,1;.4,0,.6,1"/>`
	a, ok := parseAnimateElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if len(a.KeySplines) != 8 {
		t.Fatalf("want 8 floats, got %d", len(a.KeySplines))
	}
}

// TestPhase3SplineParsingRotate confirms animateTransform rotate
// also picks up keySplines.
func TestPhase3SplineParsingRotate(t *testing.T) {
	elem := `<animateTransform type="rotate" calcMode="spline" ` +
		`dur="1s" values="0 12 12;180 12 12;360 12 12" ` +
		`keySplines=".4,0,.6,1;.4,0,.6,1"/>`
	a, ok := parseAnimateTransformElement(elem,
		ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if len(a.KeySplines) != 8 {
		t.Fatalf("want 8 floats, got %d", len(a.KeySplines))
	}
}

// TestPhase3SpaceSeparatedKeySplines accepts space-only separators
// within a tuple (SVG allows comma-or-whitespace).
func TestPhase3SpaceSeparatedKeySplines(t *testing.T) {
	elem := `<animate attributeName="cy" calcMode="spline" dur="0.6s" ` +
		`values="12;6" keySplines=".33 .66 .66 1"/>`
	a, ok := parseAnimateAttributeElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if len(a.KeySplines) != 4 {
		t.Fatalf("want 4 floats, got %d", len(a.KeySplines))
	}
	if a.KeySplines[0] != .33 || a.KeySplines[3] != 1 {
		t.Fatalf("unexpected values: %v", a.KeySplines)
	}
	_ = gui.SvgAnimAttr // keep import live
}

// TestParseKeySplinesRejectsOversizedSegs — an animation claiming
// more inter-keyframe segments than maxKeyframes must produce nil
// splines (falls back to linear) rather than allocating a huge
// control-point slice.
func TestParseKeySplinesRejectsOversizedSegs(t *testing.T) {
	raw := strings.Repeat(".33,.66,.66,1;", maxKeyframes+10)
	elem := `<animate calcMode="spline" keySplines="` + raw + `"/>`
	// nVals = maxKeyframes+11 → segs = maxKeyframes+10 > cap.
	got := parseKeySplinesIfSpline(elem, maxKeyframes+11)
	if got != nil {
		t.Fatalf("want nil for segs>maxKeyframes, got len=%d", len(got))
	}
}
