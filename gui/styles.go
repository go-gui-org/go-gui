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
	RadiusNone   float32 = 0
	RadiusSmall  float32 = 3.5
	RadiusMedium float32 = 5.5
	RadiusLarge  float32 = 7.5
)

// Text size constants.
const (
	SizeTextMedium float32 = 16
	SizeTextTiny   float32 = SizeTextMedium - 6
	SizeTextXSmall float32 = SizeTextMedium - 4
	SizeTextSmall  float32 = SizeTextMedium - 2
	SizeTextLarge  float32 = SizeTextMedium + 4
	SizeTextXLarge float32 = SizeTextMedium + 8
	SizeBorderDef  float32 = 1.5
)

// Spacing constants.
const (
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
	Color           Color
	BgColor         Color
	StrokeColor     Color
	Align           TextAlignment
	Underline       bool
	Strikethrough   bool
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
func (ts TextStyle) ToGlyphStyle() glyph.TextStyle {
	return glyph.TextStyle{
		FontName:      ts.Family,
		Color:         glyph.Color{R: ts.Color.R, G: ts.Color.G, B: ts.Color.B, A: ts.Color.A},
		BgColor:       glyph.Color{R: ts.BgColor.R, G: ts.BgColor.G, B: ts.BgColor.B, A: ts.BgColor.A},
		Size:          ts.Size,
		LetterSpacing: ts.LetterSpacing,
		Features:      ts.Features,
		Underline:     ts.Underline,
		Strikethrough: ts.Strikethrough,
		Typeface:      ts.Typeface,
		StrokeWidth:   ts.StrokeWidth,
		StrokeColor:   glyph.Color{R: ts.StrokeColor.R, G: ts.StrokeColor.G, B: ts.StrokeColor.B, A: ts.StrokeColor.A},
	}
}

// HasTextTransform reports whether the style applies a non-identity transform.
func (ts TextStyle) HasTextTransform() bool {
	if ts.AffineTransform != nil {
		return !affineTransformIsIdentity(*ts.AffineTransform)
	}
	return ts.RotationRadians != 0
}

// EffectiveTextTransform returns the explicit affine transform when present,
// otherwise a rotation-derived transform, otherwise the identity transform.
func (ts TextStyle) EffectiveTextTransform() glyph.AffineTransform {
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
type ButtonStyle struct {
	Shadow           *BoxShadow
	Gradient         *GradientDef
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	BlurRadius       float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
}

// ContainerStyle defines container visual properties.
type ContainerStyle struct {
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

// Default styles (dark theme).
var (
	DefaultTextStyle = TextStyle{
		Family: defaultFontFamily,
		Color:  colorTextDark,
		Size:   SizeTextMedium,
	}

	DefaultButtonStyle = ButtonStyle{
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorActiveDark,
		ColorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		Padding:          PaddingButton,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusMedium,
	}

	DefaultContainerStyle = ContainerStyle{
		Color:       ColorTransparent,
		ColorBorder: ColorTransparent,
		Padding:     PaddingMedium,
		Radius:      RadiusMedium,
		Spacing:     SpacingMedium,
		SizeBorder:  SizeBorderDef,
	}

	DefaultRectangleStyle = RectangleStyle{
		Color:       ColorTransparent,
		ColorBorder: colorBorderDark,
		Radius:      RadiusMedium,
		SizeBorder:  SizeBorderDef,
	}

	DefaultDataGridStyle DataGridStyle
)

// DataGridStyle defines data grid visual properties.
type DataGridStyle struct {
	TextStyle         TextStyle
	TextStyleHeader   TextStyle
	TextStyleFilter   TextStyle
	PaddingCell       Padding
	PaddingHeader     Padding
	PaddingFilter     Padding
	SizeBorder        float32
	Radius            float32
	ColorBackground   Color
	ColorHeader       Color
	ColorHeaderHover  Color
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
type InspectorStyle struct {
	ColorPanel     Color
	ColorTextHelp  Color
	ColorTextProp  Color
	ColorWireframe Color
	ColorPadding   Color
}

// DefaultInspectorStyle provides the default inspector color palette.
var DefaultInspectorStyle = InspectorStyle{
	ColorPanel:     RGBA(64, 64, 64, 245),
	ColorTextHelp:  RGBA(225, 225, 225, 130),
	ColorTextProp:  RGBA(220, 160, 60, 255),
	ColorWireframe: RGBA(0, 255, 255, 200),
	ColorPadding:   RGBA(0, 200, 0, 150),
}
