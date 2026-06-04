package gui

// SvgColor represents an RGBA color from SVG parsing.
type SvgColor struct {
	R, G, B, A uint8
}

// TessellatedPath holds triangulated SVG path geometry.
type TessellatedPath struct {
	Triangles    []float32
	VertexColors []SvgColor
	ClipGroup    int
	// Primitive carries raw attributes for re-tessellation; zero
	// value when the source is not a primitive.
	Primitive SvgPrimitive
	// PathID inherited from the source VectorPath. Uniquely identifies
	// the authored path across all its tessellated pieces (fill +
	// stroke + clip masks share the same ID). Animation state is keyed
	// by PathID. Zero = unset.
	PathID uint32
	// Author's base transform, decomposed into translate/scale/
	// rotate. Tessellated vertices are in local coords when
	// HasBaseXform is true and the decomposition was clean; the
	// render path composes BaseTransX..BaseRotAngle per vertex.
	// When decomposition fails (shear), the raw matrix is baked
	// into Triangles and HasBaseXform is false.
	BaseTransX   float32
	BaseTransY   float32
	BaseScaleX   float32
	BaseScaleY   float32
	BaseRotAngle float32 // degrees
	// BaseRotCX / BaseRotCY is the rotation pivot of the author's
	// base transform. For rotate-about-(cx,cy) authored transforms
	// the pivot is (cx,cy) and BaseTransX/Y are zero, keeping the
	// translation semantics separable from rotation — so a SMIL
	// animateTransform replace-rotate can overwrite the rotation
	// alone without disturbing an unrelated translate component.
	BaseRotCX float32
	BaseRotCY float32
	// Bbox over Triangles in local (pre-base-transform) coordinates.
	// Populated by the tessellator. Used for fast hit-test reject in
	// ContainsPoint and may be reused by future viewport culling.
	MinX, MinY, MaxX, MaxY float32
	Color                  SvgColor
	IsClipMask             bool
	// Animated marks the path as a re-tessellation target. Set when
	// an inline <animate> with an animatable attribute (cx, cy, r,
	// x, y, width, height, rx, ry) targets this shape.
	Animated bool
	// IsStroke marks this path as the stroke contribution of its
	// source shape (vs. the fill contribution). Lets opacity
	// animations targeting fill-opacity / stroke-opacity scale only
	// the matching path at render time.
	IsStroke     bool
	HasBaseXform bool
}

// SvgText holds a parsed SVG text element.
type SvgText struct {
	Text           string
	FontFamily     string
	FillGradientID string
	FilterID       string
	FontWeight     int // CSS numeric weight (100-900); 0 = default (400)
	Anchor         int // 0=start, 1=middle, 2=end
	FontSize       float32
	X, Y           float32
	Opacity        float32
	StrokeWidth    float32
	// FilterGroupKey is a per-occurrence id distinguishing distinct
	// elements that share FilterID. Zero means "no filter".
	FilterGroupKey uint32
	LetterSpacing  float32
	Color          SvgColor
	StrokeColor    SvgColor
	IsBold         bool
	IsItalic       bool
	Underline      bool
	Strikethrough  bool
}

// SvgTextPath holds a parsed SVG textPath element.
type SvgTextPath struct {
	Text          string
	PathID        string
	FontFamily    string
	FilterID      string
	FontWeight    int // CSS numeric weight (100-900); 0 = default (400)
	Anchor        int // 0=start, 1=middle, 2=end
	Method        int // 0=align, 1=stretch
	Side          int // 0=left, 1=right
	FontSize      float32
	Opacity       float32
	StartOffset   float32
	LetterSpacing float32
	StrokeWidth   float32
	// FilterGroupKey is a per-occurrence id distinguishing distinct
	// elements that share FilterID. Zero means "no filter".
	FilterGroupKey uint32
	Color          SvgColor
	StrokeColor    SvgColor
	IsBold         bool
	IsItalic       bool
	IsPercent      bool
}

