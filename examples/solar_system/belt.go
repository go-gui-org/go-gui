package main

import (
	"math/rand/v2"

	"github.com/go-gui-org/go-gui/gui"
)

const (
	// beltCount is the asteroid count.
	beltCount = 600

	// beltInner/Outer are world-unit radii of the belt's bounds.
	// Between Mars (229) and Jupiter (365).
	beltInner = 250
	beltOuter = 340

	// beltThickness is the half-thickness in world +z units.
	beltThickness = 18

	// beltRockMin/Max are the on-screen half-sizes when drawn: each
	// rock is a 2-triangle square. Kept intentionally small (sub-
	// planet) so the belt reads as dust, not moons.
	beltRockMin = 0.38
	beltRockMax = 0.84
)

// rock is one asteroid in the belt.
type rock struct {
	orbitA, ecc, phase, periodS float32
	zOff                        float32 // out-of-plane offset, world units
	size                        float32
	tint                        float32 // brightness variance
}

// beltRocks is built once with a fixed seed so a run and a test see
// the same belt.
var beltRocks = makeBelt()

func makeBelt() []rock {
	rng := rand.New(rand.NewPCG(0xB314, 0xB01D))
	rocks := make([]rock, 0, beltCount)
	// Guard against a pathological RNG sequence that keeps hitting
	// the Kirkwood gaps: cap total attempts so init cannot spin
	// forever even though construction is probabilistic.
	for attempts := 0; len(rocks) < beltCount && attempts < beltCount*20; attempts++ {
		// Density hump in the middle rather than flat: triangular
		// distribution peaked at center gives higher density there.
		u := rng.Float32()
		v := rng.Float32()
		// Triangular between inner and outer.
		t := (u + v) * 0.5
		a := beltInner + t*(beltOuter-beltInner)

		// Thin Kirkwood-like gaps: skip rocks near two resonances.
		// Places at 1/3 and 2/3 of the range with narrow width.
		if gapFactor(a, beltInner+(beltOuter-beltInner)*0.33) < 0.06 {
			if rng.Float32() < 0.75 {
				continue
			}
		}
		if gapFactor(a, beltInner+(beltOuter-beltInner)*0.68) < 0.06 {
			if rng.Float32() < 0.65 {
				continue
			}
		}

		ecc := rng.Float32() * 0.12
		phase := rng.Float32() * 6.2831855
		periodS := orbitPeriod(a)
		zOff := (rng.Float32()*2 - 1) * beltThickness
		size := beltRockMin + rng.Float32()*(beltRockMax-beltRockMin)
		tint := 0.72 + rng.Float32()*0.38

		rocks = append(rocks, rock{
			orbitA: a, ecc: ecc, phase: phase, periodS: periodS,
			zOff: zOff, size: size, tint: tint,
		})
	}
	return rocks
}

func gapFactor(a, center float32) float32 {
	d := a - center
	if d < 0 {
		d = -d
	}
	return d / (beltOuter - beltInner)
}

// beltColor is the desaturated cyan-grey of the rocks. Dark and
// translucent so the belt is dust rather than a field of moons.
var beltColor = gui.RGBA(92, 108, 112, 148)

// drawBelt paints the asteroid belt as one vertex-colored mesh. Each
// rock is a square. Skips anything off-canvas.
//
// The world +z offset contributes to screen y via the camera basis:
// screen-down = (0, sinE, -cosE), so a world +z point moves down by
// sinE and back by cosE. Only the screen component matters for y, and
// it is -zOff * cosElev * zoom. This matches lightVecAt's basis.
func drawBelt(a *App, dc *gui.DrawContext) {
	m := &a.belt
	m.tris, m.cols = m.tris[:0], m.cols[:0]

	z := a.zoom()
	cosE := cosElev

	for i := range beltRocks {
		r := &beltRocks[i]
		wx, wy := ellipsePos(r.orbitA, r.ecc, r.phase, r.periodS, a.Time)
		// World z contributes to screen.
		sx, sy := a.worldToScreen(wx, wy)
		// Apply out-of-plane offset: world (0,0,zOff) -> screen
		// delta (0, -zOff*cosE*zoom) in y. No x component because
		// world z's projection onto R is zero.
		sy += -r.zOff * cosE * z

		half := r.size
		// Cull: include rock fully vs canvas bounds with margin.
		if sx+half < 0 || sx-half > dc.Width || sy+half < 0 || sy-half > dc.Height {
			continue
		}
		c := beltColor
		// Per-rock brightness variance.
		c = gui.RGBA(
			chan8(float32(c.R)*r.tint),
			chan8(float32(c.G)*r.tint),
			chan8(float32(c.B)*r.tint),
			c.A,
		)
		x0, y0 := sx-half, sy-half
		x1, y1 := sx+half, sy+half
		m.appendTri(x0, y0, c, x1, y0, c, x1, y1, c)
		m.appendTri(x0, y0, c, x1, y1, c, x0, y1, c)
	}
	dc.FillTrianglesColors(m.tris, m.cols)
}
