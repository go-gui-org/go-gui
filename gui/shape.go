package gui

import (
	"math/rand/v2"
	"strings"

	"github.com/go-gui-org/go-glyph"
)

// Shape is the only data structure used to draw to the screen. Each
// [Layout] node carries a *Shape pointer. The layout engine computes
// X, Y, Width, Height; the renderer reads those plus color, text,
// and effects to produce [RenderCmd] values.
//
// Widget factories populate Shape via Cfg structs. Users rarely
// construct Shapes directly — prefer the widget API.
type Shape struct {

	// Optional sub-structs (nil when unused)
	events  *eventHandlers     // event handlers
	TC      *shapeTextConfig   // text/RTF fields
	fx      *shapeEffects      // visual effects
	a11Y    *accessInfo        // accessibility metadata
	bc      *shapeButtonColors // button hover/focus colors
	svgOpts *SvgParseOpts      // per-render SVG parse overrides
	tg      *TermGridData      // terminal grid buffer (shapeTermGrid)

	// ID is a user-assigned string identifier. Used for event routing,
	// form field lookup, and debugging. Optional but recommended for
	// interactive widgets.
	//
	// ID is the *leaf* name. The window-unique identity is effID, which
	// resolveShapeIDs derives by joining the IDs of ID-bearing ancestors
	// (see id_resolve.go). Read idKey() — never ID — when using a shape
	// as a key into focus, scroll, hover or per-widget state.
	ID string

	// effID is the resolved, window-unique identity: the leaf joined to
	// the enclosing scope. Stamped by resolveShapeIDs, which runs from
	// layoutArrange before any pass reads an identity, so it is empty
	// only on a tree that has not been arranged — idKey() falls back to
	// ID there.
	effID string

	// focusOwner is the ID of the widget this shape renders for. A
	// composite widget sets it on an inner shape that must consult the
	// owner's focus and spell-check state — Input's text shape draws
	// the caret, selection, IME preedit and squiggles for the outer
	// container — without claiming the owner's identity. ID is an
	// identity and must be unique per window; focusOwner is a
	// reference and deliberately is not.
	//
	// The widget writes the owner's *leaf*; resolveShapeIDs rewrites it
	// in place to the owner's effID, because that is the key the focus
	// and input-state stores hold. In place rather than in a second
	// field: adding effID already moved Shape from the 288-byte size
	// class to the 320-byte one, and a second string would leave only 8
	// bytes of headroom before the next jump. It is safe on two counts —
	// the view phase rebuilds every shape each frame, and resolving an
	// already-effective value is a no-op (it contains IDSep, so it is
	// absolute).
	focusOwner string

	// Resource is an image file path, SVG source string, or data URI.
	// Interpreted by the rendering backend.
	Resource string

	uID uint64 // internal use only

	Version     uint64 // for cache invalidation (DrawCanvas, etc.)
	FloatZIndex int    // stack order for floating elements; higher = on top

	// Structs
	shapeClip drawClip // calculated clipping rectangle
	Padding   Padding  // inner spacing

	// Numeric fields
	X         float32 // final calculated X position (absolute)
	Y         float32 // final calculated Y position (absolute)
	Width     float32
	MinWidth  float32
	MaxWidth  float32
	Height    float32
	MinHeight float32
	MaxHeight float32
	Radius    float32 // corner radius
	Spacing   float32 // spacing between children

	// FloatOffsetX shifts the floating element horizontally from its
	// anchor position after FloatAnchor/FloatTieOff alignment.
	FloatOffsetX float32

	// FloatOffsetY shifts the floating element vertically from its
	// anchor position after FloatAnchor/FloatTieOff alignment.
	FloatOffsetY float32

	// SizeBorder is the width of the border drawn inside the
	// layout bounds. Affects content area calculation.
	SizeBorder float32

	Opacity   float32
	A11YState AccessState

	Color       Color
	ColorBorder Color
	Sizing      Sizing // sizing logic

	// Accessibility
	A11YRole AccessRole

	// Enums/bools
	Axis                 Axis
	shapeType            shapeType
	HAlign               HorizontalAlign
	VAlign               verticalAlign
	ScrollMode           scrollMode
	scrollbarOrientation ScrollbarOrientation
	TextDir              textDirection

	// FloatAnchor is the attachment point on the floating element.
	// Combined with FloatTieOff to position the element relative
	// to its parent. See FloatAttach constants.
	FloatAnchor floatAttach

	// FloatTieOff is the attachment point on the parent that the
	// floating element anchors to. See FloatAttach constants.
	FloatTieOff floatAttach

	// Clip scissor-clips children to this element's bounds.
	Clip bool

	// ClipContents enables stencil-buffer clipping for nested
	// containers. More expensive than Clip but supports nested
	// clipping hierarchies.
	clipContents bool

	Disabled bool

	// Float removes the element from normal layout flow and
	// positions it relative to its parent via FloatAnchor,
	// FloatTieOff, and FloatOffset.
	Float bool

	// FloatAutoFlip flips the float position (e.g. left→right)
	// to keep the element within the window bounds.
	floatAutoFlip bool

	// Focusable opts the widget into the keyboard-focus system.
	// It receives focus via click or Tab, and its per-widget input
	// state (cursor, selection, undo/redo) is keyed by ID. Requires
	// a non-empty ID. Tab order follows layout-tree (DFS) order.
	Focusable bool

	// FocusSkip excludes a Focusable widget from Tab traversal while
	// keeping it click-focusable and able to hold selection state
	// (used by Text/RTF/Markdown selection).
	FocusSkip bool

	// Scrollable makes the widget respond to scroll events (mouse
	// wheel, trackpad) and injects scrollbars. Scroll offset is keyed
	// by ID, so a Scrollable shape requires a non-empty ID. Used with
	// ScrollMode to control which axes scroll.
	Scrollable bool

	// OverDraw draws this element on top of siblings in the same
	// container without affecting layout. Used for overlays,
	// tooltips, and drag indicators.
	OverDraw bool

	// Hero marks this element for hero transition animations.
	// When a Hero element appears in two consecutive frames with
	// the same ID, the renderer animates between positions.
	Hero bool

	// Wrap enables row-wrapping in horizontal-axis containers.
	// Children that exceed the container width wrap to the next
	// row.
	Wrap bool

	// Overflow allows content to extend beyond this element's
	// bounds. When false (default), content is clipped. When
	// true, scrollbars may appear if Scrollable is set.
	Overflow bool

	// QuarterTurns rotates the element in 90° clockwise
	// increments (0–3). Rotation is applied around the element
	// center.
	QuarterTurns uint8

	// AnimSnap marks the layout-transition channels this shape snaps
	// instead of easing. It inherits: applyTransitionRecursive ORs the
	// mask down the walk, so a snapped shape snaps its whole subtree.
	// Zero animates everything the active transition covers, which is
	// why it is safe on every shape that never sets it.
	AnimSnap AnimFlags

	// fillGen matches scratchPools.fillGen when contentW and
	// contentH are valid for the current frame. Set during the
	// fill-height pass; read during position and scroll-offset
	// passes.
	fillGen uint32

	// contentW and contentH cache total content dimensions
	// computed during the fill phase, avoiding redundant
	// child-tree summation in position and scroll passes.
	contentW, contentH float32

	// siblingSumGen matches scratchPools.fillGen when
	// siblingSumW and siblingSumH are valid for the current
	// frame. Separate from fillGen because sibling sums are
	// set on-demand inside layoutFillCrossAxis (not at
	// end-of-pass on every shape), and the width pass stamps
	// fillGen on all shapes before the height pass runs.
	siblingSumGen uint32

	// siblingSumW and siblingSumH cache the sum of sibling sizes
	// on the cross axis, computed during layoutFillCrossAxis.
	// Avoids O(n²) sibling iteration when multiple children of
	// the same parent trigger cross-axis fill.
	siblingSumW, siblingSumH float32
}

