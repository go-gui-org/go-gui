package gui

import "testing"

func TestPresetThemesDefined(t *testing.T) {
	themes := []struct {
		name  string
		theme Theme
	}{
		{"dark", ThemeDark},
		{"dark-no-padding", themeDarkNoPadding},
		{"dark-bordered", themeDarkBordered},
		{"light", ThemeLight},
		{"light-no-padding", themeLightNoPadding},
		{"light-bordered", themeLightBordered},
		{"blue-dark", themeBlue},
		{"blue-dark-bordered", themeBlueBordered},
	}
	for _, tt := range themes {
		if tt.theme.Name == "" {
			t.Errorf("%s: empty name", tt.name)
		}
		if tt.theme.ColorBackground.eq(Color{}) {
			t.Errorf("%s: zero background", tt.name)
		}
	}
}

func TestDarkThemeColors(t *testing.T) {
	if ThemeDark.ColorBackground != colorBackgroundDark {
		t.Error("dark background mismatch")
	}
	if ThemeDark.ColorSelect != colorSelectDark {
		t.Error("dark select mismatch")
	}
}

// ThemeDark carries the border since 2026-08 (issue #325 call-site
// count: 90 of 104 examples opt in via WithBorders(true)); the flip
// must keep WithBorders(false) able to restore the borderless look.
func TestDarkThemeBorderedDefault(t *testing.T) {
	if ThemeDark.Cfg.SizeBorder != sizeBorderDef {
		t.Errorf("ThemeDark SizeBorder = %v, want %v",
			ThemeDark.Cfg.SizeBorder, sizeBorderDef)
	}
	if got := ThemeDark.WithBorders(false).Cfg.SizeBorder; got != 0 {
		t.Errorf("WithBorders(false) SizeBorder = %v, want 0", got)
	}
	// dark-bordered is now an exact duplicate of dark; keep them in
	// lockstep so name-based selection cannot drift from ThemeDark.
	if themeDarkBordered.Cfg.SizeBorder != ThemeDark.Cfg.SizeBorder {
		t.Error("dark-bordered preset diverged from ThemeDark")
	}
}

func TestLightThemeColors(t *testing.T) {
	if ThemeLight.ColorBackground != colorBackgroundLight {
		t.Error("light background mismatch")
	}
	if ThemeLight.TextStyleDef.Color != colorTextLight {
		t.Error("light text color mismatch")
	}
}

func TestPresetThemesRegistered(t *testing.T) {
	names := themeRegisteredNames()
	if len(names) < 8 {
		t.Errorf("registered themes = %d, want >= 8",
			len(names))
	}
	expected := []string{
		"dark", "dark-no-padding", "dark-bordered",
		"light", "light-no-padding", "light-bordered",
		"blue-dark", "blue-dark-bordered",
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, e := range expected {
		if !nameSet[e] {
			t.Errorf("missing registered theme %q", e)
		}
	}
}
