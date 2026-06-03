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
		PaddingMedium:    PaddingMedium,
		PaddingLarge:     PaddingLarge,
		Padding:          PaddingMedium,
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
		SizeScrollbar:    7,
		SizeScrollbarMin: 20,
		SizeRadio:        16,
		SizeSwitchWidth:  36,
		SizeSwitchHeight: 22,
		ScrollMultiplier: 20,
		ScrollDeltaLine:  1,
		ScrollDeltaPage:  10,
	}
	theme := ThemeMaker(cfg)
	if theme.Name != "test" {
		t.Errorf("name = %q", theme.Name)
	}
	if theme.ButtonStyle.Color != cfg.ColorInterior {
		t.Error("button color mismatch")
	}
	if theme.N1.Size != SizeTextXLarge {
		t.Errorf("N1.Size = %f", theme.N1.Size)
	}
}

func TestSetTheme(t *testing.T) {
	saved := guiTheme
	t.Cleanup(func() { SetTheme(saved) })

	theme := Theme{
		ButtonStyle: ButtonStyle{Color: Red},
		TreeStyle:   TreeStyle{ColorHover: Blue},
	}
	SetTheme(theme)
	if DefaultButtonStyle.Color != Red {
		t.Error("SetTheme should update DefaultButtonStyle")
	}
	if DefaultTreeStyle.ColorHover != Blue {
		t.Error("SetTheme should update DefaultTreeStyle")
	}
}

func TestWithColors(t *testing.T) {
	t.Parallel()
	theme := Theme{
		ColorHover: RGB(1, 1, 1),
		ButtonStyle: ButtonStyle{
			ColorHover:       RGB(1, 1, 1),
			ColorBorderFocus: RGB(2, 2, 2),
		},
	}
	newHover := RGB(200, 200, 200)
	updated := theme.WithColors(ColorOverrides{
		ColorHover: &newHover,
	})
	if updated.ColorHover != newHover {
		t.Error("theme hover not updated")
	}
	if updated.ButtonStyle.ColorHover != newHover {
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
		Radius:           RadiusMedium,
		RadiusSmall:      RadiusSmall,
		RadiusMedium:     RadiusMedium,
		RadiusLarge:      RadiusLarge,
		SpacingMedium:    SpacingMedium,
		PaddingMedium:    PaddingMedium,
		SizeScrollbar:    7,
		SizeScrollbarMin: 20,
		SizeRadio:        16,
		SizeSwitchWidth:  36,
		SizeSwitchHeight: 22,
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

func TestWithButtonStyle(t *testing.T) {
	t.Parallel()
	theme := Theme{}
	s := ButtonStyle{Color: Blue}
	updated := theme.WithButtonStyle(s)
	if updated.ButtonStyle.Color != Blue {
		t.Error("WithButtonStyle not applied")
	}
}

func TestThemeMakerBadgeStyle(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	if theme.BadgeStyle.ColorInfo != cfg.ColorSelect {
		t.Error("badge info color should match select")
	}
	if theme.BadgeStyle.DotSize != 8 {
		t.Errorf("dot size = %f, want 8", theme.BadgeStyle.DotSize)
	}
}

func TestThemeMakerProgressBarStyle(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	if theme.ProgressBarStyle.Size != cfg.SizeProgressBar {
		t.Errorf("size = %f, want %f",
			theme.ProgressBarStyle.Size, cfg.SizeProgressBar)
	}
	if theme.ProgressBarStyle.ColorBar != cfg.ColorSelect {
		t.Error("bar color should match select")
	}
}

func TestWithColorsBadge(t *testing.T) {
	t.Parallel()
	theme := ThemeMaker(baseDarkCfg())
	sel := RGB(100, 200, 50)
	updated := theme.WithColors(ColorOverrides{
		ColorSelect: &sel,
	})
	if updated.BadgeStyle.ColorInfo != sel {
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
		{"B1", theme.B1}, {"B2", theme.B2}, {"B3", theme.B3},
		{"B4", theme.B4}, {"B5", theme.B5}, {"B6", theme.B6},
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
		{"N1", theme.N1}, {"N2", theme.N2}, {"N3", theme.N3},
		{"N4", theme.N4}, {"N5", theme.N5}, {"N6", theme.N6},
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
		ColorHover: &hover,
	})
	if updated.SliderStyle.ColorHover != hover {
		t.Error("slider hover not propagated")
	}
}

func TestThemeMakerTreeStyle(t *testing.T) {
	t.Parallel()
	cfg := baseDarkCfg()
	theme := ThemeMaker(cfg)
	if theme.TreeStyle.ColorHover != cfg.ColorHover {
		t.Errorf("TreeStyle.ColorHover = %v, want %v",
			theme.TreeStyle.ColorHover, cfg.ColorHover)
	}
	if theme.TreeStyle.Indent != 25 {
		t.Errorf("TreeStyle.Indent = %f, want 25",
			theme.TreeStyle.Indent)
	}
}
