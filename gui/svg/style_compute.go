package svg

import (
	"maps"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// applyOpacity multiplies opacity into color alpha channel.
func applyOpacity(c gui.SvgColor, opacity float32) gui.SvgColor {
	if opacity >= 1.0 {
		return c
	}
	if opacity <= 0 || opacity != opacity {
		return gui.SvgColor{R: c.R, G: c.G, B: c.B, A: 0}
	}
	return gui.SvgColor{R: c.R, G: c.G, B: c.B, A: uint8(float32(c.A) * opacity)}
}

// parseOpacityAttr extracts an opacity value from element attrs.
func parseOpacityAttr(elem, name string, fallback float32) float32 {
	val, ok := findAttrOrStyle(elem, name)
	if !ok {
		return fallback
	}
	o := parseFloatTrimmed(val)
	if o < 0 {
		return 0
	}
	if o > 1.0 {
		return 1.0
	}
	return o
}

func parseElementStyle(elem string) elementStyle {
	return elementStyle{
		Transform:        getTransform(elem),
		StrokeColor:      getStrokeColor(elem),
		StrokeWidth:      getStrokeWidth(elem),
		StrokeCap:        getStrokeLinecap(elem),
		StrokeJoin:       getStrokeLinejoin(elem),
		Opacity:          parseOpacityAttr(elem, "opacity", 1.0),
		FillOpacity:      parseOpacityAttr(elem, "fill-opacity", 1.0),
		StrokeOpacity:    parseOpacityAttr(elem, "stroke-opacity", 1.0),
		StrokeGradientID: getStrokeGradientID(elem),
		StrokeDasharray:  getStrokeDasharray(elem),
	}
}

// computeStyle walks one element under parent, returning the
// resolved ComputedStyle. Cascade order:
//
//  1. Pres-attr decls (Origin=Pres, spec=0)
//  2. Author CSS rule decls (Origin=Rule, spec from selector)
//  3. Inline style="" decls (Origin=Inline, spec=0)
//  4. !important promotes any layer above all normal layers
//
// Custom properties (--name) are gathered first and var(--x)
// substitution happens at apply time. Transform composes parent ×
// own; clip-path / filter are inherited unless the element overrides.
func computeStyle(
	elem string,
	parent ComputedStyle,
	state *parseState,
	info css.ElementInfo,
	ancestors []css.ElementInfo,
	siblings []css.ElementInfo,
) ComputedStyle {
	out := parent
	out.Transform = matrixMultiply(parent.Transform, getTransform(elem))
	out.Vars = parent.Vars
	// Reset per-element opacity scalar; combine with parent at the
	// end. FillOpacity / StrokeOpacity inherit values directly.
	out.Opacity = 1
	out.FillOpacity = parent.FillOpacity
	out.StrokeOpacity = parent.StrokeOpacity
	// CSS animations are not inherited.
	out.Animation = cssAnimSpec{}
	// transform-origin is not inherited per CSS Transforms 1; reset.
	out.TransformOrigin = ""
	// display is not inherited; visibility is. Reset display so
	// descendants of a non-skipped element start "rendered". Skip
	// logic in parseSvgContent / appendShape filters elements whose
	// own cascade resolves to display:none.
	out.Display = DisplayInline

	// clip-path / filter participate in the cascade so CSS rules and
	// inline style="" can set them, not only the bare attribute. Seed
	// from parent (inherited); the cascade fold below overwrites when
	// the element declares its own value via any origin. After the
	// fold, a fresh FilterID allocates a new per-occurrence FilterGroupKey
	// so two siblings sharing one filter render to two offscreen
	// buffers (composite-z correctness).
	out.ClipPathID = parent.ClipPathID
	out.FilterID = parent.FilterID
	out.FilterGroupKey = parent.FilterGroupKey
	out.AuthoredClipPath = false
	if gid, ok := findAttr(elem, "id"); ok {
		out.GroupID = gid
	} else {
		out.GroupID = parent.GroupID
	}

	var decls []css.MatchedDecl
	for _, name := range presAttrCascadeNames {
		if v, ok := findAttr(elem, name); ok {
			decls = append(decls, css.MatchedDecl{
				Decl:   css.Decl{Name: name, Value: v},
				Origin: css.OriginPresAttr,
			})
		}
	}
	if state != nil && len(state.cssRules) > 0 {
		decls = append(decls,
			css.Match(state.cssRules, info, ancestors, siblings)...)
	}
	if styleAttr, ok := findAttr(elem, "style"); ok {
		for _, d := range parseInlineStyle(styleAttr) {
			decls = append(decls, css.MatchedDecl{
				Decl:   d,
				Origin: css.OriginInline,
			})
		}
	}
	if len(decls) == 0 {
		out.Opacity = parent.Opacity
		return out
	}
	css.SortCascade(decls)

	out.Vars = collectVars(decls, parent.Vars)
	var authoredFilter bool
	for _, d := range decls {
		if d.CustomProp {
			continue
		}
		v := resolveVarRefs(d.Value, out.Vars)
		if v == "" {
			continue
		}
		v = resolveCalcRefs(v)
		if v == "" {
			continue
		}
		if applyCSSAnimProp(d.Name, v, &out.Animation) {
			continue
		}
		// Mark authored only when the declaration parses as a usable
		// value (url(#id) or the "none" keyword). A bogus value like
		// `clip-path: bogus` must be ignored per CSS, otherwise it
		// would either suppress the synthesized nested-svg viewport
		// clip (clip-path) or allocate a fresh per-occurrence filter
		// group buffer (filter) without any actual filter applying.
		switch d.Name {
		case "clip-path":
			if isValidClipOrFilterValue(v) {
				out.AuthoredClipPath = true
			}
		case "filter":
			if isValidClipOrFilterValue(v) {
				authoredFilter = true
			}
		}
		applyCSSProp(d.Name, v, &out)
	}
	out.Opacity = parent.Opacity * out.Opacity
	// Filter group key is per-occurrence: any element that declares
	// filter via its own cascade origin gets a fresh key so two
	// siblings allocate distinct offscreen buffers — even when the
	// declared id matches the inherited one. Pure inheritance (no
	// authored decl) shares the parent's group.
	if authoredFilter {
		if out.FilterID != "" {
			state.nextFilterGroup++
			out.FilterGroupKey = state.nextFilterGroup
		} else {
			out.FilterGroupKey = 0
		}
	}
	return out
}

// collectVars folds custom-property declarations from a sorted
// cascade into a vars map. When the element introduces no new vars,
// the parent's map is shared (no allocation).
func collectVars(decls []css.MatchedDecl,
	parentVars map[string]string,
) map[string]string {
	var out map[string]string
	for _, d := range decls {
		if !d.CustomProp {
			continue
		}
		if out == nil {
			out = make(map[string]string, len(parentVars)+4)
			maps.Copy(out, parentVars)
		}
		out[strings.ToLower(d.Name)] = d.Value
	}
	if out == nil {
		return parentVars
	}
	return out
}

// makeElementInfo builds a css.ElementInfo from the element tag,
// raw open-tag text, and tree-walk metadata (1-based child index
// in parent, isRoot for the root <svg>). attrs is the parsed
// attribute map (nil to disable attribute-selector matching for
// this element). The map is aliased, not copied: callers must treat
// it as read-only — `matchAttr` honors that contract today.
func makeElementInfo(
	tag, openTag string, index int, isRoot bool,
	attrs map[string]string,
) css.ElementInfo {
	info := css.ElementInfo{Tag: tag, Index: index, IsRoot: isRoot}
	if id, ok := findAttr(openTag, "id"); ok {
		info.ID = id
	}
	if cls, ok := findAttr(openTag, "class"); ok {
		info.Classes = splitClassAttr(cls)
	}
	info.Attrs = attrs
	return info
}

// applyPseudoState toggles ElementInfo.State.Hover / Focus when the
// element's id matches parseState.hoveredID / focusedID. Empty IDs
// disable the corresponding state.
func applyPseudoState(info *css.ElementInfo, state *parseState) {
	if state == nil {
		return
	}
	if state.hoveredID != "" && info.ID == state.hoveredID {
		info.State.Hover = true
	}
	if state.focusedID != "" && info.ID == state.focusedID {
		info.State.Focus = true
	}
}

// resolveFillRule reads fill-rule from elem, falling back to the
// inherited value. "evenodd" maps to FillRuleEvenOdd; any other
// token (including the empty string) maps to FillRuleNonzero,
// which is the SVG default. Case-sensitive per SVG spec.
func resolveFillRule(elem string, parent ComputedStyle) FillRule {
	val, ok := findAttrOrStyle(elem, "fill-rule")
	if !ok {
		return parent.FillRule
	}
	if strings.TrimSpace(val) == "evenodd" {
		return FillRuleEvenOdd
	}
	return FillRuleNonzero
}

// applyComputedStyle folds the cascade-resolved style into a
// shape's VectorPath. inh is authoritative for paint properties:
// the cascade has already merged pres-attrs, author CSS, and inline
// style with proper precedence, so any duplicate value the shape
// parser stashed onto path is overwritten. Geometry (Segments,
// Primitive, FillRule) and shape-owned IDs (ClipPathID, GroupID
// when the shape has inline animations) survive.
func applyComputedStyle(path *VectorPath, inh ComputedStyle) {
	// inh.Transform = parent × own (composed in computeStyle). The
	// shape parser already stashed `own` onto path.Transform via
	// parseElementStyle; assigning inh.Transform replaces that with
	// the fully composed matrix and avoids double-applying `own`.
	path.Transform = inh.Transform

	if path.ClipPathID == "" && inh.ClipPathID != "" {
		path.ClipPathID = inh.ClipPathID
	}
	if path.FilterID == "" && inh.FilterID != "" {
		path.FilterID = inh.FilterID
	}
	if path.FilterGroupKey == 0 {
		path.FilterGroupKey = inh.FilterGroupKey
	}

	// Fill — gradient takes precedence over color. Honor cascade
	// winner over the shape's pres-attr-derived value.
	switch {
	case inh.FillGradient != "":
		path.FillGradientID = inh.FillGradient
		path.FillColor = colorTransparent
	case inh.FillSet:
		path.FillColor = inh.Fill
		path.FillGradientID = ""
	case path.FillGradientID == "" && path.FillColor == colorInherit:
		path.FillColor = colorBlack
	}

	switch {
	case inh.StrokeGradient != "":
		path.StrokeGradientID = inh.StrokeGradient
		path.StrokeColor = colorTransparent
	case inh.StrokeSet:
		path.StrokeColor = inh.Stroke
		path.StrokeGradientID = ""
	case path.StrokeColor == colorInherit:
		path.StrokeColor = colorTransparent
	}
	if inh.StrokeWidth >= 0 {
		path.StrokeWidth = inh.StrokeWidth
	}
	if path.StrokeWidth < 0 {
		path.StrokeWidth = 1.0
	}
	if inh.StrokeCap != strokeCapInherit {
		path.StrokeCap = inh.StrokeCap
	}
	if path.StrokeCap == strokeCapInherit {
		path.StrokeCap = gui.SvgButtCap
	}
	if inh.StrokeJoin != strokeJoinInherit {
		path.StrokeJoin = inh.StrokeJoin
	}
	if path.StrokeJoin == strokeJoinInherit {
		path.StrokeJoin = gui.SvgMiterJoin
	}
	if inh.StrokeDasharray != nil {
		path.StrokeDasharray = inh.StrokeDasharray
	}
	if inh.StrokeDashOffsetSet {
		path.StrokeDashOffset = inh.StrokeDashOffset
	}

	if path.GroupID == "" && inh.GroupID != "" {
		path.GroupID = inh.GroupID
	}

	// Mirror cascade-resolved opacity scalars onto path so the
	// gradient tessellator (which composes opacity per-vertex rather
	// than baking into Color.A) reads the same values as
	// bakePathOpacity's solid-color path.
	path.Opacity = inh.Opacity
	path.FillOpacity = inh.FillOpacity
	path.StrokeOpacity = inh.StrokeOpacity

	bakePathOpacity(path, inh)
	path.Computed = inh
}

// bakePathOpacity folds the cascade-resolved opacity values into
// FillColor.A and StrokeColor.A. inh.Opacity already includes the
// element's own opacity multiplied through ancestors. Skip flags
// on the inherited style override the corresponding multiplier with
// 1 so an inline SMIL animation can supply that channel at render
// time without being clipped to zero by the static value
// (e.g. fill-opacity="0").
func bakePathOpacity(path *VectorPath, inh ComputedStyle) {
	// visibility:hidden suppresses paint without removing the element
	// from the box tree. Force fill+stroke alpha to zero so tessellate
	// drops the path and the gradient compositor sees zero opacity.
	if inh.Visibility == VisibilityHidden {
		path.FillColor = applyOpacity(path.FillColor, 0)
		path.StrokeColor = applyOpacity(path.StrokeColor, 0)
		path.Opacity = 0
		path.FillOpacity = 0
		path.StrokeOpacity = 0
		return
	}
	combinedOpacity := inh.Opacity
	if inh.SkipOpacity {
		combinedOpacity = 1
	}
	fillOpacity := inh.FillOpacity
	if inh.SkipFillOpacity {
		fillOpacity = 1
	}
	strokeOpacity := inh.StrokeOpacity
	if inh.SkipStrokeOpacity {
		strokeOpacity = 1
	}
	// Sentinel colors (colorInherit, colorCurrent) carry tiny A
	// markers that would multiply to uint8(0) under common static
	// opacities (e.g. 0.083) and cause tessellate to drop the path.
	// Bump to opaque before baking so the final alpha reflects the
	// element's real opacity. Sentinel RGB (255,0,255) survives —
	// render-side tint still replaces RGB wholesale.
	fillCol := path.FillColor
	if isSentinelColor(fillCol) {
		fillCol.A = 255
	}
	strokeCol := path.StrokeColor
	if isSentinelColor(strokeCol) {
		strokeCol.A = 255
	}
	path.FillColor = applyOpacity(fillCol, clampOpacity01(combinedOpacity*fillOpacity))
	path.StrokeColor = applyOpacity(strokeCol, clampOpacity01(combinedOpacity*strokeOpacity))
}
