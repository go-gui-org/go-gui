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
	BreadcrumbStyle  BreadcrumbStyle
	TabControlStyle  TabControlStyle
	DataGridStyle    DataGridStyle
	SelectStyle      SelectStyle
	MenubarStyle     MenubarStyle
	ToastStyle       ToastStyle
	InputStyle       InputStyle
	DialogStyle      DialogStyle
	ComboboxStyle    ComboboxStyle
	ToggleStyle      ToggleStyle
	ListBoxStyle     ListBoxStyle
	SwitchStyle      SwitchStyle
	DatePickerStyle  DatePickerStyle
	ProgressBarStyle ProgressBarStyle
	RadioStyle       RadioStyle
	TooltipStyle     TooltipStyle
	ColorPickerStyle ColorPickerStyle
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
	I1    TextStyle
	I2    TextStyle
	I3    TextStyle
	I4    TextStyle
	I5    TextStyle
	I6    TextStyle
	BI1   TextStyle
	BI2   TextStyle
	BI3   TextStyle
	BI4   TextStyle
	BI5   TextStyle
	BI6   TextStyle
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
	Icon5 TextStyle
	Icon6 TextStyle

	// Per-widget styles.
	ButtonStyle         ButtonStyle
	ContainerStyle      ContainerStyle
	RectangleStyle      RectangleStyle
	TreeStyle           TreeStyle
	CommandPaletteStyle CommandPaletteStyle
	BadgeStyle          BadgeStyle
	Name                string
	TableStyle          TableStyle
	Cfg                 ThemeCfg
	SliderStyle         SliderStyle
	SplitterStyle       SplitterStyle
	ExpandPanelStyle    ExpandPanelStyle
	ScrollbarStyle      ScrollbarStyle
	SkeletonStyle       SkeletonStyle

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
	SizeTextXSmall float32
	SizeTextSmall  float32
	SizeTextMedium float32
	SizeTextLarge  float32
	SizeTextXLarge float32

	ScrollMultiplier float32
	ScrollDeltaLine  float32
	ScrollDeltaPage  float32
	InspectorStyle   InspectorStyle

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

	MonoFontFamily string // font family for code/mono text

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
	SizeTextXSmall float32
	SizeTextSmall  float32
	SizeTextMedium float32
	SizeTextLarge  float32
	SizeTextXLarge float32

	ScrollMultiplier float32
	ScrollDeltaLine  float32
	ScrollDeltaPage  float32

	SizeSwitchWidth  float32
	SizeSwitchHeight float32
	SizeRadio        float32
	SizeScrollbar    float32
	SizeScrollbarMin float32
	SizeProgressBar  float32
	SizeSlider       float32
	SizeSliderThumb  float32
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
	Fill             bool
	FillBorder       bool
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
		cfg.Radius = RadiusNone
		cfg.RadiusSmall = RadiusNone
		cfg.RadiusMedium = RadiusNone
		cfg.RadiusLarge = RadiusNone
	}
	return ThemeMaker(cfg)
}

// WithBorders returns a new Theme with borders turned on (true) or
// off (false).
func (t Theme) WithBorders(borders bool) Theme {
	cfg := t.Cfg
	if borders {
		cfg.SizeBorder = SizeBorderDef
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
	DefaultButtonStyle = t.ButtonStyle
	DefaultContainerStyle = t.ContainerStyle
	DefaultRectangleStyle = t.RectangleStyle
	DefaultInputStyle = t.InputStyle
	DefaultScrollbarStyle = t.ScrollbarStyle
	DefaultRadioStyle = t.RadioStyle
	DefaultSwitchStyle = t.SwitchStyle
	DefaultToggleStyle = t.ToggleStyle
	DefaultSelectStyle = t.SelectStyle
	DefaultListBoxStyle = t.ListBoxStyle
	DefaultTreeStyle = t.TreeStyle
	DefaultDialogStyle = t.DialogStyle
	DefaultToastStyle = t.ToastStyle
	DefaultTooltipStyle = t.TooltipStyle
	DefaultBadgeStyle = t.BadgeStyle
	DefaultExpandPanelStyle = t.ExpandPanelStyle
	DefaultProgressBarStyle = t.ProgressBarStyle
	DefaultSliderStyle = t.SliderStyle
	DefaultTabControlStyle = t.TabControlStyle
	DefaultBreadcrumbStyle = t.BreadcrumbStyle
	DefaultSplitterStyle = t.SplitterStyle
	DefaultTableStyle = t.TableStyle
	DefaultComboboxStyle = t.ComboboxStyle
	DefaultCommandPaletteStyle = t.CommandPaletteStyle
	DefaultMenubarStyle = t.MenubarStyle
	DefaultDatePickerStyle = t.DatePickerStyle
	DefaultColorPickerStyle = t.ColorPickerStyle
	DefaultDataGridStyle = t.DataGridStyle
	DefaultSkeletonStyle = t.SkeletonStyle
	DefaultInspectorStyle = t.InspectorStyle
}
