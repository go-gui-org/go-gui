package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestThemeMaker(t *testing.T) {
	t.Parallel()
	cfg := ThemeCfg{
		Name:             "test",
		ColorBackground:  RGB(48, 48, 48),
		ColorPanel:       RGB(64, 64, 64),
		ColorInterior:    RGB(74, 74, 74),
		ColorHover:       RGB(84, 84, 84),
		ColorFocus:       RGB(94, 94, 94),
		ColorActive:      RGB(104, 104, 104),
		ColorBorder:      RGB(100, 100, 100),
		ColorSelect:      RGB(65, 105, 225),
		TextStyleDef:     TextStyle{Color: RGB(225, 225, 225), Size: 16},
		PaddingSmall:     PaddingSmall,
		PaddingMedium:    paddingMedium,
		PaddingLarge:     PaddingLarge,
		Padding:          paddingMedium,
		Radius:           radiusMedium,
		RadiusSmall:      radiusSmall,
		RadiusMedium:     radiusMedium,
		RadiusLarge:      radiusLarge,
		SpacingTight:     SpacingTight,
		SpacingSmall:     SpacingSmall,
		SpacingMedium:    SpacingMedium,
		SpacingLarge:     SpacingLarge,
		SizeTextTiny:     sizeTextTiny,
		SizeTextXSmall:   sizeTextXSmall,
		SizeTextSmall:    sizeTextSmall,
		SizeTextMedium:   sizeTextMedium,
		SizeTextLarge:    sizeTextLarge,
		SizeTextXLarge:   sizeTextXLarge,
		SizeScrollbar:    7,
		SizeScrollbarMin: 20,
		SizeRadio:        15,
		SizeSwitchWidth:  34,
		SizeSwitchHeight: 20,
		ScrollMultiplier: 20,
		ScrollDeltaLine:  1,
		ScrollDeltaPage:  10,
	}
	theme := ThemeMaker(cfg)
	if theme.Name != "test" {
		t.Errorf("name = %q", theme.Name)
	}
	if theme.buttonStyle.Colors.Base != cfg.ColorInterior {
		t.Error("button color mismatch")
	}
	if theme.TextStyleDisplay.Size != sizeTextXLarge {
		t.Errorf("Display.Size = %f", theme.TextStyleDisplay.Size)
	}
}

// The tight tier must actually drive the composite-control gaps it was
// invented for (issue #344, audit §4): the tab header, submenu items and
// calendar cells fold their old private 2/1/2 into cfg.SpacingTight, and
// the toast stack folds its 8 into cfg.SpacingMedium. A future edit that
// hardcodes one of those values would silently regress the ladder, so
// pin the mapping.
func TestThemeMakerSpacingTierWiring(t *testing.T) {
	cfg := ThemeCfg{
		SpacingTight:  4,
		SpacingMedium: 20,
	}
	theme := ThemeMaker(cfg)

	if theme.SpacingTight != cfg.SpacingTight {
		t.Errorf("Theme.SpacingTight = %v, want %v", theme.SpacingTight, cfg.SpacingTight)
	}
	if theme.tabControlStyle.spacingHeader != cfg.SpacingTight {
		t.Errorf("tab spacingHeader = %v, want %v", theme.tabControlStyle.spacingHeader, cfg.SpacingTight)
	}
	if theme.menubarStyle.spacingSubmenu != cfg.SpacingTight {
		t.Errorf("submenu spacing = %v, want %v", theme.menubarStyle.spacingSubmenu, cfg.SpacingTight)
	}
	if theme.datePickerStyle.cellSpacing != cfg.SpacingTight {
		t.Errorf("calendar cellSpacing = %v, want %v", theme.datePickerStyle.cellSpacing, cfg.SpacingTight)
	}
	if theme.toastStyle.Spacing != cfg.SpacingMedium {
		t.Errorf("toast Spacing = %v, want %v", theme.toastStyle.Spacing, cfg.SpacingMedium)
	}
}

func TestSetTheme(t *testing.T) {
	saved := guiTheme
	t.Cleanup(func() { SetTheme(saved) })

	theme := Theme{
		buttonStyle: buttonStyle{Colors: ColorSet{Base: Red}},
		treeStyle:   TreeStyle{Colors: ColorSet{Hover: Blue}},
	}
	SetTheme(theme)
	if defaultButtonStyle.Colors.Base != Red {
		t.Error("SetTheme should update DefaultButtonStyle")
	}
	if defaultTreeStyle.Colors.Hover != Blue {
		t.Error("SetTheme should update DefaultTreeStyle")
	}
}

