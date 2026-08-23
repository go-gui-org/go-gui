package main

import (
	"math"
	"math/rand/v2"

	"github.com/go-gui-org/go-gui/gui"
)

// Palette. Kept as package vars rather than theme reads because OnDraw
// runs during render, after the frame's theme cache has served its
// purpose; the two text styles that do need the theme are resolved at
// generation time and stashed on App.
var (
	colorSpace     = gui.RGB(6, 8, 18)
	colorStar      = gui.RGB(235, 240, 255)
	colorOrbit     = gui.RGBA(150, 170, 210, 46)
	colorOrbitLive = gui.RGBA(190, 210, 255, 110)
	// The sun is white-hot at the core and only turns gold near the
	// limb. Running the whole disc orange makes it read as an amber
	// lamp; the color belongs in the halo and the rim, not the body.
	colorSunCore = gui.RGB(255, 255, 252)
	colorSunBody = gui.RGB(254, 243, 206)
	colorSunLimb = gui.RGB(255, 252, 234)
	colorSunGlow = gui.RGB(255, 186, 78)

	// Granulation: the mottling across the disc. Both are laid on at
	// low alpha and stacked, so these are much lighter in effect than
	// they look.
	colorSunGranuleLit  = gui.RGBA(255, 255, 250, 34)
	colorSunGranuleDark = gui.RGBA(240, 200, 126, 34)

	// The corona ring right at the limb, brighter than the disc it
	// sits on.
	colorSunCorona = gui.RGB(255, 240, 196)
	colorRing      = gui.RGBA(226, 210, 166, 120)
	colorTipBG     = gui.RGBA(18, 22, 36, 235)
	colorTipBorder = gui.RGBA(140, 160, 210, 120)
)

const (
	// sunRadius is in world units, like the planet radii.
	sunRadius = 52

	// The halo and the disc both take their step count from their pixel
	// size, for the same reason the planet ramp does: a fixed count
	// bands visibly once the sun is close to the camera. Focusing
	// Jupiter puts a 500px halo on screen, where 30 rings is a 17px
	// band apiece.
	glowRingsPerPx = 0.22
	glowRingsMin   = 24
	glowRingsMax   = 140

	glowExtent  = 2.2 // outer glow radius, in multiples of sunRadius
	glowPulseHz = 0.28

	// The disc ramp's two hand-over points, as a fraction of the radius
	// inward from the limb. The face is *limb brightened*: a bright
	// rind at the edge, a slightly deeper body behind it, then the
	// white core. A star has a bright edge; a plain center-out ramp
	// reads as a coin lit from the front.
	discRimStop  = 0.14
	discCoreStop = 0.34

	// Granulation. The cells are a fixed set of soft blobs, each drawn
	// as granuleTiers nested circles so it has a falloff instead of a
	// hard edge. Tiers are drawn outermost-first across *all* cells at
	// once, and lit and dark cells in separate passes, so one tier of
	// one polarity is a single flat color and therefore a single
	// batch: 2*granuleTiers batches for the whole texture.
	granuleCount = 70
	granuleTiers = 3

	// granuleMinRadius is the pixel radius below which the cells are
	// too small to see and are skipped. granuleInset is how far out a
	// cell's far edge may reach, as a fraction of the sun's radius.
	granuleMinRadius = 14
	granuleInset     = 0.94

	// The corona rim: a ragged band hugging the limb, drawn as
	// coronaTiers quad strips of falling alpha. coronaSteps is how many
	// angular segments the ragged edge is traced with, and must match
	// the noise table's length.
	coronaTiers   = 6
	coronaSteps   = 96
	coronaReach   = 0.24 // how far past the limb the outermost tier goes
	coronaRagged  = 0.72 // how much of that reach the noise modulates
	coronaDriftHz = 0.05 // how fast the ragged edge crawls

	// sunShells ramps the disc itself from a near-white core to its
	// orange limb, which is what stops it reading as a flat coin.
	sunShellsPerPx = 0.9
	sunShellsMin   = 14
	sunShellsMax   = 96

	// shadeLevels is how many flat-colored bands the sphere shading is
	// quantized into, and shadeArc how many segments each band's curved
	// boundary is traced with. Both scale with the body's pixel radius,
	// bounded by their Min/Max: a fixed count bands badly once a planet
	// is zoomed to fill the view, and is wasted on a 4px Mercury.
	//
	// The level count is also the batch count for the body, since a band
	// is one flat color and its quads are emitted consecutively.
	shadeLevelsPerPx = 1.4
	shadeLevelsMin   = 20
	shadeLevelsMax   = 48
	shadeArcPerPx    = 1.2
	shadeArcMin      = 20
	shadeArcMax      = 72

	// nightScale is the unlit hemisphere's brightness, as a fraction of
	// the body's own color. Not zero on purpose: a planet you cannot see
	// is a planet you cannot click, and this is an orrery before it is a
	// simulation.
	nightScale = 0.16

	// baseStop is where the body's own color sits on the intensity ramp.
	// Well past the middle, so most of the lit side carries the planet's
	// hue and the bright end stays tight.
	baseStop = 0.72

	// seamOverlap is how far past its own share each shading quad is
	// grown, as a fraction of a step, so neighbours overlap instead of
	// merely touching. See the comment at its use.
	seamOverlap = 0.2

	// ringNightAlpha is how much of their opacity Saturn's rings keep on
	// the night side. They are lit by the same sun the body is.
	ringNightAlpha = 0.35

	// rimFeather is how many translucent rings sit just outside a body,
	// softening the polygon silhouette into something that reads as an
	// anti-aliased edge.
	rimFeather = 3

	// flatBodyRadius is the screen radius below which shading is
	// skipped: at two or three pixels the ramp is invisible and only
	// costs batches.
	flatBodyRadius = 4

	// hoverHalos is the same glow technique at planet scale.
	hoverHalos = 6

	// labelGap is the space between a body's edge and its name.
	labelGap = 5

	tipPadX = 8
	tipPadY = 5
	tipGap  = 16 // cursor-to-bubble offset
)

