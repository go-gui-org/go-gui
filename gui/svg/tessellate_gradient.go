package svg

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Gradient support ---

func resolveGradient(g gui.SvgGradientDef, minX, minY, maxX, maxY float32) gui.SvgGradientDef {
	w := maxX - minX
	h := maxY - minY
	if g.IsRadial {
		// OBB → user space mapping. Spec maps the OBB to a 1×1
		// square then transforms back, which can yield elliptical
		// gradients. Approximation: scale R uniformly by the average
		// of width and height. For square viewBoxes (most icon use)
		// this is exact; for wide/tall bboxes the gradient stays
		// circular rather than stretching to an ellipse.
		avg := (w + h) * 0.5
		return gui.SvgGradientDef{
			Stops:         g.Stops,
			CX:            minX + g.CX*w,
			CY:            minY + g.CY*h,
			R:             g.R * avg,
			FX:            minX + g.FX*w,
			FY:            minY + g.FY*h,
			IsRadial:      true,
			GradientUnits: "userSpaceOnUse",
		}
	}
	return gui.SvgGradientDef{
		Stops:         g.Stops,
		X1:            minX + g.X1*w,
		Y1:            minY + g.Y1*h,
		X2:            minX + g.X2*w,
		Y2:            minY + g.Y2*h,
		GradientUnits: "userSpaceOnUse",
	}
}

