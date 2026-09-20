package gui

import "testing"

func TestThemeRegisterAndGet(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "test-reg")
		themeRegistryMu.Unlock()
	}()

	if !ThemeRegister(Theme{Name: "test-reg", ColorPanel: Red}) {
		t.Fatal("expected registration to succeed")
	}
	got, ok := ThemeGet("test-reg")
	if !ok {
		t.Fatal("expected theme to be found")
	}
	if got.ColorPanel != Red {
		t.Error("wrong panel color")
	}
}

func TestThemeGetNotFound(t *testing.T) {
	_, ok := ThemeGet("nonexistent-theme-xyz")
	if ok {
		t.Error("expected not found")
	}
}

func TestThemeRegisteredNames(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "zz-test")
		delete(themeRegistry, "aa-test")
		themeRegistryMu.Unlock()
	}()

	ThemeRegister(Theme{Name: "zz-test"})
	ThemeRegister(Theme{Name: "aa-test"})
	names := ThemeRegisteredNames()
	// Should be sorted and contain our entries.
	found := 0
	for i, n := range names {
		if n == "aa-test" {
			found++
			// Verify aa comes before zz.
			for j := i + 1; j < len(names); j++ {
				if names[j] == "zz-test" {
					found++
				}
			}
		}
	}
	if found < 2 {
		t.Errorf("expected sorted names containing both entries, got %v", names)
	}
}

// Empty names register nothing: the picker would show a blank
// entry and ThemeGet("") would answer junk (issue #713).
func TestThemeRegisterRejectsEmptyName(t *testing.T) {
	if ThemeRegister(Theme{Name: ""}) {
		t.Error("expected empty name registration to fail")
	}
	if _, ok := ThemeGet(""); ok {
		t.Error("expected empty name to be absent from registry")
	}
	for _, n := range ThemeRegisteredNames() {
		if n == "" {
			t.Error("expected empty name to be absent from names")
		}
	}
}

// Whitespace-only names are empty names with padding: same
// rejection, or the picker lists a blank row (issue #713).
func TestThemeRegisterRejectsWhitespaceName(t *testing.T) {
	if ThemeRegister(Theme{Name: "   "}) {
		t.Error("expected whitespace name registration to fail")
	}
	if _, ok := ThemeGet("   "); ok {
		t.Error("expected whitespace name to be absent from registry")
	}
}

// Re-registering a name replaces the entry, matching the
// long-standing overwrite behavior of the presets.
func TestThemeRegisterOverwrite(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "test-overwrite")
		themeRegistryMu.Unlock()
	}()

	ThemeRegister(Theme{Name: "test-overwrite", ColorPanel: Red})
	if !ThemeRegister(Theme{Name: "test-overwrite", ColorPanel: Blue}) {
		t.Fatal("expected overwrite registration to succeed")
	}
	got, ok := ThemeGet("test-overwrite")
	if !ok {
		t.Fatal("expected overwritten theme to be found")
	}
	if got.ColorPanel != Blue {
		t.Error("expected second registration to win")
	}
}

// A name with padding around real text is not empty, so it
// registers — stored verbatim, never trimmed onto another key.
// Trimming on store would let " dark " silently overwrite the
// dark preset; the verbatim store keeps the two keys apart.
func TestThemeRegisterPaddedNameStoredVerbatim(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "  test-padded  ")
		themeRegistryMu.Unlock()
	}()

	if !ThemeRegister(Theme{Name: "  test-padded  "}) {
		t.Fatal("expected padded name registration to succeed")
	}
	if _, ok := ThemeGet("  test-padded  "); !ok {
		t.Error("expected padded name to be retrievable verbatim")
	}
	if _, ok := ThemeGet("test-padded"); ok {
		t.Error("expected padded name to stay apart from trimmed key")
	}
}

// Display order keeps the toolkit presets first (dark, then
// light) ahead of every other name.
func TestThemeRegisteredNamesOrdering(t *testing.T) {
	names := ThemeRegisteredNames()
	if len(names) < 2 {
		t.Fatalf("registered themes = %d, want >= 2", len(names))
	}
	if names[0] != "dark" || names[1] != "light" {
		t.Errorf("first names = %q, want dark then light", names[:2])
	}
}