// SvgFilter holds a parsed SVG filter definition.
type SvgFilter struct {
	ID         string
	BlurLayers int
	StdDev     float32
	KeepSource bool
}

// SvgParsedFilteredGroup holds parsed+tessellated geometry for a
// filter group (paths that share a common filter="url(#id)").
type SvgParsedFilteredGroup struct {
	Paths     []TessellatedPath
	Texts     []SvgText
	TextPaths []SvgTextPath
	Filter    SvgFilter
}

// SvgGradientStop defines a color stop in an SVG gradient.
type SvgGradientStop struct {
	Offset float32
	Color  SvgColor
}

// SvgGradientSpread selects how a gradient handles parameter values
// outside [0,1]. Pad clamps (default), Reflect mirrors, Repeat wraps.
type SvgGradientSpread uint8

// SvgGradientSpread values.
const (
	SvgSpreadPad SvgGradientSpread = iota
	SvgSpreadReflect
	SvgSpreadRepeat
)

// SvgGradientDef defines an SVG gradient (linear or radial).
type SvgGradientDef struct {
	GradientUnits string
	Stops         []SvgGradientStop
	X1, Y1        float32
	X2, Y2        float32
	CX, CY, R     float32
	FX, FY        float32
	IsRadial      bool
	SpreadMethod  SvgGradientSpread
}

// SvgAnimKind identifies the type of SMIL animation.
type SvgAnimKind uint8

// SvgAnimKind constants.
const (
	SvgAnimOpacity SvgAnimKind = iota
	SvgAnimRotate
	// SvgAnimAttr is a generic per-attribute animation (cx, cy, r,
	// x, y, width, height, rx, ry).
	SvgAnimAttr
	// SvgAnimTranslate animates <animateTransform type="translate">.
	// Values is interleaved [tx,ty, tx,ty, ...] (2 per keyframe).
	SvgAnimTranslate
	// SvgAnimScale animates <animateTransform type="scale">. Values
	// is interleaved [sx,sy, sx,sy, ...] (2 per keyframe; uniform
	// scale values are normalized to equal sx,sy at parse time).
	SvgAnimScale
	// SvgAnimMotion animates <animateMotion>: position follows a
	// path. MotionPath holds a flattened polyline as interleaved
	// [x,y,...]; MotionLengths is the cumulative arc length at each
	// vertex. The animation writes to st.TransX/TransY; with
	// MotionRotate=auto it also writes st.RotAngle (tangent angle).
	SvgAnimMotion
	// SvgAnimDashArray animates stroke-dasharray. Values is a flat
	// [f0_0..f0_k-1, f1_0..f1_k-1, ...] layout with DashKeyframeLen
	// floats per keyframe (k<=8). Per-slot linear interp.
	SvgAnimDashArray
	// SvgAnimDashOffset animates stroke-dashoffset. Values is one
	// scalar per keyframe.
	SvgAnimDashOffset
	// SvgAnimColor animates fill or stroke color. ColorValues holds
	// packed uint32 RGBA per keyframe; sRGB lerp at frame eval. Target
	// distinguishes fill (default), stroke, or all (rare).
	SvgAnimColor
)

// SvgAnimMotionRotate selects the rotate= mode on animateMotion.
type SvgAnimMotionRotate uint8

// SvgAnimMotionRotate constants.
const (
	SvgAnimMotionRotateNone SvgAnimMotionRotate = iota
	SvgAnimMotionRotateAuto
	SvgAnimMotionRotateAutoReverse
)

// SvgAttrName identifies an animatable primitive attribute.
type SvgAttrName uint8

// SvgAttrName constants.
const (
	SvgAttrNone SvgAttrName = iota
	SvgAttrCX
	SvgAttrCY
	SvgAttrR
	SvgAttrX
	SvgAttrY
	SvgAttrWidth
	SvgAttrHeight
	SvgAttrRX
	SvgAttrRY
)

