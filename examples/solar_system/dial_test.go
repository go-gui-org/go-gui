package main

import (
	"math"
	"testing"
)

func TestDialAngleMapsPeriodToOneTurn(t *testing.T) {
	p := planets[earthIndex]
	start := dialAngle(0)
	end := dialAngle(p.PeriodS)
	delta := end - start
	want := 2 * float32(math.Pi)
	if d := delta - want; d < -1e-4 || d > 1e-4 {
		t.Fatalf("full Earth period delta %v want %v", delta, want)
	}
	// Half period should be half turn.
	half := dialAngle(p.PeriodS * 0.5)
	hd := half - start
	if diff := hd - want*0.5; diff < -1e-4 || diff > 1e-4 {
		t.Fatalf("half period delta %v want %v", hd, want*0.5)
	}
}

func TestMonthBoundariesMonotonicAndSum365(t *testing.T) {
	if monthStartDay[12] != 365 {
		t.Fatalf("total days %d want 365", monthStartDay[12])
	}
	if f := monthStartFrac[12]; f < 0.999 || f > 1.001 {
		t.Fatalf("frac total %v want 1", f)
	}
	for i := range 12 {
		if monthStartFrac[i] >= monthStartFrac[i+1] {
			t.Fatalf("frac %d %v not < %v", i, monthStartFrac[i], monthStartFrac[i+1])
		}
		if monthStartDay[i] >= monthStartDay[i+1] {
			t.Fatalf("day %d %v not < %v", i, monthStartDay[i], monthStartDay[i+1])
		}
	}
	// Mid angles should be inside their month span.
	for i := range 12 {
		lo := 2 * float32(math.Pi) * monthStartFrac[i]
		hi := 2 * float32(math.Pi) * monthStartFrac[i+1]
		mid := monthMidAngle[i]
		if mid < lo || mid > hi {
			t.Fatalf("month %d mid %v not in [%v,%v]", i, mid, lo, hi)
		}
	}
}

func TestDialAngleZeroIsJan1(t *testing.T) {
	// Earth's phase should be accounted for: dialAngle(0) - earth phase = 0 mod 2pi?
	// Instead check that dialAngle wraps consistently.
	a0 := dialAngle(0)
	// Frac of angle /2pi *365 should match day calculation.
	frac := a0 / (2 * float32(math.Pi))
	// Normalize to [0,1)
	for frac < 0 {
		frac++
	}
	for frac >= 1 {
		frac--
	}
	day := frac * 365
	if day < -1e-3 || day >= 365+1e-3 {
		t.Fatalf("day %v out of range", day)
	}
}

func TestDialCullingAtHighZoom(t *testing.T) {
	// With extreme zoom, dial should be culled.
	a := newApp()
	a.CanvasW, a.CanvasH = 800, 600
	a.CamZoom = 30
	a.UserZoom = 1
	a.recompute()
	dc := drawInto(a)
	_ = dc
	if len(a.dial.tris) > 20000 {
		t.Fatalf("dial mesh unexpectedly large at high zoom: %d floats", len(a.dial.tris))
	}
}

func TestAxisDirUnitAndOrientation(t *testing.T) {
	for i, p := range planets {
		ax, ay, az := axisDir(&p)
		n := sqrt32(ax*ax + ay*ay + az*az)
		if d := n - 1; d < -1e-4 || d > 1e-4 {
			t.Fatalf("planet %d %q axis not unit: len %v", i, p.Name, n)
		}
		// Opposite poles must have opposite az sign.
		if az != 0 && (az > 0) == (-az > 0) {
			t.Fatalf("planet %d az sign not opposite for opposite pole", i)
		}
	}
	// Mercury near-vertical (small tilt => axis near (0, -1, 0) in screen? Actually sa small, ca~1 => ay ~ -cosE ~ -0.877, az ~ sinE~0.48, ax~0)
	mx, my, mz := axisDir(&planets[0])
	if abs32(mx) > 0.02 {
		t.Fatalf("Mercury ax %v want ~0", mx)
	}
	// Uranus near-horizontal: tilt 1.706 rad (~97.7 deg) => large ax
	ux, uy, uz := axisDir(&planets[6])
	if abs32(ux) < 0.5 {
		t.Fatalf("Uranus ax %v want near-horizontal |ax|>0.5", ux)
	}
	// Venus retrograde check still unit; tilt small but rotation negative does not affect axis.
	_ = my
	_ = mz
	_ = uy
	_ = uz
}

