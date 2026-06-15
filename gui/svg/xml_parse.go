package svg

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// parseSvgContent walks n's children, emitting VectorPaths for
// shape/group elements and pushing animations onto state. Recurses
// into <g>/<a> groups with merged styles; defs children are skipped
// (defs pre-pass already ran). ancestors is the css.ElementInfo
// chain from root through n; combinator and :nth-child evaluation
// reads it during cascade. Returns the accumulated path list.
//
//nolint:gocyclo // SVG element switch
func parseSvgContent(n *xmlNode, inherited ComputedStyle, depth int,
	state *parseState, ancestors []css.ElementInfo) []VectorPath {
	var paths []VectorPath
	if depth > maxGroupDepth {
		return paths
	}
	siblings := make([]css.ElementInfo, 0, len(n.Children))
	for i := range n.Children {
		if state.elemCount >= maxElements {
			break
		}
		c := &n.Children[i]
		info := makeElementInfo(c.Name, c.OpenTag, i+1, false, c.AttrMap)
		applyPseudoState(&info, state)
		// sibsForThis captures preceding-sibling state at this element's
		// position. siblings then accumulates `info` so the next
		// iteration sees the current element as a preceding sibling.
		sibsForThis := siblings
		siblings = append(siblings, info)
		switch c.Name {
		case "defs":
			// Already handled by defs pre-pass; sibling tracking above
			// keeps document order intact for combinators.

		case "g", "a":
			paths = append(paths,
				parseGroupOrLinkElement(c, inherited, depth, state,
					info, ancestors, sibsForThis)...)

		case "path":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parsePathWithStyle(c.OpenTag, gs)
				})

		case "rect":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parseRectWithStyle(c.OpenTag, gs)
				})

		case "circle":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parseCircleWithStyle(c.OpenTag, gs)
				})

		case "ellipse":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parseEllipseWithStyle(c.OpenTag, gs)
				})

		case "polygon":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parsePolygonWithStyle(c.OpenTag, gs, true)
				})

		case "polyline":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parsePolygonWithStyle(c.OpenTag, gs, false)
				})

		case "line":
			parseShapeElement(c, inherited, state, info, ancestors,
				sibsForThis, &paths,
				func(gs ComputedStyle) (VectorPath, bool) {
					return parseLineWithStyle(c.OpenTag, gs)
				})

		case "text":
			parseTextContentElement(c, inherited, state, info,
				ancestors, sibsForThis)

		case "animate":
			parseAnimationElement(c.OpenTag, inherited, state,
				func() (gui.SvgAnimation, bool) {
					return parseAnimateElement(c.OpenTag, inherited)
				})

		case "animateTransform":
			parseAnimationElement(c.OpenTag, inherited, state,
				func() (gui.SvgAnimation, bool) {
					return parseAnimateTransformElement(
						c.OpenTag, inherited)
				})

		case "set":
			parseAnimationElement(c.OpenTag, inherited, state,
				func() (gui.SvgAnimation, bool) {
					return parseSetElement(c.OpenTag, inherited)
				})

		case "animateMotion":
			parseMotionAnimationElement(c, inherited, state)

		case "svg":
			paths = append(paths,
				parseNestedSvgElement(c, inherited, depth, state,
					info, ancestors, sibsForThis)...)

		default:
			// Unknown element: ignore. (Descendants also ignored —
			// would need explicit handling.)
		}
	}
	return paths
}

