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
	// Colors carries the list's state colors. Colors.BorderFocus is
	// the border while the list holds focus: a ListBox is focusable
	// and key-navigable but drew no focus ring at all, so a keyboard
	// user could not tell which list they were in (issue #335).
	Colors      ColorSet
	ColorSelect Color
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
	Colors        ColorSet
}

// DialogStyle defines dialog visual properties.
// exportaudit:keep — reachable from an exported signature
type DialogStyle struct {
	titleTextStyle TextStyle
	TextStyle      TextStyle
	Shadow         *BoxShadow
	Padding        Padding
	SizeBorder     float32
	Radius         float32
	radiusBorder   float32 // Reserved.
	BlurRadius     float32
	MinWidth       float32
	MaxWidth       float32
	// Colors.BorderFocus is reserved for future focus-ring styling.
	Colors       ColorSet
	AlignButtons HorizontalAlign
}

// ToastAnchor specifies toast notification position.
// exportaudit:keep — caller-facing config (#698).
type ToastAnchor uint8

// ToastAnchor constants. One keep marker per member: the audit reads
// the marker off each declared name.
const (
	// exportaudit:keep — caller-facing config (#698).
	ToastTopLeft ToastAnchor = iota
	// exportaudit:keep — caller-facing config (#698).
	ToastTopRight
	// exportaudit:keep — caller-facing config (#698).
	ToastBottomLeft
	// exportaudit:keep — caller-facing config (#698).
	ToastBottomRight
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
	Colors       ColorSet
	colorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
	Anchor       ToastAnchor
}

// TooltipStyle defines tooltip visual properties.
// exportaudit:keep — reachable from an exported signature
type TooltipStyle struct {
	TextStyle  TextStyle
	Shadow     *BoxShadow
	Delay      time.Duration
	Padding    Padding
	SizeBorder float32
	Radius     float32
	Colors     ColorSet
}

// BadgeStyle defines badge visual properties.
// exportaudit:keep — reachable from an exported signature
type BadgeStyle struct {
	TextStyle    TextStyle
	Padding      Padding
	dotSize      float32
	Colors       ColorSet
	colorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
}

// ExpandPanelStyle defines expand panel visual properties.
// exportaudit:keep — reachable from an exported signature
type ExpandPanelStyle struct {
	// Colors.BorderFocus is the header's ring while the panel holds
	// focus.
	Colors       ColorSet
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
