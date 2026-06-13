// Package svg parses and tessellates SVG content.
package svg

import (
	"math"
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

// motionFlattenTolerance is the curve-flattening tolerance used for
// animateMotion paths. Matches the tessellation default at scale 1.
const motionFlattenTolerance = 0.5

// maxMotionVertices caps the flattened animateMotion polyline so a
// pathological path can't drive an unbounded per-frame arc-length
// scan or allocation. Real assets have <100 vertices after flatten.
const maxMotionVertices = 1024

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

// motionPathD extracts the path d-string from an animateMotion:
// first via path= attr, else via <mpath xlink:href="#id"/> child.
func motionPathD(n *xmlNode, state *parseState) string {
	if p, ok := n.AttrMap["path"]; ok && p != "" {
		return p
	}
	if state == nil || len(state.defsPaths) == 0 {
		return ""
	}
	mp := n.findChild("mpath")
	if mp == nil {
		return ""
	}
	href, ok := mp.AttrMap["xlink:href"]
	if !ok {
		href, ok = mp.AttrMap["href"]
	}
	if !ok || !strings.HasPrefix(href, "#") {
		return ""
	}
	return state.defsPaths[href[1:]]
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

// flattenMotionD parses a path d-string and returns the flattened
// polyline + cumulative arc length array. Only the first subpath is
// used — animateMotion conventionally follows a single continuous
// curve; multi-M paths are uncommon.
func flattenMotionD(d string) ([]float32, []float32) {
	segs := parsePathD(d)
	if len(segs) == 0 {
		return nil, nil
	}
	vp := &VectorPath{
		Segments:  segs,
		Transform: identityTransform,
	}
	polys := flattenPath(vp, motionFlattenTolerance)
	if len(polys) == 0 {
		return nil, nil
	}
	poly := polys[0]
	if len(poly) < 4 {
		return nil, nil
	}
	n := len(poly) / 2
	if n > maxMotionVertices {
		n = maxMotionVertices
		poly = poly[:2*n]
	}
	lens := make([]float32, n)
	for i := 1; i < n; i++ {
		dx := poly[i*2] - poly[(i-1)*2]
		dy := poly[i*2+1] - poly[(i-1)*2+1]
		lens[i] = lens[i-1] +
			float32(math.Sqrt(float64(dx*dx+dy*dy)))
	}
	return poly, lens
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

// parseRotateFromToBy resolves the from/to or by shorthand on an
// animateTransform type="rotate". Returns angle slice, center, and
// whether the shorthand implies additive composition. by= alone
// maps to Values=[0, byAngle] with additive=true so the animation
// sums onto the base rotation (0).
func parseRotateFromToBy(elem string) ([]float32, float32, float32, bool, bool) {
	if byStr, ok := findAttr(elem, "by"); ok {
		parts := parseSpaceFloats(byStr)
		if len(parts) < 1 {
			return nil, 0, 0, false, false
		}
		var cx, cy float32
		if len(parts) >= 3 {
			cx, cy = parts[1], parts[2]
		}
		return []float32{0, parts[0]}, cx, cy, true, true
	}
	fromStr, _ := findAttr(elem, "from")
	toStr, _ := findAttr(elem, "to")
	if fromStr == "" || toStr == "" {
		return nil, 0, 0, false, false
	}
	fromParts := parseSpaceFloats(fromStr)
	toParts := parseSpaceFloats(toStr)
	if len(fromParts) < 3 || len(toParts) < 1 {
		return nil, 0, 0, false, false
	}
	return []float32{fromParts[0], toParts[0]},
		fromParts[1], fromParts[2], false, true
}

// parseAnimateTranslateElement parses <animateTransform
// type="translate"> with values="tx ty;tx ty;..." or from/to.
func parseAnimateTranslateElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	return parsePairedAnimateTransform(
		elem, inherited, gui.SvgAnimTranslate)
}

// parseAnimateScaleElement parses <animateTransform type="scale">
// with values="s;s;..." (uniform) or "sx sy;sx sy;..." (non-
// uniform). Uniform entries are normalized to equal sx,sy.
func parseAnimateScaleElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	return parsePairedAnimateTransform(
		elem, inherited, gui.SvgAnimScale)
}

// parsePairedAnimateTransform is the shared body for translate
// and scale animateTransform elements. Both produce Values as an
// interleaved [x,y, ...] stream with 2 floats per keyframe.
//
// inherited.Transform is intentionally NOT applied to the pair
// values: translate/scale animateTransform operates in the target
// element's local coordinate space and composes with its inherited
// transform at render time (see emitSvgPathRenderer). Baking the
// ancestor transform into the values here would apply it twice.
// Rotate's CenterX/CenterY are the exception — those are absolute
// SVG-space points used as the pivot, so the ancestor transform
// must be folded in during parse.
func parsePairedAnimateTransform(
	elem string, inherited ComputedStyle, kind gui.SvgAnimKind,
) (gui.SvgAnimation, bool) {
	dur := parseDuration(elem)
	if dur <= 0 {
		return gui.SvgAnimation{}, false
	}
	pairs, additive, ok := parsePairedFromToBy(elem)
	if !ok {
		return gui.SvgAnimation{}, false
	}
	return gui.SvgAnimation{
		Kind:       kind,
		GroupID:    inherited.GroupID,
		Values:     pairs,
		KeySplines: parseKeySplinesIfSpline(elem, len(pairs)/2),
		KeyTimes:   parseKeyTimes(elem, len(pairs)/2),
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

// parsePairedFromToBy resolves values=/from+to/by shorthand on a
// paired animateTransform. by= emits Values=[0,0, bx,by] with
// additive=true so the animation sums onto the base translate (0,0)
// or scale (kept as 0,0 here — apply-time base injection of (1,1)
// depends on additive semantics; see applyAnimContrib).
func parsePairedFromToBy(elem string) ([]float32, bool, bool) {
	if valStr, ok := findAttr(elem, "values"); ok && valStr != "" {
		pairs := parsePairedValues(valStr)
		if len(pairs) < 4 {
			return nil, false, false
		}
		return pairs, false, true
	}
	if byStr, ok := findAttr(elem, "by"); ok {
		by := parseSpaceFloats(byStr)
		if len(by) < 1 {
			return nil, false, false
		}
		return []float32{0, 0, by[0], pairY(by)}, true, true
	}
	fromStr, fromOK := findAttr(elem, "from")
	toStr, toOK := findAttr(elem, "to")
	if !toOK {
		return nil, false, false
	}
	to := parseSpaceFloats(toStr)
	if len(to) < 1 {
		return nil, false, false
	}
	if fromOK {
		from := parseSpaceFloats(fromStr)
		if len(from) < 1 {
			return nil, false, false
		}
		return []float32{
			from[0], pairY(from),
			to[0], pairY(to),
		}, false, true
	}
	// to= only: sum onto base with additive=true.
	return []float32{0, 0, to[0], pairY(to)}, true, true
}

// pairY returns the second component from a parsed space-float
// list. Falls back to the first component (uniform) when only
// one value is present — matches SVG "scale(s)" shorthand.
func pairY(parts []float32) float32 {
	if len(parts) >= 2 {
		return parts[1]
	}
	return parts[0]
}

// parsePairedValues parses a semicolon-separated values= list
// where each entry is "a [b]" (space-separated). Missing second
// component is duplicated (uniform-scale / same-for-y shorthand).
// Returns an interleaved flat slice of 2 floats per entry. Caps
// keyframe count at maxKeyframes.
func parsePairedValues(s string) []float32 {
	parts := strings.Split(s, ";")
	if len(parts) > maxKeyframes {
		parts = parts[:maxKeyframes]
	}
	out := make([]float32, 0, 2*len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		nums := parseSpaceFloats(p)
		if len(nums) == 0 {
			return nil
		}
		x := nums[0]
		y := nums[0]
		if len(nums) >= 2 {
			y = nums[1]
		}
		out = append(out, x, y)
	}
	return out
}

// parseRotateValues parses a semicolon-separated list of rotate
// keyframes like "0 12 12;360 12 12". Each keyframe is "angle
// [cx cy]". Returns angle slice + center from the first keyframe.
// Center must stay constant across keyframes; mismatches are
// accepted but only the first is honored (rare in practice).
func parseRotateValues(s string) ([]float32, float32, float32, bool) {
	parts := strings.Split(s, ";")
	if len(parts) > maxKeyframes {
		parts = parts[:maxKeyframes]
	}
	angles := make([]float32, 0, len(parts))
	var cx, cy float32
	first := true
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		trip := parseSpaceFloats(p)
		if len(trip) == 0 {
			return nil, 0, 0, false
		}
		angles = append(angles, trip[0])
		if first && len(trip) >= 3 {
			cx = trip[1]
			cy = trip[2]
		}
		first = false
	}
	if len(angles) < 2 {
		return nil, 0, 0, false
	}
	return angles, cx, cy, true
}

// parseFreeze reports whether the animation has fill="freeze".
// SMIL fill defaults to "remove"; only "freeze" alters render-time
// behavior in our model.
func parseFreeze(elem string) bool {
	v, ok := findAttr(elem, "fill")
	return ok && v == "freeze"
}

// parseRepeatCycle returns the per-animation cycle period derived
// from repeatCount/repeatDur. repeatCount="indefinite" yields the
// dur (continuous loop). A finite numeric repeatCount yields
// dur*count so the animation re-fires after the full repeat span.
// Returns 0 when the animation should play once (no looping); a
// later resolveBegins pass may still inherit a chain-derived cycle.
// Hostile inputs are clamped: a huge repeatCount is capped at
// maxRepeatCountCycle and the final cycle is never allowed to
// exceed maxCycleSec so downstream comparisons / floor math stay
// finite and bounded.
func parseRepeatCycle(elem string, dur float32) float32 {
	if v, ok := findAttr(elem, "repeatCount"); ok && v != "" {
		if v == "indefinite" {
			return clampCycle(dur)
		}
		n := parseF32(v)
		if n > maxRepeatCountCycle {
			n = maxRepeatCountCycle
		}
		if n > 0 {
			return clampCycle(dur * n)
		}
	}
	if v, ok := findAttr(elem, "repeatDur"); ok && v != "" {
		if v == "indefinite" {
			return clampCycle(dur)
		}
		t := parseTimeValue(v)
		if t > 0 {
			return clampCycle(t)
		}
	}
	return 0
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

// parseDuration extracts the "dur" attribute as seconds and applies
// the SMIL min/max clamp. Effective dur = clamp(dur, min, max). Min
// and max default to 0 (unset); unset bounds do not clamp. A max ≤
// min is ignored (min wins) — matches SMIL precedence.
func parseDuration(elem string) float32 {
	s, ok := findAttr(elem, "dur")
	if !ok || s == "" {
		return 0
	}
	dur := parseTimeValue(s)
	if dur <= 0 {
		return dur
	}
	if v, ok := findAttr(elem, "min"); ok && v != "" {
		minD := parseTimeValue(v)
		if minD > 0 && dur < minD {
			dur = minD
		}
	}
	if v, ok := findAttr(elem, "max"); ok && v != "" {
		maxD := parseTimeValue(v)
		if maxD > 0 && dur > maxD {
			dur = maxD
		}
	}
	return dur
}

// parseBeginLiteral returns the first absolute-time entry in a
// semicolon-separated begin list. Syncbase references (entries
// containing ".begin" or ".end") are skipped here and resolved
// in resolveBegins post-pass. Returns 0 when no literal present.
func parseBeginLiteral(elem string) float32 {
	s, ok := findAttr(elem, "begin")
	if !ok || s == "" {
		return 0
	}
	for part := range strings.SplitSeq(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, ".begin") ||
			strings.Contains(part, ".end") {
			continue
		}
		return parseTimeValue(part)
	}
	return 0
}

// parseTimeValue converts a time string (e.g. "1.5s", "200ms") to
// seconds. Negative passes through — CSS animation-delay uses it to
// seek forward at start. Non-finite maps to 0.
func parseTimeValue(s string) float32 {
	s = strings.TrimSpace(s)
	var v float32
	switch {
	case strings.HasSuffix(s, "ms"):
		v = parseF32(s[:len(s)-2]) / 1000
	case strings.HasSuffix(s, "s"):
		v = parseF32(s[:len(s)-1])
	default:
		// Bare number defaults to seconds per SVG spec.
		v = parseF32(s)
	}
	if !finiteF32(v) {
		return 0
	}
	return v
}

// parseSemicolonFloats splits a semicolon-separated string into
// float32 values. Caps the result at maxKeyframes entries to
// bound allocation on pathological input.
func parseSemicolonFloats(s string) []float32 {
	return scanFloatList(s, maxKeyframes,
		func(b byte) bool { return b == ';' })
}

// parseSemicolonFloatsOK is the strict variant: any malformed token
// (incl. NaN/Inf) returns ok=false so SMIL values= can be rejected as
// a unit rather than coerced to silent zeros.
func parseSemicolonFloatsOK(s string) ([]float32, bool) {
	return scanFloatListOK(s, maxKeyframes,
		func(b byte) bool { return b == ';' })
}

// parseSpaceFloats splits a space-separated string into float32
// values.
func parseSpaceFloats(s string) []float32 {
	return scanFloatList(s, 0,
		func(b byte) bool {
			return b == ' ' || b == '\t' || b == '\n' || b == '\r'
		})
}

// scanFloatListOK runs scanFloatList's split logic with strict parse:
// returns ok=false on the first unparseable token.
func scanFloatListOK(
	s string, limit int, isSep func(byte) bool,
) ([]float32, bool) {
	if limit == 0 {
		limit = len(s)
	}
	var out []float32
	for i := 0; i < len(s); {
		start := i
		for start < len(s) && isSep(s[start]) {
			start++
		}
		if start >= len(s) {
			break
		}
		end := start
		for end < len(s) && !isSep(s[end]) {
			end++
		}
		if tok := strings.TrimSpace(s[start:end]); tok != "" {
			v, ok := parseFloatStrict(tok)
			if !ok {
				return nil, false
			}
			if out == nil {
				out = make([]float32, 0, min(limit, 8))
			}
			out = append(out, v)
			if len(out) >= limit {
				break
			}
		}
		i = end
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func scanFloatList(s string, limit int, isSep func(byte) bool) []float32 {
	if limit == 0 {
		limit = len(s)
	}
	out := make([]float32, 0, min(limit, 8))
	for i := 0; i < len(s); {
		start := i
		for start < len(s) && isSep(s[start]) {
			start++
		}
		if start >= len(s) {
			break
		}
		end := start
		for end < len(s) && !isSep(s[end]) {
			end++
		}
		if tok := strings.TrimSpace(s[start:end]); tok != "" {
			out = append(out, parseF32(tok))
			if len(out) >= limit {
				break
			}
		}
		i = end
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseKeyTimes parses the keyTimes attribute as a monotonic [0,1]
// list with exactly nKeys entries. Returns nil when the attribute is
// absent, malformed, mismatched in length, not bracketed by 0..1, or
// not monotonic — caller falls back to uniform spacing. Strict mode
// is the SMIL default (applies to calcMode linear/spline/paced; for
// discrete the last entry may be < 1 but we still require 0..1 for
// simplicity — authors rarely omit the trailing 1).
func parseKeyTimes(elem string, nKeys int) []float32 {
	raw, ok := findAttr(elem, "keyTimes")
	if !ok || raw == "" {
		return nil
	}
	if nKeys < 2 {
		return nil
	}
	parts := parseSemicolonFloats(raw)
	if len(parts) != nKeys {
		return nil
	}
	if parts[0] != 0 || parts[nKeys-1] != 1 {
		return nil
	}
	for i := 1; i < nKeys; i++ {
		if parts[i] < parts[i-1] {
			return nil
		}
	}
	return parts
}

// parseSetElement parses a <set attributeName="X" to="Y"> element
// as a zero-duration animation. attributeName must be opacity (or
// its fill-/stroke- variants) or a primitive attr; other names are
// rejected. IsSet=true flags the special eval path that ignores dur.
// Per plan decision: fill defaults to freeze semantics here — once
// activated, the value contributes until a later-activated <set>
// overrides, matching the corpus expectation. Actual fill="remove"
// is still honored: when !Freeze the contribution is still added
// (IsSet overrides the dur reject) but sandwich ordering and the
// cycle re-fire let subsequent activations replace it cleanly.
func parseSetElement(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	attr, ok := findAttr(elem, "attributeName")
	if !ok {
		return gui.SvgAnimation{}, false
	}
	toStr, ok := findAttr(elem, "to")
	if !ok || toStr == "" {
		return gui.SvgAnimation{}, false
	}
	kind, target, attrName, ok := classifySetAttr(attr)
	if !ok {
		return gui.SvgAnimation{}, false
	}
	v := parseF32(toStr)
	anim := gui.SvgAnimation{
		Kind:     kind,
		GroupID:  inherited.GroupID,
		Values:   []float32{v, v},
		BeginSec: parseBeginLiteral(elem),
		Freeze:   true,
		IsSet:    true,
		Target:   target,
		AttrName: attrName,
		Restart:  parseRestart(elem),
	}
	// Honor explicit fill="remove" so <set fill="remove"> still works.
	if fv, ok := findAttr(elem, "fill"); ok && fv == "remove" {
		anim.Freeze = false
	}
	return anim, true
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

// parseCalcMode returns the calcMode attribute as SvgAnimCalcMode.
// Absent / unrecognized values fall through to linear (the SMIL
// default). "paced" is treated as linear — it would need per-segment
// distance math and no corpus asset currently uses it.
func parseCalcMode(elem string) gui.SvgAnimCalcMode {
	mode, ok := findAttr(elem, "calcMode")
	if !ok {
		return gui.SvgAnimCalcLinear
	}
	switch mode {
	case "spline":
		return gui.SvgAnimCalcSpline
	case "discrete":
		return gui.SvgAnimCalcDiscrete
	}
	return gui.SvgAnimCalcLinear
}

// parseKeySplinesIfSpline returns flat 4*(nVals-1) spline control
// points when the element has calcMode="spline" and a matching
// keySplines list. Returns nil otherwise (fast-path linear lerp).
// A segment count mismatch drops splines rather than erroring —
// real-world SVGs sometimes omit the final segment.
func parseKeySplinesIfSpline(elem string, nVals int) []float32 {
	mode, ok := findAttr(elem, "calcMode")
	if !ok || mode != "spline" {
		return nil
	}
	raw, ok := findAttr(elem, "keySplines")
	if !ok || raw == "" {
		return nil
	}
	segs := nVals - 1
	if segs <= 0 || segs > maxKeyframes {
		return nil
	}
	parts := strings.Split(raw, ";")
	if len(parts) > maxKeyframes {
		parts = parts[:maxKeyframes]
	}
	out := make([]float32, 0, 4*segs)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Fields split on comma or whitespace — SVG allows either.
		quads := splitCommaOrSpace(p)
		if len(quads) != 4 {
			return nil
		}
		for _, q := range quads {
			// Strict parse so "NaN"/"Inf" tokens reject the list
			// instead of coercing to 0 and slipping past [0,1].
			f, ok := parseFloatStrict(q)
			if !ok || f < 0 || f > 1 {
				return nil
			}
			out = append(out, f)
		}
	}
	if len(out) != 4*segs {
		return nil
	}
	return out
}

// splitCommaOrSpace splits on runs of commas and/or whitespace.
func splitCommaOrSpace(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}