// parseGroupOrLinkElement handles <g> and <a> elements.
func parseGroupOrLinkElement(c *xmlNode, inherited ComputedStyle, depth int,
	state *parseState, info css.ElementInfo,
	ancestors, sibsForThis []css.ElementInfo) []VectorPath {
	gs := computeStyle(c.OpenTag, inherited, state, info, ancestors, sibsForThis)
	if gs.Display == DisplayNone {
		return nil
	}
	state.elemCount++
	// Synthesize a GroupID when the group has no id of its own
	// but carries an animation source — inline SMIL children or
	// a CSS animation-name. Descendants then bind via the
	// groupParent chain so resolveAnimationTargets can fan
	// group-level anims onto every child path.
	hasCSSAnim := gs.Animation.Name != ""
	needsGroupBinding := nodeHasInlineAnimation(c) || hasCSSAnim
	if gs.GroupID == inherited.GroupID && needsGroupBinding {
		gs.GroupID = state.synthGroupID()
	}
	if gs.GroupID != "" && gs.GroupID != inherited.GroupID {
		state.recordGroupParent(gs.GroupID, inherited.GroupID)
	}
	childAncestors := make([]css.ElementInfo, len(ancestors), len(ancestors)+1)
	copy(childAncestors, ancestors)
	childAncestors = append(childAncestors, info)
	animStart := len(state.animations)
	var paths []VectorPath
	pathStart := len(paths)
	paths = append(paths,
		parseSvgContent(c, gs, depth+1, state, childAncestors)...)
	if hasCSSAnim && pathStart < len(paths) {
		groupBox := unionPathBboxes(paths[pathStart:])
		compileCSSAnimations(gs.Animation, 0,
			gs.TransformOrigin, groupBox, gs, state)
		for ai := animStart; ai < len(state.animations); ai++ {
			a := &state.animations[ai]
			if a.GroupID == "" {
				a.GroupID = gs.GroupID
			}
			a.TargetPathIDs = nil
		}
	}
	return paths
}

// parseNestedSvgElement handles nested <svg> elements.
func parseNestedSvgElement(c *xmlNode, inherited ComputedStyle, depth int,
	state *parseState, info css.ElementInfo,
	ancestors, sibsForThis []css.ElementInfo) []VectorPath {
	gs := computeStyle(c.OpenTag, inherited, state, info,
		ancestors, sibsForThis)
	if gs.Display == DisplayNone {
		return nil
	}
	state.elemCount++
	innerVB, outerVP, viewportTx := computeNestedSvgViewport(
		c.AttrMap, state.curViewport)
	gs.Transform = matrixMultiply(gs.Transform, viewportTx)
	savedVP := state.curViewport
	// Authored = this element declared clip-path via any cascade
	// origin (presentation attr / CSS / inline style), as
	// opposed to inheriting from the parent. Inner nested <svg>s
	// without their own clip still receive a fresh synth clip
	// (innermost wins). Catches the "redeclared same id as
	// parent" case that pure value comparison would miss.
	authoredClip := gs.AuthoredClipPath && gs.ClipPathID != ""
	if !authoredClip && state.vg != nil &&
		len(c.Children) > 0 && outerVP.clippable() {
		cid := state.synthNestedClipID()
		state.vg.ClipPaths[cid] = []VectorPath{{
			Segments: segmentsForRect(
				outerVP.X, outerVP.Y, outerVP.W, outerVP.H, 0, 0),
			Transform: identityTransform,
		}}
		gs.ClipPathID = cid
	}
	state.curViewport = innerVB
	childAncestors := make([]css.ElementInfo, len(ancestors), len(ancestors)+1)
	copy(childAncestors, ancestors)
	childAncestors = append(childAncestors, info)
	var paths []VectorPath
	paths = append(paths,
		parseSvgContent(c, gs, depth+1, state, childAncestors)...)
	state.curViewport = savedVP
	return paths
}

// parseShapeElement dispatches a shape element (path, rect, circle, etc.)
// via appendShape.
func parseShapeElement(
	c *xmlNode,
	inherited ComputedStyle,
	state *parseState,
	info css.ElementInfo,
	ancestors []css.ElementInfo,
	siblings []css.ElementInfo,
	paths *[]VectorPath,
	parser func(gs ComputedStyle) (VectorPath, bool),
) {
	appendShape(c, inherited, state, info, ancestors, siblings, paths, parser)
}