// NewShape returns a Shape with default field values.
func newShape() *Shape {
	return &Shape{
		uID:     rand.Uint64(),
		Opacity: 1.0,
	}
}

// shapeType defines the kind of Shape.
type shapeType uint8

// shapeType constants.
const (
	shapeNone shapeType = iota
	shapeRectangle
	shapeText
	shapeImage
	shapeCircle
	shapeRTF
	shapeSVG
	shapeDrawCanvas
	shapeTermGrid
)

// TextDirection controls text/layout direction.
type textDirection uint8

// TextDirection constants.
const (
	textDirAuto textDirection = iota // inherit from parent/global
	TextDirLTR
	TextDirRTL
)

// ScrollMode allows scrolling in one or both directions.
type scrollMode uint8

// ScrollMode constants.
const (
	scrollBoth scrollMode = iota
	ScrollVerticalOnly
	ScrollHorizontalOnly
)

// ScrollbarOrientation determines scrollbar orientation.
// exportaudit:keep — collides with the container's scrollbarOrientation state
type ScrollbarOrientation uint8

// ScrollbarOrientation constants.
const (
	scrollbarNone ScrollbarOrientation = iota
	scrollbarVertical
	scrollbarHorizontal
)

// FloatAttach defines anchor points for floating elements.
type floatAttach uint8

