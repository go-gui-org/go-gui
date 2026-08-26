package main

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	// diskTilt squashes the vertical axis so the orbital plane reads as
	// a disc seen at an angle rather than a set of flat circles.
	diskTilt = 0.48

	// camTweenSecs is how long a selection change takes to settle.
	camTweenSecs = 0.6

	// minHitR keeps the smallest bodies clickable when zoomed out: a
	// 4-unit Mercury is under a pixel wide in the full-system view.
	minHitR = 14

	// userZoomMin/Max bound the scroll/pinch/button multiplier. It is a
	// separate factor from the camera's own zoom so a manual zoom
	// composes with a selection instead of fighting it.
	userZoomMin = 0.3
	userZoomMax = 8.0

	// selectedScreenFrac is the fraction of the canvas's short side a
	// selected planet should span, and selectedZoomMax caps the result
	// so Mercury does not fill the screen with flat color.
	selectedScreenFrac = 0.10
	selectedZoomMax    = 30
)

// orbitPos returns a planet's world position at time t. Motion is
// uniform in the parameter angle rather than Kepler-correct: the visual
// payoff of true areal velocity is small and the eccentricities here
// are all under 0.21.
//
// The ellipse is centered at (-a*e, 0) so the sun sits on the focus at
// the world origin, which is what makes eccentricity visible at all.
func orbitPos(p *Planet, t float32) (x, y float32) {
	theta := p.Phase + 2*math.Pi*t/p.PeriodS
	a := p.OrbitA
	b := a * sqrt32(1-p.Ecc*p.Ecc)
	return a*cos32(theta) - a*p.Ecc, b * sin32(theta)
}

// zoom is the effective scale: the camera's own zoom times the user's.
func (a *App) zoom() float32 { return a.CamZoom * a.UserZoom }

// worldToScreen maps world coordinates to canvas-content pixels.
func (a *App) worldToScreen(wx, wy float32) (sx, sy float32) {
	z := a.zoom()
	return a.CanvasW/2 + (wx-a.CamX)*z,
		a.CanvasH/2 + (wy-a.CamY)*diskTilt*z
}

// fullSystemTarget frames every orbit, with a margin so Neptune's
// aphelion is not flush against the edge.
func (a *App) fullSystemTarget() (tx, ty, tz float32) {
	outer := planets[len(planets)-1]
	extent := outer.OrbitA*(1+outer.Ecc) + outer.Radius
	if extent <= 0 || a.CanvasW <= 0 || a.CanvasH <= 0 {
		return 0, 0, 1
	}
	zx := a.CanvasW / (2 * extent * 1.12)
	zy := a.CanvasH / (2 * extent * diskTilt * 1.12)
	return 0, 0, min(zx, zy)
}

// planetTarget centers a planet at its live orbital position.
func (a *App) planetTarget(i int) (tx, ty, tz float32) {
	wx, wy := orbitPos(&planets[i], a.Time)
	return a.bodyTarget(&planets[i], wx, wy)
}

// sunTarget centers the sun, which is fixed at the world origin.
func (a *App) sunTarget() (tx, ty, tz float32) {
	return a.bodyTarget(&sun, 0, 0)
}

// bodyTarget frames one body at a given world position, lifted above
// center by half the info panel so the panel does not cover it.
func (a *App) bodyTarget(p *Planet, wx, wy float32) (tx, ty, tz float32) {
	short := min(a.CanvasW, a.CanvasH)
	z := selectedScreenFrac * short / p.Radius
	z = min(z, selectedZoomMax)
	if z <= 0 {
		z = 1
	}

	// The shift is expressed in world units, so it must undo both the
	// zoom and the vertical squash.
	lift := panelLiftPx / (diskTilt * z * a.UserZoom)
	return wx, wy + lift, z
}

// target returns the camera's live goal. It is re-read every tick, not
// captured at click time, so a selected planet stays centered as it
// keeps orbiting.
func (a *App) target() (tx, ty, tz float32) {
	switch {
	case a.Selected == selSun:
		return a.sunTarget()
	case a.Selected >= 0:
		return a.planetTarget(a.Selected)
	default:
		return a.fullSystemTarget()
	}
}

// beginTransition snaps the tween's origin to wherever the camera
// currently is and restarts it. Interrupting a transition mid-flight is
// therefore continuous — there is no jump back to the previous anchor.
func (a *App) beginTransition() {
	a.FromX, a.FromY, a.FromZoom = a.CamX, a.CamY, a.CamZoom
	a.TweenT = 0
}

// advanceCamera steps the tween by dt and blends toward the live
// target. At TweenT == 1 the camera equals the target exactly, so it
// simply follows from then on.
func (a *App) advanceCamera(dt float32) {
	tx, ty, tz := a.target()
	if a.TweenT < 1 {
		a.TweenT += dt / camTweenSecs
		if a.TweenT > 1 {
			a.TweenT = 1
		}
	}
	k := gui.EaseInOutQuad(a.TweenT)
	a.CamX = lerp(a.FromX, tx, k)
	a.CamY = lerp(a.FromY, ty, k)
	a.CamZoom = lerp(a.FromZoom, tz, k)
}

// cosElev is the cosine of the camera's elevation above the orbital
// plane. diskTilt is the sine of that same angle — a plane tilted by e
// projects its depth axis down to sin(e) — so this is the tilt read the
// other way, and it is the depth component the shading needs.
var cosElev = sqrt32(1 - diskTilt*diskTilt)

