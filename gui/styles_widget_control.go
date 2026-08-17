package gui

// ProgressBarStyle defines progress bar visual properties.
// exportaudit:keep — reachable from an exported signature
type ProgressBarStyle struct {
	TextStyle      TextStyle
	Padding        Padding
	textPadding    Padding
	Size           float32
	SizeBorder     float32
	Radius         float32
	Color          Color
	colorBar       Color
	ColorBorder    Color
	textBackground Color
	TextShow       bool
}

// SliderStyle defines slider visual properties.
// exportaudit:keep — reachable from an exported signature
type SliderStyle struct {
	Size             float32
	ThumbSize        float32
	Color            Color
	colorClick       Color
	colorThumb       Color
	colorLeft        Color
	ColorFocus       Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	Padding          Padding
	SizeBorder       float32
	Radius           float32
}

// TabControlStyle defines tab control visual properties.
// exportaudit:keep — reachable from an exported signature
type TabControlStyle struct {
	TextStyle           TextStyle
	textStyleSelected   TextStyle
	textStyleDisabled   TextStyle
	Padding             Padding
	PaddingHeader       Padding
	paddingContent      Padding
	paddingTab          Padding
	SizeBorder          float32
	sizeHeaderBorder    float32
	sizeContentBorder   float32
	sizeTabBorder       float32
	Radius              float32
	radiusHeader        float32
	radiusContent       float32
	radiusTab           float32
	Spacing             float32
	spacingHeader       float32
	Color               Color
	ColorBorder         Color
	ColorHeader         Color
	colorHeaderBorder   Color
	colorContent        Color
	colorContentBorder  Color
	colorTab            Color
	colorTabHover       Color
	colorTabFocus       Color
	colorTabClick       Color
	colorTabSelected    Color
	colorTabDisabled    Color
	colorTabBorder      Color
	colorTabBorderFocus Color
}

// BreadcrumbStyle defines breadcrumb visual properties.
// exportaudit:keep — reachable from an exported signature
type BreadcrumbStyle struct {
	TextStyle          TextStyle
	textStyleSelected  TextStyle
	textStyleDisabled  TextStyle
	textStyleSeparator TextStyle
	Separator          string
	Padding            Padding
	paddingTrail       Padding
	paddingCrumb       Padding
	paddingContent     Padding
	Radius             float32
	radiusCrumb        float32
	radiusContent      float32
	Spacing            float32
	spacingTrail       float32
	SizeBorder         float32
	sizeContentBorder  float32
	Color              Color
	ColorBorder        Color
	colorTrail         Color
	colorCrumb         Color
	colorCrumbHover    Color
	colorCrumbClick    Color
	colorCrumbSelected Color
	colorCrumbDisabled Color
	colorContent       Color
	colorContentBorder Color
}

// SplitterStyle defines splitter visual properties.
// exportaudit:keep — reachable from an exported signature
type SplitterStyle struct {
	HandleSize        float32
	dragStep          float32
	dragStepLarge     float32
	colorHandle       Color
	colorHandleHover  Color
	colorHandleActive Color
	colorHandleBorder Color
	colorGrip         Color
	colorButton       Color
	colorButtonHover  Color
	colorButtonActive Color
	colorButtonIcon   Color
	SizeBorder        float32
	Radius            float32
	radiusBorder      float32
}

// TableStyle defines table visual properties.
// exportaudit:keep — reachable from an exported signature
type TableStyle struct {
	TextStyle          TextStyle
	TextStyleHead      TextStyle
	cellPadding        Padding
	columnWidthDefault float32
	columnWidthMin     float32
	SizeBorder         float32
	ColorBorder        Color
	ColorBorderFocus   Color
	ColorSelect        Color
	ColorHover         Color
	alignHead          HorizontalAlign
}

// ComboboxStyle defines combobox visual properties.
// exportaudit:keep — reachable from an exported signature
type ComboboxStyle struct {
	TextStyle         TextStyle
	PlaceholderStyle  TextStyle
	Padding           Padding
	SizeBorder        float32
	Radius            float32
	MinWidth          float32
	MaxWidth          float32
	maxDropdownHeight float32
	Color             Color
	ColorHover        Color
	ColorFocus        Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorHighlight    Color
}

// CommandPaletteStyle defines command palette visual properties.
// exportaudit:keep — reachable from an exported signature
type CommandPaletteStyle struct {
	TextStyle      TextStyle
	detailStyle    TextStyle
	SizeBorder     float32
	Radius         float32
	Width          float32
	MaxHeight      float32
	Color          Color
	ColorBorder    Color
	ColorHighlight Color
	backdropColor  Color
}

// MenubarStyle defines menubar visual properties.
// exportaudit:keep — reachable from an exported signature
type MenubarStyle struct {
	TextStyle         TextStyle
	textStyleSubtitle TextStyle
	Padding           Padding
	paddingMenuItem   Padding
	paddingSubmenu    Padding
	paddingSubtitle   Padding
	widthSubmenuMin   float32
	widthSubmenuMax   float32
	SizeBorder        float32
	Radius            float32
	radiusBorder      float32
	radiusSubmenu     float32
	radiusMenuItem    float32
	Spacing           float32
	spacingSubmenu    float32
	Color             Color
	ColorHover        Color
	ColorFocus        Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorSelect       Color
}

// DatePickerStyle defines date picker visual properties.
// exportaudit:keep — reachable from an exported signature
type DatePickerStyle struct {
	TextStyle            TextStyle
	Shadow               *BoxShadow
	Padding              Padding
	cellSpacing          float32
	SizeBorder           float32
	Radius               float32
	radiusBorder         float32
	Color                Color
	ColorHover           Color
	ColorFocus           Color
	colorClick           Color
	ColorBorder          Color
	ColorBorderFocus     Color
	ColorSelect          Color
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool
	WeekdaysLen          DatePickerWeekdayLen
}

// ColorPickerStyle defines color picker visual properties.
// exportaudit:keep — reachable from an exported signature
type ColorPickerStyle struct {
	TextStyle        TextStyle
	SizeBorder       float32
	Radius           float32
	sVSize           float32
	sliderHeight     float32
	indicatorSize    float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
}

// SkeletonStyle defines skeleton loader visual properties.
// exportaudit:keep — reachable from an exported signature
type SkeletonStyle struct {
	Color          Color
	ColorHighlight Color
	Radius         float32
}

// Widget style mirrors. See the note on the mirror block in styles.go:
// ThemeMaker is the only source of these values — never add an
// initializer here.
var (
	defaultProgressBarStyle ProgressBarStyle

	defaultSliderStyle SliderStyle

	defaultTabControlStyle TabControlStyle

	defaultBreadcrumbStyle BreadcrumbStyle

	defaultSplitterStyle SplitterStyle

	defaultTableStyle TableStyle

	defaultComboboxStyle ComboboxStyle

	defaultCommandPaletteStyle CommandPaletteStyle

	defaultDatePickerStyle DatePickerStyle

	defaultColorPickerStyle ColorPickerStyle

	defaultSkeletonStyle SkeletonStyle

	defaultMenubarStyle MenubarStyle
)
