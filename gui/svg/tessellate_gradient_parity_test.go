package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// These tests mirror gui/canvas_gradient_test.go. The two gradient
// tessellators are kept in sync by hand (see the header comment on
// tessellate_gradient.go), so the properties they are held to are the
// same ones.

// svgTriWinding reports how many triangles in a flat list wind each way.
func svgTriWinding(tris []float32) (ccw, cw int) {
	for i := 0; i+5 < len(tris); i += 6 {
		d := (tris[i+2]-tris[i])*(tris[i+5]-tris[i+1]) -
			(tris[i+4]-tris[i])*(tris[i+3]-tris[i+1])
		if d < 0 {
			cw++
		} else {
			ccw++
		}
	}
	return ccw, cw
}

func svgAlmostEq(a, b, tol float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// svgRampStops returns n evenly spaced stops, enough of them that every
// triangle gets cut repeatedly.
func svgRampStops(n int) []gui.SvgGradientStop {
	stops := make([]gui.SvgGradientStop, n)
	for i := range stops {
		stops[i] = gui.SvgGradientStop{
			Offset: float32(i) / float32(n-1),
			Color:  gui.SvgColor{R: 255, G: 200, B: 80, A: uint8(255 - i*20)},
		}
	}
	return stops
}

// TestSvgGradientSubdivisionPreservesWinding is the regression test for
// issue #399's item 2: the isoline splitter sorts a triangle's vertices
// by gradient parameter, and an odd permutation reverses the winding.
// gui/backend/soft takes a whole batch as one path and accumulates
// *signed* coverage, so a mesh of mixed winding cancels itself along
// every internal seam. Uniform winding in, uniform winding out.
func TestSvgGradientSubdivisionPreservesWinding(t *testing.T) {
	stops := svgRampStops(10)
	// A quad, both triangles counter-clockwise in SVG's y-down space.
	tris := []float32{
		0, 0, 200, 0, 200, 200,
		0, 0, 200, 200, 0, 200,
	}
	cases := []struct {
		name string
		grad gui.SvgGradientDef
	}{
		{"linear", gui.SvgGradientDef{
			X1: 0, Y1: 0, X2: 200, Y2: 200, Stops: stops,
		}},
		{"radial", gui.SvgGradientDef{
			IsRadial: true, CX: 100, CY: 100, FX: 100, FY: 100, R: 100,
			Stops: stops,
		}},
		{"reflect", gui.SvgGradientDef{
			X1: 80, Y1: 0, X2: 120, Y2: 0, Stops: stops,
			SpreadMethod: gui.SvgSpreadReflect,
		}},
		{"repeat", gui.SvgGradientDef{
			X1: 80, Y1: 0, X2: 120, Y2: 0, Stops: stops,
			SpreadMethod: gui.SvgSpreadRepeat,
		}},
	}
	for _, c := range cases {
		out := subdivideGradientTris(tris, c.grad)
		if len(out) <= len(tris) {
			t.Errorf("%s: subdivision produced %d floats, want more than "+
				"the %d it was given", c.name, len(out), len(tris))
		}
		ccw, cw := svgTriWinding(out)
		if cw != 0 {
			t.Errorf("%s: %d of %d triangles wound the wrong way",
				c.name, cw, ccw+cw)
		}
	}
}

// TestSvgGradientSubdivisionPreservesWindingReversed is the same claim
// from the other side: a clockwise input must stay clockwise, not be
// normalized to the splitter's preferred orientation.
func TestSvgGradientSubdivisionPreservesWindingReversed(t *testing.T) {
	grad := gui.SvgGradientDef{X1: 0, Y1: 0, X2: 200, Y2: 0,
		Stops: svgRampStops(8)}
	tris := []float32{0, 0, 200, 200, 200, 0}
	if _, cw := svgTriWinding(tris); cw != 1 {
		t.Fatalf("test input is not clockwise")
	}
	ccw, cw := svgTriWinding(subdivideGradientTris(tris, grad))
	if ccw != 0 {
		t.Errorf("%d of %d triangles flipped to counter-clockwise",
			ccw, ccw+cw)
	}
}

// TestSvgRadialFanCostsNoExtraSubdivision pins issue #399's item 3: a
// fan struck from the gradient's own center is already radially
// aligned, so the curvature criterion measures ~0 and leaves it alone.
// The edge-length rule this replaced split a 25px circle into 80,896
// triangles.
func TestSvgRadialFanCostsNoExtraSubdivision(t *testing.T) {
	const cx, cy, r = 100, 100, 25
	const segs = 64
	tris := make([]float32, 0, segs*6)
	for i := range segs {
		a0 := float64(i) / segs * 2 * math.Pi
		a1 := float64(i+1) / segs * 2 * math.Pi
		tris = append(tris,
			cx, cy,
			cx+r*float32(math.Cos(a0)), cy+r*float32(math.Sin(a0)),
			cx+r*float32(math.Cos(a1)), cy+r*float32(math.Sin(a1)))
	}
	grad := gui.SvgGradientDef{IsRadial: true, CX: cx, CY: cy,
		FX: cx, FY: cy, R: r, Stops: []gui.SvgGradientStop{
			{Offset: 0, Color: gui.SvgColor{R: 255, A: 255}},
			{Offset: 1, Color: gui.SvgColor{B: 255, A: 0}},
		}}
	got := len(subdivideRadialTris(tris, grad)) / 6
	if got != segs {
		t.Errorf("center-struck fan = %d triangles, want the %d it was "+
			"given", got, segs)
	}
}

// TestSvgRadialRectStillSubdivides is the other half: geometry that
// does bulge across the isolines must still refine, or the criterion
// would be free by doing nothing.
func TestSvgRadialRectStillSubdivides(t *testing.T) {
	tris := []float32{
		0, 0, 200, 0, 200, 200,
		0, 0, 200, 200, 0, 200,
	}
	grad := gui.SvgGradientDef{IsRadial: true, CX: 100, CY: 100,
		FX: 100, FY: 100, R: 100}
	got := len(subdivideRadialTris(tris, grad))
	if got <= len(tris) {
		t.Errorf("radially filled rect = %d floats, want more than the "+
			"%d it was given", got, len(tris))
	}
}

// TestSvgCutFractionRadialLandsOnIsoline pins why svgCutFraction solves
// a quadratic instead of interpolating. The radial parameter is a
// distance, so it is not affine along an edge: an interpolated cut
// misses the isoline, the recursion sees the piece still straddling it
// and cuts again, and neighbours that cut at different places leave a
// seam.
func TestSvgCutFractionRadialLandsOnIsoline(t *testing.T) {
	g := gui.SvgGradientDef{IsRadial: true, CX: 0, CY: 0, FX: 0, FY: 0,
		R: 100}
	// A chord well off-center, where distance is at its least linear.
	x0, y0 := float32(-80), float32(60)
	x1, y1 := float32(80), float32(60)
	t0 := svgRawT(x0, y0, &g)
	t1 := svgRawT(x1, y1, &g)
	const tS = 0.7
	f := svgCutFraction(x0, y0, x1, y1, t0, t1, tS, &g)
	cx := x0 + f*(x1-x0)
	cy := y0 + f*(y1-y0)
	if got := svgRawT(cx, cy, &g); !svgAlmostEq(got, tS, 1e-4) {
		t.Errorf("cut at f=%v has t=%v, want %v", f, got, tS)
	}
}

// TestSvgCutFractionLinearIsExact is the other half: for a linear
// gradient the parameter is affine, so the plain ratio is already exact
// and the quadratic path must not be taken.
func TestSvgCutFractionLinearIsExact(t *testing.T) {
	g := gui.SvgGradientDef{X1: 0, Y1: 0, X2: 100, Y2: 0}
	if f := svgCutFraction(0, 0, 100, 0, 0, 1, 0.25, &g); !svgAlmostEq(f, 0.25, 1e-6) {
		t.Errorf("linear cut fraction = %v, want 0.25", f)
	}
	// A zero-length segment has no answer; the midpoint is the
	// convention, and it must not divide by zero.
	if f := svgCutFraction(5, 5, 5, 5, 0.3, 0.3, 0.5, &g); f != 0.5 {
		t.Errorf("degenerate segment cut fraction = %v, want 0.5", f)
	}
}

func TestSvgStopIsolinesPad(t *testing.T) {
	stops := []gui.SvgGradientStop{
		{Offset: 0}, {Offset: 0.25}, {Offset: 0.5}, {Offset: 1},
	}
	// Geometry running past both ends: the clamp boundaries are real
	// breaks in the ramp, and the two interior stops are too. Stops
	// sitting on 0 and 1 must not be repeated.
	got := svgStopIsolines(stops, gui.SvgSpreadPad, -0.5, 1.5, nil)
	want := []float32{0, 0.25, 0.5, 1}
	if len(got) != len(want) {
		t.Fatalf("isolines = %v, want %v", got, want)
	}
	for i := range want {
		if !svgAlmostEq(got[i], want[i], 1e-6) {
			t.Fatalf("isolines = %v, want %v", got, want)
		}
	}
	// Geometry entirely inside the ramp has no clamp boundary to split
	// at.
	got = svgStopIsolines(stops, gui.SvgSpreadPad, 0.1, 0.9, nil)
	if len(got) != 2 {
		t.Errorf("interior-only isolines = %v, want the 2 interior stops",
			got)
	}
}

func TestSvgStopIsolinesTiling(t *testing.T) {
	stops := []gui.SvgGradientStop{{Offset: 0}, {Offset: 0.25}, {Offset: 1}}
	// Three periods of geometry: each contributes its own period
	// boundary and its own copy of the interior stop.
	rep := svgStopIsolines(stops, gui.SvgSpreadRepeat, 0.5, 3.5, nil)
	wantRep := []float32{1, 1.25, 2, 2.25, 3, 3.25}
	if len(rep) != len(wantRep) {
		t.Fatalf("repeat isolines = %v, want %v", rep, wantRep)
	}
	for i := range wantRep {
		if !svgAlmostEq(rep[i], wantRep[i], 1e-6) {
			t.Fatalf("repeat isolines = %v, want %v", rep, wantRep)
		}
	}
	// Reflect mirrors on odd periods, so the stop at 0.25 lands at 1.75
	// in [1,2] — three quarters of the way through, because that period
	// runs backwards — and back at 2.25 in [2,3].
	ref := svgStopIsolines(stops, gui.SvgSpreadReflect, 0.5, 2.5, nil)
	wantRef := []float32{1, 1.75, 2, 2.25}
	if len(ref) != len(wantRef) {
		t.Fatalf("reflect isolines = %v, want %v", ref, wantRef)
	}
	for i := range wantRef {
		if !svgAlmostEq(ref[i], wantRef[i], 1e-6) {
			t.Fatalf("reflect isolines = %v, want %v", ref, wantRef)
		}
	}
}

// TestSvgStopIsolinesCapped bounds the tiling branch: a gradient whose
// endpoints are a hair apart against geometry of ordinary size spans a
// vast number of periods, and the split pass rescans this list at every
// node of its recursion.
func TestSvgStopIsolinesCapped(t *testing.T) {
	stops := make([]gui.SvgGradientStop, 64)
	for i := range stops {
		stops[i].Offset = float32(i) / float32(len(stops)-1)
	}
	got := svgStopIsolines(stops, gui.SvgSpreadRepeat, 0, 1e9, nil)
	if len(got) > maxSvgStopIsolines {
		t.Errorf("isolines = %d, want at most %d", len(got),
			maxSvgStopIsolines)
	}
}

// TestSvgStopIsolinesNonFinite rejects a range the projection could not
// make sense of, rather than looping on it.
func TestSvgStopIsolinesNonFinite(t *testing.T) {
	stops := []gui.SvgGradientStop{{Offset: 0}, {Offset: 0.5}, {Offset: 1}}
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, c := range [][2]float32{{nan, 1}, {0, nan}, {0, inf}, {1, 0}} {
		for _, sp := range []gui.SvgGradientSpread{
			gui.SvgSpreadPad, gui.SvgSpreadReflect, gui.SvgSpreadRepeat,
		} {
			if got := svgStopIsolines(stops, sp, c[0], c[1], nil); len(got) != 0 {
				t.Errorf("range %v spread %v: isolines = %v, want none",
					c, sp, got)
			}
		}
	}
}

// TestSplitTriAtStopsStopsAtBudget exercises the batch budget: once the
// batch has produced more geometry than any fill can use, the triangle
// in flight is emitted whole rather than split further.
func TestSplitTriAtStopsStopsAtBudget(t *testing.T) {
	stops := []gui.SvgGradientStop{
		{Offset: 0}, {Offset: 0.5}, {Offset: 1},
	}
	g := gui.SvgGradientDef{X1: 0, Y1: 0, X2: 100, Y2: 0, Stops: stops}
	iso := svgStopIsolines(stops, gui.SvgSpreadPad, 0, 1, nil)
	if len(iso) == 0 {
		t.Fatal("no isolines; the triangle below would not split anyway")
	}
	var fresh []float32
	splitTriAtStops(0, 0, 100, 0, 0, 100, &g, iso, 0, &fresh)
	if len(fresh) <= 6 {
		t.Fatalf("split below budget produced %d floats, want > 6",
			len(fresh))
	}
	full := make([]float32, maxSvgSplitFloats)
	splitTriAtStops(0, 0, 100, 0, 0, 100, &g, iso, 0, &full)
	if got := len(full) - maxSvgSplitFloats; got != 6 {
		t.Errorf("split at budget appended %d floats, want 6 (unsplit)",
			got)
	}
}

// TestSubdivideGradientComposesBounded is the composition guard: the
// radial pass quadruples the batch per level and the isoline pass then
// splits every leaf up to three ways per cut. Each cap alone bounds one
// factor; this pins their product for the worst input a real asset
// produces.
func TestSubdivideGradientComposesBounded(t *testing.T) {
	tris := []float32{
		0, 0, 400, 0, 400, 400,
		0, 0, 400, 400, 0, 400,
	}
	stops := make([]gui.SvgGradientStop, 200)
	for i := range stops {
		stops[i] = gui.SvgGradientStop{
			Offset: float32(i) / float32(len(stops)-1),
			Color:  gui.SvgColor{R: uint8(i), A: 255},
		}
	}
	grad := gui.SvgGradientDef{IsRadial: true, CX: 200, CY: 200,
		FX: 200, FY: 200, R: 200, Stops: stops}
	out := subdivideGradientTris(tris, grad)
	if len(out)%6 != 0 {
		t.Errorf("subdivided length %d is not a whole number of triangles",
			len(out))
	}
	if len(out) > maxSvgSplitFloats*2 {
		t.Errorf("subdivided to %d floats, past any bound the two passes "+
			"should compose to", len(out))
	}
	if _, cw := svgTriWinding(out); cw != 0 {
		t.Errorf("%d triangles wound the wrong way", cw)
	}
}

// TestSubdivideGradientSplitsRadialAtStops pins the behavior change
// that came with the port: before it, a radial gradient reached only
// the geometric pass and was never cut at its stops, so a hard color
// break inside a radial ramp had nowhere to land.
func TestSubdivideGradientSplitsRadialAtStops(t *testing.T) {
	// A center-struck fan, fine enough that its rim chords carry no
	// curvature error either: the geometric pass leaves it alone, so
	// any growth here is the isoline pass and nothing else.
	const cx, cy, r = 100, 100, 50
	const segs = 64
	tris := make([]float32, 0, segs*6)
	for i := range segs {
		a0 := float64(i) / segs * 2 * math.Pi
		a1 := float64(i+1) / segs * 2 * math.Pi
		tris = append(tris,
			cx, cy,
			cx+r*float32(math.Cos(a0)), cy+r*float32(math.Sin(a0)),
			cx+r*float32(math.Cos(a1)), cy+r*float32(math.Sin(a1)))
	}
	grad := gui.SvgGradientDef{IsRadial: true, CX: cx, CY: cy,
		FX: cx, FY: cy, R: r, Stops: []gui.SvgGradientStop{
			{Offset: 0, Color: gui.SvgColor{R: 255, A: 255}},
			{Offset: 0.5, Color: gui.SvgColor{G: 255, A: 255}},
			{Offset: 1, Color: gui.SvgColor{B: 255, A: 255}},
		}}
	if len(subdivideRadialTris(tris, grad)) != len(tris) {
		t.Fatal("geometric pass moved the fan; the count below would not " +
			"isolate the isoline pass")
	}
	out := subdivideGradientTris(tris, grad)
	if len(out) <= len(tris) {
		t.Errorf("radial with an interior stop = %d floats, want more "+
			"than the %d it was given", len(out), len(tris))
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
