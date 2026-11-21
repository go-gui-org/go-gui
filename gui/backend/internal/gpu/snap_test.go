package gpu

import (
	"math"
	"testing"
)

func TestSnapRect(t *testing.T) {
	cases := []struct {
		name           string
		x, y, w, h, s  float32
		wx, wy, ww, wh float32
	}{
		{"whole", 10, 10, 40, 20, 1, 10, 10, 40, 20},
		{"half rounds edges, not size", 10.5, 10.5, 40, 20, 1, 11, 11, 40, 20},
		{"quarter", 10.25, 10.75, 40, 20, 1, 10, 11, 40, 20},
		{"size from edges", 10.4, 10, 40.2, 20, 1, 10, 10, 41, 20},
		{"scale 1.25", 10, 10, 31, 31, 1.25, 13, 13, 38, 38},
		{"scale 2", 10.25, 10, 40, 20, 2, 21, 20, 80, 40},
		{"sub-pixel keeps 1px", 10.1, 10.1, 0.2, 0.2, 1, 10, 10, 1, 1},
		{"zero stays zero", 10, 10, 0, 0, 1, 10, 10, 0, 0},
		{"negative origin", -10.5, -3.25, 20, 6, 1, -10, -3, 20, 6},
	}
	for _, c := range cases {
		x, y, w, h := SnapRect(c.x, c.y, c.w, c.h, c.s)
		if x != c.wx || y != c.wy || w != c.ww || h != c.wh {
			t.Errorf("%s: SnapRect = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
				c.name, x, y, w, h, c.wx, c.wy, c.ww, c.wh)
		}
	}
}

// Boxes that share a logical edge must share a device edge after
// snapping, or a gap or overlap opens between them.
func TestSnapRectAdjacentBoxesMeet(t *testing.T) {
	for _, s := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, x := range []float32{0, 0.3, 0.5, 7.7} {
			const w = 13.3
			ax, _, aw, _ := SnapRect(x, 0, w, 1, s)
			bx, _, _, _ := SnapRect(x+w, 0, w, 1, s)
			if ax+aw != bx {
				t.Errorf("s=%v x=%v: left box ends at %v, right starts at %v",
					s, x, ax+aw, bx)
			}
		}
	}
}

func TestSnapRectPassesNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	if x, _, _, _ := SnapRect(nan, 0, 1, 1, 1); x == x {
		t.Errorf("NaN x snapped to %v, want NaN for the caller's guard", x)
	}
	if _, _, w, _ := SnapRect(0, 0, inf, 1, 1); !math.IsInf(float64(w), 0) &&
		w == w {
		t.Errorf("Inf w snapped to finite %v, want non-finite", w)
	}
}

func TestSnapThickness(t *testing.T) {
	cases := []struct{ th, s, want float32 }{
		{0, 1, 0}, // zero means fill
		{0, 2, 0},
		{1, 1, 1},
		{1, 1.25, 1},
		{1, 1.5, 2},
		{1, 2, 2},
		{0.25, 1, 1}, // a hairline stays visible
		{2, 1.25, 3}, // 2.5 rounds up
	}
	for _, c := range cases {
		if got := SnapThickness(c.th, c.s); got != c.want {
			t.Errorf("SnapThickness(%v, %v) = %v, want %v", c.th, c.s, got, c.want)
		}
	}
	if got := SnapThickness(float32(math.NaN()), 1); got == got {
		t.Errorf("NaN thickness snapped to %v, want NaN", got)
	}
}

func TestSnapAllocs(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() {
		SnapRect(10.5, 3.25, 40, 20, 1.25)
		SnapThickness(1, 1.25)
	}); n != 0 {
		t.Errorf("allocs = %v, want 0", n)
	}
}
