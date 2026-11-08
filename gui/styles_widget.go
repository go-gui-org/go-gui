package gui

// InputStyle defines input field visual properties.
type inputStyle struct {
	textStyleNormal  TextStyle
	PlaceholderStyle TextStyle
	Shadow           *BoxShadow
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	Colors           ColorSet
	colorSpellError  Color
}

// ScrollbarStyle defines scrollbar visual properties.
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

// SeparatorStyle defines divider rule visual properties.
type separatorStyle struct {
	Colors ColorSet
	Size   float32
}

// RadioStyle defines radio button visual properties.
type radioStyle struct {
	textStyleNormal TextStyle
	textStyleLabel  TextStyle
	Padding         Padding
	Size            float32
	SizeBorder      float32
	Colors          ColorSet
	ColorSelect     Color
	colorUnselect   Color
	// colorDot paints the selected radio's center dot over ColorSelect.
	colorDot Color
}

// SwitchStyle defines switch toggle visual properties.
type switchStyle struct {
	textStyleNormal TextStyle
	textStyleLabel  TextStyle
	Shadow          *BoxShadow
	Padding         Padding
	sizeWidth       float32
	sizeHeight      float32
	SizeBorder      float32
	Radius          float32
	Colors          ColorSet
	ColorSelect     Color
	colorUnselect   Color
}

// ToggleStyle defines toggle button visual properties.
type toggleStyle struct {
	textStyleNormal TextStyle
	textStyleLabel  TextStyle
	Padding         Padding
	// Size is the square edge length of the check box. Fixed on both
	// axes so the box stays square instead of shrinking to the width
	// of the check glyph.
	Size        float32
	SizeBorder  float32
	Radius      float32
	Colors      ColorSet
	ColorSelect Color
}

// SelectStyle defines select dropdown visual properties.
type selectStyle struct {
	textStyleNormal  TextStyle
	subheadingStyle  TextStyle
	PlaceholderStyle TextStyle
	// Shadow lifts the open dropdown off the content behind it. Nil
	// on a theme that does not describe elevation, which is every
	// theme that predates ThemeCfg.shadowPopover.
	Shadow      *BoxShadow
	Padding     Padding
	MinWidth    float32
	MaxWidth    float32
	SizeBorder  float32
	Radius      float32
	Colors      ColorSet
	ColorSelect Color
	// ColorSelectSubtle is the tint behind the highlighted dropdown
	// row — the wash, never the full accent slab; focus is the ring,
	// not a second fill (visual-refresh §4.3).
	ColorSelectSubtle Color
}

// Widget style mirrors. See the note on the mirror block in styles.go:
// ThemeMaker is the only source of these values — never add an
// initializer here.
var (
	defaultInputStyle inputStyle

	DefaultScrollbarStyle ScrollbarStyle

	defaultRadioStyle radioStyle

	defaultSwitchStyle switchStyle

	defaultToggleStyle toggleStyle

	defaultSelectStyle selectStyle

	defaultSeparatorStyle separatorStyle
)
