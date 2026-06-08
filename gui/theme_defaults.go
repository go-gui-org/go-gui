package gui

// Light theme color vars.
var (
	colorBackgroundLight  = RGB(225, 225, 225)
	colorPanelLight       = RGB(205, 205, 215)
	colorInteriorLight    = RGB(195, 195, 215)
	colorHoverLight       = RGB(185, 185, 215)
	colorFocusLight       = RGB(175, 175, 215)
	colorActiveLight      = RGB(165, 165, 215)
	colorBorderLight      = RGB(135, 135, 165)
	colorSelectLight      = RGB(65, 105, 225)
	colorBorderFocusLight = RGB(0, 0, 165)
	colorTextLight        = RGB(32, 32, 32)
)

// Scroll constants.
const (
	scrollMultiplier float32 = 20
	scrollDeltaLine  float32 = 1
	scrollDeltaPage  float32 = 10
)

// baseCfg returns the shared sizing/spacing/widget-size fields
// common to all preset themes.
func baseCfg() ThemeCfg {
	return ThemeCfg{
		MonoFontFamily:   defaultMonoFontFamily,
		Padding:          PaddingMedium,
		PaddingSmall:     PaddingSmall,
		PaddingMedium:    PaddingMedium,
		PaddingLarge:     PaddingLarge,
		Radius:           RadiusMedium,
		RadiusSmall:      RadiusSmall,
		RadiusMedium:     RadiusMedium,
		RadiusLarge:      RadiusLarge,
		SpacingSmall:     SpacingSmall,
		SpacingMedium:    SpacingMedium,
		SpacingLarge:     SpacingLarge,
		SizeTextTiny:     SizeTextTiny,
		SizeTextXSmall:   SizeTextXSmall,
		SizeTextSmall:    SizeTextSmall,
		SizeTextMedium:   SizeTextMedium,
		SizeTextLarge:    SizeTextLarge,
		SizeTextXLarge:   SizeTextXLarge,
		ScrollMultiplier: scrollMultiplier,
		ScrollDeltaLine:  scrollDeltaLine,
		ScrollDeltaPage:  scrollDeltaPage,
		SizeSwitchWidth:  36,
		SizeSwitchHeight: 22,
		SizeRadio:        16,
		SizeScrollbar:    7,
		SizeScrollbarMin: 20,
		SizeProgressBar:  20,
		SizeSlider:       6,
		SizeSliderThumb:  16,
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
	cfg.ColorBorderFocus = colorSelectDark
	cfg.ColorSelect = colorSelectDark
	cfg.TitlebarDark = true
	cfg.TextStyleDef = DefaultTextStyle
	cfg.ColorError = RGBA(218, 54, 51, 255)
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
		Size:   SizeTextMedium,
	}
	return cfg
}

// Preset themes.
var (
	ThemeDark  Theme
	ThemeLight Theme
	ThemeBlue  Theme
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
	themeDarkNoPaddingCfg.Radius = RadiusNone
	themeDarkNoPadding = ThemeMaker(themeDarkNoPaddingCfg)

	// Dark bordered.
	themeDarkBorderedCfg = baseDarkCfg()
	themeDarkBorderedCfg.Name = "dark-bordered"
	themeDarkBorderedCfg.SizeBorder = SizeBorderDef
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
	themeLightCfg.ColorBorderFocus = colorBorderFocusLight
	themeLightCfg.ColorSelect = colorSelectLight
	themeLightCfg.ColorError = RGBA(200, 40, 40, 255)
	themeLightCfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  colorTextLight,
		Size:   SizeTextMedium,
	}
	ThemeLight = ThemeMaker(themeLightCfg)

	// Light no padding.
	themeLightNoPaddingCfg = themeLightCfg
	themeLightNoPaddingCfg.Name = "light-no-padding"
	themeLightNoPaddingCfg.Padding = PaddingNone
	themeLightNoPaddingCfg.SizeBorder = 0
	themeLightNoPaddingCfg.Radius = RadiusNone
	themeLightNoPadding = ThemeMaker(themeLightNoPaddingCfg)

	// Light bordered.
	themeLightBorderedCfg = themeLightCfg
	themeLightBorderedCfg.Name = "light-bordered"
	themeLightBorderedCfg.SizeBorder = SizeBorderDef
	themeLightBordered = ThemeMaker(themeLightBorderedCfg)

	// Blue.
	themeBlueCfg = baseBlueCfg()
	ThemeBlue = ThemeMaker(themeBlueCfg)

	// Blue bordered.
	themeBlueBorderedCfg = baseBlueCfg()
	themeBlueBorderedCfg.Name = "blue-dark-bordered"
	themeBlueBorderedCfg.SizeBorder = SizeBorderDef
	themeBlueBordered = ThemeMaker(themeBlueBorderedCfg)

	// Register all preset themes.
	ThemeRegister(ThemeDark)
	ThemeRegister(themeDarkNoPadding)
	ThemeRegister(themeDarkBordered)
	ThemeRegister(ThemeLight)
	ThemeRegister(themeLightNoPadding)
	ThemeRegister(themeLightBordered)
	ThemeRegister(ThemeBlue)
	ThemeRegister(themeBlueBordered)

	// Set default active theme to dark.
	guiTheme = ThemeDark
}
