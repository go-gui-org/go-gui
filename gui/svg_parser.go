package gui

// SvgParsed holds the result of parsing an SVG document.
type SvgParsed struct {
	DefsPaths map[string]string // id → raw SVG path d attribute
	Gradients map[string]SvgGradientDef
	// A11y carries document-level accessibility metadata from <title>,
	// <desc>, and aria-* attributes on the root <svg>. Empty fields
	// indicate the source SVG omitted them.
	A11y           SvgA11y
	Paths          []TessellatedPath
	Texts          []SvgText
	TextPaths      []SvgTextPath
	FilteredGroups []SvgParsedFilteredGroup
	Animations     []SvgAnimation
	Width          float32
	Height         float32
	// ViewBoxX / ViewBoxY are the authored viewBox origin. All coord
	// fields above stay in raw viewBox space; render applies a single
	// outer translate of -(ViewBoxX*scale, ViewBoxY*scale) so SMIL
	// animateTransform in replace mode cannot clobber the mapping.
	ViewBoxX float32
	ViewBoxY float32
	// PreserveAlign encodes the alignment portion of the SVG
	// preserveAspectRatio attribute. Default is SvgAlignXMidYMid,
	// which matches the renderer's pre-existing centered scale.
	PreserveAlign SvgAlign
	// PreserveSlice is true when preserveAspectRatio specifies
	// "slice"; default false ("meet"). Slice scales to fill and
	// clips overflow; meet scales to fit and leaves slack.
	PreserveSlice bool
}

// SvgAlign encodes the SVG preserveAspectRatio alignment grid.
// Values match the spec's nine xMin/Mid/Max × yMin/Mid/Max keywords.
// SvgAlignNone is reserved for the spec's "none" keyword which
// requests non-uniform stretch — currently treated as
// SvgAlignXMidYMid + meet pending renderer support.
type SvgAlign uint8

// SvgAlign values. SvgAlignXMidYMid is the spec default and the zero
// value so default-initialized SvgParsed values match historical
// renderer behavior without explicit setup.
const (
	SvgAlignXMidYMid SvgAlign = iota
	SvgAlignXMinYMin
	SvgAlignXMidYMin
	SvgAlignXMaxYMin
	SvgAlignXMinYMid
	SvgAlignXMaxYMid
	SvgAlignXMinYMax
	SvgAlignXMidYMax
	SvgAlignXMaxYMax
	SvgAlignNone
)

// SvgA11y holds accessibility metadata from the root <svg> element.
// Title/Desc come from direct <title>/<desc> child elements; aria-*
// fields read from root attributes.
type SvgA11y struct {
	Title        string
	Desc         string
	AriaLabel    string
	AriaRoleDesc string
	AriaHidden   bool
}

// SvgParser parses and tessellates SVG documents. Set by the
// backend; nil in tests (SVG views degrade to error placeholders).
type SvgParser interface {
	ParseSvg(data string) (*SvgParsed, error)
	ParseSvgFile(path string) (*SvgParsed, error)
	ParseSvgDimensions(data string) (float32, float32, error)
	Tessellate(parsed *SvgParsed, scale float32) []TessellatedPath
}

// SvgParseOpts carries per-parse environment toggles (Phase F).
// PrefersReducedMotion is the snapshot from NativePlatform routed
// into `@media (prefers-reduced-motion: reduce)` evaluation.
// FlatnessTolerance, when > 0, overrides the tessellation tolerance
// floor (default 0.15 viewBox units). HoveredElementID /
// FocusedElementID feed CSS :hover / :focus pseudo-class matching;
// empty disables the corresponding state.
type SvgParseOpts struct {
	HoveredElementID     string
	FocusedElementID     string
	FlatnessTolerance    float32
	PrefersReducedMotion bool
}

// SvgParserWithOpts is an optional extension of SvgParser whose
// parse calls take a SvgParseOpts snapshot. LoadSvg detects this via
// type assertion and falls back to ParseSvg/ParseSvgFile when the
// backend hasn't opted in.
type SvgParserWithOpts interface {
	ParseSvgWithOpts(data string, opts SvgParseOpts) (*SvgParsed, error)
	ParseSvgFileWithOpts(path string, opts SvgParseOpts) (*SvgParsed, error)
}

// SvgAnimAttrMask flags which fields of SvgAnimAttrOverride are set.
// Flat bits + flat float32 fields avoid per-frame heap allocations
// that *float32 fields would incur through the map value.
type SvgAnimAttrMask uint16

// SvgAnimAttrMask bits.
const (
	SvgAnimMaskCX SvgAnimAttrMask = 1 << iota
	SvgAnimMaskCY
	SvgAnimMaskR
	SvgAnimMaskRX
	SvgAnimMaskRY
	SvgAnimMaskX
	SvgAnimMaskY
	SvgAnimMaskWidth
	SvgAnimMaskHeight
	// SvgAnimMaskStrokeDashArray marks StrokeDashArray/Len as live.
	SvgAnimMaskStrokeDashArray
	// SvgAnimMaskStrokeDashOffset marks StrokeDashOffset as live.
	SvgAnimMaskStrokeDashOffset
)

// SvgAnimDashArrayCap caps the max pairs-count for an animated
// stroke-dasharray. 8 floats handles every real-world spinner
// pattern (ring-resize uses 2). Fixed-size storage avoids heap
// churn in the hot-path SvgAnimAttrOverride struct.
const SvgAnimDashArrayCap = 8

// SvgAnimAttrOverride carries current animated attribute values to
// apply when re-tessellating a primitive. Fields are valid only when
// their Mask bit is set; unset fields retain the parsed static value.
// When AdditiveMask bit is also set, the override value is a delta
// to add to the parsed primitive value rather than a replacement —
// matches SMIL additive="sum" / by= shorthand semantics.
type SvgAnimAttrOverride struct {
	Mask             SvgAnimAttrMask
	AdditiveMask     SvgAnimAttrMask
	CX, CY           float32
	R                float32
	RX, RY           float32
	X, Y             float32
	Width            float32
	Height           float32
	StrokeDashOffset float32
	// StrokeDashArray carries up to SvgAnimDashArrayCap floats of
	// live stroke-dasharray pattern. StrokeDashArrayLen gives the
	// used prefix; values past Len are undefined.
	StrokeDashArray    [SvgAnimDashArrayCap]float32
	StrokeDashArrayLen uint8
}

// AnimatedSvgParser is an optional extension of SvgParser for backends
// that can re-tessellate animated primitive shapes with current
// attribute values. Detected via type assertion; parsers that do not
// implement it simply render animated paths from their static cache.
type AnimatedSvgParser interface {
	// TessellateAnimated returns fresh triangles for every path in
	// parsed whose Animated flag is set, at the given scale, with
	// optional attribute overrides keyed by PathID. Returned slice
	// order matches the Animated-flagged paths' document order.
	// reuse, if non-nil, is a caller-supplied slice the parser may
	// append into to amortize per-frame slice allocations; returned
	// slice may alias reuse's backing array. An empty overrides map
	// or nil map yields nil (caller should use cached triangles).
	TessellateAnimated(parsed *SvgParsed, scale float32,
		overrides map[uint32]SvgAnimAttrOverride,
		reuse []TessellatedPath) []TessellatedPath
}

// SetSvgParser sets the SVG parser backend.
func (w *Window) SetSvgParser(p SvgParser) {
	w.svgParser = p
}

// SvgParser returns the current SVG parser, or nil.
func (w *Window) SvgParser() SvgParser {
	return w.svgParser
}
