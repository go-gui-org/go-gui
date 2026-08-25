package main

import (
	"math"
	"os"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestMain pins the theme once, before any parallel test runs. The
// theme mirrors are process-global and are not safe to write while
// other tests read them from GenerateLayout.
func TestMain(m *testing.M) {
	gui.SetTheme(gui.ThemeDark)
	os.Exit(m.Run())
}

// tweenTicks is the number of ticks one camera transition takes.
const tweenTicks = 47 // camTweenSecs / tickSecs, rounded up

// newTestApp is a fully seeded app with a known canvas size, so camera
// and hit-test math is exercised without a real frame pass.
func newTestApp() *App {
	a := newApp()
	a.CanvasW, a.CanvasH = windowW, windowH
	a.advanceCamera(0)
	a.recompute()
	return a
}

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	w := gui.NewWindow(gui.WindowCfg{
		State:  newApp(),
		Width:  windowW,
		Height: windowH,
	})
	_ = mainView(w).GenerateLayout(w)
}

func TestSelectedViewNoPanic(t *testing.T) {
	t.Parallel()
	app := newApp()
	app.Selected = saturnIndex
	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Width:  windowW,
		Height: windowH,
	})
	_ = mainView(w).GenerateLayout(w)
}

// TestSunPanelNoPanic builds the view with the sun selected, which is
// the one selection that is not a planet index.
func TestSunPanelNoPanic(t *testing.T) {
	t.Parallel()
	app := newApp()
	app.Selected = selSun
	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Width:  windowW,
		Height: windowH,
	})
	_ = mainView(w).GenerateLayout(w)
}

// TestPlanetPeriodsOrdered is the headline behavior: Mercury fastest,
// Neptune slowest, with no ties anywhere between.
func TestPlanetPeriodsOrdered(t *testing.T) {
	t.Parallel()
	for i := 1; i < len(planets); i++ {
		if planets[i].PeriodS <= planets[i-1].PeriodS {
			t.Errorf("%s period %.1f not greater than %s period %.1f",
				planets[i].Name, planets[i].PeriodS,
				planets[i-1].Name, planets[i-1].PeriodS)
		}
		if planets[i].OrbitA <= planets[i-1].OrbitA {
			t.Errorf("%s orbit %.1f not greater than %s orbit %.1f",
				planets[i].Name, planets[i].OrbitA,
				planets[i-1].Name, planets[i-1].OrbitA)
		}
	}
}

// TestOrbitPosStaysOnEllipse checks the orbit is closed and that the
// sun sits on the focus, not the center: the near and far distances
// must differ by the eccentricity.
func TestOrbitPosStaysOnEllipse(t *testing.T) {
	t.Parallel()
	p := &planets[0] // Mercury, the most eccentric here
	var near, far float32 = 1e9, 0
	for i := range 720 {
		x, y := orbitPos(p, float32(i)*p.PeriodS/720)
		d := sqrt32(x*x + y*y)
		near = min(near, d)
		far = max(far, d)
	}
	wantNear := p.OrbitA * (1 - p.Ecc)
	wantFar := p.OrbitA * (1 + p.Ecc)
	if !closeEnough(near, wantNear, 0.5) {
		t.Errorf("perihelion = %.2f, want %.2f", near, wantNear)
	}
	if !closeEnough(far, wantFar, 0.5) {
		t.Errorf("aphelion = %.2f, want %.2f", far, wantFar)
	}
}

func TestHitTestPicksNearest(t *testing.T) {
	t.Parallel()
	a := newTestApp()

	// Two overlapping bodies at known screen positions; the nearer
	// center must win even though the other is earlier in the table.
	a.ScreenX[0], a.ScreenY[0], a.ScreenR[0] = 100, 100, 20
	a.ScreenX[1], a.ScreenY[1], a.ScreenR[1] = 110, 100, 20
	for i := 2; i < len(planets); i++ {
		a.ScreenX[i], a.ScreenY[i], a.ScreenR[i] = -1000, -1000, 1
	}
	if got := a.hitTest(108, 100); got != 1 {
		t.Errorf("hitTest near planet 1 = %d, want 1", got)
	}
	if got := a.hitTest(98, 100); got != 0 {
		t.Errorf("hitTest near planet 0 = %d, want 0", got)
	}
	if got := a.hitTest(400, 400); got != -1 {
		t.Errorf("hitTest on empty space = %d, want -1", got)
	}
}

// TestHitTestMinRadius covers the reason minHitR exists: at full-system
// zoom Mercury is under a pixel wide and would be unclickable.
func TestHitTestMinRadius(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	for i := range planets {
		a.ScreenX[i], a.ScreenY[i], a.ScreenR[i] = -1000, -1000, 1
	}
	a.ScreenX[0], a.ScreenY[0], a.ScreenR[0] = 200, 200, 0.4
	if got := a.hitTest(206, 200); got != 0 {
		t.Errorf("hitTest %d, want 0 (min hit radius should apply)", got)
	}
}

