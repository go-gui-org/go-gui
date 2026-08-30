package gui

import (
	"math"
	"testing"
)

// The concentric fast path's whole claim is that a vertex's distance
// from the center places it exactly on the ramp. These tests assert
// that directly, per vertex, because a golden records only counts and
// endpoint colors — it cannot see a band drawn at the wrong radius.

// ringVertexRadiiMatchColors checks every emitted vertex against the
// ramp: the color it carries must be the color the ramp holds at its
// own distance from the center.
func ringVertexRadiiMatchColors(t *testing.T, cx, cy, r float32,
	stops []GradientStop) {
	t.Helper()
	dc := NewDrawContext(4*r, 4*r, nil)
	// Straight at the mesh: FilledCircleGradient now hands a short
	// concentric ramp to the shader instead, and this file is about
	// what the mesh emits.
	dc.fillConcentricRings(cx, cy, r, &CanvasGradient{
		Radial: true, Stops: stops,
	})
	batches := dc.Batches()
	if len(batches) == 0 {
		t.Fatal("no batches emitted")
	}
	var norm []GradientStop
	norm = NormalizeGradientStops(stops, &norm)

	verts := 0
	var maxR float64
	for _, b := range batches {
		if len(b.VertexColors)*2 != len(b.Triangles) {
			t.Fatalf("batch lengths disagree: %d triangles floats, %d colors",
				len(b.Triangles), len(b.VertexColors))
		}
		for i := range b.VertexColors {
			x, y := b.Triangles[i*2], b.Triangles[i*2+1]
			d := math.Hypot(float64(x-cx), float64(y-cy))
			maxR = max(maxR, d)
			if d > float64(r)+0.01 {
				t.Fatalf("vertex %d at radius %.3f is outside the circle (r=%.1f)",
					i, d, r)
			}
			// A vertex sits on a ring boundary, so the ramp position it
			// stands for is exact, not interpolated.
			pos := float32(d) / r
			want := SampleGradientStopColor(norm, pos)
			if got := b.VertexColors[i]; got != want {
				t.Fatalf("vertex %d at radius %.3f (pos %.4f): color %v, want %v",
					i, d, pos, got, want)
			}
			verts++
		}
	}
	if verts == 0 {
		t.Fatal("no vertices emitted")
	}
	// The fill must reach the rim, or the circle has a missing outer
	// annulus that no per-vertex check would notice.
	if maxR < float64(r)-0.5 {
		t.Errorf("mesh reaches only radius %.3f, want %.1f", maxR, r)
	}
}

func TestConcentricRingsMatchRamp(t *testing.T) {
	cases := []struct {
		name  string
		stops []GradientStop
	}{
		{"two stops spanning the circle", []GradientStop{
			{Color: RGBA(255, 255, 255, 255), Pos: 0},
			{Color: RGBA(255, 255, 255, 0), Pos: 1},
		}},
		{"flat core then a ramp", []GradientStop{
			{Color: RGB(255, 200, 0), Pos: 0.35},
			{Color: RGBA(255, 200, 0, 0), Pos: 1},
		}},
		{"flat rim", []GradientStop{
			{Color: RGB(255, 0, 0), Pos: 0},
			{Color: RGB(0, 0, 255), Pos: 0.4},
		}},
		{"flat at both ends", []GradientStop{
			{Color: RGB(255, 0, 0), Pos: 0.25},
			{Color: RGB(0, 0, 255), Pos: 0.75},
		}},
		{"many stops", []GradientStop{
			{Color: RGB(255, 0, 0), Pos: 0},
			{Color: RGB(255, 255, 0), Pos: 0.2},
			{Color: RGB(0, 255, 0), Pos: 0.4},
			{Color: RGB(0, 255, 255), Pos: 0.6},
			{Color: RGB(0, 0, 255), Pos: 0.8},
			{Color: RGB(255, 0, 255), Pos: 1},
		}},
		{"unsorted and out of range", []GradientStop{
			{Color: RGB(0, 0, 255), Pos: 1.7},
			{Color: RGB(255, 0, 0), Pos: -0.3},
			{Color: RGB(0, 255, 0), Pos: 0.5},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ringVertexRadiiMatchColors(t, 120, 120, 60, tc.stops)
		})
	}
}

