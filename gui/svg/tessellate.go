package svg

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// getTriangles tessellates all paths in the graphic into GPU-ready
// triangle geometry. Returns triangles in viewBox coordinate space.
func (vg *VectorGraphic) getTriangles(scale float32) []gui.TessellatedPath {
	return vg.tessellatePaths(vg.Paths, scale)
}

// clampStrokeWidthForScale widens sub-pixel stroke widths to ~1 device
// pixel (in viewBox units) so AA dropouts don't erase the line at small
// render scales. Subnormal/tiny scales are floored at 1e-6 to keep
// 1/scale finite.
func clampStrokeWidthForScale(w, scale float32) float32 {
	if scale <= 0 {
		return w
	}
	const minScale = 1e-6
	if scale < minScale {
		scale = minScale
	}
	minVB := float32(1.0 / scale)
	if w < minVB {
		return minVB
	}
	return w
}

// tessellatePaths tessellates an arbitrary set of VectorPaths,
// using the VectorGraphic's clip paths and gradients.
func (vg *VectorGraphic) tessellatePaths(paths []VectorPath, scale float32) []gui.TessellatedPath {
	result := make([]gui.TessellatedPath, 0, len(paths)*2)

	baseTol := 0.5 / scale
	tolerance := baseTol
	floor := float32(0.15)
	if vg.FlatnessTolerance > 0 {
		floor = vg.FlatnessTolerance
	}
	if tolerance < floor {
		tolerance = floor
	}

	clipGroupCounter := 0
	// Cache tessellated clip-mask triangles per ClipPathID for this
	// call. tolerance is constant for the duration so caching by ID is
	// safe; without this, N paths sharing one complex clipPath force
	// N full re-tessellations (O(N * clipComplexity) DoS). nil when
	// no clipPaths exist (most icons/spinners) — appendClipMasks
	// nil-guards both read and write.
	clipTriCache := newClipTriCache(len(vg.ClipPaths))

	for i := range paths {
		path := &paths[i]

		clipGroup := 0
		if path.ClipPathID != "" {
			result, clipGroup = vg.appendClipMasks(result, path,
				tolerance, &clipGroupCounter, clipTriCache)
		}

		seed, bake := seedFromTransform(path)
		seed.ClipGroup = clipGroup
		seed.PathID = path.PathID
		// BaseRotCX/CY already set by seedFromTransform when
		// applicable; preserve through the copy here.
		seed.Animated = path.Animated
		seed.Primitive = path.Primitive
		polylines := flattenPathWithBake(path, tolerance, bake)

		emittedGeometry := false

		// Fill tessellation
		hasGradient := path.FillGradientID != ""
		if path.FillColor.A > 0 || hasGradient {
			rawTris := tessellatePolylines(polylines, path.FillRule)
			if len(rawTris) > 0 {
				if hasGradient {
					if g, ok := vg.Gradients[path.FillGradientID]; ok {
						grad := g
						if g.GradientUnits == "objectBoundingBox" || g.GradientUnits == "" {
							bx0, by0, bx1, by1 := bboxFromTriangles(rawTris)
							grad = resolveGradient(g, bx0, by0, bx1, by1)
						}
						fillTris := subdivideGradientTris(rawTris, grad)
						nVerts := len(fillTris) / 2
						vcols := make([]gui.SvgColor, nVerts)
						opacity := path.Opacity * path.FillOpacity
						for vi := range nVerts {
							vx := fillTris[vi*2]
							vy := fillTris[vi*2+1]
							t := projectAndSpread(vx, vy, grad)
							c := interpolateGradient(grad.Stops, t)
							if opacity < 1.0 {
								c = applyOpacity(c, opacity)
							}
							vcols[vi] = c
						}
						out := seed
						out.Triangles = fillTris
						out.Color = path.FillColor
						out.VertexColors = vcols
						result = append(result, out)
						emittedGeometry = true
					}
				} else {
					out := seed
					out.Triangles = rawTris
					out.Color = path.FillColor
					result = append(result, out)
					emittedGeometry = true
				}
			}
		}

		// Stroke tessellation. Stroke width stays in viewBox units
		// here — render-side vertex scaling applies once. Strokes
		// that would render sub-pixel on screen are widened to ~1px
		// in viewBox units so AA dropouts don't erase the line at
		// small render scales (e.g. spinner.svg viewBox 200 at
		// scale=0.03 gives a 0.18px hairline that effectively
		// disappears).
		hasStrokeGrad := path.StrokeGradientID != ""
		if (path.StrokeColor.A > 0 || hasStrokeGrad) && path.StrokeWidth > 0 &&
			finiteF32(path.StrokeWidth) {
			strokeWidth := clampStrokeWidthForScale(path.StrokeWidth, scale)
			strokePoly := polylines
			if len(path.StrokeDasharray) > 0 {
				strokePoly = applyDasharray(polylines,
					path.StrokeDasharray, path.StrokeDashOffset)
			}
			rawStroke := tessellateStroke(strokePoly, strokeWidth, path.StrokeCap, path.StrokeJoin)
			if len(rawStroke) > 0 {
				if hasStrokeGrad {
					if g, ok := vg.Gradients[path.StrokeGradientID]; ok {
						grad := g
						if g.GradientUnits == "objectBoundingBox" || g.GradientUnits == "" {
							bx0, by0, bx1, by1 := bboxFromTriangles(rawStroke)
							grad = resolveGradient(g, bx0, by0, bx1, by1)
						}
						sTris := subdivideGradientTris(rawStroke, grad)
						nVerts := len(sTris) / 2
						vcols := make([]gui.SvgColor, nVerts)
						opacity := path.Opacity * path.StrokeOpacity
						for vi := range nVerts {
							vx := sTris[vi*2]
							vy := sTris[vi*2+1]
							t := projectAndSpread(vx, vy, grad)
							c := interpolateGradient(grad.Stops, t)
							if opacity < 1.0 {
								c = applyOpacity(c, opacity)
							}
							vcols[vi] = c
						}
						out := seed
						out.Triangles = sTris
						out.Color = path.StrokeColor
						out.VertexColors = vcols
						out.IsStroke = true
						result = append(result, out)
						emittedGeometry = true
					}
				} else {
					out := seed
					out.Triangles = rawStroke
					out.Color = path.StrokeColor
					out.IsStroke = true
					result = append(result, out)
					emittedGeometry = true
				}
			}
		}

		if !emittedGeometry && path.Animated &&
			path.Primitive.Kind != gui.SvgPrimNone {
			result = appendDegeneratePlaceholders(result, path, seed)
		}
	}
	for i := range result {
		result[i].MinX, result[i].MinY,
			result[i].MaxX, result[i].MaxY =
			bboxFromTriangles(result[i].Triangles)
	}
	return result
}

