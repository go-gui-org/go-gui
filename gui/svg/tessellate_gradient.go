package svg

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/gradmesh"
)

// tessellate_gradient.go — gradient fills for SVG paths.
//
// The projection and subdivision live in gui/internal/gradmesh, shared
// with gui's DrawContext gradients. What stays here is this side's own
// vocabulary: resolving objectBoundingBox units, converting an
// SvgGradientDef into gradmesh's neutral parameters, and the coloring
// pass over SvgColor stops.
//
// Buffer policy stays here too, and it is the opposite of the canvas
// side's: tessellate.go retains the returned slice in
// TessellatedPath.Triangles for the lifetime of the cached asset, so
// every call hands gradmesh fresh buffers rather than shared scratch.

// --- Gradient support ---

// resolveGradient rewrites an objectBoundingBox gradient into user
// space against the geometry's bounds. Everything the def carries that
// is not geometry — the stops and the spread method — passes through
// untouched; dropping the spread here would silently pad every gradient
// that uses the default units, which is most of them.
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
			SpreadMethod:  g.SpreadMethod,
			GradientUnits: "userSpaceOnUse",
		}
	}
	return gui.SvgGradientDef{
		Stops:         g.Stops,
		X1:            minX + g.X1*w,
		Y1:            minY + g.Y1*h,
		X2:            minX + g.X2*w,
		Y2:            minY + g.Y2*h,
		SpreadMethod:  g.SpreadMethod,
		GradientUnits: "userSpaceOnUse",
	}
}

// gradSpread maps SVG's spread enum onto gradmesh's. Anything that is
// not reflect or repeat pads, which is what the spec's default and any
// value the parser failed to recognize both want.
func gradSpread(s gui.SvgGradientSpread) gradmesh.Spread {
	switch s {
	case gui.SvgSpreadReflect:
		return gradmesh.SpreadReflect
	case gui.SvgSpreadRepeat:
		return gradmesh.SpreadRepeat
	}
	return gradmesh.SpreadPad
}

// gradParams converts a resolved SvgGradientDef into the neutral
// parameters gradmesh tessellates against. Only the stops' offsets
// cross the boundary; their colors stay here for the coloring pass.
//
// offsets is the caller's destination for those offsets, and may be nil
// when the parameters are only used to project a point.
func gradParams(g *gui.SvgGradientDef, offsets *[]float32) gradmesh.Params {
	var stopOffsets []float32
	if offsets != nil {
		*offsets = (*offsets)[:0]
		for _, s := range g.Stops {
			*offsets = append(*offsets, s.Offset)
		}
		stopOffsets = *offsets
	}
	return gradmesh.Params{
		StopOffsets: stopOffsets,
		X1:          g.X1,
		Y1:          g.Y1,
		X2:          g.X2,
		Y2:          g.Y2,
		CX:          g.CX,
		CY:          g.CY,
		R:           g.R,
		FX:          g.FX,
		FY:          g.FY,
		Radial:      g.IsRadial,
		Spread:      gradSpread(g.SpreadMethod),
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
	p := gradParams(&g, nil)
	return gradmesh.ApplySpread(gradmesh.RawT(vx, vy, &p), p.Spread)
}

// applySpread maps raw gradient parameter t through SpreadMethod.
// Pad clamps to [0,1]; reflect produces a triangle wave; repeat
// produces a sawtooth. NaN/Inf coerced to 0.
func applySpread(t float32, spread gui.SvgGradientSpread) float32 {
	return gradmesh.ApplySpread(t, gradSpread(spread))
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

// subdivideGradientTris splits tris so per-vertex coloring can
// represent the gradient. Returns tris unchanged when no split is
// needed.
//
// Every buffer it hands gradmesh is freshly allocated, because the
// result outlives the call: tessellate.go stores it in the cached
// asset's TessellatedPath.
func subdivideGradientTris(tris []float32, grad gui.SvgGradientDef) []float32 {
	if len(tris) < 6 {
		return nil
	}
	var offsets []float32
	p := gradParams(&grad, &offsets)
	split := make([]float32, 0, len(tris)*2)
	var radial, isolines []float32
	return gradmesh.Subdivide(tris, &p, &split, &radial, &isolines)
}