// TestSelectionWrapsBothWays walks the whole cycle, which is the sun
// followed by the planets sun-outward. The sun's selection value is
// outside the planet index range, so the walk cannot be plain
// arithmetic on Selected and this is what pins that.
func TestSelectionWrapsBothWays(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	last := len(planets) - 1

	stepSelection(a, 1)
	if a.Selected != selSun {
		t.Fatalf("first Right = %d, want the sun (%d)", a.Selected, selSun)
	}
	stepSelection(a, 1)
	if a.Selected != 0 {
		t.Fatalf("Right from the sun = %d, want 0", a.Selected)
	}
	stepSelection(a, -1)
	if a.Selected != selSun {
		t.Errorf("Left from 0 = %d, want the sun (%d)", a.Selected, selSun)
	}
	stepSelection(a, -1)
	if a.Selected != last {
		t.Errorf("Left from the sun = %d, want %d", a.Selected, last)
	}
	stepSelection(a, 1)
	if a.Selected != selSun {
		t.Errorf("Right from last = %d, want the sun (%d)", a.Selected, selSun)
	}

	a.Selected = -1
	stepSelection(a, -1)
	if a.Selected != last {
		t.Errorf("first Left = %d, want %d", a.Selected, last)
	}
}

// TestSunIsSelectable covers the whole sun path end to end: the disc
// hit-tests, the camera frames it, and the fact sheet resolves.
func TestSunIsSelectable(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	tick(a)

	if got := a.hitTest(a.SunX, a.SunY); got != selSun {
		t.Fatalf("hitTest at the sun = %d, want %d", got, selSun)
	}
	selectBody(a, selSun)
	for range tweenTicks + 4 {
		tick(a)
	}
	// The sun sits at the world origin, so a settled camera is there
	// too apart from the info-panel lift.
	if !closeEnough(a.CamX, 0, 0.01) {
		t.Errorf("CamX = %.3f after focusing the sun, want 0", a.CamX)
	}
	tx, ty, tz := a.target()
	if !closeEnough(a.CamY, ty, 0.01) || !closeEnough(a.CamZoom, tz, 0.01) ||
		tx != 0 {
		t.Errorf("camera (%.3f,%.3f,%.3f), want (%.3f,%.3f,%.3f)",
			a.CamX, a.CamY, a.CamZoom, tx, ty, tz)
	}
	if b := bodyAt(a.Selected); b == nil || b != &sun {
		t.Errorf("bodyAt(%d) did not resolve to the sun", a.Selected)
	}
}

// TestBodyAtRange pins the encoding bodyAt decodes: only a planet index
// and selSun name a body.
func TestBodyAtRange(t *testing.T) {
	t.Parallel()
	if bodyAt(-1) != nil {
		t.Error("bodyAt(-1) should be nil: nothing is selected")
	}
	if bodyAt(len(planets)) != nil {
		t.Error("bodyAt(len(planets)) should be nil: out of range")
	}
	if bodyAt(selSun) != &sun {
		t.Error("bodyAt(selSun) should be the sun")
	}
	for i := range planets {
		if bodyAt(i) != &planets[i] {
			t.Errorf("bodyAt(%d) is not planets[%d]", i, i)
		}
	}
}

// TestSunFactsPopulated guards the fact sheet against a body with
// blank cards: every stat the six cards read must be filled.
func TestSunFactsPopulated(t *testing.T) {
	t.Parallel()
	for _, f := range [...]string{
		sun.Name, sun.Diameter, sun.Mass, sun.Distance,
		sun.DayLength, sun.Gravity, sun.Temperature, sun.FunFact,
	} {
		if f == "" {
			t.Error("the sun has an empty fact-sheet field")
		}
	}
}

// TestCameraTweenReachesTarget drives real ticks and asserts the camera
// has settled on the live target once the tween duration has elapsed.
func TestCameraTweenReachesTarget(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	selectBody(a, 4) // Jupiter

	for range tweenTicks + 4 {
		tick(a)
	}
	if a.TweenT != 1 {
		t.Fatalf("TweenT = %v, want 1", a.TweenT)
	}
	tx, ty, tz := a.target()
	if !closeEnough(a.CamX, tx, 0.01) || !closeEnough(a.CamY, ty, 0.01) ||
		!closeEnough(a.CamZoom, tz, 0.01) {
		t.Errorf("camera (%.3f,%.3f,%.3f), want (%.3f,%.3f,%.3f)",
			a.CamX, a.CamY, a.CamZoom, tx, ty, tz)
	}
}

// TestCameraFollowsOrbitingPlanet is the moving-target property: the
// target is re-read every tick, so a selected planet stays centered as
// it keeps orbiting rather than being left behind.
func TestCameraFollowsOrbitingPlanet(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	selectBody(a, 0) // Mercury, the fastest

	for range 2*tweenTicks + 200 {
		tick(a)
	}
	wx, _ := orbitPos(&planets[0], a.Time)
	if !closeEnough(a.CamX, wx, 0.5) {
		t.Errorf("CamX = %.2f after orbiting, want ~%.2f", a.CamX, wx)
	}
	// And on screen it should be near the horizontal center.
	if !closeEnough(a.ScreenX[0], a.CanvasW/2, 2) {
		t.Errorf("ScreenX = %.1f, want ~%.1f", a.ScreenX[0], a.CanvasW/2)
	}
}

