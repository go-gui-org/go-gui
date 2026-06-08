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
		{"blue-dark", ThemeBlue},
		{"blue-dark-bordered", themeBlueBordered},
	}
	for _, tt := range themes {
		if tt.theme.Name == "" {
			t.Errorf("%s: empty name", tt.name)
		}
		if tt.theme.ColorBackground.Eq(Color{}) {
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

func TestLightThemeColors(t *testing.T) {
	if ThemeLight.ColorBackground != colorBackgroundLight {
		t.Error("light background mismatch")
	}
	if ThemeLight.TextStyleDef.Color != colorTextLight {
		t.Error("light text color mismatch")
	}
}

func TestPresetThemesRegistered(t *testing.T) {
	names := ThemeRegisteredNames()
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