// parseAnimationElement handles <animate>, <animateTransform>, and <set>.
func parseAnimationElement(
	openTag string,
	inherited ComputedStyle,
	state *parseState,
	parser func() (gui.SvgAnimation, bool),
) {
	state.elemCount++
	if len(state.animations) < maxAnimations {
		if a, ok := parser(); ok {
			state.animations = append(state.animations, a)
			registerAnimation(state, openTag, len(state.animations)-1)
		}
	}
}

// parseMotionAnimationElement handles <animateMotion>.
func parseMotionAnimationElement(
	c *xmlNode,
	inherited ComputedStyle,
	state *parseState,
) {
	state.elemCount++
	if len(state.animations) < maxAnimations {
		if a, ok := parseAnimateMotionElement(c, inherited, state); ok {
			state.animations = append(state.animations, a)
			registerAnimation(state, c.OpenTag, len(state.animations)-1)
		}
	}
}

// parseTextContentElement handles <text> elements.
func parseTextContentElement(
	c *xmlNode,
	inherited ComputedStyle,
	state *parseState,
	info css.ElementInfo,
	ancestors, sibsForThis []css.ElementInfo,
) {
	// Run cascade so author CSS, :hover/:focus, and display:none
	// reach <text> the same way they reach shapes.
	textGS := computeStyle(c.OpenTag, inherited, state, info,
		ancestors, sibsForThis)
	if textGS.Display == DisplayNone {
		return
	}
	state.elemCount++
	textAncestors := make([]css.ElementInfo, len(ancestors), len(ancestors)+1)
	copy(textAncestors, ancestors)
	textAncestors = append(textAncestors, info)
	parseTextElement(c, textGS, state, textAncestors)
}

// appendShape wraps parseShapeElement's original bookkeeping: the
// shape parser runs with an optionally-synthesized GroupID if the
// node carries inline animation children, then inline anims are
// folded onto the path's state.
func appendShape(
	c *xmlNode,
	inherited ComputedStyle,
	state *parseState,
	info css.ElementInfo,
	ancestors []css.ElementInfo,
	siblings []css.ElementInfo,
	paths *[]VectorPath,
	parser func(gs ComputedStyle) (VectorPath, bool),
) {
	// Always run the cascade for the shape so pres-attrs, author
	// rules, and inline style are layered with the spec-correct
	// precedence. Pin GroupID back to the parent's value — the
	// inline-animation branch below owns shape-level GroupID
	// assignment.
	shapeGS := computeStyle(c.OpenTag, inherited, state, info, ancestors, siblings)
	if shapeGS.Display == DisplayNone {
		return
	}
	state.elemCount++
	shapeGS.GroupID = inherited.GroupID
	if nodeHasInlineAnimation(c) {
		gid := c.AttrMap["id"]
		if gid == "" {
			gid = state.synthGroupID()
		}
		shapeGS.GroupID = gid
		if gid != inherited.GroupID {
			state.recordGroupParent(gid, inherited.GroupID)
		}
		all, fill, stroke := scanOpacityAnimTargets(c)
		shapeGS.SkipOpacity = all
		shapeGS.SkipFillOpacity = fill
		shapeGS.SkipStrokeOpacity = stroke
	}

	pathIdx := -1
	if p, ok := parser(shapeGS); ok {
		if shapeGS.GroupID != inherited.GroupID {
			p.GroupID = shapeGS.GroupID
		}
		state.pathIDSeq++
		p.PathID = state.pathIDSeq
		*paths = append(*paths, p)
		pathIdx = len(*paths) - 1
	}

	animStart := len(state.animations)
	if pathIdx >= 0 && shapeGS.Animation.Name != "" {
		compileCSSAnimations(shapeGS.Animation,
			(*paths)[pathIdx].PathID,
			shapeGS.TransformOrigin,
			(*paths)[pathIdx].Bbox, shapeGS, state)
	}
	parseShapeInlineChildren(c, shapeGS, state)
	// Clip-pathed shapes skip re-tessellation.
	if pathIdx >= 0 && (*paths)[pathIdx].ClipPathID == "" {
		for i := animStart; i < len(state.animations); i++ {
			k := state.animations[i].Kind
			if k == gui.SvgAnimAttr ||
				k == gui.SvgAnimDashArray ||
				k == gui.SvgAnimDashOffset {
				(*paths)[pathIdx].Animated = true
				break
			}
		}
	}
}

