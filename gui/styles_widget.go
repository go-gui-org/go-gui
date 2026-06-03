package gui

import (
	"time"

	"github.com/go-gui-org/go-glyph"
)

// InputStyle defines input field visual properties.
type InputStyle struct {
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSpellError  Color
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	Shadow           *BoxShadow
	TextStyleNormal  TextStyle
	PlaceholderStyle TextStyle
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
	Size             float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	ColorUnselect    Color
	Padding          Padding
	SizeBorder       float32
	TextStyleNormal  TextStyle
}

// RadioGroupStyle defines radio button group visual properties.
type RadioGroupStyle struct {
	SizeBorder float32
}

// SwitchStyle defines switch toggle visual properties.
type SwitchStyle struct {
	SizeWidth        float32
	SizeHeight       float32
	Color            Color
	ColorClick       Color
	ColorFocus       Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	ColorUnselect    Color
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	Shadow           *BoxShadow
	TextStyleNormal  TextStyle
}

// ToggleStyle defines toggle button visual properties.
type ToggleStyle struct {
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorClick       Color
	ColorFocus       Color
	ColorHover       Color
	ColorSelect      Color
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	TextStyleNormal  TextStyle
	TextStyleLabel   TextStyle
}

// SelectStyle defines select dropdown visual properties.
type SelectStyle struct {
	MinWidth         float32
	MaxWidth         float32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	TextStyleNormal  TextStyle
	SubheadingStyle  TextStyle
	PlaceholderStyle TextStyle
}

// ListBoxStyle defines list box visual properties.
type ListBoxStyle struct {
	Color           Color
	ColorHover      Color
	ColorBorder     Color
	ColorSelect     Color
	Padding         Padding
	SizeBorder      float32
	Radius          float32
	TextStyleNormal TextStyle
	SubheadingStyle TextStyle
}

// TreeStyle defines tree view visual properties.
type TreeStyle struct {
	Color         Color
	ColorHover    Color
	ColorFocus    Color
	ColorBorder   Color
	Padding       Padding
	SizeBorder    float32
	Radius        float32
	TextStyle     TextStyle
	TextStyleIcon TextStyle
	Indent        float32
	Spacing       float32
}

// DialogStyle defines dialog visual properties.
type DialogStyle struct {
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color // Reserved for future focus-ring styling.
	Padding          Padding
	SizeBorder       float32
	Radius           float32
	RadiusBorder     float32 // Reserved.
	BlurRadius       float32
	Shadow           *BoxShadow
	AlignButtons     HorizontalAlign
	MinWidth         float32
	MaxWidth         float32
	TitleTextStyle   TextStyle
	TextStyle        TextStyle
}

// ToastAnchor specifies toast notification position.
type ToastAnchor uint8

// ToastAnchor constants.
const (
	ToastTopLeft ToastAnchor = iota
	ToastTopRight
	ToastBottomLeft
	ToastBottomRight
)

// ToastStyle defines toast notification visual properties.
type ToastStyle struct {
	MaxVisible   int
	Anchor       ToastAnchor
	Width        float32
	Margin       float32
	Spacing      float32
	AccentWidth  float32
	Padding      Padding
	Radius       float32
	SizeBorder   float32
	Color        Color
	ColorBorder  Color
	ColorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
	TextStyle    TextStyle
	TitleStyle   TextStyle
	Shadow       *BoxShadow
}

// TooltipStyle defines tooltip visual properties.
type TooltipStyle struct {
	Delay       time.Duration
	Color       Color
	ColorBorder Color
	Padding     Padding
	SizeBorder  float32
	Radius      float32
	Shadow      *BoxShadow
	TextStyle   TextStyle
}

// BadgeStyle defines badge visual properties.
type BadgeStyle struct {
	Color        Color
	ColorInfo    Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
	Padding      Padding
	TextStyle    TextStyle
	DotSize      float32
}

// ExpandPanelStyle defines expand panel visual properties.
type ExpandPanelStyle struct {
	Color        Color
	ColorHover   Color
	ColorClick   Color
	ColorBorder  Color
	Padding      Padding
	SizeBorder   float32
	Radius       float32
	RadiusBorder float32
}

// ProgressBarStyle defines progress bar visual properties.
type ProgressBarStyle struct {
	Size           float32
	Color          Color
	ColorBar       Color
	ColorBorder    Color
	TextBackground Color
	Padding        Padding
	TextPadding    Padding
	SizeBorder     float32
	Radius         float32
	TextShow       bool
	TextStyle      TextStyle
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
	TextStyle           TextStyle
	TextStyleSelected   TextStyle
	TextStyleDisabled   TextStyle
}

// BreadcrumbStyle defines breadcrumb visual properties.
type BreadcrumbStyle struct {
	Separator          string
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
	TextStyle          TextStyle
	TextStyleSelected  TextStyle
	TextStyleDisabled  TextStyle
	TextStyleSeparator TextStyle
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
	ColorBorder        Color
	ColorSelect        Color
	ColorHover         Color
	CellPadding        Padding
	TextStyle          TextStyle
	TextStyleHead      TextStyle
	AlignHead          HorizontalAlign
	ColumnWidthDefault float32
	ColumnWidthMin     float32
	SizeBorder         float32
}

// ComboboxStyle defines combobox visual properties.
type ComboboxStyle struct {
	Color             Color
	ColorHover        Color
	ColorFocus        Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorHighlight    Color
	Padding           Padding
	SizeBorder        float32
	Radius            float32
	MinWidth          float32
	MaxWidth          float32
	MaxDropdownHeight float32
	TextStyle         TextStyle
	PlaceholderStyle  TextStyle
}

