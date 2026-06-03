package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParseRadialGradientDefaults(t *testing.T) {
	t.Parallel()
	src := `<svg xmlns="http://www.w3.org/2000/svg">
		<defs>
			<radialGradient id="g">
				<stop offset="0" stop-color="#ff0000"/>
				<stop offset="1" stop-color="#0000ff"/>
			</radialGradient>
		</defs>
	</svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parseSvg: %v", err)
	}
	g, ok := vg.Gradients["g"]
	if !ok {
		t.Fatal("gradient g missing")
	}
	if !g.IsRadial {
		t.Error("IsRadial = false, want true")
	}
	wantPairs := []struct {
		name string
		got  float32
		want float32
	}{
		{"CX", g.CX, 0.5},
		{"CY", g.CY, 0.5},
		{"R", g.R, 0.5},
		{"FX", g.FX, 0.5},
		{"FY", g.FY, 0.5},
	}
	for _, p := range wantPairs {
		if math.Abs(float64(p.got-p.want)) > 1e-6 {
			t.Errorf("%s = %v, want %v", p.name, p.got, p.want)
		}
	}
	if len(g.Stops) != 2 {
		t.Errorf("len(Stops) = %d, want 2", len(g.Stops))
	}
}

func TestParseRadialGradientAttrs(t *testing.T) {
	t.Parallel()
	src := `<svg xmlns="http://www.w3.org/2000/svg">
		<defs>
			<radialGradient id="g" cx="25%" cy="75%" r="40%"
				fx="20%" fy="60%">
				<stop offset="0" stop-color="black"/>
				<stop offset="1" stop-color="white"/>
			</radialGradient>
		</defs>
	</svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parseSvg: %v", err)
	}
	g := vg.Gradients["g"]
	cases := []struct {
		name string
		got  float32
		want float32
	}{
		{"CX", g.CX, 0.25}, {"CY", g.CY, 0.75},
		{"R", g.R, 0.40},
		{"FX", g.FX, 0.20}, {"FY", g.FY, 0.60},
	}
	for _, c := range cases {
		if math.Abs(float64(c.got-c.want)) > 1e-6 {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestProjectOntoRadial(t *testing.T) {
	t.Parallel()
	g := gui.SvgGradientDef{
		IsRadial: true,
		FX:       50, FY: 50, R: 50,
	}
	cases := []struct {
		x, y float32
		want float32
	}{
		{50, 50, 0},   // at focal
		{100, 50, 1},  // at edge
		{75, 50, 0.5}, // mid
		{200, 50, 1},  // outside → clamp
	}
	for _, c := range cases {
		got := projectOntoRadial(c.x, c.y, g)
		if math.Abs(float64(got-c.want)) > 1e-5 {
			t.Errorf("project(%v,%v) = %v, want %v",
				c.x, c.y, got, c.want)
		}
	}
}

func TestResolveGradientRadialOBB(t *testing.T) {
	t.Parallel()
	g := gui.SvgGradientDef{
		IsRadial: true,
		CX:       0.5, CY: 0.5, R: 0.5,
		FX: 0.5, FY: 0.5,
	}
	r := resolveGradient(g, 0, 0, 100, 100)
	if !r.IsRadial {
		t.Error("IsRadial dropped")
	}
	if r.CX != 50 || r.CY != 50 {
		t.Errorf("center = (%v,%v), want (50,50)", r.CX, r.CY)
	}
	if r.R != 50 {
		t.Errorf("R = %v, want 50", r.R)
	}
}

func TestResolveGradientRadialOBBNonSquare(t *testing.T) {
	t.Parallel()
	// Wide bbox 200x100. Approximation maps R uniformly via
	// avg = (w+h)/2 = 150, so R = 0.5 * 150 = 75. Document in
	// the test so the approximation doesn't silently change.
	g := gui.SvgGradientDef{
		IsRadial: true,
		CX:       0.25, CY: 0.5, R: 0.5,
		FX: 0.10, FY: 0.5,
	}
	r := resolveGradient(g, 0, 0, 200, 100)
	if r.CX != 50 {
		t.Errorf("CX = %v, want 50 (0.25 * 200)", r.CX)
	}
	if r.CY != 50 {
		t.Errorf("CY = %v, want 50 (0.5 * 100)", r.CY)
	}
	if r.FX != 20 {
		t.Errorf("FX = %v, want 20 (0.10 * 200)", r.FX)
	}
	if r.R != 75 {
		t.Errorf("R = %v, want 75 (avg=(200+100)/2 * 0.5)", r.R)
	}
	if r.GradientUnits != "userSpaceOnUse" {
		t.Errorf("GradientUnits = %q, want userSpaceOnUse",
			r.GradientUnits)
	}
}

func TestResolveGradientRadialOffsetBBox(t *testing.T) {
	t.Parallel()
	// Non-zero bbox origin: center should land at minX + cx*w.
	g := gui.SvgGradientDef{
		IsRadial: true,
		CX:       0.5, CY: 0.5, R: 0.5,
		FX: 0.5, FY: 0.5,
	}
	r := resolveGradient(g, 100, 200, 200, 400)
	if r.CX != 150 || r.CY != 300 {
		t.Errorf("center = (%v,%v), want (150,300)", r.CX, r.CY)
	}
}

func TestProjectOntoRadialNonFinite(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		g    gui.SvgGradientDef
		vx   float32
		vy   float32
	}{
		{"NaN R", gui.SvgGradientDef{
			IsRadial: true, R: float32(math.NaN()),
		}, 1, 1},
		{"Inf R", gui.SvgGradientDef{
			IsRadial: true, R: float32(math.Inf(1)),
		}, 1, 1},
		{"zero R", gui.SvgGradientDef{IsRadial: true, R: 0}, 1, 1},
		{"NaN focal", gui.SvgGradientDef{
			IsRadial: true, R: 50,
			FX: float32(math.NaN()), FY: 0,
		}, 10, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := projectOntoRadial(c.vx, c.vy, c.g)
			if math.IsNaN(float64(got)) || math.IsInf(float64(got), 0) {
				t.Errorf("got %v, want finite", got)
			}
			if got < 0 || got > 1 {
				t.Errorf("got %v, want in [0,1]", got)
			}
		})
	}
}

