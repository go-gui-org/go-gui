package gui

import "sync"

// guiTheme is the package-level active theme.
var (
	guiTheme   Theme
	guiThemeMu sync.RWMutex
)

// Theme describes a complete GUI theme. Only styles for existing
// Go views are populated (Button, Container, Rectangle, Text,
// Input, Scrollbar, Radio, Switch, Toggle, Select, ListBox, Tree).
type Theme struct {
	breadcrumbStyle  BreadcrumbStyle
	tabControlStyle  TabControlStyle
	dataGridStyle    DataGridStyle
	selectStyle      SelectStyle
	MenubarStyle     MenubarStyle
	toastStyle       ToastStyle
	InputStyle       InputStyle
	dialogStyle      DialogStyle
	comboboxStyle    ComboboxStyle
	toggleStyle      ToggleStyle
	listBoxStyle     ListBoxStyle
	switchStyle      SwitchStyle
	datePickerStyle  DatePickerStyle
	progressBarStyle ProgressBarStyle
	radioStyle       RadioStyle
	tooltipStyle     TooltipStyle
	colorPickerStyle ColorPickerStyle
	TextStyleDef     TextStyle

	// Text size shortcuts (N = normal, B = bold,
	// I = italic, M = mono, BI = bold+italic).
	N1    TextStyle
	N2    TextStyle
	N3    TextStyle
	N4    TextStyle
	N5    TextStyle
	N6    TextStyle
	B1    TextStyle
	B2    TextStyle
	B3    TextStyle
	B4    TextStyle
	B5    TextStyle
	B6    TextStyle
	i1    TextStyle
	i2    TextStyle
	I3    TextStyle
	i4    TextStyle
	i5    TextStyle
	i6    TextStyle
	bI1   TextStyle
	bI2   TextStyle
	BI3   TextStyle
	bI4   TextStyle
	bI5   TextStyle
	bI6   TextStyle
	M1    TextStyle
	M2    TextStyle
	M3    TextStyle
	M4    TextStyle
	M5    TextStyle
	M6    TextStyle
	Icon1 TextStyle
	Icon2 TextStyle
	Icon3 TextStyle
	Icon4 TextStyle
	icon5 TextStyle
	icon6 TextStyle

	// Per-widget styles.
	ButtonStyle         buttonStyle
	ContainerStyle      containerStyle
	rectangleStyle      RectangleStyle
	treeStyle           TreeStyle
	commandPaletteStyle CommandPaletteStyle
	badgeStyle          BadgeStyle
	Name                string
	tableStyle          TableStyle
	Cfg                 ThemeCfg
	sliderStyle         SliderStyle
	splitterStyle       SplitterStyle
	expandPanelStyle    ExpandPanelStyle
	ScrollbarStyle      ScrollbarStyle
	skeletonStyle       SkeletonStyle

	// Layout constants.
	PaddingSmall  Padding
	PaddingMedium Padding
	PaddingLarge  Padding
	SizeBorder    float32

	RadiusSmall  float32
	RadiusMedium float32
	RadiusLarge  float32

	SpacingSmall  float32
	SpacingMedium float32
	SpacingLarge  float32

	SizeTextTiny   float32
	sizeTextXSmall float32
	sizeTextSmall  float32
	sizeTextMedium float32
	sizeTextLarge  float32
	sizeTextXLarge float32

	scrollMultiplier float32
	scrollDeltaLine  float32
	scrollDeltaPage  float32
	inspectorStyle   InspectorStyle

	ColorBackground Color
	ColorPanel      Color
	ColorInterior   Color
	ColorHover      Color
	ColorFocus      Color
	ColorActive     Color
	ColorBorder     Color
	ColorSelect     Color
	TitlebarDark    bool
}

