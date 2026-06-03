package svg

import (
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// compileCSSAnimations materializes SvgAnimation records for all
// @keyframes-driven properties on a single shape. spec describes
// the element's animation-* values; pathID is the shape's PathID
// (used as TargetPathIDs[0]). state.cssKeyframes is the lookup
// table; state.animations receives appended records.
//
// Returns the number of records appended. Zero means the element
// has no animation-name, no matching @keyframes, or every property
// timeline lacked enough keyframes.
func compileCSSAnimations(spec cssAnimSpec, pathID uint32,
	transformOrigin string, b bbox, computed ComputedStyle,
	state *parseState) int {
	if spec.Name == "" || spec.DurationSec <= 0 {
		return 0
	}
	def := lookupKeyframes(state.cssKeyframes, spec.Name)
	if def == nil || len(def.Stops) < 1 {
		return 0
	}
	cx, cy := resolveTransformOrigin(transformOrigin, b)
	added := 0
	added += compileColorTimeline(def, "fill",
		gui.SvgAnimTargetFill, spec, pathID, computed, state)
	added += compileColorTimeline(def, "stroke",
		gui.SvgAnimTargetStroke, spec, pathID, computed, state)
	added += compileOpacityTimeline(def, "opacity",
		gui.SvgAnimTargetAll, spec, pathID, computed, state)
	added += compileOpacityTimeline(def, "fill-opacity",
		gui.SvgAnimTargetFill, spec, pathID, computed, state)
	added += compileOpacityTimeline(def, "stroke-opacity",
		gui.SvgAnimTargetStroke, spec, pathID, computed, state)
	added += compileTransformTimeline(def, spec, pathID, cx, cy, state)
	added += compileScalarTimeline(def, "stroke-dashoffset",
		gui.SvgAnimDashOffset, spec, pathID, computed, state)
	return added
}

// staticValueFor returns the element's computed value for a CSS
// property as a string, used to synthesize missing keyframe
// endpoints (CSS spec: implicit 0% / 100% stops inherit the
// element's static value). "" means no usable static — caller
// declines to pad that side.
func staticValueFor(prop string, c ComputedStyle) string {
	switch prop {
	case "stroke-dashoffset":
		return strconv.FormatFloat(float64(c.StrokeDashOffset), 'f', -1, 32)
	case "opacity":
		return strconv.FormatFloat(float64(c.Opacity), 'f', -1, 32)
	case "fill-opacity":
		return strconv.FormatFloat(float64(c.FillOpacity), 'f', -1, 32)
	case "stroke-opacity":
		return strconv.FormatFloat(float64(c.StrokeOpacity), 'f', -1, 32)
	case "fill":
		if c.FillSet {
			return svgColorToString(c.Fill)
		}
	case "stroke":
		if c.StrokeSet {
			return svgColorToString(c.Stroke)
		}
	}
	return ""
}

func svgColorToString(c gui.SvgColor) string {
	return "rgba(" +
		strconv.Itoa(int(c.R)) + "," +
		strconv.Itoa(int(c.G)) + "," +
		strconv.Itoa(int(c.B)) + "," +
		strconv.FormatFloat(float64(c.A)/255, 'f', -1, 32) + ")"
}

// compileScalarTimeline emits a scalar SvgAnimation (e.g.
// stroke-dashoffset) by reading one float per keyframe stop. Returns
// 1 when emitted.
func compileScalarTimeline(def *css.KeyframesDef, prop string,
	kind gui.SvgAnimKind, spec cssAnimSpec, pathID uint32,
	computed ComputedStyle, state *parseState) int {
	offsets, raw := gatherStopValuesPadded(def, prop, computed)
	if offsets == nil {
		return 0
	}
	vals := make([]float32, len(raw))
	for i, v := range raw {
		vals[i] = parseFloatTrimmed(v)
	}
	keyTimes := normalizeKeyTimes(offsets)
	a := buildBaseCSSAnim(spec, pathID)
	a.Kind = kind
	a.Values = vals
	a.KeyTimes = keyTimes
	applyTimingToAnim(spec, len(vals), &a)
	return appendCompiledAnim(state, a)
}

func lookupKeyframes(defs []css.KeyframesDef, name string) *css.KeyframesDef {
	for i := range defs {
		if defs[i].Name == name {
			return &defs[i]
		}
	}
	return nil
}

// gatherStopValuesForTransform is gatherStopValues for the transform
// property with identity-padded endpoints. Per CSS spec the missing
// 0% / 100% stops resolve to the element's static transform —
// effectively identity for SVG shapes that animate via CSS. The
// identity placeholder is a synthetic value string per function so
// sameFnAtIndex still passes.
func gatherStopValuesForTransform(def *css.KeyframesDef) ([]float32, []string) {
	var (
		offsets []float32
		values  []string
	)
	for i := range def.Stops {
		s := &def.Stops[i]
		v, ok := lookupDecl(s.Decls, "transform")
		if !ok {
			continue
		}
		offsets = append(offsets, s.Offset)
		values = append(values, v)
	}
	if len(offsets) == 0 {
		return nil, nil
	}
	if len(offsets) >= 2 && offsets[0] == 0 && offsets[len(offsets)-1] == 1 {
		return offsets, values
	}
	// Build an identity placeholder matching the function names in the
	// first observed stop so per-function emit code can pad the
	// timeline endpoints.
	fns := parseTransformFunctions(values[0])
	if len(fns) == 0 {
		return nil, nil
	}
	identity := transformIdentityFor(fns)
	if offsets[0] != 0 {
		offsets = append([]float32{0}, offsets...)
		values = append([]string{identity}, values...)
	}
	if offsets[len(offsets)-1] != 1 {
		offsets = append(offsets, 1)
		values = append(values, identity)
	}
	if len(offsets) < 2 {
		return nil, nil
	}
	return offsets, values
}

// transformIdentityFor returns the CSS transform string that leaves
// the rendered geometry unchanged for the given function list.
func transformIdentityFor(fns []cssTxFunc) string {
	var b strings.Builder
	for i, fn := range fns {
		if i > 0 {
			b.WriteByte(' ')
		}
		switch fn.name {
		case "rotate":
			b.WriteString("rotate(0)")
		case "translate":
			b.WriteString("translate(0,0)")
		case "translate3d":
			b.WriteString("translate3d(0,0,0)")
		case "scale":
			b.WriteString("scale(1,1)")
		default:
			b.WriteString(fn.name)
			b.WriteString("()")
		}
	}
	return b.String()
}

// gatherStopValuesPadded is gatherStopValues with implicit endpoint
// synthesis. When the keyframes block omits the 0% / 100% stop for a
// property, CSS substitutes the element's static computed value.
// Without this, partial keyframes (e.g. only `to { ... }`) compile
// to nothing.
func gatherStopValuesPadded(def *css.KeyframesDef, prop string,
	computed ComputedStyle) ([]float32, []string) {
	var (
		offsets []float32
		values  []string
	)
	for i := range def.Stops {
		s := &def.Stops[i]
		v, ok := lookupDecl(s.Decls, prop)
		if !ok {
			continue
		}
		offsets = append(offsets, s.Offset)
		values = append(values, v)
	}
	if len(offsets) == 0 {
		return nil, nil
	}
	staticVal := staticValueFor(prop, computed)
	if staticVal != "" && offsets[0] != 0 {
		offsets = append([]float32{0}, offsets...)
		values = append([]string{staticVal}, values...)
	}
	if staticVal != "" && offsets[len(offsets)-1] != 1 {
		offsets = append(offsets, 1)
		values = append(values, staticVal)
	}
	if len(offsets) < 2 {
		return nil, nil
	}
	return offsets, values
}

func lookupDecl(decls []css.Decl, name string) (string, bool) {
	for i := range decls {
		if decls[i].Name == name {
			return decls[i].Value, true
		}
	}
	return "", false
}

// compileColorTimeline emits one SvgAnimColor record for the
// fill or stroke channel. Returns 1 when emitted.
func compileColorTimeline(def *css.KeyframesDef, prop string,
	target gui.SvgAnimTarget, spec cssAnimSpec, pathID uint32,
	computed ComputedStyle, state *parseState) int {
	offsets, raw := gatherStopValuesPadded(def, prop, computed)
	if offsets == nil {
		return 0
	}
	cols := make([]uint32, len(raw))
	for i, v := range raw {
		c, ok := parseSvgColor(v)
		if !ok {
			// Invalid paint in a keyframe stop drops the whole color
			// timeline, matching applyPaintProp's "invalid → ignore"
			// rule for static declarations. Emitting cols[i]=0 would
			// silently animate to/from black instead.
			return 0
		}
		cols[i] = packRGBA(c)
	}
	keyTimes := normalizeKeyTimes(offsets)
	a := buildBaseCSSAnim(spec, pathID)
	a.Kind = gui.SvgAnimColor
	a.ColorValues = cols
	a.Target = target
	a.KeyTimes = keyTimes
	applyTimingToAnim(spec, len(cols), &a)
	return appendCompiledAnim(state, a)
}

// compileOpacityTimeline emits SvgAnimOpacity for the named opacity
// sub-channel. Returns 1 when emitted.
func compileOpacityTimeline(def *css.KeyframesDef, prop string,
	target gui.SvgAnimTarget, spec cssAnimSpec, pathID uint32,
	computed ComputedStyle, state *parseState) int {
	offsets, raw := gatherStopValuesPadded(def, prop, computed)
	if offsets == nil {
		return 0
	}
	vals := make([]float32, len(raw))
	for i, v := range raw {
		// parseOpacityNumber matches applyCSSProp's static path: a
		// trailing `%` divides by 100 instead of being clamped to 1.
		vals[i] = clampOpacity01(parseOpacityNumber(v))
	}
	keyTimes := normalizeKeyTimes(offsets)
	a := buildBaseCSSAnim(spec, pathID)
	a.Kind = gui.SvgAnimOpacity
	a.Values = vals
	a.Target = target
	a.KeyTimes = keyTimes
	applyTimingToAnim(spec, len(vals), &a)
	return appendCompiledAnim(state, a)
}

// compileTransformTimeline reads the `transform` property at each
// keyframe stop and splits per CSS transform function into one
// SvgAnimation record. Function order across stops must match —
// mismatches drop that function from the timeline. Lossy ordering
// is accepted; a matrix-kind animation is not yet supported.
func compileTransformTimeline(def *css.KeyframesDef, spec cssAnimSpec,
	pathID uint32, originX, originY float32, state *parseState) int {
	offsets, raw := gatherStopValuesForTransform(def)
	if offsets == nil {
		return 0
	}
	parsed := make([][]cssTxFunc, len(raw))
	for i, v := range raw {
		parsed[i] = parseTransformFunctions(v)
	}
	// Function-by-function — bail out as soon as any stop's function
	// name differs from the first stop's name at the same index.
	if len(parsed[0]) == 0 {
		return 0
	}
	added := 0
	for fi := range parsed[0] {
		name := parsed[0][fi].name
		if !sameFnAtIndex(parsed, fi, name) {
			continue
		}
		switch name {
		case "rotate":
			added += emitRotateAnim(parsed, fi, offsets, spec, pathID,
				originX, originY, state)
		case "translate", "translate3d":
			added += emitPairedAnim(parsed, fi, offsets, spec, pathID,
				state, gui.SvgAnimTranslate, 0, 0)
		case "scale":
			added += emitPairedAnim(parsed, fi, offsets, spec, pathID,
				state, gui.SvgAnimScale, 1, 1)
		}
	}
	return added
}

func sameFnAtIndex(parsed [][]cssTxFunc, fi int, name string) bool {
	for i := range parsed {
		if fi >= len(parsed[i]) || parsed[i][fi].name != name {
			return false
		}
	}
	return true
}

func emitRotateAnim(parsed [][]cssTxFunc, fi int, offsets []float32,
	spec cssAnimSpec, pathID uint32, originX, originY float32,
	state *parseState) int {
	angles := make([]float32, len(parsed))
	for i := range parsed {
		args := parsed[i][fi].args
		if len(args) >= 1 {
			angles[i] = args[0]
		}
	}
	a := buildBaseCSSAnim(spec, pathID)
	a.Kind = gui.SvgAnimRotate
	a.Values = angles
	a.KeyTimes = normalizeKeyTimes(offsets)
	a.CenterX = originX
	a.CenterY = originY
	applyTimingToAnim(spec, len(angles), &a)
	return appendCompiledAnim(state, a)
}

func emitPairedAnim(parsed [][]cssTxFunc, fi int, offsets []float32,
	spec cssAnimSpec, pathID uint32, state *parseState,
	kind gui.SvgAnimKind, defX, defY float32) int {
	pairs := make([]float32, 0, 2*len(parsed))
	for i := range parsed {
		args := parsed[i][fi].args
		x, y := defX, defY
		switch {
		case len(args) >= 2:
			x, y = args[0], args[1]
		case len(args) == 1:
			x, y = args[0], args[0]
		}
		pairs = append(pairs, x, y)
	}
	a := buildBaseCSSAnim(spec, pathID)
	a.Kind = kind
	a.Values = pairs
	a.KeyTimes = normalizeKeyTimes(offsets)
	applyTimingToAnim(spec, len(pairs)/2, &a)
	return appendCompiledAnim(state, a)
}

// buildBaseCSSAnim populates the timing/lifecycle fields shared by
// every CSS-compiled record.
func buildBaseCSSAnim(spec cssAnimSpec, pathID uint32) gui.SvgAnimation {
	iter := spec.IterCount
	if !spec.IterCountSet {
		iter = 1
	}
	freeze := spec.FillMode == cssAnimFillForwards ||
		spec.FillMode == cssAnimFillBoth
	backwards := spec.FillMode == cssAnimFillBackwards ||
		spec.FillMode == cssAnimFillBoth
	alternate := spec.Direction == cssAnimDirAlternate ||
		spec.Direction == cssAnimDirAlternateReverse
	return gui.SvgAnimation{
		DurSec:        spec.DurationSec,
		BeginSec:      spec.DelaySec,
		Iterations:    iter,
		Freeze:        freeze,
		FillBackwards: backwards,
		Alternate:     alternate,
		TargetPathIDs: []uint32{pathID},
		Restart:       gui.SvgAnimRestartAlways,
	}
}

// applyTimingToAnim folds the spec's timing-function into the
// animation. Cubic-bezier becomes a single keySplines tuple
// applied to every segment; steps becomes Discrete with synthetic
// stop offsets. Linear is the no-op default.
func applyTimingToAnim(spec cssAnimSpec, nKeys int, a *gui.SvgAnimation) {
	switch spec.TimingFn {
	case cssAnimTimingCubic:
		segs := nKeys - 1
		if segs <= 0 {
			return
		}
		a.CalcMode = gui.SvgAnimCalcSpline
		a.KeySplines = make([]float32, 0, 4*segs)
		for range segs {
			a.KeySplines = append(a.KeySplines,
				spec.TimingArgs[0], spec.TimingArgs[1],
				spec.TimingArgs[2], spec.TimingArgs[3])
		}
	case cssAnimTimingSteps:
		a.CalcMode = gui.SvgAnimCalcDiscrete
	default:
		a.CalcMode = gui.SvgAnimCalcLinear
	}
	if spec.Direction == cssAnimDirReverse ||
		spec.Direction == cssAnimDirAlternateReverse {
		reverseTimeline(a)
	}
}

// reverseTimeline reverses Values / ColorValues / KeyTimes /
// KeySplines for animation-direction: reverse / alternate-reverse.
func reverseTimeline(a *gui.SvgAnimation) {
	switch a.Kind {
	case gui.SvgAnimColor:
		for i, j := 0, len(a.ColorValues)-1; i < j; i, j = i+1, j-1 {
			a.ColorValues[i], a.ColorValues[j] = a.ColorValues[j], a.ColorValues[i]
		}
	case gui.SvgAnimTranslate, gui.SvgAnimScale:
		n := len(a.Values) / 2
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			a.Values[2*i], a.Values[2*j] = a.Values[2*j], a.Values[2*i]
			a.Values[2*i+1], a.Values[2*j+1] = a.Values[2*j+1], a.Values[2*i+1]
		}
	default:
		for i, j := 0, len(a.Values)-1; i < j; i, j = i+1, j-1 {
			a.Values[i], a.Values[j] = a.Values[j], a.Values[i]
		}
	}
	if len(a.KeyTimes) > 1 {
		// Reverse + complement so 0..1 stays monotonic ascending.
		n := len(a.KeyTimes)
		out := make([]float32, n)
		for i := range n {
			out[i] = 1 - a.KeyTimes[n-1-i]
		}
		a.KeyTimes = out
	}
}

