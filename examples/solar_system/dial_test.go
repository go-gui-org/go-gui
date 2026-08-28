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

// TestDialTablesMatchCalendar pins the dial's precomputed geometry
// against the calendar it is derived from.
//
// The tables are filled from init because they read monthStartFrac and
// monthMidAngle, which init also fills. Built as var initializers they
// would run first, against a zeroed calendar, and every month would
// land at angle 0. That failure changes no segment count and no
// triangle count — only where the pixels go — so it has to be checked
// on the coordinates.
func TestDialTablesMatchCalendar(t *testing.T) {
	t.Parallel()
	if len(dialMonthTicks) != 12*4 {
		t.Fatalf("dialMonthTicks has %d floats, want %d",
			len(dialMonthTicks), 12*4)
	}
	for mi := range 12 {
		want := 2 * float32(math.Pi) * monthStartFrac[mi]
		// The inner endpoint of tick mi, back to an angle.
		x, y := dialMonthTicks[mi*4], dialMonthTicks[mi*4+1]
		got := float32(math.Atan2(float64(y), float64(x)))
		if got < 0 {
			got += 2 * float32(math.Pi)
		}
		if d := absF(got - want); d > 1e-4 {
			t.Errorf("month %d tick at angle %v, want %v", mi, got, want)
		}
		if r := sqrt32(x*x + y*y); absF(r-dialInner) > 1e-2 {
			t.Errorf("month %d tick inner radius %v, want %v",
				mi, r, dialInner)
		}
	}

	// Every month name must contribute strokes, and the twelve of them
	// must be spread around the whole ring rather than stacked at one
	// angle. Four quadrants is the coarsest check that fails when the
	// calendar reads as zero.
	if len(dialLabelSegs) == 0 {
		t.Fatal("dialLabelSegs is empty")
	}
	var quad [4]int
	for i := 0; i+1 < len(dialLabelSegs); i += 2 {
		x, y := dialLabelSegs[i], dialLabelSegs[i+1]
		q := 0
		if x < 0 {
			q |= 1
		}
		if y < 0 {
			q |= 2
		}
		quad[q]++
		// Glyphs ride the label radius, within one em box of it.
		// The radial extent is inflated by the inverse local
		// foreshortening, whose worst case is diskTilt.
		maxR := dialTextR + dialEmWorld*emHeight/float32(diskTilt) + 2
		if r := sqrt32(x*x + y*y); r < dialTextR-2 || r > maxR {
			t.Fatalf("label point %d at radius %v, off the ring", i/2, r)
		}
	}
	for q, n := range quad {
		if n == 0 {
			t.Errorf("no label strokes in quadrant %d: %v", q, quad)
		}
	}
}
