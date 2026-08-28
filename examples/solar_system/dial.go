package main

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	// dialInner/Outer are world-unit radii of the calendar ring,
	// just outside Neptune's aphelion (~730).
	dialInner float32 = 780
	dialOuter float32 = 830

	// dialTextR is the radius glyph baselines sit at. Outside the
	// outermost rail so labels read from outside the whole way around.
	// Reduced to 838 so December/January and June/July (east/west)
	// baselines plus the 18.7-unit cap height stay inside the framed
	// extent (dialOuter 830 ×1.04 = 863 world) with a few pixels of
	// screen margin; at 860 the caps sat on the window edge.
	dialTextR float32 = 838

	// dialEmWorld is how tall an em is in world units at the label
	// radius. Chosen so labels read as engraving, not headline —
	// legible in the full-system view without dominating the system.
	dialEmWorld float32 = 15

	// dialStrokeW is the screen-space half-thickness of label strokes,
	// in pixels. Fixed in screen space so weight stays uniform despite
	// the in-plane squash.
	dialStrokeW float32 = 0.85

	// dialLabelMinPx is the legibility gate: below this em height in
	// pixels, skip labels.
	dialLabelMinPx float32 = 6

	// dialDayMinRx is the gate below which day ticks are dropped,
	// leaving only month ticks when zoomed out.
	dialDayMinRx float32 = 110
)

// monthNames in calendar order; angle 0 (world +x) is Jan 1.
var monthNames = [12]string{
	"JANUARY", "FEBRUARY", "MARCH", "APRIL",
	"MAY", "JUNE", "JULY", "AUGUST",
	"SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER",
}

var monthDays = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

var monthStartDay = [13]int{} // cumulative days, 0..365
var monthStartFrac = [13]float32{}
var monthMidAngle = [12]float32{}

func init() {
	s := 0
	for i, d := range monthDays {
		monthStartDay[i] = s
		monthStartFrac[i] = float32(s) / 365
		s += d
	}
	monthStartDay[12] = s
	monthStartFrac[12] = 1
	for i := range 12 {
		midDay := float32(monthStartDay[i]) + float32(monthDays[i])*0.5
		monthMidAngle[i] = 2 * float32(math.Pi) * midDay / 365
	}

	// Only now that the calendar is filled.
	dialDayTicks = makeDialTicks(365, dialInner,
		dialInner+(dialOuter-dialInner)*0.48)
	dialMonthTicks = makeDialMonthTicks()
	dialLabelSegs = makeDialLabelSegs()
}

// dialAngle is the heliocentric longitude the calendar reads as "now".
// It tracks Earth's orbital angle directly.
func dialAngle(t float32) float32 {
	p := &planets[earthIndex]
	return p.Phase + 2*float32(math.Pi)*t/p.PeriodS
}

// dialColors.
var (
	colorDialRail   = gui.RGBA(170, 180, 200, 78)
	colorDialTick   = gui.RGBA(170, 180, 200, 95)
	colorDialMonth  = gui.RGBA(200, 210, 230, 130)
	colorDialText   = gui.RGBA(214, 220, 238, 210)
	colorDialMarker = gui.RGBA(255, 210, 110, 220)
)