func TestWithColors(t *testing.T) {
	t.Parallel()
	theme := Theme{
		ColorHover: RGB(1, 1, 1),
		buttonStyle: buttonStyle{
			Colors: ColorSet{Hover: RGB(1, 1, 1), BorderFocus: RGB(2, 2, 2)},
		},
	}
	newHover := RGB(200, 200, 200)
	updated := theme.WithColors(ColorOverrides{
		ColorHover: newHover,
	})
	if updated.ColorHover != newHover {
		t.Error("theme hover not updated")
	}
	if updated.buttonStyle.Colors.Hover != newHover {
		t.Error("button hover not propagated")
	}
}

func TestAdjustFontSize(t *testing.T) {
	t.Parallel()
	cfg := ThemeCfg{
		TextStyleDef:     TextStyle{Color: RGB(225, 225, 225), Size: 16},
		SizeTextTiny:     10,
		SizeTextXSmall:   12,
		SizeTextSmall:    14,
		SizeTextMedium:   16,
		SizeTextLarge:    20,
		SizeTextXLarge:   24,
		Radius:           radiusMedium,
		RadiusSmall:      radiusSmall,
		RadiusMedium:     radiusMedium,
		RadiusLarge:      radiusLarge,
		SpacingTight:     SpacingTight,
		SpacingMedium:    SpacingMedium,
		PaddingMedium:    paddingMedium,
		SizeScrollbar:    7,
		SizeScrollbarMin: 20,
		SizeRadio:        15,
		SizeSwitchWidth:  34,
		SizeSwitchHeight: 20,
	}
	theme := ThemeMaker(cfg)
	bigger, err := theme.AdjustFontSize(2, 8, 32)
	if err != nil {
		t.Fatal(err)
	}
	if bigger.SizeTextMedium != 18 {
		t.Errorf("medium = %f, want 18", bigger.SizeTextMedium)
	}
	_, err = theme.AdjustFontSize(100, 1, 32)
	if err == nil {
		t.Error("should error on out of range")
	}
	_, err = theme.AdjustFontSize(1, 0, 32)
	if err == nil {
		t.Error("should error on minSize < 1")
	}
}

func TestThemeMakerBadgeStyle(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	// baseDarkCfg states no ColorSelect; it resolves to the accent,
	// and the accent-role slots follow the resolved value.
	if theme.badgeStyle.colorInfo != theme.ColorSelect {
		t.Error("badge info color should match the resolved select")
	}
	if theme.badgeStyle.dotSize != 8 {
		t.Errorf("dot size = %f, want 8", theme.badgeStyle.dotSize)
	}
}

func TestThemeMakerProgressBarStyle(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	if theme.progressBarStyle.Size != cfg.SizeProgressBar {
		t.Errorf("size = %f, want %f",
			theme.progressBarStyle.Size, cfg.SizeProgressBar)
	}
	if theme.progressBarStyle.colorBar != theme.ColorSelect {
		t.Error("bar color should match the resolved select")
	}
}

func TestWithColorsBadge(t *testing.T) {
	t.Parallel()
	theme := ThemeMaker(baseDarkCfg())
	sel := RGB(100, 200, 50)
	updated := theme.WithColors(ColorOverrides{
		ColorSelect: sel,
	})
	if updated.badgeStyle.colorInfo != sel {
		t.Error("badge info not propagated from select")
	}
}

func TestThemeBoldTypeface(t *testing.T) {
	t.Parallel()
	theme := ThemeMaker(baseDarkCfg())
	bold := []struct {
		name  string
		style TextStyle
	}{
		{"Display", theme.TextStyleDisplay},
		{"Title", theme.TextStyleTitle},
		{"TitleSmall", theme.TextStyleTitleSmall},
		{"BodySmall.Bold", theme.TextStyleBodySmall.Bold()},
		{"Caption.Bold", theme.TextStyleCaption.Bold()},
		{"CaptionSmall.Bold", theme.TextStyleCaptionSmall.Bold()},
	}
	for _, s := range bold {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			if s.style.Typeface != glyph.TypefaceBold {
				t.Errorf("%s.Typeface = %d, want TypefaceBold(%d)",
					s.name, s.style.Typeface, glyph.TypefaceBold)
			}
		})
	}
	normal := []struct {
		name  string
		style TextStyle
	}{
		{"Display.Roman", theme.TextStyleDisplay.Roman()},
		{"BodyLarge", theme.TextStyleBodyLarge},
		{"Body", theme.TextStyleBody},
		{"BodySmall", theme.TextStyleBodySmall},
		{"Caption", theme.TextStyleCaption},
		{"CaptionSmall", theme.TextStyleCaptionSmall},
	}
	for _, s := range normal {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			if s.style.Typeface != 0 {
				t.Errorf("%s.Typeface = %d, want 0 (regular)",
					s.name, s.style.Typeface)
			}
		})
	}
}

