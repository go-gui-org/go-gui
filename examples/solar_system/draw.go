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
	// Jupiter puts a ~380px halo on screen, where 30 rings is a 13px
	// band apiece.
	glowRingsPerPx = 0.22
	glowRingsMin   = 24
	glowRingsMax   = 140

	// glowExtent is the outer glow radius in multiples of sunRadius.
	// Worth knowing where Mercury is: its closest *projected* approach
	// is at the semi-minor ends of its orbit, where the world offset
	// is (-a*e, +/-b) = (-28, +/-133) and the vertical axis is
	// squashed by diskTilt, giving a screen distance of
	// sqrt(28^2 + (133*0.48)^2) ~= 70 world units, or 1.34 sun radii.
	// So 1.7 does put Mercury inside the glow twice per orbit; the low
	// alpha and the cubic falloff are what keep that from reading as
	// flying through a cloud. Clearing the pass outright needs 1.2.
	glowExtent  = 1.7
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

	// shadeRows is how many rings of constant Lambert intensity the
	// sphere mesh is built from, and shadeArc how many segments each
	// ring is traced with. Both scale with the body's pixel radius,
	// bounded by their Min/Max, and are wasted on a 4px Mercury.
	//
	// These counts buy *geometry*, not color steps. The mesh carries a
	// color per vertex, so the intensity ramp is continuous however
	// coarse the rows are; what the rows have to resolve is the
	// curvature of the elliptical rings and the chord error where the
	// mesh boundary meets the limb. Both fall off as the square of the
	// spacing, which is why far fewer rows suffice here than the flat
	// bands this replaced needed to hide their quantization.
	// The Max ends are set by *texture* resolution rather than by
	// silhouette smoothness, which the far lower ceilings this
	// replaced already handled. A body's albedo is sampled per vertex
	// and interpolated between vertices, so at a selected planet's
	// zoom — up to roughly 640 px of radius — an arc of 64 puts one
	// quad across 30-60 px and the surface reads as coarse blobs. 72
	// x 128 is matched to the 128x64 texture. The PerPx rates are
	// unchanged, so these ceilings are only reached past 144 px and
	// 183 px of radius: a zoomed selection, never the eight small
	// bodies of the full-system view.
	shadeRowsPerPx = 0.5
	shadeRowsMin   = 12
	shadeRowsMax   = 72
	shadeArcPerPx  = 0.7
	shadeArcMin    = 16
	shadeArcMax    = 128

	// nightScale is the unlit hemisphere's brightness, as a fraction of
	// the body's own color. Not zero on purpose: a planet you cannot see
	// is a planet you cannot click, and this is an orrery before it is a
	// simulation.
	nightScale = 0.16

	// baseStop is where the body's own color sits on the intensity ramp.
	// Well past the middle, so most of the lit side carries the planet's
	// hue and the bright end stays tight.
	baseStop = 0.72

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
	// Alpha comes down with the extent. The cubic falloff piles most
	// of the opacity into the innermost third, so a halo squeezed from
	// 2.2 to 1.2 radii concentrates what is left into a much smaller
	// area and would read *brighter* at the limb if the alpha stayed.
	glowAlpha := float32(0.30)
	if a.Hovered == selSun {
		glowAlpha = 0.46
		outer *= 1.1
	}

	// One radial fill. The cubic falloff is the shape the ring stack
	// was approximating — linear reads as a hard-edged disc and even
	// quadratic leaves a visible shoulder at this extent, while the
	// cube puts most of the opacity in the innermost third, which is
	// what a real halo looks like. rings no longer sets the smoothness,
	// only the depth the stack would have accumulated to.
	rings := int(clamp32(outer*glowRingsPerPx, glowRingsMin, glowRingsMax))
	a.glowStops = haloStops(colorSunGlow, r, outer, glowAlpha, 3, rings,
		a.glowStops)
	dc.FilledCircleGradient(cx, cy, outer, &gui.CanvasGradient{
		Radial: true,
		Stops:  a.glowStops,
	})

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

	// Cull off-screen bodies. The shading mesh's ring count scales with
	// pixel radius, so at a planet-focused zoom the seven planets that
	// are nowhere near the viewport were each still building a
	// full-resolution sphere, thousands of triangles apiece.
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
		a.haloStops = haloStops(p.Color, r, r+reach, 0.42, 2, hoverHalos,
			a.haloStops)
		dc.FilledCircleGradient(cx, cy, r+reach, &gui.CanvasGradient{
			Radial: true,
			Stops:  a.haloStops,
		})
	}

	// One vector carries both facts the shading needs: which way the
	// sun lies on screen, and how far around behind the planet it is.
	lx, ly, lz := a.lightVec(i)

	// Stack-allocated: initSurface fills it, and drawBody does not let
	// the pointer escape.
	var sf surface
	var sp *surface
	if initSurface(&sf, i, a.Time) {
		sp = &sf
	}

	if i == saturnIndex {
		// Back half of the rings first, then the body, then the front
		// half — which is what makes the rings pass behind Saturn.
		k := a.litFraction(i)
		drawRings(dc, cx, cy, r, math.Pi, math.Pi, k)
		drawBody(dc, &a.body, cx, cy, r, p.Color, lx, ly, lz, sp)
		drawRings(dc, cx, cy, r, 0, math.Pi, k)
		return
	}
	drawBody(dc, &a.body, cx, cy, r, p.Color, lx, ly, lz, sp)
}

