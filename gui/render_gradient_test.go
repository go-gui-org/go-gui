package gui

import (
	"math"
	"testing"
)

func TestClampUnit(t *testing.T) {
	tests := []struct{ in, want float32 }{
		{-1, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {2, 1},
		{float32(math.NaN()), 0},
		{float32(math.Inf(1)), 1},
		{float32(math.Inf(-1)), 0},
	}
	for _, tc := range tests {
		if got := clampUnit(tc.in); got != tc.want {
			t.Errorf("clampUnit(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestAngleToDirectionCardinal(t *testing.T) {
	const eps = 1e-5
	cases := []struct {
		deg  float32
		wDx  float32
		wDy  float32
		name string
	}{
		{0, 0, -1, "top"},
		{90, 1, 0, "right"},
		{180, 0, 1, "bottom"},
		{270, -1, 0, "left"},
	}
	for _, c := range cases {
		dx, dy := angleToDirection(c.deg)
		if math.Abs(float64(dx-c.wDx)) > eps || math.Abs(float64(dy-c.wDy)) > eps {
			t.Errorf("%s: got (%v,%v), want (%v,%v)", c.name, dx, dy, c.wDx, c.wDy)
		}
	}
}

func TestAngleToDirectionDiagonal(t *testing.T) {
	const eps = 1e-5
	dx, dy := angleToDirection(45)
	expected := float32(math.Sqrt(2) / 2)
	if math.Abs(float64(dx-expected)) > eps || math.Abs(float64(dy+expected)) > eps {
		t.Errorf("45deg: got (%v,%v)", dx, dy)
	}
}

func TestGradientDirectionKeywords(t *testing.T) {
	const eps = 1e-5
	g := &GradientDef{}
	cases := []struct {
		dir GradientDirection
		wDx float32
		wDy float32
	}{
		{GradientToTop, 0, -1},
		{GradientToRight, 1, 0},
		{GradientToBottom, 0, 1},
		{GradientToLeft, -1, 0},
	}
	for _, c := range cases {
		g.Direction = c.dir
		dx, dy := GradientDir(g, 100, 100)
		if math.Abs(float64(dx-c.wDx)) > eps || math.Abs(float64(dy-c.wDy)) > eps {
			t.Errorf("dir %d: got (%v,%v), want (%v,%v)", c.dir, dx, dy, c.wDx, c.wDy)
		}
	}
}

func TestGradientDirectionAngleOverride(t *testing.T) {
	const eps = 1e-5
	g := &GradientDef{hasAngle: true, angle: 90, Direction: GradientToTop}
	dx, dy := GradientDir(g, 100, 100)
	if math.Abs(float64(dx-1)) > eps || math.Abs(float64(dy)) > eps {
		t.Errorf("angle override: got (%v,%v), want (1,0)", dx, dy)
	}
}

func TestPackRGB(t *testing.T) {
	c := RGBA(100, 150, 200, 0)
	p := PackRGB(c)
	// Unpack: R = p mod 256, G = (p/256) mod 256, B = p/65536
	r := uint8(math.Mod(float64(p), 256))
	g := uint8(math.Mod(float64(p)/256, 256))
	b := uint8(p / 65536)
	if r != 100 || g != 150 || b != 200 {
		t.Errorf("unpack: got (%d,%d,%d), want (100,150,200)", r, g, b)
	}
}

func TestPackAlphaPos(t *testing.T) {
	c := RGBA(0, 0, 0, 128)
	p := PackAlphaPos(c, 0.5)
	// Alpha = p mod 256 = 128
	a := uint8(math.Mod(float64(p), 256))
	if a != 128 {
		t.Errorf("alpha: got %d, want 128", a)
	}
}

func TestF32ToU8Saturated(t *testing.T) {
	tests := []struct {
		in   float32
		want uint8
	}{
		{-10, 0}, {0, 0}, {127.5, 128}, {255, 255}, {300, 255},
	}
	for _, tc := range tests {
		if got := f32ToU8Saturated(tc.in); got != tc.want {
			t.Errorf("f32ToU8(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestLerpColorPremultipliedEndpoints(t *testing.T) {
	a := RGBA(255, 0, 0, 255)
	b := RGBA(0, 0, 255, 255)
	c0 := lerpColorPremultiplied(a, b, 0)
	if c0 != a {
		t.Errorf("t=0: got %v, want %v", c0, a)
	}
	c1 := lerpColorPremultiplied(a, b, 1)
	if c1 != b {
		t.Errorf("t=1: got %v, want %v", c1, b)
	}
}

func TestLerpColorPremultipliedMid(t *testing.T) {
	a := RGBA(0, 0, 0, 255)
	b := RGBA(254, 254, 254, 255)
	mid := lerpColorPremultiplied(a, b, 0.5)
	if mid.R < 125 || mid.R > 129 {
		t.Errorf("mid R: got %d", mid.R)
	}
}

func TestLerpColorPremultipliedZeroAlpha(t *testing.T) {
	a := RGBA(255, 0, 0, 0)
	b := RGBA(0, 0, 0, 0)
	c := lerpColorPremultiplied(a, b, 0.5)
	if c.A != 0 {
		t.Errorf("zero alpha: got A=%d", c.A)
	}
}

// TestLerpColorPremultipliedKeepsHueAtZeroAlpha pins the rule that a
// fade to transparent must not fade to black. Premultiplied space has
// no hue left at zero alpha, but the color is handed to consumers that
// interpolate in straight-alpha space — every vertex-colored triangle
// mesh does — where a transparent black reads as a real color and the
// fade darkens instead of thinning.
func TestLerpColorPremultipliedKeepsHueAtZeroAlpha(t *testing.T) {
	white := RGBA(255, 255, 255, 255)
	clear := RGBA(255, 255, 255, 0)
	c := lerpColorPremultiplied(white, clear, 1)
	if c.A != 0 {
		t.Errorf("A = %d, want 0", c.A)
	}
	if c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("rgb = (%d,%d,%d), want white kept", c.R, c.G, c.B)
	}
	// The nearer end supplies the hue, so a fade from red keeps red.
	red := RGBA(255, 0, 0, 0)
	blue := RGBA(0, 0, 255, 0)
	if got := lerpColorPremultiplied(red, blue, 0.25); got.R != 255 {
		t.Errorf("t=0.25 hue = %v, want the red end", got)
	}
	if got := lerpColorPremultiplied(red, blue, 0.75); got.B != 255 {
		t.Errorf("t=0.75 hue = %v, want the blue end", got)
	}
}

func TestSampleGradientStopColorEmpty(t *testing.T) {
	c := SampleGradientStopColor(nil, 0.5)
	if c.A != 0 {
		t.Error("empty stops should return transparent")
	}
}

func TestSampleGradientStopColorSingle(t *testing.T) {
	stops := []GradientStop{{Color: RGBA(100, 0, 0, 255), Pos: 0.5}}
	c := SampleGradientStopColor(stops, 0.0)
	if c.R != 100 {
		t.Errorf("single stop: got R=%d", c.R)
	}
}

func TestSampleGradientStopColorTwoStop(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(0, 0, 0, 255), Pos: 0},
		{Color: RGBA(254, 0, 0, 255), Pos: 1},
	}
	mid := SampleGradientStopColor(stops, 0.5)
	if mid.R < 125 || mid.R > 129 {
		t.Errorf("two-stop mid: got R=%d", mid.R)
	}
}

func TestSampleGradientStopColorBoundary(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(10, 0, 0, 255), Pos: 0},
		{Color: RGBA(200, 0, 0, 255), Pos: 1},
	}
	c0 := SampleGradientStopColor(stops, 0)
	if c0.R != 10 {
		t.Errorf("pos=0: got R=%d", c0.R)
	}
	c1 := SampleGradientStopColor(stops, 1)
	if c1.R != 200 {
		t.Errorf("pos=1: got R=%d", c1.R)
	}
}

func TestNormalizeGradientStopsIntoEmpty(t *testing.T) {
	norm := make([]GradientStop, 0)
	sampled := make([]GradientStop, 0)
	if got := NormalizeGradientStopsInto(nil, &norm, &sampled); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestNormalizeGradientStopsIntoSorted(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0.8},
		{Color: RGBA(0, 0, 0, 255), Pos: 0.2},
	}
	norm := make([]GradientStop, 0, 8)
	sampled := make([]GradientStop, 0, 8)
	result := NormalizeGradientStopsInto(stops, &norm, &sampled)
	if result[0].Pos > result[1].Pos {
		t.Error("should be sorted")
	}
}

func TestNormalizeGradientStopsIntoOverLimit(t *testing.T) {
	stops := make([]GradientStop, gradientShaderStopLimit+6)
	for i := range stops {
		u := float32(i) / float32(len(stops)-1)
		stops[i] = GradientStop{
			// A curve, so no subset of it is collinear and the
			// resample has to fill its whole budget.
			Color: RGBA(uint8(255*u*u), 0, 0, 255),
			Pos:   u,
		}
	}
	norm := make([]GradientStop, 0, len(stops))
	sampled := make([]GradientStop, 0, gradientShaderStopLimit)
	result := NormalizeGradientStopsInto(stops, &norm, &sampled)
	if len(result) != gradientShaderStopLimit {
		t.Fatalf("want %d stops, got %d", gradientShaderStopLimit, len(result))
	}
	// First and last sampled positions must be 0.0 and 1.0.
	if result[0].Pos != 0.0 {
		t.Errorf("first pos: got %v, want 0.0", result[0].Pos)
	}
	if result[len(result)-1].Pos != 1.0 {
		t.Errorf("last pos: got %v, want 1.0", result[len(result)-1].Pos)
	}
}

// NormalizeGradientStops keeps every stop: it is the no-resample
// variant for backends without a stop limit (web canvas gradients).
func TestNormalizeGradientStopsKeepsAll(t *testing.T) {
	stops := make([]GradientStop, 10)
	for i := range stops {
		stops[i] = GradientStop{
			Color: RGBA(uint8(i*25), 0, 0, 255),
			Pos:   float32(i) / 9.0,
		}
	}
	// Out-of-range and unsorted input exercises the clamp and sort.
	stops[9].Pos = 1.5
	stops[8].Pos = -0.5
	norm := make([]GradientStop, 0, 16)
	result := NormalizeGradientStops(stops, &norm)
	if len(result) != 10 {
		t.Fatalf("want all 10 stops preserved, got %d", len(result))
	}
	if result[0].Pos != 0 {
		t.Errorf("first pos: got %v, want 0.0", result[0].Pos)
	}
	if result[9].Pos != 1 {
		t.Errorf("last pos: got %v, want 1.0 (clamped)", result[9].Pos)
	}
	for i := 1; i < len(result); i++ {
		if result[i-1].Pos > result[i].Pos {
			t.Fatalf("stops must be sorted, index %d: %v > %v",
				i, result[i-1].Pos, result[i].Pos)
		}
	}
}

func TestNormalizeGradientStopsEmpty(t *testing.T) {
	norm := make([]GradientStop, 0)
	if got := NormalizeGradientStops(nil, &norm); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

// NaN positions must fold to 0 (clampUnit) rather than poison the
// sort order or the output positions.
func TestNormalizeGradientStopsNaNPosition(t *testing.T) {
	nan := float32(math.NaN())
	stops := []GradientStop{
		{Color: RGBA(0, 255, 0, 255), Pos: 1},
		{Color: RGBA(255, 0, 0, 255), Pos: nan},
		{Color: RGBA(0, 0, 255, 255), Pos: 0.5},
	}
	norm := make([]GradientStop, 0, 8)
	result := NormalizeGradientStops(stops, &norm)
	if len(result) != 3 {
		t.Fatalf("want 3 stops, got %d", len(result))
	}
	for i, s := range result {
		if s.Pos != s.Pos {
			t.Fatalf("stop %d keeps a NaN position", i)
		}
		if s.Pos < 0 || s.Pos > 1 {
			t.Fatalf("stop %d out of range: %v", i, s.Pos)
		}
	}
}

func TestNormalizeGradientStopsIntoReuse(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(0, 0, 0, 255), Pos: 0},
		{Color: RGBA(255, 0, 0, 255), Pos: 1},
	}
	norm := make([]GradientStop, 0, 8)
	sampled := make([]GradientStop, 0, 8)
	result := NormalizeGradientStopsInto(stops, &norm, &sampled)
	if len(result) != 2 {
		t.Fatalf("want 2, got %d", len(result))
	}
}

