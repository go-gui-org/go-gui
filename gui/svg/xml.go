package svg

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// maxElementIDLen caps pseudo-state element IDs forwarded into the
// cascade. SVG ids in practice are short (icon-set IDs rarely exceed
// 64 bytes); a cap protects the cache key, hash mixer, and per-
// element ID compare from a hostile multi-MB id.
const maxElementIDLen = 256

// maxFlatnessTolerance caps the upper bound on user-supplied
// FlatnessTolerance. Beyond a few units the tessellator collapses
// curves to chords already; refusing absurd values keeps the floor
// math, cache keying, and hash mixing on safe ranges.
const maxFlatnessTolerance float32 = 64

// sanitizeFlatness drops NaN/Inf/negative inputs and caps the upper
// bound. Returning 0 disables the override and falls back to the
// built-in 0.15 floor.
func sanitizeFlatness(t float32) float32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) || t <= 0 {
		return 0
	}
	if t > maxFlatnessTolerance {
		return maxFlatnessTolerance
	}
	return t
}

// clampElementID truncates s to maxElementIDLen UTF-8 bytes. Cheap
// byte slice; the cascade compares IDs by exact match so trimming a
// hostile string still produces correct (no-match) behavior.
func clampElementID(s string) string {
	if len(s) > maxElementIDLen {
		return s[:maxElementIDLen]
	}
	return s
}

// ParseOptions controls environment-dependent parsing behavior.
// PrefersReducedMotion is the snapshot fed to
// `@media (prefers-reduced-motion: reduce)` evaluation; see
// docs/svg-css-design.md "prefers-reduced-motion".
// FlatnessTolerance overrides the tessellation tolerance floor when
// > 0. HoveredElementID / FocusedElementID feed :hover / :focus
// pseudo-class matching during the cascade.
type ParseOptions struct {
	HoveredElementID     string
	FocusedElementID     string
	FlatnessTolerance    float32
	PrefersReducedMotion bool
}

// parseSvg parses an SVG string and returns a VectorGraphic.
func parseSvg(content string) (*VectorGraphic, error) {
	return parseSvgWith(content, ParseOptions{})
}