// drawDial paints the calendar ring. It lives in the orbital plane,
// so every point goes through worldToScreen. Three concentric rails
// are drawn as ellipses, ticks and labels go into one vertex-colored
// mesh, and a date marker tracks dialAngle.
func drawDial(a *App, dc *gui.DrawContext) {
	z := a.zoom()
	rx := dialOuter * z
	// Cull when zoomed to a planet: at 30x the ring is nowhere near.
	if rx > 4*max(dc.Width, dc.Height) {
		return
	}
	// Also cull if ring is too small to see? No, rails still visible.
	cx, cy := a.worldToScreen(0, 0)

	// Three rails, same squash as orbits.
	if rx >= 2 {
		for _, r := range [...]float32{dialInner, (dialInner + dialOuter) * 0.5, dialOuter} {
			rrx := r * z
			rry := rrx * diskTilt
			if rrx < 1 || rry < 0.5 {
				continue
			}
			dc.Arc(cx, cy, rrx, rry, 0, 2*math.Pi, colorDialRail, 1)
		}
	}

	// Build ticks + labels + marker into one mesh.
	m := &a.dial
	m.tris, m.cols = m.tris[:0], m.cols[:0]

	// Day ticks: 365 short radial segments.
	if rx >= dialDayMinRx {
		appendDialSegs(m, a, dialDayTicks, colorDialTick, 0.62)
	}

	// Month ticks: 12 longer segments crossing both rails.
	appendDialSegs(m, a, dialMonthTicks, colorDialMonth, 1.05)

	// Month labels — stroke font, curving along the ring.
	emPx := dialEmWorld * z // rough: world em * zoom, but foreshortened at top/bottom; conservative gate uses max.
	// More precise legibility: minimum of horizontal and vertical scale. Use emWorld*z*diskTilt as worst case?
	// Skip only when definitely illegible.
	if emPx >= dialLabelMinPx {
		appendDialSegs(m, a, dialLabelSegs, colorDialText, dialStrokeW)
	}

	// Date marker — small filled triangle at dialAngle, pointing inward.
	theta := dialAngle(a.Time)
	// Normalize to [0, 2pi) without per-cycle looping — a.Time
	// grows without bound and a loop would be O(cycles) per frame.
	theta = float32(math.Mod(float64(theta), 2*math.Pi))
	if theta < 0 {
		theta += 2 * float32(math.Pi)
	}
	markerR := dialOuter
	// Tip slightly inside outer rail, base slightly outside.
	tipWx, tipWy := dialPolar(markerR-6, theta)
	// Base corners offset in angle. Angular width so marker spans ~6° at that radius ~ small.
	angW := float32(0.024) // radians on each side
	baseR := markerR + 10
	bx1, by1 := dialPolar(baseR, theta-angW)
	bx2, by2 := dialPolar(baseR, theta+angW)
	sx0, sy0 := a.worldToScreen(tipWx, tipWy)
	sx1, sy1 := a.worldToScreen(bx1, by1)
	sx2, sy2 := a.worldToScreen(bx2, by2)
	m.appendTri(sx0, sy0, colorDialMarker, sx1, sy1, colorDialMarker, sx2, sy2, colorDialMarker)
	// Also a small dot at outer rail for visibility.
	// (appendTri already consumed; this is the marker geometry.)

	dc.FillTrianglesColors(m.tris, m.cols)
}

// dialDayTicks, dialMonthTicks and dialLabelSegs hold the dial's
// static geometry in world coordinates, four floats per segment
// (x0,y0,x1,y1). All three are pure functions of the calendar and of
// the ring's fixed radii, so the ~2,200 sines and cosines they used to
// cost on every frame are paid once at startup and the frame path is
// left with worldToScreen. The ring itself never moves; only the
// camera does.
//
// Filled from init, not from a var initializer, because two of them
// read monthStartFrac and monthMidAngle — which init fills. Go orders
// var initializers by the references it can see, and it cannot see
// through init, so as initializers these would have run against a
// zeroed calendar. The failure is quiet: every month lands at angle 0,
// the segment *count* is unchanged, and only the pixels move.
var (
	dialDayTicks   []float32
	dialMonthTicks []float32
	dialLabelSegs  []float32
)

// makeDialTicks lays n evenly spaced radial segments from r0 to r1
// around the whole ring.
func makeDialTicks(n int, r0, r1 float32) []float32 {
	dst := make([]float32, 0, n*4)
	for d := range n {
		theta := 2 * float32(math.Pi) * float32(d) / float32(n)
		dst = appendDialTick(dst, r0, r1, theta)
	}
	return dst
}

// makeDialMonthTicks lays one longer segment at each month boundary,
// which is not an even division: the months differ in length.
func makeDialMonthTicks() []float32 {
	dst := make([]float32, 0, 12*4)
	for mi := range 12 {
		theta := 2 * float32(math.Pi) * monthStartFrac[mi]
		dst = appendDialTick(dst, dialInner, dialOuter, theta)
	}
	return dst
}

