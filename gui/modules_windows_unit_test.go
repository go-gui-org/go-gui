//go:build windows

package gui

import (
	"strings"
	"testing"
)

// The #886 diagnosis needs ntdll.dll in the list: every Windows process maps
// it, so a list without it means the enumeration is broken.
func TestLoadedModulesIncludesNtdll(t *testing.T) {
	mods, err := loadedModules()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mods {
		if strings.EqualFold(m.name, "ntdll.dll") {
			if m.base == 0 {
				t.Fatal("ntdll.dll has a zero base")
			}
			return
		}
	}
	t.Fatalf("ntdll.dll not in %d modules", len(mods))
}
