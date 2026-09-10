package gui

import "math"

// ContainsPoint reports whether the path's filled region covers
// (px, py). Coordinates are in raw viewBox space — the same space
// used by Triangles. When HasBaseXform is set, the point is mapped
// back through the inverse of the author's translate/scale/rotate
// before the triangle test, since Triangles are stored in local
// (pre-base-transform) coords.
//
// Implementation: bbox fast-reject, then a barycentric containment
// test against each triangle. Stroke-only paths report false (their
// triangulation covers a thin band, which is rarely the user's
// intent for hit-testing). Use the fill TessellatedPath of a shape
// for hit-testing.
func (tp *TessellatedPath) ContainsPoint(px, py float32) bool {
	if tp == nil || len(tp.Triangles) < 6 || tp.IsStroke {
		return false
	}
	// Reject NaN/Inf input outright; the bbox check below would
	// silently misbehave (NaN compares false in both directions, so
	// the fast-reject would let NaN points through into the bary
	// loop where they yield non-deterministic results).
	if !f32IsFinite(px) || !f32IsFinite(py) {
		return false
	}
	if tp.HasBaseXform {
		px, py = inverseBaseXform(tp, px, py)
		if !f32IsFinite(px) || !f32IsFinite(py) {
			return false
		}
	}
	if px < tp.MinX || px > tp.MaxX || py < tp.MinY || py > tp.MaxY {
		return false
	}
	verts := tp.Triangles
	for i := 0; i+5 < len(verts); i += 6 {
		if pointInTri(px, py,
			verts[i], verts[i+1],
			verts[i+2], verts[i+3],
			verts[i+4], verts[i+5]) {
			return true
		}
	}
	return false
}

// pointInTri does a barycentric containment test. Mirrors the
// internal helper in gui/svg/tessellate.go. Inlined here so the gui
// package does not gain a dependency on gui/svg.
func pointInTri(px, py, ax, ay, bx, by, cx, cy float32) bool {
	v0x := cx - ax
	v0y := cy - ay
	v1x := bx - ax
	v1y := by - ay
	v2x := px - ax
	v2y := py - ay
	dot00 := v0x*v0x + v0y*v0y
	dot01 := v0x*v1x + v0y*v1y
	dot02 := v0x*v2x + v0y*v2y
	dot11 := v1x*v1x + v1y*v1y
	dot12 := v1x*v2x + v1y*v2y
	denom := dot00*dot11 - dot01*dot01
	if denom == 0 {
		return false
	}
	inv := 1 / denom
	uu := (dot11*dot02 - dot01*dot12) * inv
	vv := (dot00*dot12 - dot01*dot02) * inv
	return uu >= 0 && vv >= 0 && (uu+vv) <= 1
}

// inverseBaseXform applies the inverse of the path's decomposed base
// transform to (px, py), returning the equivalent point in local
// (Triangles) space. Compose order at render time is
// scale → rotate-about-pivot → translate; invert in reverse.
//
// Invariant from seedFromTransform (gui/svg/tessellate.go): when
// BaseRotAngle != 0, the matrix translate is absorbed into the
// rotation pivot (BaseRotCX/CY) and BaseTransX/Y are forced to 0.
// The inverse below relies on that — a non-zero translate combined
// with rotation would require the order T^-1 → R^-1 → S^-1 to keep
// the pivot in world coords, which is what the code does, but the
// pivot itself would need to be in post-translate space. Keep the
// two paths in sync if seedFromTransform's decomposition changes.
func inverseBaseXform(tp *TessellatedPath, px, py float32) (float32, float32) {
	tx := tp.BaseTransX
	ty := tp.BaseTransY
	sx := tp.BaseScaleX
	sy := tp.BaseScaleY
	// Coerce non-finite or zero scale to 1 — division would emit
	// Inf/NaN that propagates through the bary test.
	if sx == 0 || !f32IsFinite(sx) {
		sx = 1
	}
	if sy == 0 || !f32IsFinite(sy) {
		sy = 1
	}
	if !f32IsFinite(tx) {
		tx = 0
	}
	if !f32IsFinite(ty) {
		ty = 0
	}
	// Undo translate.
	x := px - tx
	y := py - ty
	// Undo rotation about (BaseRotCX, BaseRotCY). Skip when angle
	// is non-finite — sin/cos of NaN propagate as NaN and corrupt
	// the result.
	if tp.BaseRotAngle != 0 && f32IsFinite(tp.BaseRotAngle) {
		rad := float64(tp.BaseRotAngle) * math.Pi / 180
		cos := float32(math.Cos(rad))
		sin := float32(math.Sin(rad))
		dx := x - tp.BaseRotCX
		dy := y - tp.BaseRotCY
		// Inverse rotation: angle → -angle (cos same, sin negated).
		x = tp.BaseRotCX + cos*dx + sin*dy
		y = tp.BaseRotCY - sin*dx + cos*dy
	}
	// Undo scale.
	return x / sx, y / sy
}