// drawSystem is the canvas OnDraw. It stashes the canvas size first —
// the camera and hit-test both need it and nothing else supplies it —
// then paints strictly back to front.
func drawSystem(a *App, dc *gui.DrawContext) {
	if a.CanvasW != dc.Width || a.CanvasH != dc.Height {
		a.CanvasW, a.CanvasH = dc.Width, dc.Height
		a.recompute()
	}

	dc.FilledRect(0, 0, dc.Width, dc.Height, colorSpace)
	drawStars(a, dc)
	drawOrbits(a, dc)
	drawSun(a, dc)
	drawPlanets(a, dc)
	drawTooltip(a, dc)
}

// drawStars paints the twinkling background.
//
// Brightness is quantized to starAlphaLevels and the field is drawn one
// level at a time. DrawContext merges only *consecutive* same-color
// triangles, so 220 stars at 220 distinct alphas would open 220 batches
// every frame; grouping opens 8. At star size the banding is invisible.
// The stars are squares rather than circles for the same reason: two
// triangles instead of ~64, and at 1-2px nobody can tell.
//
// Levels are bucketed once up front rather than re-derived inside the
// per-level pass: the twinkle is a sine per star, and evaluating it
// once per level instead would cost starAlphaLevels times as many.
// The bucket array is fixed-size and does not escape, so it stays on
// the stack and the whole pass allocates nothing.
func drawStars(a *App, dc *gui.DrawContext) {
	var bucket [starCount]uint8
	n := min(len(a.Stars), starCount)
	for i := range n {
		s := &a.Stars[i]
		bucket[i] = uint8(starLevel(s.Base +
			s.Amp*sin32(a.Time*s.Speed+s.Phase)))
	}

	for level := range starAlphaLevels {
		// Level midpoint, so the darkest bucket is not fully invisible.
		alpha := (float32(level) + 0.5) / starAlphaLevels
		c := colorStar.WithOpacity(alpha)
		for i := range n {
			if int(bucket[i]) != level {
				continue
			}
			s := &a.Stars[i]
			dc.FilledRect(s.X*dc.Width, s.Y*dc.Height, s.Size, s.Size, c)
		}
	}
}

// starLevel buckets a brightness in [0,1] into [0, starAlphaLevels).
func starLevel(b float32) int {
	lvl := int(floor32(clamp32(b, 0, 0.999) * starAlphaLevels))
	// clamp32 already bounds the input, so this only guards against a
	// future change to the levels constant.
	return min(max(lvl, 0), starAlphaLevels-1)
}