// FloatAttach constants.
const (
	FloatTopLeft floatAttach = iota
	FloatTopCenter
	FloatTopRight
	floatMiddleLeft
	FloatMiddleCenter
	FloatMiddleRight
	FloatBottomLeft
	FloatBottomCenter
	FloatBottomRight
)

// AccessRole identifies a shape's semantic role.
type AccessRole uint8

// AccessRole constants.
const (
	AccessRoleNone AccessRole = iota
	AccessRoleButton
	AccessRoleCheckbox
	AccessRoleColorWell
	AccessRoleComboBox
	AccessRoleDateField
	AccessRoleDialog
	AccessRoleDisclosure
	AccessRoleGrid
	AccessRoleGridCell
	AccessRoleGroup
	AccessRoleHeading
	AccessRoleImage
	AccessRoleLink
	AccessRoleList
	AccessRoleListItem
	AccessRoleMenu
	AccessRoleMenuBar
	AccessRoleMenuItem
	AccessRoleProgressBar
	AccessRoleRadioButton
	AccessRoleRadioGroup
	AccessRoleScrollArea
	AccessRoleScrollBar
	AccessRoleSlider
	AccessRoleSplitter
	AccessRoleStaticText
	AccessRoleSwitchToggle
	AccessRoleTab
	AccessRoleTabItem
	AccessRoleTextField
	AccessRoleTextArea
	AccessRoleToolbar
	AccessRoleTree
	AccessRoleTreeItem
)

// AccessState is a bitmask of dynamic accessibility states.
type AccessState uint16

// AccessState constants.
const (
	AccessStateNone     AccessState = 0
	AccessStateExpanded AccessState = 1
	AccessStateSelected AccessState = 2
	AccessStateChecked  AccessState = 4
	AccessStateRequired AccessState = 8
	AccessStateInvalid  AccessState = 16
	AccessStateBusy     AccessState = 32
	AccessStateReadOnly AccessState = 64
	AccessStateModal    AccessState = 128
	AccessStateLive     AccessState = 256
	AccessStateDisabled AccessState = 512
)

// Has checks if the state bitmask contains the given flag.
func (s AccessState) Has(flag AccessState) bool {
	return s&flag == flag
}

// drawClip represents a clipping rectangle.
type drawClip struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// ShapeTextConfig holds text/RTF-specific fields for a Shape.
type shapeTextConfig struct {
	textLayoutStyle    TextStyle
	TextStyle          *TextStyle
	textLayout         *glyph.Layout
	rTFRuns            *RichText
	rTFLayout          *glyph.Layout
	rtfGlyphRT         *glyph.RichText // cached conversion
	Text               string
	rTFFlatText        string // concatenation of all run texts; rune↔byte conversion for selection
	textLayoutText     string
	rtfMathHashes      []int64 // cache keys per inline math object
	rTFBaseStyle       glyph.TextStyle
	rTFLineSpacing     float32 // gui LineSpacing; glyph.TextStyle drops it, so carried separately for BlockStyle
	textSelBeg         uint32
	textSelEnd         uint32
	TextTabSize        uint32
	hangingIndent      float32
	markdownID         string // non-empty when this RTF block belongs to a markdown widget
	markdownBlockStart uint32 // rune offset of this block within the markdown flat text
	markdownRuneLen    uint32 // rune count of this block's flat text
	wrapCacheWidth     float32
	wrapCacheHeight    float32
	textLayoutWidth    float32
	TextMode           textMode
	textIsPassword     bool
	textIsPlaceholder  bool
	// TextReadOnly suppresses IME preedit rendering for read-only
	// fields, which stay Focusable (for selection/cursor) but can
	// never commit a composition. See render_text.go.
	textReadOnly    bool
	wrapCacheValid  bool
	textLayoutValid bool
	textLayoutMode  textMode
}