// bodyLitLift is how far the brightest point of a body is pushed
// toward white. Named because the textured path has to reproduce the
// same ramp arithmetically rather than by calling sphereTone.
const bodyLitLift = 0.26

// surface is the texture half of a body's appearance: its albedo map
// and the orientation of the body-fixed frame that map is pinned to.
// A nil *surface means no texture, and the flat per-ring ramp that
// drawBody has always drawn.
//
// The axes are already in *camera* coordinates. Both the axial tilt
// and the camera's elevation are constants, so the world-to-camera
// rotation can be applied once when the surface is built and never
// again — which is what removes the world transform from the
// per-vertex path entirely.
type surface struct {
	tex           *bodyTexture
	ax, ay, az    float32 // spin axis
	e1x, e1y, e1z float32 // equator reference direction
	e2x, e2y, e2z float32 // completes the right-handed frame
	spin          float32 // rotation so far, in turns
}

// initSurface resolves planet i's body frame into camera coordinates
// at time t.
//
// The orbital plane is the world xy-plane, so the orbital north pole
// is world +z and a tilt of theta leans the spin axis that far off it.
// Which way it leans is a free choice, taken as world +x. The camera's
// basis in world coordinates is right (1,0,0), screen-down
// (0, sinE, -cosE) and to-viewer (0, cosE, sinE) — see lightVec, which
// is where those come from — and projecting the three body axes onto
// it gives the vectors below.
//
// e1 x e2 == a, so the frame is right-handed and longitude increases
// in one consistent direction for every planet.
// It fills a caller-provided surface and reports whether the body has
// one, rather than returning a pointer, because returning one would
// put a heap allocation on the frame path: this runs per planet per
// tick, and nine escaping values a frame is the whole session's worth
// of garbage that drawBody's scratch reuse exists to avoid.
func initSurface(s *surface, i int, t float32) bool {
	p := &planets[i]
	if p.RotS == 0 || planetTextures[i] == nil {
		return false
	}
	sa, ca := sin32(p.Tilt), cos32(p.Tilt)
	// diskTilt is an untyped constant; name it float32 once here so
	// the six products below stay in one type.
	sinE, cosE := float32(diskTilt), cosElev
	*s = surface{
		tex: planetTextures[i],
		ax:  sa, ay: -ca * cosE, az: ca * sinE,
		e1x: ca, e1y: sa * cosE, e1z: -sa * sinE,
		e2x: 0, e2y: sinE, e2z: cosE,
		// Phase doubles as the starting meridian so the eight bodies
		// do not all begin with the same face toward the sun.
		spin: t/p.RotS + p.Phase*(1/(2*math.Pi)),
	}
	return true
}

// surfaceProj is the body frame resolved against the light basis:
// nine dot products, constant for a whole body.
//
// This is the step that makes per-vertex texturing affordable. A
// vertex normal is n = cosPhi*l + sinPhi*(ct*u + st*v), so for any
// fixed axis W the quantity n.W is linear in ct and st with
// coefficients that depend only on the ring. Sampling a vertex is then
// three multiply-adds per axis on values the ring already has, and no
// transcendental at all.
//
// v contributes only two terms because its z is zero by construction
// (see lightBasis).
type surfaceProj struct {
	la, ua, va float32
	l1, u1, v1 float32
	l2, u2, v2 float32
}