func TestGradientDirNil(t *testing.T) {
	dx, dy := GradientDir(nil, 100, 100)
	if dx != 0 || dy != -1 {
		t.Errorf("nil GradientDef: got (%v,%v), want (0,-1)", dx, dy)
	}
}

func TestGradientDirectionDiagonals(t *testing.T) {
	const eps = 1e-4
	// Square: all diagonals should be ±√2/2 ≈ ±0.7071.
	sq := float32(math.Sqrt(2) / 2)
	cases := []struct {
		dir  GradientDirection
		wDx  float32
		wDy  float32
		w, h float32
		name string
	}{
		{GradientToTopRight, sq, -sq, 100, 100, "top-right 100x100"},
		{GradientToBottomRight, sq, sq, 100, 100, "bottom-right 100x100"},
		{GradientToBottomLeft, -sq, sq, 100, 100, "bottom-left 100x100"},
		{GradientToTopLeft, -sq, -sq, 100, 100, "top-left 100x100"},
	}
	g := &GradientDef{}
	for _, c := range cases {
		g.Direction = c.dir
		dx, dy := GradientDir(g, c.w, c.h)
		if math.Abs(float64(dx-c.wDx)) > eps ||
			math.Abs(float64(dy-c.wDy)) > eps {
			t.Errorf("%s: got (%v,%v), want (%v,%v)",
				c.name, dx, dy, c.wDx, c.wDy)
		}
	}
	// Non-square (200×100): atan2(100,200) ≈ 26.57°, not 45°.
	g.Direction = GradientToTopRight
	dx, dy := GradientDir(g, 200, 100)
	// CSS angle = 90 - atan2(100,200)°, so dx > dy magnitude.
	if math.Abs(float64(dx)) < math.Abs(float64(dy)) {
		t.Errorf("non-square: expected |dx| > |dy|, got (%v,%v)",
			dx, dy)
	}
	if dx <= 0 || dy >= 0 {
		t.Errorf("non-square top-right: expected dx>0, dy<0, got (%v,%v)",
			dx, dy)
	}
}