// appendDialTick appends one radial segment. The two endpoints share
// an angle, so the trigonometry is done once for both.
func appendDialTick(dst []float32, r0, r1, theta float32) []float32 {
	c, s := cos32(theta), sin32(theta)
	return append(dst, r0*c, r0*s, r1*c, r1*s)
}

// makeDialLabelSegs flattens all twelve month names into one segment
// list. They share a color and a stroke width, so nothing at draw time
// needs to know where one label ends and the next begins.
func makeDialLabelSegs() []float32 {
	var dst []float32
	for mi, name := range monthNames {
		dst = appendDialLabel(dst, name, monthMidAngle[mi],
			dialTextR, dialEmWorld)
	}
	return dst
}

// appendDialSegs projects a world-space segment list, four floats per
// segment, and thickens each one in screen space.
func appendDialSegs(m *bodyMesh, a *App, segs []float32,
	c gui.Color, halfW float32,
) {
	for i := 0; i+3 < len(segs); i += 4 {
		sx0, sy0 := a.worldToScreen(segs[i], segs[i+1])
		sx1, sy1 := a.worldToScreen(segs[i+2], segs[i+3])
		appendDialSeg(m, sx0, sy0, sx1, sy1, c, halfW)
	}
}

// dialPolar converts polar in the orbital plane (r, theta) to world
// cartesian, with theta measured from world +x.
func dialPolar(r, theta float32) (wx, wy float32) {
	return r * cos32(theta), r * sin32(theta)
}

// appendDialLabel places a month name centered on thetaMid at radius R,
// appending its stroke segments to dst as world-space endpoint quads.
// A glyph point (gx,gy) maps with gy outward radially and gx to
// angular offset by arc length dθ = x_arc / R. Glyph "up" points
// radially outward everywhere, so labels read from outside.
//
// Every term here is fixed by the label's own angle and radius, so the
// whole placement is done once at init; the frame path only projects
// the result. Each segment is thickened perpendicular in screen space
// by fixed dialStrokeW after projection, so weight stays uniform. The orbital plane is
// foreshortened by diskTilt, so a ring viewed at 29° squashes the
// vertical axis to 48%: a world arc along +y reaches the screen at
// half the length of one along +x. Without compensation a Dec/Jun
// label at east/west (tangent vertical) is 48% of its north/south
// width and letters touch. World extents are inflated by the inverse
// local foreshortening so every label reaches the screen at the same
// em size — ft for the along-ring (tangential) axis and fr for the
// outward radial axis.
//
// Spacing is uniform between glyph *ink* edges, not between em-box
// centers: the stroke font is declared monospace (emAdvance 0.68) but
// its glyphs have varying ink widths (L 0.44, Y 0.58, etc.) and Y's
// stem sits centrally at 0.35, so a fixed center distance leaves
// JULY as "Jul y" with a 0.36 em ink gap before Y vs 0.28 before U.
// Measuring each glyph's min/max X and kerning ink-to-ink makes the
// visual gaps even; the total label shortens by about 0.7 em but stays
// within the framed extent.
func appendDialLabel(dst []float32, name string,
	thetaMid, radius, emW float32,
) []float32 {
	n := len(name)
	if n == 0 {
		return dst
	}
	// Local foreshortening at thetaMid. Tangent is (-sin, cos), radial
	// is (cos, sin); projected length per world unit is sqrt(dx^2 +
	// (dy*diskTilt)^2).
	sinMid, cosMid := sin32(thetaMid), cos32(thetaMid)
	dt := float32(diskTilt)
	ft := sqrt32(sinMid*sinMid + cosMid*cosMid*dt*dt) // tangential
	fr := sqrt32(cosMid*cosMid + sinMid*sinMid*dt*dt) // radial
	if ft < 1e-4 {
		ft = 1e-4
	}
	if fr < 1e-4 {
		fr = 1e-4
	}
	// Ink-based placement: measure each glyph's bounds so the gap
	// between ink edges is a constant 0.14 em (in screen space, hence
	// emW/ft in world). This evens JULY's "L–Y" which otherwise
	// appears as "Jul y" because Y's stem is central.
	type bounds struct{ minX, maxX float32 }
	glyphs := make([]strokeGlyph, 0, n)
	bnds := make([]bounds, 0, n)
	for _, ch := range name {
		g, ok := glyphFor(ch)
		if !ok {
			continue
		}
		mn, mx := glyphBounds(g)
		glyphs = append(glyphs, g)
		bnds = append(bnds, bounds{mn, mx})
	}
	if len(glyphs) == 0 {
		return dst
	}
	gapInk := 0.14 * emW / ft
	// Total ink width along the ring.
	var total float32
	for i, b := range bnds {
		total += (b.maxX - b.minX) * emW / ft
		if i+1 < len(bnds) {
			total += gapInk
		}
	}
	// Position of first ink left edge relative to label center.
	cursor := -total * 0.5
	for i, g := range glyphs {
		b := bnds[i]
		// Origin of this glyph's em box (x=0) relative to label center:
		// ink left = cursor, so origin = cursor - minX*emW/ft.
		origin := cursor - b.minX*emW/ft
		for _, poly := range g {
			for k := 0; k+3 < len(poly); k += 2 {
				x0, y0 := poly[k], poly[k+1]
				x1, y1 := poly[k+2], poly[k+3]
				wx0, wy0 := dialGlyphOriginToWorld(x0, y0, thetaMid, radius, emW, ft, fr, origin)
				wx1, wy1 := dialGlyphOriginToWorld(x1, y1, thetaMid, radius, emW, ft, fr, origin)
				dst = append(dst, wx0, wy0, wx1, wy1)
			}
		}
		// Advance cursor to next ink left edge.
		cursor += (b.maxX - b.minX) * emW / ft
		if i+1 < len(glyphs) {
			cursor += gapInk
		}
	}
	return dst
}