func (s *surface) project(b lightBasis) surfaceProj {
	return surfaceProj{
		la: b.lx*s.ax + b.ly*s.ay + b.lz*s.az,
		ua: b.ux*s.ax + b.uy*s.ay + b.uz*s.az,
		va: b.vx*s.ax + b.vy*s.ay,

		l1: b.lx*s.e1x + b.ly*s.e1y + b.lz*s.e1z,
		u1: b.ux*s.e1x + b.uy*s.e1y + b.uz*s.e1z,
		v1: b.vx*s.e1x + b.vy*s.e1y,

		l2: b.lx*s.e2x + b.ly*s.e2y + b.lz*s.e2z,
		u2: b.ux*s.e2x + b.uy*s.e2y + b.uz*s.e2z,
		v2: b.vx*s.e2x + b.vy*s.e2y,
	}
}

// ringTex is one ring's slice of all that: the linear coefficients for
// the three body axes, the spin offset, and the shading ramp folded to
// an affine (k, w) on each color channel. A nil tex means the ring
// takes its flat color instead.
type ringTex struct {
	tex        *bodyTexture
	a0, a1, a2 float32 // sin(latitude)
	b0, b1, b2 float32 // equator reference component
	c0, c1, c2 float32 // quadrature component
	spin       float32
	k, w       float32
}

// ringTexFor folds the per-body projection and the per-ring geometry
// into the coefficients appendRing consumes.
func (p surfaceProj) ringTexFor(s *surface, cosPhi, sinPhi, k, w float32) ringTex {
	return ringTex{
		tex: s.tex,
		a0:  cosPhi * p.la, a1: sinPhi * p.ua, a2: sinPhi * p.va,
		b0: cosPhi * p.l1, b1: sinPhi * p.u1, b2: sinPhi * p.v1,
		c0: cosPhi * p.l2, c1: sinPhi * p.u2, c2: sinPhi * p.v2,
		spin: s.spin,
		k:    k, w: w,
	}
}

// ringRamp is sphereTone rewritten as the affine map it already is.
//
// sphereTone mixes three colors that are all derived from one base, so
// for a fixed intensity the whole ramp collapses to channel*k + w. The
// textured path needs exactly that, because there the "base" is a
// different albedo at every vertex and calling sphereTone per vertex
// would redo the same interpolation thousands of times.
//
// This is not bit-identical to sphereTone: that function quantizes the
// night and lit tones to bytes before mixing, and this one does not.
// The difference is at most one count in a channel, and the untextured
// path still goes through sphereTone unchanged.
func ringRamp(intensity float32) (k, w float32) {
	if intensity < baseStop {
		t := intensity / baseStop
		return nightScale*(1-t) + t, 0
	}
	t := (intensity - baseStop) / (1 - baseStop)
	return 1 - bodyLitLift*t, 255 * bodyLitLift * t
}