// hasRtfLayout returns true if the shape has an RTF layout.
func (s *Shape) hasRtfLayout() bool {
	return s.TC != nil && s.TC.rTFLayout != nil
}

// TextMode controls how a text view renders text.
type textMode uint8

// TextMode constants.
const (
	TextModeSingleLine textMode = iota
	TextModeMultiline
	TextModeWrap
	TextModeWrapKeepSpaces
)

// eventHandlers holds optional event callback fields.
type eventHandlers struct {
	// One rule for all of these: the callback must call ctx.Consume()
	// to stop propagation. Until v0.55.0 the five below were
	// "consume-class" and dispatch marked their events handled before
	// the callback ran; that split is gone.
	OnChar      func(EventCtx)
	OnClick     func(EventCtx)
	OnMouseUp   func(EventCtx)
	OnMouseDown func(EventCtx)
	OnGesture   func(EventCtx)
	OnFileDrop  func(EventCtx)

	OnKeyDown     func(EventCtx)
	OnKeyUp       func(EventCtx)
	OnMouseMove   func(EventCtx)
	OnMouseScroll func(EventCtx)
	OnHover       func(EventCtx)
	OnMouseLeave  func(EventCtx)
	onIMECommit   func(string, EventCtx)

	// No originating event — ctx.Event is nil in these.
	onScroll    func(EventCtx)
	AmendLayout func(EventCtx)

	OnDraw func(*DrawContext)

	// Click filters — set by widget factories to avoid the per-frame
	// closure allocation a wrapper callback would cost.
	clickButton  MouseButton // non-zero filters OnClick by mouse button
	clickOnSpace bool        // fire OnClick on spacebar via OnChar dispatch
	clickOnEnter bool        // fire OnClick on Enter key via OnKeyDown dispatch
}

// shapeButtonColors holds per-button color state read by
// package-level button event handlers, avoiding per-frame
// closure allocations.
type shapeButtonColors struct {
	OnHover          func(EventCtx)
	OnAmend          func(EventCtx)
	ColorHover       Color
	colorClick       Color
	ColorFocus       Color
	ColorBorderFocus Color
	// focusRing is the theme's focus-ring shadow, captured at
	// generation because buttonAmendLayout is a plain func with no
	// closure to read guiTheme from at the right time.
	focusRing *BoxShadow
	// opticalDigits centres the button's label on the face's figure
	// band instead of its cap band, for a button whose label is digits
	// by construction — a date picker's day cell. Set from the
	// unexported ButtonCfg.opticalDigitLabel; see buttonAmendLayout.
	opticalDigits bool
}

// shapeEffects holds optional visual effect fields.
type shapeEffects struct {
	Shadow         *BoxShadow
	Gradient       *GradientDef
	BorderGradient *GradientDef
	Shader         *Shader
	ColorFilter    *ColorFilter
	BlurRadius     float32
}

// BoxShadow defines drop shadow properties.
type BoxShadow struct {
	Color      Color
	OffsetX    float32
	OffsetY    float32
	BlurRadius float32
	// Spread grows the shadow's own shape beyond the caster before
	// blurring; zero is today's behavior.
	Spread float32
}

// GradientType specifies the gradient algorithm.
type gradientType uint8

// GradientType constants.
const (
	GradientLinear gradientType = iota
	GradientRadial
)

// GradientDirection specifies the gradient direction for linear
// gradients.
type GradientDirection uint8