// appendCompiledAnim respects the maxAnimations cap.
func appendCompiledAnim(state *parseState, a gui.SvgAnimation) int {
	if len(state.animations) >= maxAnimations {
		return 0
	}
	state.animations = append(state.animations, a)
	return 1
}

// normalizeKeyTimes returns the offsets verbatim when they form a
// valid [0,1] monotonic series starting at 0 and ending at 1; nil
// otherwise so the engine falls back to uniform spacing.
func normalizeKeyTimes(offsets []float32) []float32 {
	n := len(offsets)
	if n < 2 {
		return nil
	}
	if offsets[0] != 0 || offsets[n-1] != 1 {
		return nil
	}
	for i := 1; i < n; i++ {
		if offsets[i] < offsets[i-1] {
			return nil
		}
	}
	out := make([]float32, n)
	copy(out, offsets)
	return out
}

func packRGBA(c gui.SvgColor) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

// cssTxFunc is one parsed CSS transform function: name + numeric
// args (rotate angle in degrees; translate / scale in author units).
type cssTxFunc struct {
	name string
	args []float32
}

// parseTransformFunctions splits a CSS transform value into its
// constituent function calls. CSS transform syntax matches SVG
// closely enough that the SVG transform parser (parseTransform)
// would compose into a 3x3 matrix — but the design splits per
// function into separate SvgAnimation records, so re-parse here.
func parseTransformFunctions(s string) []cssTxFunc {
	var out []cssTxFunc
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == ',' ||
			s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
			i++
		}
		if i >= len(s) {
			break
		}
		nameStart := i
		for i < len(s) && s[i] != '(' && s[i] != ' ' {
			i++
		}
		name := strings.ToLower(s[nameStart:i])
		// Skip whitespace before '('.
		for i < len(s) && s[i] == ' ' {
			i++
		}
		if i >= len(s) || s[i] != '(' {
			break
		}
		i++ // past '('
		argStart := i
		depth := 1
		for i < len(s) && depth > 0 {
			switch s[i] {
			case '(':
				depth++
				i++
			case ')':
				depth--
				if depth > 0 {
					i++
				}
			default:
				i++
			}
		}
		if depth != 0 {
			break
		}
		args := parseNumberList(s[argStart:i])
		i++ // past ')'
		out = append(out, cssTxFunc{name: name, args: args})
	}
	return out
}
