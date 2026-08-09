package gui

import (
	"time"

	"github.com/go-gui-org/go-glyph"
)

// ListBoxStyle defines list box visual properties.
// exportaudit:keep — reachable from an exported signature
type ListBoxStyle struct {
	textStyleNormal TextStyle
	subheadingStyle TextStyle
	Padding         Padding
	SizeBorder      float32
	Radius          float32
	Color           Color
	ColorHover      Color
	ColorBorder     Color
	ColorSelect     Color
}

// TreeStyle defines tree view visual properties.
// exportaudit:keep — reachable from an exported signature
type TreeStyle struct {
	TextStyle     TextStyle
	textStyleIcon TextStyle
	Padding       Padding
	SizeBorder    float32
	Radius        float32
	indent        float32
	Spacing       float32
	Color         Color
	ColorHover    Color
	ColorFocus    Color
	ColorBorder   Color
}

// DialogStyle defines dialog visual properties.
// exportaudit:keep — reachable from an exported signature
type DialogStyle struct {
	titleTextStyle   TextStyle
	TextStyle        TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	radiusBorder     float32 // Reserved.
	BlurRadius       float32
	MinWidth         float32
	MaxWidth         float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color // Reserved for future focus-ring styling.
	AlignButtons     HorizontalAlign
}

// ToastAnchor specifies toast notification position.
type toastAnchor uint8

// ToastAnchor constants.
const (
	toastTopLeft toastAnchor = iota
	toastTopRight
	toastBottomLeft
	toastBottomRight
)

// ToastStyle defines toast notification visual properties.
// exportaudit:keep — reachable from an exported signature
type ToastStyle struct {
	TextStyle    TextStyle
	TitleStyle   TextStyle
	Shadow       *BoxShadow
	maxVisible   int
	Padding      Padding
	Width        float32
	margin       float32
	Spacing      float32
	accentWidth  float32
	Radius       float32
	SizeBorder   float32
	Color        Color
	ColorBorder  Color
	colorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
	Anchor       toastAnchor
}

// TooltipStyle defines tooltip visual properties.
// exportaudit:keep — reachable from an exported signature
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
// exportaudit:keep — reachable from an exported signature
type BadgeStyle struct {
	TextStyle    TextStyle
	Padding      Padding
	dotSize      float32
	Color        Color
	colorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
}

// ExpandPanelStyle defines expand panel visual properties.
// exportaudit:keep — reachable from an exported signature
type ExpandPanelStyle struct {
	Color        Color
	ColorHover   Color
	colorClick   Color
	ColorBorder  Color
	Padding      Padding
	SizeBorder   float32
	Radius       float32
	radiusBorder float32
}

// Default widget styles (dark theme).
var (
	defaultListBoxStyle = ListBoxStyle{
		Color:           colorInteriorDark,
		ColorHover:      colorHoverDark,
		ColorBorder:     colorBorderDark,
		ColorSelect:     colorSelectDark,
		Padding:         paddingButton,
		SizeBorder:      sizeBorderDef,
		Radius:          radiusMedium,
		textStyleNormal: DefaultTextStyle,
		subheadingStyle: DefaultTextStyle,
	}

	defaultTreeStyle = TreeStyle{
		Color:       ColorTransparent,
		ColorHover:  colorHoverDark,
		ColorFocus:  colorFocusDark,
		ColorBorder: ColorTransparent,
		Padding:     PaddingNone,
		SizeBorder:  sizeBorderDef,
		Radius:      radiusMedium,
		TextStyle:   DefaultTextStyle,
		textStyleIcon: TextStyle{
			Color:  colorTextDark,
			Size:   sizeTextSmall,
			Family: IconFontName,
		},
		indent:  25,
		Spacing: 0,
	}

	DefaultDialogStyle = DialogStyle{
		Color:            colorPanelDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		Padding:          PaddingLarge,
		SizeBorder:       sizeBorderDef,
		Radius:           radiusMedium,
		radiusBorder:     radiusMedium,
		AlignButtons:     HAlignCenter,
		titleTextStyle: TextStyle{
			Color: colorTextDark,
			Size:  sizeTextLarge,
		},
		TextStyle: DefaultTextStyle,
	}

	defaultToastStyle = ToastStyle{
		maxVisible:   5,
		Anchor:       toastBottomRight,
		Width:        260,
		margin:       16,
		Spacing:      8,
		accentWidth:  4,
		Padding:      paddingMedium,
		Radius:       radiusMedium,
		SizeBorder:   sizeBorderDef,
		Color:        colorPanelDark,
		ColorBorder:  colorBorderDark,
		colorInfo:    colorSelectDark,
		ColorSuccess: RGBA(46, 160, 67, 255),
		ColorWarning: RGBA(210, 153, 34, 255),
		ColorError:   RGBA(218, 54, 51, 255),
		TextStyle:    DefaultTextStyle,
		TitleStyle: TextStyle{
			Color:    colorTextDark,
			Size:     sizeTextMedium,
			Typeface: glyph.TypefaceBold,
		},
	}

	defaultTooltipStyle = TooltipStyle{
		Delay:       500 * time.Millisecond,
		Color:       colorInteriorDark,
		ColorBorder: colorBorderDark,
		Padding:     PaddingSmall,
		SizeBorder:  sizeBorderDef,
		Radius:      radiusSmall,
		TextStyle:   DefaultTextStyle,
	}

	defaultBadgeStyle = BadgeStyle{
		Color:        colorSelectDark,
		colorInfo:    colorSelectDark,
		ColorSuccess: RGBA(46, 160, 67, 255),
		ColorWarning: RGBA(210, 153, 34, 255),
		ColorError:   RGBA(218, 54, 51, 255),
		Padding:      NewPadding(2, 6, 2, 6),
		dotSize:      8,
	}

	defaultExpandPanelStyle = ExpandPanelStyle{
		Color:        colorPanelDark,
		ColorHover:   colorHoverDark,
		colorClick:   colorActiveDark,
		ColorBorder:  colorBorderDark,
		Padding:      paddingMedium,
		SizeBorder:   sizeBorderDef,
		Radius:       radiusMedium,
		radiusBorder: radiusMedium,
	}
)