// parseSvgWith is the options-aware variant of parseSvg. opts is
// snapshotted into the cascade (e.g. for @media reduced-motion).
func parseSvgWith(content string, opts ParseOptions) (*VectorGraphic, error) {
	if len(content) > maxSvgFileSize {
		return nil, fmt.Errorf("svg: content too large: %d bytes", len(content))
	}
	root, err := decodeSvgTree(content)
	if err != nil {
		return nil, err
	}
	expandUseElements(root)

	vg := &VectorGraphic{
		Width:  defaultIconSize,
		Height: defaultIconSize,
	}

	// viewBox on root. Fall back to lowercase "viewbox" — SVG-in-HTML
	// authoring (and several svg-spinners assets) ship the attribute
	// lowercased; XHTML strict-mode is rare in the wild.
	vb, ok := root.AttrMap["viewBox"]
	if !ok {
		vb, ok = root.AttrMap["viewbox"]
	}
	if ok {
		nums := parseNumberList(vb)
		if len(nums) >= 4 {
			vg.ViewBoxX = nums[0]
			vg.ViewBoxY = nums[1]
			vg.Width = clampViewBoxDim(nums[2])
			vg.Height = clampViewBoxDim(nums[3])
		}
	} else {
		if w, ok := root.AttrMap["width"]; ok {
			vg.Width = clampViewBoxDim(parseLength(w))
		}
		if h, ok := root.AttrMap["height"]; ok {
			vg.Height = clampViewBoxDim(parseLength(h))
		}
	}

	vg.A11y = parseRootA11y(root)
	vg.PreserveAlign, vg.PreserveSlice = parsePreserveAspectRatio(
		root.AttrMap["preserveAspectRatio"])

	// Pre-pass: extract <defs>.
	vg.ClipPaths = parseDefsClipPaths(root)
	vg.Gradients = parseDefsGradients(root)
	vg.Filters = parseDefsFilters(root)
	vg.DefsPaths = parseDefsPaths(root)

	// viewBox offset is applied at render time; triangles, animation
	// centers, and motion paths all stay in raw viewBox space.
	sheet := css.ParseFull(collectStyleBlocks(root), css.ParseOptions{
		PrefersReducedMotion: opts.PrefersReducedMotion,
	})
	state := &parseState{
		defsPaths:    vg.DefsPaths,
		cssRules:     sheet.Rules,
		cssKeyframes: sheet.Keyframes,
		hoveredID:    clampElementID(opts.HoveredElementID),
		focusedID:    clampElementID(opts.FocusedElementID),
		curViewport: viewportRect{
			X: vg.ViewBoxX, Y: vg.ViewBoxY,
			W: vg.Width, H: vg.Height,
		},
	}
	state.vg = vg
	vg.FlatnessTolerance = sanitizeFlatness(opts.FlatnessTolerance)
	// Merge presentation attributes (and matched author rules, when
	// any) from the root <svg> tag so shapes that inherit e.g.
	// fill="currentColor" pick it up.
	rootInfo := makeElementInfo("svg", root.OpenTag, 1, true, root.AttrMap)
	applyPseudoState(&rootInfo, state)
	defStyle := computeStyle(root.OpenTag,
		defaultComputedStyle(identityTransform), state, rootInfo, nil, nil)
	ancestors := []css.ElementInfo{rootInfo}
	allPaths := parseSvgContent(root, defStyle, 0, state, ancestors)

	// Separate filtered paths from main paths.
	if len(vg.Filters) > 0 {
		// Bucket by per-occurrence key: non-contiguous filter uses must
		// composite separately or z-order against unfiltered siblings
		// between them is wrong. Map records first-seen index per key
		// so groups stay in document order across map iteration.
		idx := map[uint32]int{}
		for _, p := range allPaths {
			key := p.FilterGroupKey
			if key == 0 || p.FilterID == "" {
				vg.Paths = append(vg.Paths, p)
				continue
			}
			if _, ok := vg.Filters[p.FilterID]; !ok {
				vg.Paths = append(vg.Paths, p)
				continue
			}
			i, ok := idx[key]
			if !ok {
				i = len(vg.FilteredGroups)
				idx[key] = i
				vg.FilteredGroups = append(vg.FilteredGroups,
					svgFilteredGroup{FilterID: p.FilterID, GroupKey: key})
			}
			vg.FilteredGroups[i].Paths = append(
				vg.FilteredGroups[i].Paths, p)
		}
		for _, t := range state.texts {
			key := t.FilterGroupKey
			if key == 0 || t.FilterID == "" {
				vg.Texts = append(vg.Texts, t)
				continue
			}
			if _, ok := vg.Filters[t.FilterID]; !ok {
				vg.Texts = append(vg.Texts, t)
				continue
			}
			gi, ok := idx[key]
			if !ok {
				gi = len(vg.FilteredGroups)
				idx[key] = gi
				vg.FilteredGroups = append(vg.FilteredGroups,
					svgFilteredGroup{FilterID: t.FilterID, GroupKey: key})
			}
			vg.FilteredGroups[gi].Texts = append(
				vg.FilteredGroups[gi].Texts, t)
		}
		for _, tp := range state.textPaths {
			key := tp.FilterGroupKey
			if key == 0 || tp.FilterID == "" {
				vg.TextPaths = append(vg.TextPaths, tp)
				continue
			}
			if _, ok := vg.Filters[tp.FilterID]; !ok {
				vg.TextPaths = append(vg.TextPaths, tp)
				continue
			}
			gi, ok := idx[key]
			if !ok {
				gi = len(vg.FilteredGroups)
				idx[key] = gi
				vg.FilteredGroups = append(vg.FilteredGroups,
					svgFilteredGroup{FilterID: tp.FilterID, GroupKey: key})
			}
			vg.FilteredGroups[gi].TextPaths = append(
				vg.FilteredGroups[gi].TextPaths, tp)
		}
	} else {
		vg.Paths = allPaths
		vg.Texts = state.texts
		vg.TextPaths = state.textPaths
	}

	vg.Animations = state.animations
	vg.GroupParent = state.groupParent
	resolveBegins(vg.Animations, state.animBeginSpecs, state.animIDIndex)
	return vg, nil
}