// atan2Turns is atan2(y, x) scaled to turns, in [-0.5, 0.5].
//
// A polynomial stand-in for the real thing, with a measured worst
// case of 0.0102 rad (see TestAtan2TurnsMatchesLibrary). That sounds
// sloppy and is not: one texel of a 128-wide texture spans 0.049 rad,
// so the whole error budget is about a fifth of the smallest thing
// the result can address, and it costs no library call on a path that
// runs once per vertex.
func atan2Turns(y, x float32) float32 {
	const (
		quarter = float32(math.Pi / 4)
		toTurns = float32(1 / (2 * math.Pi))
	)
	ay := abs32(y) + 1e-10
	var a float32
	if x >= 0 {
		r := (x - ay) / (x + ay)
		a = (0.1963*r*r-0.9817)*r + quarter
	} else {
		r := (x + ay) / (ay - x)
		a = (0.1963*r*r-0.9817)*r + 3*quarter
	}
	if y < 0 {
		a = -a
	}
	return a * toTurns
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

// ringPlan lays out the rings drawBody walks, in cos(phi) — the Lambert
// intensity itself.
//
// Even steps in intensity are the obvious choice, and they are what
// made the silhouette go polygonal. The mesh boundary *is* the limb: a
// ring only partly on the near side ends on the silhouette, and the
// mesh edge between two such rings is a chord of it. A limb point's
// intensity is m*cos(delta) — delta the angle around the limb from the
// light's screen direction, m the light's screen-plane length — so even
// steps in intensity are wildly uneven steps in delta. Near delta = 0
// the step grows as a square root and one chord can span 20 degrees.
// With the light in the screen plane that lands on the disc's own edge,
// which is exactly where the facets showed: the sun-facing and anti-sun
// edges of a planet beside the sun.
//
// So the rings that reach the limb are spaced evenly in delta instead,
// which makes every limb chord the same length. Those are the rings
// with |cos(phi)| < m; outside that band a ring is either wholly
// visible — a closed curve around the pole the light points at — or
// wholly hidden. The visible cap gets rings spaced evenly in phi, the
// hidden one gets none: it covers no visible surface, and even spacing
// spent nearly half its rings there only for appendTri to drop them.
//
// Rows are split between cap and band by the share of phi each spans.
// That gives them all to the band when the light lies in the screen
// plane (there is no visible cap) and about half to the cap when it
// points nearly at the camera (the band is then a sliver around the
// terminator, which is all the limb it can touch).
type ringPlan struct {
	capPhi float32 // polar angle the visible cap spans
	m      float32 // screen-plane length of the light vector
	sgn    float32 // -1 when the visible cap is the unlit pole
	nCap   int
	nBand  int
}

func newRingPlan(b lightBasis, rows int) ringPlan {
	p := ringPlan{m: min(b.m, 1), sgn: 1}
	if b.lz < 0 {
		p.sgn = -1
	}
	p.capPhi = acos32(p.m)
	// The cap gets at least one ring even when it is tiny or empty. The
	// ring at the visible pole is what turns the cap lens into a
	// triangle fan, and without it the lens on the light side of the
	// limb shows background — a small body's ring budget can leave
	// zero cap rings while the lens is still a visible patch. When the
	// light lies in the screen plane the cap is empty and this ring is
	// the pole point itself, which is also the ring the band would
	// have started with.
	p.nCap = max(int(float32(rows)*p.capPhi/math.Pi), 1)
	// A band of four rings is a coarse crescent, but it is still a
	// crescent; the cap has taken the rest because the light is nearly
	// head-on, and then there is barely any terminator to resolve.
	p.nBand = max(rows-p.nCap, 4)
	return p
}

// count is how many rings the plan holds.
func (p ringPlan) count() int { return p.nCap + p.nBand + 1 }

// cosPhi is ring i's Lambert intensity. The sequence is monotone, from
// the visible pole through the band to the far side of the terminator,
// so consecutive rings always bound a strip.
func (p ringPlan) cosPhi(i int) float32 {
	if i < p.nCap {
		return p.sgn * cos32(p.capPhi*float32(i)/float32(p.nCap))
	}
	d := float32(math.Pi) * float32(i-p.nCap) / float32(p.nBand)
	return p.sgn * p.m * cos32(d)
}

// bodyMesh is drawBody's reusable scratch. It holds the mesh handed to
// FillTrianglesColors plus the two rings of screen positions the sphere
// is walked with, so a frame that shades nine planets builds all their
// geometry without allocating — the per-fill batch copy
// FillTrianglesColors makes is all that is left.
//
// prev and cur are one ring each, x,y interleaved. Every ring is
// evaluated once and used twice: as the outer edge of one strip and the
// inner edge of the next. That sharing is what makes the mesh
// watertight — adjacent strips do not merely meet along a boundary,
// they are built from the same vertices.
type bodyMesh struct {
	tris      []float32
	cols      []gui.Color
	prev, cur []float32

	// prevCol and curCol are the vertex colors of those same two
	// rings, one per position. Color is per vertex rather than per
	// ring so the albedo can vary along a ring while the Lambert term
	// stays constant on it; with a flat color they hold arc+1 copies
	// of one value and the mesh is identical to a per-ring ramp.
	prevCol, curCol []gui.Color
}

// appendTri adds one triangle, wound consistently with every other
// triangle in the mesh, and drops it if it has no area.
//
// Winding is not cosmetic. gui/backend/soft rasterizes a vertex-colored
// batch as a single path and accumulates *signed* coverage, so a
// triangle wound against its neighbours subtracts from them and carves
// a seam through the body. lightBasis.point pushes hidden vertices out
// onto the limb, which can reorder a quad's corners, so the sign is
// measured rather than assumed.
//
// Dropping the zero-area case is also what removes the rings that lie
// entirely on the far side: every one of their points is pushed to the
// same limb position, so their strips collapse.
func (m *bodyMesh) appendTri(x0, y0 float32, c0 gui.Color,
	x1, y1 float32, c1 gui.Color, x2, y2 float32, c2 gui.Color,
) {
	switch area := (x1-x0)*(y2-y0) - (x2-x0)*(y1-y0); {
	case area > 0:
		m.tris = append(m.tris, x0, y0, x1, y1, x2, y2)
		m.cols = append(m.cols, c0, c1, c2)
	case area < 0:
		m.tris = append(m.tris, x0, y0, x2, y2, x1, y1)
		m.cols = append(m.cols, c0, c2, c1)
	}
}

// drawBody paints one shaded sphere: a feathered rim, then a Lambert
// intensity ramp laid down as a single vertex-colored mesh.
//
// The mesh is the real thing rather than a stand-in. On a sphere lit by
// a distant source the brightness at a surface point is N·L, so the
// lines of equal brightness are the circles of constant angle from the
// light. The mesh is those circles: one ring of vertices per intensity,
// traced in the light-aligned frame and projected to screen, with the
// intensity carried as a color on every vertex.
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
// Nor can any other gradient. The isophote at N·L = k projects to an
// ellipse whose semi-axis is sqrt(1-k²): a point at k = -1, the full
// limb at the terminator, a point again at k = 1. Gradient level sets
// are nested by construction and grow linearly, so no gradient
// primitive of any kind emits that family — and none of them knows to
// stop at the silhouette. Hence FillTrianglesColors (issue #400): the
// shading model is evaluated here, per vertex, and handed over already
// solved.
//
// The whole body is one batch.
// s carries the body's surface texture and orientation, or is nil for
// an untextured body — in which case every vertex on a ring takes the
// ring's flat tone and the mesh is exactly what it was before
// textures existed.
func drawBody(dc *gui.DrawContext, m *bodyMesh, cx, cy, r float32,
	base gui.Color, lx, ly, lz float32, s *surface,
) {
	// The frame, the visibility test and ringPlan's tangency all read
	// the light as a unit vector; normalizing once here is cheaper than
	// each of them defending itself.
	if d := sqrt32(lx*lx + ly*ly + lz*lz); d > 0 {
		lx, ly, lz = lx/d, ly/d, lz/d
	}
	night := scaleColor(base, nightScale)
	lit := lightenColor(base, bodyLitLift)
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
		// Too small for the ramp to be visible, or nothing to orient it
		// by. Draw the average tone flat — which for a head-on light is
		// the fully lit tone.
		dc.FilledCircle(cx, cy, r, flat)
		return
	}

	b := newLightBasis(lx, ly, lz)
	rows := int(clamp32(r*shadeRowsPerPx, shadeRowsMin, shadeRowsMax))
	arc := int(clamp32(r*shadeArcPerPx, shadeArcMin, shadeArcMax))
	p := newRingPlan(b, rows)

	m.tris, m.cols = m.tris[:0], m.cols[:0]

	// Nine dot products for the whole body; the per-ring and
	// per-vertex work below is all multiply-adds on top of them.
	var proj surfaceProj
	if s != nil {
		proj = s.project(b)
	}

	// Where the rings go is ringPlan's decision, and not the obvious
	// one; see it for why even steps in intensity make the silhouette
	// go polygonal.
	for i := range p.count() {
		c := p.cosPhi(i)
		sn := sqrt32(max(0, 1-c*c))
		intensity := clamp32(c, 0, 1)
		col := sphereTone(night, base, lit, intensity)
		var rt ringTex
		if s != nil {
			k, w := ringRamp(intensity)
			rt = proj.ringTexFor(s, c, sn, k, w)
		}
		m.cur, m.curCol = appendRing(m.cur[:0], m.curCol[:0],
			b, r, cx, cy, c, sn, arc, col, rt)
		if i > 0 {
			m.appendStrip(arc)
		}
		// Positions and colors are two halves of one ring and must be
		// swapped together, or a strip would pair ring i's vertices
		// with ring i-1's colors.
		m.prev, m.cur = m.cur, m.prev
		m.prevCol, m.curCol = m.curCol, m.prevCol
	}
	dc.FillTrianglesColors(m.tris, m.cols)
}

