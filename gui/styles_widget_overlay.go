package gui

import (
	"time"

	"github.com/go-gui-org/go-glyph"
)

// ListBoxStyle defines list box visual properties.
type ListBoxStyle struct {
	TextStyleNormal TextStyle
	SubheadingStyle TextStyle
	Padding         Padding
	SizeBorder      float32
	Radius          float32
	Color           Color
	ColorHover      Color
	ColorBorder     Color
	ColorSelect     Color
}

// TreeStyle defines tree view visual properties.
type TreeStyle struct {
	TextStyle     TextStyle
	TextStyleIcon TextStyle
	Padding       Padding
	SizeBorder    float32
	Radius        float32
	Indent        float32
	Spacing       float32
	Color         Color
	ColorHover    Color
	ColorFocus    Color
	ColorBorder   Color
}

// DialogStyle defines dialog visual properties.
type DialogStyle struct {
	TitleTextStyle   TextStyle
	TextStyle        TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	RadiusBorder     float32 // Reserved.
	BlurRadius       float32
	MinWidth         float32
	MaxWidth         float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color // Reserved for future focus-ring styling.
	AlignButtons     HorizontalAlign
}

// ToastAnchor specifies toast notification position.
type ToastAnchor uint8

// ToastAnchor constants.
const (
	ToastTopLeft ToastAnchor = iota
	ToastTopRight
	ToastBottomLeft
	ToastBottomRight
)

// ToastStyle defines toast notification visual properties.
type ToastStyle struct {
	TextStyle    TextStyle
	TitleStyle   TextStyle
	Shadow       *BoxShadow
	MaxVisible   int
	Padding      Padding
	Width        float32
	Margin       float32
	Spacing      float32
	AccentWidth  float32
	Radius       float32
	SizeBorder   float32
	Color        Color
	ColorBorder  Color
	ColorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
	Anchor       ToastAnchor
}

// TooltipStyle defines tooltip visual properties.
type TooltipStyle struct {
	TextStyle   TextStyle
	Shadow      *BoxShadow
	Delay       time.Duration
	Padding     Padding
	SizeBorder  float32
	Radius      float32
	Color       Color
	ColorBorder Color
}

// BadgeStyle defines badge visual properties.
type BadgeStyle struct {
	TextStyle    TextStyle
	Padding      Padding
	DotSize      float32
	Color        Color
	ColorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
}

// ExpandPanelStyle defines expand panel visual properties.
type ExpandPanelStyle struct {
	Color        Color
	ColorHover   Color
	ColorClick   Color
	ColorBorder  Color
	Padding      Padding
	SizeBorder   float32
	Radius       float32
	RadiusBorder float32
}

// Default widget styles (dark theme).
var (
	DefaultListBoxStyle = ListBoxStyle{
		Color:           colorInteriorDark,
		ColorHover:      colorHoverDark,
		ColorBorder:     colorBorderDark,
		ColorSelect:     colorSelectDark,
		Padding:         PaddingButton,
		SizeBorder:      SizeBorderDef,
		Radius:          RadiusMedium,
		TextStyleNormal: DefaultTextStyle,
		SubheadingStyle: DefaultTextStyle,
	}

	DefaultTreeStyle = TreeStyle{
		Color:       ColorTransparent,
		ColorHover:  colorHoverDark,
		ColorFocus:  colorFocusDark,
		ColorBorder: ColorTransparent,
		Padding:     PaddingNone,
		SizeBorder:  SizeBorderDef,
		Radius:      RadiusMedium,
		TextStyle:   DefaultTextStyle,
		TextStyleIcon: TextStyle{
			Color:  colorTextDark,
			Size:   SizeTextSmall,
			Family: IconFontName,
		},
		Indent:  25,
		Spacing: 0,
	}

	DefaultDialogStyle = DialogStyle{
		Color:            colorPanelDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		Padding:          PaddingLarge,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusMedium,
		RadiusBorder:     RadiusMedium,
		AlignButtons:     HAlignCenter,
		TitleTextStyle: TextStyle{
			Color: colorTextDark,
			Size:  SizeTextLarge,
		},
		TextStyle: DefaultTextStyle,
	}

	DefaultToastStyle = ToastStyle{
		MaxVisible:   5,
		Anchor:       ToastBottomRight,
		Width:        260,
		Margin:       16,
		Spacing:      8,
		AccentWidth:  4,
		Padding:      PaddingMedium,
		Radius:       RadiusMedium,
		SizeBorder:   SizeBorderDef,
		Color:        colorPanelDark,
		ColorBorder:  colorBorderDark,
		ColorInfo:    colorSelectDark,
		ColorSuccess: RGBA(46, 160, 67, 255),
		ColorWarning: RGBA(210, 153, 34, 255),
		ColorError:   RGBA(218, 54, 51, 255),
		TextStyle:    DefaultTextStyle,
		TitleStyle: TextStyle{
			Color:    colorTextDark,
			Size:     SizeTextMedium,
			Typeface: glyph.TypefaceBold,
		},
	}

	DefaultTooltipStyle = TooltipStyle{
		Delay:       500 * time.Millisecond,
		Color:       colorInteriorDark,
		ColorBorder: colorBorderDark,
		Padding:     PaddingSmall,
		SizeBorder:  SizeBorderDef,
		Radius:      RadiusSmall,
		TextStyle:   DefaultTextStyle,
	}

	DefaultBadgeStyle = BadgeStyle{
		Color:        colorSelectDark,
		ColorInfo:    colorSelectDark,
		ColorSuccess: RGBA(46, 160, 67, 255),
		ColorWarning: RGBA(210, 153, 34, 255),
		ColorError:   RGBA(218, 54, 51, 255),
		Padding:      NewPadding(2, 6, 2, 6),
		DotSize:      8,
	}

	DefaultExpandPanelStyle = ExpandPanelStyle{
		Color:        colorPanelDark,
		ColorHover:   colorHoverDark,
		ColorClick:   colorActiveDark,
		ColorBorder:  colorBorderDark,
		Padding:      PaddingMedium,
		SizeBorder:   SizeBorderDef,
		Radius:       RadiusMedium,
		RadiusBorder: RadiusMedium,
	}
)
