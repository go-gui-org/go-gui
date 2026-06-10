package svg

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// Security limits for SVG parsing to prevent DoS attacks.
const (
	defaultIconSize = 24
	maxGroupDepth   = 32
	maxElements     = 100000
	maxPathSegments = 100000
	maxViewBoxDim   = float32(10000)
	maxAttrLen      = 1048576 // 1MB
	maxCoordinate   = float32(1000000)
	maxAnimations   = 100
	maxFlattenDepth = 16
	// maxKeyframes caps the number of keyframes (values) and
	// syncbase begin entries parsed from a single <animate> /
	// <animateTransform>. Real assets use <20; this guards
	// against a pathological input that lists thousands of
	// semicolon-separated entries.
	maxKeyframes = 256
)

// Tessellation and stroke constants.
const (
	strokeCrossTolerance = float32(0.001)
	strokeMiterLimit     = float32(4.0)
	strokeRoundCapSegs   = 8
	curveDegenThreshold  = float32(0.0001)
	closedPathEpsilon    = float32(0.0001)
	maxSplitTriDepth     = 8
)

// PathCmd identifies a path segment type.
type PathCmd uint8

// PathCmd constants.
const (
	CmdMoveTo PathCmd = iota
	CmdLineTo
	CmdQuadTo
	CmdCubicTo
	CmdClose
)

// FillRule selects how overlapping subpaths are filled. Matches
// the SVG fill-rule presentation attribute.
type FillRule uint8

// FillRule constants. Zero value is nonzero (SVG default).
const (
	FillRuleNonzero FillRule = iota
	FillRuleEvenOdd
)

// PathSegment is one segment of an SVG path.
type PathSegment struct {
	Cmd    PathCmd
	Points []float32 // moveTo/lineTo: [x,y]; quad: [cx,cy,x,y]; cubic: [6]
}

// VectorPath represents a single filled/stroked path.
type VectorPath struct {
	ClipPathID       string
	FillGradientID   string
	StrokeGradientID string
	FilterID         string
	GroupID          string
	Segments         []PathSegment
	StrokeDasharray  []float32
	// Computed snapshots the resolved cascade context (transform,
	// fill, stroke, opacity, font, …) at parse time. Phase A only
	// records the result; later CSS phases will use it as the hot-
	// path style accessor. Embedding a value (not a pointer) keeps
	// the per-frame access allocation-free.
	Computed  ComputedStyle
	Primitive gui.SvgPrimitive
	Transform [6]float32 // affine [a,b,c,d,e,f]
	// Bbox is the geometric bounding box in author (viewBox) units,
	// computed pre-tessellation. Used to resolve transform-origin
	// keywords/percentages into numeric rotation centers at compile
	// time. Untransformed — caller composes path.Transform when an
	// origin in transformed space is needed. Zero value means
	// "unknown" (group/use without resolved children).
	Bbox bbox
	// PathID is a monotonic per-SvgParsed identifier assigned at parse
	// time. Every VectorPath carries a unique ID; derived tessellated
	// paths (fill, stroke, clip masks) inherit it so animation state
	// is routed per-path rather than per-GroupID. Zero means unset
	// (clip-mask or fallback). GroupID stays as a debug hint.
	PathID           uint32
	StrokeWidth      float32
	StrokeDashOffset float32
	Opacity          float32
	FillOpacity      float32
	StrokeOpacity    float32
	// FilterGroupKey is a per-occurrence id assigned at parse time.
	// Distinct elements with filter="url(#X)" get distinct keys so the
	// renderer can composite their offscreen buffers separately and in
	// document order, while FilterID remains the actual filter def id.
	FilterGroupKey uint32
	FillColor      gui.SvgColor
	StrokeColor    gui.SvgColor
	StrokeCap      gui.SvgStrokeCap
	StrokeJoin     gui.SvgStrokeJoin
	Animated       bool
	FillRule       FillRule
}

// bbox is an axis-aligned bounding rectangle in author coordinates.
// IsEmpty() reports whether the bbox carries useful extents.
type bbox struct {
	MinX, MinY, MaxX, MaxY float32
	Set                    bool
}

func (b bbox) Width() float32  { return b.MaxX - b.MinX }
func (b bbox) Height() float32 { return b.MaxY - b.MinY }