// SvgAnimTarget identifies which sub-attribute an opacity animation
// targets. Non-opacity kinds ignore this field.
type SvgAnimTarget uint8

// SvgAnimTarget constants. SvgAnimTargetAll covers attributeName=
// "opacity"; Fill and Stroke cover the per-paint variants and only
// affect the matching tessellated path role at render time.
const (
	SvgAnimTargetAll SvgAnimTarget = iota
	SvgAnimTargetFill
	SvgAnimTargetStroke
)

// SvgAnimRestart controls re-trigger behavior on repeating begin
// entries: "always" fires each activation (default), "whenNotActive"
// skips re-triggers while the previous activation is still within
// its active duration, "never" keeps only the first activation.
type SvgAnimRestart uint8

// SvgAnimRestart constants.
const (
	SvgAnimRestartAlways SvgAnimRestart = iota
	SvgAnimRestartWhenNotActive
	SvgAnimRestartNever
)

// SvgAnimCalcMode selects the keyframe interpolation style.
type SvgAnimCalcMode uint8

// SvgAnimCalcMode constants. Linear is the SMIL default; Spline bends
// per-segment fraction via KeySplines; Discrete holds each keyframe's
// value for its entire segment (no interpolation).
const (
	SvgAnimCalcLinear SvgAnimCalcMode = iota
	SvgAnimCalcSpline
	SvgAnimCalcDiscrete
)

// SvgAnimation holds parsed SMIL animation data.
type SvgAnimation struct {
	// GroupID is the authored binding hint; retained for debug only.
	// Render-time routing uses TargetPathIDs, resolved at parse.
	GroupID string
	// TargetPathIDs lists the VectorPath.PathID values this animation
	// affects. An animation bound to a <g> expands to every descendant
	// primitive path's ID; an animation bound to a single shape lists
	// just that shape's ID. Populated during parse.
	TargetPathIDs []uint32
	// Values layout depends on Kind:
	//   SvgAnimOpacity / SvgAnimRotate / SvgAnimAttr — one scalar
	//     per keyframe (opacity 0..1, rotate angle in deg, attr
	//     native units).
	//   SvgAnimTranslate / SvgAnimScale — interleaved [x,y,...]
	//     with 2 floats per keyframe.
	Values []float32
	// KeySplines stores cubic-bezier control points for spline
	// easing: flat [x1,y1,x2,y2, x1,y1,x2,y2, ...] with one
	// 4-tuple per inter-keyframe segment (len-1 segments where
	// len counts keyframes — scalar Values have len=len(Values);
	// paired Values have len=len(Values)/2). Nil when calcMode !=
	// "spline" or keySplines mismatched.
	KeySplines []float32
	// KeyTimes is the non-uniform keyframe timing list on [0,1].
	// Length equals the number of keyframes (scalar Values have
	// len=len(Values); paired Values have len=len(Values)/2).
	// Must start at 0, end at 1, and be monotonic non-decreasing.
	// Nil when absent or when validation failed — uniform i/(n-1)
	// spacing is used instead.
	KeyTimes []float32
	// MotionPath is a flattened polyline [x0,y0, x1,y1, ...] for
	// SvgAnimMotion. MotionLengths is the cumulative arc length
	// (same count as vertices; last entry = total length). Nil on
	// non-motion kinds.
	MotionPath    []float32
	MotionLengths []float32
	// ColorValues holds packed RGBA stops for SvgAnimColor: one
	// uint32 (0xRRGGBBAA) per keyframe. Nil on all other kinds.
	ColorValues []uint32
	CenterX     float32 // rotation center (SVG coords)
	CenterY     float32
	DurSec      float32
	BeginSec    float32
	// Cycle is the activation period in seconds. 0 means single-play
	// (no looping; freeze or remove after dur). >0 means the
	// animation re-fires every Cycle seconds, allowing chained-
	// freeze SMIL sequences and indefinite repeats to be modeled
	// uniformly: lastActivation = BeginSec + n*Cycle for the largest
	// n with lastActivation <= elapsed.
	Cycle float32
	// Iterations is the CSS animation-iteration-count: 0 means use
	// the legacy SMIL Cycle-based scheduling (one play per Cycle);
	// >0 means run that many DurSec iterations starting at BeginSec
	// (CSS semantics), with a sentinel value of 0xFFFF meaning
	// infinite. Alternate flips the phase on odd iterations.
	Iterations uint16
	Kind       SvgAnimKind
	// Freeze reflects fill="freeze". When true, after dur elapses
	// the animation continues to contribute its last keyframe value
	// until either the cycle restarts or another animation takes
	// over the same attribute (sandwich semantics).
	Freeze bool
	// Accumulate=true reflects accumulate="sum": each repeat starts
	// from the prior end value so repeated animations stack.
	Accumulate bool
	// Additive=true reflects additive="sum": the animation adds its
	// value to the base rather than replacing. For rotate/translate/
	// scale the base is identity (0 / (0,0) / (1,1)); for opacity the
	// base is 1; for attr the base is the primitive's static value.
	Additive bool
	// IsSet marks a <set> element: zero-duration animation that
	// contributes its single to-value from BeginSec onward. Sandwich
	// ordering lets later <set>s override earlier ones.
	IsSet        bool
	AttrName     SvgAttrName     // valid when Kind == SvgAnimAttr
	Target       SvgAnimTarget   // valid when Kind == SvgAnimOpacity
	CalcMode     SvgAnimCalcMode // keyframe interpolation mode
	Restart      SvgAnimRestart  // re-trigger policy
	MotionRotate SvgAnimMotionRotate
	// DashKeyframeLen is the number of floats per keyframe for
	// SvgAnimDashArray (1..8). Keyframe count = len(Values) /
	// DashKeyframeLen. Zero on all other kinds.
	DashKeyframeLen uint8
	// Alternate reflects CSS animation-direction: alternate /
	// alternate-reverse. Each odd iteration plays in reverse phase.
	Alternate bool
	// FillBackwards reflects CSS animation-fill-mode: backwards / both.
	// When true, the first keyframe value is applied during the
	// pre-begin delay (BeginSec > elapsed) so the element reads the
	// "0%" pose before the animation activates.
	FillBackwards bool
}

