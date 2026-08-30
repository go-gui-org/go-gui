package gui

import (
	"math"
	"testing"
)

// Stop resampling is the one GPU-only behaviour no other gate can see:
// the soft and web backends honour the full stop list, and the golden
// files record the pre-resample list, so nothing but these tests
// notices when the shader path's stops land in the wrong places.

// haloCurve is the accumulated-ring glow profile from
// examples/solar_system: alpha 1-exp(-k*(1-u)^e) over the falloff,
// flat across the body it surrounds. It is the in-tree curve that
// actually exceeds the shader stop limit, and its opacity piles into
// the innermost third of the ramp, which is what makes even spacing a
// poor fit.
type haloCurve struct {
	c            Color
	inFrac, k, e float64
}

func newHaloCurve(c Color, inner, outer float32,
	peak float32, falloff, rings int,
) haloCurve {
	e := float64(falloff + 1)
	return haloCurve{
		c:      c,
		inFrac: float64(inner / outer),
		k:      float64(float32(rings)*peak) / e,
		e:      e,
	}
}

// at returns the curve's color at gradient parameter t.
func (h haloCurve) at(t float64) Color {
	u := 0.0
	if t > h.inFrac {
		u = (t - h.inFrac) / (1 - h.inFrac)
	}
	a := 1 - math.Exp(-h.k*math.Pow(1-u, h.e))
	return h.c.WithOpacity(float32(a))
}

// stops samples the curve the way the example does — two flat stops
// across the body, then a run across the falloff — but with enough
// samples to land over the shader stop limit whatever that limit is.
// The example itself writes ten, which the uniforms now carry
// untouched; the placement policy this file is about only runs on a
// list too long to carry, so the fixture has to be one.
func (h haloCurve) stops() []GradientStop {
	out := []GradientStop{
		{Color: h.at(0), Pos: 0},
		{Color: h.at(h.inFrac), Pos: float32(h.inFrac)},
	}
	samples := 2 * gradientShaderStopLimit
	for i := 1; i <= samples; i++ {
		u := float64(i) / float64(samples)
		pos := h.inFrac + (1-h.inFrac)*u
		out = append(out, GradientStop{
			Color: h.at(pos), Pos: float32(pos)})
	}
	return out
}

// worstErrVsCurve is the largest premultiplied-channel gap between a
// stop list and the curve it approximates, in 0..255 units. Anything
// under 8 is at or below the source list's own sampling error.
func worstErrVsCurve(h haloCurve, stops []GradientStop) float32 {
	var worst float32
	const n = 2048
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		want := premulChannels(h.at(t))
		got := premulChannels(SampleGradientStopColor(stops, float32(t)))
		for ch := range 4 {
			d := want[ch] - got[ch]
			if d < 0 {
				d = -d
			}
			if d > worst {
				worst = d
			}
		}
	}
	return worst
}

// evenResampleForTest is the placement this package used before
// error-driven placement replaced it. Kept only so the tests can show
// the gap it left; it is not called by anything but this file.
func evenResampleForTest(stops []GradientStop) []GradientStop {
	out := make([]GradientStop, 0, gradientShaderStopLimit)
	for i := range gradientShaderStopLimit {
		p := float32(i) / float32(gradientShaderStopLimit-1)
		out = append(out, GradientStop{
			Color: SampleGradientStopColor(stops, p), Pos: p})
	}
	return out
}

func haloTestCurves() []struct {
	name string
	h    haloCurve
} {
	sunRings := func(outer float32) int {
		return int(min(max(float64(outer*0.22), 24), 140))
	}
	sun := RGB(255, 186, 78)
	hover := RGB(90, 140, 255)
	return []struct {
		name string
		h    haloCurve
	}{
		{"sun small", newHaloCurve(sun, 40, 68, 0.30, 3, sunRings(68))},
		{"sun mid", newHaloCurve(sun, 200, 340, 0.30, 3, sunRings(340))},
		{"sun large", newHaloCurve(sun, 700, 1190, 0.46, 3, sunRings(1190))},
		{"hover small", newHaloCurve(hover, 12, 25.2, 0.42, 2, 6)},
		{"hover large", newHaloCurve(hover, 200, 242, 0.42, 2, 6)},
	}
}

// TestResampleStopsBeatsEvenSpacing pins the fidelity the error-driven
// placement buys. The bound is what matters; the even-spacing figure
// is measured alongside it so a regression to positional spacing shows
// up as a number rather than as a shrug.
func TestResampleStopsBeatsEvenSpacing(t *testing.T) {
	// Measured worst case is 3 at a twelve-stop limit, against a source
	// list whose own error on the same curve is 1. Even spacing scores
	// 9 to 21 on the same fixtures. Retighten with
	// gradientShaderStopLimit.
	const bound = 4
	for _, tc := range haloTestCurves() {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.h.stops()
			if len(src) <= gradientShaderStopLimit {
				t.Skipf("curve has %d stops, under the limit", len(src))
			}
			var norm, sampled []GradientStop
			got := NormalizeGradientStopsInto(src, &norm, &sampled)

			gotErr := worstErrVsCurve(tc.h, got)
			evenErr := worstErrVsCurve(tc.h, evenResampleForTest(src))
			t.Logf("resampled %.0f, even %.0f, full %.0f (0..255)",
				gotErr, evenErr, worstErrVsCurve(tc.h, src))

			if gotErr > bound {
				t.Errorf("worst premultiplied error %.0f exceeds %d",
					gotErr, bound)
			}
			if gotErr >= evenErr {
				t.Errorf("error-driven placement %.0f is no better than "+
					"even spacing %.0f", gotErr, evenErr)
			}
		})
	}
}

