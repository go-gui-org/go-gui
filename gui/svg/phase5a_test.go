package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestPhase5aTranslateValuesParsed — values="12 12;0 0" produces
// SvgAnimTranslate with 4 interleaved floats.
func TestPhase5aTranslateValuesParsed(t *testing.T) {
	elem := `<animateTransform type="translate" dur="1s" ` +
		`values="12 12;0 0"/>`
	a, ok := parseAnimateTransformElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.Kind != gui.SvgAnimTranslate {
		t.Fatalf("want SvgAnimTranslate, got %d", a.Kind)
	}
	want := []float32{12, 12, 0, 0}
	if len(a.Values) != len(want) {
		t.Fatalf("len want %d, got %d (%v)", len(want), len(a.Values), a.Values)
	}
	for i, v := range want {
		if a.Values[i] != v {
			t.Fatalf("idx %d: want %g got %g", i, v, a.Values[i])
		}
	}
}

// TestPhase5aScaleUniformNormalized — values="0;1" (uniform) becomes
// interleaved [0,0,1,1].
func TestPhase5aScaleUniformNormalized(t *testing.T) {
	elem := `<animateTransform type="scale" dur="1s" values="0;1"/>`
	a, ok := parseAnimateTransformElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if a.Kind != gui.SvgAnimScale {
		t.Fatalf("want SvgAnimScale, got %d", a.Kind)
	}
	want := []float32{0, 0, 1, 1}
	if len(a.Values) != len(want) {
		t.Fatalf("len want %d, got %d (%v)", len(want),
			len(a.Values), a.Values)
	}
	for i, v := range want {
		if a.Values[i] != v {
			t.Fatalf("idx %d: want %g got %g", i, v, a.Values[i])
		}
	}
}

// TestPhase5aScaleNonUniform — "0.5 1;1 1" stays as the supplied
// pairs, no normalization.
func TestPhase5aScaleNonUniform(t *testing.T) {
	elem := `<animateTransform type="scale" dur="1s" ` +
		`values="0.5 1;1 1"/>`
	a, ok := parseAnimateTransformElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	want := []float32{0.5, 1, 1, 1}
	for i, v := range want {
		if a.Values[i] != v {
			t.Fatalf("idx %d: want %g got %g", i, v, a.Values[i])
		}
	}
}

// TestPhase5aTranslateFromTo — `from="10 20" to="0 0"` form lowers
// to the same 4-float layout.
func TestPhase5aTranslateFromTo(t *testing.T) {
	elem := `<animateTransform type="translate" dur="1s" ` +
		`from="10 20" to="0 0"/>`
	a, ok := parseAnimateTransformElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	want := []float32{10, 20, 0, 0}
	if len(a.Values) != 4 {
		t.Fatalf("want 4 values, got %d", len(a.Values))
	}
	for i, v := range want {
		if a.Values[i] != v {
			t.Fatalf("idx %d: want %g got %g", i, v, a.Values[i])
		}
	}
}

// TestPhase5aSplineAppliesToTranslate — calcMode="spline" +
// keySplines are recorded on a translate animation.
func TestPhase5aSplineAppliesToTranslate(t *testing.T) {
	elem := `<animateTransform type="translate" calcMode="spline" ` +
		`dur="1.2s" values="12 12;0 0" ` +
		`keySplines=".52,.6,.25,.99"/>`
	a, ok := parseAnimateTransformElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatal("parse failed")
	}
	if len(a.KeySplines) != 4 {
		t.Fatalf("want 4 spline floats, got %d", len(a.KeySplines))
	}
}

