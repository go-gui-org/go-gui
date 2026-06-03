package svg

// xml_use.go — resolves SVG <use href="#id"> references by inlining
// a clone of the target subtree. <symbol> targets surface as their
// children wrapped in a synthesized <g>; cycles are guarded by a
// per-path visited set and a hard depth cap.

import (
	"maps"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// maxUseDepth bounds nested <use> resolution. Real-world SVGs nest
// 1–3 deep (icon sets that use symbol-of-symbol composition).
const maxUseDepth = 8

// maxUseExpandClones caps the total node count produced by <use>
// expansion. The decode-side maxElements limit bounds the source
// tree; this companion budget bounds the post-expansion blowup so a
// single 1KB <symbol> referenced 10 000 times cannot multiply into a
// hundred-million-node tree before any later pass can reject it.
const maxUseExpandClones = maxElements

// synthUseClipPrefix is the id stem for clipPaths minted by
// mintUseSliceClipID. Mirrors synthNestedClipPrefix in xml_nested_svg.go.
const synthUseClipPrefix = "__use_clip_"

// expandUseElements rewrites root in place: every <use href="#id">
// element is replaced with a synthesized <g> that wraps a clone of
// the referenced subtree. The clone has its id stripped so the
// post-expansion tree carries no duplicate ids. Use targets that
// require a clip-to-use-box (a <symbol> referenced via <use> with
// preserveAspectRatio="... slice") get a synthesized <clipPath>
// emitted into a defs node appended to root, so spec-required slice
// cropping holds.
func expandUseElements(root *xmlNode) {
	if root == nil || !hasUseElement(root) {
		return
	}
	idIndex := make(map[string]*xmlNode)
	indexByID(root, idIndex)
	visited := make(map[string]struct{}, 4)
	budget := maxUseExpandClones
	ctx := &useCtx{idIndex: idIndex, visited: visited, budget: &budget}
	expandUseRec(root, ctx, 0)
	if len(ctx.clipDefs) > 0 {
		root.Children = append(root.Children, makeUseClipDefs(ctx.clipDefs))
	}
}

// useCtx threads expansion state (id index, cycle set, clone budget)
// plus a registry of synthesized use-box clipPaths.
type useCtx struct {
	idIndex    map[string]*xmlNode
	visited    map[string]struct{}
	budget     *int
	clipDefs   []xmlNode
	nextClipID int
}

// mintUseClipID returns a fresh synth clipPath id that does not
// collide with any author-supplied id already in the document.
// Without the idIndex check an authored id="__use_clip_1" would
// either shadow the synth rect (redirecting authored url(#…)
// references) or be shadowed by it. Synth ids are monotonically
// numbered so two synth ids can never collide with each other —
// only authored collisions need to be skipped.
func (c *useCtx) mintUseClipID() string {
	for {
		c.nextClipID++
		id := synthUseClipPrefix + strconv.Itoa(c.nextClipID)
		if _, taken := c.idIndex[id]; !taken {
			return id
		}
	}
}

// makeUseClipDefs wraps the synthesized <clipPath> nodes in a single
// <defs> for parseDefsClipPaths to pick up via its ordinary walk.
func makeUseClipDefs(clipPaths []xmlNode) xmlNode {
	defs := xmlNode{
		Name:     "defs",
		Children: clipPaths,
	}
	defs.OpenTag = buildOpenTag("defs", nil, false)
	return defs
}

// hasUseElement returns true if the subtree contains any <use> node.
// Most icon SVGs have none; skipping the id-index walk keeps parse
// hot path lean.
func hasUseElement(n *xmlNode) bool {
	for i := range n.Children {
		c := &n.Children[i]
		if c.Name == "use" {
			return true
		}
		if hasUseElement(c) {
			return true
		}
	}
	return false
}

func indexByID(n *xmlNode, out map[string]*xmlNode) {
	if id := n.AttrMap["id"]; id != "" {
		if _, exists := out[id]; !exists {
			out[id] = n
		}
	}
	for i := range n.Children {
		indexByID(&n.Children[i], out)
	}
}

func expandUseRec(n *xmlNode, ctx *useCtx, depth int) {
	for i := range n.Children {
		if *ctx.budget <= 0 {
			return
		}
		c := &n.Children[i]
		if c.Name == "use" {
			if depth >= maxUseDepth {
				continue
			}
			href := c.AttrMap["href"]
			if href == "" {
				href = c.AttrMap["xlink:href"]
			}
			if !strings.HasPrefix(href, "#") || len(href) <= 1 {
				continue
			}
			id := href[1:]
			if _, cyc := ctx.visited[id]; cyc {
				continue
			}
			target, ok := ctx.idIndex[id]
			if !ok {
				continue
			}
			ctx.visited[id] = struct{}{}
			*c = synthesizeUseGroup(c, target, ctx)
			expandUseRec(c, ctx, depth+1)
			delete(ctx.visited, id)
			continue
		}
		expandUseRec(c, ctx, depth)
	}
}

// synthesizeUseGroup builds a <g> wrapper that replaces a <use>
// element. The wrapper carries the <use>'s presentation attrs (fill,
// class, style, ...) so they cascade into the inlined clone, plus a
// translate(x,y) transform composed with any author-supplied
// transform on the <use>. <symbol> targets contribute their children;
// other targets are inlined as a single cloned subtree.
func synthesizeUseGroup(useNode, target *xmlNode, ctx *useCtx) xmlNode {
	x := useNode.AttrMap["x"]
	y := useNode.AttrMap["y"]

	var children []xmlNode
	if target.Name == "symbol" {
		children = cloneChildrenForUse(target.Children, ctx.budget)
	} else {
		clone := cloneNode(target, ctx.budget)
		stripID(&clone)
		children = []xmlNode{clone}
	}

	gAttrs := make([]xmlAttr, 0, len(useNode.Attrs))
	gAttrMap := make(map[string]string, len(useNode.AttrMap))
	for _, a := range useNode.Attrs {
		switch a.Name {
		case "x", "y", "href", "xlink:href", "id", "width", "height":
			continue
		}
		gAttrs = append(gAttrs, a)
		gAttrMap[a.Name] = a.Value
	}

	posTransform := positioningTransform(useNode, target, x, y)
	if posTransform != "" {
		composed := posTransform
		if existing, ok := gAttrMap["transform"]; ok {
			composed = existing + " " + posTransform
		}
		gAttrMap["transform"] = composed
		replaced := false
		for i := range gAttrs {
			if gAttrs[i].Name == "transform" {
				gAttrs[i].Value = composed
				replaced = true
				break
			}
		}
		if !replaced {
			gAttrs = append(gAttrs,
				xmlAttr{Name: "transform", Value: composed})
		}
	}

	if cid, ok := mintUseSliceClipID(useNode, target, ctx); ok {
		// Authored clip-path on the <use> wins per cascade; only set
		// the synth clip when the <use> doesn't already declare one.
		if _, has := gAttrMap["clip-path"]; !has {
			ref := "url(#" + cid + ")"
			gAttrMap["clip-path"] = ref
			gAttrs = append(gAttrs,
				xmlAttr{Name: "clip-path", Value: ref})
		}
	}

	g := xmlNode{
		Name:     "g",
		Attrs:    gAttrs,
		AttrMap:  gAttrMap,
		Children: children,
	}
	g.OpenTag = buildOpenTag(g.Name, g.Attrs, false)
	return g
}

// mintUseSliceClipID emits a synth <clipPath> covering the <use>'s
// (x, y, width, height) box when the target is a <symbol> with
// preserveAspectRatio="... slice" — the SVG spec requires that
// content overflowing the use box be clipped, otherwise slice
// scaling produces visible overflow. Returns the synth clip id
// (already registered in ctx.clipDefs) and ok=true when a clip
// applies. The clipPath default clipPathUnits="userSpaceOnUse"
// resolves the rect in the parent's coordinate system, matching the
// space the <g>'s clip-path is referenced from.
func mintUseSliceClipID(
	useNode, target *xmlNode, ctx *useCtx,
) (string, bool) {
	if useNode == nil || target == nil || target.Name != "symbol" {
		return "", false
	}
	_, slice := effectivePreserveAspectRatio(target)
	if !slice {
		return "", false
	}
	// Use box dimensions: mirror symbolViewportScale's fallback so the
	// clip rect matches the box the same code path scales content into.
	// When width/height absent on the <use>, fall back to the symbol's
	// viewBox dim (scale on that axis = 1, but the other axis can still
	// overflow under slice scaling — the clip still has to bound it).
	useW := useNode.AttrMap["width"]
	useH := useNode.AttrMap["height"]
	if useW == "" && useH == "" {
		return "", false
	}
	if hasPercent(useW) || hasPercent(useH) {
		return "", false
	}
	vb := target.AttrMap["viewBox"]
	if vb == "" {
		vb = target.AttrMap["viewbox"]
	}
	vbNums := parseNumberList(vb)
	// `<= 0` admits NaN; reject explicitly so an Inf/NaN viewBox never
	// flows into the fallback below. Also bound vb width/height to
	// boundedScale so downstream rect coords stay finite.
	if len(vbNums) < 4 ||
		!finiteF32(vbNums[2]) || !finiteF32(vbNums[3]) ||
		vbNums[2] <= 0 || vbNums[3] <= 0 ||
		!boundedScale(vbNums[2]) || !boundedScale(vbNums[3]) {
		return "", false
	}
	uw := parseLength(useW)
	uh := parseLength(useH)
	if useW == "" {
		uw = vbNums[2]
	}
	if useH == "" {
		uh = vbNums[3]
	}
	if !(uw > 0) || !(uh > 0) || !boundedScale(uw) || !boundedScale(uh) {
		return "", false
	}
	x := useNode.AttrMap["x"]
	y := useNode.AttrMap["y"]
	if hasPercent(x) || hasPercent(y) {
		return "", false
	}
	tx := parseLength(x)
	ty := parseLength(y)
	if !boundedScale(tx) || !boundedScale(ty) {
		return "", false
	}

	cid := ctx.mintUseClipID()
	xs, ys := f32String(tx), f32String(ty)
	ws, hs := f32String(uw), f32String(uh)
	rect := xmlNode{
		Name:      "rect",
		SelfClose: true,
	}
	rect.Attrs = []xmlAttr{
		{Name: "x", Value: xs},
		{Name: "y", Value: ys},
		{Name: "width", Value: ws},
		{Name: "height", Value: hs},
	}
	rect.AttrMap = map[string]string{
		"x":      xs,
		"y":      ys,
		"width":  ws,
		"height": hs,
	}
	rect.OpenTag = buildOpenTag(rect.Name, rect.Attrs, true)

	cp := xmlNode{
		Name:     "clipPath",
		Children: []xmlNode{rect},
	}
	cp.Attrs = []xmlAttr{{Name: "id", Value: cid}}
	cp.AttrMap = map[string]string{"id": cid}
	cp.OpenTag = buildOpenTag(cp.Name, cp.Attrs, false)
	ctx.clipDefs = append(ctx.clipDefs, cp)
	return cid, true
}

func f32String(v float32) string {
	var buf [32]byte
	return string(strconv.AppendFloat(buf[:0], float64(v), 'f', -1, 32))
}

// positioningTransform returns the SVG transform string that places
// the synthesized <g> at the <use>'s (x,y) and, when the target is a
// <symbol> with a viewBox plus author-specified width/height on the
// <use>, scales the symbol's viewport to fit the requested box per
// preserveAspectRatio.
//
// SVG transform list reads left-to-right but applies right-to-left to
// child points: child p gets translate(-vbX,-vbY) first, then scale,
// then translate(x+ax, y+ay). Non-symbol targets and symbols without a
// viewBox (or without numeric use width/height) get only the
// translate. ax/ay are align offsets; with align=none they're zero
// and the scale is non-uniform — the SVG-1.1 default for <symbol> is
// xMidYMid meet, so authors who relied on stretch must opt in via
// preserveAspectRatio="none" on the symbol.
//
// Slice scaling uses uniform max(sx,sy). The accompanying
// clip-to-use-box that the SVG spec requires is emitted by
// mintUseSliceClipID via expandUseElements, so slice content can't
// overflow the requested width/height.
//
// Parses x/y numerically rather than splicing raw author strings into
// the transform attribute: `<use x="0)scale(99)">` would otherwise
// inject extra transforms. parseLength clamps to ±maxCoordinate and
// rejects NaN/Inf.
func positioningTransform(useNode, target *xmlNode, x, y string) string {
	if useNode == nil || target == nil {
		return ""
	}
	// Percentages on x/y resolve against the parent viewport, which is
	// out of scope here. Drop rather than treat "50%" as raw 50.
	if hasPercent(x) || hasPercent(y) {
		return ""
	}
	wantTranslate := x != "" || y != ""
	tx := parseLength(x)
	ty := parseLength(y)

	sx, sy, vbX, vbY, ax, ay, ok := symbolViewportScale(useNode, target)
	if !ok {
		if !wantTranslate {
			return ""
		}
		return writeTranslate(tx, ty)
	}
	combX, combY := tx+ax, ty+ay
	if !boundedScale(combX) || !boundedScale(combY) {
		return ""
	}
	var b strings.Builder
	b.Grow(64)
	writeTranslateTo(&b, combX, combY)
	switch {
	case sx == sy && sx != 1:
		b.WriteString(" scale(")
		writeF32(&b, sx)
		b.WriteByte(')')
	case sx != 1 || sy != 1:
		b.WriteString(" scale(")
		writeF32(&b, sx)
		b.WriteByte(',')
		writeF32(&b, sy)
		b.WriteByte(')')
	}
	if vbX != 0 || vbY != 0 {
		b.WriteString(" ")
		writeTranslateTo(&b, -vbX, -vbY)
	}
	return b.String()
}

func writeTranslate(x, y float32) string {
	var b strings.Builder
	b.Grow(32)
	writeTranslateTo(&b, x, y)
	return b.String()
}

func writeTranslateTo(b *strings.Builder, x, y float32) {
	b.WriteString("translate(")
	writeF32(b, x)
	b.WriteByte(',')
	writeF32(b, y)
	b.WriteByte(')')
}

func writeF32(b *strings.Builder, v float32) {
	var buf [32]byte
	b.Write(strconv.AppendFloat(buf[:0], float64(v), 'f', -1, 32))
}

// symbolViewportScale returns the scale factors, viewBox origin, and
// alignment offsets needed to map a <symbol viewBox=...>'s viewport
// into the <use>'s width/height box per preserveAspectRatio.
//
// Default is xMidYMid meet (SVG 1.1): uniform min-scale + center.
// preserveAspectRatio="none" on the symbol opts into the legacy
// independent-axis stretch. Slice scaling uses uniform max-scale; the
// accompanying clip-to-box is emitted by mintUseSliceClipID.
//
// ok is false when the target is not a <symbol>, when the symbol has
// no usable viewBox, when use width/height are missing or
// percentage-based, or when the resulting scale would exceed
// ±maxCoordinate (pathological viewBox like 1e-30 against use width=1).
func symbolViewportScale(useNode, target *xmlNode) (
	sx, sy, vbX, vbY, ax, ay float32, ok bool,
) {
	if target.Name != "symbol" {
		return
	}
	vb := target.AttrMap["viewBox"]
	if vb == "" {
		vb = target.AttrMap["viewbox"]
	}
	useW := useNode.AttrMap["width"]
	useH := useNode.AttrMap["height"]
	if vb == "" || (useW == "" && useH == "") {
		return
	}
	if hasPercent(useW) || hasPercent(useH) {
		return
	}
	nums := parseNumberList(vb)
	if len(nums) < 4 || nums[2] <= 0 || nums[3] <= 0 {
		return
	}
	uw := parseLength(useW)
	uh := parseLength(useH)
	if useW == "" {
		uw = nums[2]
	}
	if useH == "" {
		uh = nums[3]
	}
	if uw <= 0 || uh <= 0 {
		return
	}
	rawSx := uw / nums[2]
	rawSy := uh / nums[3]

	align, slice := effectivePreserveAspectRatio(target)
	if align == gui.SvgAlignNone {
		sx, sy = rawSx, rawSy
	} else {
		var s float32
		if slice {
			s = max(rawSx, rawSy)
		} else {
			s = min(rawSx, rawSy)
		}
		sx, sy = s, s
		xFrac, yFrac := gui.PreserveAlignFractions(align)
		ax = xFrac * (uw - nums[2]*s)
		ay = yFrac * (uh - nums[3]*s)
	}
	if !boundedScale(sx) || !boundedScale(sy) {
		return 0, 0, 0, 0, 0, 0, false
	}
	return sx, sy, nums[0], nums[1], ax, ay, true
}

// effectivePreserveAspectRatio returns the preserveAspectRatio value
// for a <symbol> referenced by <use>. SVG 1.1 only honors the
// symbol's own attribute (default xMidYMid meet); SVG 2 added
// override on <use> but is not yet supported here.
func effectivePreserveAspectRatio(target *xmlNode) (gui.SvgAlign, bool) {
	if target == nil || target.Name != "symbol" {
		return gui.SvgAlignXMidYMid, false
	}
	if v, ok := target.AttrMap["preserveAspectRatio"]; ok && v != "" {
		return parsePreserveAspectRatio(v)
	}
	return gui.SvgAlignXMidYMid, false
}

func boundedScale(v float32) bool {
	if !finiteF32(v) {
		return false
	}
	if v < 0 {
		v = -v
	}
	return v <= maxCoordinate
}

func hasPercent(s string) bool {
	return strings.IndexByte(s, '%') >= 0
}

func cloneChildrenForUse(src []xmlNode, budget *int) []xmlNode {
	if len(src) == 0 {
		return nil
	}
	out := make([]xmlNode, 0, len(src))
	for i := range src {
		if *budget <= 0 {
			break
		}
		c := cloneNode(&src[i], budget)
		stripID(&c)
		out = append(out, c)
	}
	return out
}

// cloneNode copies n into a fresh subtree, decrementing budget per
// node. When budget is exhausted the returned subtree is truncated
// (children dropped) so a malicious <use> fanout cannot inflate the
// tree past maxUseExpandClones.
func cloneNode(n *xmlNode, budget *int) xmlNode {
	if *budget <= 0 {
		return xmlNode{}
	}
	*budget--
	out := xmlNode{
		Name:      n.Name,
		OpenTag:   n.OpenTag,
		Leading:   n.Leading,
		Text:      n.Text,
		Tail:      n.Tail,
		SelfClose: n.SelfClose,
	}
	if len(n.Attrs) > 0 {
		out.Attrs = make([]xmlAttr, len(n.Attrs))
		copy(out.Attrs, n.Attrs)
	}
	if len(n.AttrMap) > 0 {
		out.AttrMap = make(map[string]string, len(n.AttrMap))
		maps.Copy(out.AttrMap, n.AttrMap)
	}
	if len(n.Children) > 0 {
		out.Children = make([]xmlNode, 0, len(n.Children))
		for i := range n.Children {
			if *budget <= 0 {
				break
			}
			out.Children = append(out.Children, cloneNode(&n.Children[i], budget))
		}
	}
	return out
}

// stripID removes id attributes from n and every descendant. Cloned
// <use> subtrees must not duplicate ids: leftover descendant ids would
// collide with the original's ids and corrupt url(#id) resolution,
// CSS #id selector matching, and animation targeting.
func stripID(n *xmlNode) {
	if _, ok := n.AttrMap["id"]; ok {
		delete(n.AttrMap, "id")
		filtered := make([]xmlAttr, 0, len(n.Attrs))
		for _, a := range n.Attrs {
			if a.Name != "id" {
				filtered = append(filtered, a)
			}
		}
		n.Attrs = filtered
		n.OpenTag = buildOpenTag(n.Name, n.Attrs, n.SelfClose)
	}
	for i := range n.Children {
		stripID(&n.Children[i])
	}
}
