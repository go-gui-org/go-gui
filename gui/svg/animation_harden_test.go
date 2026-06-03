package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParseTimeValue_RejectsNonFinite(t *testing.T) {
	for _, in := range []string{"NaN", "nan", "inf", "Inf", "+Inf",
		"-Inf", "1e9999s", "1e9999ms", "1e9999"} {
		if v := parseTimeValue(in); v != 0 {
			t.Errorf("parseTimeValue(%q) = %v, want 0", in, v)
		}
	}
}

// NaN needs explicit guard; -Inf and +Inf fall through naturally.
func TestClampCycle_BoundsAndNaN(t *testing.T) {
	cases := []struct {
		in   float32
		want float32
	}{
		{float32(math.NaN()), 0},
		{float32(math.Inf(-1)), 0},
		{float32(math.Inf(1)), maxCycleSec},
	}
	for _, tc := range cases {
		if got := clampCycle(tc.in); got != tc.want {
			t.Errorf("clampCycle(%v) = %v, want %v",
				tc.in, got, tc.want)
		}
	}
}

func TestParseKeySplinesIfSpline_RejectsOutOfRange(t *testing.T) {
	cases := []string{
		`<animate calcMode="spline" keySplines="-0.1 0 1 1">`,
		`<animate calcMode="spline" keySplines="0 0 1.5 1">`,
		`<animate calcMode="spline" keySplines="0 0 NaN 1">`,
		`<animate calcMode="spline" keySplines="0 0 inf 1">`,
	}
	for _, elem := range cases {
		if got := parseKeySplinesIfSpline(elem, 2); got != nil {
			t.Errorf("parseKeySplinesIfSpline(%q) = %v, want nil",
				elem, got)
		}
	}
}

func TestParseKeySplinesIfSpline_AcceptsInRange(t *testing.T) {
	elem := `<animate calcMode="spline" keySplines="0 0 1 1">`
	got := parseKeySplinesIfSpline(elem, 2)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}

func TestPutAnimatedScratch_RejectsOversizeBuffer(t *testing.T) {
	p := New()
	huge := make([]VectorPath, 0, maxAnimatedScratchCap+1)
	p.putAnimatedScratch(huge)
	// Drain the pool a few times — sync.Pool may return nil
	// legitimately, but must never surface the oversize buffer.
	for range 4 {
		v := p.animatedScratch.Get()
		if v == nil {
			continue
		}
		buf, ok := v.(*[]VectorPath)
		if ok && cap(*buf) > maxAnimatedScratchCap {
			t.Fatalf("oversize buffer (cap=%d) parked in pool",
				cap(*buf))
		}
	}
}

func TestGetAnimatedScratch_CapsHugeMinCap(t *testing.T) {
	p := New()
	got := p.getAnimatedScratch(maxAnimatedScratchCap * 4)
	if cap(got) > maxAnimatedScratchCap {
		t.Fatalf("cap=%d, want <= %d",
			cap(got), maxAnimatedScratchCap)
	}
}

func TestGetAnimatedScratch_NegativeMinCapZeroed(t *testing.T) {
	p := New()
	got := p.getAnimatedScratch(-1)
	if cap(got) != 0 {
		t.Fatalf("cap=%d, want 0", cap(got))
	}
	if len(got) != 0 {
		t.Fatalf("len=%d, want 0", len(got))
	}
}

func TestPutAnimatedScratch_RetainsModestBuffer(t *testing.T) {
	p := New()
	buf := make([]VectorPath, 0, 8)
	p.putAnimatedScratch(buf)
	got := p.getAnimatedScratch(8)
	if cap(got) < 8 {
		t.Fatalf("getAnimatedScratch returned cap=%d, want >=8",
			cap(got))
	}
}

func TestParseAnimateDashArrayElement_UpperBoundFrames(t *testing.T) {
	var sb []byte
	for i := range maxKeyframes {
		if i > 0 {
			sb = append(sb, ';')
		}
		sb = append(sb, '1', ' ', '2')
	}
	elem := `<animate attributeName="stroke-dasharray" dur="1s" ` +
		`values="` + string(sb) + `">`
	a, ok := parseAnimateDashArrayElement(elem, ComputedStyle{GroupID: "g"})
	if !ok {
		t.Fatalf("expected ok=true at upper bound")
	}
	if a.Kind != gui.SvgAnimDashArray {
		t.Fatalf("kind=%d", a.Kind)
	}
}
