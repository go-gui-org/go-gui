package gui

import (
	"math"
	"testing"
	"time"
)

func TestMathSpinnerDefaultLayout(t *testing.T) {
	w := &Window{}
	v := MathSpinner(MathSpinnerCfg{ID: "s1"}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Axis != axisLeftToRight {
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

func TestMathSpinnerConfigDefaults(t *testing.T) {
	w := &Window{}
	v := MathSpinner(MathSpinnerCfg{ID: "s2"}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Errorf("default width = %f, want 48", layout.Shape.Width)
	}
	if layout.Shape.Height != 48 {
		t.Errorf("default height = %f, want 48", layout.Shape.Height)
	}
}

func TestMathSpinnerCustomColor(t *testing.T) {
	w := &Window{}
	c := RGB(255, 0, 0)
	v := MathSpinner(MathSpinnerCfg{ID: "s3", Color: c}, w)
	layout := generateViewLayout(v, w)
	cv := layout.Children[0]
	if cv.Shape.events == nil || cv.Shape.events.OnDraw == nil {
		t.Fatal("OnDraw not set")
	}
}

func TestMathSpinnerExplicitZeroParam(t *testing.T) {
	w := &Window{}
	// ParamB explicitly set to 0 should NOT be overridden.
	v := MathSpinner(MathSpinnerCfg{
		ID:        "s4",
		CurveType: CurveLissajous,
		ParamB:    Some[float32](0),
	}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Errorf("width = %f, want 48", layout.Shape.Width)
	}
}

func TestMathSpinnerInvalidCurveTypeClamped(t *testing.T) {
	w := &Window{}
	// Should not panic with out-of-range CurveType.
	v := MathSpinner(MathSpinnerCfg{ID: "s5", CurveType: CurveType(200)}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Errorf("width = %f, want 48", layout.Shape.Width)
	}
}

func TestMathSpinnerFixedSizing(t *testing.T) {
	w := &Window{}
	v := MathSpinner(MathSpinnerCfg{
		ID:     "s6",
		Width:  200,
		Height: 100,
	}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 200 {
		t.Errorf("width = %f, want 200", layout.Shape.Width)
	}
	if layout.Shape.Height != 100 {
		t.Errorf("height = %f, want 100", layout.Shape.Height)
	}
}

func TestMathSpinnerNormalize(t *testing.T) {
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
			got := mathSpinnerNormalize(tt.input)
			if math.Abs(float64(got-tt.want)) > 0.001 {
				t.Errorf("normalize(%f) = %f, want ~%f",
					tt.input, got, tt.want)
			}
		})
	}
}

func TestMathSpinnerClampPoint(t *testing.T) {
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
			x, y := mathSpinnerClampPoint(tt.inX, tt.inY)
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

func TestMathSpinnerAllCurvesFullSweepFinite(t *testing.T) {
	for ct := CurveOriginalThinking; ct <= CurveFourier; ct++ {
		defs := mathSpinnerCurveDefaults[ct]
		for i := range 101 {
			progress := float32(i) / 100
			x, y := mathSpinnerCurvePoint(
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

func TestMathSpinnerCurvePointNaNFree(t *testing.T) {
	// Fuzz-style sweep with unusual params.
	params := []struct {
		family  mathSpinnerFamily
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
			x, y := mathSpinnerCurvePoint(
				p.family, progress, p.a, p.b, p.d)
			if !isFinite(x) || !isFinite(y) {
				t.Errorf("family %d params(%f,%f,%f) progress=%f: "+
					"(%f,%f) not finite",
					p.family, p.a, p.b, p.d, progress, x, y)
			}
		}
	}
}

func TestMathSpinnerButterflyNegativeSinNoPanic(t *testing.T) {
	// progress=0.75 makes sin(t/12) negative with default turns=12.
	x, y := mathSpinnerButterfly(0.75, 12, 2, 5)
	if !isFinite(x) || !isFinite(y) {
		t.Errorf("butterfly(0.75) = (%f, %f), not finite", x, y)
	}
}

func TestMathSpinnerCurveZeroParams(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (float32, float32)
	}{
		{"hypotrochoid_r=0", func() (float32, float32) {
			return mathSpinnerHypotrochoid(0.5, 5, 0, 3)
		}},
		{"rose_a=0", func() (float32, float32) {
			return mathSpinnerRose(0.5, 0, 5)
		}},
		{"cardioid_a=0", func() (float32, float32) {
			return mathSpinnerCardioid(0.5, 0, 0)
		}},
		{"heartWave_root=-1", func() (float32, float32) {
			return mathSpinnerHeartWave(0.5, 6, -1, 0.9)
		}},
		{"fourier_x1=0_y1=0", func() (float32, float32) {
			return mathSpinnerFourier(0.5, 0, 0)
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

func TestMathSpinnerDrawZeroSizeNoOp(t *testing.T) {
	dc := &DrawContext{Width: 0, Height: 0}
	mathSpinnerDraw(dc, familyRose, 0.5, 0,
		60, 0.35, 2.5, 9, 5, 0, RGB(100, 100, 255))
}

func TestMathSpinnerDrawMinParticles(t *testing.T) {
	dc := &DrawContext{Width: 100, Height: 100}
	mathSpinnerDraw(dc, familyRose, 0.5, 0,
		2, 0.35, 2.5, 9, 5, 0, RGB(100, 100, 255))
}

func TestMathSpinnerDrawMaxParticles(t *testing.T) {
	dc := &DrawContext{Width: 100, Height: 100}
	mathSpinnerDraw(dc, familyLemniscate, 0.5, 0,
		500, 0.35, 2.5, 1, 0, 0, RGB(100, 100, 255))
}

func TestMathSpinnerParticlesClamped(t *testing.T) {
	w := &Window{}
	v := MathSpinner(MathSpinnerCfg{ID: "clamp", Particles: 10000}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 48 {
		t.Error("layout not generated")
	}
}

func TestMathSpinnerAnimationIsViewBound(t *testing.T) {
	w := &Window{}
	MathSpinner(MathSpinnerCfg{ID: "sp1"}, w)
	if w.animViewBound == nil {
		t.Fatal("animViewBound nil after MathSpinner — animation not view-bound")
	}
	if _, ok := w.animViewBound["math_spinner:sp1"]; !ok {
		t.Error("math_spinner animation not registered as view-bound")
	}
}

func TestMathSpinnerSanitizeSpeed(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	tests := []struct {
		name  string
		input float32
		want  float32
	}{
		{"normal", 2, 2},
		{"zero", 0, 1},
		{"negative", -3, 1},
		{"nan", nan, 1},
		{"pos inf", inf, 1},
		{"neg inf", float32(math.Inf(-1)), 1},
	}
	for _, tt := range tests {
		if got := mathSpinnerSanitizeSpeed(tt.input); got != tt.want {
			t.Errorf("%s: sanitize(%v) = %v, want %v",
				tt.name, tt.input, got, tt.want)
		}
	}
}

func TestMathSpinnerDuration(t *testing.T) {
	if got := mathSpinnerDuration(1); got != 5*time.Second {
		t.Errorf("speed 1 = %v, want 5s", got)
	}
	if got := mathSpinnerDuration(2); got != 2500*time.Millisecond {
		t.Errorf("speed 2 = %v, want 2.5s", got)
	}
	// Extreme speeds clamp instead of producing a zero or
	// overflowing Duration.
	if got := mathSpinnerDuration(1e30); got <= 0 {
		t.Errorf("huge speed = %v, want positive clamp", got)
	}
	if got := mathSpinnerDuration(1e-30); got <= 0 {
		t.Errorf("tiny speed = %v, want positive clamp", got)
	}
	if got := mathSpinnerDuration(mathSpinnerSanitizeSpeed(
		float32(math.NaN()))); got != 5*time.Second {
		t.Errorf("NaN speed = %v, want 5s", got)
	}
}

func TestMathSpinnerBadSpeedStillAnimates(t *testing.T) {
	for _, speed := range []float32{0, -2,
		float32(math.NaN()), float32(math.Inf(1))} {
		w := &Window{}
		v := MathSpinner(MathSpinnerCfg{ID: "bad", Speed: speed}, w)
		layout := generateViewLayout(v, w)
		if layout.Shape.Width != 48 {
			t.Errorf("speed %v: width = %f, want 48",
				speed, layout.Shape.Width)
		}
		if _, ok := w.animViewBound["math_spinner:bad"]; !ok {
			t.Errorf("speed %v: animation not registered", speed)
		}
	}
}

func TestMathSpinnerAsymmetricSize(t *testing.T) {
	w := &Window{}
	v := MathSpinner(MathSpinnerCfg{ID: "asym", Width: 200}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Width != 200 {
		t.Errorf("width = %f, want 200", layout.Shape.Width)
	}
	if layout.Shape.Height != 48 {
		t.Errorf("height = %f, want 48 fallback", layout.Shape.Height)
	}
}

func TestMathSpinnerFourierSilentX(t *testing.T) {
	// x1 = 0 used to zero the y axis too through the shared norm.
	found := false
	for i := range 101 {
		progress := float32(i) / 100
		x, y := mathSpinnerFourier(progress, 0, 15)
		if x != 0 {
			t.Fatalf("x1=0: x = %f, want 0", x)
		}
		if !isFinite(y) {
			t.Fatalf("x1=0: y = %f, want finite", y)
		}
		if math.Abs(float64(y)) > 0.01 {
			found = true
		}
	}
	if !found {
		t.Error("x1=0: y stayed ~0 all sweep, the live axis is lost")
	}
}

func TestMathSpinnerFourierAxesBounded(t *testing.T) {
	// Per-axis norms bound each axis to [-1,1] by construction.
	for i := range 101 {
		progress := float32(i) / 100
		x, y := mathSpinnerFourier(progress, 17, 15)
		if math.Abs(float64(x)) > 1.001 ||
			math.Abs(float64(y)) > 1.001 {
			t.Fatalf("progress=%f: (%f, %f) escapes [-1,1]",
				progress, x, y)
		}
	}
}

func TestMathSpinnerButterflyFractionalPower(t *testing.T) {
	// A fractional power must stay on the positive branch: Pow ran
	// on |sin|, and only exact odd integers restore the sign.
	x, y := mathSpinnerButterfly(0.75, 12, 2, 5.5)
	if !isFinite(x) || !isFinite(y) {
		t.Fatalf("butterfly(power=5.5) = (%f, %f), want finite", x, y)
	}
	oddX, oddY := mathSpinnerButterfly(0.75, 12, 2, 5)
	if x == oddX && y == oddY {
		t.Error("fractional power renders identically to odd power")
	}
}

func TestMathSpinnerGhostCacheDeterministic(t *testing.T) {
	first := mathSpinnerCachedGhost(familyRose, 9, 5, 0)
	second := mathSpinnerCachedGhost(familyRose, 9, 5, 0)
	if first != second {
		t.Error("same params returned different cache entries")
	}
	other := mathSpinnerCachedGhost(familyRose, 9, 2, 0)
	if other == first {
		t.Error("different params returned the same cache entry")
	}
	// Cached points match a direct evaluation.
	px, py := mathSpinnerRose(0.5, 9, 5)
	if first[200] != px || first[201] != py {
		t.Errorf("cached mid %f,%f != direct %f,%f",
			first[200], first[201], px, py)
	}
}

func TestMathSpinnerFadeTable(t *testing.T) {
	if got := mathSpinnerFade(0); got != 0 {
		t.Errorf("fade(0) = %f, want 0", got)
	}
	if got := mathSpinnerFade(1); got != 1 {
		t.Errorf("fade(1) = %f, want 1", got)
	}
	if got := mathSpinnerFade(float32(math.NaN())); got != 0 {
		t.Errorf("fade(NaN) = %f, want 0", got)
	}
	for i := range 101 {
		r := float32(i) / 100
		want := float32(math.Pow(float64(r), 0.56))
		if got := mathSpinnerFade(r); math.Abs(float64(got-want)) > 0.005 {
			t.Fatalf("fade(%f) = %f, want ~%f", r, got, want)
		}
	}
}

func TestMathSpinnerDrawSingleParticle(t *testing.T) {
	// Direct callers may pass 1 particle; the trail spacing must
	// not divide by zero.
	dc := &DrawContext{Width: 100, Height: 100}
	mathSpinnerDraw(dc, familyRose, 0.5, 0,
		1, 0.35, 2.5, 9, 5, 0, RGB(100, 100, 255))
}

func TestMathSpinnerHostileSizes(t *testing.T) {
	// NaN and +Inf pass a plain `<= 0` guard, so before
	// mathSpinnerPositive they reached the layout tree and the
	// stroke radius intact.
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, bad := range []float32{nan, inf, 0, -5} {
		w := &Window{}
		v := MathSpinner(MathSpinnerCfg{
			ID:          "hostile",
			Size:        bad,
			Width:       bad,
			Height:      bad,
			StrokeWidth: bad,
			TrailLength: bad,
		}, w)
		layout := generateViewLayout(v, w)
		if layout.Shape.Width != 48 || layout.Shape.Height != 48 {
			t.Errorf("%v: size = %f x %f, want 48 x 48",
				bad, layout.Shape.Width, layout.Shape.Height)
		}
	}
}

func TestMathSpinnerPositive(t *testing.T) {
	tests := []struct {
		name   string
		v, def float32
		want   float32
	}{
		{"finite positive", 3, 7, 3},
		{"zero", 0, 7, 7},
		{"negative", -1, 7, 7},
		{"nan", float32(math.NaN()), 7, 7},
		{"pos inf", float32(math.Inf(1)), 7, 7},
		{"neg inf", float32(math.Inf(-1)), 7, 7},
	}
	for _, tt := range tests {
		if got := mathSpinnerPositive(tt.v, tt.def); got != tt.want {
			t.Errorf("%s: mathSpinnerPositive(%v, %v) = %v, want %v",
				tt.name, tt.v, tt.def, got, tt.want)
		}
	}
}

func TestMathSpinnerGhostCacheEviction(t *testing.T) {
	// Filling past the bound must keep serving correct points,
	// not panic or hand back another entry's curve.
	for i := range mathSpinnerGhostCacheSize + 4 {
		pts := mathSpinnerCachedGhost(
			familyRose, float32(20+i), 5, 0)
		px, py := mathSpinnerRose(0.5, float32(20+i), 5)
		if pts[200] != px || pts[201] != py {
			t.Fatalf("entry %d: cached %f,%f != direct %f,%f",
				i, pts[200], pts[201], px, py)
		}
	}
	if len(mathSpinnerGhostCache.pts) > mathSpinnerGhostCacheSize {
		t.Errorf("cache holds %d entries, want <= %d",
			len(mathSpinnerGhostCache.pts),
			mathSpinnerGhostCacheSize)
	}
}