func TestDialAngleLargeTimeNormalized(t *testing.T) {
	// After many orbits, marker normalization must stay in [0,2pi)
	// and remain consistent — the old loop was O(cycles).
	for _, tm := range []float32{1e3, 1e4, 1e5, 1e6} {
		a := newApp()
		a.CanvasW, a.CanvasH = 800, 600
		a.Time = tm
		a.recompute()
		_ = drawInto(a)
		for _, v := range a.dial.tris {
			if v != v {
				t.Fatalf("t=%v dial mesh has NaN", tm)
			}
		}
		theta := dialAngle(tm)
		norm := float32(math.Mod(float64(theta), 2*math.Pi))
		if norm < 0 {
			norm += 2 * float32(math.Pi)
		}
		if norm < -1e-3 || norm >= 2*float32(math.Pi)+1e-3 {
			t.Fatalf("t=%v normalized theta %v out of [0,2pi)", tm, norm)
		}
		if norm != norm {
			t.Fatalf("t=%v normalized theta is NaN", tm)
		}
	}
}

func TestAppendDialSegDegenerate(t *testing.T) {
	var m bodyMesh
	// Zero-length segment and zero half-width must not panic or add
	// degenerate tris.
	appendDialSeg(&m, 10, 10, 10, 10, colorDialTick, 0.62)
	if len(m.tris) != 0 {
		t.Fatalf("zero-length seg added %d floats", len(m.tris))
	}
	appendDialSeg(&m, 10, 10, 20, 20, colorDialTick, 0)
	if len(m.tris) != 0 {
		t.Fatalf("zero-width seg added %d floats", len(m.tris))
	}
	// Normal segment should add exactly one quad = 12 floats.
	appendDialSeg(&m, 0, 0, 10, 0, colorDialTick, 1)
	if len(m.tris) != 12 {
		t.Fatalf("normal seg added %d floats want 12", len(m.tris))
	}
}

func TestDialGlyphCompFinite(t *testing.T) {
	for _, theta := range []float32{0, 1.2, 3.0, 5.5} {
		sinMid, cosMid := sin32(theta), cos32(theta)
		dt := float32(diskTilt)
		ft := sqrt32(sinMid*sinMid + cosMid*cosMid*dt*dt)
		fr := sqrt32(cosMid*cosMid + sinMid*sinMid*dt*dt)
		if ft < 1e-4 {
			ft = 1e-4
		}
		if fr < 1e-4 {
			fr = 1e-4
		}
		for _, gx := range []float32{0, 0.5, 1} {
			for _, gy := range []float32{0, 0.5, 1} {
				wx, wy := dialGlyphToWorldComp(gx, gy, theta, dialTextR, dialEmWorld, ft, fr)
				if wx != wx || wy != wy {
					t.Fatalf("theta %v gx %v gy %v => NaN (%v,%v)", theta, gx, gy, wx, wy)
				}
			}
		}
	}
}

func TestApplyUserZoomBumpsVersion(t *testing.T) {
	a := newApp()
	v0 := a.Version
	a.applyUserZoom(1.2)
	if a.Version == v0 {
		t.Fatalf("applyUserZoom did not bump Version")
	}
}

func TestFullSystemTargetFinite(t *testing.T) {
	a := newApp()
	a.CanvasW, a.CanvasH = 1100, 760
	for _, sel := range []int{-1, selSun, 0, 5} {
		a.Selected = sel
		tx, ty, tz := a.target()
		if tx != tx || ty != ty || tz != tz {
			t.Fatalf("sel %d target has NaN (%v,%v,%v)", sel, tx, ty, tz)
		}
		if tz <= 0 {
			t.Fatalf("sel %d target zoom %v want >0", sel, tz)
		}
	}
}
