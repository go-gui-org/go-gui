package gui

// InputStyle defines input field visual properties.
// exportaudit:keep — reachable from an exported signature
type InputStyle struct {
	textStyleNormal  TextStyle
	PlaceholderStyle TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	colorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	colorSpellError  Color
}

// ScrollbarStyle defines scrollbar visual properties.
// exportaudit:keep — reachable from an exported signature
type ScrollbarStyle struct {
	Size            float32
	minThumbSize    float32
	colorThumb      Color
	ColorBackground Color
	Radius          float32
	radiusThumb     float32
	GapEdge         float32
	GapEnd          float32
}

// RadioStyle defines radio button visual properties.
// exportaudit:keep — reachable from an exported signature
type RadioStyle struct {
	textStyleNormal  TextStyle
	Padding          Padding
	Size             float32
	SizeBorder       float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	colorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	colorUnselect    Color
}

// SwitchStyle defines switch toggle visual properties.
// exportaudit:keep — reachable from an exported signature
type SwitchStyle struct {
	textStyleNormal  TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	sizeWidth        float32
	sizeHeight       float32
	SizeBorder       float32
	Radius           float32
	Color            Color
	colorClick       Color
	ColorFocus       Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	colorUnselect    Color
}

// ToggleStyle defines toggle button visual properties.
// exportaudit:keep — reachable from an exported signature
type ToggleStyle struct {
	textStyleNormal TextStyle
	textStyleLabel  TextStyle
	Padding         Padding
	// Size is the square edge length of the check box. Fixed on both
	// axes so the box stays square instead of shrinking to the width
	// of the check glyph.
	Size             float32
	SizeBorder       float32
	Radius           float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	colorClick       Color
	ColorFocus       Color
	ColorHover       Color
	ColorSelect      Color
}

// SelectStyle defines select dropdown visual properties.
// exportaudit:keep — reachable from an exported signature
type SelectStyle struct {
	textStyleNormal  TextStyle
	subheadingStyle  TextStyle
	PlaceholderStyle TextStyle
	Padding          Padding
	MinWidth         float32
	MaxWidth         float32
	SizeBorder       float32
	Radius           float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	colorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
}

// Widget style mirrors. See the note on the mirror block in styles.go:
// ThemeMaker is the only source of these values — never add an
// initializer here.
var (
	defaultInputStyle InputStyle

	DefaultScrollbarStyle ScrollbarStyle

	defaultRadioStyle RadioStyle

	defaultSwitchStyle SwitchStyle

	defaultToggleStyle ToggleStyle

	defaultSelectStyle SelectStyle
)
