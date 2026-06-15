package gui

import (
	"math"
	"testing"
)

// approxEq32 compares float32 within a small absolute tolerance.
// Phase math is hand-computed so 1e-5 catches any drift but tolerates
// the float32 accumulator inside cssIterPhase.
func approxEq32(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func TestSmilPhase_SinglePlayActiveDuringDur(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 1, Cycle: 0}
	ok, frac, act := smilPhase(a, 1.5)
	if !ok {
		t.Fatal("expected active during dur")
	}
	if !approxEq32(frac, 0.25, 1e-6) {
		t.Errorf("frac=%v want 0.25", frac)
	}
	if act != 1 {
		t.Errorf("activation=%v want 1", act)
	}
}

func TestSmilPhase_SinglePlayInactiveAfterDurNoFreeze(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 1, Cycle: 0}
	ok, _, _ := smilPhase(a, 5)
	if ok {
		t.Fatal("expected inactive past dur with no freeze")
	}
}

func TestSmilPhase_SinglePlayFreezeAfterDur(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 1, Cycle: 0, Freeze: true}
	ok, frac, _ := smilPhase(a, 5)
	if !ok || frac != 1 {
		t.Fatalf("expected frozen at frac=1; got ok=%v frac=%v", ok, frac)
	}
}

func TestSmilPhase_CycleRefiresActivation(t *testing.T) {
	// Cycle=3, third cycle starts at t=7.
	a := &SvgAnimation{DurSec: 2, BeginSec: 1, Cycle: 3,
		Restart: SvgAnimRestartAlways}
	ok, frac, act := smilPhase(a, 7.5)
	if !ok {
		t.Fatal("expected active in third cycle")
	}
	if !approxEq32(act, 7, 1e-6) {
		t.Errorf("activation=%v want 7", act)
	}
	if !approxEq32(frac, 0.25, 1e-6) {
		t.Errorf("frac=%v want 0.25", frac)
	}
}

func TestSmilPhase_RestartWhenNotActiveHoldsCurrentCycle(t *testing.T) {
	// Without WhenNotActive, t=4 lands in cycle n=1 (activation=4).
	// WhenNotActive: prior cycle activation=1, t-prev=3 < dur=4, so n
	// is decremented and activation stays at 1.
	a := &SvgAnimation{DurSec: 4, BeginSec: 1, Cycle: 3,
		Restart: SvgAnimRestartWhenNotActive}
	_, _, act := smilPhase(a, 4)
	if !approxEq32(act, 1, 1e-6) {
		t.Errorf("activation=%v want 1 (held by WhenNotActive)", act)
	}
}

func TestSmilPhase_RestartNeverIgnoresCycle(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 1, Cycle: 3,
		Restart: SvgAnimRestartNever, Freeze: true}
	// activation stays at BeginSec; t=10 gives phase=9 > dur, freeze.
	ok, frac, act := smilPhase(a, 10)
	if act != 1 {
		t.Errorf("activation=%v want 1 (RestartNever)", act)
	}
	if !ok || frac != 1 {
		t.Errorf("expected frozen; ok=%v frac=%v", ok, frac)
	}
}

func TestCssIterPhase_BeforeBeginFillBackwards(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 5, Iterations: 3}
	ok, frac, act := cssIterPhase(a, 1)
	if !ok || frac != 0 || act != 5 {
		t.Errorf("pre-begin: ok=%v frac=%v act=%v want true 0 5",
			ok, frac, act)
	}
}

func TestCssIterPhase_BeforeBeginAlternateOddIterations(t *testing.T) {
	// Iterations=2 with alternate: last iteration is reversed, so
	// the pre-begin "0%" pose lerps to 1.
	a := &SvgAnimation{DurSec: 2, BeginSec: 5, Iterations: 2,
		Alternate: true}
	_, frac, _ := cssIterPhase(a, 0)
	if frac != 1 {
		t.Errorf("alternate pre-begin frac=%v want 1", frac)
	}
}

func TestCssIterPhase_AlternateFlipsOddIterPhase(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 0, Iterations: 4,
		Alternate: true}
	// t=2.5: iter=1, iterPhase=0.5, frac before flip = 0.25.
	// Alternate: iter%2==1 → frac = 1-0.25 = 0.75.
	_, frac, _ := cssIterPhase(a, 2.5)
	if !approxEq32(frac, 0.75, 1e-6) {
		t.Errorf("frac=%v want 0.75", frac)
	}
}