// TestInterruptedTransitionIsContinuous covers beginTransition: a new
// selection mid-tween must start from where the camera actually is, not
// snap back to the previous anchor.
func TestInterruptedTransitionIsContinuous(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	selectBody(a, 7)
	for range 10 {
		tick(a)
	}
	midX, midZoom := a.CamX, a.CamZoom

	selectBody(a, 1)
	if a.FromX != midX || a.FromZoom != midZoom {
		t.Errorf("transition origin (%.3f,%.3f), want current camera "+
			"(%.3f,%.3f)", a.FromX, a.FromZoom, midX, midZoom)
	}
	tick(a)
	// One tick in, the camera has barely moved from the interruption
	// point — no jump.
	if !closeEnough(a.CamZoom, midZoom, absF(midZoom)*0.25+0.05) {
		t.Errorf("CamZoom jumped from %.3f to %.3f", midZoom, a.CamZoom)
	}
}

func TestUserZoomClamped(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	for range 100 {
		a.applyUserZoom(2)
	}
	if a.UserZoom != userZoomMax {
		t.Errorf("UserZoom = %v, want %v", a.UserZoom, userZoomMax)
	}
	for range 100 {
		a.applyUserZoom(0.5)
	}
	if a.UserZoom != userZoomMin {
		t.Errorf("UserZoom = %v, want %v", a.UserZoom, userZoomMin)
	}
}

// TestTooltipClampedToCanvas puts the cursor in each corner and asserts
// the bubble's text still lands inside the canvas.
func TestTooltipClampedToCanvas(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	a.MouseIn = true
	a.Hovered = 4

	const cw, ch float32 = 400, 300
	corners := [][2]float32{{0, 0}, {cw, 0}, {0, ch}, {cw, ch}}
	for _, c := range corners {
		dc := gui.NewDrawContext(cw, ch, nil)
		a.MouseX, a.MouseY = c[0], c[1]
		drawTooltip(a, dc)
		texts := dc.Texts()
		if len(texts) != 1 {
			t.Fatalf("cursor %v: got %d text entries, want 1", c, len(texts))
		}
		if texts[0].X < 0 || texts[0].Y < 0 ||
			texts[0].X > cw || texts[0].Y > ch {
			t.Errorf("cursor %v: tooltip at (%.1f,%.1f) escaped %vx%v",
				c, texts[0].X, texts[0].Y, cw, ch)
		}
	}
}

// TestStarLevelsBounded guards the bucketing that keeps the starfield
// at starAlphaLevels batches instead of one per star.
func TestStarLevelsBounded(t *testing.T) {
	t.Parallel()
	a := newApp()
	for step := range 200 {
		tm := float32(step) * 0.05
		for i := range a.Stars {
			s := &a.Stars[i]
			b := s.Base + s.Amp*sin32(tm*s.Speed+s.Phase)
			if lvl := starLevel(b); lvl < 0 || lvl >= starAlphaLevels {
				t.Fatalf("starLevel(%v) = %d, out of range", b, lvl)
			}
		}
	}
}

