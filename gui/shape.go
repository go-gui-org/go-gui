package gui

import (
	"math/rand/v2"

	"github.com/mike-ward/go-glyph"
)

// Shape is the only data structure used to draw to the screen.
type Shape struct {
	UID uint64 // internal use only

	// String fields
	ID       string // unique identifier assigned by the user
	Resource string // image path or SVG source

	Version     uint64 // for cache invalidation (DrawCanvas, etc.)
	FloatZIndex int

	// Optional sub-structs (nil when unused)
	Events  *EventHandlers     // event handlers
	TC      *ShapeTextConfig   // text/RTF fields
	FX      *ShapeEffects      // visual effects
	A11Y    *AccessInfo        // accessibility metadata
	BC      *ShapeButtonColors // button hover/focus colors
	SvgOpts *SvgParseOpts      // per-render SVG parse overrides

	// Structs
	ShapeClip DrawClip // calculated clipping rectangle
	Padding   Padding  // inner spacing
	Sizing    Sizing   // sizing logic

	// Numeric fields
	X            float32 // final calculated X position (absolute)
	Y            float32 // final calculated Y position (absolute)
	Width        float32
	MinWidth     float32
	MaxWidth     float32
	Height       float32
	MinHeight    float32
	MaxHeight    float32
	Radius       float32 // corner radius
	Spacing      float32 // spacing between children
	FloatOffsetX float32
	FloatOffsetY float32

	IDFocus           uint32 // >0 means focusable; value = tab order
	IDScroll          uint32 // >0 means receives scroll events
	IDScrollContainer uint32

	Color       Color
	ColorBorder Color
	SizeBorder  float32

	// Accessibility
	A11YRole  AccessRole
	A11YState AccessState

	// Enums/bools
	Axis                 Axis
	ShapeType            ShapeType
	HAlign               HorizontalAlign
	VAlign               VerticalAlign
	ScrollMode           ScrollMode
	ScrollbarOrientation ScrollbarOrientation
	TextDir              TextDirection
	FloatAnchor          FloatAttach
	FloatTieOff          FloatAttach

	Clip          bool
	ClipContents  bool
	Disabled      bool
	Float         bool
	FloatAutoFlip bool // flip float position to stay in window
	FocusSkip     bool
	OverDraw      bool
	Hero          bool
	Wrap          bool
	Overflow      bool
	QuarterTurns  uint8 // 0-3: rotation in 90° CW increments
	Opacity       float32
}

// NewShape returns a Shape with default field values.
func NewShape() *Shape {
	return &Shape{
		UID:     rand.Uint64(),
		Opacity: 1.0,
	}
}

// ShapeType defines the kind of Shape.
type ShapeType uint8

// ShapeType constants.
const (
	ShapeNone ShapeType = iota
	ShapeRectangle
	ShapeText
	ShapeImage
	ShapeCircle
	ShapeRTF
	ShapeSVG
	ShapeDrawCanvas
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

// DrawClip represents a clipping rectangle.
type DrawClip struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// ShapeTextConfig holds text/RTF-specific fields for a Shape.
type ShapeTextConfig struct {
	Text              string
	TextStyle         *TextStyle
	TextMode          TextMode
	TextSelBeg        uint32
	TextSelEnd        uint32
	TextTabSize       uint32
	TextIsPassword    bool
	TextIsPlaceholder bool
	HangingIndent     float32
	TextLayout        *glyph.Layout
	RtfRuns           *RichText
	RtfLayout         *glyph.Layout
	RtfBaseStyle      glyph.TextStyle
	rtfGlyphRT        *glyph.RichText // cached conversion
	rtfMathHashes     []int64         // cache keys per inline math object
	wrapCacheWidth    float32
	wrapCacheValid    bool
	wrapCacheHeight   float32
	textLayoutWidth   float32
	textLayoutValid   bool
	textLayoutText    string
	textLayoutStyle   TextStyle
	textLayoutMode    TextMode
}

// HasRtfLayout returns true if the shape has an RTF layout.
func (s *Shape) HasRtfLayout() bool {
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

// EventHandlers holds optional event callback fields.
type EventHandlers struct {
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

// ShapeButtonColors holds per-button color state read by
// package-level button event handlers, avoiding per-frame
// closure allocations.
type ShapeButtonColors struct {
	ColorHover       Color
	ColorClick       Color
	ColorFocus       Color
	ColorBorderFocus Color
	OnHover          func(*Layout, *Event, *Window)
}

// ShapeEffects holds optional visual effect fields.
type ShapeEffects struct {
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
	Type      GradientType
	Direction GradientDirection
	Angle     float32 // explicit angle in degrees
	HasAngle  bool    // true when Angle overrides Direction
}

// AccessInfo holds string accessibility data.
type AccessInfo struct {
	Label       string
	Description string
	ValueNum    float32
	ValueMin    float32
	ValueMax    float32
}

// HasEvents returns true if EventHandlers is allocated.
func (s *Shape) HasEvents() bool {
	return s.Events != nil
}

// PointInShape determines if the given point is within ShapeClip.
func (s *Shape) PointInShape(x, y float32) bool {
	sc := s.ShapeClip
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

// PaddingWidth returns total horizontal padding.
func (s *Shape) PaddingWidth() float32 {
	return s.Padding.Width() + (s.SizeBorder * 2)
}

// PaddingHeight returns total vertical padding.
func (s *Shape) PaddingHeight() float32 {
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
