package gui

import "testing"

func TestPresetThemesDefined(t *testing.T) {
	themes := []struct {
		name  string
		theme Theme
	}{
		{"dark", ThemeDark},
		{"light", ThemeLight},
		{"macos", themeMacOS},
		{"macos-dark", themeMacOSDark},
		{"gnome", themeGnome},
		{"gnome-dark", themeGnomeDark},
		{"windows", themeWindows},
		{"windows-dark", themeWindowsDark},
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

// The preset palette pins (visual-refresh §4): the dark ramp, and
// ColorSelect resolving to the accent now that baseDarkCfg states
// only ColorAccent.
func TestDarkThemeColors(t *testing.T) {
	if ThemeDark.ColorBackground != colorBackgroundDark {
		t.Error("dark background mismatch")
	}
	if ThemeDark.ColorSelect != colorAccentDark {
		t.Errorf("dark select = %v, want accent %v",
			ThemeDark.ColorSelect, colorAccentDark)
	}
	if ThemeDark.ColorAccent != colorAccentDark {
		t.Error("dark accent mismatch")
	}
	if ThemeDark.ColorBorder != colorBorderDark {
		t.Error("dark border mismatch")
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
}

func TestLightThemeColors(t *testing.T) {
	if ThemeLight.ColorBackground != colorBackgroundLight {
		t.Error("light background mismatch")
	}
	if ThemeLight.TextStyleDef.Color != colorTextLight {
		t.Error("light text color mismatch")
	}
	if ThemeLight.ColorSelect != colorAccentLight {
		t.Errorf("light select = %v, want accent %v",
			ThemeLight.ColorSelect, colorAccentLight)
	}
	if ThemeLight.ButtonStyle.ColorBorderFocus != colorAccentLight {
		t.Errorf("light focus border = %v, want accent %v",
			ThemeLight.ButtonStyle.ColorBorderFocus, colorAccentLight)
	}
}

func TestPresetThemesRegistered(t *testing.T) {
	names := themeRegisteredNames()
	if len(names) < 8 {
		t.Errorf("registered themes = %d, want >= 8",
			len(names))
	}
	expected := []string{
		"dark", "light",
		"macos", "macos-dark",
		"gnome", "gnome-dark",
		"windows", "windows-dark",
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
