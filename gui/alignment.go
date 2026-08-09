package gui

// Axis defines if a Layout arranges children horizontally,
// vertically, or not at all.
// exportaudit:keep — lowercase axis is pervasive as a local (layout code)
type Axis uint8

// Axis constants.
const (
	axisNone        Axis = iota
	axisTopToBottom      // vertical
	axisLeftToRight      // horizontal
)

// HorizontalAlign specifies horizontal alignment.
type HorizontalAlign uint8

// HorizontalAlign constants.
const (
	HAlignStart HorizontalAlign = iota // culture-dependent
	HAlignEnd                          // culture-dependent
	HAlignCenter
	HAlignLeft  // always left
	HAlignRight // always right
)

// VerticalAlign specifies vertical alignment.
type verticalAlign uint8

// VerticalAlign constants.
const (
	VAlignTop verticalAlign = iota
	VAlignMiddle
	VAlignBottom
)

// TextAlignment specifies horizontal text alignment.
type TextAlignment uint8

// TextAlignment constants.
const (
	TextAlignLeft TextAlignment = iota
	TextAlignCenter
	TextAlignRight
)
