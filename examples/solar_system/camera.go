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
	return ellipsePos(p.OrbitA, p.Ecc, p.Phase, p.PeriodS, t)
}

// ellipsePos is orbitPos with the orbit terms supplied directly, so
// the asteroid belt can share it without a Planet table entry.
func ellipsePos(a, ecc, phase, periodS, t float32) (x, y float32) {
	if periodS <= 0 {
		periodS = 1
	}
	if ecc < 0 {
		ecc = 0
	} else if ecc >= 1 {
		ecc = 0.99
	}
	theta := phase + 2*math.Pi*t/periodS
	b := a * sqrt32(1-ecc*ecc)
	return a*cos32(theta) - a*ecc, b * sin32(theta)
}

// orbitPeriod is the PeriodS a body at world radius a would have
// under the table's compression: realAU = (a/195)^(1/0.38), then
// realDays^0.45 * 0.8. Checks out against the table — Mercury 6.0,
// Earth 11.4, Jupiter 34.6.
//
// Init-time only; never on the frame path.
func orbitPeriod(a float32) float32 {
	if a <= 0 {
		return 1
	}
	realAU := math.Pow(float64(a)/195, 1/0.38)
	realDays := 365.25 * math.Pow(realAU, 1.5)
	return float32(0.8 * math.Pow(realDays, 0.45))
}

// zoom is the effective scale: the camera's own zoom times the user's.
func (a *App) zoom() float32 { return a.CamZoom * a.UserZoom }

// worldToScreen maps world coordinates to canvas-content pixels.
func (a *App) worldToScreen(wx, wy float32) (sx, sy float32) {
	z := a.zoom()
	return a.CanvasW/2 + (wx-a.CamX)*z,
		a.CanvasH/2 + (wy-a.CamY)*diskTilt*z
}

