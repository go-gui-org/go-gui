package gui

import "github.com/go-gui-org/go-gui/gui/internal/gradmesh"

// canvas_gradient.go — gradient fills for DrawContext.
//
// A canvas gradient is baked on the CPU into per-vertex colors on the
// batch's triangles, riding the existing RenderSvg command's
// VertexColors channel. Every backend already consumes that channel
// (metal, gl, web, soft, ios, android), and so does PDF export, so a
// canvas gradient needs no shader and no backend change.
//
// The projection and subdivision live in gui/internal/gradmesh, shared
// with gui/svg's path gradients. What stays here is this side's own
// vocabulary: the exported gradient type, the defaulting rules, and the
// conversion into gradmesh's neutral parameters.

// GradientSpread selects how a canvas gradient handles parameter
// values outside [0,1]. Pad clamps (the zero value), Reflect mirrors,
// Repeat wraps.
// exportaudit:keep — the enum a CanvasGradient's Spread field takes
type GradientSpread uint8

// GradientSpread values.
const (
	// exportaudit:keep — the zero value, named for callers that set it
	// explicitly rather than relying on it
	SpreadPad GradientSpread = iota
	// exportaudit:keep — a member of an exported enum
	SpreadReflect
	SpreadRepeat
)

// CanvasGradient describes a gradient fill for DrawContext, in the
// canvas's own coordinate space (the same coordinates the drawing
// methods take).
//
// Geometry left degenerate is derived from the bounds of the geometry
// being filled, so the minimum useful value is a Stops list:
//
//   - a linear gradient whose endpoints coincide runs top-to-bottom
//     across the fill's bounding box
//   - a radial gradient with R <= 0 is centered on the fill's bounding
//     box with R set to half its larger extent
//
// FX/FY default to CX/CY when both are zero, matching SVG's default
// for fx/fy. A focal point genuinely at the origin, with a center
// elsewhere, must be nudged off it.
//
// Stops need not be sorted or in range; they are normalized on use.
// There is no stop-count limit — the GPU shader's five-stop cap
// applies to the uniform-packed shape gradient path, not to this one.
type CanvasGradient struct {
	Stops     []GradientStop
	X1, Y1    float32 // linear: start point
	X2, Y2    float32 // linear: end point
	CX, CY, R float32 // radial: center and radius
	FX, FY    float32 // radial: focal point
	Radial    bool
	Spread    GradientSpread
}

// triBounds returns the bounding box of a flat x,y triangle list.
func triBounds(tris []float32) (minX, minY, maxX, maxY float32) {
	if len(tris) < 2 {
		return 0, 0, 0, 0
	}
	minX, minY = tris[0], tris[1]
	maxX, maxY = minX, minY
	for i := 2; i+1 < len(tris); i += 2 {
		x, y := tris[i], tris[i+1]
		minX = min(minX, x)
		maxX = max(maxX, x)
		minY = min(minY, y)
		maxY = max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

// resolveCanvasGradient fills in geometry the caller left degenerate
// from the bounds of the geometry being filled. See CanvasGradient for
// the defaulting rules.
func resolveCanvasGradient(g CanvasGradient,
	minX, minY, maxX, maxY float32) CanvasGradient {
	if g.Radial {
		if !(g.R > 0) { // negated > also rejects NaN
			g.CX = (minX + maxX) * 0.5
			g.CY = (minY + maxY) * 0.5
			g.R = max(maxX-minX, maxY-minY) * 0.5
			g.FX, g.FY = 0, 0
		}
		if g.FX == 0 && g.FY == 0 {
			g.FX, g.FY = g.CX, g.CY
		}
		return g
	}
	if g.X1 == g.X2 && g.Y1 == g.Y2 {
		// Top-to-bottom, matching the CSS default direction.
		g.X1, g.Y1 = (minX+maxX)*0.5, minY
		g.X2, g.Y2 = (minX+maxX)*0.5, maxY
	}
	return g
}

// gradSpread maps this side's spread enum onto gradmesh's. The two
// happen to agree numerically; the switch is what keeps them free to
// stop agreeing, and gives an out-of-range value the pad behaviour the
// coloring pass assumes.
func gradSpread(s GradientSpread) gradmesh.Spread {
	switch s {
	case SpreadReflect:
		return gradmesh.SpreadReflect
	case SpreadRepeat:
		return gradmesh.SpreadRepeat
	}
	return gradmesh.SpreadPad
}

// gradParams converts a resolved CanvasGradient into the neutral
// parameters gradmesh tessellates against. Only the stops' positions
// cross the boundary; the colors stay here for the coloring pass.
//
// offsets is caller scratch, so building the parameters for a fill
// allocates nothing after the first frame.
func gradParams(g *CanvasGradient, offsets *[]float32) gradmesh.Params {
	*offsets = (*offsets)[:0]
	for _, s := range g.Stops {
		*offsets = append(*offsets, s.Pos)
	}
	return gradmesh.Params{
		StopOffsets: *offsets,
		X1:          g.X1,
		Y1:          g.Y1,
		X2:          g.X2,
		Y2:          g.Y2,
		CX:          g.CX,
		CY:          g.CY,
		R:           g.R,
		FX:          g.FX,
		FY:          g.FY,
		Radial:      g.Radial,
		Spread:      gradSpread(g.Spread),
	}
}