func TestWithColorsSlider(t *testing.T) {
	t.Parallel()
	theme := ThemeMaker(baseDarkCfg())
	hover := RGB(99, 99, 99)
	updated := theme.WithColors(ColorOverrides{
		ColorHover: hover,
	})
	if updated.sliderStyle.Colors.Hover != hover {
		t.Error("slider hover not propagated")
	}
}

func TestThemeMakerTreeStyle(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	if theme.treeStyle.Colors.Hover != cfg.ColorHover {
		t.Errorf("TreeStyle.ColorHover = %v, want %v",
			theme.treeStyle.Colors.Hover, cfg.ColorHover)
	}
	if theme.treeStyle.indent != 25 {
		t.Errorf("TreeStyle.Indent = %f, want 25",
			theme.treeStyle.indent)
	}
}

func TestThemeWithPadding(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	cfg.Padding = PadAll(4)
	cfg.PaddingSmall = PadAll(3)
	cfg.PaddingMedium = PadAll(6)
	cfg.PaddingLarge = PadAll(8)
	cfg.SizeBorder = 2
	cfg.Radius = 5
	cfg.RadiusSmall = 3
	cfg.RadiusMedium = 6
	cfg.RadiusLarge = 10
	theme := ThemeMaker(cfg)

	// Off: every padding/radius/border is zeroed.
	flat := theme.WithPadding(false)
	if flat.Cfg.Padding != PaddingNone ||
		flat.Cfg.PaddingSmall != PaddingNone ||
		flat.Cfg.PaddingMedium != PaddingNone ||
		flat.Cfg.PaddingLarge != PaddingNone {
		t.Error("WithPadding(false) must zero all paddings")
	}
	if flat.Cfg.SizeBorder != 0 {
		t.Errorf("SizeBorder = %v, want 0", flat.Cfg.SizeBorder)
	}
	if flat.Cfg.Radius != radiusNone ||
		flat.Cfg.RadiusSmall != radiusNone ||
		flat.Cfg.RadiusMedium != radiusNone ||
		flat.Cfg.RadiusLarge != radiusNone {
		t.Error("WithPadding(false) must zero all radii")
	}
	// Non-padding fields survive.
	if flat.Name != theme.Name {
		t.Error("WithPadding(false) must not touch the theme name")
	}

	// On: rebuild from the stored config, restoring the values.
	restored := theme.WithPadding(true)
	if restored.Cfg.Padding != cfg.Padding ||
		restored.Cfg.SizeBorder != cfg.SizeBorder ||
		restored.Cfg.Radius != cfg.Radius {
		t.Error("WithPadding(true) must restore the stored config")
	}
}

// A strip followed by a restore round-trips: the stripped theme
// remembers the pre-strip configuration instead of rebuilding from
// its own zeroed Cfg.
func TestThemeWithPaddingRoundTrip(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	cfg.Padding = PadAll(4)
	cfg.SizeBorder = 2
	cfg.Radius = 5
	theme := ThemeMaker(cfg)

	flat := theme.WithPadding(false)
	restored := flat.WithPadding(true)
	if restored.Cfg.Padding != cfg.Padding ||
		restored.Cfg.PaddingSmall != cfg.PaddingSmall ||
		restored.Cfg.PaddingMedium != cfg.PaddingMedium ||
		restored.Cfg.PaddingLarge != cfg.PaddingLarge {
		t.Error("WithPadding(false).WithPadding(true) must restore paddings")
	}
	if restored.Cfg.SizeBorder != cfg.SizeBorder {
		t.Errorf("SizeBorder = %v, want %v",
			restored.Cfg.SizeBorder, cfg.SizeBorder)
	}
	if restored.Cfg.Radius != cfg.Radius ||
		restored.Cfg.RadiusSmall != cfg.RadiusSmall ||
		restored.Cfg.RadiusMedium != cfg.RadiusMedium ||
		restored.Cfg.RadiusLarge != cfg.RadiusLarge {
		t.Error("WithPadding(false).WithPadding(true) must restore radii")
	}
}