// TestConcentricRingsHardStop keeps a repeated position a color jump.
// A band of zero width contributes no geometry, but the color on the
// far side of it must still change.
func TestConcentricRingsHardStop(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	dc := NewDrawContext(400, 400, nil)
	dc.fillConcentricRings(100, 100, 50, &CanvasGradient{
		Radial: true,
		Stops: []GradientStop{
			{Color: red, Pos: 0},
			{Color: red, Pos: 0.5},
			{Color: blue, Pos: 0.5},
			{Color: blue, Pos: 1},
		},
	})
	var sawRed, sawBlue bool
	for _, b := range dc.Batches() {
		for i, c := range b.VertexColors {
			d := math.Hypot(float64(b.Triangles[i*2]-100),
				float64(b.Triangles[i*2+1]-100))
			switch {
			case d < 24.9:
				if c != red {
					t.Fatalf("inside the jump at r=%.2f: %v, want red", d, c)
				}
				sawRed = true
			case d > 25.1:
				if c != blue {
					t.Fatalf("outside the jump at r=%.2f: %v, want blue", d, c)
				}
				sawBlue = true
			}
		}
	}
	if !sawRed || !sawBlue {
		t.Errorf("hard stop not exercised: red=%v blue=%v", sawRed, sawBlue)
	}
}

// TestConcentricRingsDeclined pins the cases that must stay on the
// general path, where the ring mesh would be wrong.
func TestConcentricRingsDeclined(t *testing.T) {
	stops := []GradientStop{
		{Color: RGB(255, 0, 0), Pos: 0},
		{Color: RGB(0, 0, 255), Pos: 1},
	}
	cases := []struct {
		name string
		g    CanvasGradient
	}{
		{"linear", CanvasGradient{Stops: stops}},
		{"off-center", CanvasGradient{
			Radial: true, Stops: stops, CX: 10, CY: 100, R: 50}},
		{"radius not the circle's", CanvasGradient{
			Radial: true, Stops: stops, CX: 100, CY: 100, R: 20}},
		{"offset focal", CanvasGradient{
			Radial: true, Stops: stops, CX: 100, CY: 100, R: 50,
			FX: 90, FY: 100}},
		{"repeat spread", CanvasGradient{
			Radial: true, Stops: stops, Spread: SpreadRepeat}},
		{"reflect spread", CanvasGradient{
			Radial: true, Stops: stops, Spread: SpreadReflect}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dc := NewDrawContext(400, 400, nil)
			g := tc.g
			if dc.fillConcentricRings(100, 100, 50, &g) {
				t.Error("took the concentric fast path")
			}
		})
	}
}

// TestConcentricRingsAccepted is the other half: the spellings that
// must reach the fast path, including the fully explicit one.
func TestConcentricRingsAccepted(t *testing.T) {
	stops := []GradientStop{
		{Color: RGB(255, 0, 0), Pos: 0},
		{Color: RGB(0, 0, 255), Pos: 1},
	}
	cases := []struct {
		name string
		g    CanvasGradient
	}{
		{"defaulted geometry", CanvasGradient{Radial: true, Stops: stops}},
		{"explicit center and radius", CanvasGradient{
			Radial: true, Stops: stops, CX: 100, CY: 100, R: 50}},
		{"explicit focal on center", CanvasGradient{
			Radial: true, Stops: stops, CX: 100, CY: 100, R: 50,
			FX: 100, FY: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dc := NewDrawContext(400, 400, nil)
			g := tc.g
			if !dc.fillConcentricRings(100, 100, 50, &g) {
				t.Error("fell through to the general path")
			}
		})
	}
}