// svgFilteredGroup holds paths/texts belonging to a single filter
// occurrence — the subtree under one element that set
// filter="url(#X)". FilterID is the filter def to apply; GroupKey is
// the per-occurrence id that disambiguates non-contiguous uses of the
// same filter.
type svgFilteredGroup struct {
	FilterID  string
	Paths     []VectorPath
	Texts     []gui.SvgText
	TextPaths []gui.SvgTextPath
	GroupKey  uint32
}

// VectorGraphic holds the full parsed SVG before tessellation.
type VectorGraphic struct {
	DefsPaths map[string]string
	ClipPaths map[string][]VectorPath
	Gradients map[string]gui.SvgGradientDef
	Filters   map[string]gui.SvgFilter
	// GroupParent maps a synth group ID (e.g. "__anim_3") to its
	// parent synth/author group ID. Empty parent = no enclosing group.
	// Used by resolveAnimationTargets to propagate group-level
	// animations onto descendant paths whose own GroupID is a
	// nested synth ID.
	GroupParent map[string]string
	// A11y carries root-level accessibility metadata; mirrored into
	// gui.SvgParsed by buildParsed.
	A11y               gui.SvgA11y
	Paths              []VectorPath
	Texts              []gui.SvgText
	TextPaths          []gui.SvgTextPath
	FilteredGroups     []svgFilteredGroup
	Animations         []gui.SvgAnimation
	Width, Height      float32
	ViewBoxX, ViewBoxY float32
	// FlatnessTolerance, when > 0, overrides the default tessellation
	// tolerance floor (0.15). Higher = coarser triangles.
	FlatnessTolerance float32
	// PreserveAlign / PreserveSlice mirror the parsed
	// preserveAspectRatio attribute. Defaults: xMidYMid meet.
	PreserveAlign gui.SvgAlign
	PreserveSlice bool
}

// identityTransform is the identity affine matrix.
var identityTransform = [6]float32{1, 0, 0, 1, 0, 0}

// Sentinel colors for style inheritance.
// colorInherit marks a fill/stroke that was not explicitly set on
// the element — resolve from the parent chain during
// applyComputedStyle.
// colorCurrent marks an explicit "currentColor" or "inherit"
// keyword — preserve it so the render-time tint (SvgCfg.Color)
// can substitute the actual color. Both are recognized by their
// magenta RGB so opacity baking can still scale alpha safely.
var (
	colorInherit     = gui.SvgColor{R: 255, G: 0, B: 255, A: 1}
	colorCurrent     = gui.SvgColor{R: 255, G: 0, B: 255, A: 2}
	colorTransparent = gui.SvgColor{}
	colorBlack       = gui.SvgColor{R: 0, G: 0, B: 0, A: 255}
)