// drawOrbits traces each orbit as a full ellipse. Arc is the ellipse
// primitive — FilledCircle is just FilledArc with rx == ry — so the
// tilt is applied by squashing ry alone.
//
// The ellipse's center is offset by a*e because the sun sits on the
// focus, not the center. Without that the eccentricity would not show.
func drawOrbits(a *App, dc *gui.DrawContext) {
	z := a.zoom()
	for i := range planets {
		p := &planets[i]
		cx, cy := a.worldToScreen(-p.OrbitA*p.Ecc, 0)
		rx := p.OrbitA * z
		ry := p.OrbitA * sqrt32(1-p.Ecc*p.Ecc) * z * diskTilt
		if rx < 2 || ry < 0.5 {
			continue
		}
		c := colorOrbit
		if i == a.Selected || i == a.Hovered {
			c = colorOrbitLive
		}
		dc.Arc(cx, cy, rx, ry, 0, 2*math.Pi, c, 1)
	}
}

// granule is one convection cell on the sun's face: a soft blob at a
// fixed spot, either brighter or darker than the disc under it.
type granule struct {
	// Polar placement inside the disc, so the field stays inside the
	// limb at any radius without a clip.
	ang, dist float32
	size      float32 // blob radius, as a fraction of the sun's
	lit       bool
}

// sunGranules and sunEdge are both built once with a fixed seed. The
// sun has to look the same every frame: regenerating either per frame
// would make the texture boil and the rim strobe.
var (
	sunGranules = makeGranules()
	sunEdge     = makeSunEdge()
)

func makeGranules() []granule {
	rng := rand.New(rand.NewPCG(0x5C0FFEE, 0xB1A5ED))
	g := make([]granule, granuleCount)
	for i := range g {
		size := 0.08 + rng.Float32()*0.20
		// sqrt on the radial coordinate spreads the cells evenly over
		// the *area*; without it they crowd the middle. The low end
		// keeps them off the very center, where the core is meant to
		// read as a clean hot spot, and the clamp keeps a cell's far
		// edge inside the limb so the texture needs no clipping.
		dist := 0.16 + sqrt32(rng.Float32())*0.60
		g[i] = granule{
			ang:  rng.Float32() * 2 * math.Pi,
			dist: min(dist, granuleInset-size),
			size: size,
			lit:  rng.Float32() < 0.55,
		}
	}
	return g
}

// makeSunEdge builds the noise ring that makes the corona ragged.
//
// Raw uniform noise gives a spiky rim that reads as a gear rather than
// a star, so it is smoothed by neighbour averaging, twice, and the ring
// wraps — index 0 and the last index are neighbours, and a seam there
// would show as a notch that never moves.
func makeSunEdge() []float32 {
	rng := rand.New(rand.NewPCG(0x50A12, 0xF1A2E))
	e := make([]float32, coronaSteps)
	for i := range e {
		e[i] = rng.Float32()
	}
	for range 2 {
		prev := make([]float32, len(e))
		copy(prev, e)
		for i := range e {
			l := prev[(i-1+len(e))%len(e)]
			r := prev[(i+1)%len(e)]
			e[i] = (l + prev[i]*2 + r) / 4
		}
	}
	return e
}

// edgeAt samples the noise ring at a fractional index, wrapping and
// interpolating, so the corona can crawl slowly instead of stepping.
func edgeAt(idx float32) float32 {
	n := float32(len(sunEdge))
	idx -= floor32(idx/n) * n
	i0 := int(idx)
	f := idx - float32(i0)
	return lerp(sunEdge[i0%len(sunEdge)], sunEdge[(i0+1)%len(sunEdge)], f)
}