// seedFromTransform decides whether path.Transform is deferred to
// render-time Base* composition or baked into vertex coords.
// Per-path animation routing (each path gets its own svgAnimState
// keyed by PathID) means sibling paths cannot collide on one base,
// so every TRS-decomposable transform defers. Non-decomposable
// (shear) matrices bake into vertex coords.
//
// When the decomposed rotation is non-zero, the translate column of
// the matrix is absorbed into a rotation pivot (BaseRotCX/CY) with
// BaseTransX/Y=0. This preserves the semantic separation between
// translation and rotation so a SMIL replace-rotate animation can
// overwrite the rotation component alone without disturbing a
// separate translate.
func seedFromTransform(
	path *VectorPath,
) (gui.TessellatedPath, bool) {
	var seed gui.TessellatedPath
	if isIdentityTransform(path.Transform) {
		return seed, false
	}
	tx, ty, sx, sy, rot, ok := decomposeTRS(path.Transform)
	if !ok {
		return seed, true
	}
	seed.BaseScaleX, seed.BaseScaleY = sx, sy
	seed.BaseRotAngle = rot
	seed.HasBaseXform = true
	if rot != 0 {
		// Solve (rcx, rcy) from the rotate-about-pivot identity:
		//   e = rcx*(1-cos) + rcy*sin*sy/sx
		//   f = -rcx*sin*sx/sy + rcy*(1-cos)
		// With uniform (or separable) scale the off-diagonal terms
		// vanish to sin/-sin; solve the 2x2 linear system. Non-zero
		// rotation guarantees det = 2*(1-cos) != 0.
		rcx, rcy, piv := pivotFromTrans(tx, ty, rot)
		if piv {
			seed.BaseRotCX = rcx
			seed.BaseRotCY = rcy
			seed.BaseTransX = 0
			seed.BaseTransY = 0
			return seed, false
		}
	}
	seed.BaseTransX, seed.BaseTransY = tx, ty
	return seed, false
}

