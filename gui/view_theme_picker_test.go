package gui

import "testing"

func TestThemePickerClosed(t *testing.T) {
	w := &Window{}
	v := ThemePicker(ThemePickerCfg{ID: "tt1"})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "tt1" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
}

func TestThemePickerOpen(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "dark-test")
		delete(themeRegistry, "light-test")
		themeRegistryMu.Unlock()
	}()
	themeRegister(Theme{Name: "dark-test"})
	themeRegister(Theme{Name: "light-test"})

	w := &Window{}
	ss := StateMap[string, bool](w, nsSelect, capModerate)
	ss.Set("tt-open", true)

	v := ThemePicker(ThemePickerCfg{ID: "tt-open"})
	layout := generateViewLayout(v, w)
	// Should have icon + dropdown.
	if len(layout.Children) < 2 {
		t.Errorf("children = %d, want >= 2", len(layout.Children))
	}
}

func TestThemePickerSyncHighlight(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "alpha")
		delete(themeRegistry, "beta")
		themeRegistryMu.Unlock()
	}()
	themeRegister(Theme{Name: "alpha"})
	themeRegister(Theme{Name: "beta"})

	// The sync runs post-generation and reads the window's theme, so
	// pin the name on the window rather than on the frame cache. "dark"
	// is the first name in display order (toolkit presets first,
	// visual-refresh §7 ordering).
	w := &Window{}
	pinTheme(w, func(th *Theme) { th.Name = "dark" })
	themePickerSyncHighlight("test-lb", w)
	idx := StateReadOr(w, nsListBoxFocus, "test-lb", -1)
	if idx != 0 {
		t.Errorf("highlight = %d, want 0", idx)
	}
}

func TestThemePickerSetTheme(t *testing.T) {
	saved := guiTheme
	defer SetTheme(saved)

	w := &Window{}
	// Verify SetTheme on Window exists and doesn't panic.
	w.SetTheme(Theme{Name: "set-theme-test"})
	if guiTheme.Name != "set-theme-test" {
		t.Errorf("theme name = %q", guiTheme.Name)
	}
}
