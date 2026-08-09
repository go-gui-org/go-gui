package gui

import (
	"fmt"
	"path/filepath"
	"slices"
	"sync"
)

var (
	localeRegistryMu sync.RWMutex
	localeRegistry   = map[string]Locale{}
)

func init() {
	LocaleRegister(localeEnUS)
	LocaleRegister(localeDeDE)
	LocaleRegister(localeArSA)
	LocaleRegister(localeFrFR)
	LocaleRegister(localeEsES)
	LocaleRegister(localePtBR)
	LocaleRegister(localeJaJP)
	LocaleRegister(localeZhCN)
	LocaleRegister(localeKoKR)
	LocaleRegister(localeHeIL)
}

// LocaleRegister adds a locale to the global registry by ID.
// Overwrites any existing entry with the same ID.
func LocaleRegister(l Locale) {
	localeRegistryMu.Lock()
	localeRegistry[l.ID] = l
	localeRegistryMu.Unlock()
}

// LocaleGet retrieves a registered locale by ID.
func LocaleGet(id string) (Locale, bool) {
	localeRegistryMu.RLock()
	l, ok := localeRegistry[id]
	localeRegistryMu.RUnlock()
	return l, ok
}

// LocaleRegisteredNames returns sorted IDs of all registered
// locales.
func localeRegisteredNames() []string {
	localeRegistryMu.RLock()
	names := make([]string, 0, len(localeRegistry))
	for k := range localeRegistry {
		names = append(names, k)
	}
	localeRegistryMu.RUnlock()
	slices.Sort(names)
	return names
}

// LocaleLoadDir loads all *.json files from a directory and
// registers each as a locale.
// exportaudit:keep — documented public API (showcase docs)
func LocaleLoadDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return fmt.Errorf("LocaleLoadDir: glob: %w", err)
	}
	for _, f := range files {
		l, err := LocaleLoad(f)
		if err != nil {
			return fmt.Errorf("LocaleLoadDir: load %s: %w", filepath.Base(f), err)
		}
		LocaleRegister(l)
	}
	return nil
}
