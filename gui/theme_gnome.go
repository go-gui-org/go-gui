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

	// Corner rounding. libadwaita keeps controls rounder than Windows:
	// ~6px buttons and entries, 9-12px menus and dialogs.
	cfg.Radius = 6
	cfg.RadiusSmall = 4
	cfg.RadiusMedium = 6
	cfg.RadiusLarge = 12

	// Control geometry. Adwaita switches are the biggest of the three
	// platforms: 40x24 with the knob travel to match.
	cfg.SizeSwitchWidth = 40
	cfg.SizeSwitchHeight = 24
	cfg.SizeScrollbar = 8

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
	cfg.ColorBackground = ColorFromString("#F6F5F4")
	cfg.ColorPanel = ColorFromString("#FAFAFA")
	cfg.ColorInterior = ColorFromString("#FFFFFF")
	cfg.ColorHover = ColorFromString("#F1EFEE")
	cfg.ColorFocus = ColorFromString("#FFFFFF")
	cfg.ColorActive = ColorFromString("#E2E0DE")
	cfg.ColorBorder = ColorFromString("#C0BFBC")
	cfg.ColorSelect = ColorFromString("#3584E4")
	cfg.ColorBorderFocus = ColorFromString("#3584E4")
	cfg.ColorSuccess = ColorFromString("#2EC27E")
	cfg.ColorWarning = ColorFromString("#F5C211")
	cfg.ColorError = ColorFromString("#E01B24")
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#1E1E1E"),
		Size:   sizeTextMedium,
	}
	return cfg
}

// gnomeDarkCfg returns the dark GNOME ThemeCfg.
func gnomeDarkCfg() ThemeCfg {
	cfg := basegnomeCfg()
	cfg.Name = "gnome-dark"
	cfg.ColorBackground = ColorFromString("#242424")
	cfg.ColorPanel = ColorFromString("#2E2E2E")
	cfg.ColorInterior = ColorFromString("#1E1E1E")
	cfg.ColorHover = ColorFromString("#333333")
	cfg.ColorFocus = ColorFromString("#1E1E1E")
	cfg.ColorActive = ColorFromString("#3D3D3D")
	cfg.ColorBorder = ColorFromString("#505050")
	cfg.ColorSelect = ColorFromString("#3584E4")
	cfg.ColorBorderFocus = ColorFromString("#3584E4")
	cfg.ColorSuccess = ColorFromString("#26A269")
	cfg.ColorWarning = ColorFromString("#C88800")
	cfg.ColorError = ColorFromString("#C01C28")
	cfg.TitlebarDark = true
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#FFFFFF"),
		Size:   sizeTextMedium,
	}
	return cfg
}