// drawSun paints the halo, the disc, its granulation, and the ragged
// corona at the limb.
//
// The disc is deliberately near-white across most of its face with the
// gold kept to a rind at the limb and to the halo around it. A sun
// ramped orange all the way to the middle reads as an amber lamp; the
// heat is supposed to be at the core.
func drawSun(a *App, dc *gui.DrawContext) {
	cx, cy := a.SunX, a.SunY
	r := max(a.SunR, 1.5)

	pulse := 1 + 0.06*sin32(2*math.Pi*glowPulseHz*a.Time)
	outer := r * glowExtent * pulse

	// Hovering brightens the halo rather than adding a ring of its own:
	// the sun is already the brightest thing on the canvas, so a halo
	// drawn on top of this one would not be visible as a change.
	glowAlpha := float32(0.38)
	if a.Hovered == selSun {
		glowAlpha = 0.58
		outer *= 1.1
	}

	rings := int(clamp32(outer*glowRingsPerPx, glowRingsMin, glowRingsMax))
	for i := rings; i > 0; i-- {
		t := float32(i) / float32(rings) // 1 at the outer edge
		ringR := lerp(r, outer, t)
		// Cubic falloff. Linear reads as a hard-edged disc and even
		// quadratic leaves a visible shoulder at this extent; the cube
		// puts most of the opacity in the innermost third, which is
		// what a real halo looks like.
		f := 1 - t
		alpha := f * f * f * glowAlpha
		dc.FilledCircle(cx, cy, ringR, colorSunGlow.WithOpacity(alpha))
	}

	// The disc: shells from the gold limb through cream to a white
	// core. Three stops for the same reason the planets use three —
	// two would walk the middle of the face through a washed-out
	// midpoint of the two ends.
	shells := int(clamp32(r*sunShellsPerPx, sunShellsMin, sunShellsMax))
	for i := range shells {
		t := float32(i) / float32(shells-1) // 0 at the limb, 1 at the core
		dc.FilledCircle(cx, cy, r*(1-0.97*t), sunDiscTone(t))
	}

	drawGranulation(dc, cx, cy, r)
	drawCorona(dc, cx, cy, r, a.Time)
}

// sunDiscTone ramps the face from the limb (t = 0) to the core
// (t = 1), in three segments: a bright rind, the body behind it, then
// the white core. The dip between the first two is the point — it is
// what limb brightening looks like.
func sunDiscTone(t float32) gui.Color {
	switch {
	case t < discRimStop:
		return mixColor(colorSunLimb, colorSunBody, t/discRimStop)
	case t < discCoreStop:
		return colorSunBody
	default:
		return mixColor(colorSunBody, colorSunCore,
			(t-discCoreStop)/(1-discCoreStop))
	}
}

// drawGranulation lays the mottled convection texture over the disc.
//
// Tier-major, and lit and dark in separate passes: every circle inside
// one pass shares a color, and getBatch merges consecutive same-color
// triangles, so the whole texture costs 2*granuleTiers batches instead
// of one per cell. Drawing cell-major would cost granuleCount times
// more.
func drawGranulation(dc *gui.DrawContext, cx, cy, r float32) {
	if r < granuleMinRadius {
		return // the cells would be sub-pixel; they would only cost batches
	}
	for _, lit := range [2]bool{true, false} {
		base := colorSunGranuleDark
		if lit {
			base = colorSunGranuleLit
		}
		for tier := range granuleTiers {
			// Outermost tier first, so the stack builds a soft falloff
			// rather than a hard disc.
			scale := 1 - float32(tier)/granuleTiers
			for _, g := range sunGranules {
				if g.lit != lit {
					continue
				}
				dc.FilledCircle(
					cx+cos32(g.ang)*g.dist*r,
					cy+sin32(g.ang)*g.dist*r,
					g.size*r*scale, base)
			}
		}
	}
}

// drawCorona paints the ragged bright fringe just outside the limb.
//
// The rim is what makes the sun read as a star rather than a disc, and
// it has to be irregular: a clean circular edge looks like a coin no
// matter how bright it is.
//
// The tiers are *nested* annuli sharing boundaries — each band's inner
// edge is the previous band's outer edge — rather than a stack all
// starting from the same inner radius. Stacking them accumulates alpha
// at that shared inner edge and paints a hard bright ring inside the
// limb, which is a worse artifact than the one it was meant to fix.
// The same noise modulates every boundary, so the raggedness is
// coherent from the limb outward instead of a set of independent
// wobbles, and its index drifts with time so the fringe crawls.
func drawCorona(dc *gui.DrawContext, cx, cy, r, now float32) {
	var quad [8]float32
	drift := now * coronaDriftHz * float32(coronaSteps)

	for tier := range coronaTiers {
		// Innermost, brightest band first; each one starts where the
		// last ended.
		f0 := float32(tier) / coronaTiers
		f1 := float32(tier+1) / coronaTiers
		alpha := (1 - f0) * (1 - f0) * 0.5
		col := colorSunCorona.WithOpacity(alpha)

		aX, aY := coronaPoint(r, 0, f0, drift)
		bX, bY := coronaPoint(r, 0, f1, drift)
		for k := 1; k <= coronaSteps; k++ {
			ang := 2 * math.Pi * float32(k) / coronaSteps
			cX, cY := coronaPoint(r, ang, f0, drift)
			dX, dY := coronaPoint(r, ang, f1, drift)
			quad = [8]float32{
				cx + aX, cy + aY,
				cx + cX, cy + cY,
				cx + dX, cy + dY,
				cx + bX, cy + bY,
			}
			dc.FilledPolygon(quad[:], col)
			aX, aY, bX, bY = cX, cY, dX, dY
		}
	}
}

