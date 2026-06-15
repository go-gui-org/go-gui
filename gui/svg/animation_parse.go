package svg

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// parseAnimateElement parses an <animate> element targeting
// opacity (or fill-opacity / stroke-opacity, which scale the same
// rendered alpha channel). Returns the animation and true if valid.
func parseAnimateElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	attr, ok := findAttr(elem, "attributeName")
	if !ok {
		return gui.SvgAnimation{}, false
	}
	var target gui.SvgAnimTarget
	switch attr {
	case "opacity":
		target = gui.SvgAnimTargetAll
	case "fill-opacity":
		target = gui.SvgAnimTargetFill
	case "stroke-opacity":
		target = gui.SvgAnimTargetStroke
	default:
		return gui.SvgAnimation{}, false
	}
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}
	vals, additive, ok := parseScalarValues(elem)
	if !ok {
		return gui.SvgAnimation{}, false
	}
	return gui.SvgAnimation{
		Kind:       gui.SvgAnimOpacity,
		GroupID:    inherited.GroupID,
		Values:     vals,
		KeySplines: parseKeySplinesIfSpline(elem, len(vals)),
		KeyTimes:   parseKeyTimes(elem, len(vals)),
		DurSec:     dur,
		BeginSec:   parseBeginLiteral(elem),
		Cycle:      parseRepeatCycle(elem, dur),
		Freeze:     parseFreeze(elem),
		Additive:   additive || parseAdditiveSum(elem),
		Accumulate: parseAccumulateSum(elem),
		Target:     target,
		CalcMode:   parseCalcMode(elem),
		Restart:    parseRestart(elem),
	}, true
}

// parseScalarValues resolves values=/from+to/by into a 2+-entry
// Values slice. Returns (vals, additiveImplied, ok). additiveImplied
// is true when only by= was provided: Values=[0, by] with Additive=
// true composes correctly against the base value at apply time.
// Explicit additive="sum" may further upgrade the flag.
func parseScalarValues(elem string) ([]float32, bool, bool) {
	if valStr, ok := findAttr(elem, "values"); ok && valStr != "" {
		vs, vok := parseSemicolonFloatsOK(valStr)
		if !vok || len(vs) < 2 {
			return nil, false, false
		}
		return vs, false, true
	}
	fromStr, fromOK := findAttr(elem, "from")
	toStr, toOK := findAttr(elem, "to")
	if fromOK && toOK {
		f, fok := parseFloatStrict(fromStr)
		t, tok := parseFloatStrict(toStr)
		if !fok || !tok {
			return nil, false, false
		}
		return []float32{f, t}, false, true
	}
	if byStr, ok := findAttr(elem, "by"); ok {
		b, bok := parseFloatStrict(byStr)
		if !bok {
			return nil, false, false
		}
		return []float32{0, b}, true, true
	}
	if toOK {
		// to= without from= is spec-legal but needs the base value
		// at apply time. Emit [0, to] + additive so the base sums in.
		t, tok := parseFloatStrict(toStr)
		if !tok {
			return nil, false, false
		}
		return []float32{0, t}, true, true
	}
	return nil, false, false
}

// parseAdditiveSum reports whether the element has additive="sum".
func parseAdditiveSum(elem string) bool {
	v, ok := findAttr(elem, "additive")
	return ok && v == "sum"
}

// parseAccumulateSum reports whether the element has accumulate="sum".
func parseAccumulateSum(elem string) bool {
	v, ok := findAttr(elem, "accumulate")
	return ok && v == "sum"
}

// parseAnimateMotionElement parses an <animateMotion> element with
// an optional <mpath> body. Supports path=/from/to/by; the <mpath>
// href resolves from state.defsPaths. Returns a flattened polyline
// + cumulative arc lengths baked into the SvgAnimation.
func parseAnimateMotionElement(
	n *xmlNode, inherited ComputedStyle, state *parseState,
) (gui.SvgAnimation, bool) {
	if inherited.GroupID == "" {
		return gui.SvgAnimation{}, false
	}
	elem := n.OpenTag
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}
	d := motionPathD(n, state)
	if d == "" {
		return gui.SvgAnimation{}, false
	}
	poly, lens := flattenMotionD(d)
	if len(poly) < 4 || len(lens) < 2 {
		return gui.SvgAnimation{}, false
	}
	return gui.SvgAnimation{
		Kind:          gui.SvgAnimMotion,
		GroupID:       inherited.GroupID,
		DurSec:        dur,
		BeginSec:      parseBeginLiteral(elem),
		Cycle:         parseRepeatCycle(elem, dur),
		Freeze:        parseFreeze(elem),
		Additive:      parseAdditiveSum(elem),
		Accumulate:    parseAccumulateSum(elem),
		KeyTimes:      parseKeyTimes(elem, len(poly)/2),
		CalcMode:      parseCalcMode(elem),
		Restart:       parseRestart(elem),
		MotionPath:    poly,
		MotionLengths: lens,
		MotionRotate:  parseMotionRotate(elem),
	}, true
}