const maxSvgFileSize = 4 << 20 // 4 MB

// parseSvgFile loads and parses an SVG file.
func parseSvgFile(path string) (*VectorGraphic, error) {
	data, err := loadSvgFile(path)
	if err != nil {
		return nil, err
	}
	return parseSvg(string(data))
}

func loadSvgFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read SVG file: %w", err)
	}
	if info.Size() > maxSvgFileSize {
		return nil, fmt.Errorf("SVG file too large: %d bytes", info.Size())
	}
	// #nosec G304 — path validated by caller through AllowedSvgRoots
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SVG file: %w", err)
	}
	return data, nil
}

// parseSvgDimensions extracts only width/height without a full
// parse. Operates on the raw string so callers can probe dimensions
// on incomplete or fragment-only SVG content (no closing tag
// required).
func parseSvgDimensions(content string) (float32, float32) {
	// Cap probe input. Caller-supplied string of arbitrary length
	// would force extractRootSVGOpenTag/findAttr to scan unbounded
	// memory; the dimension probe never needs more than the root
	// open tag, well under maxSvgFileSize.
	if len(content) > maxSvgFileSize {
		content = content[:maxSvgFileSize]
	}
	openTag := extractRootSVGOpenTag(content)
	if openTag == "" {
		openTag = content
	}
	vb, ok := findAttr(openTag, "viewBox")
	if !ok {
		vb, ok = findAttr(openTag, "viewbox")
	}
	if ok {
		nums := parseNumberList(vb)
		if len(nums) >= 4 {
			return clampViewBoxDim(nums[2]), clampViewBoxDim(nums[3])
		}
	}
	w := float32(defaultIconSize)
	h := float32(defaultIconSize)
	if ws, ok := findAttr(openTag, "width"); ok {
		w = clampViewBoxDim(parseLength(ws))
	}
	if hs, ok := findAttr(openTag, "height"); ok {
		h = clampViewBoxDim(parseLength(hs))
	}
	return w, h
}

func extractRootSVGOpenTag(content string) string {
	start := strings.Index(content, "<svg")
	if start < 0 {
		return ""
	}
	nameEnd := start + len("<svg")
	if nameEnd < len(content) {
		switch content[nameEnd] {
		case '>', '/', ' ', '\t', '\n', '\r':
		default:
			return ""
		}
	}
	inQuote := byte(0)
	for i := nameEnd; i < len(content); i++ {
		switch c := content[i]; c {
		case '"', '\'':
			switch inQuote {
			case 0:
				inQuote = c
			case c:
				inQuote = 0
			}
		case '>':
			if inQuote == 0 {
				return content[start : i+1]
			}
		}
	}
	return content[start:]
}

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

// mintSynthID bumps n and returns prefix+N. Callers split counters
// per id namespace so concurrent prefixes never alias on the integer.
func mintSynthID(prefix string, n *int) string {
	*n++
	return prefix + strconv.Itoa(*n)
}

func (s *parseState) synthGroupID() string {
	return mintSynthID("__anim_", &s.synthID)
}

func (s *parseState) synthNestedClipID() string {
	return mintSynthID(synthNestedClipPrefix, &s.synthClipID)
}

// recordGroupParent registers child→parent in the GroupParent edge
// map, lazy-initing the map on first write.
func (s *parseState) recordGroupParent(child, parent string) {
	if s.groupParent == nil {
		s.groupParent = make(map[string]string, 16)
	}
	s.groupParent[child] = parent
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
