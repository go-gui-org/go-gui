package gui

import "github.com/go-gui-org/go-glyph"

// Theme default colors (dark theme).
var (
	colorBackgroundDark = RGB(48, 48, 48)
	colorPanelDark      = RGB(64, 64, 64)
	colorInteriorDark   = RGB(74, 74, 74)
	colorHoverDark      = RGB(84, 84, 84)
	colorFocusDark      = RGB(94, 94, 94)
	colorActiveDark     = RGB(104, 104, 104)
	colorBorderDark     = RGB(100, 100, 100)
	colorSelectDark     = RGB(65, 105, 225)
	colorTextDark       = RGB(225, 225, 225)
)

// Radius constants.
const (
	radiusNone   float32 = 0
	radiusSmall  float32 = 3.5
	radiusMedium float32 = 5.5
	radiusLarge  float32 = 7.5
)

// Text size constants.
const (
	sizeTextMedium float32 = 16
	sizeTextTiny   float32 = sizeTextMedium - 6
	sizeTextXSmall float32 = sizeTextMedium - 4
	sizeTextSmall  float32 = sizeTextMedium - 2
	sizeTextLarge  float32 = sizeTextMedium + 4
	sizeTextXLarge float32 = sizeTextMedium + 8
	sizeBorderDef  float32 = 1.5
)

// Spacing constants.
const (
	// exportaudit:keep — const name collides with the spacingSmall helper
	SpacingSmall  float32 = 5
	SpacingMedium float32 = 10
	SpacingLarge  float32 = 15
)

// TextStyle defines text rendering properties.
type TextStyle struct {
	AffineTransform *glyph.AffineTransform
	Gradient        *glyph.GradientConfig
	Features        *glyph.FontFeatures
	Family          string
	Typeface        glyph.Typeface
	Size            float32
	LineSpacing     float32
	LetterSpacing   float32
	RotationRadians float32
	StrokeWidth     float32
	// EmojiBoxWidth, when > 0, makes color/emoji glyphs scale to fill this
	// box width (logical px), preserving aspect and centered. Grid callers
	// (terminals) set it to cells×cellWidth. 0 = default emoji sizing.
	EmojiBoxWidth float32
	// CellWidth and CellHeight are the grid cell size in logical px. Grid callers
	// (terminals) set them so the built-in box-drawing and block-element glyphs
	// fill the cell exactly and neighbouring cells abut. 0 = derive the cell from
	// the glyph advance and the run's ascent+descent, which leaves a sub-pixel
	// overlap wherever the advance is fractional.
	CellWidth   float32
	CellHeight  float32
	Color       Color
	BgColor     Color
	StrokeColor Color
	Align       TextAlignment
	Underline   bool
	// NoBuiltinBoxGlyphs keeps the font's own glyphs for U+2500–257F and
	// U+2580–259F instead of the built-in procedural rasterizer.
	NoBuiltinBoxGlyphs bool
	Strikethrough      bool
	// disabledRole marks a style whose Color already expresses the
	// disabled state, because it came from Theme.TextStyleDisabled.
	//
	// layoutDisables (gui/layout.go) stamps Disabled onto every
	// descendant of a disabled shape, and renderText then halves the
	// alpha of anything carrying it. A widget that had already applied
	// the themed disabled color therefore got dimmed twice, which is
	// why tab text rendered at alpha 65 while breadcrumb text — same
	// state, same themed value — rendered at 130 (issue #335). This
	// flag lets the renderer skip the second dim.
	//
	// Unexported and set only by themeTextRoles, so it cannot be
	// spelled at a call site: a widget states the role, never the
	// mechanism. It rides along through the ordinary struct copies
	// widgets make of a TextStyle.
	disabledRole bool
}

// mergeTextStyle fills zero fields in s from fallback.
func mergeTextStyle(s, fallback TextStyle) TextStyle {
	if !s.Color.IsSet() {
		s.Color = fallback.Color
	}
	if s.Size == 0 {
		s.Size = fallback.Size
	}
	return s
}

// ToGlyphStyle converts a gui TextStyle to a glyph.TextStyle.
func (ts TextStyle) toGlyphStyle() glyph.TextStyle {
	return glyph.TextStyle{
		FontName:      ts.Family,
		Color:         glyph.Color{R: ts.Color.R, G: ts.Color.G, B: ts.Color.B, A: ts.Color.A},
		BgColor:       glyph.Color{R: ts.BgColor.R, G: ts.BgColor.G, B: ts.BgColor.B, A: ts.BgColor.A},
		Size:          ts.Size,
		LetterSpacing: ts.LetterSpacing,
		EmojiBoxWidth: ts.EmojiBoxWidth,
		// Grid geometry and the box-glyph opt-out are rasterization inputs, like
		// EmojiBoxWidth; glyph inherits them per run from the base style.
		CellWidth:          ts.CellWidth,
		CellHeight:         ts.CellHeight,
		NoBuiltinBoxGlyphs: ts.NoBuiltinBoxGlyphs,
		Features:           ts.Features,
		Underline:          ts.Underline,
		Strikethrough:      ts.Strikethrough,
		Typeface:           ts.Typeface,
		StrokeWidth:        ts.StrokeWidth,
		StrokeColor:        glyph.Color{R: ts.StrokeColor.R, G: ts.StrokeColor.G, B: ts.StrokeColor.B, A: ts.StrokeColor.A},
	}
}

