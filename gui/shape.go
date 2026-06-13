package gui

import (
	"math/rand/v2"

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
	TC      *ShapeTextConfig   // text/RTF fields
	fx      *shapeEffects      // visual effects
	A11Y    *AccessInfo        // accessibility metadata
	bc      *shapeButtonColors // button hover/focus colors
	SvgOpts *SvgParseOpts      // per-render SVG parse overrides

	// ID is a user-assigned string identifier. Used for event routing,
	// form field lookup, and debugging. Optional but recommended for
	// interactive widgets.
	ID string

	// Resource is an image file path, SVG source string, or data URI.
	// Interpreted by the rendering backend.
	Resource string

	UID uint64 // internal use only

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

	// IDFocus > 0 makes the widget focusable. The numeric value
	// determines tab order — lower values receive focus first.
	// Focused widgets receive keyboard events.
	IDFocus uint32

	// IDScroll > 0 makes the widget respond to scroll events
	// (mouse wheel, trackpad). Used with ScrollMode to control
	// which axes scroll.
	IDScroll uint32

	// IDScrollContainer identifies a parent scroll container.
	// When set, scroll events bubble up to the matching IDScroll.
	IDScrollContainer uint32

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
	VAlign               VerticalAlign
	ScrollMode           ScrollMode
	ScrollbarOrientation ScrollbarOrientation
	TextDir              TextDirection

	// FloatAnchor is the attachment point on the floating element.
	// Combined with FloatTieOff to position the element relative
	// to its parent. See FloatAttach constants.
	FloatAnchor FloatAttach

	// FloatTieOff is the attachment point on the parent that the
	// floating element anchors to. See FloatAttach constants.
	FloatTieOff FloatAttach

	// Clip scissor-clips children to this element's bounds.
	Clip bool

	// ClipContents enables stencil-buffer clipping for nested
	// containers. More expensive than Clip but supports nested
	// clipping hierarchies.
	ClipContents bool

	Disabled bool

	// Float removes the element from normal layout flow and
	// positions it relative to its parent via FloatAnchor,
	// FloatTieOff, and FloatOffset.
	Float bool

	// FloatAutoFlip flips the float position (e.g. left→right)
	// to keep the element within the window bounds.
	FloatAutoFlip bool

	FocusSkip bool

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
	// true, scrollbars may appear if IDScroll is set.
	Overflow bool

	// QuarterTurns rotates the element in 90° clockwise
	// increments (0–3). Rotation is applied around the element
	// center.
	QuarterTurns uint8
}

// NewShape returns a Shape with default field values.
func NewShape() *Shape {
	return &Shape{
		UID:     rand.Uint64(),
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
)

// TextDirection controls text/layout direction.
type TextDirection uint8

// TextDirection constants.
const (
	TextDirAuto TextDirection = iota // inherit from parent/global
	TextDirLTR
	TextDirRTL
)

// ScrollMode allows scrolling in one or both directions.
type ScrollMode uint8

// ScrollMode constants.
const (
	ScrollBoth ScrollMode = iota
	ScrollVerticalOnly
	ScrollHorizontalOnly
)

// ScrollbarOrientation determines scrollbar orientation.
type ScrollbarOrientation uint8

// ScrollbarOrientation constants.
const (
	ScrollbarNone ScrollbarOrientation = iota
	ScrollbarVertical
	ScrollbarHorizontal
)

// FloatAttach defines anchor points for floating elements.
type FloatAttach uint8

// FloatAttach constants.
const (
	FloatTopLeft FloatAttach = iota
	FloatTopCenter
	FloatTopRight
	FloatMiddleLeft
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
type ShapeTextConfig struct {
	textLayoutStyle    TextStyle
	TextStyle          *TextStyle
	TextLayout         *glyph.Layout
	RtfRuns            *RichText
	RtfLayout          *glyph.Layout
	rtfGlyphRT         *glyph.RichText // cached conversion
	Text               string
	RtfFlatText        string // concatenation of all run texts; rune↔byte conversion for selection
	textLayoutText     string
	rtfMathHashes      []int64 // cache keys per inline math object
	RtfBaseStyle       glyph.TextStyle
	TextSelBeg         uint32
	TextSelEnd         uint32
	TextTabSize        uint32
	HangingIndent      float32
	MarkdownID         uint32 // non-zero when this RTF block belongs to a markdown widget
	MarkdownBlockStart uint32 // rune offset of this block within the markdown flat text
	MarkdownRuneLen    uint32 // rune count of this block's flat text
	wrapCacheWidth     float32
	wrapCacheHeight    float32
	textLayoutWidth    float32
	TextMode           TextMode
	TextIsPassword     bool
	TextIsPlaceholder  bool
	wrapCacheValid     bool
	textLayoutValid    bool
	textLayoutMode     TextMode
}

// hasRtfLayout returns true if the shape has an RTF layout.
func (s *Shape) hasRtfLayout() bool {
	return s.TC != nil && s.TC.RtfLayout != nil
}

// TextMode controls how a text view renders text.
type TextMode uint8

// TextMode constants.
const (
	TextModeSingleLine TextMode = iota
	TextModeMultiline
	TextModeWrap
	TextModeWrapKeepSpaces
)

// eventHandlers holds optional event callback fields.
type eventHandlers struct {
	OnChar        func(*Layout, *Event, *Window)
	OnKeyDown     func(*Layout, *Event, *Window)
	OnKeyUp       func(*Layout, *Event, *Window)
	OnClick       func(*Layout, *Event, *Window)
	OnMouseMove   func(*Layout, *Event, *Window)
	OnMouseUp     func(*Layout, *Event, *Window)
	OnMouseScroll func(*Layout, *Event, *Window)
	OnScroll      func(*Layout, *Window)
	AmendLayout   func(*Layout, *Window)
	OnHover       func(*Layout, *Event, *Window)
	OnMouseLeave  func(*Layout, *Event, *Window)
	OnGesture     func(*Layout, *Event, *Window)
	OnFileDrop    func(*Layout, *Event, *Window)
	OnIMECommit   func(*Layout, string, *Window)
	OnDraw        func(*DrawContext)
}

// shapeButtonColors holds per-button color state read by
// package-level button event handlers, avoiding per-frame
// closure allocations.
type shapeButtonColors struct {
	OnHover          func(*Layout, *Event, *Window)
	OnAmend          func(*Layout, *Window)
	ColorHover       Color
	ColorClick       Color
	ColorFocus       Color
	ColorBorderFocus Color
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
}

// GradientType specifies the gradient algorithm.
type GradientType uint8

// GradientType constants.
const (
	GradientLinear GradientType = iota
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
	Angle     float32 // explicit angle in degrees
	Type      GradientType
	Direction GradientDirection
	HasAngle  bool // true when Angle overrides Direction
}

// AccessInfo holds string accessibility data.
type AccessInfo struct {
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
func makeA11YInfo(label, desc string) *AccessInfo {
	if label == "" && desc == "" {
		return nil
	}
	return &AccessInfo{Label: label, Description: desc}
}

// a11yLabel returns label if set, otherwise falls back to text.
func a11yLabel(label, text string) string {
	if label != "" {
		return label
	}
	return text
}