// coronaPoint is a point on the fringe boundary at fraction f of the
// corona's reach, f = 0 being the limb itself. The boundary starts a
// hair inside the limb so the innermost band always covers the disc's
// polygon edge.
func coronaPoint(r, ang, f, drift float32) (x, y float32) {
	n := edgeAt(ang/(2*math.Pi)*coronaSteps + drift)
	reach := coronaReach * f * (1 - coronaRagged + coronaRagged*n)
	rr := r * (1 - 0.015 + reach)
	return cos32(ang) * rr, sin32(ang) * rr
}

// drawPlanets paints the bodies back to front. With the disc tilted,
// "back" is simply smaller screen Y, so ordering by ScreenY makes a
// near planet correctly overlap a far one. Eight elements, so an
// insertion sort over a fixed array beats sort.Slice and allocates
// nothing.
func drawPlanets(a *App, dc *gui.DrawContext) {
	var order [len(planets)]int
	for i := range order {
		order[i] = i
	}
	for i := 1; i < len(order); i++ {
		k := order[i]
		j := i - 1
		for j >= 0 && a.ScreenY[order[j]] > a.ScreenY[k] {
			order[j+1] = order[j]
			j--
		}
		order[j+1] = k
	}

	for _, i := range order {
		drawPlanet(a, dc, i)
	}
	// Labels last, so a nearer body cannot paint over a farther one's
	// name.
	for _, i := range order {
		drawLabel(a, dc, i)
	}
}

func drawPlanet(a *App, dc *gui.DrawContext, i int) {
	p := &planets[i]
	cx, cy := a.ScreenX[i], a.ScreenY[i]
	r := max(a.ScreenR[i], 1.5)

	// Cull off-screen bodies. The shading ramp's step count scales with
	// pixel radius, so at a planet-focused zoom the seven planets that
	// are nowhere near the viewport were still each emitting a
	// full-resolution ramp — 537 batches a frame instead of ~120.
	// The margin covers Saturn's rings, which reach about 2r.
	if m := r * 2.5; cx+m < 0 || cx-m > dc.Width ||
		cy+m < 0 || cy-m > dc.Height {
		return
	}

	// Hover halo: the same concentric-ring glow at planet scale.
	if i == a.Hovered {
		// Reach is capped in pixels as well as proportionally: on a
		// planet zoomed to fill the view, a purely proportional halo
		// washes the whole canvas in its own color.
		reach := min(r*1.1, 42)
		for h := hoverHalos; h > 0; h-- {
			t := float32(h) / hoverHalos
			alpha := (1 - t) * (1 - t) * 0.42
			dc.FilledCircle(cx, cy, r+reach*t, p.Color.WithOpacity(alpha))
		}
	}

	// One vector carries both facts the shading needs: which way the
	// sun lies on screen, and how far around behind the planet it is.
	lx, ly, lz := a.lightVec(i)

	if i == saturnIndex {
		// Back half of the rings first, then the body, then the front
		// half — which is what makes the rings pass behind Saturn.
		k := a.litFraction(i)
		drawRings(dc, cx, cy, r, math.Pi, math.Pi, k)
		drawBody(dc, cx, cy, r, p.Color, lx, ly, lz)
		drawRings(dc, cx, cy, r, 0, math.Pi, k)
		return
	}
	drawBody(dc, cx, cy, r, p.Color, lx, ly, lz)
}

// lightBasis is an orthonormal frame with the light direction as its
// pole. Shading a sphere is easy in this frame and awkward in any
// other: the lines of equal brightness are exactly the circles of
// constant polar angle from l, so a band of brightness is a band of
// polar angle, and nothing has to be solved for.
//
// u is the axis perpendicular to l tilted as far toward the viewer as
// it can go, so it alone carries the depth; v is what is left over,
// and it lies flat in the screen plane (its z is zero by construction,
// so it is not stored). m is the length of l's screen-plane part.
type lightBasis struct {
	lx, ly, lz float32
	ux, uy, uz float32
	vx, vy     float32
	m          float32
}

