package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State: &AppState{
			SidebarWidth: 200,
			BoxX:         50,
			SpringValue:  100,
		},
		Width:  800,
		Height: 600,
	})
	_ = mainView(w).GenerateLayout(w)

}

func TestMainViewNoDuplicateIDs(t *testing.T) {
	// Duplicate IDs are not cosmetic here: the layout transition
	// snapshots by effective ID, so two shapes sharing one make each
	// other's geometry the start of the lerp.
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State: &AppState{
			SidebarWidth: 200,
			BoxX:         50,
			SpringValue:  100,
		},
		Width:  800,
		Height: 600,
	})
	w.UpdateView(mainView)
	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Errorf("duplicate IDs: %v", dups)
	}
}
