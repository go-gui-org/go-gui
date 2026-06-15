package gui

import "github.com/go-gui-org/go-glyph"

// ProgressBarStyle defines progress bar visual properties.
type ProgressBarStyle struct {
	TextStyle      TextStyle
	Padding        Padding
	TextPadding    Padding
	Size           float32
	SizeBorder     float32
	Radius         float32
	Color          Color
	ColorBar       Color
	ColorBorder    Color
	TextBackground Color
	TextShow       bool
}

// SliderStyle defines slider visual properties.
type SliderStyle struct {
	Size             float32
	ThumbSize        float32
	Color            Color
	ColorClick       Color
	ColorThumb       Color
	ColorLeft        Color
	ColorFocus       Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	Padding          Padding
	SizeBorder       float32
	Radius           float32
}

// TabControlStyle defines tab control visual properties.
type TabControlStyle struct {
	TextStyle           TextStyle
	TextStyleSelected   TextStyle
	TextStyleDisabled   TextStyle
	Padding             Padding
	PaddingHeader       Padding
	PaddingContent      Padding
	PaddingTab          Padding
	SizeBorder          float32
	SizeHeaderBorder    float32
	SizeContentBorder   float32
	SizeTabBorder       float32
	Radius              float32
	RadiusHeader        float32
	RadiusContent       float32
	RadiusTab           float32
	Spacing             float32
	SpacingHeader       float32
	Color               Color
	ColorBorder         Color
	ColorHeader         Color
	ColorHeaderBorder   Color
	ColorContent        Color
	ColorContentBorder  Color
	ColorTab            Color
	ColorTabHover       Color
	ColorTabFocus       Color
	ColorTabClick       Color
	ColorTabSelected    Color
	ColorTabDisabled    Color
	ColorTabBorder      Color
	ColorTabBorderFocus Color
}

// BreadcrumbStyle defines breadcrumb visual properties.
type BreadcrumbStyle struct {
	TextStyle          TextStyle
	TextStyleSelected  TextStyle
	TextStyleDisabled  TextStyle
	TextStyleSeparator TextStyle
	Separator          string
	Padding            Padding
	PaddingTrail       Padding
	PaddingCrumb       Padding
	PaddingContent     Padding
	Radius             float32
	RadiusCrumb        float32
	RadiusContent      float32
	Spacing            float32
	SpacingTrail       float32
	SizeBorder         float32
	SizeContentBorder  float32
	Color              Color
	ColorBorder        Color
	ColorTrail         Color
	ColorCrumb         Color
	ColorCrumbHover    Color
	ColorCrumbClick    Color
	ColorCrumbSelected Color
	ColorCrumbDisabled Color
	ColorContent       Color
	ColorContentBorder Color
}

// SplitterStyle defines splitter visual properties.
type SplitterStyle struct {
	HandleSize        float32
	DragStep          float32
	DragStepLarge     float32
	ColorHandle       Color
	ColorHandleHover  Color
	ColorHandleActive Color
	ColorHandleBorder Color
	ColorGrip         Color
	ColorButton       Color
	ColorButtonHover  Color
	ColorButtonActive Color
	ColorButtonIcon   Color
	SizeBorder        float32
	Radius            float32
	RadiusBorder      float32
}

// TableStyle defines table visual properties.
type TableStyle struct {
	TextStyle          TextStyle
	TextStyleHead      TextStyle
	CellPadding        Padding
	ColumnWidthDefault float32
	ColumnWidthMin     float32
	SizeBorder         float32
	ColorBorder        Color
	ColorSelect        Color
	ColorHover         Color
	AlignHead          HorizontalAlign
}

// ComboboxStyle defines combobox visual properties.
type ComboboxStyle struct {
	TextStyle         TextStyle
	PlaceholderStyle  TextStyle
	Padding           Padding
	SizeBorder        float32
	Radius            float32
	MinWidth          float32
	MaxWidth          float32
	MaxDropdownHeight float32
	Color             Color
	ColorHover        Color
	ColorFocus        Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorHighlight    Color
}

// CommandPaletteStyle defines command palette visual properties.
type CommandPaletteStyle struct {
	TextStyle      TextStyle
	DetailStyle    TextStyle
	SizeBorder     float32
	Radius         float32
	Width          float32
	MaxHeight      float32
	Color          Color
	ColorBorder    Color
	ColorHighlight Color
	BackdropColor  Color
}

// MenubarStyle defines menubar visual properties.
type MenubarStyle struct {
	TextStyle         TextStyle
	TextStyleSubtitle TextStyle
	Padding           Padding
	PaddingMenuItem   Padding
	PaddingSubmenu    Padding
	PaddingSubtitle   Padding
	WidthSubmenuMin   float32
	WidthSubmenuMax   float32
	SizeBorder        float32
	Radius            float32
	RadiusBorder      float32
	RadiusSubmenu     float32
	RadiusMenuItem    float32
	Spacing           float32
	SpacingSubmenu    float32
	Color             Color
	ColorHover        Color
	ColorFocus        Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorSelect       Color
}

