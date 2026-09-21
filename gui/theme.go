package gui

import (
	"reflect"
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
	// guiTheme and the style mirrors. Written by applyTheme, which
	// SetTheme also calls eagerly outside the frame pass, so the
	// fast-path word itself is atomic; the mirrors stay
	// frame-thread (or init/test) state guarded by guiThemeMu on
	// the write side.
	installedThemeID atomic.Uint64

	// themeIDCounter hands out Theme.id values. Starts at 1 so a
	// zero-valued Theme never collides with a real one.
	themeIDCounter atomic.Uint64
)

// nextThemeID returns a fresh theme identity.
func nextThemeID() uint64 {
	return themeIDCounter.Add(1)
}

// cloneBoxShadow returns a copy of s, or nil when s is nil. ThemeMaker
// isolates every built theme this way: the package-level shadow and
// focus-ring values are shared presets, and a caller that mutates
// its cfg after the build — or one theme's style through a shared
// pointer — must not move every other theme.
func cloneBoxShadow(s *BoxShadow) *BoxShadow {
	if s == nil {
		return nil
	}
	dup := *s
	return &dup
}

// isolateThemeShadows rebinds cfg's elevation pointers to isolated
// copies. The styles ThemeMaker builds below and the stored theme.Cfg
// then share one copy per theme that nothing else can write through.
func isolateThemeShadows(cfg *ThemeCfg) {
	cfg.ShadowPopover = cloneBoxShadow(cfg.ShadowPopover)
	cfg.ShadowDialog = cloneBoxShadow(cfg.ShadowDialog)
	cfg.FocusRing = cloneBoxShadow(cfg.FocusRing)
}