// fullSystemTarget frames every orbit plus the calendar ring, with
// a margin so the ring is not flush against the edge. The ring sits
// outside Neptune's aphelion, so it now sets the extent rather than
// Neptune. Margin trimmed to 1.04 so the system does not shrink. The
// label caps (dialTextR + cap height) are also included so
// December/January and June/July do not sit on the window edge.
func (a *App) fullSystemTarget() (tx, ty, tz float32) {
	outer := planets[len(planets)-1]
	extent := outer.OrbitA*(1+outer.Ecc) + outer.Radius
	// Calendar ring lies in the orbital plane outside Neptune; size to
	// it instead when it is larger.
	if dialOuter > extent {
		extent = dialOuter
	}
	// Include the month labels' cap height so east/west labels stay
	// inside the window. dialTextR is the baseline, cap is emHeight*emW.
	textTop := dialTextR + emHeight*dialEmWorld
	if textTop > extent {
		extent = textTop
	}
	if extent <= 0 || a.CanvasW <= 0 || a.CanvasH <= 0 {
		return 0, 0, 1
	}
	zx := a.CanvasW / (2 * extent * 1.04)
	zy := a.CanvasH / (2 * extent * diskTilt * 1.04)
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

// zoomRho is the curvature of the zoom-and-pan path: how much the view
// is allowed to widen in order to cover ground. sqrt(2) is Van Wijk and
// Nuij's own measured optimum and is what every implementation of this
// uses; larger zooms out more and arrives sooner, smaller flies lower
// and further.
const zoomRho = math.Sqrt2

// zoomPath is a camera move that zooms and pans as one motion.
//
// Interpolating the center and the zoom separately is what made a
// selection read as two steps. Zoom is a *ratio*, so a straight line
// through it spends its first half covering most of the magnification
// — from the full system to a planet that is five of the seventeen
// times over in the first quarter of the tween — while the center is
// only a quarter of the way across. The remaining pan then happens at
// high magnification, where a world unit is many pixels, so the
// destination first slides off toward the edge of the screen and only
// afterwards swings back to the middle. Zooming in on the sun, then
// sliding, is exactly what that looks like.
//
// The fix is to ask for the right thing: a path along which the view
// appears to move at a constant rate. That is Van Wijk and Nuij,
// "Smooth and efficient zooming and panning" (InfoVis 2003). Their
// result is that the optimal path is a straight line in the plane
// traversed on a hyperbolic schedule, with the view width following
// from it — the view widens while crossing distance and narrows on
// arrival, and the two are one motion rather than two.
//
// The state it interpolates is a center and a *view width* in world
// units, not a zoom factor: the whole derivation is about how much
// world the screen shows, and a width is what makes distance and
// magnification the same currency.
type zoomPath struct {
	x0, y0, w0 float32
	dx, dy     float32 // full displacement, in world units

	// r0 and s scale the hyperbolic schedule; u1 is the displacement
	// fraction the schedule reaches at t = 1, which normalizes the
	// path so it lands exactly on the target.
	r0, s, u1 float32

	// pure is the degenerate straight zoom, taken when the two centers
	// coincide and there is no direction to travel in.
	pure bool
}

// newZoomPath solves the path from one (center, view width) to another.
//
// Distances are measured with the vertical axis pre-squashed by
// diskTilt, because that is what the projection does to it: the path
// has to be even in *apparent* motion, and a world unit north covers
// less screen than a world unit east.
func newZoomPath(x0, y0, w0, x1, y1, w1 float32) zoomPath {
	p := zoomPath{x0: x0, y0: y0, w0: w0, dx: x1 - x0, dy: y1 - y0}
	if w0 <= 0 || w1 <= 0 {
		// No usable width to interpolate. Hold the one we have rather
		// than invent a rate: s = 0 makes at() a straight pan.
		p.pure = true
		return p
	}
	// The apparent displacement, which is what the schedule is solved
	// against; dx and dy stay unsquashed for reconstructing the center.
	ex, ey := p.dx, p.dy*diskTilt
	d2 := float64(ex*ex + ey*ey)
	b0 := float64(w0)
	b1 := float64(w1)
	// A move too short to have a direction is a straight zoom, and the
	// width is exponential in t so that equal steps are equal ratios.
	if d2 < 1e-9 {
		p.pure = true
		p.s = float32(math.Log(b1 / b0))
		return p
	}
	d1 := math.Sqrt(d2)
	const rho2, rho4 = 2.0, 4.0
	c0 := (b1*b1 - b0*b0 + rho4*d2) / (2 * b0 * rho2 * d1)
	c1 := (b1*b1 - b0*b0 - rho4*d2) / (2 * b1 * rho2 * d1)
	// The two path ends, in the standard asinh(-c) form. Written out
	// as log(sqrt(c*c+1) - c) it loses the whole result to
	// cancellation once c passes about 1e7 and returns -Inf by 1e8,
	// which a short pan under a large change of width does reach.
	r0 := math.Asinh(-c0)
	r1 := math.Asinh(-c1)
	p.r0 = float32(r0)
	p.s = float32((r1 - r0) / zoomRho)
	// Where the schedule lands at t = 1. Solving for it rather than
	// trusting it to be 1 keeps rounding out of the arrival: the tween
	// is also read at t = 1 every frame once it has settled, and a
	// camera that stopped a hair short of a planet would never catch up
	// to it.
	p.u1 = p.displacement(1)
	if p.u1 == 0 {
		p.pure = true
	}
	return p
}

// displacement is the un-normalized fraction of the way along the line
// at t, on the hyperbolic schedule.
func (p zoomPath) displacement(t float32) float32 {
	r0 := float64(p.r0)
	x := zoomRho*float64(t*p.s) + r0
	return float32(math.Cosh(r0)*math.Tanh(x) - math.Sinh(r0))
}

// at returns the center and view width at t in [0,1].
func (p zoomPath) at(t float32) (x, y, w float32) {
	if p.pure {
		// Exponential in the width, linear in the center — which for a
		// pure zoom is the same constant-rate path the general case
		// gives, with the hyperbolic part degenerate.
		return p.x0 + t*p.dx, p.y0 + t*p.dy,
			p.w0 * float32(math.Exp(float64(t*p.s)))
	}
	u := p.displacement(t) / p.u1
	r0 := float64(p.r0)
	x = p.x0 + u*p.dx
	y = p.y0 + u*p.dy
	w = p.w0 * float32(math.Cosh(r0)/
		math.Cosh(zoomRho*float64(t*p.s)+r0))
	return x, y, w
}

// viewWidth is how much world the canvas spans at a given camera zoom.
// It is the currency zoomPath works in. UserZoom is deliberately left
// out: it is the viewer's own multiplier, and folding it in would let a
// scroll mid-flight bend the path.
func (a *App) viewWidth(camZoom float32) float32 {
	if camZoom <= 0 || a.CanvasW <= 0 {
		return 0
	}
	return a.CanvasW / camZoom
}

// advanceCamera steps the tween by dt and blends toward the live
// target. At TweenT == 1 the camera equals the target exactly, so it
// simply follows from then on.
//
// The path is re-solved every tick against the *live* target, for the
// same reason target() is re-read: a selected planet keeps orbiting,
// and a path captured at click time would aim at where it used to be.
func (a *App) advanceCamera(dt float32) {
	tx, ty, tz := a.target()
	if a.TweenT < 1 {
		a.TweenT += dt / camTweenSecs
		if a.TweenT > 1 {
			a.TweenT = 1
		}
	}
	if a.TweenT >= 1 {
		// Settled: the camera simply follows its target. Solving the
		// path here would land on exactly this and pay four logs and
		// three hyperbolics for it, every tick, for the whole time a
		// planet stays selected.
		a.CamX, a.CamY, a.CamZoom = tx, ty, tz
		return
	}
	k := gui.EaseInOutQuad(a.TweenT)
	w0, w1 := a.viewWidth(a.FromZoom), a.viewWidth(tz)
	if w0 <= 0 || w1 <= 0 {
		// No canvas yet, or a degenerate zoom. Nothing to solve; fall
		// back to the plain blend so the camera still tracks.
		a.CamX = lerp(a.FromX, tx, k)
		a.CamY = lerp(a.FromY, ty, k)
		a.CamZoom = lerp(a.FromZoom, tz, k)
		return
	}
	x, y, w := newZoomPath(a.FromX, a.FromY, w0, tx, ty, w1).at(k)
	a.CamX, a.CamY = x, y
	a.CamZoom = a.CanvasW / w
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
// It bumps Version so the DrawCanvas re-tessellates on the next frame
// without waiting for the 16 ms tick; a zoom otherwise feels one frame
// behind, which reads as sluggish on a trackpad.
func (a *App) applyUserZoom(factor float32) {
	a.UserZoom = clamp32(a.UserZoom*factor, userZoomMin, userZoomMax)
	a.Version++
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