func TestCssIterPhase_FreezeAtFinalIteration(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 0, Iterations: 2,
		Freeze: true}
	// t=10 well past 2 iterations → freeze at frac=1.
	ok, frac, _ := cssIterPhase(a, 10)
	if !ok || frac != 1 {
		t.Errorf("freeze: ok=%v frac=%v want true 1", ok, frac)
	}
}

func TestCssIterPhase_NoFreezeInactiveAfterIters(t *testing.T) {
	a := &SvgAnimation{DurSec: 2, BeginSec: 0, Iterations: 2}
	ok, _, _ := cssIterPhase(a, 10)
	if ok {
		t.Fatal("expected inactive past iterations with no freeze")
	}
}

func TestCssIterPhase_InfiniteRunsForever(t *testing.T) {
	a := &SvgAnimation{DurSec: 1, BeginSec: 0,
		Iterations: SvgAnimIterInfinite}
	ok, frac, _ := cssIterPhase(a, 1_000_000.5)
	if !ok {
		t.Fatal("infinite should always be active")
	}
	if !approxEq32(frac, 0.5, 1e-3) {
		t.Errorf("frac=%v want ~0.5", frac)
	}
}

func TestUnpackRGBA_BitOrder(t *testing.T) {
	got := unpackRGBA(0x11_22_33_44)
	want := SvgColor{R: 0x11, G: 0x22, B: 0x33, A: 0x44}
	if got != want {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestLerpU8_Boundaries(t *testing.T) {
	if lerpU8(10, 200, 0) != 10 {
		t.Error("t=0 should return a")
	}
	if lerpU8(10, 200, 1) != 200 {
		t.Error("t=1 should return b")
	}
	if lerpU8(10, 200, -1) != 10 {
		t.Error("t<0 should clamp to a")
	}
	if lerpU8(10, 200, 2) != 200 {
		t.Error("t>1 should clamp to b")
	}
	got := lerpU8(0, 100, 0.5)
	if got < 49 || got > 51 {
		t.Errorf("midpoint got %d want ~50", got)
	}
}

func TestLerpU8_NaNTreatedAsClamp(t *testing.T) {
	// NaN explicitly caught and clamped to the first endpoint (a).
	got := lerpU8(50, 200, float32(math.NaN()))
	if got != 50 {
		t.Errorf("NaN frac got %d want 50 (clamped to a)", got)
	}
}

func TestLerpColorKeyframes_TwoStopsLinear(t *testing.T) {
	// 0xFF000000 (red, A=0) → 0x00FF00FF (green, A=255) at t=0.5.
	got := lerpColorKeyframes(
		[]uint32{0xFF000000, 0x00FF00FF}, nil, nil,
		SvgAnimCalcLinear, 0.5)
	// Each channel ~halfway.
	if got.R < 126 || got.R > 129 {
		t.Errorf("R=%d want ~127", got.R)
	}
	if got.G < 126 || got.G > 129 {
		t.Errorf("G=%d want ~127", got.G)
	}
	if got.A < 126 || got.A > 129 {
		t.Errorf("A=%d want ~127", got.A)
	}
}

func TestLerpColorKeyframes_DiscreteStepsAcrossStops(t *testing.T) {
	stops := []uint32{0x11223344, 0x55667788, 0x99AABBCC}
	// locateSeg discrete: idx = int(frac*n), so for n=3:
	// frac=0.1→0, frac=0.5→1, frac=0.7→2.
	cases := []struct {
		frac float32
		want uint32
	}{
		{0.1, stops[0]},
		{0.5, stops[1]},
		{0.7, stops[2]},
	}
	for _, tc := range cases {
		got := lerpColorKeyframes(stops, nil, nil,
			SvgAnimCalcDiscrete, tc.frac)
		if got != unpackRGBA(tc.want) {
			t.Errorf("frac=%v got %+v want %#x",
				tc.frac, got, tc.want)
		}
	}
}

func TestLerpColorKeyframes_AtEndReturnsLast(t *testing.T) {
	stops := []uint32{0x11223344, 0xAABBCCDD}
	got := lerpColorKeyframes(stops, nil, nil, SvgAnimCalcLinear, 1)
	if got != unpackRGBA(stops[1]) {
		t.Errorf("frac=1 got %+v want last stop", got)
	}
}

func TestLerpColorKeyframes_FracClampedAboveOne(t *testing.T) {
	stops := []uint32{0x10203040, 0xA0B0C0D0}
	got := lerpColorKeyframes(stops, nil, nil, SvgAnimCalcLinear, 5)
	if got != unpackRGBA(stops[1]) {
		t.Errorf("frac>1 should snap to last; got %+v", got)
	}
}
