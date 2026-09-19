package gui

// GNOME platform theme, modelled on Adwaita / libadwaita.
//
// Not a pixel match for a GTK desktop — a similar feel. What reads as
// "GNOME" at a glance: a light grey window ground with white control
// interiors, hairline borders, the libadwaita blue as the one accent,
// gently rounded corners (rounder than Windows, not as round as
// nothing), and popovers that float over a subtle shadow.
//
// The shadow tokens exist because ThemeCfg grew
// shadowPopover/shadowDialog in the macOS pass (issue #374); nil is
// the no-elevation answer, and GNOME deliberately keeps it subtle.
// Focus is NOT the macOS glow: Adwaita draws a hard accent outline,
// which this toolkit expresses with ColorBorderFocus. That is why
// FocusRing stays nil here.

// GNOME elevation values.
//
// Package-level vars, not literals inside the cfg functions, because
// ThemeMaker copies the pointer into every popover style: one value
// per theme, shared by every shape in the window, never written
// through. Shared across polarities, like macOS: Adwaita keeps its
// shadows dark in dark mode, and a shadow meant for dark grey reads
// fine on light grey too.
//
// Adwaita's elevation ladders are small — elevation-1 is a 1px,
// low-alpha step, not the 5px float macOS uses — so these are tuned
// down from the macOS pair.
var (
	// Popover tier: menus, dropdowns, tooltips, toasts. Just enough
	// offset and blur to read as floating off the window ground.
	gnomeShadowPopover = &BoxShadow{
		Color:      RGBA(0, 0, 0, 60),
		OffsetY:    3,
		BlurRadius: 12,
	}

	// Modal tier: dialogs and the command palette. Floats further
	// because it sits above everything.
	gnomeShadowDialog = &BoxShadow{
		Color:      RGBA(0, 0, 0, 80),
		OffsetY:    8,
		BlurRadius: 24,
	}
)

// basegnomeCfg returns the geometry shared by both GNOME polarities.
//
// Starts from baseCfg so the text ladder, spacing ladder and scroll
// deltas stay the toolkit's, and overrides only what GNOME actually
// does differently.
func basegnomeCfg() ThemeCfg {
	cfg := baseCfg()

	// Hairlines. Adwaita outlines controls at a single pixel and leans
	// on fill contrast, the same call macOS makes.
	cfg.SizeBorder = 1

	// Control geometry. Adwaita switches are the biggest of the three
	// platforms: 40x24 with the knob travel to match.
	cfg.SizeSwitchWidth = 40
	cfg.SizeSwitchHeight = 24
	cfg.SizeScrollbar = 8

	// Text ladder at GNOME's native body size (15), derived by
	// textSizes like every theme's (visual-refresh §2.1).
	setTextLadder(&cfg, 15)

	cfg.ShadowPopover = gnomeShadowPopover
	cfg.ShadowDialog = gnomeShadowDialog

	// Focus stays a hard accent outline (ColorBorderFocus), never the
	// macOS glow: FocusRing is deliberately left nil.
	//
	// Fonts need no override on Linux: defaultFontFamily is "", which
	// resolves to the backend's system face (Cantarell on a GNOME
	// desktop). Naming a family here would pin the theme to one
	// machine's font set.
	return cfg
}

// gnomeCfg returns the light GNOME ThemeCfg.
func gnomeCfg() ThemeCfg {
	cfg := basegnomeCfg()
	cfg.Name = "gnome"
	cfg.ColorBackground = mustThemeColor("#F6F5F4")
	cfg.ColorPanel = mustThemeColor("#FAFAFA")
	cfg.ColorInterior = mustThemeColor("#FFFFFF")
	cfg.ColorHover = mustThemeColor("#F1EFEE")
	cfg.ColorFocus = mustThemeColor("#FFFFFF")
	cfg.ColorActive = mustThemeColor("#E2E0DE")
	cfg.ColorBorder = mustThemeColor("#C0BFBC")
	cfg.ColorSelect = mustThemeColor("#3584E4")
	cfg.ColorBorderFocus = mustThemeColor("#3584E4")
	cfg.ColorSuccess = mustThemeColor("#2EC27E")
	cfg.ColorWarning = mustThemeColor("#F5C211")
	cfg.ColorError = mustThemeColor("#E01B24")
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  mustThemeColor("#1E1E1E"),
		Size:   15,
	}
	return cfg
}

// gnomeDarkCfg returns the dark GNOME ThemeCfg.
func gnomeDarkCfg() ThemeCfg {
	cfg := basegnomeCfg()
	cfg.Name = "gnome-dark"
	cfg.ColorBackground = mustThemeColor("#242424")
	cfg.ColorPanel = mustThemeColor("#2E2E2E")
	cfg.ColorInterior = mustThemeColor("#1E1E1E")
	cfg.ColorHover = mustThemeColor("#333333")
	cfg.ColorFocus = mustThemeColor("#1E1E1E")
	cfg.ColorActive = mustThemeColor("#3D3D3D")
	cfg.ColorBorder = mustThemeColor("#505050")
	cfg.ColorSelect = mustThemeColor("#3584E4")
	cfg.ColorBorderFocus = mustThemeColor("#3584E4")
	cfg.ColorSuccess = mustThemeColor("#26A269")
	cfg.ColorWarning = mustThemeColor("#C88800")
	cfg.ColorError = mustThemeColor("#C01C28")
	cfg.TitlebarDark = true
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  mustThemeColor("#FFFFFF"),
		Size:   15,
	}
	return cfg
}