// parseMotionRotate reads the rotate= attr on animateMotion.
func parseMotionRotate(elem string) gui.SvgAnimMotionRotate {
	v, ok := findAttr(elem, "rotate")
	if !ok {
		return gui.SvgAnimMotionRotateNone
	}
	switch v {
	case "auto":
		return gui.SvgAnimMotionRotateAuto
	case "auto-reverse":
		return gui.SvgAnimMotionRotateAutoReverse
	}
	return gui.SvgAnimMotionRotateNone
}

// parseRestart returns the restart attribute as SvgAnimRestart.
// Defaults to always (SMIL default).
func parseRestart(elem string) gui.SvgAnimRestart {
	v, ok := findAttr(elem, "restart")
	if !ok {
		return gui.SvgAnimRestartAlways
	}
	switch v {
	case "never":
		return gui.SvgAnimRestartNever
	case "whenNotActive":
		return gui.SvgAnimRestartWhenNotActive
	}
	return gui.SvgAnimRestartAlways
}

// parseAnimateAttributeElement parses an <animate> element
// targeting an animatable primitive attribute (cx, cy, r, x, y,
// width, height, rx, ry).
func parseAnimateAttributeElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	attr, ok := findAttr(elem, "attributeName")
	if !ok {
		return gui.SvgAnimation{}, false
	}
	name := attrNameFromString(attr)
	if name == gui.SvgAttrNone {
		return gui.SvgAnimation{}, false
	}
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}
	vals, additive, ok := parseScalarValues(elem)
	if !ok {
		return gui.SvgAnimation{}, false
	}
	return gui.SvgAnimation{
		Kind:       gui.SvgAnimAttr,
		GroupID:    inherited.GroupID,
		Values:     vals,
		KeySplines: parseKeySplinesIfSpline(elem, len(vals)),
		KeyTimes:   parseKeyTimes(elem, len(vals)),
		DurSec:     dur,
		BeginSec:   parseBeginLiteral(elem),
		Cycle:      parseRepeatCycle(elem, dur),
		Freeze:     parseFreeze(elem),
		Additive:   additive || parseAdditiveSum(elem),
		Accumulate: parseAccumulateSum(elem),
		AttrName:   name,
		CalcMode:   parseCalcMode(elem),
		Restart:    parseRestart(elem),
	}, true
}

// parseAnimateDashOffsetElement parses an <animate> element
// targeting stroke-dashoffset. Scalar values per keyframe.
func parseAnimateDashOffsetElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}
	vals, additive, ok := parseScalarValues(elem)
	if !ok {
		return gui.SvgAnimation{}, false
	}
	return gui.SvgAnimation{
		Kind:       gui.SvgAnimDashOffset,
		GroupID:    inherited.GroupID,
		Values:     vals,
		KeySplines: parseKeySplinesIfSpline(elem, len(vals)),
		KeyTimes:   parseKeyTimes(elem, len(vals)),
		DurSec:     dur,
		BeginSec:   parseBeginLiteral(elem),
		Cycle:      parseRepeatCycle(elem, dur),
		Freeze:     parseFreeze(elem),
		Additive:   additive || parseAdditiveSum(elem),
		Accumulate: parseAccumulateSum(elem),
		CalcMode:   parseCalcMode(elem),
		Restart:    parseRestart(elem),
	}, true
}