// pivotFromTrans solves for the rotation pivot (rcx, rcy) that makes
// R_(rcx,rcy)(v) equivalent to the decomposed trans+rot. Returns
// ok=false for near-identity rotations where the pivot is
// numerically unstable.
func pivotFromTrans(tx, ty, rotDeg float32) (float32, float32, bool) {
	rad := float64(rotDeg) * math.Pi / 180
	cosA := float32(math.Cos(rad))
	sinA := float32(math.Sin(rad))
	det := 2 * (1 - cosA)
	if det > -1e-5 && det < 1e-5 {
		return 0, 0, false
	}
	// Solve [[1-cos, sin], [-sin, 1-cos]] * [rcx, rcy] = [tx, ty].
	rcx := ((1-cosA)*tx - sinA*ty) / det
	rcy := (sinA*tx + (1-cosA)*ty) / det
	return rcx, rcy, true
}

// newClipTriCache returns a presized clipPath triangle cache, or nil
// if the graphic declares no clipPaths so callers skip the alloc.
func newClipTriCache(nClipPaths int) map[string][][]float32 {
	if nClipPaths == 0 {
		return nil
	}
	return make(map[string][][]float32, nClipPaths)
}

// appendClipMasks emits per-subpath clip-mask TessellatedPaths for
// the path's referenced clipPath. Bumps counter and returns the
// assigned clipGroup so subsequent fill/stroke entries inherit it.
func (vg *VectorGraphic) appendClipMasks(result []gui.TessellatedPath,
	path *VectorPath, tolerance float32, counter *int,
	cache map[string][][]float32,
) ([]gui.TessellatedPath, int) {
	clipGeom, ok := vg.ClipPaths[path.ClipPathID]
	if !ok {
		return result, 0
	}
	*counter++
	clipGroup := *counter
	var cached [][]float32
	hit := false
	if cache != nil {
		cached, hit = cache[path.ClipPathID]
	}
	if !hit {
		cached = make([][]float32, 0, len(clipGeom))
		for j := range clipGeom {
			cpPoly := flattenPath(&clipGeom[j], tolerance)
			clipTris := tessellatePolylines(cpPoly, FillRuleNonzero)
			if len(clipTris) == 0 {
				continue
			}
			cached = append(cached, clipTris)
		}
		if cache != nil {
			cache[path.ClipPathID] = cached
		}
	}
	for _, clipTris := range cached {
		result = append(result, gui.TessellatedPath{
			Triangles:  clipTris,
			Color:      gui.SvgColor{R: 255, G: 255, B: 255, A: 255},
			IsClipMask: true,
			ClipGroup:  clipGroup,
			PathID:     path.PathID,
		})
	}
	return result, clipGroup
}