// HasTextTransform reports whether the style applies a non-identity transform.
func (ts TextStyle) hasTextTransform() bool {
	if ts.AffineTransform != nil {
		return !affineTransformIsIdentity(*ts.AffineTransform)
	}
	return ts.RotationRadians != 0
}

// EffectiveTextTransform returns the explicit affine transform when present,
// otherwise a rotation-derived transform, otherwise the identity transform.
func (ts TextStyle) effectiveTextTransform() glyph.AffineTransform {
	if ts.AffineTransform != nil {
		return *ts.AffineTransform
	}
	if ts.RotationRadians != 0 {
		return glyph.AffineRotation(ts.RotationRadians)
	}
	return glyph.AffineIdentity()
}

func affineTransformIsIdentity(t glyph.AffineTransform) bool {
	return t.XX == 1 && t.XY == 0 &&
		t.YX == 0 && t.YY == 1 &&
		t.X0 == 0 && t.Y0 == 0
}

// ButtonStyle defines button visual properties.
type buttonStyle struct {
	Shadow           *BoxShadow
	Gradient         *GradientDef
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	BlurRadius       float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	colorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
}

// ContainerStyle defines container visual properties.
type containerStyle struct {
	Shadow         *BoxShadow
	Gradient       *GradientDef
	BorderGradient *GradientDef
	Padding        Padding
	Radius         float32
	BlurRadius     float32
	Spacing        float32
	SizeBorder     float32
	Color          Color
	ColorBorder    Color
}

// RectangleStyle defines rectangle visual properties.
// exportaudit:keep — reachable from an exported signature
type RectangleStyle struct {
	Shadow         *BoxShadow
	Gradient       *GradientDef
	BorderGradient *GradientDef
	Radius         float32
	BlurRadius     float32
	SizeBorder     float32
	Color          Color
	ColorBorder    Color
}

// DefaultTextStyle is a ThemeMaker *input*, not a mirror: baseDarkCfg
// reads it while building ThemeDark, which happens during init before
// any theme is installed, so it needs a real literal. applyTheme
// re-assigns it afterwards (a no-op for ThemeDark).
var DefaultTextStyle = TextStyle{
	Family: defaultFontFamily,
	Color:  colorTextDark,
	Size:   sizeTextMedium,
}

// Widget style mirrors (issue #300). ThemeMaker is the only source of
// these values: init() fills them via applyTheme(ThemeDark) and
// (*Window).installTheme refills them whenever the active theme changes.
//
// Never give one an initializer. A literal here is a second source of
// truth that silently drifts from ThemeMaker — which is exactly the bug
// this block used to carry (button/input/container borders were 1.5 here
// and 0 in ThemeDark, and DefaultDataGridStyle stayed unstyled until an
// app happened to call SetTheme).
var (
	defaultButtonStyle    buttonStyle
	defaultContainerStyle containerStyle

	DefaultDataGridStyle DataGridStyle
)

// DataGridStyle defines data grid visual properties.
// exportaudit:keep — reachable from an exported signature
type DataGridStyle struct {
	TextStyle        TextStyle
	TextStyleHeader  TextStyle
	TextStyleFilter  TextStyle
	PaddingCell      Padding
	PaddingHeader    Padding
	PaddingFilter    Padding
	SizeBorder       float32
	Radius           float32
	ColorBackground  Color
	ColorHeader      Color
	ColorHeaderHover Color
	// exportaudit:keep — reachable from an exported signature
	ColorFilter       Color
	ColorQuickFilter  Color
	ColorRowHover     Color
	ColorRowAlt       Color
	ColorRowSelected  Color
	ColorBorder       Color
	ColorResizeHandle Color
	ColorResizeActive Color
}

// InspectorStyle defines the look and feel of the GUI inspector.
// exportaudit:keep — reachable from an exported signature
type InspectorStyle struct {
	ColorPanel     Color
	colorTextHelp  Color
	colorTextProp  Color
	colorWireframe Color
	colorPadding   Color
}

// defaultInspectorStyle provides the default inspector color palette.
// Like DefaultTextStyle it is a ThemeMaker input (ThemeMaker copies it
// into every theme's inspectorStyle), so it keeps a real literal.
var defaultInspectorStyle = InspectorStyle{
	ColorPanel:     RGBA(64, 64, 64, 245),
	colorTextHelp:  RGBA(225, 225, 225, 130),
	colorTextProp:  RGBA(220, 160, 60, 255),
	colorWireframe: RGBA(0, 255, 255, 200),
	colorPadding:   RGBA(0, 200, 0, 150),
}