// TestConcentricRingsMatchGeneralPath is the equivalence check the
// speedup rests on: the ring mesh and the mesh the subdivision pass
// builds must paint the same picture.
//
// FilledArcGradient is the seam — it is what FilledCircleGradient
// delegates to when the fast path declines, so calling it directly
// with a full sweep renders the identical fill the slow way.
func TestConcentricRingsMatchGeneralPath(t *testing.T) {
	const cx, cy, r = 120, 120, 70
	for _, tc := range []struct {
		name  string
		stops []GradientStop
	}{
		{"opaque ramp", []GradientStop{
			{Color: RGB(255, 220, 120), Pos: 0},
			{Color: RGB(255, 160, 40), Pos: 0.45},
			{Color: RGB(200, 80, 0), Pos: 1},
		}},
		{"alpha ramp", []GradientStop{
			{Color: RGBA(255, 220, 120, 255), Pos: 0},
			{Color: RGBA(255, 200, 90, 120), Pos: 0.45},
			{Color: RGBA(255, 180, 60, 0), Pos: 1},
		}},
		{"flat core", []GradientStop{
			{Color: RGB(255, 200, 0), Pos: 0.35},
			{Color: RGBA(255, 200, 0, 0), Pos: 1},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := CanvasGradient{Radial: true, Stops: tc.stops}

			fast := NewDrawContext(4*r, 4*r, nil)
			fast.fillConcentricRings(cx, cy, r, &g)

			slow := NewDrawContext(4*r, 4*r, nil)
			slow.FilledArcGradient(cx, cy, r, r, 0, 2*math.Pi, &g)

			var worst int
			var worstAt float32
			for ring := 1; ring <= 12; ring++ {
				rad := float32(ring) / 13 * r
				for a := range 24 {
					ang := float64(a) / 24 * 2 * math.Pi
					px := cx + rad*float32(math.Cos(ang))
					py := cy + rad*float32(math.Sin(ang))
					got, ok := sampleMeshAt(fast, px, py)
					if !ok {
						t.Fatalf("ring mesh does not cover radius %.1f", rad)
					}
					want, ok := sampleMeshAt(slow, px, py)
					if !ok {
						t.Fatalf("general mesh does not cover radius %.1f", rad)
					}
					if d := colorDelta(got, want); d > worst {
						worst, worstAt = d, rad
					}
				}
			}
			// Both meshes are Gouraud-interpolated straight colors, so
			// they agree wherever they place vertices the same way. The
			// slack is for the band interiors, where the ring mesh
			// spans a whole stop pair and the subdivided one carries
			// extra vertices through the same straight interpolation.
			if worst > 8 {
				t.Errorf("worst channel delta %d at radius %.1f, want <= 8",
					worst, worstAt)
			}
		})
	}
}

func colorDelta(a, b Color) int {
	d := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G},
		{a.B, b.B}, {a.A, b.A}} {
		v := int(p[0]) - int(p[1])
		if v < 0 {
			v = -v
		}
		d = max(d, v)
	}
	return d
}

// sampleMeshAt finds the triangle covering a point and returns its
// barycentric blend of the three vertex colors — what the backend's
// Gouraud shading paints there.
func sampleMeshAt(dc *DrawContext, px, py float32) (Color, bool) {
	for _, b := range dc.Batches() {
		for i := 0; i+5 < len(b.Triangles); i += 6 {
			ax, ay := b.Triangles[i], b.Triangles[i+1]
			bx, by := b.Triangles[i+2], b.Triangles[i+3]
			cx, cy := b.Triangles[i+4], b.Triangles[i+5]
			den := (by-cy)*(ax-cx) + (cx-bx)*(ay-cy)
			if den == 0 {
				continue
			}
			l0 := ((by-cy)*(px-cx) + (cx-bx)*(py-cy)) / den
			l1 := ((cy-ay)*(px-cx) + (ax-cx)*(py-cy)) / den
			l2 := 1 - l0 - l1
			const eps = -1e-4
			if l0 < eps || l1 < eps || l2 < eps {
				continue
			}
			v := i / 2
			c0, c1, c2 := b.VertexColors[v], b.VertexColors[v+1],
				b.VertexColors[v+2]
			mix := func(x, y, z uint8) uint8 {
				return f32ToU8Saturated(l0*float32(x) +
					l1*float32(y) + l2*float32(z))
			}
			return RGBA(mix(c0.R, c1.R, c2.R), mix(c0.G, c1.G, c2.G),
				mix(c0.B, c1.B, c2.B), mix(c0.A, c1.A, c2.A)), true
		}
	}
	return Color{}, false
}