// glyphBounds returns the ink extents of g in em units [0,1].
func glyphBounds(g strokeGlyph) (minX, maxX float32) {
	minX = 1
	maxX = 0
	for _, poly := range g {
		for k := 0; k+1 < len(poly); k += 2 {
			x := poly[k]
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
	}
	if maxX < minX {
		return 0, 0
	}
	return minX, maxX
}

// dialGlyphOriginToWorld maps a glyph point (gx,gy) where the glyph's
// em-box origin sits at arc offset origin from the label center.
func dialGlyphOriginToWorld(gx, gy, thetaMid, radius, emW, ft, fr, origin float32) (wx, wy float32) {
	arc := origin + gx*emW/ft
	theta := thetaMid + arc/radius
	rad := radius + gy*emHeight*emW/fr
	return rad * cos32(theta), rad * sin32(theta)
}

// dialGlyphToWorldComp maps a glyph point (gx,gy) in [0,1] to world
// coords for a glyph centered at thetaCenter, with foreshortening
// compensation so screen size stays uniform with viewing angle.
// ft scales the along-ring axis, fr the outward radial axis.
func dialGlyphToWorldComp(gx, gy, thetaCenter, radius, emW, ft, fr float32) (wx, wy float32) {
	dArc := (gx - 0.5) * emW / ft
	dTheta := dArc / radius
	theta := thetaCenter + dTheta
	rad := radius + gy*emHeight*emW/fr
	return rad * cos32(theta), rad * sin32(theta)
}

// appendDialSeg appends a line segment as a quad (two triangles)
// thickened perpendicular in screen space.
func appendDialSeg(m *bodyMesh, x0, y0, x1, y1 float32, c gui.Color, halfW float32) {
	dx, dy := x1-x0, y1-y0
	l := sqrt32(dx*dx + dy*dy)
	if l < 1e-4 {
		return
	}
	nx, ny := -dy/l*halfW, dx/l*halfW
	// Quad corners.
	ax0, ay0 := x0+nx, y0+ny
	ax1, ay1 := x0-nx, y0-ny
	bx0, by0 := x1+nx, y1+ny
	bx1, by1 := x1-nx, y1-ny
	m.appendTri(ax0, ay0, c, ax1, ay1, c, bx0, by0, c)
	m.appendTri(ax1, ay1, c, bx1, by1, c, bx0, by0, c)
}
