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
	ThemeRegister(Theme{Name: "dark-test"})
	ThemeRegister(Theme{Name: "light-test"})

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
	savedName := guiTheme.Name
	defer func() {
		guiTheme.Name = savedName
		themeRegistryMu.Lock()
		delete(themeRegistry, "alpha")
		delete(themeRegistry, "beta")
		themeRegistryMu.Unlock()
	}()
	ThemeRegister(Theme{Name: "alpha"})
	ThemeRegister(Theme{Name: "beta"})
	guiTheme.Name = "beta"

	w := &Window{}
	themePickerSyncHighlight("test-lb", w)
	idx := StateReadOr(w, nsListBoxFocus, "test-lb", -1)
	// "alpha"=0, "beta"=1 (sorted).
	if idx != 1 {
		t.Errorf("highlight = %d, want 1", idx)
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