// CommandPaletteStyle defines command palette visual properties.
type CommandPaletteStyle struct {
	Color          Color
	ColorBorder    Color
	ColorHighlight Color
	SizeBorder     float32
	Radius         float32
	Width          float32
	MaxHeight      float32
	TextStyle      TextStyle
	DetailStyle    TextStyle
	BackdropColor  Color
}

// MenubarStyle defines menubar visual properties.
type MenubarStyle struct {
	WidthSubmenuMin   float32
	WidthSubmenuMax   float32
	Color             Color
	ColorHover        Color
	ColorFocus        Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorSelect       Color
	Padding           Padding
	PaddingMenuItem   Padding
	PaddingSubmenu    Padding
	PaddingSubtitle   Padding
	SizeBorder        float32
	Radius            float32
	RadiusBorder      float32
	RadiusSubmenu     float32
	RadiusMenuItem    float32
	Spacing           float32
	SpacingSubmenu    float32
	TextStyle         TextStyle
	TextStyleSubtitle TextStyle
}

// DatePickerStyle defines date picker visual properties.
type DatePickerStyle struct {
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool
	CellSpacing          float32
	WeekdaysLen          DatePickerWeekdayLen
	Color                Color
	ColorHover           Color
	ColorFocus           Color
	ColorClick           Color
	ColorBorder          Color
	ColorBorderFocus     Color
	ColorSelect          Color
	Padding              Padding
	SizeBorder           float32
	Radius               float32
	RadiusBorder         float32
	Shadow               *BoxShadow
	TextStyle            TextStyle
}

// ColorPickerStyle defines color picker visual properties.
type ColorPickerStyle struct {
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	SizeBorder       float32
	Radius           float32
	SVSize           float32
	SliderHeight     float32
	IndicatorSize    float32
	TextStyle        TextStyle
}

// SkeletonStyle defines skeleton loader visual properties.
type SkeletonStyle struct {
	Color          Color
	ColorHighlight Color
	Radius         float32
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
		Padding:          PaddingThree,
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

	DefaultListBoxStyle = ListBoxStyle{
		Color:           colorInteriorDark,
		ColorHover:      colorHoverDark,
		ColorBorder:     colorBorderDark,
		ColorSelect:     colorSelectDark,
		Padding:         PaddingButton,
		SizeBorder:      SizeBorderDef,
		Radius:          RadiusMedium,
		TextStyleNormal: DefaultTextStyle,
		SubheadingStyle: DefaultTextStyle,
	}

	DefaultTreeStyle = TreeStyle{
		Color:       ColorTransparent,
		ColorHover:  colorHoverDark,
		ColorFocus:  colorFocusDark,
		ColorBorder: ColorTransparent,
		Padding:     PaddingNone,
		SizeBorder:  SizeBorderDef,
		Radius:      RadiusMedium,
		TextStyle:   DefaultTextStyle,
		TextStyleIcon: TextStyle{
			Color:  colorTextDark,
			Size:   SizeTextSmall,
			Family: IconFontName,
		},
		Indent:  25,
		Spacing: 0,
	}

	DefaultDialogStyle = DialogStyle{
		Color:            colorPanelDark,
		ColorBorder:      colorBorderDark,
		ColorBorderFocus: colorSelectDark,
		Padding:          PaddingLarge,
		SizeBorder:       SizeBorderDef,
		Radius:           RadiusMedium,
		RadiusBorder:     RadiusMedium,
		AlignButtons:     HAlignCenter,
		TitleTextStyle: TextStyle{
			Color: colorTextDark,
			Size:  SizeTextLarge,
		},
		TextStyle: DefaultTextStyle,
	}

	DefaultToastStyle = ToastStyle{
		MaxVisible:   5,
		Anchor:       ToastBottomRight,
		Width:        260,
		Margin:       16,
		Spacing:      8,
		AccentWidth:  4,
		Padding:      PaddingMedium,
		Radius:       RadiusMedium,
		SizeBorder:   SizeBorderDef,
		Color:        colorPanelDark,
		ColorBorder:  colorBorderDark,
		ColorInfo:    colorSelectDark,
		ColorSuccess: RGBA(46, 160, 67, 255),
		ColorWarning: RGBA(210, 153, 34, 255),
		ColorError:   RGBA(218, 54, 51, 255),
		TextStyle:    DefaultTextStyle,
		TitleStyle: TextStyle{
			Color:    colorTextDark,
			Size:     SizeTextMedium,
			Typeface: glyph.TypefaceBold,
		},
	}

	DefaultTooltipStyle = TooltipStyle{
		Delay:       500 * time.Millisecond,
		Color:       colorInteriorDark,
		ColorBorder: colorBorderDark,
		Padding:     PaddingSmall,
		SizeBorder:  SizeBorderDef,
		Radius:      RadiusSmall,
		TextStyle:   DefaultTextStyle,
	}

	DefaultBadgeStyle = BadgeStyle{
		Color:        colorSelectDark,
		ColorInfo:    colorSelectDark,
		ColorSuccess: RGBA(46, 160, 67, 255),
		ColorWarning: RGBA(210, 153, 34, 255),
		ColorError:   RGBA(218, 54, 51, 255),
		Padding:      NewPadding(2, 6, 2, 6),
		DotSize:      8,
	}

	DefaultExpandPanelStyle = ExpandPanelStyle{
		Color:        colorPanelDark,
		ColorHover:   colorHoverDark,
		ColorClick:   colorActiveDark,
		ColorBorder:  colorBorderDark,
		Padding:      PaddingMedium,
		SizeBorder:   SizeBorderDef,
		Radius:       RadiusMedium,
		RadiusBorder: RadiusMedium,
	}

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
