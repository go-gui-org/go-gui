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
var (
	darkShadowPopover = &BoxShadow{
		Color:      RGBA(0, 0, 0, 140),
		OffsetY:    4,
		BlurRadius: 12,
	}
	darkShadowDialog = &BoxShadow{
		Color:      RGBA(0, 0, 0, 170),
		OffsetY:    12,
		BlurRadius: 32,
	}

	// Light elevation tiers. The same geometry as dark with a softer
	// ink: dark-on-dark shadows are barely visible at the low alphas a
	// light surface needs, so the light ink is the page tint instead.
	lightShadowPopover = &BoxShadow{
		Color:      RGBA(16, 24, 40, 40),
		OffsetY:    4,
		BlurRadius: 12,
	}
	lightShadowDialog = &BoxShadow{
		Color:      RGBA(16, 24, 40, 60),
		OffsetY:    12,
		BlurRadius: 32,
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
		SizeSwitchWidth:   36,
		SizeSwitchHeight:  22,
		SizeRadio:         16,
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
	// the borderless look. The "dark-bordered" preset is now identical
	// to ThemeDark but stays registered for name-based theme selection.
	cfg.SizeBorder = sizeBorderDef
	// Elevation (visual-refresh §5.3): the dark presets float their
	// popovers and modals. Derived presets (dark-no-padding,
	// dark-bordered) copy the cfg and inherit it, which is what makes
	// them dark's visual twins.
	cfg.ShadowPopover = darkShadowPopover
	cfg.ShadowDialog = darkShadowDialog
	return cfg
}

// baseBlueCfg returns the blue ThemeCfg.
func baseBlueCfg() ThemeCfg {
	cfg := baseCfg()
	cfg.Name = "blue-dark"
	cfg.ColorBackground = ColorFromString("#151C30")
	cfg.ColorPanel = ColorFromString("#1C243F")
	cfg.ColorInterior = ColorFromString("#202A49")
	cfg.ColorHover = ColorFromString("#243054")
	cfg.ColorFocus = ColorFromString("#29365E")
	cfg.ColorActive = ColorFromString("#2D3C68")
	cfg.ColorBorder = ColorFromString("#364263")
	cfg.ColorBorderFocus = ColorFromString("#617AC3")
	cfg.ColorSelect = ColorFromString("#3E65D8")
	cfg.ColorError = RGBA(218, 54, 51, 255)
	cfg.TitlebarDark = true
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#E1E1E1"),
		Size:   sizeTextMedium,
	}
	return cfg
}

// Preset themes.
var (
	ThemeDark  Theme
	ThemeLight Theme
	themeBlue  Theme

	// Platform themes; see gui/theme_macos.go, gui/theme_gnome.go and
	// gui/theme_windows.go. Registered as "macos"/"macos-dark",
	// "gnome"/"gnome-dark" and "windows"/"windows-dark", and reached by
	// name through ThemeGet. Unexported, following themeBlue rather
	// than ThemeDark/ThemeLight: those two are the defaults an app
	// assigns directly, these are presets a user picks. ThemePicker
	// lists them off the registry.
	themeMacOS       Theme
	themeMacOSDark   Theme
	themeGnome       Theme
	themeGnomeDark   Theme
	themeWindows     Theme
	themeWindowsDark Theme
)

// Unexported preset configs and derived themes — kept for
// registration and backward compatibility.
var (
	themeDarkCfg          ThemeCfg
	themeDarkNoPaddingCfg ThemeCfg
	themeDarkNoPadding    Theme
	themeDarkBorderedCfg  ThemeCfg
	themeDarkBordered     Theme

	themeLightCfg          ThemeCfg
	themeLightNoPaddingCfg ThemeCfg
	themeLightNoPadding    Theme
	themeLightBorderedCfg  ThemeCfg
	themeLightBordered     Theme

	themeBlueCfg         ThemeCfg
	themeBlueBorderedCfg ThemeCfg
	themeBlueBordered    Theme
)

func init() {
	// Dark.
	themeDarkCfg = baseDarkCfg()
	ThemeDark = ThemeMaker(themeDarkCfg)

	// Dark no padding.
	themeDarkNoPaddingCfg = baseDarkCfg()
	themeDarkNoPaddingCfg.Name = "dark-no-padding"
	themeDarkNoPaddingCfg.Padding = PaddingNone
	themeDarkNoPaddingCfg.SizeBorder = 0
	themeDarkNoPaddingCfg.Radius = radiusNone
	themeDarkNoPadding = ThemeMaker(themeDarkNoPaddingCfg)

	// Dark bordered. Now identical to ThemeDark (baseDarkCfg carries
	// the border); kept registered for backward compatibility with
	// name-based theme selection.
	themeDarkBorderedCfg = baseDarkCfg()
	themeDarkBorderedCfg.Name = "dark-bordered"
	themeDarkBordered = ThemeMaker(themeDarkBorderedCfg)

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
	// dark pattern is in baseDarkCfg. Derived presets (light-no-padding,
	// light-bordered) copy the cfg and inherit it.
	themeLightCfg.ShadowPopover = lightShadowPopover
	themeLightCfg.ShadowDialog = lightShadowDialog
	ThemeLight = ThemeMaker(themeLightCfg)

	// Light no padding.
	themeLightNoPaddingCfg = themeLightCfg
	themeLightNoPaddingCfg.Name = "light-no-padding"
	themeLightNoPaddingCfg.Padding = PaddingNone
	themeLightNoPaddingCfg.SizeBorder = 0
	themeLightNoPaddingCfg.Radius = radiusNone
	themeLightNoPadding = ThemeMaker(themeLightNoPaddingCfg)

	// Light bordered.
	themeLightBorderedCfg = themeLightCfg
	themeLightBorderedCfg.Name = "light-bordered"
	themeLightBorderedCfg.SizeBorder = sizeBorderDef
	themeLightBordered = ThemeMaker(themeLightBorderedCfg)

	// Blue.
	themeBlueCfg = baseBlueCfg()
	themeBlue = ThemeMaker(themeBlueCfg)

	// macOS, light and dark.
	themeMacOS = ThemeMaker(macOSCfg())
	themeMacOSDark = ThemeMaker(macOSDarkCfg())

	// GNOME, light and dark.
	themeGnome = ThemeMaker(gnomeCfg())
	themeGnomeDark = ThemeMaker(gnomeDarkCfg())

	// Windows, light and dark.
	themeWindows = ThemeMaker(windowsCfg())
	themeWindowsDark = ThemeMaker(windowsDarkCfg())

	// Blue bordered.
	themeBlueBorderedCfg = baseBlueCfg()
	themeBlueBorderedCfg.Name = "blue-dark-bordered"
	themeBlueBorderedCfg.SizeBorder = sizeBorderDef
	themeBlueBordered = ThemeMaker(themeBlueBorderedCfg)

	// Register all preset themes.
	themeRegister(ThemeDark)
	themeRegister(themeDarkNoPadding)
	themeRegister(themeDarkBordered)
	themeRegister(ThemeLight)
	themeRegister(themeLightNoPadding)
	themeRegister(themeLightBordered)
	themeRegister(themeBlue)
	themeRegister(themeBlueBordered)
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
