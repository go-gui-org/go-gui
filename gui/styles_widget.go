package gui

// InputStyle defines input field visual properties.
type InputStyle struct {
	TextStyleNormal  TextStyle
	PlaceholderStyle TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSpellError  Color
}

// ScrollbarStyle defines scrollbar visual properties.
type ScrollbarStyle struct {
	Size            float32
	MinThumbSize    float32
	ColorThumb      Color
	ColorBackground Color
	Radius          float32
	RadiusThumb     float32
	GapEdge         float32
	GapEnd          float32
}

// RadioStyle defines radio button visual properties.
type RadioStyle struct {
	TextStyleNormal  TextStyle
	Padding          Padding
	Size             float32
	SizeBorder       float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	ColorUnselect    Color
}

// RadioGroupStyle defines radio button group visual properties.
type RadioGroupStyle struct {
	SizeBorder float32
}

// SwitchStyle defines switch toggle visual properties.
type SwitchStyle struct {
	TextStyleNormal  TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	SizeWidth        float32
	SizeHeight       float32
	SizeBorder       float32
	Radius           float32
	Color            Color
	ColorClick       Color
	ColorFocus       Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	ColorUnselect    Color
}

// ToggleStyle defines toggle button visual properties.
type ToggleStyle struct {
	TextStyleNormal TextStyle
	TextStyleLabel  TextStyle
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
	ColorClick       Color
	ColorFocus       Color
	ColorHover       Color
	ColorSelect      Color
}

// SelectStyle defines select dropdown visual properties.
type SelectStyle struct {
	TextStyleNormal  TextStyle
	SubheadingStyle  TextStyle
	PlaceholderStyle TextStyle
	Padding          Padding
	MinWidth         float32
	MaxWidth         float32
	SizeBorder       float32
	Radius           float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
}

// Default widget styles (dark theme).
var (
	DefaultInputStyle = InputStyle{
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorActiveDark,
		ColorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSpellError:  RGBA(255, 80, 80, 220),
		Padding:          PaddingSmall,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusMedium,
		TextStyleNormal:  DefaultTextStyle,
		PlaceholderStyle: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 100),
			Size:  SizeTextMedium,
		},
	}

	DefaultScrollbarStyle = ScrollbarStyle{
		Size:            7,
		MinThumbSize:    20,
		ColorThumb:      colorActiveDark,
		ColorBackground: ColorTransparent,
		Radius:          RadiusSmall,
		RadiusThumb:     RadiusSmall,
		GapEdge:         3,
		GapEnd:          2,
	}

	DefaultRadioStyle = RadioStyle{
		Size:             SizeTextMedium,
		Color:            colorPanelDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorSelectDark,
		ColorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		ColorUnselect:    colorActiveDark,
		Padding:          PadAll(4),
		SizeBorder:       2.0,
		TextStyleNormal:  DefaultTextStyle,
	}

	DefaultSwitchStyle = SwitchStyle{
		SizeWidth:        36,
		SizeHeight:       22,
		Color:            colorPanelDark,
		ColorClick:       colorInteriorDark,
		ColorFocus:       colorFocusDark,
		ColorHover:       colorHoverDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		ColorUnselect:    colorActiveDark,
		Padding:          paddingThree,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusLarge * 2,
		TextStyleNormal:  DefaultTextStyle,
	}

	DefaultToggleStyle = ToggleStyle{
		Color:            colorPanelDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorClick:       colorInteriorDark,
		ColorFocus:       colorActiveDark,
		ColorHover:       colorHoverDark,
		ColorSelect:      colorInteriorDark,
		Padding:          NewPadding(1, 1, 1, 2),
		Size:             SizeTextMedium + 4,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusSmall,
		TextStyleNormal:  DefaultTextStyle,
		TextStyleLabel:   DefaultTextStyle,
	}

	DefaultSelectStyle = SelectStyle{
		MinWidth:         75,
		MaxWidth:         200,
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorFocusDark,
		ColorClick:       colorActiveDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		Padding:          PaddingSmall,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusMedium,
		TextStyleNormal:  DefaultTextStyle,
		SubheadingStyle:  DefaultTextStyle,
		PlaceholderStyle: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 100),
			Size:  SizeTextMedium,
		},
	}
)
