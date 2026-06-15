// Package svg parses and tessellates SVG content.
package svg

import (
	"math"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// motionFlattenTolerance is the curve-flattening tolerance used for
// animateMotion paths. Matches the tessellation default at scale 1.
const motionFlattenTolerance = 0.5

// maxMotionVertices caps the flattened animateMotion polyline so a
// pathological path can't drive an unbounded per-frame arc-length
// scan or allocation. Real assets have <100 vertices after flatten.
const maxMotionVertices = 1024

func attrNameFromString(s string) gui.SvgAttrName {
	switch s {
	case "cx":
		return gui.SvgAttrCX
	case "cy":
		return gui.SvgAttrCY
	case "r":
		return gui.SvgAttrR
	case "x":
		return gui.SvgAttrX
	case "y":
		return gui.SvgAttrY
	case "width":
		return gui.SvgAttrWidth
	case "height":
		return gui.SvgAttrHeight
	case "rx":
		return gui.SvgAttrRX
	case "ry":
		return gui.SvgAttrRY
	}
	return gui.SvgAttrNone
}

// pairY returns the second component from a parsed space-float
// list. Falls back to the first component (uniform) when only
// one value is present — matches SVG "scale(s)" shorthand.
func pairY(parts []float32) float32 {
	if len(parts) == 0 {
		return 0
	}
	if len(parts) >= 2 {
		return parts[1]
	}
	return parts[0]
}

// maxRepeatCountCycle caps repeatCount to bound cycle duration.
// Large finite repeats (e.g. 1e9) are semantically equivalent to
// "indefinite" for any practical session length.
const maxRepeatCountCycle = 1_000_000

// maxCycleSec caps a single cycle period (seconds). Upper bound
// is generous enough for any real asset (hours) while preventing
// +Inf / absurd values from authoring mistakes or hostile SVGs.
const maxCycleSec = float32(3600 * 24)

func clampCycle(v float32) float32 {
	// NaN compares false against both bounds below, so guard it
	// explicitly. -Inf and +Inf fall through to the natural branches.
	if math.IsNaN(float64(v)) {
		return 0
	}
	if v <= 0 {
		return 0
	}
	if v > maxCycleSec {
		return maxCycleSec
	}
	return v
}

// classifySetAttr maps an attributeName to the set's target Kind +
// sub-selectors. Only opacity and primitive attrs are supported.
func classifySetAttr(
	attr string,
) (gui.SvgAnimKind, gui.SvgAnimTarget, gui.SvgAttrName, bool) {
	switch attr {
	case "opacity":
		return gui.SvgAnimOpacity, gui.SvgAnimTargetAll,
			gui.SvgAttrNone, true
	case "fill-opacity":
		return gui.SvgAnimOpacity, gui.SvgAnimTargetFill,
			gui.SvgAttrNone, true
	case "stroke-opacity":
		return gui.SvgAnimOpacity, gui.SvgAnimTargetStroke,
			gui.SvgAttrNone, true
	}
	if n := attrNameFromString(attr); n != gui.SvgAttrNone {
		return gui.SvgAnimAttr, gui.SvgAnimTargetAll, n, true
	}
	return 0, 0, 0, false
}

// splitCommaOrSpace splits on runs of commas and/or whitespace.
func splitCommaOrSpace(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}