func bboxFromTriangles(tris []float32) (float32, float32, float32, float32) {
	if len(tris) < 2 {
		return 0, 0, 0, 0
	}
	minX, minY := tris[0], tris[1]
	maxX, maxY := minX, minY
	for i := 2; i < len(tris); i += 2 {
		x, y := tris[i], tris[i+1]
		minX = min(minX, x)
		maxX = max(maxX, x)
		minY = min(minY, y)
		maxY = max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

func projectOntoGradient(vx, vy float32, g gui.SvgGradientDef) float32 {
	if g.IsRadial {
		return projectOntoRadial(vx, vy, g)
	}
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return 0
	}
	t := ((vx-g.X1)*dx + (vy-g.Y1)*dy) / lenSq
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// projectAndSpread projects (vx, vy) onto g without clamping to [0,1]
// then applies g.SpreadMethod. With pad (default) the clamp matches
// projectOntoGradient's historic behavior; reflect mirrors and
// repeat wraps for t outside [0,1].
func projectAndSpread(vx, vy float32, g gui.SvgGradientDef) float32 {
	t := projectOntoGradientRaw(vx, vy, g)
	return applySpread(t, g.SpreadMethod)
}

func projectOntoGradientRaw(vx, vy float32, g gui.SvgGradientDef) float32 {
	if g.IsRadial {
		r64 := float64(g.R)
		if g.R <= 0 || math.IsNaN(r64) || math.IsInf(r64, 0) {
			return 0
		}
		dx := vx - g.FX
		dy := vy - g.FY
		d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		t := d / g.R
		if t != t {
			return 0
		}
		return t
	}
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return 0
	}
	return ((vx-g.X1)*dx + (vy-g.Y1)*dy) / lenSq
}

// applySpread maps raw gradient parameter t through SpreadMethod.
// Pad clamps to [0,1]; reflect produces a triangle wave; repeat
// produces a sawtooth. NaN/Inf coerced to 0.
func applySpread(t float32, spread gui.SvgGradientSpread) float32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) {
		return 0
	}
	// Clamp to a safe int64-convertible range so math.Floor's int64
	// cast for reflect parity cannot hit implementation-defined
	// overflow on hostile inputs. ±2^31 covers any plausible
	// gradient projection by many orders of magnitude.
	const spreadLimit = float64(1 << 31)
	if t64 > spreadLimit {
		t64 = spreadLimit
	} else if t64 < -spreadLimit {
		t64 = -spreadLimit
	}
	switch spread {
	case gui.SvgSpreadReflect:
		n := math.Floor(t64)
		frac := float32(t64 - n)
		if int64(n)&1 != 0 {
			return 1 - frac
		}
		return frac
	case gui.SvgSpreadRepeat:
		n := math.Floor(t64)
		return float32(t64 - n)
	}
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// projectOntoRadial computes gradient parameter t for a radial
// gradient at vertex (vx, vy). Simplified implementation: distance
// from focal point divided by R, clamped to [0,1]. Full spec maps
// the focal-to-edge vector through a cone, which produces subtly
// different falloff when fx,fy != cx,cy. Tracked as future polish.
func projectOntoRadial(vx, vy float32, g gui.SvgGradientDef) float32 {
	r64 := float64(g.R)
	if g.R <= 0 || math.IsNaN(r64) || math.IsInf(r64, 0) {
		return 0
	}
	dx := vx - g.FX
	dy := vy - g.FY
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	t := d / g.R
	if t != t { // NaN
		return 0
	}
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

func interpolateGradient(stops []gui.SvgGradientStop, t float32) gui.SvgColor {
	if len(stops) == 0 {
		return gui.SvgColor{A: 255}
	}
	if t <= stops[0].Offset || len(stops) == 1 {
		return stops[0].Color
	}
	last := stops[len(stops)-1]
	if t >= last.Offset {
		return last.Color
	}
	for i := 0; i < len(stops)-1; i++ {
		s0 := stops[i]
		s1 := stops[i+1]
		if t >= s0.Offset && t <= s1.Offset {
			r := s1.Offset - s0.Offset
			if r <= 0 {
				return s0.Color
			}
			f := (t - s0.Offset) / r
			return gui.SvgColor{
				R: uint8(float32(s0.Color.R) + (float32(s1.Color.R)-float32(s0.Color.R))*f),
				G: uint8(float32(s0.Color.G) + (float32(s1.Color.G)-float32(s0.Color.G))*f),
				B: uint8(float32(s0.Color.B) + (float32(s1.Color.B)-float32(s0.Color.B))*f),
				A: uint8(float32(s0.Color.A) + (float32(s1.Color.A)-float32(s0.Color.A))*f),
			}
		}
	}
	return last.Color
}

func subdivideGradientTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	if grad.IsRadial {
		return subdivideRadialTris(tris, grad)
	}
	if len(grad.Stops) <= 2 {
		return tris
	}
	stopTs := make([]float32, 0, len(grad.Stops))
	for _, s := range grad.Stops {
		if s.Offset > 0.001 && s.Offset < 0.999 {
			stopTs = append(stopTs, s.Offset)
		}
	}
	if len(stopTs) == 0 {
		return tris
	}
	result := make([]float32, 0, len(tris)*2)
	for i := 0; i < len(tris)-5; i += 6 {
		splitTriAtStops(tris[i], tris[i+1], tris[i+2], tris[i+3],
			tris[i+4], tris[i+5], grad, stopTs, 0, &result)
	}
	return result
}

// subdivideRadialTris recursively splits triangles whose edges span
// more than ~1/24 of the gradient radius. Per-vertex sampling on
// long edges interpolates linearly across the triangle, which
// linearizes the radial falloff and produces a flat appearance for
// large primitives (e.g. a 100×100 rect filled by 2 triangles).
// Smaller triangles approximate the circular iso-t lines closely
// enough that vertex coloring reads as a smooth radial gradient.
func subdivideRadialTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	// Guard against non-finite R (NaN survives the < comparison and
	// would defeat the depth-cap heuristic, forcing every source
	// triangle to recurse to maxRadialDepth = 4096 splits).
	r64 := float64(grad.R)
	if math.IsNaN(r64) || math.IsInf(r64, 0) || grad.R <= 0 {
		return tris
	}
	target := grad.R / 24
	if target < 1e-3 {
		target = 1e-3
	}
	result := make([]float32, 0, len(tris)*4)
	for i := 0; i+5 < len(tris); i += 6 {
		splitRadialTri(tris[i], tris[i+1], tris[i+2], tris[i+3],
			tris[i+4], tris[i+5], target, 0, &result)
	}
	return result
}

