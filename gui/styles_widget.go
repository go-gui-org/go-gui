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

// RadioGroupStyle defines radio button group visual properties.
type radioGroupStyle struct {
	SizeBorder float32
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

// Default widget styles (dark theme).
var (
	defaultInputStyle = InputStyle{
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorActiveDark,
		colorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		colorSpellError:  RGBA(255, 80, 80, 220),
		Padding:          PaddingSmall,
		SizeBorder:       sizeBorderDef,
		Radius:           radiusMedium,
		textStyleNormal:  DefaultTextStyle,
		PlaceholderStyle: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 100),
			Size:  sizeTextMedium,
		},
	}

	DefaultScrollbarStyle = ScrollbarStyle{
		Size:            7,
		minThumbSize:    20,
		colorThumb:      colorActiveDark,
		ColorBackground: ColorTransparent,
		Radius:          radiusSmall,
		radiusThumb:     radiusSmall,
		GapEdge:         3,
		GapEnd:          2,
	}

	defaultRadioStyle = RadioStyle{
		Size:             sizeTextMedium,
		Color:            colorPanelDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorSelectDark,
		colorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		colorUnselect:    colorActiveDark,
		Padding:          PadAll(4),
		SizeBorder:       2.0,
		textStyleNormal:  DefaultTextStyle,
	}

	defaultSwitchStyle = SwitchStyle{
		sizeWidth:        36,
		sizeHeight:       22,
		Color:            colorPanelDark,
		colorClick:       colorInteriorDark,
		ColorFocus:       colorFocusDark,
		ColorHover:       colorHoverDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		colorUnselect:    colorActiveDark,
		Padding:          paddingThree,
		SizeBorder:       sizeBorderDef,
		Radius:           radiusLarge * 2,
		textStyleNormal:  DefaultTextStyle,
	}

	defaultToggleStyle = ToggleStyle{
		Color:            colorPanelDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		colorClick:       colorInteriorDark,
		ColorFocus:       colorActiveDark,
		ColorHover:       colorHoverDark,
		ColorSelect:      colorInteriorDark,
		Padding:          NewPadding(1, 1, 1, 2),
		Size:             sizeTextMedium + 4,
		SizeBorder:       sizeBorderDef,
		Radius:           radiusSmall,
		textStyleNormal:  DefaultTextStyle,
		textStyleLabel:   DefaultTextStyle,
	}

	defaultSelectStyle = SelectStyle{
		MinWidth:         75,
		MaxWidth:         200,
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorFocusDark,
		colorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		Padding:          PaddingSmall,
		SizeBorder:       sizeBorderDef,
		Radius:           radiusMedium,
		textStyleNormal:  DefaultTextStyle,
		subheadingStyle:  DefaultTextStyle,
		PlaceholderStyle: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 100),
			Size:  sizeTextMedium,
		},
	}
)