func TestSubdivideRadialTrisNonFiniteR(t *testing.T) {
	t.Parallel()
	tris := []float32{0, 0, 100, 0, 0, 100}
	cases := []float32{
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
		0,
		-1,
	}
	for _, r := range cases {
		g := gui.SvgGradientDef{IsRadial: true, R: r}
		got := subdivideRadialTris(tris, g)
		if len(got) != len(tris) {
			t.Errorf("R=%v: got %d floats, want %d (no subdivide)",
				r, len(got), len(tris))
		}
	}
}

func TestSubdivideRadialTrisRespectsDepthCap(t *testing.T) {
	t.Parallel()
	// Single huge triangle, tiny target. Depth cap = 6, so 1 source
	// triangle → at most 4^6 = 4096 sub-triangles = 24576 floats.
	tris := []float32{0, 0, 1000, 0, 0, 1000}
	g := gui.SvgGradientDef{IsRadial: true, R: 0.024} // target ≈ 1e-3
	got := subdivideRadialTris(tris, g)
	const maxFloats = 6 * 4096
	if len(got) > maxFloats {
		t.Errorf("got %d floats, exceeds depth cap (max %d)",
			len(got), maxFloats)
	}
	if len(got) < 6 {
		t.Errorf("got %d floats, want at least 6", len(got))
	}
}

func TestTessellatePopulatesPathBBox(t *testing.T) {
	t.Parallel()
	// Single rect 10..40 × 20..50. After tessellation, path[0]
	// MinX/MaxX/MinY/MaxY must reflect that extent.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<rect x="10" y="20" width="30" height="30" fill="#000"/>
	</svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parseSvg: %v", err)
	}
	tris := vg.tessellatePaths(vg.Paths, 1)
	if len(tris) == 0 {
		t.Fatal("no tessellated paths")
	}
	p := tris[0]
	if p.MinX != 10 || p.MaxX != 40 {
		t.Errorf("X bbox = (%v,%v), want (10,40)", p.MinX, p.MaxX)
	}
	if p.MinY != 20 || p.MaxY != 50 {
		t.Errorf("Y bbox = (%v,%v), want (20,50)", p.MinY, p.MaxY)
	}
}

func TestSubdivideRadialTrisShortInput(t *testing.T) {
	t.Parallel()
	// Less than one full triangle: must not panic, just no-op.
	g := gui.SvgGradientDef{IsRadial: true, R: 50}
	for _, in := range [][]float32{nil, {}, {0, 0}, {0, 0, 1, 0}} {
		got := subdivideRadialTris(in, g)
		// May return nil or empty allocation; both are acceptable
		// since no full triangle exists to emit.
		if len(got) != 0 {
			t.Errorf("input %v: got %d floats, want 0", in, len(got))
		}
	}
}