// ringAngles walks (cos t, sin t) along a ring by a fixed angular
// step, so the whole ring costs four transcendentals instead of two per
// vertex. appendRing's step is uniform by construction, which is what
// makes the rotation an increment rather than a fresh evaluation.
//
// This is the stable form of the recurrence (Numerical Recipes 5.4),
// not the naive product one. Writing the rotation as a *correction* to
// the current value keeps the correction small, so its rounding error
// is small in absolute terms too; the product form rounds the full
// value every step and its magnitude walks away from 1. Magnitude is
// exactly what must not drift here: |c| or |s| creeping above 1 puts a
// vertex outside the disc, which is the property
// TestBodyMeshStaysInsideDisc pins.
//
// The seed and the two coefficients are computed in float64 because
// they are paid once per ring, not once per vertex, so the wider math
// is free and it keeps the starting error at float32 rounding instead
// of compounding a float32 one over up to shadeArcMax steps.
type ringAngles struct {
	c, s        float32
	alpha, beta float32
}

// newRingAngles seeds the walk at t0 with a per-vertex step of step
// radians. alpha is 2*sin^2(step/2) and beta is sin(step).
func newRingAngles(t0, step float64) ringAngles {
	h := math.Sin(step / 2)
	sn, cs := math.Sincos(t0)
	return ringAngles{
		c:     float32(cs),
		s:     float32(sn),
		alpha: float32(2 * h * h),
		beta:  float32(math.Sin(step)),
	}
}