// newLightBasis builds the frame from a unit screen-space light vector.
//
// u = normalize((0,0,1) - lz*l) is the viewer direction with the part
// along l removed. v = l x u works out to the screen-plane normal of
// the light's own screen direction — the algebra collapses because
// m² + lz² = 1.
func newLightBasis(lx, ly, lz float32) lightBasis {
	m := sqrt32(lx*lx + ly*ly)
	return lightBasis{
		lx: lx, ly: ly, lz: lz,
		ux: -lz * lx / m, uy: -lz * ly / m, uz: m,
		vx: ly / m, vy: -lx / m,
		m: m,
	}
}

// point returns the screen offset from the disc center of the surface
// point at polar angle phi from the light and azimuth t around it,
// with hidden points pushed radially out onto the limb.
//
// The clamp is what closes each band off at the silhouette. A point on
// the far side has a projection strictly inside the disc, so pushing it
// out to radius r lands it on the limb — exactly where the band's
// boundary belongs, and exactly where it already is at the crossing.
func (b lightBasis) point(r, cosPhi, sinPhi, ct, st float32) (x, y float32) {
	x = r * (cosPhi*b.lx + sinPhi*(ct*b.ux+st*b.vx))
	y = r * (cosPhi*b.ly + sinPhi*(ct*b.uy+st*b.vy))
	// z = r*(cosPhi*lz + sinPhi*ct*uz); v contributes nothing to depth.
	if cosPhi*b.lz+sinPhi*ct*b.uz >= 0 {
		return x, y
	}
	d := sqrt32(x*x + y*y)
	if d == 0 {
		return x, y
	}
	return x * r / d, y * r / d
}

// visibleAzimuth returns the half-width in t of the visible part of the
// circle at polar angle phi: pi when all of it faces the viewer, 0 when
// none of it does.
//
// Visibility is z >= 0, which is cos(t) >= -cos(phi)*lz / (sin(phi)*m).
func (b lightBasis) visibleAzimuth(cosPhi, sinPhi float32) float32 {
	den := sinPhi * b.m
	if den <= 0 {
		// The circle has collapsed to a point at one pole.
		if cosPhi*b.lz >= 0 {
			return math.Pi
		}
		return 0
	}
	switch a := -cosPhi * b.lz / den; {
	case a <= -1:
		return math.Pi
	case a >= 1:
		return 0
	default:
		return acos32(a)
	}
}

// drawBody paints one shaded sphere: a feathered rim, then a Lambert
// intensity ramp laid down as flat-colored bands.
//
// The bands are the real thing rather than a stand-in. On a sphere lit
// by a distant source the brightness at a surface point is N·L, so the
// lines of equal brightness are the circles of constant angle from the
// light — and each band is the strip of surface between two of them,
// traced directly in the light-aligned frame and projected to screen.
//
// That geometry is the whole point, because it is what a crescent *is*.
// An earlier version built the body from a focal radial gradient, the
// fx/fy construction an SVG radialGradient uses, whose isophotes are
// circles. Circles cannot make a crescent: pushing the focal point out
// to the limb to fake one puts a compact bright blob on a dark disc,
// which reads as a second small sphere sitting inside the planet. The
// terminator is an ellipse, not a circle, and no amount of softening
// the blob's edge turns one into the other.
//
// Each band is emitted as a strip of quads sharing one flat color, so
// it costs one batch: the color only changes between bands, and
// getBatch merges consecutive same-color triangles. Adjacent bands
// trace the same boundary curve from both sides, so there are no seams.
func drawBody(dc *gui.DrawContext, cx, cy, r float32, base gui.Color,
	lx, ly, lz float32,
) {
	night := scaleColor(base, nightScale)
	lit := lightenColor(base, 0.26)
	// The disc's average tone, used where there is no room to shade.
	flat := sphereTone(night, base, lit, clamp32((1+lz)/2, 0, 1))

	// Feather: translucent rings just outside the silhouette. The
	// circle is a polygon, so its edge is hard; this reads as the
	// anti-aliasing the canvas does not do.
	for f := rimFeather; f > 0; f-- {
		t := float32(f) / rimFeather
		dc.FilledCircle(cx, cy, r*(1+t*0.05), flat.WithOpacity((1-t)*0.16))
	}

	// The light-aligned frame divides by the screen-plane length of l,
	// so a light pointing straight down the camera axis has no frame:
	// every axis perpendicular to it is equally valid. lightVec returns
	// exactly that for a planet sitting on the sun's own position, so
	// its divide-by-zero guard would otherwise be undone here.
	degenerate := sqrt32(lx*lx+ly*ly) < 1e-4

	if r < flatBodyRadius || degenerate {
		// Too small for the ramp to be visible (it would only cost
		// batches), or nothing to orient it by. Draw the average tone
		// flat — which for a head-on light is the fully lit tone.
		dc.FilledCircle(cx, cy, r, flat)
		return
	}

	b := newLightBasis(lx, ly, lz)
	levels := int(clamp32(r*shadeLevelsPerPx, shadeLevelsMin, shadeLevelsMax))
	arc := int(clamp32(r*shadeArcPerPx, shadeArcMin, shadeArcMax))

	// One reused quad, so the strips do not allocate per segment.
	var quad [8]float32

	// Bands are spaced evenly in *intensity*, not in angle. Even
	// angular spacing looks fine over the middle of the lit side and
	// stripes badly at the terminator, because cos is flat at the poles
	// and steepest at 90°: the same slice of angle is a much bigger
	// slice of brightness there. Stepping cos directly makes every band
	// the same color distance from its neighbour, and spends angular
	// resolution where the eye is actually looking.
	//
	// Darkest first, so the brighter band's seam overlap lands on top.
	//
	// The night half is subdivided too, even though every band there
	// comes out the same color and merges into one batch. It cannot be
	// one band: at cos = -1 the boundary circle has collapsed to the
	// antisolar point, and a strip run from a single point does not
	// cover the region — it fans out as a wedge with the rest of the
	// night side left unpainted.
	bands := levels * 2
	for j := range bands {
		c0 := -1 + 2*float32(j)/float32(bands)
		c1 := -1 + 2*float32(j+1)/float32(bands)
		shadeBand(dc, cx, cy, r, b, &quad,
			c0-2*seamOverlap/float32(bands), c1,
			sphereTone(night, base, lit, clamp32((c0+c1)/2, 0, 1)), arc)
	}
}