// appendDegeneratePlaceholders emits zero-triangle TessellatedPath
// entries for an Animated primitive that produced no static geometry
// (e.g. <circle r="0"> animating r). One placeholder per configured
// paint (fill / stroke) keeps span counts matching the live result
// from TessellateAnimated; without these the spinner is invisible.
// seed carries clip/group/primitive/base-transform state shared with
// the concrete fill/stroke emissions.
func appendDegeneratePlaceholders(result []gui.TessellatedPath,
	path *VectorPath, seed gui.TessellatedPath,
) []gui.TessellatedPath {
	wantsFill := path.FillColor.A > 0 || path.FillGradientID != ""
	wantsStroke := (path.StrokeColor.A > 0 ||
		path.StrokeGradientID != "") && path.StrokeWidth > 0
	if !wantsFill && !wantsStroke {
		wantsFill = true // ensure at least one placeholder
	}
	seed.Animated = true
	if wantsFill {
		out := seed
		out.Color = path.FillColor
		result = append(result, out)
	}
	if wantsStroke {
		out := seed
		out.Color = path.StrokeColor
		out.IsStroke = true
		result = append(result, out)
	}
	return result
}

// minDashCycleLen bounds the smallest accepted dasharray cycle.
// Sub-threshold cycles would force the inner consume loop to iterate
// segLen / cycleLen times — hostile or buggy authors with cycles
// near float32 epsilon could DoS the tessellator. ~thousandth of a
// pixel is finer than any real renderer needs.
const minDashCycleLen = float32(1e-3)

// maxDashIterPerPoly caps the inner dash-consume loop per polyline
// so a finite-but-pathological dasharray (extremely small relative
// to segment length) cannot stall tessellation.
const maxDashIterPerPoly = 1 << 20

// applyDasharray splits polylines into dash segments. offset is the
// SVG stroke-dashoffset in viewBox units: positive advances the dash
// phase forward (first dash starts later); negative wraps backward.
func applyDasharray(polylines [][]float32, dasharray []float32,
	offset float32) [][]float32 {
	if len(dasharray) == 0 {
		return polylines
	}
	// All-zero / non-finite dasharray: per SVG spec, treat as solid.
	// Also guards the inner loop, where remaining=0 never advances.
	cycleLen := float32(0)
	for _, v := range dasharray {
		cycleLen += v
	}
	if cycleLen < minDashCycleLen ||
		math.IsInf(float64(cycleLen), 0) ||
		math.IsNaN(float64(cycleLen)) {
		return polylines
	}
	startIdx, startDrawing, startRemaining := dashPhase(dasharray, offset, cycleLen)
	result := make([][]float32, 0, len(polylines)*2)
	for _, poly := range polylines {
		if len(poly) < 4 {
			continue
		}
		dashIdx := startIdx
		drawing := startDrawing
		remaining := startRemaining
		// Emitted sub-slices use cap=len to block future appends
		// from stomping retained segments.
		arena := make([]float32, 0, len(poly)*2)
		segStart := 0
		px, py := poly[0], poly[1]
		if drawing {
			arena = append(arena, px, py)
		}
		iter := 0
	walkPoly:
		for i := 2; i < len(poly); i += 2 {
			nx, ny := poly[i], poly[i+1]
			dx, dy := nx-px, ny-py
			segLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if segLen < 1e-6 {
				continue
			}
			consumed := float32(0)
			for consumed < segLen-1e-6 {
				if iter++; iter > maxDashIterPerPoly {
					break walkPoly
				}
				avail := segLen - consumed
				if remaining <= avail {
					t := (consumed + remaining) / segLen
					ix := px + t*dx
					iy := py + t*dy
					if drawing {
						arena = append(arena, ix, iy)
						if len(arena)-segStart >= 4 {
							end := len(arena)
							result = append(result,
								arena[segStart:end:end])
						}
						segStart = len(arena)
					} else {
						segStart = len(arena)
						arena = append(arena, ix, iy)
					}
					consumed += remaining
					drawing = !drawing
					dashIdx = (dashIdx + 1) % len(dasharray)
					remaining = dasharray[dashIdx]
				} else {
					remaining -= avail
					if drawing {
						arena = append(arena, nx, ny)
					}
					break
				}
			}
			px, py = nx, ny
		}
		if drawing && len(arena)-segStart >= 4 {
			end := len(arena)
			result = append(result, arena[segStart:end:end])
		}
	}
	return result
}