func closeEnough(got, want, tol float32) bool {
	return absF(got-want) <= tol
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestShadingFacesTheSun sweeps every planet through a full orbit and
// checks the shading's light direction points at the sun's *drawn*
// position, in both the full-system view and a planet-focused one.
//
// The check is worth having because the vertical squash makes the world
// direction and the screen direction differ: a planet directly below
// the sun in world space is drawn only 0.48x that far below it, so
// anything computing the highlight in world coordinates would drift
// everywhere except on the horizontal axis.
func TestShadingFacesTheSun(t *testing.T) {
	t.Parallel()
	check := func(t *testing.T, a *App) {
		t.Helper()
		sx, sy := a.worldToScreen(0, 0)
		for i := range planets {
			// The screen-plane part of the light vector, normalized —
			// the same two components drawBody builds its frame from.
			vx, vy, _ := a.lightVec(i)
			m := sqrt32(vx*vx + vy*vy)
			// Expected direction, computed independently of lightVec.
			dx, dy := sx-a.ScreenX[i], sy-a.ScreenY[i]
			d := sqrt32(dx*dx + dy*dy)
			if d < 1 || m < 1e-4 {
				continue // degenerate: planet drawn on top of the sun
			}
			lx, ly := vx/m, vy/m
			// Dot product of two unit vectors: 1 is exact agreement.
			if dot := lx*dx/d + ly*dy/d; dot < 0.9999 {
				t.Errorf("%s at t=%.1f: light dir (%.3f,%.3f) does not "+
					"point at the sun (%.3f,%.3f), dot=%.4f",
					planets[i].Name, a.Time, lx, ly, dx/d, dy/d, dot)
			}
		}
	}

	a := newTestApp()
	// 240 samples spans more than a full orbit for every planet except
	// Neptune, and well past the bottom-of-the-arc case for the inner
	// ones — the position where a top-facing highlight looks wrong but
	// is right, because the sun is genuinely above.
	for range 240 {
		for range 30 {
			tick(a)
		}
		check(t, a)
	}

	b := newTestApp()
	selectBody(b, 4)
	for range 240 {
		for range 30 {
			tick(b)
		}
		check(t, b)
	}
}

// TestLightVecDegenerate covers a planet sitting on the sun's own world
// position, where the light direction is undefined. lightVec must
// answer with the camera axis rather than dividing by zero, and the
// answer must still be a unit vector — drawBody's degenerate check
// keys on its screen-plane length being zero.
func TestLightVecDegenerate(t *testing.T) {
	saved := planets[0]
	defer func() { planets[0] = saved }()
	planets[0].OrbitA = 0
	planets[0].Ecc = 0

	a := newTestApp()
	lx, ly, lz := a.lightVec(0)
	if got := lx*lx + ly*ly + lz*lz; !closeEnough(got, 1, 1e-3) {
		t.Errorf("degenerate light vec is not a unit vector: %v", got)
	}
	if m := sqrt32(lx*lx + ly*ly); m >= 1e-4 {
		t.Errorf("degenerate light has a screen-plane direction: %v", m)
	}
}

// TestLitFractionTracksOrbit pins the phase term to the geometry it
// claims to model: darkest on the near side (drawn low on the screen,
// sun beyond it), brightest on the far side, and exactly half lit at
// the two quarter points where the sun is off to the side.
func TestLitFractionTracksOrbit(t *testing.T) {
	a := newTestApp()

	// Earth is nearly circular, so its world Y is a clean proxy for
	// which side of the orbit it is on.
	const idx = 2
	p := &planets[idx]

	var minK, maxK float32 = 2, -1
	var atMinY, atMaxY float32
	var minY, maxY float32 = 1e9, -1e9

	const samples = 360
	for i := range samples {
		a.Time = p.PeriodS * float32(i) / samples
		_, wy := orbitPos(p, a.Time)
		k := a.litFraction(idx)

		minK = min(minK, k)
		maxK = max(maxK, k)
		if wy < minY {
			minY, atMinY = wy, k
		}
		if wy > maxY {
			maxY, atMaxY = wy, k
		}
	}

	// +y is screen-down, the near side: that is where the night side
	// faces the viewer.
	// Not exact equality: eccentricity means the extreme world Y and the
	// extreme Y/|p| fall on neighbouring samples.
	if !closeEnough(atMaxY, minK, 1e-3) {
		t.Errorf("near side (max world Y) is not the darkest: k=%v, min=%v",
			atMaxY, minK)
	}
	if !closeEnough(atMinY, maxK, 1e-3) {
		t.Errorf("far side (min world Y) is not the brightest: k=%v, max=%v",
			atMinY, maxK)
	}

	// The camera is above the plane, not in it, so a pure new or full
	// phase is never seen. Those bounds are (1 +/- cosElev) / 2.
	wantMin := (1 - cosElev) / 2
	wantMax := (1 + cosElev) / 2
	if !closeEnough(minK, wantMin, 1e-3) {
		t.Errorf("darkest phase = %v, want %v", minK, wantMin)
	}
	if !closeEnough(maxK, wantMax, 1e-3) {
		t.Errorf("brightest phase = %v, want %v", maxK, wantMax)
	}
}

// TestLitFractionHalfAtQuarters checks the quarter phase: with the sun
// directly to one side, half the visible disc is lit, whichever planet
// and whichever side.
func TestLitFractionHalfAtQuarters(t *testing.T) {
	a := newTestApp()

	for i := range planets {
		p := &planets[i]
		for s := range 2000 {
			a.Time = p.PeriodS * float32(s) / 2000
			_, wy := orbitPos(p, a.Time)
			if absF(wy) > p.OrbitA*0.002 {
				continue // not near a quarter point
			}
			if k := a.litFraction(i); !closeEnough(k, 0.5, 0.01) {
				t.Errorf("%s at quarter phase: k = %v, want 0.5",
					p.Name, k)
			}
		}
	}
}

// TestLitFractionDegenerate covers the sun-position guard, which no
// real orbit reaches but the divide would otherwise produce NaN for.
func TestLitFractionDegenerate(t *testing.T) {
	a := newTestApp()
	saved := planets[0]
	defer func() { planets[0] = saved }()

	planets[0].OrbitA = 0
	planets[0].Ecc = 0
	if k := a.litFraction(0); k != 1 {
		t.Errorf("degenerate litFraction = %v, want 1", k)
	}
}

// TestLightBasisOrthonormal pins the frame the sphere shading is built
// in. Everything downstream assumes it: that the three axes are unit
// and mutually perpendicular is what makes cos(phi) equal N·L, and that
// v has no depth is what lets point skip a z term for it.
func TestLightBasisOrthonormal(t *testing.T) {
	a := newTestApp()
	for i := range planets {
		for s := range 90 {
			a.Time = planets[i].PeriodS * float32(s) / 90
			b := newLightBasis(a.lightVec(i))

			unit := func(name string, x, y, z float32) {
				if n := x*x + y*y + z*z; !closeEnough(n, 1, 1e-4) {
					t.Fatalf("%s: |%s|² = %v, want 1", planets[i].Name, name, n)
				}
			}
			unit("l", b.lx, b.ly, b.lz)
			unit("u", b.ux, b.uy, b.uz)
			unit("v", b.vx, b.vy, 0)

			perp := func(name string, d float32) {
				if !closeEnough(d, 0, 1e-4) {
					t.Fatalf("%s: %s = %v, want 0", planets[i].Name, name, d)
				}
			}
			perp("l·u", b.lx*b.ux+b.ly*b.uy+b.lz*b.uz)
			perp("l·v", b.lx*b.vx+b.ly*b.vy)
			perp("u·v", b.ux*b.vx+b.uy*b.vy)

			// u carries all the depth the light's perpendicular has, so
			// it must lean toward the viewer, never away.
			if b.uz < 0 {
				t.Fatalf("%s: u leans away from the viewer (uz=%v)",
					planets[i].Name, b.uz)
			}
		}
	}
}

// TestShadePointsStayInsideDisc is the property that lets the shading
// skip a clip: every surface point it emits projects inside the
// silhouette, hidden ones included, because those are pushed out onto
// the limb rather than left where they fell.
func TestShadePointsStayInsideDisc(t *testing.T) {
	a := newTestApp()
	const r = 64
	for i := range planets {
		a.Time = planets[i].PeriodS * 0.37
		b := newLightBasis(a.lightVec(i))
		for j := range 41 {
			c := -1 + 2*float32(j)/40
			sn := sqrt32(1 - c*c)
			for k := range 72 {
				ang := 2 * math.Pi * float32(k) / 72
				x, y := b.point(r, c, sn, cos32(ang), sin32(ang))
				if d := sqrt32(x*x + y*y); d > r+1e-3 {
					t.Fatalf("%s: point at cos=%v t=%v is %v from center, "+
						"outside r=%v", planets[i].Name, c, ang, d, r)
				}
			}
		}
	}
}

// TestVisibleAzimuthMatchesDepth checks the closed form against the
// thing it stands in for: inside the returned half-width the surface
// point faces the viewer, outside it does not.
func TestVisibleAzimuthMatchesDepth(t *testing.T) {
	a := newTestApp()
	a.Time = 7.3
	for i := range planets {
		b := newLightBasis(a.lightVec(i))
		for j := 1; j < 40; j++ {
			c := -1 + 2*float32(j)/40
			sn := sqrt32(1 - c*c)
			tMax := b.visibleAzimuth(c, sn)

			// z(t) = cos(phi)*lz + sin(phi)*cos(t)*uz, largest at t=0
			// and smallest at t=pi, so the two probes bracket it.
			depth := func(tt float32) float32 {
				return c*b.lz + sn*cos32(tt)*b.uz
			}
			switch {
			case tMax <= 0:
				if depth(0) > 1e-4 {
					t.Errorf("%s: called wholly hidden, but z(0)=%v",
						planets[i].Name, depth(0))
				}
			case tMax >= math.Pi:
				if depth(math.Pi) < -1e-4 {
					t.Errorf("%s: called wholly visible, but z(pi)=%v",
						planets[i].Name, depth(math.Pi))
				}
			default:
				if depth(tMax*0.98) < -1e-3 {
					t.Errorf("%s: just inside tMax is hidden (z=%v)",
						planets[i].Name, depth(tMax*0.98))
				}
				if depth(min(tMax*1.02, math.Pi)) > 1e-3 {
					t.Errorf("%s: just outside tMax is visible (z=%v)",
						planets[i].Name, depth(tMax*1.02))
				}
			}
		}
	}
}

// TestGranulesStayInsideDisc is what lets the sun's mottling skip a
// clip: every cell's far edge is inside the limb by construction.
func TestGranulesStayInsideDisc(t *testing.T) {
	t.Parallel()
	for i, g := range sunGranules {
		if reach := g.dist + g.size; reach > granuleInset+1e-6 {
			t.Errorf("granule %d reaches %v, past the %v inset",
				i, reach, granuleInset)
		}
		if g.dist < 0 || g.size <= 0 {
			t.Errorf("granule %d has bad placement: %+v", i, g)
		}
	}
}

// TestSunEdgeWrapsSmoothly checks the corona's noise ring is seamless.
// It is sampled at a drifting index, so a discontinuity at the join
// would show as a notch travelling around the rim forever.
func TestSunEdgeWrapsSmoothly(t *testing.T) {
	t.Parallel()
	n := float32(len(sunEdge))

	for i, v := range sunEdge {
		if v < 0 || v > 1 {
			t.Fatalf("sunEdge[%d] = %v, outside [0,1]", i, v)
		}
	}
	if got, want := edgeAt(n), edgeAt(0); !closeEnough(got, want, 1e-6) {
		t.Errorf("edgeAt wraps to %v, want %v", got, want)
	}
	// Negative and far-out indices must land on the same sample too.
	if got, want := edgeAt(-n+3), edgeAt(3); !closeEnough(got, want, 1e-6) {
		t.Errorf("edgeAt(-n+3) = %v, want %v", got, want)
	}

	// Smoothing is what keeps the rim a star rather than a gear: no
	// two neighbours may differ by anything like the full range.
	for i := range sunEdge {
		d := absF(sunEdge[(i+1)%len(sunEdge)] - sunEdge[i])
		if d > 0.35 {
			t.Errorf("sunEdge jumps %v between %d and its neighbour", d, i)
		}
	}
}

// TestSunDiscToneLimbBrightening pins the shape of the disc ramp: the
// rim is brighter than the body behind it, and the core is brightest.
// A plain center-out ramp reads as a coin, not a star.
func TestSunDiscToneLimbBrightening(t *testing.T) {
	t.Parallel()
	lum := func(c gui.Color) float32 {
		return 0.2126*float32(c.R) + 0.7152*float32(c.G) + 0.0722*float32(c.B)
	}
	rim := lum(sunDiscTone(0))
	body := lum(sunDiscTone(discRimStop + 0.05))
	core := lum(sunDiscTone(1))

	if rim <= body {
		t.Errorf("limb %v is not brighter than the body %v", rim, body)
	}
	if core <= rim {
		t.Errorf("core %v is not brighter than the limb %v", core, rim)
	}
}

// drawInto runs the full paint into a headless DrawContext and returns
// it. A nil TextMeasurer is what tests get, and every DrawContext text
// call is nil-safe, so the geometry is exercised end to end without a
// backend.
func drawInto(a *App) *gui.DrawContext {
	dc := gui.NewDrawContext(a.CanvasW, a.CanvasH, nil)
	drawSystem(a, dc)
	return dc
}

// assertFinite fails on any non-finite vertex. NaN is the failure mode
// every degenerate case in the shading math produces: it does not
// panic, it silently emits triangles the rasterizer drops, so the only
// way to catch it is to look at the numbers.
func assertFinite(t *testing.T, dc *gui.DrawContext, what string) {
	t.Helper()
	verts := 0
	for _, b := range dc.Batches() {
		for _, v := range b.Triangles {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("%s: non-finite vertex %v", what, v)
			}
		}
		verts += len(b.Triangles)
	}
	if verts == 0 {
		t.Errorf("%s: nothing was drawn", what)
	}
}

