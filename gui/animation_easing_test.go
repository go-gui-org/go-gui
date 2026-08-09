package gui

import (
	"math"
	"testing"
)

func TestLerpBoundaries(t *testing.T) {
	if lerp(10, 20, 0) != 10 {
		t.Error("Lerp(10,20,0) should be 10")
	}
	if lerp(10, 20, 1) != 20 {
		t.Error("Lerp(10,20,1) should be 20")
	}
}

func TestLerpMidpoint(t *testing.T) {
	got := lerp(0, 100, 0.5)
	if got != 50 {
		t.Errorf("Lerp(0,100,0.5) = %f, want 50", got)
	}
}

func TestEasingEndpoints(t *testing.T) {
	fns := []struct {
		name string
		fn   EasingFn
	}{
		{"Linear", EaseLinear},
		{"InQuad", easeInQuad},
		{"OutQuad", EaseOutQuad},
		{"InOutQuad", EaseInOutQuad},
		{"InCubic", easeInCubic},
		{"OutCubic", EaseOutCubic},
		{"InOutCubic", easeInOutCubic},
		{"OutBounce", EaseOutBounce},
	}
	for _, tt := range fns {
		v0 := tt.fn(0)
		v1 := tt.fn(1)
		if math.Abs(float64(v0)) > 0.001 {
			t.Errorf("%s(0) = %f, want ~0", tt.name, v0)
		}
		if math.Abs(float64(v1)-1) > 0.001 {
			t.Errorf("%s(1) = %f, want ~1", tt.name, v1)
		}
	}
}

func TestEaseInBackEndpoints(t *testing.T) {
	if easeInBack(0) != 0 {
		t.Error("EaseInBack(0) != 0")
	}
	if math.Abs(float64(easeInBack(1))-1) > 0.001 {
		t.Errorf("EaseInBack(1) = %f", easeInBack(1))
	}
}

func TestEaseOutBackEndpoints(t *testing.T) {
	if math.Abs(float64(easeOutBack(0))-0) > 0.001 {
		t.Errorf("EaseOutBack(0) = %f", easeOutBack(0))
	}
	if math.Abs(float64(easeOutBack(1))-1) > 0.001 {
		t.Errorf("EaseOutBack(1) = %f", easeOutBack(1))
	}
}

func TestEaseOutElasticEndpoints(t *testing.T) {
	if EaseOutElastic(0) != 0 {
		t.Error("EaseOutElastic(0) != 0")
	}
	if EaseOutElastic(1) != 1 {
		t.Error("EaseOutElastic(1) != 1")
	}
}

func TestBezierLUT(t *testing.T) {
	lut := buildBezierLUT(0.25, 0.1, 0.25, 1.0)
	if lut.lookup(0) != 0 {
		t.Error("LUT(0) != 0")
	}
	if lut.lookup(1) != 1 {
		t.Error("LUT(1) != 1")
	}
	mid := lut.lookup(0.5)
	if mid <= 0 || mid >= 1 {
		t.Errorf("LUT(0.5) = %f, expected in (0,1)", mid)
	}
}

func TestEaseCSSEndpoints(t *testing.T) {
	fns := []struct {
		name string
		fn   EasingFn
	}{
		{"CSS", easeCSS},
		{"InCSS", easeInCSS},
		{"OutCSS", easeOutCSS},
		{"InOutCSS", easeInOutCSS},
	}
	for _, tt := range fns {
		if tt.fn(0) != 0 {
			t.Errorf("%s(0) != 0", tt.name)
		}
		if tt.fn(1) != 1 {
			t.Errorf("%s(1) != 1", tt.name)
		}
	}
}

func TestCubicBezierFactory(t *testing.T) {
	fn := cubicBezier(0.25, 0.1, 0.25, 1.0)
	v := fn(0.5)
	if v <= 0 || v >= 1 {
		t.Errorf("CubicBezier(0.5) = %f", v)
	}
}

func TestEaseOutBounceMonotonic(t *testing.T) {
	// Not strictly monotonic, but should end at 1
	if math.Abs(float64(EaseOutBounce(1))-1) > 0.001 {
		t.Errorf("EaseOutBounce(1) = %f", EaseOutBounce(1))
	}
}

func TestBezierLUTClamping(t *testing.T) {
	lut := buildBezierLUT(0.25, 0.1, 0.25, 1.0)
	if lut.lookup(-1) != 0 {
		t.Error("LUT(-1) should clamp to 0")
	}
	if lut.lookup(2) != 1 {
		t.Error("LUT(2) should clamp to 1")
	}
}

func TestEaseInOutCubicSymmetry(t *testing.T) {
	v := easeInOutCubic(0.5)
	if math.Abs(float64(v)-0.5) > 0.01 {
		t.Errorf("EaseInOutCubic(0.5) = %f, want ~0.5", v)
	}
}