// shadeBand fills the strip of sphere surface whose Lambert intensity
// lies between cOut and cIn, as a run of quads sharing one flat color —
// so the whole band costs a single batch.
func shadeBand(dc *gui.DrawContext, cx, cy, r float32, b lightBasis,
	quad *[8]float32, cOut, cIn float32, col gui.Color, arc int,
) {
	cOut = clamp32(cOut, -1, 1)
	s0, s1 := sqrt32(1-cOut*cOut), sqrt32(1-cIn*cIn)

	tMax := max(b.visibleAzimuth(cOut, s0), b.visibleAzimuth(cIn, s1))
	if tMax <= 0 {
		return // this band is entirely on the far side
	}

	step := 2 * tMax / float32(arc)
	over := step * seamOverlap
	for k := range arc {
		// Every quad is grown slightly past its own share of the
		// surface. Adjacent triangles that merely *share* an edge do
		// not tile cleanly: the rasterizer antialiases each one on its
		// own, so both sides come out at partial coverage and the
		// background shows through as a hairline. The bands are
		// opaque, so overlapping them costs nothing and closes the
		// seam.
		ta := -tMax + step*float32(k) - over
		tb := ta + step + 2*over
		cta, sta := cos32(ta), sin32(ta)
		ctb, stb := cos32(tb), sin32(tb)

		ax0, ay0 := b.point(r, cOut, s0, cta, sta)
		bx0, by0 := b.point(r, cOut, s0, ctb, stb)
		ax1, ay1 := b.point(r, cIn, s1, cta, sta)
		bx1, by1 := b.point(r, cIn, s1, ctb, stb)

		*quad = [8]float32{
			cx + ax0, cy + ay0,
			cx + bx0, cy + by0,
			cx + bx1, cy + by1,
			cx + ax1, cy + ay1,
		}
		dc.FilledPolygon(quad[:], col)
	}
}

// sphereTone is the body's color ramp against Lambert intensity: the
// night tone where no light reaches, the planet's own color across most
// of the lit side, a modest lift at the brightest point.
//
// Three stops, not two. Interpolating straight from dark to light walks
// through the midpoint of those two, which is a desaturated grey — the
// planet loses its hue and reads as a lit stone. Keeping the base color
// as a middle stop is what an SVG gradient would do, and it is what
// keeps Uranus cyan and Mars red.
//
// The lift is modest for the same reason: pushing far toward white
// washes the bright end out to a pale disc and desaturates the body at
// the same time.
func sphereTone(night, base, lit gui.Color, intensity float32) gui.Color {
	if intensity < baseStop {
		return mixColor(night, base, intensity/baseStop)
	}
	return mixColor(base, lit, (intensity-baseStop)/(1-baseStop))
}

