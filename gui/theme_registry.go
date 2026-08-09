package gui

import (
	"slices"
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

// ThemeRegisteredNames returns sorted names of all registered
// themes.
func themeRegisteredNames() []string {
	themeRegistryMu.RLock()
	names := make([]string, 0, len(themeRegistry))
	for k := range themeRegistry {
		names = append(names, k)
	}
	themeRegistryMu.RUnlock()
	slices.Sort(names)
	return names
}
