package gui

import (
	"math"
	"testing"
)

func TestSpinnerDefaultLayout(t *testing.T) {
	w := &Window{}
	v := Spinner(SpinnerCfg{ID: "s1"}, w)
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Axis != AxisLeftToRight {
		t.Error("default should be row")
	}
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
	cv := layout.Children[0]
	if cv.Shape.shapeType != shapeDrawCanvas {
		t.Errorf("child shapeType = %d, want DrawCanvas", cv.Shape.shapeType)
	}
}

func TestSpinnerConfigDefaults(t *testing.T) {
	w := &Window{}
	v := Spinner(SpinnerCfg{ID: "s2"}, w)
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Errorf("default width = %f, want 48", layout.Shape.Width)
	}
	if layout.Shape.Height != 48 {
		t.Errorf("default height = %f, want 48", layout.Shape.Height)
	}
}

func TestSpinnerCustomColor(t *testing.T) {
	w := &Window{}
	c := RGB(255, 0, 0)
	v := Spinner(SpinnerCfg{ID: "s3", Color: c}, w)
	layout := GenerateViewLayout(v, w)
	cv := layout.Children[0]
	if cv.Shape.events == nil || cv.Shape.events.OnDraw == nil {
		t.Fatal("OnDraw not set")
	}
}

func TestSpinnerExplicitZeroParam(t *testing.T) {
	w := &Window{}
	// ParamB explicitly set to 0 should NOT be overridden.
	v := Spinner(SpinnerCfg{
		ID:        "s4",
		CurveType: CurveLissajous,
		ParamB:    Some[float32](0),
	}, w)
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Errorf("width = %f, want 48", layout.Shape.Width)
	}
}

func TestSpinnerInvalidCurveTypeClamped(t *testing.T) {
	w := &Window{}
	// Should not panic with out-of-range CurveType.
	v := Spinner(SpinnerCfg{ID: "s5", CurveType: CurveType(200)}, w)
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Errorf("width = %f, want 48", layout.Shape.Width)
	}
}

func TestSpinnerFixedSizing(t *testing.T) {
	w := &Window{}
	v := Spinner(SpinnerCfg{
		ID:     "s6",
		Width:  200,
		Height: 100,
	}, w)
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Width != 200 {
		t.Errorf("width = %f, want 200", layout.Shape.Width)
	}
	if layout.Shape.Height != 100 {
		t.Errorf("height = %f, want 100", layout.Shape.Height)
	}
}

func TestSpinnerNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input float32
		want  float32
	}{
		{"positive", 2.7, 0.7},
		{"negative", -0.3, 0.7},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spinnerNormalize(tt.input)
			if math.Abs(float64(got-tt.want)) > 0.001 {
				t.Errorf("normalize(%f) = %f, want ~%f",
					tt.input, got, tt.want)
			}
		})
	}
}

