package gpu

import (
	"math"
	"testing"
)

func TestClipRectRoundsOutward(t *testing.T) {
	tests := []struct {
		name              string
		x, y, w, h, scale float32
		cx, cy, cw, ch    int32
	}{
		{
			// Exact at scale 1: no inflation.
			name: "integer rect scale 1",
			x:    10, y: 20, w: 100, h: 40, scale: 1,
			cx: 10, cy: 20, cw: 100, ch: 40,
		},
		{
			// 125% DPI, origin at zero: the extent scales exactly.
			name: "scale 1.25 at origin",
			x:    0, y: 0, w: 100, h: 40, scale: 1.25,
			cx: 0, cy: 0, cw: 125, ch: 50,
		},
		{
			// The reported bug: far edge 165.75 must ceil to 166
			// against an origin floored to 15, giving width 151.
			// Truncating each term gives 150 — a pixel short.
			name: "fractional origin and scale",
			x:    10.5, y: 10.5, w: 100, h: 40, scale: 1.5,
			cx: 15, cy: 15, cw: 151, ch: 61,
		},
		{
			// Zero-size clip stays zero-size: backends read that
			// as clip-everything, so it must not inflate to 1px.
			name: "zero size",
			x:    12.5, y: 12.5, w: 0, h: 0, scale: 1.25,
			cx: 15, cy: 15, cw: 0, ch: 0,
		},
		{
			// Negative origin floors below zero; the extent stays
			// positive so the scissor call is still valid.
			name: "negative origin",
			x:    -10.5, y: -0.25, w: 20, h: 10, scale: 1,
			cx: -11, cy: -1, cw: 21, ch: 11,
		},
		{
			// Finite but huge: clamped, not wrapped.
			name: "huge coordinate clamps",
			x:    0, y: 0, w: 1e30, h: 1e30, scale: 2,
			cx: 0, cy: 0, cw: 1 << 24, ch: 1 << 24,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cx, cy, cw, ch := ClipRect(
				tt.x, tt.y, tt.w, tt.h, tt.scale)
			if cx != tt.cx || cy != tt.cy ||
				cw != tt.cw || ch != tt.ch {
				t.Errorf("ClipRect = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					cx, cy, cw, ch, tt.cx, tt.cy, tt.cw, tt.ch)
			}
		})
	}
}

// The device rect must contain the float rect at every sampled
// fractional scale — the property the per-term truncation broke.
func TestClipRectContainsFloatRect(t *testing.T) {
	scales := []float32{1, 1.25, 1.5, 1.75, 2, 2.5, 3}
	for _, s := range scales {
		for i := range 20 {
			x := float32(i) * 0.37
			y := float32(i) * 0.61
			w := float32(i)*1.3 + 0.5
			h := float32(i)*0.9 + 0.5
			cx, cy, cw, ch := ClipRect(x, y, w, h, s)
			if float32(cx) > x*s || float32(cy) > y*s {
				t.Fatalf("scale %v i %d: origin (%d,%d) inside (%v,%v)",
					s, i, cx, cy, x*s, y*s)
			}
			if float32(cx+cw) < (x+w)*s ||
				float32(cy+ch) < (y+h)*s {
				t.Fatalf("scale %v i %d: far edge (%d,%d) short of (%v,%v)",
					s, i, cx+cw, cy+ch, (x+w)*s, (y+h)*s)
			}
		}
	}
}

// Hostile scales and coordinates must degrade to an empty clip, never
// to a reversed rect (GL_INVALID_VALUE) or a panic.
func TestClipRectHostileInputs(t *testing.T) {
	inf := float32(math.Inf(1))
	nan := float32(math.NaN())
	tests := []struct {
		name              string
		x, y, w, h, scale float32
	}{
		{"NaN scale", 0, 0, 10, 10, nan},
		{"negative scale", 0, 0, 10, 10, -2},
		{"zero scale", 5, 5, 10, 10, 0},
		{"NaN origin", nan, nan, 10, 10, 2},
		{"inf extent", 0, 0, inf, inf, 2},
		{"negative extent", 10, 10, -5, -5, 2},
		{"huge negative origin", -1e30, -1e30, 10, 10, 2},
		{"negative infinity origin", -inf, -inf, 10, 10, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cx, cy, cw, ch := ClipRect(
				tt.x, tt.y, tt.w, tt.h, tt.scale)
			if cw < 0 || ch < 0 {
				t.Errorf("negative extent: (%d,%d)", cw, ch)
			}
			// Origin plus extent must stay inside int32 so the
			// backends' physH-y-h flip cannot wrap.
			if int64(cx) < -(1<<26) || int64(cy) < -(1<<26) ||
				int64(cx)+int64(cw) > 1<<26 ||
				int64(cy)+int64(ch) > 1<<26 {
				t.Errorf("unbounded rect: (%d,%d,%d,%d)",
					cx, cy, cw, ch)
			}
		})
	}
}

// A NaN or negative scale must produce a fully empty clip, not a
// partial one that lets stray pixels through.
func TestClipRectBadScaleIsEmpty(t *testing.T) {
	for _, s := range []float32{float32(math.NaN()), -1, 0} {
		_, _, cw, ch := ClipRect(3.5, 3.5, 10, 10, s)
		if cw != 0 || ch != 0 {
			t.Errorf("scale %v: extent (%d,%d), want (0,0)",
				s, cw, ch)
		}
	}
}