// TestDrawSystemGeometryIsFinite paints every view state the app can be
// in and checks the emitted geometry. This is the only coverage the
// draw path has: the shading, the corona, the rings and the tooltip all
// run here.
func TestDrawSystemGeometryIsFinite(t *testing.T) {
	cases := []struct {
		name  string
		setup func(a *App)
	}{
		{"full system", func(*App) {}},
		{"hovered planet", func(a *App) { a.Hovered = 2 }},
		{"hovered sun", func(a *App) { a.Hovered = selSun }},
		{"selected planet", func(a *App) { selectBody(a, saturnIndex) }},
		{"selected sun", func(a *App) { selectBody(a, selSun) }},
		{"zoomed in", func(a *App) { a.applyUserZoom(userZoomMax) }},
		{"zoomed out", func(a *App) { a.applyUserZoom(userZoomMin) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			tc.setup(a)
			// Settle any transition the setup started, so the frame
			// under test is the one the case names.
			for range tweenTicks {
				tick(a)
			}
			assertFinite(t, drawInto(a), tc.name)
		})
	}
}

// TestDrawTooltipGeometryIsFinite covers the cursor-following bubble,
// which only paints while the pointer is over a body.
func TestDrawTooltipGeometryIsFinite(t *testing.T) {
	a := newTestApp()
	a.MouseIn = true
	// Park the cursor on Earth, then in each corner, so the bubble's
	// flip-and-clamp branches all run.
	corners := [][2]float32{
		{a.ScreenX[2], a.ScreenY[2]},
		{0, 0},
		{a.CanvasW, 0},
		{0, a.CanvasH},
		{a.CanvasW, a.CanvasH},
	}
	for _, c := range corners {
		a.MouseX, a.MouseY = c[0], c[1]
		a.Hovered = 2 // force the bubble regardless of the hit-test
		assertFinite(t, drawInto(a), "tooltip")
	}
}

