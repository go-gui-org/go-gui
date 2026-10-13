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

// firstText returns the text of the first text shape in a layout tree.
func firstText(l gui.Layout) string {
	if l.Shape != nil && l.Shape.TC != nil {
		return l.Shape.TC.Text
	}
	for i := range l.Children {
		if s := firstText(l.Children[i]); s != "" {
			return s
		}
	}
	return ""
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

// The group picker and the catalog headers come from one list
// (#1025): "All" first, then demoGroups with the same keys, order and
// labels.
func TestGroupPickerMatchesDemoGroups(t *testing.T) {
	app := newShowcaseApp()
	w := gui.NewWindow(gui.WindowCfg{State: app})
	layout := groupPicker(app).GenerateLayout(w)

	type item struct{ id, label string }
	want := []item{{gui.ScopeID("showcase", "grp", groupAll), "All"}}
	for _, g := range demoGroups {
		want = append(want, item{gui.ScopeID("showcase", "grp", g.Key), g.Label})
	}
	if len(layout.Children) != len(want) {
		t.Fatalf("picker has %d items, want %d", len(layout.Children), len(want))
	}
	for i, c := range layout.Children {
		got := item{c.Shape.ID, firstText(c)}
		if got != want[i] {
			t.Errorf("picker item %d = %+v, want %+v", i, got, want[i])
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

// A doc page (Welcome, Sound Feedback, ...) is its own doc, so the
// Docs toggle must not swap it for an empty widget doc. This used to
// key on the Welcome group; Sound Feedback left that group in #1025.
func TestDocPagesIgnoreDocsToggle(t *testing.T) {
	showsDocsView := func(id string) bool {
		app := newShowcaseApp()
		app.SelectedComponent = id
		app.ShowDocs = true
		w := gui.NewWindow(gui.WindowCfg{State: app})
		return hasShapeID(gui.GenerateViewLayout(detailPanel(w), w), "doc-"+id)
	}
	// Control: a widget does switch to its doc, so the probe works.
	if !showsDocsView("button") {
		t.Fatal("button does not show the widget-doc view with Docs on")
	}
	for id := range docPageFiles {
		if showsDocsView(id) {
			t.Errorf("doc page %q shows the widget-doc view with Docs on", id)
		}
	}
}
