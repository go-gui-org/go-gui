package gui

import (
	"slices"
	"strings"
	"sync"
)

var (
	themeRegistryMu sync.RWMutex
	themeRegistry   = map[string]Theme{}
)

// ThemeRegister adds a theme to the global registry by name.
// Overwrites any existing entry with the same name.
func themeRegister(t Theme) {
	themeRegistryMu.Lock()
	themeRegistry[t.Name] = t
	themeRegistryMu.Unlock()
}

// ThemeGet retrieves a registered theme by name.
func ThemeGet(name string) (Theme, bool) {
	themeRegistryMu.RLock()
	t, ok := themeRegistry[name]
	themeRegistryMu.RUnlock()
	return t, ok
}

// ThemeRegisteredNames returns the names of all registered themes in
// display order: the toolkit's own presets first (dark, then light),
// then the platform pairs alphabetically. ThemePicker lists them in
// this order, so the picker reads dark, light, macos, macos-dark,
// gnome, gnome-dark, windows, windows-dark rather than splitting the
// two toolkit themes apart alphabetically.
func themeRegisteredNames() []string {
	themeRegistryMu.RLock()
	names := make([]string, 0, len(themeRegistry))
	for k := range themeRegistry {
		names = append(names, k)
	}
	themeRegistryMu.RUnlock()
	slices.SortFunc(names, func(a, b string) int {
		if ra, rb := themeSortRank(a), themeSortRank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a, b)
	})
	return names
}

// themeSortRank groups the two toolkit presets ahead of everything
// else; any other name (platform pairs, user registrations) sorts
// alphabetically within its group.
func themeSortRank(name string) int {
	switch name {
	case "dark":
		return 0
	case "light":
		return 1
	}
	return 2
}