// TestDrawBodyDegenerateLight pins the guard in drawBody. lightVec
// answers a planet sitting on the sun with a light straight down the
// camera axis, which has no screen-plane direction and therefore no
// light-aligned frame: building one divides by zero and every shading
// vertex comes out NaN.
func TestDrawBodyDegenerateLight(t *testing.T) {
	saved := planets[0]
	defer func() { planets[0] = saved }()
	planets[0].OrbitA = 0
	planets[0].Ecc = 0

	a := newTestApp()
	tick(a)
	assertFinite(t, drawInto(a), "planet on the sun")
}

// bodyBatches draws one shaded sphere in isolation and returns the
// context it landed in.
func bodyBatches(t *testing.T, r float32, lx, ly, lz float32) *gui.DrawContext {
	t.Helper()
	dc := gui.NewDrawContext(200, 200, nil)
	var m bodyMesh
	drawBody(dc, &m, 100, 100, r, gui.RGB(180, 140, 90), lx, ly, lz, nil)
	return dc
}

// TestBodyIsOneVertexColoredBatch pins the payoff of issue #400: the
// whole intensity ramp is one batch of interpolated vertices, where the
// flat-band version it replaced cost one batch per band.
func TestBodyIsOneVertexColoredBatch(t *testing.T) {
	dc := bodyBatches(t, 60, 0.6, 0.2, -0.77)
	meshes := 0
	for _, b := range dc.Batches() {
		if len(b.VertexColors) == 0 {
			continue // the rim feather, which is flat by design
		}
		meshes++
		if len(b.VertexColors)*2 != len(b.Triangles) {
			t.Fatalf("VertexColors=%d, Triangles=%d: lengths must agree",
				len(b.VertexColors), len(b.Triangles))
		}
		if len(b.Triangles) == 0 {
			t.Fatal("the shading mesh is empty")
		}
	}
	if meshes != 1 {
		t.Errorf("body emitted %d vertex-colored batches, want 1", meshes)
	}
}

