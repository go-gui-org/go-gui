package gui

// Light theme color vars. The ramp mirrors the dark ramp's
// direction: the page is the tint and the controls are the light
// surface, moving down from white as energy increases
// (visual-refresh §4.2).
var (
	colorBackgroundLight = RGB(242, 244, 247) // #F2F4F7
	colorPanelLight      = RGB(255, 255, 255) // #FFFFFF
	colorInteriorLight   = RGB(255, 255, 255) // #FFFFFF
	colorHoverLight      = RGB(237, 240, 244) // #EDF0F4
	colorFocusLight      = RGB(231, 235, 241) // #E7EBF1
	colorActiveLight     = RGB(224, 229, 236) // #E0E5EC
	colorBorderLight     = RGB(216, 221, 228) // #D8DDE4
	colorSeparatorLight  = RGB(230, 234, 239) // #E6EAEF
	// colorAccentLight is the single accent decision; the rest of the
	// ramp derives from it in ThemeMaker (visual-refresh §4.3).
	colorAccentLight = RGB(47, 111, 224) // #2F6FE0
	// Semantic colors, chosen so each hue family reads at a glance on
	// its polarity's ground (visual-refresh §4.4).
	colorErrorLight   = RGB(214, 69, 69)  // #D64545
	colorWarningLight = RGB(176, 126, 31) // #B07E1F
	colorSuccessLight = RGB(46, 158, 91)  // #2E9E5B
	colorTextLight    = RGB(26, 29, 33)   // #1A1D21
)

// Scroll constants.
const (
	scrollMultiplier float32 = 20
	scrollDeltaLine  float32 = 1
	scrollDeltaPage  float32 = 10
)

// Dark elevation tiers (visual-refresh §5.3). Floating surfaces only:
// menus, dropdowns, tooltips, toasts on the popover tier; dialogs and
// the command palette on the dialog tier. Inline panels and cards
// separate by fill value, never by shadow.
//
// Two properties these values depend on, both learned from the first
// pass reading as a design-tool drop shadow rather than as depth:
//
//   - **Concentric, not offset.** A directional shadow puts a heavy
//     band under one edge and nothing above it, which reads as the
//     surface sliding rather than floating — most visible on a dialog,
//     which is centred in the window with nothing to cast onto. Both
//     tiers use OffsetY 0; the tier is expressed by blur and alpha.
//   - **The alphas are CSS-scale, and now render that way.** The GPU
//     shadow shader used to ramp from full opacity AT the caster edge
//     out to BlurRadius, where a Gaussian — what the soft and web
//     backends produce, and what a CSS box-shadow means — is ~50% at
//     that edge. Every GPU shadow was therefore about twice the ink
//     over twice the width, which is what read as a grey cloud. The
//     shader is fixed (fs_shadow, gui/backend/internal/msl); these
//     alphas are the spec table's, unchanged.
var (
	darkShadowPopover = &BoxShadow{
		Color:      RGBA(0, 0, 0, 140),
		BlurRadius: 12,
	}
	darkShadowDialog = &BoxShadow{
		Color:      RGBA(0, 0, 0, 170),
		BlurRadius: 32,
	}

	// Light elevation tiers. The same geometry as dark with a softer
	// ink: dark-on-dark shadows are barely visible at the low alphas a
	// light surface needs, so the light ink is the page tint instead.
	lightShadowPopover = &BoxShadow{
		Color:      RGBA(16, 24, 40, 40),
		BlurRadius: 12,
	}
	lightShadowDialog = &BoxShadow{
		Color:      RGBA(16, 24, 40, 60),
		BlurRadius: 32,
	}

	// Focus rings (visual-refresh § 5.4). Tinted with the theme's own
	// accent so the ring is the same decision as the accent, built per
	// polarity because the accents differ.
	//
	// No spread: the ring is a zero-offset blurred glow, so only the
	// blur's tail escapes the caster (the shadow emits before the fill,
	// which then covers everything inside the control's own rect). That
	// is what keeps it subtle — spread would add a crisp opaque plateau
	// at full ring alpha, which reads as a second border rather than as
	// focus indication. Same construction as the macOS theme's rings.
	//
	// The glow is drawn *outside* the control's layout bounds, but a
	// shape's clip is shapeBounds ∩ parentClip (layout_position.go) and
	// the parent emits that scissor before its children — so a Fill
	// control sitting against its parent's content edge has the glow
	// scissored away on the sides it touches, while the sides with
	// slack keep it. The real fix is a draw-outset the ancestors' clips
	// can honour; until then a spread-free glow degrades gracefully (a
	// clipped side loses a faint tail, not a hard-edged band) and the
	// focus *border* (ColorBorderFocus) remains the indicator that is
	// never clipped.
	darkFocusRing = &BoxShadow{
		Color:      colorAccentDark.WithOpacity(0.25),
		BlurRadius: 2,
	}
	lightFocusRing = &BoxShadow{
		Color:      colorAccentLight.WithOpacity(0.25),
		BlurRadius: 2,
	}
)