// parseAnimateForDispatch picks the right <animate> parser based on
// attributeName: opacity/fill-opacity/stroke-opacity → opacity;
// stroke-dasharray → dash array; stroke-dashoffset → dash offset;
// primitive attrs (cx/cy/r/...) → attribute animation. Unknown names
// reject (ok=false).
func parseAnimateForDispatch(
	elem string, inherited ComputedStyle,
) (gui.SvgAnimation, bool) {
	attr, ok := findAttr(elem, "attributeName")
	if !ok {
		return gui.SvgAnimation{}, false
	}
	switch attr {
	case "opacity", "fill-opacity", "stroke-opacity":
		return parseAnimateElement(elem, inherited)
	case "stroke-dasharray":
		return parseAnimateDashArrayElement(elem, inherited)
	case "stroke-dashoffset":
		return parseAnimateDashOffsetElement(elem, inherited)
	}
	return parseAnimateAttributeElement(elem, inherited)
}

// nodeHasInlineAnimation reports whether any direct child of n is a
// SMIL animation element.
func nodeHasInlineAnimation(n *xmlNode) bool {
	for i := range n.Children {
		switch n.Children[i].Name {
		case "animate", "animateTransform", "animateMotion", "set":
			return true
		}
	}
	return false
}

// scanOpacityAnimTargets reports which opacity sub-attributes are
// animated by inline <animate> children of a shape. Used to suppress
// static opacity baking for channels the animation will overwrite at
// render time.
func scanOpacityAnimTargets(n *xmlNode) (all, fill, stroke bool) {
	for i := range n.Children {
		c := &n.Children[i]
		if c.Name != "animate" {
			continue
		}
		switch c.AttrMap["attributeName"] {
		case "opacity":
			all = true
		case "fill-opacity":
			fill = true
		case "stroke-opacity":
			stroke = true
		}
	}
	return
}

// parseShapeInlineChildren walks a shape's children for
// <animate>/<animateTransform>/<animateMotion>/<set> and appends
// them to state.animations keyed by shapeGS.GroupID.
func parseShapeInlineChildren(
	n *xmlNode, shapeGS ComputedStyle, state *parseState,
) {
	for i := range n.Children {
		c := &n.Children[i]
		switch c.Name {
		case "animate":
			if len(state.animations) < maxAnimations {
				if a, ok := parseAnimateForDispatch(c.OpenTag, shapeGS); ok {
					state.animations = append(state.animations, a)
					registerAnimation(state, c.OpenTag,
						len(state.animations)-1)
				}
			}
		case "animateTransform":
			if len(state.animations) < maxAnimations {
				if a, ok := parseAnimateTransformElement(
					c.OpenTag, shapeGS); ok {
					state.animations = append(state.animations, a)
					registerAnimation(state, c.OpenTag,
						len(state.animations)-1)
				}
			}
		case "set":
			if len(state.animations) < maxAnimations {
				if a, ok := parseSetElement(c.OpenTag, shapeGS); ok {
					state.animations = append(state.animations, a)
					registerAnimation(state, c.OpenTag,
						len(state.animations)-1)
				}
			}
		case "animateMotion":
			if len(state.animations) < maxAnimations {
				if a, ok := parseAnimateMotionElement(
					c, shapeGS, state); ok {
					state.animations = append(state.animations, a)
					registerAnimation(state, c.OpenTag,
						len(state.animations)-1)
				}
			}
		}
	}
}