// drawLabel names a body under its disc, the way a reference orrery
// does. The hovered or selected planet takes the brighter style so it
// separates from its neighbours.
func drawLabel(a *App, dc *gui.DrawContext, i int) {
	r := max(a.ScreenR[i], 1.5)
	cx, cy := a.ScreenX[i], a.ScreenY[i]
	if cx < -200 || cx > dc.Width+200 || cy < -200 || cy > dc.Height+200 {
		return
	}
	if i == a.Selected {
		// The info panel already names it; a drawn label under a body
		// that fills the screen is just a third copy.
		return
	}
	style := a.LabelStyle
	if i == a.Hovered {
		style = a.TipStyle
	}
	name := planets[i].Name
	dc.Text(cx-dc.TextWidth(name, style)/2, cy+r+labelGap, name, style)
}

// scaleColor multiplies RGB by f, keeping alpha.
func scaleColor(c gui.Color, f float32) gui.Color {
	return gui.RGBA(chan8(float32(c.R)*f), chan8(float32(c.G)*f),
		chan8(float32(c.B)*f), c.A)
}

// lightenColor mixes toward white by t.
func lightenColor(c gui.Color, t float32) gui.Color {
	return mixColor(c, gui.RGBA(255, 255, 255, c.A), t)
}

// mixColor linearly interpolates two colors, alpha included.
func mixColor(a, b gui.Color, t float32) gui.Color {
	t = clamp32(t, 0, 1)
	mix := func(x, y uint8) uint8 {
		return chan8(lerp(float32(x), float32(y), t))
	}
	return gui.RGBA(mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B),
		mix(a.A, b.A))
}

// chan8 clamps a float channel into a byte.
func chan8(v float32) uint8 { return uint8(clamp32(v, 0, 255)) }

// drawRings draws Saturn's rings as three concentric ellipse arcs over
// the given angular span.
func drawRings(dc *gui.DrawContext, cx, cy, r, start, sweep,
	litFrac float32,
) {
	// The rings take the same sunlight the body does, so they fade with
	// the phase rather than staying bright over a night-side Saturn.
	col := colorRing.WithOpacity(ringNightAlpha + (1-ringNightAlpha)*litFrac)
	// Stroke width scales with the body but stays hairline-ish: at
	// r*0.10 the bands read as three brown tubes rather than rings.
	width := clamp32(r*0.035, 1, 5)
	for _, k := range [3]float32{1.42, 1.70, 1.96} {
		rx := r * k
		ry := rx * 0.26
		if rx < 3 {
			continue
		}
		dc.Arc(cx, cy, rx, ry, start, sweep, col, width)
	}
}

// drawTooltip labels the hovered body next to the cursor.
//
// The built-in gui.WithTooltip anchors to a widget and cannot follow a
// cursor or target a sub-region of a canvas, so the bubble is drawn
// here and clamped to the canvas so it never runs off an edge.
func drawTooltip(a *App, dc *gui.DrawContext) {
	b := bodyAt(a.Hovered)
	if !a.MouseIn || b == nil {
		return
	}
	name := b.Name
	tw := dc.TextWidth(name, a.TipStyle)
	th := dc.FontHeight(a.TipStyle)
	bw := tw + 2*tipPadX
	bh := th + 2*tipPadY

	x := a.MouseX + tipGap
	y := a.MouseY - bh - tipGap*0.5
	// Flip to the other side of the cursor rather than merely clamping
	// when the bubble would leave the canvas: clamping alone parks it
	// under the pointer.
	if x+bw > dc.Width {
		x = a.MouseX - tipGap - bw
	}
	if y < 0 {
		y = a.MouseY + tipGap
	}
	x = clamp32(x, 0, max(0, dc.Width-bw))
	y = clamp32(y, 0, max(0, dc.Height-bh))

	dc.FilledRoundedRect(x, y, bw, bh, 5, colorTipBG)
	dc.RoundedRect(x, y, bw, bh, 5, colorTipBorder, 1)
	dc.Text(x+tipPadX, y+tipPadY, name, a.TipStyle)
}