// baseCfg returns the shared sizing/spacing/widget-size fields
// common to all preset themes.
func baseCfg() ThemeCfg {
	return ThemeCfg{
		MonoFontFamily:    defaultMonoFontFamily,
		IconFontFamily:    IconFontName,
		Padding:           paddingMedium,
		PaddingSmall:      PaddingSmall,
		PaddingMedium:     paddingMedium,
		PaddingLarge:      PaddingLarge,
		Radius:            radiusMedium,
		RadiusSmall:       radiusSmall,
		RadiusMedium:      radiusMedium,
		RadiusLarge:       radiusLarge,
		SpacingTight:      SpacingTight,
		SpacingSmall:      SpacingSmall,
		SpacingMedium:     SpacingMedium,
		SpacingLarge:      SpacingLarge,
		SizeTextTiny:      sizeTextTiny,
		SizeTextXSmall:    sizeTextXSmall,
		SizeTextSmall:     sizeTextSmall,
		SizeTextMedium:    sizeTextMedium,
		SizeTextLarge:     sizeTextLarge,
		SizeTextXLarge:    sizeTextXLarge,
		ScrollMultiplier:  scrollMultiplier,
		ScrollDeltaLine:   scrollDeltaLine,
		ScrollDeltaPage:   scrollDeltaPage,
		SizeSwitchWidth:   34,
		SizeSwitchHeight:  20,
		SizeRadio:         15,
		SizeScrollbar:     7,
		SizeScrollbarMin:  20,
		SizeProgressBar:   20,
		SizeSlider:        6,
		SizeSliderThumb:   16,
		SizeFieldMinWidth: 160,
	}
}

// baseDarkCfg returns the common dark ThemeCfg.
func baseDarkCfg() ThemeCfg {
	cfg := baseCfg()
	cfg.Name = "dark"
	cfg.ColorBackground = colorBackgroundDark
	cfg.ColorPanel = colorPanelDark
	cfg.ColorInterior = colorInteriorDark
	cfg.ColorHover = colorHoverDark
	cfg.ColorFocus = colorFocusDark
	cfg.ColorActive = colorActiveDark
	cfg.ColorBorder = colorBorderDark
	// No ColorSelect, no ColorBorderFocus: both resolve to the
	// accent, so selection and focus stay the same decision as the
	// accent (visual-refresh §4.3).
	cfg.ColorSeparator = colorSeparatorDark
	cfg.ColorAccent = colorAccentDark
	cfg.ColorSuccess = colorSuccessDark
	cfg.ColorWarning = colorWarningDark
	cfg.ColorError = colorErrorDark
	cfg.TitlebarDark = true
	cfg.TextStyleDef = DefaultTextStyle
	// Bordered by default since 2026-08: the call-site count behind
	// issue #325 found 90 of 104 example files explicitly calling
	// SetTheme(ThemeDark.WithBorders(true)), so bordered is what
	// applications actually want. Theme.WithBorders(false) restores
	// the borderless look.
	cfg.SizeBorder = sizeBorderDef
	// Elevation (visual-refresh §5.3): the dark preset floats its
	// popovers and modals.
	cfg.ShadowPopover = darkShadowPopover
	cfg.ShadowDialog = darkShadowDialog
	// Focus ring (visual-refresh § 5.4): the default preset carries a
	// ring so every wired focusable shows focus by default. Platform
	// themes that want their own (macOS) or none (Windows, GNOME —
	// border recolor only) override or leave it nil from baseCfg.
	cfg.FocusRing = darkFocusRing
	return cfg
}