// DatePickerStyle defines date picker visual properties.
type DatePickerStyle struct {
	TextStyle            TextStyle
	Shadow               *BoxShadow
	Padding              Padding
	CellSpacing          float32
	SizeBorder           float32
	Radius               float32
	RadiusBorder         float32
	Color                Color
	ColorHover           Color
	ColorFocus           Color
	ColorClick           Color
	ColorBorder          Color
	ColorBorderFocus     Color
	ColorSelect          Color
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool
	WeekdaysLen          DatePickerWeekdayLen
}

// ColorPickerStyle defines color picker visual properties.
type ColorPickerStyle struct {
	TextStyle        TextStyle
	SizeBorder       float32
	Radius           float32
	SVSize           float32
	SliderHeight     float32
	IndicatorSize    float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
}

// SkeletonStyle defines skeleton loader visual properties.
type SkeletonStyle struct {
	Color          Color
	ColorHighlight Color
	Radius         float32
}

// Default widget styles (dark theme).
var (
	DefaultProgressBarStyle = ProgressBarStyle{
		Size:           20,
		Color:          colorInteriorDark,
		ColorBar:       colorSelectDark,
		ColorBorder:    colorBorderDark,
		TextBackground: ColorTransparent,
		Padding:        PaddingNone,
		TextPadding:    NewPadding(1, 4, 1, 4),
		SizeBorder:     0,
		Radius:         RadiusSmall,
		TextShow:       true,
		TextStyle:      DefaultTextStyle,
	}

	DefaultSliderStyle = SliderStyle{
		Size:             6,
		ThumbSize:        16,
		Color:            colorInteriorDark,
		ColorClick:       colorActiveDark,
		ColorThumb:       colorPanelDark,
		ColorLeft:        colorSelectDark,
		ColorFocus:       colorSelectDark,
		ColorHover:       colorHoverDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		Padding:          PaddingNone,
		SizeBorder:       1,
		Radius:           3,
	}

	DefaultTabControlStyle = TabControlStyle{
		Color:               colorPanelDark,
		ColorBorder:         colorBorderDark,
		ColorHeader:         ColorTransparent,
		ColorHeaderBorder:   ColorTransparent,
		ColorContent:        colorPanelDark,
		ColorContentBorder:  colorBorderDark,
		ColorTab:            colorInteriorDark,
		ColorTabHover:       colorHoverDark,
		ColorTabFocus:       colorFocusDark,
		ColorTabClick:       colorActiveDark,
		ColorTabSelected:    colorSelectDark,
		ColorTabDisabled:    colorPanelDark,
		ColorTabBorder:      colorBorderDark,
		ColorTabBorderFocus: colorSelectDark,
		Padding:             PaddingNone,
		PaddingHeader:       PaddingNone,
		PaddingContent:      PaddingMedium,
		PaddingTab:          PaddingSmall,
		SizeBorder:          SizeBorderDef,
		SizeTabBorder:       SizeBorderDef,
		Radius:              RadiusMedium,
		RadiusHeader:        RadiusSmall,
		RadiusContent:       RadiusMedium,
		RadiusTab:           RadiusSmall,
		SpacingHeader:       2,
		TextStyle:           DefaultTextStyle,
		TextStyleSelected: TextStyle{
			Color: colorTextDark,
			Size:  SizeTextMedium,
		},
		TextStyleDisabled: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 130),
			Size:  SizeTextMedium,
		},
	}

	DefaultBreadcrumbStyle = BreadcrumbStyle{
		Separator:          "/",
		Color:              ColorTransparent,
		ColorBorder:        ColorTransparent,
		ColorTrail:         ColorTransparent,
		ColorCrumb:         ColorTransparent,
		ColorCrumbHover:    colorHoverDark,
		ColorCrumbClick:    colorActiveDark,
		ColorCrumbSelected: ColorTransparent,
		ColorCrumbDisabled: ColorTransparent,
		ColorContent:       colorPanelDark,
		ColorContentBorder: colorBorderDark,
		Padding:            PaddingNone,
		PaddingTrail:       PaddingSmall,
		PaddingCrumb:       NewPadding(2, 4, 2, 4),
		PaddingContent:     PaddingMedium,
		Radius:             RadiusMedium,
		RadiusCrumb:        RadiusSmall,
		RadiusContent:      RadiusMedium,
		Spacing:            SpacingSmall,
		SpacingTrail:       SpacingSmall,
		SizeContentBorder:  SizeBorderDef,
		TextStyle:          DefaultTextStyle,
		TextStyleSelected: TextStyle{
			Color: colorTextDark,
			Size:  SizeTextMedium,
		},
		TextStyleDisabled: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 130),
			Size:  SizeTextMedium,
		},
		TextStyleSeparator: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 160),
			Size:  SizeTextMedium,
		},
	}

	DefaultSplitterStyle = SplitterStyle{
		HandleSize:        9,
		DragStep:          0.02,
		DragStepLarge:     0.10,
		ColorHandle:       colorInteriorDark,
		ColorHandleHover:  colorHoverDark,
		ColorHandleActive: colorActiveDark,
		ColorHandleBorder: colorBorderDark,
		ColorGrip:         colorSelectDark,
		ColorButton:       colorInteriorDark,
		ColorButtonHover:  colorHoverDark,
		ColorButtonActive: colorActiveDark,
		ColorButtonIcon:   colorTextDark,
		SizeBorder:        SizeBorderDef,
		Radius:            RadiusSmall,
		RadiusBorder:      RadiusSmall,
	}

	DefaultTableStyle = TableStyle{
		ColorBorder: colorBorderDark,
		ColorSelect: colorSelectDark,
		ColorHover:  colorHoverDark,
		CellPadding: PaddingTwoFive,
		TextStyle:   DefaultTextStyle,
		TextStyleHead: TextStyle{
			Color:    DefaultTextStyle.Color,
			Size:     DefaultTextStyle.Size,
			Typeface: glyph.TypefaceBold,
		},
		AlignHead:          HAlignCenter,
		ColumnWidthDefault: 50,
		ColumnWidthMin:     20,
	}

	DefaultComboboxStyle = ComboboxStyle{
		Color:             colorInteriorDark,
		ColorHover:        colorHoverDark,
		ColorFocus:        colorInteriorDark,
		ColorBorder:       colorBorderDark,
		ColorBorderFocus:  colorSelectDark,
		ColorHighlight:    colorSelectDark,
		Padding:           PaddingSmall,
		SizeBorder:        SizeBorderDef,
		Radius:            RadiusMedium,
		MinWidth:          75,
		MaxWidth:          200,
		MaxDropdownHeight: 200,
		TextStyle:         DefaultTextStyle,
		PlaceholderStyle: TextStyle{
			Color: RGBA(colorTextDark.R, colorTextDark.G, colorTextDark.B, 100),
			Size:  SizeTextMedium,
		},
	}

	DefaultCommandPaletteStyle = CommandPaletteStyle{
		Color:          colorPanelDark,
		ColorBorder:    colorBorderDark,
		ColorHighlight: colorSelectDark,
		SizeBorder:     SizeBorderDef,
		Radius:         RadiusMedium,
		Width:          500,
		MaxHeight:      400,
		TextStyle:      DefaultTextStyle,
		DetailStyle: TextStyle{
			Color: RGBA(128, 128, 128, 200),
			Size:  SizeTextMedium,
		},
		BackdropColor: RGBA(0, 0, 0, 120),
	}

	DefaultDatePickerStyle = DatePickerStyle{
		CellSpacing:      2,
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
		RadiusBorder:     RadiusMedium,
		TextStyle:        DefaultTextStyle,
	}

	DefaultColorPickerStyle = ColorPickerStyle{
		Color:            colorInteriorDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusMedium,
		SVSize:           200,
		SliderHeight:     24,
		IndicatorSize:    16,
		TextStyle:        DefaultTextStyle,
	}

	DefaultSkeletonStyle = SkeletonStyle{
		Color:          colorInteriorDark,
		ColorHighlight: colorInteriorDark.Add(RGBA(20, 20, 20, 0)),
		Radius:         RadiusSmall,
	}

	DefaultMenubarStyle = MenubarStyle{
		WidthSubmenuMin:  50,
		WidthSubmenuMax:  200,
		Color:            colorInteriorDark,
		ColorHover:       colorHoverDark,
		ColorFocus:       colorFocusDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		ColorSelect:      colorSelectDark,
		Padding:          PaddingSmall,
		PaddingMenuItem:  PaddingTwoFive,
		PaddingSubmenu:   PaddingSmall,
		PaddingSubtitle:  NewPadding(0, PadSmall, 0, PadSmall),
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusSmall,
		RadiusBorder:     RadiusMedium,
		RadiusSubmenu:    RadiusSmall,
		RadiusMenuItem:   RadiusSmall,
		Spacing:          SpacingMedium,
		SpacingSubmenu:   0,
		TextStyle:        DefaultTextStyle,
		TextStyleSubtitle: TextStyle{
			Color: colorTextDark,
			Size:  SizeTextSmall,
		},
	}
)
