package gui

import "time"

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

// Widget style mirrors. See the note on the mirror block in styles.go:
// ThemeMaker is the only source of these values — never add an
// initializer here.
var (
	defaultListBoxStyle ListBoxStyle

	defaultTreeStyle TreeStyle

	DefaultDialogStyle DialogStyle

	defaultToastStyle ToastStyle

	defaultTooltipStyle TooltipStyle

	defaultBadgeStyle BadgeStyle

	defaultExpandPanelStyle ExpandPanelStyle
)