// dashPhase advances the dash cycle by offset and returns the
// starting (dashIdx, drawing, remaining) triple. Positive offset
// advances forward; negative wraps via cycleLen. NaN/Inf → 0.
func dashPhase(dasharray []float32, offset, cycleLen float32) (int, bool, float32) {
	if math.IsNaN(float64(offset)) || math.IsInf(float64(offset), 0) {
		return 0, true, dasharray[0]
	}
	// Normalize into [0, cycleLen).
	skip := float32(math.Mod(float64(offset), float64(cycleLen)))
	if skip < 0 {
		skip += cycleLen
	}
	dashIdx := 0
	drawing := true
	remaining := dasharray[0]
	for skip > remaining {
		skip -= remaining
		dashIdx = (dashIdx + 1) % len(dasharray)
		remaining = dasharray[dashIdx]
		drawing = !drawing
	}
	remaining -= skip
	// Skip past zero-length dash/gap entries so the main loop does
	// not emit degenerate zero-point segments or mis-start in the
	// wrong phase ([0,150] wants to begin in the gap).
	for remaining == 0 && len(dasharray) > 1 {
		dashIdx = (dashIdx + 1) % len(dasharray)
		remaining = dasharray[dashIdx]
		drawing = !drawing
	}
	return dashIdx, drawing, remaining
}

// --- Curve flattening ---

func flattenPath(path *VectorPath, tolerance float32) [][]float32 {
	return flattenPathWithBake(path, tolerance, !isIdentityTransform(path.Transform))
}

// flattenPathWithBake is flattenPath with explicit control over whether
// path.Transform is baked into vertex coordinates. When bakeXform is
// false, vertices emit in local (pre-transform) space — caller applies
// the transform at render time via TessellatedPath.Base* fields.
func flattenPathWithBake(path *VectorPath, tolerance float32, bakeXform bool) [][]float32 {
	var polylines [][]float32
	estimatedCap := len(path.Segments) * 16
	current := make([]float32, 0, estimatedCap)
	var x, y, startX, startY float32
	hasTx := bakeXform

	for _, seg := range path.Segments {
		switch seg.Cmd {
		case CmdMoveTo:
			if len(current) >= 4 {
				polylines = append(polylines, current)
			}
			current = make([]float32, 0, estimatedCap)
			x = seg.Points[0]
			y = seg.Points[1]
			startX = x
			startY = y
			if hasTx {
				tx, ty := applyTransformPt(x, y, path.Transform)
				current = append(current, tx, ty)
			} else {
				current = append(current, x, y)
			}

		case CmdLineTo:
			x = seg.Points[0]
			y = seg.Points[1]
			if hasTx {
				tx, ty := applyTransformPt(x, y, path.Transform)
				if len(current) >= 2 && tx == current[len(current)-2] && ty == current[len(current)-1] {
					continue
				}
				current = append(current, tx, ty)
			} else {
				if len(current) >= 2 && x == current[len(current)-2] && y == current[len(current)-1] {
					continue
				}
				current = append(current, x, y)
			}

		case CmdQuadTo:
			cx := seg.Points[0]
			cy := seg.Points[1]
			ex := seg.Points[2]
			ey := seg.Points[3]
			if hasTx {
				tx, ty := applyTransformPt(x, y, path.Transform)
				tcx, tcy := applyTransformPt(cx, cy, path.Transform)
				tex, tey := applyTransformPt(ex, ey, path.Transform)
				flattenQuad(tx, ty, tcx, tcy, tex, tey, tolerance, &current)
			} else {
				flattenQuad(x, y, cx, cy, ex, ey, tolerance, &current)
			}
			x = ex
			y = ey

		case CmdCubicTo:
			c1x := seg.Points[0]
			c1y := seg.Points[1]
			c2x := seg.Points[2]
			c2y := seg.Points[3]
			ex := seg.Points[4]
			ey := seg.Points[5]
			if hasTx {
				tx, ty := applyTransformPt(x, y, path.Transform)
				tc1x, tc1y := applyTransformPt(c1x, c1y, path.Transform)
				tc2x, tc2y := applyTransformPt(c2x, c2y, path.Transform)
				tex, tey := applyTransformPt(ex, ey, path.Transform)
				flattenCubic(tx, ty, tc1x, tc1y, tc2x, tc2y, tex, tey, tolerance, &current)
			} else {
				flattenCubic(x, y, c1x, c1y, c2x, c2y, ex, ey, tolerance, &current)
			}
			x = ex
			y = ey

		case CmdClose:
			if len(current) >= 2 {
				if x != startX || y != startY {
					if hasTx {
						tx, ty := applyTransformPt(startX, startY, path.Transform)
						current = append(current, tx, ty)
					} else {
						current = append(current, startX, startY)
					}
				}
			}
			if len(current) >= 6 {
				polylines = append(polylines, current)
			}
			current = make([]float32, 0, estimatedCap)
			x = startX
			y = startY
		}
	}

	if len(current) >= 4 {
		polylines = append(polylines, current)
	}
	return polylines
}