// Preset themes.
var (
	ThemeDark  Theme
	ThemeLight Theme

	// Platform themes; see gui/theme_macos.go, gui/theme_gnome.go and
	// gui/theme_windows.go. Registered as "macos"/"macos-dark",
	// "gnome"/"gnome-dark" and "windows"/"windows-dark", and reached by
	// name through ThemeGet. Unexported: ThemeDark/ThemeLight are the
	// defaults an app assigns directly, these are presets a user
	// picks. ThemePicker lists them off the registry.
	themeMacOS       Theme
	themeMacOSDark   Theme
	themeGnome       Theme
	themeGnomeDark   Theme
	themeWindows     Theme
	themeWindowsDark Theme
)

// Unexported preset configs — the platform pair configs live in
// theme_macos.go, theme_gnome.go and theme_winui.go; the dark and
// light cfgs are built in init() below.
var (
	themeDarkCfg  ThemeCfg
	themeLightCfg ThemeCfg
)

func init() {
	// Dark.
	themeDarkCfg = baseDarkCfg()
	ThemeDark = ThemeMaker(themeDarkCfg)

	// Light.
	themeLightCfg = baseCfg()
	themeLightCfg.Name = "light"
	themeLightCfg.ColorBackground = colorBackgroundLight
	themeLightCfg.ColorPanel = colorPanelLight
	themeLightCfg.ColorInterior = colorInteriorLight
	themeLightCfg.ColorHover = colorHoverLight
	themeLightCfg.ColorFocus = colorFocusLight
	themeLightCfg.ColorActive = colorActiveLight
	themeLightCfg.ColorBorder = colorBorderLight
	// No ColorSelect, no ColorBorderFocus: both resolve to the
	// accent, so selection and focus stay the same decision as the
	// accent (visual-refresh §4.3). The dark preset does the same.
	themeLightCfg.ColorSeparator = colorSeparatorLight
	themeLightCfg.ColorAccent = colorAccentLight
	themeLightCfg.ColorSuccess = colorSuccessLight
	themeLightCfg.ColorWarning = colorWarningLight
	themeLightCfg.ColorError = colorErrorLight
	themeLightCfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  colorTextLight,
		Size:   sizeTextMedium,
	}
	// Elevation (visual-refresh §5.3), the light tier consts; the
	// dark pattern is in baseDarkCfg.
	themeLightCfg.ShadowPopover = lightShadowPopover
	themeLightCfg.ShadowDialog = lightShadowDialog
	// Focus ring: same construction as the dark preset's, tinted with
	// the light accent.
	themeLightCfg.FocusRing = lightFocusRing
	ThemeLight = ThemeMaker(themeLightCfg)

	// macOS, light and dark.
	themeMacOS = ThemeMaker(macOSCfg())
	themeMacOSDark = ThemeMaker(macOSDarkCfg())

	// GNOME, light and dark.
	themeGnome = ThemeMaker(gnomeCfg())
	themeGnomeDark = ThemeMaker(gnomeDarkCfg())

	// Windows, light and dark.
	themeWindows = ThemeMaker(windowsCfg())
	themeWindowsDark = ThemeMaker(windowsDarkCfg())

	// Register all preset themes.
	themeRegister(ThemeDark)
	themeRegister(ThemeLight)
	themeRegister(themeMacOS)
	themeRegister(themeMacOSDark)
	themeRegister(themeGnome)
	themeRegister(themeGnomeDark)
	themeRegister(themeWindows)
	themeRegister(themeWindowsDark)

	// Dark is both the app default and the initially installed theme.
	// applyTheme is what fills the default*Style mirrors — they carry no
	// literals of their own, so an app that never calls SetTheme still
	// gets ThemeDark everywhere rather than a mixture (issue #300). It
	// also sets guiTheme and installedThemeID.
	defaultTheme = &ThemeDark
	applyTheme(ThemeDark)
}