// parseAnimateDashArrayElement parses an <animate> element targeting
// stroke-dasharray. Each keyframe is a whitespace/comma-separated
// list of floats; all keyframes must have the same count (1..cap).
// Mismatched lengths or cap overflow reject the animation.
func parseAnimateDashArrayElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}
	valStr, ok := findAttr(elem, "values")
	if !ok || valStr == "" {
		return gui.SvgAnimation{}, false
	}
	frames := strings.Split(valStr, ";")
	if len(frames) > maxKeyframes {
		frames = frames[:maxKeyframes]
	}
	// flat is sized at frames × stride once stride is known from the
	// first frame, avoiding 4× over-allocation for the common stride=2.
	var flat []float32
	stride := -1
	for _, f := range frames {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		nums := parseDashFrameFloats(f)
		if len(nums) == 0 {
			return gui.SvgAnimation{}, false
		}
		if len(nums) > gui.SvgAnimDashArrayCap {
			return gui.SvgAnimation{}, false
		}
		if stride < 0 {
			stride = len(nums)
			flat = make([]float32, 0, len(frames)*stride)
		} else if len(nums) != stride {
			return gui.SvgAnimation{}, false
		}
		flat = append(flat, nums...)
	}
	if stride <= 0 || len(flat) < 2*stride {
		return gui.SvgAnimation{}, false
	}
	nKeys := len(flat) / stride
	return gui.SvgAnimation{
		Kind:            gui.SvgAnimDashArray,
		GroupID:         inherited.GroupID,
		Values:          flat,
		KeySplines:      parseKeySplinesIfSpline(elem, nKeys),
		KeyTimes:        parseKeyTimes(elem, nKeys),
		DurSec:          dur,
		BeginSec:        parseBeginLiteral(elem),
		Cycle:           parseRepeatCycle(elem, dur),
		Freeze:          parseFreeze(elem),
		Additive:        parseAdditiveSum(elem),
		Accumulate:      parseAccumulateSum(elem),
		CalcMode:        parseCalcMode(elem),
		Restart:         parseRestart(elem),
		DashKeyframeLen: uint8(stride),
	}, true
}

// parseDashFrameFloats splits one dasharray keyframe (e.g.
// "42 150" or "42,150") into float32s. Field count is capped at
// SvgAnimDashArrayCap+1 so a hostile keyframe with millions of
// fields cannot drive a huge alloc; caller rejects overflow.
func parseDashFrameFloats(s string) []float32 {
	return scanFloatList(s, gui.SvgAnimDashArrayCap+1,
		func(b byte) bool {
			return b == ' ' || b == '\t' || b == ',' ||
				b == '\n' || b == '\r'
		})
}

// parseAnimateTransformElement parses an <animateTransform>
// element. Supports type="rotate" (from/to or values), plus
// type="translate" and type="scale" (values form). additive="sum"
// is not honored — animated values replace the base transform.
// The only corpus asset affected is pulse-ring.svg, where the
// base transform is a scale(0) placeholder that the animation
// fully overrides.
func parseAnimateTransformElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	typ, ok := findAttr(elem, "type")
	if !ok {
		return gui.SvgAnimation{}, false
	}
	switch typ {
	case "rotate":
		// fallthrough to original rotate logic below.
	case "translate":
		return parseAnimateTranslateElement(elem, inherited)
	case "scale":
		return parseAnimateScaleElement(elem, inherited)
	default:
		return gui.SvgAnimation{}, false
	}
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}

	if valStr, ok := findAttr(elem, "values"); ok && valStr != "" {
		angles, cx, cy, ok := parseRotateValues(valStr)
		if !ok {
			return gui.SvgAnimation{}, false
		}
		cx, cy = applyInheritedTransformPt(cx, cy, inherited.Transform)
		return gui.SvgAnimation{
			Kind:       gui.SvgAnimRotate,
			GroupID:    inherited.GroupID,
			Values:     angles,
			KeySplines: parseKeySplinesIfSpline(elem, len(angles)),
			KeyTimes:   parseKeyTimes(elem, len(angles)),
			CenterX:    cx,
			CenterY:    cy,
			DurSec:     dur,
			BeginSec:   parseBeginLiteral(elem),
			Cycle:      parseRepeatCycle(elem, dur),
			Freeze:     parseFreeze(elem),
			Additive:   parseAdditiveSum(elem),
			Accumulate: parseAccumulateSum(elem),
			CalcMode:   parseCalcMode(elem),
			Restart:    parseRestart(elem),
		}, true
	}

	angles, cx, cy, additive, ok := parseRotateFromToBy(elem)
	if !ok {
		return gui.SvgAnimation{}, false
	}
	cx, cy = applyInheritedTransformPt(cx, cy, inherited.Transform)
	return gui.SvgAnimation{
		Kind:       gui.SvgAnimRotate,
		GroupID:    inherited.GroupID,
		Values:     angles,
		KeySplines: parseKeySplinesIfSpline(elem, 2),
		KeyTimes:   parseKeyTimes(elem, 2),
		CenterX:    cx,
		CenterY:    cy,
		DurSec:     dur,
		BeginSec:   parseBeginLiteral(elem),
		Cycle:      parseRepeatCycle(elem, dur),
		Freeze:     parseFreeze(elem),
		Additive:   additive || parseAdditiveSum(elem),
		Accumulate: parseAccumulateSum(elem),
		CalcMode:   parseCalcMode(elem),
		Restart:    parseRestart(elem),
	}, true
}