// Theme describes a complete GUI theme. Only styles for existing
// Go views are populated (Button, Container, Rectangle, Text,
// Input, Scrollbar, Radio, Switch, Toggle, Select, ListBox, Tree).
type Theme struct {
	breadcrumbStyle  BreadcrumbStyle
	tabControlStyle  TabControlStyle
	dataGridStyle    DataGridStyle
	selectStyle      SelectStyle
	menubarStyle     MenubarStyle
	toastStyle       ToastStyle
	inputStyle       InputStyle
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

	// Text roles name text by purpose instead of by face and step
	// (issue #734): a caller rendering a title names TitleSmall rather
	// than spelling a bold medium style, so two callers with the same
	// purpose cannot drift apart. They replace the numbered rung grid
	// (N1..Icon6), removed in the same change: every role derives in
	// fillTextRungs from the size ladder, and the Bold/Italic/Roman
	// modifiers plus Theme.Mono cover the donor and emphasis spellings
	// the grid used to serve.
	//
	// Mapping from the removed grid: Display=B1 Title=B2
	// TitleSmall=B3 BodyLarge=N2 Body=N3 BodySmall=N4 Caption=N5
	// CaptionSmall=N6 Code=M3 CodeSmall=M5 CodeTiny=M6
	// IconXLarge=Icon1 IconLarge=Icon2 IconMedium=Icon3
	// IconSmall=Icon4 IconXSmall=Icon5 IconTiny=Icon6.
	//
	// Declared one purpose per line so a single keep marker covers the
	// role.
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleDisplay TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleTitle TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleTitleSmall TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleBodyLarge TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleBody TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleBodySmall TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleCaption TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleCaptionSmall TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleCode TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleCodeSmall TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleCodeTiny TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleIconXLarge TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleIconLarge TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleIconMedium TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleIconSmall TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleIconXSmall TextStyle
	// exportaudit:keep — semantic text roles (issue #734).
	TextStyleIconTiny TextStyle

	// Per-widget styles. Private and derived only (issue #735): a
	// style is customized through ThemeCfg tokens, never by field
	// assignment. ScrollbarStyle and TextStyleDef stay public —
	// external layout code reads the scrollbar gutter size, and
	// TextStyleDef is a ThemeMaker input, not a widget style.
	buttonStyle         buttonStyle
	buttonStylePrimary  buttonStyle
	buttonStyleGhost    buttonStyle
	buttonStyleDanger   buttonStyle
	containerStyle      containerStyle
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

	// Sounds maps interaction roles onto cues. The zero set is silent
	// and is the intended default, so ThemeDark and ThemeLight make no
	// sound. An app opts the whole widget set in with
	// theme.Sounds = gui.SoundsDefault() and an installed SoundPlayer
	// (issue #446).
	// exportaudit:keep — caller-facing config (issue #446)
	Sounds SoundSet

	// id identifies this exact theme value. Stamped by ThemeMaker and
	// re-stamped by every with*Style helper, so a derived theme never
	// reuses its parent's id. Zero means "built outside ThemeMaker" and
	// forces a re-install rather than a wrong fast-path hit.
	id uint64

	// restoreCfg remembers the padded configuration across a
	// WithPadding(false) strip, so a later WithPadding(true)
	// restores the original instead of rebuilding from the zeroed
	// Cfg. Nil on themes that were never stripped.
	restoreCfg *ThemeCfg

	// ext holds sibling theme extensions, keyed by exact value
	// type (issue #733). A sibling stores its own style value
	// with WithExt and reads it with Ext. Copies share the
	// backing map, so stored values must be immutable;
	// WithExt clones on write. Nil on themes with no extensions.
	ext map[reflect.Type]any

	ColorBackground Color
	ColorPanel      Color
	ColorInterior   Color
	ColorHover      Color
	ColorFocus      Color
	ColorActive     Color
	ColorBorder     Color
	ColorSelect     Color

	// Accent ramp, resolved (visual-refresh §4.3). ThemeMaker fills
	// every unset slot from ColorAccent (hover = L+0.12, pressed =
	// L-0.12 in sRGB HSL; subtle = accent at a polarity-fixed alpha;
	// textOnAccent = white/black by luminance), and an unstated
	// accent resolves to ColorSelect. Read these during generation
	// off the bare guiTheme, like any other theme read.
	ColorAccent        Color
	ColorAccentHover   Color
	ColorAccentPressed Color
	ColorAccentSubtle  Color
	ColorTextOnAccent  Color

	// Subtle companions for the semantic colors (visual-refresh
	// §4.4), resolved the same way as ColorAccentSubtle. Consumers
	// land with phase 7's validation work.
	ColorSuccessSubtle Color
	ColorWarningSubtle Color
	ColorErrorSubtle   Color

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

	// Sounds seeds Theme.Sounds; see there. Zero is silent, which is
	// the default for every built-in theme.
	// exportaudit:keep — caller-facing config (issue #446)
	Sounds SoundSet

	SizeBorder float32
	Radius     float32

	RadiusSmall  float32
	RadiusMedium float32
	// RadiusLarge seeds Theme.RadiusLarge, which no widget reads yet. It
	// is the reserved top tier of the radius ladder: themes set it so the
	// ladder stays complete, and a widget that needs the largest corner
	// takes it from here instead of spelling a number.
	//
	// ergonomics-audit:deadcfg-keep — reserved radius tier, set by themes
	RadiusLarge float32

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
	// SizeScrollbarGap seeds ScrollbarStyle.GapEdge — the inset
	// between the scrollbar and the edge it tracks. Unset takes the
	// built-in scrollbarGapEdge; zero is a legitimate choice (a bar
	// flush against the edge), which is why this is an Opt.
	//
	// exportaudit:keep — themable scrollbar geometry.
	SizeScrollbarGap Opt[float32]
	// SizeScrollbarGapEnd seeds ScrollbarStyle.GapEnd — the inset at
	// the two ends of the track. Unset takes the built-in
	// scrollbarGapEnd; zero is a full-length track.
	//
	// exportaudit:keep — themable scrollbar geometry.
	SizeScrollbarGapEnd Opt[float32]
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

	// Accent ramp. ColorAccent is the single accent decision; every
	// other slot derives from it in ThemeMaker (visual-refresh §4.3):
	// hover = L+0.12, pressed = L-0.12 in sRGB HSL, ColorAccentSubtle
	// is the accent at a polarity-fixed alpha, ColorTextOnAccent is
	// white or black by luminance. Unset ColorAccent resolves to
	// ColorSelect, and unset ColorSelect to ColorAccent — a theme that
	// states one color gets a working ramp, and existing themes keep
	// their appearance (their select becomes their accent).
	//
	// exportaudit:keep — theme override seam (accent ramp).
	ColorAccent Color
	// exportaudit:keep — theme override seam (accent ramp).
	ColorAccentHover Color
	// exportaudit:keep — theme override seam (accent ramp).
	ColorAccentPressed Color
	// exportaudit:keep — theme override seam (accent ramp).
	ColorAccentSubtle Color
	// exportaudit:keep — theme override seam (accent ramp).
	ColorTextOnAccent Color

	// Subtle companions for the semantic colors, on the same
	// derivation rule as ColorAccentSubtle: the fill behind a
	// validation message is the same decision as the message color
	// (visual-refresh §4.4). No consumers yet; the widgets that paint
	// them land with phase 7's validation work.
	//
	// exportaudit:keep — theme override seam (semantic colors).
	ColorSuccessSubtle Color
	// exportaudit:keep — theme override seam (semantic colors).
	ColorWarningSubtle Color
	// exportaudit:keep — theme override seam (semantic colors).
	ColorErrorSubtle Color

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
	//
	// ergonomics-audit:deadcfg-keep — documented public API (showcase docs)
	FillBorder bool
}