func flattenQuad(x0, y0, cx, cy, x1, y1, tolerance float32, points *[]float32) {
	flattenQuadRec(x0, y0, cx, cy, x1, y1, tolerance, 0, points)
}

func flattenQuadRec(x0, y0, cx, cy, x1, y1, tolerance float32, depth int, points *[]float32) {
	mx := (x0 + x1) / 2
	my := (y0 + y1) / 2
	dx := cx - mx
	dy := cy - my
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d <= tolerance || depth >= maxFlattenDepth {
		*points = append(*points, x1, y1)
	} else {
		ax := (x0 + cx) / 2
		ay := (y0 + cy) / 2
		bx := (cx + x1) / 2
		by := (cy + y1) / 2
		abx := (ax + bx) / 2
		aby := (ay + by) / 2
		flattenQuadRec(x0, y0, ax, ay, abx, aby, tolerance, depth+1, points)
		flattenQuadRec(abx, aby, bx, by, x1, y1, tolerance, depth+1, points)
	}
}

func flattenCubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance float32, points *[]float32) {
	flattenCubicRec(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance, 0, points)
}

func flattenCubicRec(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance float32, depth int, points *[]float32) {
	dx := x1 - x0
	dy := y1 - y0
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d < curveDegenThreshold {
		*points = append(*points, x1, y1)
		return
	}

	d1 := f32Abs((c1x-x0)*dy-(c1y-y0)*dx) / d
	d2 := f32Abs((c2x-x0)*dy-(c2y-y0)*dx) / d

	if d1+d2 <= tolerance || depth >= maxFlattenDepth {
		*points = append(*points, x1, y1)
	} else {
		ax := (x0 + c1x) / 2
		ay := (y0 + c1y) / 2
		bx := (c1x + c2x) / 2
		by := (c1y + c2y) / 2
		cx := (c2x + x1) / 2
		cy := (c2y + y1) / 2
		abx := (ax + bx) / 2
		aby := (ay + by) / 2
		bcx := (bx + cx) / 2
		bcy := (by + cy) / 2
		mx := (abx + bcx) / 2
		my := (aby + bcy) / 2
		flattenCubicRec(x0, y0, ax, ay, abx, aby, mx, my, tolerance, depth+1, points)
		flattenCubicRec(mx, my, bcx, bcy, cx, cy, x1, y1, tolerance, depth+1, points)
	}
}
