package gui

// macOS platform theme.
//
// Not a pixel match for Aqua — a similar feel. What actually reads as
// "macOS" at a glance is a short list: a light grey window ground with
// white control interiors, hairline borders, systemBlue as the one
// accent, gently rounded corners, floating popovers with a real
// shadow, and a soft accent glow for focus rather than a hard border.
//
// The first two of those were already expressible. The last two are
// why ThemeCfg grew shadowPopover/shadowDialog/focusRing; see the
// comments there for why they are pointers and why nil is the
// no-elevation answer that leaves every older theme untouched.

// macOS elevation and focus values.
//
// Package-level vars, not literals inside the cfg functions, because
// ThemeMaker copies the pointer into every popover style: one value
// per theme, shared by every shape in the window, never written
// through. The two shadows are shared across polarities — macOS does
// not lighten its shadows in dark mode, it deepens them, and the
// deeper of the two reads correctly on both grounds. The ring is not
// shared; it carries the accent, which does shift.
var (
	// Popover tier: menus, dropdowns, tooltips, toasts. Offset down
	// and blurred wide enough to read as floating at a glance without
	// looking like a drop shadow from a design tool.
	macOSShadowPopover = &BoxShadow{
		Color:      RGBA(0, 0, 0, 76),
		OffsetY:    5,
		BlurRadius: 20,
	}

	// Modal tier: dialogs and the command palette. Floats further
	// because it sits above everything, not just above the row under
	// it.
	macOSShadowDialog = &BoxShadow{
		Color:      RGBA(0, 0, 0, 102),
		OffsetY:    12,
		BlurRadius: 32,
	}

	// Focus ring. Zero offset so the glow is concentric with the
	// control, and a small blur so it reads as a ring rather than a
	// cloud. Rendered behind the control's fill (renderContainer emits
	// the shadow first), which is what keeps it from tinting the
	// control's own interior.
	//
	// One per polarity, unlike the shadows: the ring is tinted with the
	// theme's own accent, and macOS shifts that accent between light
	// (#007AFF) and dark (#0A84FF).
	macOSFocusRing = &BoxShadow{
		Color:      RGBA(0, 122, 255, 90),
		BlurRadius: 3,
	}

	macOSFocusRingDark = &BoxShadow{
		Color:      RGBA(10, 132, 255, 110),
		BlurRadius: 3,
	}
)

// baseMacOSCfg returns the geometry shared by both macOS polarities.
//
// Starts from baseCfg so the text ladder, spacing ladder and scroll
// deltas stay the toolkit's, and overrides only what macOS actually
// does differently.
func baseMacOSCfg() ThemeCfg {
	cfg := baseCfg()

	// Hairlines. sizeBorderDef (1.5) is a visible stroke; macOS draws
	// control outlines at a single pixel and leans on fill contrast
	// instead.
	cfg.SizeBorder = 1

	// Control geometry. The switch is a true capsule — height 22 with
	// a 38 width gives the knob room to travel without the track
	// looking stretched.
	cfg.SizeSwitchWidth = 38
	cfg.SizeSwitchHeight = 22
	cfg.SizeScrollbar = 8

	// Text ladder at macOS's native body size (13), derived by
	// textSizes like every theme's (visual-refresh §2.1).
	setTextLadder(&cfg, 13)

	cfg.ShadowPopover = macOSShadowPopover
	cfg.ShadowDialog = macOSShadowDialog
	cfg.FocusRing = macOSFocusRing

	// Fonts need no override on darwin: defaultFontFamily is "", which
	// resolves to the backend's system face, and defaultMonoFontFamily
	// is already Menlo. Naming a family here would pin the theme to
	// one machine's font set.
	return cfg
}

// macOSCfg returns the light macOS ThemeCfg.
func macOSCfg() ThemeCfg {
	cfg := baseMacOSCfg()
	cfg.Name = "macos"
	cfg.ColorBackground = ColorFromString("#ECECEC")
	cfg.ColorPanel = ColorFromString("#F5F5F5")
	cfg.ColorInterior = ColorFromString("#FFFFFF")
	cfg.ColorHover = ColorFromString("#F0F0F0")
	cfg.ColorFocus = ColorFromString("#FFFFFF")
	cfg.ColorActive = ColorFromString("#E1E1E1")
	cfg.ColorBorder = ColorFromString("#C6C6C8")
	cfg.ColorSelect = ColorFromString("#007AFF")
	cfg.ColorBorderFocus = ColorFromString("#007AFF")
	cfg.ColorSuccess = ColorFromString("#28A745")
	cfg.ColorWarning = ColorFromString("#FF9500")
	cfg.ColorError = ColorFromString("#FF3B30")
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#1D1D1F"),
		Size:   13,
	}
	return cfg
}

// macOSDarkCfg returns the dark macOS ThemeCfg.
func macOSDarkCfg() ThemeCfg {
	cfg := baseMacOSCfg()
	cfg.Name = "macos-dark"
	cfg.ColorBackground = ColorFromString("#1E1E1E")
	cfg.ColorPanel = ColorFromString("#2A2A2A")
	cfg.ColorInterior = ColorFromString("#3A3A3C")
	cfg.ColorHover = ColorFromString("#464648")
	cfg.ColorFocus = ColorFromString("#3A3A3C")
	cfg.ColorActive = ColorFromString("#545456")
	cfg.ColorBorder = ColorFromString("#48484A")
	cfg.ColorSelect = ColorFromString("#0A84FF")
	cfg.ColorBorderFocus = ColorFromString("#0A84FF")
	cfg.ColorSuccess = ColorFromString("#32D74B")
	cfg.ColorWarning = ColorFromString("#FF9F0A")
	cfg.ColorError = ColorFromString("#FF453A")
	cfg.TitlebarDark = true
	cfg.FocusRing = macOSFocusRingDark
	cfg.TextStyleDef = TextStyle{
		Family: defaultFontFamily,
		Color:  ColorFromString("#FFFFFF"),
		Size:   13,
	}
	return cfg
}
