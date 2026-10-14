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

// The sidebar pins Start first and renders the rest in
// alphabetical order by label, regardless of demoGroups order.
func TestCatalogGroupsAlphabetical(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupAll
	items := catalogItems(filteredEntries(app))
	var got []string
	for _, it := range items {
		if it.entry == nil {
			got = append(got, it.group.Label)
		}
	}
	if len(got) == 0 {
		t.Fatal("catalog has no group headers")
	}
	if got[0] != "Start" {
		t.Fatalf("first group = %q, want %q (full order %q)", got[0], "Start", got)
	}
	for i := 2; i < len(got); i++ {
		if strings.Compare(got[i-1], got[i]) >= 0 {
			t.Fatalf("group headers after Start not alphabetical: %q", got)
		}
	}
}

// Every entry names a group that exists. An entry with an unknown
// group is in no catalog section, so it shows only under "All".
func TestDemoEntryGroupsAreKnown(t *testing.T) {
	keys := make(map[string]bool, len(demoGroups))
	for _, g := range demoGroups {
		if keys[g.Key] {
			t.Fatalf("group key %q is in demoGroups twice", g.Key)
		}
		keys[g.Key] = true
	}
	for _, e := range demoEntries {
		if !keys[e.Group] {
			t.Errorf("entry %q has group %q, which is not in demoGroups", e.ID, e.Group)
		}
	}
}

// hasShapeID reports whether a layout tree holds a shape whose ID is id
// or ends in id as its last scope part. Markdown stores its effective ID.
func hasShapeID(l gui.Layout, id string) bool {
	if l.Shape != nil && (l.Shape.ID == id || strings.HasSuffix(l.Shape.ID, ":"+id)) {
		return true
	}
	for i := range l.Children {
		if hasShapeID(l.Children[i], id) {
			return true
		}
	}
	return false
}

// A widget page shows its docs under the demo, with no toggle to find.
// A doc page (Welcome, Sound Feedback, ...) is its own doc, so it gets
// no second copy. This keys on the page, not its catalog group: Sound
// Feedback left the Welcome group in #1025.
func TestDocsAppendOnlyToWidgetPages(t *testing.T) {
	appendsDocs := func(id string) bool {
		app := newShowcaseApp()
		app.SelectedComponent = id
		w := gui.NewWindow(gui.WindowCfg{State: app})
		return hasShapeID(gui.GenerateViewLayout(detailPanel(w), w), docsSectionID)
	}
	// Control: a widget page does append its doc, so the probe works.
	if !appendsDocs("button") {
		t.Fatal("button page does not append its docs")
	}
	for id := range docPageFiles {
		if appendsDocs(id) {
			t.Errorf("doc page %q appends a widget-docs section", id)
		}
	}
}

// The docs link scrolls so the section's top meets the viewport's
// content top. The offset is negative, as ScrollVerticalTo takes it.
func TestDocsScrollOffset(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		viewTop, docsY, current float32
		want                    float32
	}{
		{"from top", 100, 700, 0, -600},
		{"already scrolled", 100, 300, -400, -600},
		{"already there", 100, 100, -600, -600},
	} {
		if got := docsScrollOffset(tc.viewTop, tc.docsY, tc.current); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
