package gradmesh

import (
	"math"
	"testing"
	"time"
)

// These tests were the two suites the duplicated tessellators carried
// (gui/canvas_gradient_test.go and gui/svg/tessellate_gradient_parity_test.go).
// They held the same properties in two vocabularies; here they hold them
// once, over the one implementation. Each side keeps the tests for its
// own adapter and coloring pass.

func almostEq(a, b, tol float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// twoStops is the cheapest ramp: nothing breaks between its ends.
func twoStops() []float32 { return []float32{0, 1} }

// rampOffsets returns n evenly spaced stop offsets, enough of them that
// every triangle gets cut repeatedly.
func rampOffsets(n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(i) / float32(n-1)
	}
	return out
}

// triWinding reports how many triangles in a flat list wind each way.
func triWinding(tris []float32) (ccw, cw int) {
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

// centerStruckFan builds a fan of segs wedges around (cx,cy). It is the
// geometry the curvature criterion must leave alone.
func centerStruckFan(cx, cy, r float32, segs int) []float32 {
	tris := make([]float32, 0, segs*6)
	for i := range segs {
		a0 := float64(i) / float64(segs) * 2 * math.Pi
		a1 := float64(i+1) / float64(segs) * 2 * math.Pi
		tris = append(tris,
			cx, cy,
			cx+r*float32(math.Cos(a0)), cy+r*float32(math.Sin(a0)),
			cx+r*float32(math.Cos(a1)), cy+r*float32(math.Sin(a1)))
	}
	return tris
}

// subdivide runs Subdivide with fresh buffers, the shape most tests want.
func subdivide(tris []float32, p *Params) []float32 {
	var split, radial, isolines []float32
	return Subdivide(tris, p, &split, &radial, &isolines)
}

func TestRawTLinear(t *testing.T) {
	p := &Params{X1: 0, Y1: 0, X2: 100, Y2: 0}
	cases := []struct {
		x, y float32
		want float32
	}{
		{0, 0, 0},
		{50, 0, 0.5},
		{100, 0, 1},
		{150, 0, 1.5},  // raw t is unclamped; spread decides
		{-50, 0, -0.5}, //
		{50, 999, 0.5}, // perpendicular offset does not move t
	}
	for _, c := range cases {
		if got := RawT(c.x, c.y, p); !almostEq(got, c.want, 1e-5) {
			t.Errorf("RawT(%v,%v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestRawTDegenerateLinear(t *testing.T) {
	// Coincident endpoints have no direction; t must fold to 0 rather
	// than produce a NaN that would poison every vertex color.
	p := &Params{X1: 5, Y1: 5, X2: 5, Y2: 5}
	if got := RawT(99, 99, p); got != 0 {
		t.Errorf("degenerate linear t = %v, want 0", got)
	}
}

func TestRawTRadial(t *testing.T) {
	p := &Params{Radial: true, CX: 10, CY: 10, FX: 10, FY: 10, R: 20}
	if got := RawT(10, 10, p); got != 0 {
		t.Errorf("center t = %v, want 0", got)
	}
	if got := RawT(30, 10, p); !almostEq(got, 1, 1e-5) {
		t.Errorf("rim t = %v, want 1", got)
	}
	if got := RawT(50, 10, p); !almostEq(got, 2, 1e-5) {
		t.Errorf("outside t = %v, want 2 (raw, unclamped)", got)
	}
}

func TestRawTRadialFocal(t *testing.T) {
	// The focal point, not the center, is where t == 0.
	p := &Params{Radial: true, CX: 0, CY: 0, FX: 5, FY: 0, R: 10}
	if got := RawT(5, 0, p); got != 0 {
		t.Errorf("focal t = %v, want 0", got)
	}
	if got := RawT(0, 0, p); !almostEq(got, 0.5, 1e-5) {
		t.Errorf("center t = %v, want 0.5", got)
	}
}

func TestRawTBadRadius(t *testing.T) {
	for _, r := range []float32{0, -1,
		float32(math.NaN()), float32(math.Inf(1))} {
		p := &Params{Radial: true, R: r}
		if got := RawT(3, 4, p); got != 0 {
			t.Errorf("R=%v: t = %v, want 0", r, got)
		}
	}
}

func TestApplySpread(t *testing.T) {
	cases := []struct {
		name   string
		spread Spread
		in     float32
		want   float32
	}{
		{"pad below", SpreadPad, -0.5, 0},
		{"pad inside", SpreadPad, 0.25, 0.25},
		{"pad above", SpreadPad, 2, 1},
		{"repeat wraps", SpreadRepeat, 1.25, 0.25},
		{"repeat negative", SpreadRepeat, -0.25, 0.75},
		{"reflect even period", SpreadReflect, 0.25, 0.25},
		{"reflect odd period", SpreadReflect, 1.25, 0.75},
		// Reflect is an even function: -0.25 folds onto +0.25.
		{"reflect negative", SpreadReflect, -0.25, 0.25},
	}
	for _, c := range cases {
		got := ApplySpread(c.in, c.spread)
		if !almostEq(got, c.want, 1e-5) {
			t.Errorf("%s: ApplySpread(%v) = %v, want %v",
				c.name, c.in, got, c.want)
		}
	}
	for _, bad := range []float32{
		float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		if got := ApplySpread(bad, SpreadReflect); got != 0 {
			t.Errorf("non-finite %v: got %v, want 0", bad, got)
		}
	}
}

// TestApplySpreadHugeParameterIsFinite covers the ±2^31 clamp: past the
// int64 range the reflect parity test would be implementation-defined.
func TestApplySpreadHugeParameterIsFinite(t *testing.T) {
	const huge = float32(1e30)
	for _, sp := range []Spread{SpreadReflect, SpreadRepeat} {
		got := ApplySpread(huge, sp)
		if got < 0 || got > 1 || got != got {
			t.Errorf("spread %v: ApplySpread(%v) = %v, want in [0,1]",
				sp, huge, got)
		}
	}
}

func TestStopIsolinesPad(t *testing.T) {
	offsets := []float32{0, 0.3, 0.7, 1}
	// Geometry entirely inside [0,1]: only the interior stops break.
	got := stopIsolines(offsets, SpreadPad, 0, 1, nil)
	if len(got) != 2 || !almostEq(got[0], 0.3, 1e-6) ||
		!almostEq(got[1], 0.7, 1e-6) {
		t.Errorf("inside range isolines = %v, want [0.3 0.7]", got)
	}
	// Geometry overhanging both ends adds the two clamp boundaries,
	// which are real slope changes for pad.
	got = stopIsolines(offsets, SpreadPad, -1, 2, nil)
	if len(got) != 4 || got[0] != 0 || got[3] != 1 {
		t.Errorf("overhanging isolines = %v, want [0 0.3 0.7 1]", got)
	}
}

func TestStopIsolinesRepeatTilesPeriods(t *testing.T) {
	got := stopIsolines([]float32{0, 0.5, 1}, SpreadRepeat, 0, 3, nil)
	// Each period contributes its own breakpoints: the sawtooth's step
	// at every integer, plus the mid stop inside each period.
	want := []float32{0.5, 1, 1.5, 2, 2.5}
	if len(got) != len(want) {
		t.Fatalf("repeat isolines = %v, want %v", got, want)
	}
	for i := range want {
		if !almostEq(got[i], want[i], 1e-6) {
			t.Fatalf("repeat isolines = %v, want %v", got, want)
		}
	}
}

// TestStopIsolinesTilingNegative pins that the tiling walk handles
// negative k too: in the odd period [-3,-2) a stop at 0.25 sits at
// -2.25 — inside the range even though its period base (-3) is not. The
// fold at -2 is a break in both spreads.
func TestStopIsolinesTilingNegative(t *testing.T) {
	offsets := []float32{0, 0.25, 1}
	rep := stopIsolines(offsets, SpreadRepeat, -2.5, -1.5, nil)
	wantRep := []float32{-2, -1.75}
	if len(rep) != len(wantRep) {
		t.Fatalf("repeat isolines = %v, want %v", rep, wantRep)
	}
	for i := range wantRep {
		if !almostEq(rep[i], wantRep[i], 1e-6) {
			t.Fatalf("repeat isolines = %v, want %v", rep, wantRep)
		}
	}
	ref := stopIsolines(offsets, SpreadReflect, -2.5, -1.5, nil)
	wantRef := []float32{-2.25, -2, -1.75}
	if len(ref) != len(wantRef) {
		t.Fatalf("reflect isolines = %v, want %v", ref, wantRef)
	}
	for i := range wantRef {
		if !almostEq(ref[i], wantRef[i], 1e-6) {
			t.Fatalf("reflect isolines = %v, want %v", ref, wantRef)
		}
	}
}

// TestStopIsolinesNonFinite rejects a range the projection could not
// make sense of, rather than looping on it.
func TestStopIsolinesNonFinite(t *testing.T) {
	offsets := []float32{0, 0.5, 1}
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, c := range [][2]float32{{nan, 1}, {0, nan}, {0, inf}, {1, 0}} {
		for _, sp := range []Spread{
			SpreadPad, SpreadReflect, SpreadRepeat} {
			if got := stopIsolines(offsets, sp, c[0], c[1], nil); len(got) != 0 {
				t.Errorf("range %v spread %v: isolines = %v, want none",
					c, sp, got)
			}
		}
	}
}

// TestStopIsolinesTilingCapped bounds the tiling branch: a gradient
// whose endpoints are a hair apart against geometry of ordinary size
// spans a vast number of periods, and the split pass rescans this list
// at every node of its recursion.
func TestStopIsolinesTilingCapped(t *testing.T) {
	got := stopIsolines(rampOffsets(64), SpreadRepeat, 0, 1e9, nil)
	if len(got) > maxStopIsolines {
		t.Errorf("isolines = %d, want at most %d", len(got), maxStopIsolines)
	}
}

// TestStopIsolinesPadCapped covers the Pad branch's ceiling. It emits
// one breakpoint per interior stop, and the split pass rescans the
// whole list at every node of its recursion, so an unbounded stop count
// costs there and not only here.
func TestStopIsolinesPadCapped(t *testing.T) {
	offsets := make([]float32, 4000)
	for i := range offsets {
		offsets[i] = float32(i+1) / float32(len(offsets)+2)
	}
	got := stopIsolines(offsets, SpreadPad, 0, 1, nil)
	if len(got) > maxStopIsolines {
		t.Errorf("pad isolines = %d, want at most %d", len(got), maxStopIsolines)
	}
	if len(got) == 0 {
		t.Error("pad isolines = 0, want the cap's worth")
	}
}

// TestStopIsolinesHugeParameterTerminates pins the guard on the tiling
// loop. Before it the loop counted periods in a float64, and past 2^53
// that cannot hold consecutive integers: k++ stopped advancing and the
// loop never reached its bound. A gradient whose endpoints are a hair
// apart against geometry of ordinary size reaches those parameters —
// 100 units across a 1e-14 gradient is 1e16.
func TestStopIsolinesHugeParameterTerminates(t *testing.T) {
	done := make(chan int, 1)
	go func() {
		got := stopIsolines(twoStops(), SpreadRepeat, 1e16, 1e16+10, nil)
		done <- len(got)
	}()
	select {
	case n := <-done:
		if n > maxStopIsolines {
			t.Errorf("isolines = %d, want at most %d", n, maxStopIsolines)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stopIsolines did not terminate at 1e16")
	}
}

// TestCutFractionRadialLandsOnIsoline is why the radial cut solves a
// quadratic instead of interpolating. The parameter is a distance, so
// it is not affine along an edge: an interpolated cut misses the
// isoline, the recursion sees the piece still straddling it and cuts
// again, and neighbours that cut at different places leave a seam.
func TestCutFractionRadialLandsOnIsoline(t *testing.T) {
	p := &Params{Radial: true, CX: 0, CY: 0, FX: 0, FY: 0, R: 100}
	// A chord well off-center, where distance is at its least linear.
	x0, y0 := float32(-80), float32(60)
	x1, y1 := float32(80), float32(60)
	t0 := RawT(x0, y0, p)
	t1 := RawT(x1, y1, p)
	const tS = 0.7
	f := cutFraction(x0, y0, x1, y1, t0, t1, tS, p)
	cx := x0 + f*(x1-x0)
	cy := y0 + f*(y1-y0)
	if got := RawT(cx, cy, p); !almostEq(got, tS, 1e-4) {
		t.Errorf("cut at f=%v has t=%v, want %v", f, got, tS)
	}
}

// TestCutFractionLinearIsExact is the other half: for a linear gradient
// the parameter is affine, so the plain ratio is already exact and the
// quadratic path must not be taken.
func TestCutFractionLinearIsExact(t *testing.T) {
	p := &Params{X1: 0, Y1: 0, X2: 100, Y2: 0}
	f := cutFraction(0, 0, 100, 0, 0, 1, 0.25, p)
	if !almostEq(f, 0.25, 1e-6) {
		t.Errorf("linear cut fraction = %v, want 0.25", f)
	}
	// A zero-length segment has no answer; the midpoint is the
	// convention, and it must not divide by zero.
	if f = cutFraction(5, 5, 5, 5, 0.3, 0.3, 0.5, p); f != 0.5 {
		t.Errorf("degenerate segment cut fraction = %v, want 0.5", f)
	}
}

// TestSplitTriAtStopsStopsAtBudget exercises the batch budget: once the
// batch has produced more geometry than any fill can use, the triangle
// in flight is emitted whole rather than split further.
func TestSplitTriAtStopsStopsAtBudget(t *testing.T) {
	offsets := []float32{0, 0.5, 1}
	p := &Params{X1: 0, Y1: 0, X2: 100, Y2: 0, StopOffsets: offsets}
	iso := stopIsolines(offsets, SpreadPad, 0, 1, nil)
	if len(iso) == 0 {
		t.Fatal("no isolines; the triangle below would not split anyway")
	}
	// Below the budget the triangle splits.
	var fresh []float32
	splitTriAtStops(0, 0, 100, 0, 0, 100, p, iso, 0, &fresh)
	if len(fresh) <= 6 {
		t.Fatalf("split below budget produced %d floats, want > 6", len(fresh))
	}
	// At the budget the same triangle comes through whole.
	full := make([]float32, maxSplitFloats)
	splitTriAtStops(0, 0, 100, 0, 0, 100, p, iso, 0, &full)
	if got := len(full) - maxSplitFloats; got != 6 {
		t.Errorf("split at budget appended %d floats, want 6 (unsplit)", got)
	}
}

// TestSubdivisionPreservesWinding is the regression test for issue
// #399's item 2: the isoline splitter sorts a triangle's vertices by
// gradient parameter, and an odd permutation reverses the winding.
// gui/backend/soft takes a whole batch as one path and accumulates
// *signed* coverage, so a mesh of mixed winding cancels itself along
// every internal seam. Uniform winding in, uniform winding out.
func TestSubdivisionPreservesWinding(t *testing.T) {
	offsets := rampOffsets(10)
	// A quad, both triangles counter-clockwise in y-down space.
	tris := []float32{
		0, 0, 200, 0, 200, 200,
		0, 0, 200, 200, 0, 200,
	}
	cases := []struct {
		name string
		p    Params
	}{
		{"linear", Params{X1: 0, Y1: 0, X2: 200, Y2: 200}},
		{"radial", Params{Radial: true, CX: 100, CY: 100,
			FX: 100, FY: 100, R: 100}},
		{"reflect", Params{X1: 80, Y1: 0, X2: 120, Y2: 0,
			Spread: SpreadReflect}},
		{"repeat", Params{X1: 80, Y1: 0, X2: 120, Y2: 0,
			Spread: SpreadRepeat}},
	}
	for _, c := range cases {
		p := c.p
		p.StopOffsets = offsets
		out := subdivide(tris, &p)
		if len(out) <= len(tris) {
			t.Errorf("%s: subdivision produced %d floats, want more than "+
				"the %d it was given", c.name, len(out), len(tris))
		}
		ccw, cw := triWinding(out)
		if cw != 0 {
			t.Errorf("%s: %d of %d triangles wound the wrong way",
				c.name, cw, ccw+cw)
		}
	}
}

// TestSubdivisionPreservesWindingReversed is the same claim from the
// other side: a clockwise input must stay clockwise, not be normalized
// to the splitter's preferred orientation.
func TestSubdivisionPreservesWindingReversed(t *testing.T) {
	p := &Params{X1: 0, Y1: 0, X2: 200, Y2: 0, StopOffsets: rampOffsets(8)}
	tris := []float32{0, 0, 200, 200, 200, 0}
	if _, cw := triWinding(tris); cw != 1 {
		t.Fatalf("test input is not clockwise")
	}
	ccw, cw := triWinding(subdivide(tris, p))
	if ccw != 0 {
		t.Errorf("%d of %d triangles flipped to counter-clockwise",
			ccw, ccw+cw)
	}
}

func TestSubdivideRadialSplits(t *testing.T) {
	// One big triangle against a small radius must come back split:
	// a radial ramp is not affine, so long edges would linearize it.
	tris := []float32{0, 0, 100, 0, 0, 100}
	p := &Params{Radial: true, CX: 50, CY: 50, FX: 50, FY: 50, R: 50}
	out := subdivide(tris, p)
	if len(out) <= len(tris) {
		t.Fatalf("radial subdivision produced %d floats, want > %d",
			len(out), len(tris))
	}
	if len(out)%6 != 0 {
		t.Errorf("subdivided length %d is not a whole number of triangles",
			len(out))
	}
	// The depth cap bounds the split: 4^6 leaves per source triangle.
	maxTris := 1
	for range maxRadialDepth {
		maxTris *= 4
	}
	if len(out)/6 > maxTris {
		t.Errorf("produced %d triangles, above the depth cap of %d",
			len(out)/6, maxTris)
	}
}

func TestSubdivideLinearTwoStopsIsNoop(t *testing.T) {
	// Between two stops a linear ramp is exactly reproduced by vertex
	// interpolation, so there is nothing to split and nothing to pay.
	tris := []float32{0, 0, 100, 0, 0, 100}
	p := &Params{X1: 0, Y1: 0, X2: 100, Y2: 0, StopOffsets: twoStops()}
	if out := subdivide(tris, p); len(out) != len(tris) {
		t.Errorf("two-stop linear split to %d floats, want %d",
			len(out), len(tris))
	}
}

func TestSubdivideLinearSplitsAtStops(t *testing.T) {
	tris := []float32{0, 0, 100, 0, 0, 100}
	p := &Params{X1: 0, Y1: 0, X2: 100, Y2: 0,
		StopOffsets: []float32{0, 0.5, 1}}
	if out := subdivide(tris, p); len(out) <= len(tris) {
		t.Errorf("three-stop linear did not split: %d floats", len(out))
	}
}

// TestSubdivideNonFiniteRadius leaves the geometry alone rather than
// letting a NaN radius poison the depth heuristic.
func TestSubdivideNonFiniteRadius(t *testing.T) {
	tris := []float32{0, 0, 100, 0, 0, 100}
	for _, r := range []float32{float32(math.NaN()),
		float32(math.Inf(1)), float32(math.Inf(-1)), 0, -1} {
		p := &Params{Radial: true, R: r, StopOffsets: rampOffsets(8)}
		if got := subdivide(tris, p); len(got) != len(tris) {
			t.Errorf("R=%v: got %d floats, want %d (no subdivide)",
				r, len(got), len(tris))
		}
	}
}

// TestSubdivideRadialRespectsDepthCap bounds a single huge triangle
// against a tiny radius: depth cap 6 means at most 4^6 leaves.
func TestSubdivideRadialRespectsDepthCap(t *testing.T) {
	tris := []float32{0, 0, 1000, 0, 0, 1000}
	p := &Params{Radial: true, R: 0.024} // target ≈ 1e-3
	// The geometric pass on its own: the isoline pass would add cuts of
	// its own and blur what this bounds.
	var got []float32
	depth := radialSplitDepth(tris, p)
	splitRadialTri(tris[0], tris[1], tris[2], tris[3], tris[4], tris[5],
		depth, &got)
	const maxFloats = 6 * 4096
	if len(got) > maxFloats {
		t.Errorf("got %d floats, exceeds depth cap (max %d)",
			len(got), maxFloats)
	}
	if len(got) < 6 {
		t.Errorf("got %d floats, want at least 6", len(got))
	}
}

// TestRadialFanCostsNoSubdivision pins the reason the split criterion
// measures curvature rather than edge length: a fan struck from the
// gradient's own center is already radially aligned, so the deviation
// is ~0 and an edge-length rule would shatter it for nothing. The rule
// this replaced split a 25px circle into 80,896 triangles.
func TestRadialFanCostsNoSubdivision(t *testing.T) {
	const cx, cy, r = 100, 100, 25
	const segs = 64
	tris := centerStruckFan(cx, cy, r, segs)
	p := &Params{Radial: true, CX: cx, CY: cy, FX: cx, FY: cy, R: r,
		StopOffsets: twoStops()}
	// Two stops means no isolines either, so this must be the bare fan.
	if got := len(subdivide(tris, p)) / 6; got != segs {
		t.Errorf("center-struck fan = %d triangles, want the %d it was "+
			"given", got, segs)
	}
}

// TestRadialRectStillSubdivides is the other half: geometry that does
// bulge across the isolines must still refine, or the criterion would
// be free by doing nothing.
func TestRadialRectStillSubdivides(t *testing.T) {
	tris := []float32{
		0, 0, 200, 0, 200, 200,
		0, 0, 200, 200, 0, 200,
	}
	p := &Params{Radial: true, CX: 100, CY: 100, FX: 100, FY: 100, R: 100}
	if got := len(subdivide(tris, p)); got <= len(tris) {
		t.Errorf("radially filled rect = %d floats, want more than the "+
			"%d it was given", got, len(tris))
	}
}

// TestSubdivideSplitsRadialAtStops pins the behavior #399 gave the SVG
// side: before it, a radial gradient reached only the geometric pass
// and was never cut at its stops, so a hard color break inside a radial
// ramp had nowhere to land.
func TestSubdivideSplitsRadialAtStops(t *testing.T) {
	// A center-struck fan, fine enough that its rim chords carry no
	// curvature error either: the geometric pass leaves it alone, so
	// any growth here is the isoline pass and nothing else.
	const cx, cy, r = 100, 100, 50
	const segs = 64
	tris := centerStruckFan(cx, cy, r, segs)
	flat := &Params{Radial: true, CX: cx, CY: cy, FX: cx, FY: cy, R: r,
		StopOffsets: twoStops()}
	if len(subdivide(tris, flat)) != len(tris) {
		t.Fatal("geometric pass moved the fan; the count below would not " +
			"isolate the isoline pass")
	}
	p := *flat
	p.StopOffsets = []float32{0, 0.5, 1}
	if out := subdivide(tris, &p); len(out) <= len(tris) {
		t.Errorf("radial with an interior stop = %d floats, want more "+
			"than the %d it was given", len(out), len(tris))
	}
}

// TestSubdivideComposesBounded is the composition guard: the radial
// pass quadruples the batch per level and the isoline pass then splits
// every leaf up to three ways per cut. Each cap alone bounds one
// factor, and this pins their product for the worst input a real fill
// produces — a large rect, filled radially, with a stop list far past
// anything a designer would write.
func TestSubdivideComposesBounded(t *testing.T) {
	tris := []float32{
		0, 0, 400, 0, 400, 400,
		0, 0, 400, 400, 0, 400,
	}
	p := &Params{Radial: true, CX: 200, CY: 200, FX: 200, FY: 200, R: 200,
		StopOffsets: rampOffsets(200)}
	out := subdivide(tris, p)
	if len(out)%6 != 0 {
		t.Errorf("subdivided length %d is not a whole number of triangles",
			len(out))
	}
	// Comfortably inside the budget, which is the claim: the two passes
	// compose to something bounded on their own for realistic input,
	// and the budget is the backstop for input that is not.
	if len(out) > maxSplitFloats {
		t.Errorf("subdivision produced %d floats, above the budget %d",
			len(out), maxSplitFloats)
	}
	if len(out) <= len(tris) {
		t.Errorf("subdivision produced %d floats, want a real split",
			len(out))
	}
	if _, cw := triWinding(out); cw != 0 {
		t.Errorf("%d triangles wound the wrong way", cw)
	}
}

// TestRadialSplitDepthBacksOffOnLargeMeshes covers the other half of
// the budget. Depth is chosen from the worst triangle's curvature, but
// every level quadruples the *whole* batch, so a mesh that is already
// dense must not take the depth a single triangle would earn.
func TestRadialSplitDepthBacksOffOnLargeMeshes(t *testing.T) {
	p := &Params{Radial: true, CX: 0, CY: 0, FX: 0, FY: 0, R: 10}
	// One badly-aligned triangle, repeated until the batch is large.
	// Each copy is offset so none of them is degenerate.
	var tris []float32
	const copies = 5000
	for i := range copies {
		o := float32(i) * 0.001
		tris = append(tris, -100+o, -100, 100+o, -100, 0+o, 100)
	}
	depth := radialSplitDepth(tris, p)
	if got := copies << (2 * depth); got > maxRadialTris {
		t.Errorf("depth %d on %d triangles expands to %d, above %d",
			depth, copies, got, maxRadialTris)
	}
	// The same triangle on its own earns a real depth, so the backoff
	// above is the mesh size talking and not a broken criterion.
	if solo := radialSplitDepth(tris[:6], p); solo <= depth {
		t.Errorf("single triangle depth %d, batch depth %d: want the "+
			"single triangle to earn more", solo, depth)
	}
}

// TestSubdivideReusesCallerBuffers is the buffer contract: the scratch
// slices are the caller's, so a second call over the same buffers must
// not grow them without bound.
func TestSubdivideReusesCallerBuffers(t *testing.T) {
	tris := []float32{0, 0, 100, 0, 0, 100}
	p := &Params{X1: 0, Y1: 0, X2: 100, Y2: 0,
		StopOffsets: []float32{0, 0.5, 1}}
	var split, radial, isolines []float32
	first := len(Subdivide(tris, p, &split, &radial, &isolines))
	second := len(Subdivide(tris, p, &split, &radial, &isolines))
	if first != second {
		t.Errorf("second run over the same buffers = %d floats, want %d",
			second, first)
	}
}