// GradientDirection constants.
const (
	GradientToTop GradientDirection = iota
	GradientToTopRight
	GradientToRight
	GradientToBottomRight
	GradientToBottom
	GradientToBottomLeft
	GradientToLeft
	GradientToTopLeft
)

// GradientStop defines a color at a position along the gradient.
type GradientStop struct {
	Color Color
	Pos   float32 // 0.0 to 1.0
}

// GradientDef defines a gradient with stops and direction.
type GradientDef struct {
	Stops     []GradientStop
	angle     float32 // explicit angle in degrees
	Type      gradientType
	Direction GradientDirection
	hasAngle  bool // true when Angle overrides Direction
}

// AccessInfo holds string accessibility data.
type accessInfo struct {
	Label       string
	Description string
	ValueNum    float32
	ValueMin    float32
	ValueMax    float32
}

// hasEvents returns true if eventHandlers is allocated.
func (s *Shape) hasEvents() bool {
	return s.events != nil
}

// idKey returns the identity this shape is keyed by in every ID-keyed
// store and match: the resolved effID, falling back to the leaf ID on a
// tree resolveShapeIDs has not walked yet (a hand-built Layout in a
// test). Under an empty scope the two are equal, so the fallback only
// ever matters before the pipeline runs.
func (s *Shape) idKey() string {
	if s.effID != "" {
		return s.effID
	}
	return s.ID
}

// focusKey returns the ID whose focus and per-widget state this shape
// renders from: focusOwner when the shape belongs to a composite
// widget, otherwise its own ID. Empty for a shape that participates in
// neither, which is the signal to render no caret or selection.
//
// Both arms return an *effective* ID, because that is what the focus,
// input-state and spell-check stores hold.
func (s *Shape) focusKey() string {
	if s.focusOwner != "" {
		return s.focusOwner
	}
	return s.idKey()
}

// rendersFocusState reports whether this shape draws caret, selection
// or spell-check state for some widget: for itself when Focusable, or
// for the owner named by focusOwner. False for ordinary text, which
// must render none of it even when it carries an ID.
func (s *Shape) rendersFocusState() bool {
	return s.Focusable || s.focusOwner != ""
}

// PointInShape determines if the given point is within shapeClip.
func (s *Shape) PointInShape(x, y float32) bool {
	sc := s.shapeClip
	if sc.Width <= 0 || sc.Height <= 0 {
		return false
	}
	return x >= sc.X && y >= sc.Y &&
		x < (sc.X+sc.Width) && y < (sc.Y+sc.Height)
}

// PaddingLeft returns effective left padding (padding + border).
func (s *Shape) PaddingLeft() float32 {
	return s.Padding.Left + s.SizeBorder
}

// PaddingTop returns effective top padding (padding + border).
func (s *Shape) PaddingTop() float32 {
	return s.Padding.Top + s.SizeBorder
}

// paddingWidth returns total horizontal padding.
func (s *Shape) paddingWidth() float32 {
	return s.Padding.Width() + (s.SizeBorder * 2)
}

// paddingHeight returns total vertical padding.
func (s *Shape) paddingHeight() float32 {
	return s.Padding.Height() + (s.SizeBorder * 2)
}

// makeA11YInfo returns an AccessInfo if label or desc is set.
func makeA11YInfo(label, desc string) *accessInfo {
	if label == "" && desc == "" {
		return nil
	}
	return &accessInfo{Label: label, Description: desc}
}

// a11yLabel returns label if set, otherwise falls back to text.
//
// The fallback is often a widget's ID, and an ID may be a scoped path
// ("settings:name") once its widget resolves its identity. A screen
// reader must announce a name, not a path, so only the last segment
// survives the fallback. An explicit A11YLabel is returned untouched —
// a colon an author wrote is part of the label.
func a11yLabel(label, text string) string {
	if label != "" {
		return label
	}
	// A trailing separator leaves nothing to announce, so the whole
	// string is better than silence.
	if i := strings.LastIndex(text, IDSep); i >= 0 &&
		i+len(IDSep) < len(text) {
		return text[i+len(IDSep):]
	}
	return text
}
