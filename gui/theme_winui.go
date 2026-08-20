package gui

// Windows platform theme, modelled on WinUI / Fluent (Windows 11).
//
// Not a pixel match for WinUI — a similar feel. What reads as
// "Windows" at a glance: a near-white window ground, white control
// interiors, hairline borders, the WinUI blue as the one accent,
// squarer corners than macOS or GNOME, and flyouts that float over a
// modest shadow.
//
// The shadow tokens exist because ThemeCfg grew
// shadowPopover/shadowDialog in the macOS pass (issue #374); nil is
// the no-elevation answer, and Windows 11 keeps its flyout elevation
// smaller than macOS's. Focus is NOT the macOS glow: WinUI draws a
// hard accent outline, which this toolkit expresses with
// ColorBorderFocus. That is why FocusRing stays nil here.

// Windows elevation values.
//
// Package-level vars, not literals inside the cfg functions, because
// ThemeMaker copies the pointer into every popover style: one value
// per theme, shared by every shape in the window, never written
// through. Shared across polarities, like macOS: Windows does not
// lighten its shadows in dark mode, and a shadow meant for dark grey
// reads fine on light grey too.
//
// WinUI flyout elevation is subtle — small offset, tight blur — so
// these are tuned down from the macOS pair.
var (
	// Popover tier: menus, dropdowns, tooltips, toasts. Just enough
	// offset and blur to read as floating off the window ground.
	windowsShadowPopover = &BoxShadow{
		Color:      RGBA(0, 0, 0, 50),
		OffsetY:    3,
		BlurRadius: 8,
	}

	// Modal tier: dialogs and the command palette. Floats further
	// because it sits above everything.
	windowsShadowDialog = &BoxShadow{
		Color:      RGBA(0, 0, 0, 70),
		OffsetY:    8,
		BlurRadius: 16,
	}
)

// basewindowsCfg returns the geometry shared by both Windows
// polarities.
//
// Starts from baseCfg so the text ladder, spacing ladder and scroll
// deltas stay the toolkit's, and overrides only what Windows actually
// does differently.
func basewindowsCfg() ThemeCfg {
	cfg := baseCfg()

	// Hairlines. Windows 11 outlines controls at a single pixel.
	cfg.SizeBorder = 1

	// Corner rounding. WinUI is the squarer of the three platforms:
	// 4px controls, 2px chips, 8px flyouts.
	cfg.Radius = 4
	cfg.RadiusSmall = 2
	cfg.RadiusMedium = 4
	cfg.RadiusLarge = 8

	// Control geometry. The Windows 11 toggle is a 40x20 capsule —
	// flatter than the others. Scrollbars are chunkier than the
	// toolkit default.
	cfg.SizeSwitchWidth = 40
	cfg.SizeSwitchHeight = 20
	cfg.SizeScrollbar = 10

	// Text ladder at Windows' native body size (12), derived by
	// textSizes like every theme's (visual-refresh §2.1).
	setTextLadder(&cfg, 12)

	cfg.ShadowPopover = windowsShadowPopover
	cfg.ShadowDialog = windowsShadowDialog

	// Focus stays a hard accent outline (ColorBorderFocus), never the
	// macOS glow: FocusRing is deliberately left nil.
	//
	// Fonts need no override on Windows: defaultFontFamily is "", which
	// resolves to the backend's system face (Segoe UI on Windows).
	// Naming a family here would pin the theme to one machine's font
	// set.
	return cfg
}

// windowsCfg returns the light Windows ThemeCfg.
func windowsCfg() ThemeCfg {
	cfg := basewindowsCfg()
	cfg.Name = "windows"
	cfg.ColorBackground = ColorFromString("#F3F3F3")
	cfg.ColorPanel = ColorFromString("#FFFFFF")
	cfg.ColorInterior = ColorFromString("#FFFFFF")
	cfg.ColorHover = ColorFromString("#F5F5F5")
	cfg.ColorFocus = ColorFromString("#FFFFFF")
	cfg.ColorActive = ColorFromString("#E5E5E5")
	cfg.ColorBorder = ColorFromString("#D1D1D1")
	cfg.ColorSelect = ColorFromString("#005FB8")
	cfg.ColorBorderFocus = ColorFromString("#005FB8")
	cfg.ColorSuccess = ColorFromString("#0F7B0F")
	cfg.ColorWarning = ColorFromString("#9D5D00")
	cfg.ColorError = ColorFromString("#C42B1C")
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#1B1B1B"),
		Size:   12,
	}
	return cfg
}

// windowsDarkCfg returns the dark Windows ThemeCfg.
func windowsDarkCfg() ThemeCfg {
	cfg := basewindowsCfg()
	cfg.Name = "windows-dark"
	cfg.ColorBackground = ColorFromString("#202020")
	cfg.ColorPanel = ColorFromString("#2B2B2B")
	cfg.ColorInterior = ColorFromString("#3A3A3A")
	cfg.ColorHover = ColorFromString("#3B3B3B")
	cfg.ColorFocus = ColorFromString("#3A3A3A")
	cfg.ColorActive = ColorFromString("#4C4C4C")
	cfg.ColorBorder = ColorFromString("#5C5C5C")
	cfg.ColorSelect = ColorFromString("#60CDFF")
	cfg.ColorBorderFocus = ColorFromString("#60CDFF")
	cfg.ColorSuccess = ColorFromString("#6CCB5F")
	cfg.ColorWarning = ColorFromString("#FCE100")
	cfg.ColorError = ColorFromString("#FF99A4")
	cfg.TitlebarDark = true
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#FFFFFF"),
		Size:   12,
	}
	return cfg
}
