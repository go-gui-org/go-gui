package gui

import "testing"

func TestThemeRegisterAndGet(t *testing.T) {
	defer func() {
		themeRegistryMu.Lock()
		delete(themeRegistry, "test-reg")
		themeRegistryMu.Unlock()
	}()

	themeRegister(Theme{Name: "test-reg", ColorPanel: Red})
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

	themeRegister(Theme{Name: "zz-test"})
	themeRegister(Theme{Name: "aa-test"})
	names := themeRegisteredNames()
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