// ComputedStyle is the resolved presentation state for an SVG
// element after the cascade walk (presentation attributes + inline
// style + ancestor inheritance). Phase A populates this from
// presentation attrs only; Phase B/C plug CSS rules into the same
// pipeline. Paint-related fields hold parsed values (gui.SvgColor,
// float32, enums); font-related fields are still kept as raw
// strings — text shaping consumes them as-is and has no need to
// see them resolved at this layer.
type ComputedStyle struct {

	// Vars holds custom properties (--name → value) inherited from
	// ancestors plus any defined on this element. Lazy: shares the
	// parent's map when the element introduces no new vars. Resolved
	// at value-substitution time; undefined references drop the
	// declaration per the design doc.
	Vars map[string]string

	FillGradient   string
	StrokeGradient string

	ClipPathID string
	FilterID   string
	GroupID    string

	// Text-related properties remain strings until text shaping
	// consumes them.
	FontFamily string
	FontSize   string
	FontWeight string
	FontStyle  string
	TextAnchor string

	// TransformOrigin holds the raw `transform-origin` declaration
	// (e.g. "center", "50% 50%", "10px 20px"). Resolution to numeric
	// (x,y) is deferred until shape bbox is known at compile time.
	// Empty string == unset (CSS default = "50% 50%" = bbox center).
	// Inherited from parent so `<g style="transform-origin:...">` flows
	// to descendants — matches author intent even though spec calls it
	// non-inherited.
	TransformOrigin string

	StrokeDasharray []float32

	// Animation collects CSS animation-* properties that landed on
	// this element. CSS animations are not inherited; the cascade
	// walk clears this at the start of each element so children only
	// see their own animation-* writes.
	Animation cssAnimSpec

	Transform [6]float32

	// StrokeWidth = -1 means unset (inherit further or default to 1.0
	// at apply time). StrokeCap/StrokeJoin use strokeCapInherit /
	// strokeJoinInherit sentinels for the same purpose.
	StrokeWidth float32

	// StrokeDashOffset = 0 by default; *Set distinguishes "explicitly
	// 0" (stop dash advance) from "unset" so a CSS @keyframes timeline
	// driving the property doesn't have to fight a baked static value.
	StrokeDashOffset float32

	// Opacity multiplies through ancestors. FillOpacity / StrokeOpacity
	// inherit values directly (per SVG spec they do not multiply, but
	// each element's value is fed into its descendants' default).
	Opacity       float32
	FillOpacity   float32
	StrokeOpacity float32

	// FilterGroupKey is bumped each time an element introduces a fresh
	// filter="url(#X)" via the cascade; descendants inherit the same
	// key. Zero means "no filter applies in this subtree".
	FilterGroupKey uint32

	// Fill/Stroke hold resolved colors. *Set marks whether the
	// cascade actually wrote the property, distinguishing "fill
	// resolved to colorInherit" (initial) from "fill explicitly
	// set in this element or an ancestor". *Gradient holds a
	// gradient ID when the source was url(#id).
	Fill                gui.SvgColor
	Stroke              gui.SvgColor
	FillSet             bool
	StrokeSet           bool
	StrokeCap           gui.SvgStrokeCap
	StrokeJoin          gui.SvgStrokeJoin
	StrokeDashOffsetSet bool

	// Skip* tells the opacity baker that an inline animation will
	// supply the corresponding alpha at render time, so a static
	// fill-opacity="0" must not bake the fill alpha to zero.
	SkipOpacity       bool
	SkipFillOpacity   bool
	SkipStrokeOpacity bool

	// AuthoredClipPath = true when this element declared clip-path
	// itself (presentation attr / CSS rule / inline style), as opposed
	// to inheriting the value from its parent. Distinguishes "redeclared
	// same id as parent" from "inherited unchanged" — the synth-clip
	// path in xml.go uses it to skip the viewport clip on author-clipped
	// nested <svg>s without false negatives.
	AuthoredClipPath bool

	// FillRule resolved from element/ancestors. Zero == nonzero
	// (SVG default).
	FillRule FillRule

	// Display: zero == DisplayInline (rendered). DisplayNone removes
	// the element and descendants from the box tree (Phase F T2).
	// Not inherited — reset each element so child elements aren't
	// hidden by a parent's "display: none" leaking via inheritance.
	Display DisplayMode

	// Visibility inherits per spec. VisibilityHidden keeps layout but
	// suppresses paint; descendants can re-show via "visibility:
	// visible" on themselves. (Phase F T2.)
	Visibility VisibilityMode
}

// DisplayMode encodes the subset of CSS `display` values the SVG
// cascade recognizes. The zero value is DisplayInline so a
// zero-valued ComputedStyle defaults to "rendered".
type DisplayMode uint8

// DisplayMode values.
const (
	DisplayInline DisplayMode = iota
	DisplayNone
)

// VisibilityMode encodes the subset of CSS `visibility` values the
// SVG cascade recognizes. Zero value is VisibilityVisible so a
// zero-valued ComputedStyle defaults to "painted".
type VisibilityMode uint8

// VisibilityMode values.
const (
	VisibilityVisible VisibilityMode = iota
	VisibilityHidden
)

// cssAnimSpec mirrors the CSS animation-* shorthand for one
// element. AnimationName == "" means no animation. Compile happens
// after the cascade walk by looking up the named @keyframes block.
type cssAnimSpec struct {
	Name         string
	DurationSec  float32 // 0 when unset (animation does nothing)
	DelaySec     float32
	IterCount    uint16 // 0 = unset → 1; SvgAnimIterInfinite for infinite
	IterCountSet bool
	Direction    cssAnimDir
	FillMode     cssAnimFill
	TimingFn     cssAnimTiming
	TimingArgs   [4]float32 // cubic-bezier(x1,y1,x2,y2) when TimingFn=cubic
	StepsCount   uint16     // steps(N, ...) when TimingFn=steps
	StepsAtStart bool       // steps(N, start)
}

