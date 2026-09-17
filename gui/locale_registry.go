package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

var (
	localeRegistryMu sync.RWMutex
	localeRegistry   = map[string]Locale{}
)

func init() {
	LocaleRegister(LocaleEnUS)
	LocaleRegister(LocaleDeDE)
	LocaleRegister(LocaleArSA)
	LocaleRegister(LocaleFrFR)
	LocaleRegister(LocaleEsES)
	LocaleRegister(LocalePtBR)
	LocaleRegister(LocaleJaJP)
	LocaleRegister(LocaleZhCN)
	LocaleRegister(LocaleKoKR)
	LocaleRegister(LocaleHeIL)
}

// LocaleRegister adds a locale to the global registry by ID.
// Overwrites any existing entry with the same ID. The locale is
// cloned on the way in, so later edits by the caller do not
// mutate the registered copy.
func LocaleRegister(l Locale) {
	localeRegistryMu.Lock()
	localeRegistry[l.ID] = l.clone()
	localeRegistryMu.Unlock()
}

// LocaleGet retrieves a registered locale by ID. The result is a
// clone; mutating it does not affect the registry.
func LocaleGet(id string) (Locale, bool) {
	localeRegistryMu.RLock()
	l, ok := localeRegistry[id]
	localeRegistryMu.RUnlock()
	if !ok {
		return Locale{}, false
	}
	return l.clone(), true
}

// LocaleRegisteredNames returns sorted IDs of all registered
// locales.
// exportaudit:keep — documented public API (showcase docs)
func LocaleRegisteredNames() []string {
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
// registers each as a locale. Loading is two-phase: every file
// must parse before anything is registered, so a bad bundle
// leaves the registry untouched. A missing or unreadable
// directory is an error; an existing but empty one succeeds.
// exportaudit:keep — documented public API (showcase docs)
func LocaleLoadDir(dir string) error {
	// os.ReadDir, not filepath.Glob: the directory name is data, and
	// a name holding glob metacharacters ("loc[1]") must not become a
	// pattern that silently matches nothing. ReadDir also errors on a
	// missing path or a non-directory, and returns entries sorted by
	// name, so load order stays deterministic.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("LocaleLoadDir: %w", err)
	}
	loaded := make([]Locale, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		l, loadErr := LocaleLoad(path)
		if loadErr != nil {
			return fmt.Errorf("LocaleLoadDir: load %s: %w", path, loadErr)
		}
		loaded = append(loaded, l)
	}
	for _, l := range loaded {
		LocaleRegister(l)
	}
	return nil
}
