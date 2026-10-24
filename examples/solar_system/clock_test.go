package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// TestMotionSmoothAfterLongRun guards the float32 clock bug. Time 1e5
// is about six days of wall clock at 1x. There a float32 Time step is
// larger than a whole tick, and a float32 orbit angle near 55000 rad
// steps by more than Earth moves in a tick, so the planet froze for
// some ticks and jumped on others. Each tick must move Earth by about
// the same distance.
func TestMotionSmoothAfterLongRun(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	a.Time = 1e5
	a.recompute()

	minD, maxD := float32(1e9), float32(0)
	for range 100 {
		x0, y0 := a.WorldX[earthIndex], a.WorldY[earthIndex]
		before := a.Time
		tick(a, wallTickSecs)
		if got, want := a.Time-before, float64(tickSecs); got < want*0.999 || got > want*1.001 {
			t.Fatalf("Time advanced %v in one tick, want %v", got, want)
		}
		dx, dy := a.WorldX[earthIndex]-x0, a.WorldY[earthIndex]-y0
		d := sqrt32(dx*dx + dy*dy)
		minD, maxD = min(minD, d), max(maxD, d)
	}
	if minD <= 0 || maxD/minD > 1.2 {
		t.Fatalf("per-tick Earth step ranges %v..%v; want steady motion", minD, maxD)
	}
}

func TestTurnsWrapsToUnitInterval(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want float64 }{
		{0, 0}, {0.25, 0.25}, {3.75, 0.75}, {-0.25, 0.75}, {1e9 + 0.5, 0.5},
	} {
		if got := turns(c.in); float64(got) < c.want-1e-6 || float64(got) > c.want+1e-6 {
			t.Fatalf("turns(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestWallStepMeasuresRealTime guards the frame-rate coupling bug: a
// slow frame must advance by the time it took, not a fixed 16 ms.
func TestWallStepMeasuresRealTime(t *testing.T) {
	t.Parallel()
	a := newApp()
	t0 := time.Unix(1000, 0)
	if got := a.wallStep(t0); got != wallTickSecs {
		t.Fatalf("first step %v, want nominal %v", got, wallTickSecs)
	}
	if got := a.wallStep(t0.Add(33 * time.Millisecond)); got < 0.0329 || got > 0.0331 {
		t.Fatalf("33 ms frame stepped %v, want 0.033", got)
	}
	if got := a.wallStep(t0.Add(5 * time.Second)); got != maxWallStep {
		t.Fatalf("5 s stall stepped %v, want cap %v", got, maxWallStep)
	}
	// A clock that steps backwards must not run the orrery in reverse.
	if got := a.wallStep(t0); got != 0 {
		t.Fatalf("backwards clock stepped %v, want 0", got)
	}
}

func TestTickScalesWithWallStep(t *testing.T) {
	t.Parallel()
	a, b := newTestApp(), newTestApp()
	tick(a, wallTickSecs)
	tick(b, 2*wallTickSecs)
	if got, want := b.Time, 2*a.Time; got < want*0.999 || got > want*1.001 {
		t.Fatalf("double wall step advanced %v, want %v", got, want)
	}
}

// TestScreenshotStartsNoAudio guards the -screenshot bug: RenderToPNG
// runs OnInit, and OnInit used to start the ambience track.
func TestScreenshotStartsNoAudio(t *testing.T) {
	before := ambienceStarts
	w := newWindow()
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := soft.RenderToPNG(w, 1, path); err != nil {
		t.Fatalf("RenderToPNG: %v", err)
	}
	if ambienceStarts != before {
		t.Fatalf("rendering a screenshot started the ambience track")
	}
}