type cssAnimDir uint8

const (
	cssAnimDirNormal cssAnimDir = iota
	cssAnimDirReverse
	cssAnimDirAlternate
	cssAnimDirAlternateReverse
)

type cssAnimFill uint8

const (
	cssAnimFillNone cssAnimFill = iota
	cssAnimFillForwards
	cssAnimFillBackwards
	cssAnimFillBoth
)

type cssAnimTiming uint8

const (
	cssAnimTimingLinear cssAnimTiming = iota
	cssAnimTimingCubic
	cssAnimTimingSteps
)

// defaultComputedStyle returns the root style with sensible
// "unset" sentinels — opacity multipliers at 1.0, paint flags
// cleared, stroke-width = -1, caps/joins at the inherit sentinel.
func defaultComputedStyle(transform [6]float32) ComputedStyle {
	return ComputedStyle{
		Transform:     transform,
		StrokeWidth:   -1,
		StrokeCap:     strokeCapInherit,
		StrokeJoin:    strokeJoinInherit,
		Opacity:       1.0,
		FillOpacity:   1.0,
		StrokeOpacity: 1.0,
	}
}

// parseState tracks mutable state during SVG parsing.
type parseState struct {
	// animIDIndex maps an <animate id="..."> to its index in
	// animations. Populated only when an animation carries an id.
	animIDIndex map[string]int
	// animBeginSpecs holds unresolved begin specs per animation
	// index. Populated only when begin= references another
	// animation via syncbase (.begin/.end).
	animBeginSpecs map[int][]beginSpec
	// defsPaths maps defs <path id="..."> to its d-attribute. Used
	// by animateMotion's <mpath xlink:href="#id"/>.
	defsPaths map[string]string
	// groupParent records each synthesized group ID's enclosing
	// group ID so resolveAnimationTargets can fan group-level
	// animations out onto descendant paths.
	groupParent map[string]string
	// vg is nil during defs-subtree parses (parseDefsClipPaths
	// constructs empty-literal parseStates); synth code must nil-guard.
	vg *VectorGraphic
	// hoveredID / focusedID feed CSS :hover / :focus pseudo-class
	// matching during the cascade. Empty disables the corresponding
	// state.
	hoveredID  string
	focusedID  string
	texts      []gui.SvgText
	textPaths  []gui.SvgTextPath
	animations []gui.SvgAnimation
	// cssRules holds the parsed author CSS rules from <style>
	// blocks. Empty when the document carries no embedded CSS.
	// Phase B: tag/id/class compound selectors with paint
	// declarations only.
	cssRules []css.Rule
	// cssKeyframes holds @keyframes blocks gathered alongside cssRules.
	// Phase D: looked up at compile time when an element has
	// animation-name: foo set.
	cssKeyframes []css.KeyframesDef
	elemCount    int
	synthID      int
	synthClipID  int
	// curViewport is the active SVG viewport in user-space units.
	// parseSvgWith seeds it from the root <svg>; nested <svg> pushes
	// the inner viewBox (or viewport rect when no viewBox) and the
	// switch case restores the previous value on return so x/y/w/h
	// percentages on a deeper nested viewport resolve against the
	// correct ancestor.
	curViewport viewportRect
	// pathIDSeq assigns monotonic PathIDs to every parsed VectorPath.
	// Starts at 1; zero is reserved for "unset".
	pathIDSeq uint32
	// Per-occurrence counter for filter cascade; 0 = no filter.
	nextFilterGroup uint32
}

// elementStyle holds common style properties extracted from an SVG element.
type elementStyle struct {
	StrokeGradientID string
	StrokeDasharray  []float32
	Transform        [6]float32
	StrokeWidth      float32
	Opacity          float32
	FillOpacity      float32
	StrokeOpacity    float32
	StrokeColor      gui.SvgColor
	StrokeCap        gui.SvgStrokeCap
	StrokeJoin       gui.SvgStrokeJoin
}