// lightVec returns the unit direction from a planet toward the sun in
// *screen* coordinates: x right, y down, z out of the screen toward
// the viewer. It is the one source for everything the shading needs —
// which way the highlight points, and how much of the disc is lit.
//
// The frame is the camera's, not the world's. The camera sits at
// elevation e above the orbital plane, and diskTilt is sin(e): that is
// what the vertical squash in worldToScreen *is*. So the camera's basis
// in world coordinates is
//
//	right       R = (1, 0, 0)
//	screen-down D = (0, sin e, -cos e)
//	to viewer   V = (0, cos e,  sin e)
//
// The sun is at the world origin, so the world-space light direction is
// L = normalize(-px, -py, 0), and projecting it onto that basis gives
// the three components below. They are still unit length, because the
// two components that pick up Lwy square back to Lwy² (sin²e + cos²e).
//
// The z component is exactly cos of the phase angle, which is why phase
// falls out of this for free rather than needing its own derivation.
func (a *App) lightVec(i int) (lx, ly, lz float32) {
	px, py := orbitPos(&planets[i], a.Time)
	return lightVecAt(px, py)
}

// lightVecAt is lightVec with the planet's world position supplied.
//
// The draw path takes this door, reading the position recompute
// already cached, so the eight planets do not repeat the orbit
// trigonometry a second time in the same tick. lightVec keeps its own
// orbitPos call rather than reading the cache: it is also reached from
// litFraction and from callers that move Time on their own, and a
// cache read there would answer with the previous position instead of
// failing.
func lightVecAt(px, py float32) (lx, ly, lz float32) {
	d := sqrt32(px*px + py*py)
	if d == 0 {
		// Degenerate: a planet at the sun's own position. Light it
		// straight down the camera axis rather than dividing by zero.
		return 0, 0, 1
	}
	return -px / d, (-py / d) * diskTilt, (-py / d) * cosElev
}

// litFraction returns how much of a planet's visible disc is sunlit: 0
// is back-lit with the night side toward the viewer, 1 is fully lit.
//
// This is the phase, and it answers a different question from
// lightVec's screen-plane part. That part says which way on screen the
// lit side lies; phase says how much of it faces the viewer at all. A
// planet on the near side of its orbit — drawn low on the screen — has
// the sun beyond it, so the hemisphere facing the viewer is its night
// side and it reads as a crescent. Leave this term out and every planet
// is painted fully lit with only the highlight direction turning, which
// is what most orreries do and what reads as wrong once you go looking
// for it.
//
// The lit fraction of a sphere's disc is (1 + cos(phase)) / 2, and the
// z component of lightVec is that cosine.
//
// The result never reaches 0 or 1, and that is correct rather than a
// clamp: the camera sits about 29° above the plane instead of in it, so
// it never sees a pure new or full phase. The range is about
// [0.06, 0.94]. drawBody does not consume this — it works from the full
// vector — but Saturn's rings and the tests do.
func (a *App) litFraction(i int) float32 {
	_, _, lz := a.lightVec(i)
	return (1 + lz) / 2
}

// recompute refreshes the cached screen position and radius of every
// planet. Hit-testing and painting both read these, so computing them
// once per tick is what guarantees the two agree.
func (a *App) recompute() {
	z := a.zoom()
	a.SunX, a.SunY = a.worldToScreen(0, 0)
	a.SunR = sunRadius * z
	for i := range planets {
		wx, wy := orbitPos(&planets[i], a.Time)
		// Kept as well as the screen position: the shading needs the
		// world vector to the sun, which the screen squash has already
		// destroyed. See lightVecAt.
		a.WorldX[i], a.WorldY[i] = wx, wy
		a.ScreenX[i], a.ScreenY[i] = a.worldToScreen(wx, wy)
		a.ScreenR[i] = planets[i].Radius * z
	}
}

// hitTest returns the body under a canvas-content point: a planet
// index, selSun for the sun, or -1 for empty space. Nearest wins, so an
// overlap resolves to the body whose center is closest rather than
// whichever happens to be first in the table.
//
// Only the sun's disc is a target, not its halo — the glow reaches
// 1.7x the disc (up to ~2x with the pulse and the hover stretch) and
// would swallow the inner planets whenever they pass in front of it.
func (a *App) hitTest(x, y float32) int {
	best, bestD := -1, float32(math.MaxFloat32)
	test := func(idx int, sx, sy, r float32) {
		if r < minHitR {
			r = minHitR
		}
		dx, dy := x-sx, y-sy
		d := dx*dx + dy*dy
		if d <= r*r && d < bestD {
			best, bestD = idx, d
		}
	}
	test(selSun, a.SunX, a.SunY, a.SunR)
	for i := range planets {
		test(i, a.ScreenX[i], a.ScreenY[i], a.ScreenR[i])
	}
	return best
}

// applyUserZoom multiplies the manual zoom factor and clamps it.
func (a *App) applyUserZoom(factor float32) {
	a.UserZoom = clamp32(a.UserZoom*factor, userZoomMin, userZoomMax)
}

// --- small float helpers (math is float64-only) ---

func sqrt32(v float32) float32  { return float32(math.Sqrt(float64(v))) }
func sin32(v float32) float32   { return float32(math.Sin(float64(v))) }
func cos32(v float32) float32   { return float32(math.Cos(float64(v))) }
func acos32(v float32) float32  { return float32(math.Acos(float64(v))) }
func floor32(v float32) float32 { return float32(math.Floor(float64(v))) }

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
