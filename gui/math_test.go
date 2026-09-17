package gui

import (
	"math"
	"testing"
)

func TestIntClamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		v, lo, hi int
		want      int
	}{
		{"below min", -10, 0, 5, 0},
		{"above max", 10, 0, 5, 5},
		{"within range", 3, 0, 5, 3},
		{"on min", 0, 0, 5, 0},
		{"on max", 5, 0, 5, 5},
		{"negative within", -3, -5, -1, -3},
		{"negative below", -10, -5, -1, -5},
		{"negative above", 0, -5, -1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := intClamp(tt.v, tt.lo, tt.hi); got != tt.want {
				t.Errorf("intClamp(%d, %d, %d) = %d, want %d",
					tt.v, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}

func TestF32Clamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		v, lo, hi float32
		want      float32
	}{
		{"below min", -1.5, 0.0, 2.5, 0.0},
		{"above max", 3.14, 0.0, 2.5, 2.5},
		{"within range", 1.25, 0.0, 2.5, 1.25},
		{"on min", 0.0, 0.0, 2.0, 0.0},
		{"on max", 2.0, 0.0, 2.0, 2.0},
		{"negative within", -3.0, -5.0, -1.0, -3.0},
		{"negative below", -10.0, -5.0, -1.0, -5.0},
		{"negative above", 0.0, -5.0, -1.0, -1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := f32Clamp(tt.v, tt.lo, tt.hi); got != tt.want {
				t.Errorf("f32Clamp(%f, %f, %f) = %f, want %f",
					tt.v, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}

func TestF32AreClose(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b float32
		want bool
	}{
		{"within tolerance", 1.00, 1.005, true},
		{"within negative", -2.50, -2.507, true},
		{"at boundary", 10.00, 10.009, true},
		{"at boundary negative", -3.33, -3.339, true},
		{"outside tolerance", 0.0, 0.02, false},
		{"outside negative", -1.0, -1.02, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := f32AreClose(tt.a, tt.b); got != tt.want {
				t.Errorf("f32AreClose(%f, %f) = %v, want %v",
					tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestF32Mod(t *testing.T) {
	t.Parallel()
	nan := float32(math.NaN())
	tests := []struct {
		name    string
		x, y    float32
		want    float32
		wantNaN bool
	}{
		{"positive", 10, 360, 10, false},
		{"wrap", 370, 360, 10, false},
		{"negative keeps dividend sign", -10, 360, -10, false},
		{"negative wrap", -370, 360, -10, false},
		{"sector", 4.5, 6, 4.5, false},
		{"sector wrap", 7.5, 6, 1.5, false},
		{"large quotient finite", float32(1e20), 360, float32(math.Mod(float64(float32(1e20)), 360)), false},
		{"zero divisor", 10, 0, 0, true},
		{"zero dividend", 0, 360, 0, false},
		{"nan dividend", nan, 360, 0, true},
		{"nan divisor", 10, nan, 0, true},
		{"inf dividend", float32(math.Inf(1)), 360, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := f32Mod(tt.x, tt.y)
			if tt.wantNaN {
				if !math.IsNaN(float64(got)) {
					t.Errorf("f32Mod(%v, %v) = %v, want NaN",
						tt.x, tt.y, got)
				}
				return
			}
			if math.IsNaN(float64(got)) || got != tt.want {
				t.Errorf("f32Mod(%v, %v) = %v, want %v",
					tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestF32MinMaxNaNPropagates(t *testing.T) {
	t.Parallel()
	nan := float32(math.NaN())
	if got := f32Min(nan, 1); !math.IsNaN(float64(got)) {
		t.Errorf("f32Min(NaN, 1) = %v, want NaN", got)
	}
	if got := f32Min(1, nan); !math.IsNaN(float64(got)) {
		t.Errorf("f32Min(1, NaN) = %v, want NaN", got)
	}
	if got := f32Max(nan, 1); !math.IsNaN(float64(got)) {
		t.Errorf("f32Max(NaN, 1) = %v, want NaN", got)
	}
	if got := f32Max(1, nan); !math.IsNaN(float64(got)) {
		t.Errorf("f32Max(1, NaN) = %v, want NaN", got)
	}
	if got := f32Min(2, 5); got != 2 {
		t.Errorf("f32Min(2, 5) = %v, want 2", got)
	}
	if got := f32Max(2, 5); got != 5 {
		t.Errorf("f32Max(2, 5) = %v, want 5", got)
	}
	if got := f32Min(float32(math.Inf(1)), 1); got != 1 {
		t.Errorf("f32Min(+Inf, 1) = %v, want 1", got)
	}
	if got := f32Max(float32(math.Inf(-1)), 1); got != 1 {
		t.Errorf("f32Max(-Inf, 1) = %v, want 1", got)
	}
}

func TestF32ClampNaNPassthrough(t *testing.T) {
	t.Parallel()
	nan := float32(math.NaN())
	if got := f32Clamp(nan, 0, 1); !math.IsNaN(float64(got)) {
		t.Errorf("f32Clamp(NaN, 0, 1) = %v, want NaN passthrough", got)
	}
	if got := f64Clamp(math.NaN(), 0, 1); !math.IsNaN(got) {
		t.Errorf("f64Clamp(NaN, 0, 1) = %v, want NaN passthrough", got)
	}
}