// WithPadding returns a new Theme with padding, radius, and border
// turned on (true) or off (false). When off, all padding, radius, and
// border sizing are set to zero/none. When on, the theme is rebuilt
// from its stored configuration — or from the pre-strip
// configuration when called on a stripped theme, so
// WithPadding(false).WithPadding(true) round-trips.
func (t Theme) WithPadding(padding bool) Theme {
	if padding {
		source := t.Cfg
		if t.restoreCfg != nil {
			source = *t.restoreCfg
		}
		out := ThemeMaker(source)
		out.ext = t.ext
		return out
	}
	cfg := t.Cfg
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
	out := ThemeMaker(cfg)
	out.ext = t.ext
	if t.restoreCfg != nil {
		out.restoreCfg = t.restoreCfg
	} else {
		orig := t.Cfg
		out.restoreCfg = &orig
	}
	return out
}

// WithBorders returns a new Theme with borders turned on (true) or
// off (false). A stripped theme keeps its restore point, updated
// with the new border choice, so a later WithPadding(true) restores
// with the tune kept.
func (t Theme) WithBorders(borders bool) Theme {
	cfg := t.Cfg
	if borders {
		cfg.SizeBorder = sizeBorderDef
	} else {
		cfg.SizeBorder = 0
	}
	out := ThemeMaker(cfg)
	out.ext = t.ext
	if t.restoreCfg != nil {
		dup := *t.restoreCfg
		dup.SizeBorder = cfg.SizeBorder
		out.restoreCfg = &dup
	}
	return out
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
	//
	// Frame-thread only: main before Run, or an event handler. See
	// applyTheme. From another goroutine, queue it instead —
	// w.QueueCommand(func(*Window) { gui.SetTheme(t) }).
	applyTheme(&published)
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
		// a Window in an init of its own). A copy, for the same reason
		// init publishes one: the caller must not end up holding a
		// pointer into the exported ThemeDark var.
		fallback := ThemeDark
		return &fallback
	}
	return t
}

// applyTheme installs t as the active theme: guiTheme plus every
// default*Style mirror that widget factories read.
//
// Called at frame start ((*Window).installTheme), around a scoped
// subtree (Themed's push/pop), at package init, and eagerly from
// SetTheme so callers outside a frame pass see the change at once.
//
// Frame-thread only. The write side takes guiThemeMu, but the ~166
// factory-time reads of guiTheme and the default*Style mirrors take
// no lock at all — they rely on the single-frame-thread invariant at
// the top of this file. So a SetTheme from a timer or a network
// goroutine races every widget the current frame is building. Both
// exported entry points say so; nothing enforces it.
//
// Takes a pointer: Theme is ~12 KB and this is on the frame path.
// The value is read, never retained.
func applyTheme(t *Theme) {
	guiThemeMu.Lock()
	defer guiThemeMu.Unlock()
	guiTheme = *t
	DefaultTextStyle = t.TextStyleDef
	defaultButtonStyle = t.buttonStyle
	defaultContainerStyle = t.containerStyle
	defaultInputStyle = t.inputStyle
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
	defaultMenubarStyle = t.menubarStyle
	defaultDatePickerStyle = t.datePickerStyle
	defaultColorPickerStyle = t.colorPickerStyle
	DefaultDataGridStyle = t.dataGridStyle
	defaultSkeletonStyle = t.skeletonStyle
	defaultSeparatorStyle = t.separatorStyle
	defaultInspectorStyle = t.inspectorStyle
	// Publish the id last, under the same lock as the mirrors: the
	// Store synchronizes with the Load in needsInstall, so a frame
	// that sees the new id also sees the new mirrors. Storing
	// before the lock let two concurrent applyTheme calls publish
	// id and mirrors in opposite orders, mixing one theme's id
	// with the other's mirrors for a frame.
	installedThemeID.Store(t.id)
}