// SvgAnimIterInfinite marks an infinite CSS animation.
const SvgAnimIterInfinite uint16 = 0xFFFF

// SvgPrimitiveKind identifies the source primitive of a VectorPath.
type SvgPrimitiveKind uint8

// SvgPrimitiveKind constants.
const (
	SvgPrimNone SvgPrimitiveKind = iota
	SvgPrimCircle
	SvgPrimEllipse
	SvgPrimRect
	SvgPrimLine
)

// SvgPrimitive carries raw primitive attributes so animated paths can
// be re-tessellated each frame with current attribute values. Kind is
// SvgPrimNone for non-primitive paths (<path>, polygons, etc.). The
// composed transform lives on the owning VectorPath / GroupID; only
// the primitive-local attributes are captured here.
type SvgPrimitive struct {
	Kind   SvgPrimitiveKind
	CX, CY float32 // circle / ellipse center
	R      float32 // circle radius
	RX, RY float32 // ellipse / rect corner radii
	X, Y   float32 // rect upper-left, line endpoint
	W, H   float32 // rect size
	X2, Y2 float32 // line far endpoint
}

// StrokeCap defines SVG stroke line cap styles.
type StrokeCap uint8

// StrokeCap constants.
const (
	ButtCap StrokeCap = iota
	RoundCap
	SquareCap
)

// StrokeJoin defines SVG stroke line join styles.
type StrokeJoin uint8

// StrokeJoin constants.
const (
	MiterJoin StrokeJoin = iota
	RoundJoin
	BevelJoin
)

// svgToColor converts an SvgColor to a gui Color.
func svgToColor(c SvgColor) Color {
	return Color{c.R, c.G, c.B, c.A, true}
}
