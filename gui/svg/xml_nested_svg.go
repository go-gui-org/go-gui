package svg

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// finiteOrDef returns def when v is NaN or ±Inf, otherwise v clamped
// to ±maxCoordinate so downstream arithmetic cannot overflow.
func finiteOrDef(v, def float32) float32 {
	f := float64(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return def
	}
	if v > maxCoordinate {
		return maxCoordinate
	}
	if v < -maxCoordinate {
		return -maxCoordinate
	}
	return v
}

// viewportRect describes the active SVG viewport in user-space
// coordinates. Root viewport is (vg.ViewBoxX, vg.ViewBoxY, vg.Width,
// vg.Height); each nested <svg> pushes a child viewport whose origin
// and dimensions match the inner viewBox (or the inner viewport rect
// when no viewBox is authored).
type viewportRect struct{ X, Y, W, H float32 }

// clippable reports whether the rect bounds a positive area worth
// emitting as a clip path. Zero/negative dims short-circuit synth.
func (v viewportRect) clippable() bool { return v.W > 0 && v.H > 0 }

// synthNestedClipPrefix is the id namespace for rectangle clip-paths
// minted when entering a nested <svg> viewport.
const synthNestedClipPrefix = "__nested_svg_clip_"

// resolveViewportLength resolves an x/y/width/height attribute on a
// nested <svg> against the parent viewport. Empty falls back to def
// (typically 0 for x/y or the parent dim for w/h). Bare numbers parse
// as user-space units. Trailing "%" interprets the leading number as
// a percentage of base.
func resolveViewportLength(s string, base, def float32) float32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	base = finiteOrDef(base, 0)
	def = finiteOrDef(def, 0)
	if hasPercent(s) {
		// Trim trailing % then parse as float64 so e.g. "1e30%" doesn't
		// truncate to ±Inf before scaling. Reject non-finite parses;
		// final value is clamped to ±maxCoordinate.
		t := strings.TrimSpace(strings.TrimRight(s, "%"))
		v, err := strconv.ParseFloat(t, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return def
		}
		return finiteOrDef(float32(v/100*float64(base)), def)
	}
	return finiteOrDef(parseLength(s), def)
}

// computeNestedSvgViewport derives the inner-viewBox-to-outer-coords
// affine for a nested <svg>, plus the viewBox rect that descendants
// treat as their user-space and the rect in outer-parent coordinates
// (the synthesized clip-to-viewport target).
//
// Rules:
//   - x/y default 0; width/height default 100% of parent viewport.
//   - With viewBox: scale = w/vbW and h/vbH; preserveAspectRatio
//     selects meet (min) or slice (max) for uniform align modes,
//     "none" keeps independent scales. Alignment fractions translate
//     residual space.
//   - Without viewBox: transform is plain translate(x,y); inner
//     user-space inherits the rect dims at origin (0,0).
func computeNestedSvgViewport(
	attrs map[string]string, parent viewportRect,
) (inner, outer viewportRect, m [6]float32) {
	parent.W = finiteOrDef(parent.W, 0)
	parent.H = finiteOrDef(parent.H, 0)
	x := finiteOrDef(resolveViewportLength(attrs["x"], parent.W, 0), 0)
	y := finiteOrDef(resolveViewportLength(attrs["y"], parent.H, 0), 0)
	w := clampViewBoxDim(finiteOrDef(
		resolveViewportLength(attrs["width"], parent.W, parent.W), 0))
	h := clampViewBoxDim(finiteOrDef(
		resolveViewportLength(attrs["height"], parent.H, parent.H), 0))
	outer = viewportRect{X: x, Y: y, W: w, H: h}

	if vb, ok := attrs["viewBox"]; ok {
		nums := parseNumberList(vb)
		// nums[2]/nums[3] > 0 rejects zero, negative, and NaN (NaN > 0
		// is false). +Inf passes; clampViewBoxDim then caps it.
		if len(nums) >= 4 && nums[2] > 0 && nums[3] > 0 {
			vbX := finiteOrDef(nums[0], 0)
			vbY := finiteOrDef(nums[1], 0)
			vbW := clampViewBoxDim(finiteOrDef(nums[2], 1))
			vbH := clampViewBoxDim(finiteOrDef(nums[3], 1))
			align, slice := parsePreserveAspectRatio(attrs["preserveAspectRatio"])
			sx := w / vbW
			sy := h / vbH
			if align != gui.SvgAlignNone {
				uniform := min(sx, sy)
				if slice {
					uniform = max(sx, sy)
				}
				sx, sy = uniform, uniform
			}
			xFrac, yFrac := gui.PreserveAlignFractions(align)
			tx := x + (w-sx*vbW)*xFrac - sx*vbX
			ty := y + (h-sy*vbH)*yFrac - sy*vbY
			sx = finiteOrDef(sx, 1)
			sy = finiteOrDef(sy, 1)
			tx = finiteOrDef(tx, 0)
			ty = finiteOrDef(ty, 0)
			inner = viewportRect{X: vbX, Y: vbY, W: vbW, H: vbH}
			m = [6]float32{sx, 0, 0, sy, tx, ty}
			return
		}
	}
	inner = viewportRect{X: 0, Y: 0, W: w, H: h}
	m = [6]float32{1, 0, 0, 1, x, y}
	return
}