// TestResampleStopsShape covers the contract the packing and the
// shader rely on: never more stops than the uniforms hold, sorted,
// and spanning the whole ramp.
func TestResampleStopsShape(t *testing.T) {
	for _, tc := range haloTestCurves() {
		t.Run(tc.name, func(t *testing.T) {
			var norm, sampled []GradientStop
			got := NormalizeGradientStopsInto(tc.h.stops(), &norm, &sampled)

			if len(got) > gradientShaderStopLimit {
				t.Fatalf("got %d stops, limit is %d",
					len(got), gradientShaderStopLimit)
			}
			for i := 1; i < len(got); i++ {
				if got[i].Pos < got[i-1].Pos {
					t.Fatalf("stop %d at %v precedes stop %d at %v",
						i, got[i].Pos, i-1, got[i-1].Pos)
				}
			}
			if got[0].Pos != 0 {
				t.Errorf("first stop at %v, want 0", got[0].Pos)
			}
			if got[len(got)-1].Pos != 1 {
				t.Errorf("last stop at %v, want 1", got[len(got)-1].Pos)
			}
		})
	}
}

// TestResampleStopsPinsPartialRange covers a list that stops short of
// both ends. SampleGradientStopColor clamps outside the stop range,
// but the shader interpolates outward from the first stop's position,
// so the resample has to spend budget on a flat stop at each missing
// end or the GPU extrapolates past the ramp.
func TestResampleStopsPinsPartialRange(t *testing.T) {
	// Long enough to be over the limit, so the resample runs at all.
	src := make([]GradientStop, gradientShaderStopLimit+4)
	for i := range src {
		u := float32(i) / float32(len(src)-1)
		src[i] = GradientStop{
			// A curve, not a line: a straight ramp would let any
			// placement through.
			Color: RGBA(uint8(255*(1-u*u)), uint8(255*u), 60, 255),
			Pos:   0.2 + 0.6*u,
		}
	}
	var norm, sampled []GradientStop
	got := NormalizeGradientStopsInto(src, &norm, &sampled)

	if len(got) > gradientShaderStopLimit {
		t.Fatalf("got %d stops, limit is %d",
			len(got), gradientShaderStopLimit)
	}
	if got[0].Pos != 0 || got[0].Color != src[0].Color {
		t.Errorf("first stop %v, want %v at 0", got[0], src[0].Color)
	}
	last := got[len(got)-1]
	if last.Pos != 1 || last.Color != src[len(src)-1].Color {
		t.Errorf("last stop %v, want %v at 1",
			last, src[len(src)-1].Color)
	}
	// The flat lead-in must actually be flat: the second stop has to
	// still sit where the source range starts, not somewhere the
	// ramp has already moved.
	if got[1].Pos != src[0].Pos || got[1].Color != src[0].Color {
		t.Errorf("second stop %v, want the source's first stop %v",
			got[1], src[0])
	}
}

// TestResampleStopsUnderLimitUntouched keeps the fast path honest: a
// list the uniforms can carry is returned as-is, with the sampled
// buffer left empty for the next caller.
func TestResampleStopsUnderLimitUntouched(t *testing.T) {
	src := []GradientStop{
		{Color: RGBA(10, 20, 30, 255), Pos: 0},
		{Color: RGBA(40, 50, 60, 255), Pos: 0.5},
		{Color: RGBA(70, 80, 90, 0), Pos: 1},
	}
	var norm, sampled []GradientStop
	got := NormalizeGradientStopsInto(src, &norm, &sampled)
	if len(got) != len(src) {
		t.Fatalf("got %d stops, want %d", len(got), len(src))
	}
	for i := range src {
		if got[i] != src[i] {
			t.Errorf("stop %d = %v, want %v", i, got[i], src[i])
		}
	}
	if len(sampled) != 0 {
		t.Errorf("sampled buffer holds %d stops, want 0", len(sampled))
	}
}

// TestResampleStopsNoAllocs holds the per-frame contract: the GPU
// backends call this once per gradient per frame with buffers they
// keep, so a warm call must not reach the heap.
func TestResampleStopsNoAllocs(t *testing.T) {
	src := haloTestCurves()[2].h.stops()
	var norm, sampled []GradientStop
	NormalizeGradientStopsInto(src, &norm, &sampled) // warm the buffers

	got := testing.AllocsPerRun(100, func() {
		NormalizeGradientStopsInto(src, &norm, &sampled)
	})
	if got != 0 {
		t.Errorf("resample allocated %.0f times per call, want 0", got)
	}
}