func TestSpinnerClampPoint(t *testing.T) {
	nan := float32(math.NaN())
	tests := []struct {
		name         string
		inX, inY     float32
		wantX, wantY float32
	}{
		{"NaN", nan, nan, 0, 0},
		{"out_of_range", 5, -5, 0, 0},
		{"in_range", 0.5, -0.8, 0.5, -0.8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := spinnerClampPoint(tt.inX, tt.inY)
			if x != tt.wantX || y != tt.wantY {
				t.Errorf("clamp(%f, %f) = (%f, %f), want (%f, %f)",
					tt.inX, tt.inY, x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func isFinite(v float32) bool {
	return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
}

func TestSpinnerAllCurvesFullSweepFinite(t *testing.T) {
	for ct := CurveOriginalThinking; ct <= CurveFourier; ct++ {
		defs := spinnerCurveDefaults[ct]
		for i := range 101 {
			progress := float32(i) / 100
			x, y := spinnerCurvePoint(
				defs.family, progress, defs.a, defs.b, defs.d)
			if !isFinite(x) || !isFinite(y) {
				t.Errorf("curve %d at progress=%f: (%f, %f) not finite",
					ct, progress, x, y)
			}
			if x < -2 || x > 2 || y < -2 || y > 2 {
				t.Errorf("curve %d at progress=%f: (%f, %f) out of [-2,2]",
					ct, progress, x, y)
			}
		}
	}
}

func TestSpinnerCurvePointNaNFree(t *testing.T) {
	// Fuzz-style sweep with unusual params.
	params := []struct {
		family  spinnerFamily
		a, b, d float32
	}{
		{familyEpitrochoid, 0, 0, 0},
		{familyRose, 0, 5, 0},
		{familyHypotrochoid, 5, 0, 3},
		{familyHypotrochoid, 3, 3, 0},
		{familyCardioid, 0, 0, 0},
		{familyHeartWave, 6, -1, 0.5},
		{familyHeartWave, 6, 0, 0.5},
		{familySpiral, 4, 0, 0},
		{familyFourier, 0, 0, 0},
		{familyButterfly, 12, 2, 5},
		{familyButterfly, 0, 0, 0},
	}
	for _, p := range params {
		for i := range 101 {
			progress := float32(i) / 100
			x, y := spinnerCurvePoint(
				p.family, progress, p.a, p.b, p.d)
			if !isFinite(x) || !isFinite(y) {
				t.Errorf("family %d params(%f,%f,%f) progress=%f: "+
					"(%f,%f) not finite",
					p.family, p.a, p.b, p.d, progress, x, y)
			}
		}
	}
}

func TestSpinnerButterflyNegativeSinNoPanic(t *testing.T) {
	// progress=0.75 makes sin(t/12) negative with default turns=12.
	x, y := spinnerButterfly(0.75, 12, 2, 5)
	if !isFinite(x) || !isFinite(y) {
		t.Errorf("butterfly(0.75) = (%f, %f), not finite", x, y)
	}
}

func TestSpinnerCurveZeroParams(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (float32, float32)
	}{
		{"hypotrochoid_r=0", func() (float32, float32) {
			return spinnerHypotrochoid(0.5, 5, 0, 3)
		}},
		{"rose_a=0", func() (float32, float32) {
			return spinnerRose(0.5, 0, 5)
		}},
		{"cardioid_a=0", func() (float32, float32) {
			return spinnerCardioid(0.5, 0, 0)
		}},
		{"heartWave_root=-1", func() (float32, float32) {
			return spinnerHeartWave(0.5, 6, -1, 0.9)
		}},
		{"fourier_x1=0_y1=0", func() (float32, float32) {
			return spinnerFourier(0.5, 0, 0)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := tt.fn()
			if x != 0 || y != 0 {
				t.Errorf("got (%f, %f), want (0, 0)", x, y)
			}
		})
	}
}

func TestSpinnerDrawZeroSizeNoOp(t *testing.T) {
	dc := &DrawContext{Width: 0, Height: 0}
	spinnerDraw(dc, familyRose, 0.5, 0,
		60, 0.35, 2.5, 9, 5, 0, RGB(100, 100, 255))
}

func TestSpinnerDrawMinParticles(t *testing.T) {
	dc := &DrawContext{Width: 100, Height: 100}
	spinnerDraw(dc, familyRose, 0.5, 0,
		2, 0.35, 2.5, 9, 5, 0, RGB(100, 100, 255))
}

func TestSpinnerDrawMaxParticles(t *testing.T) {
	dc := &DrawContext{Width: 100, Height: 100}
	spinnerDraw(dc, familyLemniscate, 0.5, 0,
		500, 0.35, 2.5, 1, 0, 0, RGB(100, 100, 255))
}

func TestSpinnerParticlesClamped(t *testing.T) {
	w := &Window{}
	v := Spinner(SpinnerCfg{ID: "clamp", Particles: 10000}, w)
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Error("layout not generated")
	}
}

func TestSpinnerAnimationIsViewBound(t *testing.T) {
	w := &Window{}
	Spinner(SpinnerCfg{ID: "sp1"}, w)
	if w.animViewBound == nil {
		t.Fatal("animViewBound nil after Spinner — animation not view-bound")
	}
	if _, ok := w.animViewBound["spinner_sp1"]; !ok {
		t.Error("spinner animation not registered as view-bound")
	}
}