// ThemeCfg is the configuration struct for ThemeMaker.
type ThemeCfg struct {
	TextStyleDef TextStyle

	Name string

	monoFontFamily string // font family for code/mono text

	// IconFontFamily is the font family for the themed icon styles
	// (Icon1..Icon6, TreeStyle.TextStyleIcon). Defaults to
	// IconFontName; an app that ships its own icon font sets this to
	// that font's family name. Empty falls back to IconFontName.
	//
	// Setting this only retargets the styles — the font itself must
	// still be registered with RegisterAppFont or RegisterAppFontBytes
	// before the backend starts, or icons render as tofu.
	iconFontFamily string

	Padding Padding

	PaddingSmall  Padding
	PaddingMedium Padding
	PaddingLarge  Padding

	SizeBorder float32
	Radius     float32

	RadiusSmall  float32
	RadiusMedium float32
	RadiusLarge  float32

	SpacingSmall  float32
	SpacingMedium float32
	SpacingLarge  float32

	SizeTextTiny   float32
	sizeTextXSmall float32
	sizeTextSmall  float32
	sizeTextMedium float32
	sizeTextLarge  float32
	sizeTextXLarge float32

	scrollMultiplier float32
	scrollDeltaLine  float32
	scrollDeltaPage  float32

	sizeSwitchWidth  float32
	sizeSwitchHeight float32
	sizeRadio        float32
	sizeScrollbar    float32
	sizeScrollbarMin float32
	sizeProgressBar  float32
	sizeSlider       float32
	sizeSliderThumb  float32
	ColorBackground  Color
	ColorPanel       Color
	ColorInterior    Color
	ColorHover       Color
	ColorFocus       Color
	ColorActive      Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	ColorSuccess     Color
	ColorWarning     Color
	ColorError       Color
	TitlebarDark     bool
	// exportaudit:keep — documented public API (showcase docs)
	FillBorder bool
}

// WithPadding returns a new Theme with padding, radius, and border
// turned on (true) or off (false). When off, all padding, radius, and
// border sizing are set to zero/none. When on, the theme is rebuilt
// from its stored configuration.
func (t Theme) WithPadding(padding bool) Theme {
	cfg := t.Cfg
	if !padding {
		cfg.Padding = PaddingNone
		cfg.PaddingSmall = PaddingNone
		cfg.PaddingMedium = PaddingNone
		cfg.PaddingLarge = PaddingNone
		cfg.SizeBorder = 0
		cfg.Radius = radiusNone
		cfg.RadiusSmall = radiusNone
		cfg.RadiusMedium = radiusNone
		cfg.RadiusLarge = radiusNone
	}
	return ThemeMaker(cfg)
}

// WithBorders returns a new Theme with borders turned on (true) or
// off (false).
func (t Theme) WithBorders(borders bool) Theme {
	cfg := t.Cfg
	if borders {
		cfg.SizeBorder = sizeBorderDef
	} else {
		cfg.SizeBorder = 0
	}
	return ThemeMaker(cfg)
}

// CurrentTheme returns the active theme.
func CurrentTheme() Theme {
	guiThemeMu.RLock()
	defer guiThemeMu.RUnlock()
	return guiTheme
}

// SetTheme sets the active theme and updates all Default*Style vars.
func SetTheme(t Theme) {
	guiThemeMu.Lock()
	defer guiThemeMu.Unlock()
	guiTheme = t
	DefaultTextStyle = t.TextStyleDef
	defaultButtonStyle = t.ButtonStyle
	defaultContainerStyle = t.ContainerStyle
	defaultInputStyle = t.InputStyle
	DefaultScrollbarStyle = t.ScrollbarStyle
	defaultRadioStyle = t.radioStyle
	defaultSwitchStyle = t.switchStyle
	defaultToggleStyle = t.toggleStyle
	defaultSelectStyle = t.selectStyle
	defaultListBoxStyle = t.listBoxStyle
	defaultTreeStyle = t.treeStyle
	DefaultDialogStyle = t.dialogStyle
	defaultToastStyle = t.toastStyle
	defaultTooltipStyle = t.tooltipStyle
	defaultBadgeStyle = t.badgeStyle
	defaultExpandPanelStyle = t.expandPanelStyle
	defaultProgressBarStyle = t.progressBarStyle
	defaultSliderStyle = t.sliderStyle
	defaultTabControlStyle = t.tabControlStyle
	defaultBreadcrumbStyle = t.breadcrumbStyle
	defaultSplitterStyle = t.splitterStyle
	defaultTableStyle = t.tableStyle
	defaultComboboxStyle = t.comboboxStyle
	defaultCommandPaletteStyle = t.commandPaletteStyle
	defaultMenubarStyle = t.MenubarStyle
	defaultDatePickerStyle = t.datePickerStyle
	defaultColorPickerStyle = t.colorPickerStyle
	DefaultDataGridStyle = t.dataGridStyle
	defaultSkeletonStyle = t.skeletonStyle
	defaultInspectorStyle = t.inspectorStyle
}
