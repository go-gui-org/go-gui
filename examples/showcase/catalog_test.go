package main

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// catalogRowIDs returns the effective IDs of the catalog rows the last
// frame built.
func catalogRowIDs(w *gui.Window) []string {
	var ids []string
	prefix := gui.ScopeID("showcase", "cat", "")
	for _, id := range w.EffectiveIDs() {
		if strings.HasPrefix(id, prefix) {
			ids = append(ids, id)
		}
	}
	return ids
}

// The catalog lists every demo, but a frame builds only the rows that
// fit in the viewport. Building all of them every frame was most of
// the showcase's per-frame allocation.
func TestCatalogBuildsOnlyVisibleRows(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupAll
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 700})
	w.TestRender(mainView)
	w.TestRender(nil)

	got := len(catalogRowIDs(w))
	total := len(filteredEntries(app))
	if got == 0 || got >= total {
		t.Fatalf("built %d catalog rows of %d, want some but not all", got, total)
	}
}

// A click on a catalog row still selects its demo.
func TestCatalogRowClickSelects(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupAll
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 700})
	w.TestRender(mainView)
	w.TestRender(nil)

	ids := catalogRowIDs(w)
	if len(ids) < 2 {
		t.Fatalf("built %d catalog rows, want at least 2", len(ids))
	}
	target := ids[1]
	if err := w.TestClick(target); err != nil {
		t.Fatal(err)
	}
	want := target[strings.LastIndex(target, ":")+1:]
	if app.SelectedComponent != want {
		t.Fatalf("SelectedComponent = %q, want %q", app.SelectedComponent, want)
	}
}