// next advances one step. A wholly hidden ring has step 0, hence
// alpha and beta 0, and every vertex repeats the seed — the single
// collapsed point appendTri drops.
func (a *ringAngles) next() {
	c, s := a.c, a.s
	a.c = c - (a.alpha*c + a.beta*s)
	a.s = s - (a.alpha*s - a.beta*c)
}

// appendRing evaluates one circle of constant Lambert intensity into
// dst as arc+1 screen x,y pairs, spanning that circle's *own* visible
// azimuth range.
//
// Each ring gets its own range rather than a shared one, so a ring that
// is only partly on the near side ends exactly at the silhouette and
// its two end points land on the limb. The mesh boundary is then a
// chord of the limb per row, and lightBasis.point has already pushed
// any hidden interior point out onto the limb as well. A ring entirely
// on the far side collapses to a single point; it covers no visible
// surface, and appendTri drops the strips it takes part in.
// Colors are written to cdst in step with the positions, one per
// vertex. With rt.tex nil every vertex on the ring takes col, since
// the Lambert term is constant along a ring by construction. With a
// texture the Lambert term is still constant and only the albedo
// varies, which is what keeps the sampling cheap.
//
// The direction sampled is the true surface normal, not the position
// b.point returns: point pushes hidden vertices out onto the limb, and
// the linearization deliberately does not model that clamp. It does
// not need to. Each ring spans its own visible azimuth, so interior
// vertices are genuinely visible and the two end points sit where
// z is about 0 and the clamp does nothing. The clamp only really acts
// on rings that are wholly hidden, whose strips have no area and are
// dropped by appendTri.
func appendRing(dst []float32, cdst []gui.Color, b lightBasis,
	r, cx, cy, cosPhi, sinPhi float32, arc int, col gui.Color,
	rt ringTex,
) ([]float32, []gui.Color) {
	tMax := b.visibleAzimuth(cosPhi, sinPhi)
	step := 2 * float64(tMax) / float64(arc)
	ang := newRingAngles(-float64(tMax), step)
	// The two end points are the ring's limb vertices, and there
	// lightBasis.point's z >= 0 test is a knife edge: an ulp of
	// difference in cos t decides whether the vertex is pushed out onto
	// the limb or left where it fell, which is a fraction of a pixel of
	// silhouette. So neither end point is left to the walk. The first
	// is the seed, and the last is its mirror — cos is even and sin is
	// odd about t = 0, and the range is symmetric — which costs
	// nothing and is no less exact than evaluating cos32(tMax) would
	// be.
	c0, s0 := ang.c, ang.s
	for k := 0; k <= arc; k++ {
		ct, st := ang.c, ang.s
		if k == arc {
			ct, st = c0, -s0
		}
		// Advanced here rather than at the bottom of the loop, which
		// the flat-color path leaves by continue.
		ang.next()
		x, y := b.point(r, cosPhi, sinPhi, ct, st)
		dst = append(dst, cx+x, cy+y)
		if rt.tex == nil {
			cdst = append(cdst, col)
			continue
		}
		sinLat := rt.a0 + ct*rt.a1 + st*rt.a2
		p1 := rt.b0 + ct*rt.b1 + st*rt.b2
		p2 := rt.c0 + ct*rt.c1 + st*rt.c2
		texel := rt.tex.at(sinLat, atan2Turns(p2, p1)-rt.spin)
		cdst = append(cdst, gui.RGBA(
			chan8(float32(texel.R)*rt.k+rt.w),
			chan8(float32(texel.G)*rt.k+rt.w),
			chan8(float32(texel.B)*rt.k+rt.w),
			col.A))
	}
	return dst, cdst
}