// TestBodyMeshWindingIsConsistent guards the hazard that makes a single
// batch possible in the first place. gui/backend/soft rasterizes a
// vertex-colored batch as one path and accumulates signed coverage, so
// a triangle wound against its neighbours subtracts from them and cuts
// a seam through the body. Every light direction is worth checking:
// the limb clamp in lightBasis.point is what can reorder a quad, and
// which quads it reaches depends on where the light is.
func TestBodyMeshWindingIsConsistent(t *testing.T) {
	lights := []struct {
		name       string
		lx, ly, lz float32
	}{
		{"full face", 0.02, 0, 0.999},
		{"quarter", 0.7, 0, 0.71},
		{"terminator", 1, 0, 0},
		{"crescent", 0.6, 0.2, -0.77},
		{"near eclipse", 0.05, 0.02, -0.998},
	}
	for _, l := range lights {
		dc := bodyBatches(t, 60, l.lx, l.ly, l.lz)
		for _, b := range dc.Batches() {
			if len(b.VertexColors) == 0 {
				continue
			}
			for i := 0; i+5 < len(b.Triangles); i += 6 {
				v := b.Triangles[i : i+6]
				area := (v[2]-v[0])*(v[5]-v[1]) - (v[4]-v[0])*(v[3]-v[1])
				// The bound is not zero. appendTri sorts on its own
				// float32 evaluation of the same cross product, and a
				// sliver whose true area is a millionth of a pixel can
				// come out either side of zero here — Go may fuse the
				// multiply-subtract in one function and not the other.
				// Coverage cancellation needs area to cancel with, so
				// the thing worth failing on is a triangle that is
				// visibly reversed, not one at the noise floor.
				if area < -1e-4 {
					t.Fatalf("%s: triangle %d has signed area %v; every "+
						"triangle in the mesh must wind the same way",
						l.name, i/6, area)
				}
			}
		}
	}
}

// TestBodyMeshStaysInsideDisc checks the mesh boundary against the
// silhouette. Every vertex is a surface point of the sphere or a hidden
// point pushed onto the limb, so none of them may escape the disc.
func TestBodyMeshStaysInsideDisc(t *testing.T) {
	const cx, cy, r = 100, 100, 60
	dc := bodyBatches(t, 60, 0.6, 0.2, -0.77)
	for _, b := range dc.Batches() {
		if len(b.VertexColors) == 0 {
			continue
		}
		for i := 0; i+1 < len(b.Triangles); i += 2 {
			dx := float64(b.Triangles[i] - cx)
			dy := float64(b.Triangles[i+1] - cy)
			if d := math.Sqrt(dx*dx + dy*dy); d > r+0.01 {
				t.Fatalf("vertex %d is %v from the center, outside r=%v",
					i/2, d, float32(r))
			}
		}
	}
}

