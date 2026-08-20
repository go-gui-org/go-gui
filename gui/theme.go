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

	// Named text roles. A widget that wants quiet text names the
	// reason it is quiet and takes the theme's answer, rather than
	// spelling an alpha (issue #335). Built by themeTextRoles; see
	// gui/theme_text_roles.go for what each one means and why the
	// values differ per theme.
	//
	// Plain Theme fields, deliberately not default*Style mirrors:
	// applyTheme mirrors widget style structs, and these follow the
	// N1..N6 precedent instead. Read them as guiTheme.TextStyleX
	// during generation, w.Theme().TextStyleX outside it.
	//
	// The four are the vocabulary an app styles its own widgets with.
	// Unexporting any would leave an app unable to match the toolkit's
	// own de-emphasis, which is the divergence #335 exists to end.
	//
	// exportaudit:keep — public styling vocabulary (see above).
	TextStyleSecondary, TextStyleLabel, TextStyleDisabled, TextStylePlaceholder TextStyle

	// Text size shortcuts (N = normal, B = bold,
	// I = italic, M = mono, BI = bold+italic).
	//
	// The rungs are the toolkit's shorthand for the two dimensions a
	// caller actually chooses — face and size — so a widget names one
	// handle instead of filling in a whole TextStyle. Read them the way
	// HTML's h1..h6 are read: the number is the step, not a measurement.
	//
	// The set is a closed 6x6 grid, and every cell is exported because a
	// half-exported grid is a broken vocabulary: bold-small reachable
	// and italic-small not is an accident of which cell a widget in this
	// repo happened to need first. Completeness is the design here, so
	// "no caller yet" is not the test for whether a rung belongs.
	//
	// The M ladder sits +1 above the roman ladder at every rung (M4 is
	// 15 where N4 is 14); see ThemeMaker.
	//
	// Declared one face per line so a single keep marker covers the
	// face: exportaudit reads the marker off the field declaration, and
	// every name on that line shares it.
	N1, N2, N3, N4, N5, N6 TextStyle
	B1, B2, B3, B4, B5, B6 TextStyle
	// exportaudit:keep — closed grid, exported for completeness (see above).
	I1, I2, I3, I4, I5, I6 TextStyle
	// exportaudit:keep — closed grid, exported for completeness (see above).
	BI1, BI2, BI3, BI4, BI5, BI6 TextStyle
	// exportaudit:keep — closed grid, exported for completeness (see above).
	M1, M2, M3, M4, M5, M6 TextStyle
	// exportaudit:keep — closed grid, exported for completeness (see above).
	Icon1, Icon2, Icon3, Icon4, Icon5, Icon6 TextStyle

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

	// PaddingField is the inset a text-bearing form control puts
	// around its text. Separate from the Small/Medium/Large ladder
	// because it answers a different question: those size the gap
	// between things, this one sizes a control. Sharing it is what
	// makes an Input and a Select in one row the same height.
	//
	// exportaudit:keep — themable form density.
	PaddingField Padding

	// SizeFieldMinWidth is the MinWidth floor a text-bearing form
	// control takes when its Cfg states none. A Fit-sized empty field
	// is a stub; without a floor a field is the width of whatever
	// happens to be typed into it.
	//
	// Zero means no floor, not "derive the default": baseCfg seeds
	// it, so every preset carries a value, and a hand-built ThemeCfg
	// that leaves it zero is asking for Fit-to-content.
	//
	// exportaudit:keep — themable form density.
	SizeFieldMinWidth float32

	SizeBorder float32

	RadiusSmall  float32
	RadiusMedium float32
	RadiusLarge  float32

	// SpacingTight is the inside-a-composite tier of the spacing
	// ladder (2px): calendar cells, the tab strip, submenu items.
	//
	// exportaudit:keep — themable tight spacing, consumed inside gui/ only.
	SpacingTight  float32
	SpacingSmall  float32
	SpacingMedium float32
	SpacingLarge  float32

	SizeTextTiny float32
	// SizeTextXSmall .. SizeTextXLarge are the text size ladder. Zero
	// takes the built-in defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextXSmall float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextSmall float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextMedium float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextLarge float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextXLarge float32

	// ScrollMultiplier scales wheel/trackpad scroll distance.
	// exportaudit:keep — caller-facing config (issue #372)
	ScrollMultiplier float32
	// exportaudit:keep — caller-facing config (issue #372)
	ScrollDeltaLine float32
	// exportaudit:keep — caller-facing config (issue #372)
	ScrollDeltaPage float32
	inspectorStyle  InspectorStyle

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

	// ColorTextOnSelect is the resolved text color drawn over
	// ColorSelect fills (selected rows, highlighted list items).
	// ThemeMaker resolves an unset Cfg token to the body text color,
	// so a theme that never states it keeps its current appearance.
	ColorTextOnSelect Color

	// focusRing carries ThemeCfg.focusRing through to the widgets.
	// Read it during generation off the bare guiTheme, like any other
	// factory-time theme read.
	focusRing *BoxShadow

	TitlebarDark bool
}

