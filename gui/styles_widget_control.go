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
	Colors         ColorSet
	colorBar       Color
	textBackground Color
	TextShow       bool
}

// SliderStyle defines slider visual properties.
// exportaudit:keep — reachable from an exported signature
type SliderStyle struct {
	Size       float32
	ThumbSize  float32
	Colors     ColorSet
	colorThumb Color
	colorLeft  Color
	Padding    Padding
	SizeBorder float32
	Radius     float32
}

// TabControlStyle defines tab control visual properties.
// exportaudit:keep — reachable from an exported signature
type TabControlStyle struct {
	TextStyle          TextStyle
	textStyleSelected  TextStyle
	textStyleDisabled  TextStyle
	Padding            Padding
	PaddingHeader      Padding
	paddingContent     Padding
	paddingTab         Padding
	SizeBorder         float32
	sizeHeaderBorder   float32
	sizeContentBorder  float32
	sizeTabBorder      float32
	Radius             float32
	radiusHeader       float32
	radiusContent      float32
	radiusTab          float32
	Spacing            float32
	spacingHeader      float32
	Colors             ColorSet
	ColorHeader        Color
	colorHeaderBorder  Color
	colorContent       Color
	colorContentBorder Color
	// ColorsTab themes the tabs, the selected and disabled tab
	// included (ColorsTab.Selected, ColorsTab.Disabled; #741).
	ColorsTab ColorSet
}

// segmentedControlStyle defines segmented control visual properties
// (issue #600). Unexported, like buttonStyle: nothing outside the
// package names it, and a theme customizes it through ThemeCfg
// tokens (#735).
//
// The control is a track with an inset: the selected segment is its
// own rounded fill (the pill) inside the track, never a flush fill.
// The renderer has one radius per rect, so a flush fill with rounded
// outer corners only is not drawable without per-corner radius.
type segmentedControlStyle struct {
	textStyle         TextStyle
	textStyleSelected TextStyle
	textStyleDisabled TextStyle
	textStyleIcon     TextStyle
	// padding is the track's inset: the gap between the track edge
	// and the pill.
	padding Padding
	// paddingSegment is the text inset inside a segment. It is
	// Theme.PaddingField, so the control shares a row height with an
	// Input or a Select beside it.
	paddingSegment Padding
	sizeBorder     float32
	sizeDivider    float32
	radius         float32
	radiusSegment  float32
	// colors themes the track: Base is the track fill, Border and
	// BorderFocus its outline.
	colors ColorSet
	// colorsSegment themes one segment. Base is transparent so the
	// track shows through; Selected is the pill.
	colorsSegment ColorSet
	colorDivider  Color
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
	Colors             ColorSet
	colorTrail         Color
	// ColorsCrumb themes the crumbs, the current and disabled crumb
	// included (ColorsCrumb.Selected, ColorsCrumb.Disabled; #741).
	ColorsCrumb        ColorSet
	colorContent       Color
	colorContentBorder Color
}

// SplitterStyle defines splitter visual properties.
// exportaudit:keep — reachable from an exported signature
type SplitterStyle struct {
	HandleSize    float32
	dragStep      float32
	dragStepLarge float32
	// ColorsHandle themes the drag handle; ColorsButton themes the
	// collapse buttons. Active maps to Click: a drag is a held
	// press (issue #720).
	ColorsHandle    ColorSet
	ColorsButton    ColorSet
	colorGrip       Color
	colorButtonIcon Color
	SizeBorder      float32
	Radius          float32
	radiusBorder    float32
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
	Colors             ColorSet
	ColorSelect        Color
	alignHead          HorizontalAlign
}

// ComboboxStyle defines combobox visual properties.
// exportaudit:keep — reachable from an exported signature
type ComboboxStyle struct {
	TextStyle        TextStyle
	PlaceholderStyle TextStyle
	// Shadow lifts the open dropdown off the content behind it.
	Shadow            *BoxShadow
	Padding           Padding
	SizeBorder        float32
	Radius            float32
	MinWidth          float32
	MaxWidth          float32
	maxDropdownHeight float32
	Colors            ColorSet
	ColorHighlight    Color
	// ColorHighlightSubtle is the tint behind the highlighted
	// dropdown row: the wash, never the full accent slab — focus is
	// the ring, not a second fill (visual-refresh §4.3).
	ColorHighlightSubtle Color
}

// CommandPaletteStyle defines command palette visual properties.
// exportaudit:keep — reachable from an exported signature
type CommandPaletteStyle struct {
	TextStyle   TextStyle
	detailStyle TextStyle
	// Shadow lifts the palette off the backdrop. Modal tier, not
	// popover tier — it floats further than a menu.
	Shadow         *BoxShadow
	SizeBorder     float32
	Radius         float32
	Width          float32
	MaxHeight      float32
	Colors         ColorSet
	ColorHighlight Color
	// ColorHighlightSubtle is the tint behind the highlighted row:
	// the wash, never the full accent slab (visual-refresh §4.3).
	ColorHighlightSubtle Color
	backdropColor        Color
}

// MenubarStyle defines menubar visual properties.
// exportaudit:keep — reachable from an exported signature
type MenubarStyle struct {
	TextStyle         TextStyle
	textStyleSubtitle TextStyle
	// Shadow lifts an open menu surface — a popup menu or a submenu —
	// off the content behind it. The menubar strip itself is flush
	// with the window chrome and takes no elevation, so this is
	// applied only where the menu actually floats.
	Shadow          *BoxShadow
	Padding         Padding
	paddingMenuItem Padding
	paddingSubmenu  Padding
	paddingSubtitle Padding
	widthSubmenuMin float32
	widthSubmenuMax float32
	SizeBorder      float32
	Radius          float32
	radiusBorder    float32
	radiusSubmenu   float32
	radiusMenuItem  float32
	Spacing         float32
	spacingSubmenu  float32
	Colors          ColorSet
	ColorSelect     Color
	// ColorTextOnSelect is the text color drawn over the selected
	// menu item. Resolved by ThemeMaker; unset themes keep body text.
	ColorTextOnSelect Color
}

// DatePickerStyle defines date picker visual properties.
// exportaudit:keep — reachable from an exported signature
type DatePickerStyle struct {
	TextStyle    TextStyle
	Shadow       *BoxShadow
	Padding      Padding
	cellSpacing  float32
	SizeBorder   float32
	Radius       float32
	radiusBorder float32
	Colors       ColorSet
	ColorSelect  Color
	// ColorTextOnSelect is the text color drawn over ColorSelect
	// fills. Resolved by ThemeMaker; unset themes keep body text.
	ColorTextOnSelect    Color
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool
	WeekdaysLen          DatePickerWeekdayLen
}

// ColorPickerStyle defines color picker visual properties.
// exportaudit:keep — reachable from an exported signature
type ColorPickerStyle struct {
	TextStyle     TextStyle
	SizeBorder    float32
	Radius        float32
	sVSize        float32
	sliderHeight  float32
	indicatorSize float32
	Colors        ColorSet
}

// SkeletonStyle defines skeleton loader visual properties.
// exportaudit:keep — reachable from an exported signature
type SkeletonStyle struct {
	Colors         ColorSet
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

	defaultSegmentedControlStyle segmentedControlStyle

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
