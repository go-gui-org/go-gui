package gui

import (
	"sync"
	"sync/atomic"
)

// Theme state comes in two layers.
//
// guiTheme (and the ~30 default*Style mirrors that applyTheme writes) is
// the *installed* theme: a frame-scoped cache of the theme belonging to
// the window currently being generated. Widget factories read it at
// construction time, which is inside that window's frame pass, so the
// value they see is that window's theme. Only the frame thread writes it,
// through applyTheme.
//
// defaultTheme is *app* state: the theme a window follows until it sets
// its own with (*Window).SetTheme. Package-level SetTheme writes this one.
var (
	guiTheme   Theme
	guiThemeMu sync.RWMutex

	// Held as a pointer to an immutable value for the same reason
	// Window.theme is: readers take the pointer and skip copying a
	// large struct. SetTheme publishes a new value; nothing writes
	// through the pointer.
	defaultTheme   *Theme
	defaultThemeMu sync.RWMutex

	// installedThemeID is the id of the theme currently written into
	// guiTheme and the style mirrors. Frame-thread only.
	installedThemeID uint64

	// themeIDCounter hands out Theme.id values. Starts at 1 so a
	// zero-valued Theme never collides with a real one.
	themeIDCounter atomic.Uint64
)

// nextThemeID returns a fresh theme identity.
func nextThemeID() uint64 {
	return themeIDCounter.Add(1)
}

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
	separatorStyle      SeparatorStyle

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

	// id identifies this exact theme value. Stamped by ThemeMaker and
	// re-stamped by every with*Style helper, so a derived theme never
	// reuses its parent's id. Zero means "built outside ThemeMaker" and
	// forces a re-install rather than a wrong fast-path hit.
	id uint64

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
	sizeSeparator    float32
	ColorBackground  Color
	ColorPanel       Color
	ColorInterior    Color
	ColorHover       Color
	ColorFocus       Color
	ColorActive      Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSeparator   Color
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

// SetTheme sets the app-default theme: the theme every window follows
// until it pins its own with (*Window).SetTheme. Windows that never pin
// one pick this up on their next frame, so calling SetTheme from main or
// from an event handler rethemes the app as it always has.
func SetTheme(t Theme) {
	published := t
	defaultThemeMu.Lock()
	defaultTheme = &published
	defaultThemeMu.Unlock()
	// Install eagerly as well, so callers outside a frame pass — main
	// before Run, tests building views directly — see the change at
	// once. Each window's frame start re-installs its own theme, so
	// this cannot outlive the next frame of a window that pinned one.
	applyTheme(t)
	appUpdateWindows()
}

// currentDefaultTheme returns the app-default theme.
func currentDefaultTheme() Theme {
	return *currentDefaultThemeRef()
}

// currentDefaultThemeRef is currentDefaultTheme without the copy. The
// value it points at is never written through, so the pointer stays
// valid after the lock is dropped. Callers must not mutate it.
func currentDefaultThemeRef() *Theme {
	defaultThemeMu.RLock()
	t := defaultTheme
	defaultThemeMu.RUnlock()
	if t == nil {
		// Before init's SetTheme-equivalent runs (a test constructing
		// a Window in an init of its own).
		return &ThemeDark
	}
	return t
}

// applyTheme installs t as the active theme: guiTheme plus every
// default*Style mirror that widget factories read.
//
// Frame-thread only. Callers are (*Window).installTheme at frame start,
// Themed's push/pop around a scoped subtree, and package init.
func applyTheme(t Theme) {
	installedThemeID = t.id
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
	defaultSeparatorStyle = t.separatorStyle
	defaultInspectorStyle = t.inspectorStyle
}
