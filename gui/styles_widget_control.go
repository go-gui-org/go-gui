package gui

import "github.com/go-gui-org/go-glyph"

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

// Default widget styles (dark theme).
var (
	defaultProgressBarStyle = ProgressBarStyle{
		Size:           20,
		Color:          colorInteriorDark,
		colorBar:       colorSelectDark,
		ColorBorder:    colorBorderDark,
		textBackground: ColorTransparent,
		Padding:        PaddingNone,
		textPadding:    NewPadding(1, 4, 1, 4),
		SizeBorder:     0,
		Radius:         radiusSmall,
		TextShow:       true,
		TextStyle:      DefaultTextStyle,
	}

	defaultSliderStyle = SliderStyle{
		Size:             6,
		ThumbSize:        16,
		Color:            colorInteriorDark,
		colorClick:       colorActiveDark,
		colorThumb:       colorPanelDark,
		colorLeft:        colorSelectDark,
		ColorFocus:       colorSelectDark,
		ColorHover:       colorHoverDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		Padding:          PaddingNone,
		SizeBorder:       1,
		Radius:           3,
	}

	defaultTabControlStyle = TabControlStyle{
		Color:               colorPanelDark,
		ColorBorder:         colorBorderDark,
		ColorHeader:         ColorTransparent,
		colorHeaderBorder:   ColorTransparent,
		colorContent:        colorPanelDark,
		colorContentBorder:  colorBorderDark,
		colorTab:            colorInteriorDark,
		colorTabHover:       colorHoverDark,
		colorTabFocus:       colorFocusDark,
		colorTabClick:       colorActiveDark,
		colorTabSelected:    colorSelectDark,
		colorTabDisabled:    colorPanelDark,
		colorTabBorder:      colorBorderDark,
		colorTabBorderFocus: colorSelectDark,
		Padding:             PaddingNone,
		PaddingHeader:       PaddingNone,
		paddingContent:      paddingMedium,
		paddingTab:          PaddingSmall,
		SizeBorder:          sizeBorderDef,
		sizeTabBorder:       sizeBorderDef,
		Radius:              radiusMedium,
		radiusHeader:        radiusSmall,
		radiusContent:       radiusMedium,
		radiusTab:           radiusSmall,
		spacingHeader:       2,
		TextStyle:           DefaultTextStyle,
		textStyleSelected: TextStyle{
			Color: colorTextDark,
			Size:  sizeTextMedium,
		},
		textStyleDisabled: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 130),
			Size:  sizeTextMedium,
		},
	}

	defaultBreadcrumbStyle = BreadcrumbStyle{
		Separator:          "/",
		Color:              ColorTransparent,
		ColorBorder:        ColorTransparent,
		colorTrail:         ColorTransparent,
		colorCrumb:         ColorTransparent,
		colorCrumbHover:    colorHoverDark,
		colorCrumbClick:    colorActiveDark,
		colorCrumbSelected: ColorTransparent,
		colorCrumbDisabled: ColorTransparent,
		colorContent:       colorPanelDark,
		colorContentBorder: colorBorderDark,
		Padding:            PaddingNone,
		paddingTrail:       PaddingSmall,
		paddingCrumb:       NewPadding(2, 4, 2, 4),
		paddingContent:     paddingMedium,
		Radius:             radiusMedium,
		radiusCrumb:        radiusSmall,
		radiusContent:      radiusMedium,
		Spacing:            SpacingSmall,
		spacingTrail:       SpacingSmall,
		sizeContentBorder:  sizeBorderDef,
		TextStyle:          DefaultTextStyle,
		textStyleSelected: TextStyle{
			Color: colorTextDark,
			Size:  sizeTextMedium,
		},
		textStyleDisabled: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 130),
			Size:  sizeTextMedium,
		},
		textStyleSeparator: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 160),
			Size:  sizeTextMedium,
		},
	}

	defaultSplitterStyle = SplitterStyle{
		HandleSize:        9,
		dragStep:          0.02,
		dragStepLarge:     0.10,
		colorHandle:       colorInteriorDark,
		colorHandleHover:  colorHoverDark,
		colorHandleActive: colorActiveDark,
		colorHandleBorder: colorBorderDark,
		colorGrip:         colorSelectDark,
		colorButton:       colorInteriorDark,
		colorButtonHover:  colorHoverDark,
		colorButtonActive: colorActiveDark,
		colorButtonIcon:   colorTextDark,
		SizeBorder:        sizeBorderDef,
		Radius:            radiusSmall,
		radiusBorder:      radiusSmall,
	}

	defaultTableStyle = TableStyle{
		ColorBorder: colorBorderDark,
		ColorSelect: colorSelectDark,
		ColorHover:  colorHoverDark,
		cellPadding: PaddingTwoFive,
		TextStyle:   DefaultTextStyle,
		TextStyleHead: TextStyle{
			Color:    DefaultTextStyle.Color,
			Size:     DefaultTextStyle.Size,
			Typeface: glyph.TypefaceBold,
		},
		alignHead:          HAlignCenter,
		columnWidthDefault: 50,
		columnWidthMin:     20,
	}

	defaultComboboxStyle = ComboboxStyle{
		Color:             colorInteriorDark,
		ColorHover:        colorHoverDark,
		ColorFocus:        colorInteriorDark,
		ColorBorder:       colorBorderDark,
		ColorBorderFocus:  colorSelectDark,
		ColorHighlight:    colorSelectDark,
		Padding:           PaddingSmall,
		SizeBorder:        sizeBorderDef,
		Radius:            radiusMedium,
		MinWidth:          75,
		MaxWidth:          200,
		maxDropdownHeight: 200,
		TextStyle:         DefaultTextStyle,
		PlaceholderStyle: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 100),
			Size:  sizeTextMedium,
		},
	}

	defaultCommandPaletteStyle = CommandPaletteStyle{
		Color:          colorPanelDark,
		ColorBorder:    colorBorderDark,
		ColorHighlight: colorSelectDark,
		SizeBorder:     sizeBorderDef,
		Radius:         radiusMedium,
		Width:          500,
		MaxHeight:      400,
		TextStyle:      DefaultTextStyle,
		detailStyle: TextStyle{
			Color: RGBA(128, 128, 128, 200),
			Size:  sizeTextMedium,
		},
		backdropColor: RGBA(0, 0, 0, 120),
	}

	defaultDatePickerStyle = DatePickerStyle{
		cellSpacing:      2,
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
		radiusBorder:     radiusMedium,
		TextStyle:        DefaultTextStyle,
	}

	defaultColorPickerStyle = ColorPickerStyle{
		Color:            colorInteriorDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		SizeBorder:       sizeBorderDef,
		Radius:           radiusMedium,
		sVSize:           200,
		sliderHeight:     24,
		indicatorSize:    16,
		TextStyle:        DefaultTextStyle,
	}

	defaultSkeletonStyle = SkeletonStyle{
		Color:          colorInteriorDark,
		ColorHighlight: colorInteriorDark.Add(RGBA(20, 20, 20, 0)),
		Radius:         radiusSmall,
	}

	defaultMenubarStyle = MenubarStyle{
		widthSubmenuMin:  50,
		widthSubmenuMax:  200,
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorFocusDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		Padding:          PaddingSmall,
		paddingMenuItem:  PaddingTwoFive,
		paddingSubmenu:   PaddingSmall,
		paddingSubtitle:  NewPadding(0, PadSmall, 0, PadSmall),
		SizeBorder:       sizeBorderDef,
		Radius:           radiusSmall,
		radiusBorder:     radiusMedium,
		radiusSubmenu:    radiusSmall,
		radiusMenuItem:   radiusSmall,
		Spacing:          SpacingMedium,
		spacingSubmenu:   0,
		TextStyle:        DefaultTextStyle,
		textStyleSubtitle: TextStyle{
			Color: colorTextDark,
			Size:  sizeTextSmall,
		},
	}
)