func splitRadialTri(ax, ay, bx, by, cx, cy float32,
	target float32, depth int, result *[]float32) {
	const maxRadialDepth = 6
	abx := bx - ax
	aby := by - ay
	bcx := cx - bx
	bcy := cy - by
	cax := ax - cx
	cay := ay - cy
	maxLenSq := max(abx*abx+aby*aby,
		max(bcx*bcx+bcy*bcy, cax*cax+cay*cay))
	if depth >= maxRadialDepth || maxLenSq <= target*target {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	mabx, maby := (ax+bx)*0.5, (ay+by)*0.5
	mbcx, mbcy := (bx+cx)*0.5, (by+cy)*0.5
	mcax, mcay := (cx+ax)*0.5, (cy+ay)*0.5
	splitRadialTri(ax, ay, mabx, maby, mcax, mcay,
		target, depth+1, result)
	splitRadialTri(mabx, maby, bx, by, mbcx, mbcy,
		target, depth+1, result)
	splitRadialTri(mcax, mcay, mbcx, mbcy, cx, cy,
		target, depth+1, result)
	splitRadialTri(mabx, maby, mbcx, mbcy, mcax, mcay,
		target, depth+1, result)
}

func splitTriAtStops(ax, ay, bx, by, cx, cy float32, grad gui.SvgGradientDef, stopTs []float32, depth int, result *[]float32) {
	if depth >= maxSplitTriDepth {
		*result = append(*result, ax, ay, bx, by, cx, cy)
		return
	}
	ta := projectOntoGradient(ax, ay, grad)
	tb := projectOntoGradient(bx, by, grad)
	tc := projectOntoGradient(cx, cy, grad)

	tMin := ta
	tMin = min(tMin, tb, tc)
	tMax := ta
	tMax = max(tMax, tb, tc)

	for _, tS := range stopTs {
		if tS > tMin+1e-4 && tS < tMax-1e-4 {
			// Sort vertices by t
			p0x, p0y, t0 := ax, ay, ta
			p1x, p1y, t1 := bx, by, tb
			p2x, p2y, t2 := cx, cy, tc
			if t0 > t1 {
				p0x, p0y, t0, p1x, p1y, t1 = p1x, p1y, t1, p0x, p0y, t0
			}
			if t1 > t2 {
				p1x, p1y, t1, p2x, p2y, t2 = p2x, p2y, t2, p1x, p1y, t1
			}
			if t0 > t1 {
				p0x, p0y, t0, p1x, p1y, t1 = p1x, p1y, t1, p0x, p0y, t0
			}

			f02 := float32(0.5)
			if t2-t0 > 1e-6 {
				f02 = (tS - t0) / (t2 - t0)
			}
			i1x := p0x + f02*(p2x-p0x)
			i1y := p0y + f02*(p2y-p0y)

			if tS < t1-1e-4 {
				f01 := float32(0.5)
				if t1-t0 > 1e-6 {
					f01 = (tS - t0) / (t1 - t0)
				}
				i2x := p0x + f01*(p1x-p0x)
				i2y := p0y + f01*(p1y-p0y)
				splitTriAtStops(p0x, p0y, i2x, i2y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(i2x, i2y, p1x, p1y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(p1x, p1y, p2x, p2y, i1x, i1y, grad, stopTs, depth+1, result)
			} else if tS > t1+1e-4 {
				f12 := float32(0.5)
				if t2-t1 > 1e-6 {
					f12 = (tS - t1) / (t2 - t1)
				}
				i2x := p1x + f12*(p2x-p1x)
				i2y := p1y + f12*(p2y-p1y)
				splitTriAtStops(p0x, p0y, p1x, p1y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(p1x, p1y, i2x, i2y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(i1x, i1y, i2x, i2y, p2x, p2y, grad, stopTs, depth+1, result)
			} else {
				splitTriAtStops(p0x, p0y, p1x, p1y, i1x, i1y, grad, stopTs, depth+1, result)
				splitTriAtStops(p1x, p1y, p2x, p2y, i1x, i1y, grad, stopTs, depth+1, result)
			}
			return
		}
	}
	*result = append(*result, ax, ay, bx, by, cx, cy)
}