// TestBodyMeshDoesNotAllocatePerFrame pins the scratch reuse. drawBody
// runs nine times a tick; a mesh rebuilt from a fresh slice each time
// would allocate for the whole session.
func TestBodyMeshDoesNotAllocatePerFrame(t *testing.T) {
	var m bodyMesh
	dc := gui.NewDrawContext(200, 200, nil)
	drawBody(dc, &m, 100, 100, 60, gui.RGB(180, 140, 90), 0.6, 0.2, -0.77, nil)

	got := testing.AllocsPerRun(20, func() {
		// A fresh context each run would allocate for its own batches,
		// which is not what this measures; the mesh scratch is.
		m.tris = m.tris[:0]
		m.cols = m.cols[:0]
		// The color rings are scratch on the same footing as the
		// position rings: reused with [:0], never reallocated.
		m.cur, m.curCol = appendRing(m.cur[:0], m.curCol[:0],
			newLightBasis(0.6, 0.2, -0.77),
			60, 100, 100, 0.3, 0.95, 32, gui.RGB(1, 2, 3), ringTex{})
		m.prev, m.prevCol = appendRing(m.prev[:0], m.prevCol[:0],
			newLightBasis(0.6, 0.2, -0.77),
			60, 100, 100, 0.2, 0.98, 32, gui.RGB(4, 5, 6), ringTex{})
		m.appendStrip(32)
	})
	if got != 0 {
		t.Errorf("mesh rebuild allocated %v times per run, want 0", got)
	}
}

// pointInMesh reports whether (px,py) lies in some triangle of the
// vertex-colored batches. The mesh is wound consistently, so a point is
// inside a triangle when all three edge cross products share a sign;
// the epsilon lets a point sitting exactly on a shared edge count.
func pointInMesh(dc *gui.DrawContext, px, py float32) bool {
	const eps = 1e-4
	for _, b := range dc.Batches() {
		if len(b.VertexColors) == 0 {
			continue
		}
		for i := 0; i+5 < len(b.Triangles); i += 6 {
			v := b.Triangles[i : i+6]
			a := (v[2]-v[0])*(py-v[1]) - (px-v[0])*(v[3]-v[1])
			c := (v[4]-v[2])*(py-v[3]) - (px-v[2])*(v[5]-v[3])
			d := (v[0]-v[4])*(py-v[5]) - (px-v[4])*(v[1]-v[5])
			if a >= -eps && c >= -eps && d >= -eps {
				return true
			}
			if a <= eps && c <= eps && d <= eps {
				return true
			}
		}
	}
	return false
}

// TestBodyMeshCoversTheLimb guards the silhouette against the facets
// ringPlan exists to remove.
//
// The mesh boundary is the limb, drawn as one chord per ring that ends
// there, so a ring spacing that samples the limb unevenly leaves wide
// chords and the disc's edge goes visibly polygonal — background
// showing through where the sphere should be. Sampling just inside the
// limb all the way around is the direct test: with even steps in
// intensity these points fall outside the chord near the light's own
// screen direction, which is on the disc edge whenever a planet sits
// beside the sun.
//
// The small-body case is the other failure mode of the same sampling:
// a small body's ring budget can leave zero rings for the visible
// polar cap, whose lens sits right on the light side of the limb — a
// bare patch of background at the point where the planet meets the
// sun. The plan floors the cap at one ring, which turns the whole lens
// into a fan from the pole.
func TestBodyMeshCoversTheLimb(t *testing.T) {
	const cx, cy = 100, 100
	lights := []struct {
		name       string
		lx, ly, lz float32
		r          float32
	}{
		{"terminator", 1, 0, 0, 60},
		{"beside the sun", 0.999, 0.045, 0, 60},
		{"quarter", 0.7, 0, 0.71, 60},
		{"crescent", 0.6, 0.2, -0.77, 60},
		{"full face", 0.02, 0, 0.999, 60},
	}
	for _, l := range lights {
		dc := bodyBatches(t, l.r, l.lx, l.ly, l.lz)
		for k := range 256 {
			a := 2 * math.Pi * float64(k) / 256
			px := cx + 0.99*l.r*float32(math.Cos(a))
			py := cy + 0.99*l.r*float32(math.Sin(a))
			if !pointInMesh(dc, px, py) {
				t.Fatalf("%s: the mesh leaves a gap at %.0f° around the "+
					"limb; the silhouette is faceted there",
					l.name, a*180/math.Pi)
			}
		}
	}
}

// TestBodyMeshSmallBodyCoversTheLightSide pins the cap's floor of one
// ring. A small body's ring budget can leave zero rings for the
// visible polar cap, and without the floor the cap lens on the light
// side of the limb is painted by nothing — a bare patch of background
// where the planet meets the sun.
//
// The samples stay a hair off the light axis: the fan from the pole
// ring closes with a sub-pixel float32 wedge along it, and the point
// here is the unpainted lens, not that sliver.
func TestBodyMeshSmallBodyCoversTheLightSide(t *testing.T) {
	const cx, cy = 100, 100
	const r float32 = 20
	// The light 14.5° out of the screen plane leaves the cap lens
	// spanning about 17.5..20 px from the center along the light axis.
	dc := bodyBatches(t, r, 0.97, 0, 0.25)
	for _, a := range []float64{math.Pi / 16, -math.Pi / 16} {
		px := cx + 0.97*r*float32(math.Cos(a))
		py := cy + 0.97*r*float32(math.Sin(a))
		if !pointInMesh(dc, px, py) {
			t.Fatalf("the mesh leaves a gap at %.0f° on the light side "+
				"of the limb; the visible polar cap is unpainted",
				a*180/math.Pi)
		}
	}
}