// TestPhase5bResetsPlaceholderTransform — a path with a scale(0)
// placeholder transform that is animated must tessellate at its
// local (pre-transform) coords so the render-time sandwich can
// compose the animation over the decomposed base. The emitted
// TessellatedPath carries the decomposed base (BaseScaleX=0,
// BaseTransX=12) for the animation sandwich to consume.
// Regression guard for the pulse-ring "paths=0" bug.
func TestPhase5bResetsPlaceholderTransform(t *testing.T) {
	asset := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">` +
		`<circle cx="12" cy="12" r="10" ` +
		`transform="translate(12, 12) scale(0)">` +
		`<animateTransform attributeName="transform" type="translate" ` +
		`dur="1s" values="12 12;0 0"/>` +
		`<animateTransform attributeName="transform" type="scale" ` +
		`dur="1s" values="0;1"/>` +
		`</circle></svg>`
	p := New()
	parsed, err := p.ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(parsed.Paths))
	}
	tris := p.Tessellate(parsed, 1)
	if len(tris) == 0 || len(tris[0].Triangles) == 0 {
		t.Fatal("tessellation produced no triangles — placeholder " +
			"transform was not reset")
	}
	// With the reset, the circle tessellates in its own 0..24
	// coord space; bbox must span something larger than a point.
	minX, maxX := float32(1e30), float32(-1e30)
	for i := 0; i < len(tris[0].Triangles); i += 2 {
		x := tris[0].Triangles[i]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
	}
	if maxX-minX < 10 {
		t.Fatalf("tessellation collapsed (span=%g); placeholder "+
			"transform leaked into vertices", maxX-minX)
	}
}

// TestPhase5aEndToEndPulseRing — parsing pulse-ring.svg yields a
// translate + scale + opacity animation all pointed at the same
// GroupID.
func TestPhase5aEndToEndPulseRing(t *testing.T) {
	asset := `<svg fill="currentColor" viewBox="0 0 24 24" ` +
		`xmlns="http://www.w3.org/2000/svg">` +
		`<path d="M12,1A11,11,0,1,0,23,12,11,11,0,0,0,12,1Z" ` +
		`transform="translate(12, 12) scale(0)">` +
		`<animateTransform attributeName="transform" ` +
		`calcMode="spline" type="translate" dur="1.2s" ` +
		`values="12 12;0 0" keySplines=".52,.6,.25,.99" ` +
		`repeatCount="indefinite"/>` +
		`<animateTransform attributeName="transform" ` +
		`calcMode="spline" additive="sum" type="scale" dur="1.2s" ` +
		`values="0;1" keySplines=".52,.6,.25,.99" ` +
		`repeatCount="indefinite"/>` +
		`<animate attributeName="opacity" calcMode="spline" ` +
		`dur="1.2s" values="1;0" keySplines=".52,.6,.25,.99" ` +
		`repeatCount="indefinite"/>` +
		`</path></svg>`
	parsed, err := New().ParseSvg(asset)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Animations) != 3 {
		t.Fatalf("want 3 animations, got %d", len(parsed.Animations))
	}
	kinds := map[gui.SvgAnimKind]int{}
	var groupID string
	for i, a := range parsed.Animations {
		kinds[a.Kind]++
		if i == 0 {
			groupID = a.GroupID
		} else if a.GroupID != groupID {
			t.Fatalf("anim[%d] GroupID %q != first %q",
				i, a.GroupID, groupID)
		}
	}
	if kinds[gui.SvgAnimTranslate] != 1 ||
		kinds[gui.SvgAnimScale] != 1 ||
		kinds[gui.SvgAnimOpacity] != 1 {
		t.Fatalf("unexpected kind mix: %+v", kinds)
	}
	if groupID == "" {
		t.Fatal("expected shared GroupID across the three animations")
	}
}

// TestParsePairedValuesCapsAtMaxKeyframes — oversized paired
// values list is truncated; output length at most 2*maxKeyframes.
func TestParsePairedValuesCapsAtMaxKeyframes(t *testing.T) {
	s := strings.Repeat("1 2;", maxKeyframes+50)
	got := parsePairedValues(s)
	if len(got) > 2*maxKeyframes {
		t.Fatalf("want len<=%d, got %d", 2*maxKeyframes, len(got))
	}
}