// appendStrip tiles the surface between the two rings currently held in
// prev and cur with quads, two triangles apiece. Each vertex carries
// the color its own ring assigned it, and the rasterizer interpolates
// across the quad — which is the ramp.
func (m *bodyMesh) appendStrip(arc int) {
	for k := range arc {
		ax0, ay0 := m.prev[k*2], m.prev[k*2+1]
		bx0, by0 := m.prev[k*2+2], m.prev[k*2+3]
		ax1, ay1 := m.cur[k*2], m.cur[k*2+1]
		bx1, by1 := m.cur[k*2+2], m.cur[k*2+3]
		ac0, bc0 := m.prevCol[k], m.prevCol[k+1]
		ac1, bc1 := m.curCol[k], m.curCol[k+1]
		m.appendTri(ax0, ay0, ac0, bx0, by0, bc0, bx1, by1, bc1)
		m.appendTri(ax0, ay0, ac0, bx1, by1, bc1, ax1, ay1, ac1)
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

// haloStops samples an accumulated-ring glow profile into gradient
// stops, for a fill that runs from the body's edge out to reach.
//
// Every glow on this canvas used to be a stack of translucent discs of
// decreasing radius. Painted back to front, their composite opacity at
// radius rho was 1 - product(1 - a_i) over the rings outside it — and
// with each a_i small that product is exp(-sum a_i), which integrates
// in closed form. Sampling that curve keeps exactly the look the stack
// had while emitting one fill instead of up to 140, and drops the
// banding that made ring count a tuning knob in the first place: the
// gradient interpolates between stops instead of stepping.
//
// falloff is the exponent the old per-ring alpha used ((1-t)^falloff),
// peak its scale, and rings the count it would have drawn — the three
// numbers that set the curve's shape and depth.
func haloStops(c gui.Color, inner, outer float32,
	peak float32, falloff int, rings int,
	dst []gui.GradientStop,
) []gui.GradientStop {
	dst = dst[:0]
	if outer <= 0 || inner < 0 || inner >= outer {
		return dst
	}
	// Integrating (1-u)^falloff over [u,1] raises the exponent by one.
	e := float64(falloff + 1)
	k := float64(float32(rings)*peak) / e
	alphaAt := func(u float32) float32 {
		f := math.Pow(float64(1-u), e)
		return float32(1 - math.Exp(-k*f))
	}

	// The glow is flat across the body it surrounds: the disc covers
	// that region anyway, and a stop at 0 keeps the ramp from starting
	// at the center.
	core := alphaAt(0)
	inFrac := inner / outer
	dst = append(dst,
		gui.GradientStop{Color: c.WithOpacity(core), Pos: 0},
		gui.GradientStop{Color: c.WithOpacity(core), Pos: inFrac})

	// Eight samples across the falloff. The curve is smooth and the
	// renderer interpolates between stops, so this is well past the
	// point where more of them change anything.
	const samples = 8
	for i := 1; i <= samples; i++ {
		u := float32(i) / samples
		pos := inFrac + (1-inFrac)*u
		dst = append(dst,
			gui.GradientStop{Color: c.WithOpacity(alphaAt(u)), Pos: pos})
	}
	return dst
}