// ThemeCfg is the configuration struct for ThemeMaker.
type ThemeCfg struct {
	TextStyleDef TextStyle

	Name string

	// MonoFontFamily is the font family for code/mono text.
	// exportaudit:keep — caller-facing config (issue #372)
	MonoFontFamily string // font family for code/mono text

	// IconFontFamily is the font family for the themed icon styles
	// (Icon1..Icon6, TreeStyle.TextStyleIcon). Defaults to
	// IconFontName; an app that ships its own icon font sets this to
	// that font's family name. Empty falls back to IconFontName.
	//
	// Setting this only retargets the styles — the font itself must
	// still be registered with RegisterAppFont or RegisterAppFontBytes
	// before the backend starts, or icons render as tofu.
	// exportaudit:keep — caller-facing config (issue #372)
	IconFontFamily string

	Padding Padding

	PaddingSmall  Padding
	PaddingMedium Padding
	PaddingLarge  Padding

	// PaddingField seeds Theme.PaddingField; see there. Unset falls
	// back to paddingField.
	//
	// exportaudit:keep — themable form density.
	PaddingField Padding

	// SizeFieldMinWidth seeds Theme.SizeFieldMinWidth; see there.
	//
	// exportaudit:keep — themable form density.
	SizeFieldMinWidth float32

	SizeBorder float32
	Radius     float32

	RadiusSmall  float32
	RadiusMedium float32
	RadiusLarge  float32

	// SpacingTight seeds Theme.SpacingTight; see there.
	//
	// exportaudit:keep — themable tight spacing.
	SpacingTight  float32
	SpacingSmall  float32
	SpacingMedium float32
	SpacingLarge  float32

	SizeTextTiny float32
	// SizeTextXSmall .. SizeTextXLarge are the text size ladder. Zero
	// takes the built-in defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextXSmall float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextSmall float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextMedium float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextLarge float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTextXLarge float32

	// ScrollMultiplier scales wheel/trackpad scroll distance.
	// exportaudit:keep — caller-facing config (issue #372)
	ScrollMultiplier float32
	// exportaudit:keep — caller-facing config (issue #372)
	ScrollDeltaLine float32
	// exportaudit:keep — caller-facing config (issue #372)
	ScrollDeltaPage float32

	// exportaudit:keep — caller-facing config (issue #372)
	SizeSwitchWidth float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeSwitchHeight float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeRadio float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeScrollbar float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeScrollbarMin float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeProgressBar float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeSlider float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeSliderThumb float32
	// exportaudit:keep — caller-facing config (issue #372)
	SizeSeparator    float32
	ColorBackground  Color
	ColorPanel       Color
	ColorInterior    Color
	ColorHover       Color
	ColorFocus       Color
	ColorActive      Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSeparator   Color

	// De-emphasized text colors, one per named role. Unset derives
	// from TextStyleDef.Color and the theme's polarity — see
	// textRolesFor — so a theme only states these to override.
	//
	// The four are the override seam for a theme whose de-emphasis
	// cannot be derived — a tinted or deliberately low-contrast
	// palette.

	// exportaudit:keep — theme override seam (see above).
	ColorTextSecondary Color
	// exportaudit:keep — theme override seam (see above).
	ColorTextLabel Color
	// exportaudit:keep — theme override seam (see above).
	ColorTextDisabled Color
	// exportaudit:keep — theme override seam (see above).
	ColorTextPlaceholder Color

	ColorSelect  Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color

	// ColorTextOnSelect is the text color drawn over ColorSelect
	// fills — selected rows, highlighted list items. The fill and
	// the text on it are decided together because contrast is a
	// property of the pair, not of either color alone (issue #373).
	// Unset resolves to TextStyleDef.Color, so existing themes stay
	// byte-identical unless they opt in.
	//
	// exportaudit:keep — theme override seam.
	ColorTextOnSelect Color

	// Elevation. Two tiers, because that is how the platforms this
	// exists to imitate actually think about it: a menu floats a
	// little, a modal floats a lot. A per-widget field would be seven
	// knobs nobody sets differently.
	//
	// Nil means "this theme does not describe elevation", which keeps
	// every theme predating these fields byte-identical: nil reaches
	// ContainerCfg.Shadow unchanged, makeContainerEffects still
	// short-circuits, and no shapeEffects is ever allocated.
	//
	// Theme-owned and built once by the theme's cfg function, so
	// assigning one into a ContainerCfg costs a pointer copy and no
	// per-frame allocation. Never write through the pointer — one
	// value is shared by every shape in the window.
	// ShadowPopover/ShadowDialog are the two elevation tiers.
	// exportaudit:keep — caller-facing config (issue #372)
	ShadowPopover *BoxShadow // menus, dropdowns, tooltips, toasts
	// exportaudit:keep — caller-facing config (issue #372)
	ShadowDialog *BoxShadow // modals, command palette

	// focusRing is the focus indication drawn *outside* a control's
	// bounds, as a zero-offset tinted shadow. macOS's ring is a soft
	// accent glow, which the inset ColorBorderFocus border cannot
	// express at any width. Nil leaves the border ring in charge, so
	// themes that do not set it are unaffected.
	// FocusRing is the outside-the-bounds focus glow. Nil leaves the
	// border ring in charge.
	// exportaudit:keep — caller-facing config (issue #372)
	FocusRing *BoxShadow

	TitlebarDark bool
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
		cfg.PaddingField = PaddingNone
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
