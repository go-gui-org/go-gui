package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/gradmesh"
)

// The tessellation itself is tested once, in gui/internal/gradmesh.
// What is left on this side is the adapter: resolving objectBoundingBox
// units, and converting an SvgGradientDef into gradmesh's parameters.
// A mistake there is the failure unification makes possible, and the
// only one these tests exist to catch.

// TestGradParamsCarriesGeometry pins every field's destination. A
// transposed pair here would move gradients without moving a single
// gradmesh test.
func TestGradParamsCarriesGeometry(t *testing.T) {
	g := gui.SvgGradientDef{
		X1: 1, Y1: 2, X2: 3, Y2: 4,
		CX: 5, CY: 6, R: 7, FX: 8, FY: 9,
		Stops: []gui.SvgGradientStop{
			{Offset: 0.25}, {Offset: 0.5}, {Offset: 0.75},
		},
	}
	var offsets []float32
	p := gradParams(&g, &offsets)
	want := gradmesh.Params{
		X1: 1, Y1: 2, X2: 3, Y2: 4,
		CX: 5, CY: 6, R: 7, FX: 8, FY: 9,
		StopOffsets: []float32{0.25, 0.5, 0.75},
	}
	if p.X1 != want.X1 || p.Y1 != want.Y1 || p.X2 != want.X2 ||
		p.Y2 != want.Y2 || p.CX != want.CX || p.CY != want.CY ||
		p.R != want.R || p.FX != want.FX || p.FY != want.FY {
		t.Errorf("geometry = %+v, want %+v", p, want)
	}
	if p.Radial {
		t.Error("Radial set from a linear def")
	}
	if len(p.StopOffsets) != len(want.StopOffsets) {
		t.Fatalf("offsets = %v, want %v", p.StopOffsets, want.StopOffsets)
	}
	// Order matters: the isoline pass mirrors an offset by its index's
	// period, so a reordered list cuts in the wrong places.
	for i := range want.StopOffsets {
		if p.StopOffsets[i] != want.StopOffsets[i] {
			t.Fatalf("offsets = %v, want %v", p.StopOffsets, want.StopOffsets)
		}
	}
	rad := gui.SvgGradientDef{IsRadial: true, R: 10}
	if !gradParams(&rad, nil).Radial {
		t.Error("IsRadial did not reach Params.Radial")
	}
	// A nil destination is the projection-only case: no offsets, no
	// allocation, and the geometry still lands.
	if got := gradParams(&g, nil); got.StopOffsets != nil {
		t.Errorf("nil destination produced offsets %v", got.StopOffsets)
	}
}

// TestGradSpreadMapping covers the one place the two enums could drift,
// including the arm SVG has and gradmesh does not: anything that is not
// reflect or repeat pads, which is what applySpread's default does too.
func TestGradSpreadMapping(t *testing.T) {
	cases := []struct {
		in   gui.SvgGradientSpread
		want gradmesh.Spread
	}{
		{gui.SvgSpreadPad, gradmesh.SpreadPad},
		{gui.SvgSpreadReflect, gradmesh.SpreadReflect},
		{gui.SvgSpreadRepeat, gradmesh.SpreadRepeat},
		{gui.SvgGradientSpread(99), gradmesh.SpreadPad},
	}
	for _, c := range cases {
		if got := gradSpread(c.in); got != c.want {
			t.Errorf("gradSpread(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	// An unrecognized spread must pad in the coloring pass as well, or
	// the cuts and the colors would disagree about the ramp.
	if got := applySpread(2, gui.SvgGradientSpread(99)); got != 1 {
		t.Errorf("applySpread(2, unknown) = %v, want 1 (pad)", got)
	}
}

// TestSubdivideGradientTrisShortInput keeps this side's own guard: less
// than one full triangle is nothing to fill.
func TestSubdivideGradientTrisShortInput(t *testing.T) {
	g := gui.SvgGradientDef{IsRadial: true, R: 50}
	for _, in := range [][]float32{nil, {}, {0, 0}, {0, 0, 1, 0}} {
		if got := subdivideGradientTris(in, g); len(got) != 0 {
			t.Errorf("input %v: got %d floats, want 0", in, len(got))
		}
	}
}

// TestResolveGradientCarriesSpread pins what the objectBoundingBox
// rewrite must pass through. Only the geometry is in user space; the
// stops and the spread method describe the ramp, not where it sits.
//
// Dropping the spread here is silent and total: objectBoundingBox is
// the default units, so every gradient that did not spell out
// gradientUnits padded no matter what spreadMethod it asked for.
func TestResolveGradientCarriesSpread(t *testing.T) {
	stops := []gui.SvgGradientStop{{Offset: 0}, {Offset: 1}}
	for _, sp := range []gui.SvgGradientSpread{
		gui.SvgSpreadPad, gui.SvgSpreadReflect, gui.SvgSpreadRepeat,
	} {
		lin := resolveGradient(gui.SvgGradientDef{
			X1: 0, Y1: 0, X2: 1, Y2: 1, Stops: stops, SpreadMethod: sp,
		}, 0, 0, 100, 100)
		if lin.SpreadMethod != sp {
			t.Errorf("linear: spread %v became %v", sp, lin.SpreadMethod)
		}
		if len(lin.Stops) != len(stops) {
			t.Errorf("linear: %d stops survived, want %d",
				len(lin.Stops), len(stops))
		}
		rad := resolveGradient(gui.SvgGradientDef{
			IsRadial: true, CX: 0.5, CY: 0.5, R: 0.5,
			Stops: stops, SpreadMethod: sp,
		}, 0, 0, 100, 100)
		if rad.SpreadMethod != sp {
			t.Errorf("radial: spread %v became %v", sp, rad.SpreadMethod)
		}
	}
}
