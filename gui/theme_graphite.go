package gui

// Graphite: a from-scratch dark theme.
//
// Not a reskin of ThemeDark — every slot below is staked on a norm,
// and where two norms disagree the file says which one won:
//
//   - Ground #161618 sits in the GitHub/VS Code depth band: dark
//     enough for elevation-by-lightness to read, never pure black.
//     Black grounds halate white text and leave shadows (which are
//     themselves black) nowhere to go.
//   - Elevation is lightness, not shadow (the Material dark-model
//     rule: shadows are invisible on dark). Each tier rises ~8-11
//     steps per channel, and the rise is the hierarchy.
//   - Definition comes from opaque hairlines (the GitHub school),
//     not translucent washes. Border #444A52 reads ~2:1 on the
//     ground, separator #666769 ~3.2:1 — a divider spans the full
//     width, so it carries more presence than an edge that hugs a
//     shape.
//   - Near-neutral fills, blue leading by two steps. The tint is
//     only there so a white wash (focus glow, subtle fills) lays no
//     hue shift; at two steps it never reads as blue.
//   - Accent violet #6E6AD2 (Linear's proven value) instead of the
//     toolkit's blue: a second dark theme must be recognizable at a
//     glance, and white text on it clears AA (4.5:1), so primary
//     buttons keep white labels.
//   - Semantic colors are GitHub-dark's (#F85149/#3FB950/#D29922):
//     hues picked as fills, brightened until they survive as text.
//     Where they fall short of 4.5:1 as text, ThemeMaker's
//     readableOnPage lifts the derived role automatically.
//   - Text #E7E9EC with derived quiet roles (the dark ladder is
//     ~63/50/39% alpha, the Material 87/60/38 spirit): roles name a
//     reason, never an alpha.
//   - Selection follows the accent (ColorSelect unset): one decision,
//     per the toolkit's unity norm.
//   - Geometry is modern-desktop: 8px control radius, hairlines,
//     body 14 inherited from baseCfg.

// graphiteFocusRing is the outside-the-bounds focus glow, tinted
// with graphite's own violet. Same zero-offset, small-blur
// construction as the other dark rings; violet needs a touch more
// alpha (0.30 vs 0.25) to read at the same weight.
var graphiteFocusRing = &BoxShadow{
	Color:      RGBA(110, 106, 210, 77),
	BlurRadius: 2,
}

// graphiteCfg returns the graphite ThemeCfg.
func graphiteCfg() ThemeCfg {
	cfg := baseCfg()
	cfg.Name = "graphite"
	cfg.ColorBackground = mustThemeColor("#161618")
	cfg.ColorPanel = mustThemeColor("#1E1E20")
	cfg.ColorInterior = mustThemeColor("#272729")
	cfg.ColorHover = mustThemeColor("#313133")
	cfg.ColorFocus = mustThemeColor("#3C3C3E")
	cfg.ColorActive = mustThemeColor("#49494B")
	// Opaque hairlines: definition, not wash. BorderFocus stays
	// unset so it follows the selection color.
	cfg.ColorBorder = mustThemeColor("#444A52")
	cfg.ColorSeparator = mustThemeColor("#666769")
	cfg.ColorAccent = mustThemeColor("#6E6AD2")
	cfg.ColorSuccess = mustThemeColor("#3FB950")
	cfg.ColorWarning = mustThemeColor("#D29922")
	cfg.ColorError = mustThemeColor("#F85149")
	cfg.TitlebarDark = true
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  mustThemeColor("#E7E9EC"),
		Size:   sizeTextMedium,
	}
	// Modern radius ladder: 8px controls, small chips, floating
	// panels one step rounder. Hairline borders like every other
	// preset (visual-refresh §5.1).
	cfg.SizeBorder = sizeBorderDef
	cfg.Radius = 8
	cfg.RadiusSmall = 4
	cfg.RadiusMedium = 8
	cfg.RadiusLarge = 12
	// Elevation tiers shared with the default dark preset: the
	// tiers are the spec's, the palette above is graphite's.
	cfg.ShadowPopover = darkShadowPopover
	cfg.ShadowDialog = darkShadowDialog
	cfg.FocusRing = graphiteFocusRing
	return cfg
}

// themeGraphite is the built graphite preset. Unexported like the
// platform pairs: apps reach it by name through ThemeGet, and
// ThemePicker lists it off the registry.
var themeGraphite Theme

func init() {
	themeGraphite = ThemeMaker(graphiteCfg())
	ThemeRegister(themeGraphite)
}
