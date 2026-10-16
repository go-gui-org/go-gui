package main

import (
	"slices"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	navSearchID      = gui.ScopeID(catalogScopeID, "showcase-nav-search")
	navToggleScopeID = gui.ScopeID(catalogScopeID, navToggleID)
)

// newNavTestWindow renders the showcase in a window of the given width.
func newNavTestWindow(t *testing.T, width int) (*gui.Window, *ShowcaseApp) {
	t.Helper()
	app := newShowcaseApp()
	app.SelectedGroup = groupAll
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: width, Height: 800})
	if err := w.RegisterCommand(closeCatalogCommand()); err != nil {
		t.Fatal(err)
	}
	w.TestRender(mainView)
	w.TestRender(nil)
	return w, app
}

func hasEffectiveID(w *gui.Window, id string) bool {
	return slices.Contains(w.EffectiveIDs(), id)
}

// A wide window keeps the catalog inline, with no top bar.
func TestNavWideShowsCatalogInline(t *testing.T) {
	w, _ := newNavTestWindow(t, 950)
	if !hasEffectiveID(w, navSearchID) {
		t.Fatalf("wide window: no %s, want the catalog inline", navSearchID)
	}
	if hasEffectiveID(w, navOpenID) {
		t.Fatalf("wide window: has %s, want no top bar", navOpenID)
	}
}

// Collapsing the catalog on a wide window brings up the top bar, whose
// button opens it again.
func TestNavWideCollapseAndRestore(t *testing.T) {
	w, app := newNavTestWindow(t, 950)
	if err := w.TestClick(navToggleScopeID); err != nil {
		t.Fatal(err)
	}
	if !app.CatalogCollapsed {
		t.Fatal("CatalogCollapsed = false after the toggle, want true")
	}
	if !hasEffectiveID(w, navOpenID) {
		t.Fatalf("collapsed: no %s, want the top bar", navOpenID)
	}
	if got := w.FocusID(); got != navOpenID {
		t.Fatalf("collapsed: focus = %q, want %q", got, navOpenID)
	}

	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	if app.CatalogCollapsed {
		t.Fatal("CatalogCollapsed = true after reopening, want false")
	}
	if hasEffectiveID(w, navOpenID) {
		t.Fatalf("restored: has %s, want no top bar", navOpenID)
	}
}

// A phone-width window gives the page the full width: the catalog is
// not in the tree until the menu button opens the drawer.
func TestNavNarrowHidesCatalogUntilOpened(t *testing.T) {
	w, app := newNavTestWindow(t, 390)
	if hasEffectiveID(w, navSearchID) {
		t.Fatalf("narrow window: has %s, want the catalog hidden", navSearchID)
	}

	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	if !app.CatalogDrawerOpen {
		t.Fatal("CatalogDrawerOpen = false after the menu button, want true")
	}
	if !hasEffectiveID(w, navSearchID) {
		t.Fatalf("drawer open: no %s, want the catalog", navSearchID)
	}
	if got := w.FocusID(); got != navToggleScopeID {
		t.Fatalf("drawer open: focus = %q, want %q", got, navToggleScopeID)
	}
}

// Picking a page in the drawer shows it: the drawer closes and focus
// goes back to the button that opened it.
func TestNavNarrowPickClosesDrawer(t *testing.T) {
	w, app := newNavTestWindow(t, 390)
	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	target := gui.ScopeID("showcase", "cat", "button")
	if err := w.TestClick(target); err != nil {
		t.Fatal(err)
	}
	if app.SelectedComponent != "button" {
		t.Fatalf("SelectedComponent = %q, want %q", app.SelectedComponent, "button")
	}
	if app.CatalogDrawerOpen {
		t.Fatal("CatalogDrawerOpen = true after a pick, want false")
	}
	if got := w.FocusID(); got != navOpenID {
		t.Fatalf("after a pick: focus = %q, want %q", got, navOpenID)
	}
}

// Escape closes the drawer, even from the search field.
func TestNavNarrowEscapeClosesDrawer(t *testing.T) {
	w, app := newNavTestWindow(t, 390)
	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	if err := w.TestKey(navSearchID, gui.KeyEscape, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if app.CatalogDrawerOpen {
		t.Fatal("CatalogDrawerOpen = true after Escape, want false")
	}
}

// The menu button in the drawer closes it again.
func TestNavNarrowToggleClosesDrawer(t *testing.T) {
	w, app := newNavTestWindow(t, 390)
	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	if err := w.TestClick(navToggleScopeID); err != nil {
		t.Fatal(err)
	}
	if app.CatalogDrawerOpen {
		t.Fatal("CatalogDrawerOpen = true after the drawer's toggle, want false")
	}
	if hasEffectiveID(w, navSearchID) {
		t.Fatalf("drawer closed: has %s, want the catalog hidden", navSearchID)
	}
}

// resizeNavTestWindow resizes the window the way the backend does and
// renders the next frame.
func resizeNavTestWindow(w *gui.Window, width int) {
	w.EventFn(&gui.Event{Type: gui.EventResized, WindowWidth: width, WindowHeight: 800})
	w.TestRender(nil)
}

// A drawer left open when the window grows wide must not stay open in
// state. Before the fix, the stale flag sent focus to the closing
// sidebar on collapse and reopened the drawer on the next shrink.
func TestNavDrawerClosesWhenWindowGrowsWide(t *testing.T) {
	w, app := newNavTestWindow(t, 390)
	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	resizeNavTestWindow(w, 950)
	if app.CatalogDrawerOpen {
		t.Fatal("CatalogDrawerOpen = true after the window grew wide, want false")
	}

	// Collapse the inline catalog: focus goes to the top bar's button.
	if err := w.TestClick(navToggleScopeID); err != nil {
		t.Fatal(err)
	}
	if got := w.FocusID(); got != navOpenID {
		t.Fatalf("collapsed: focus = %q, want %q", got, navOpenID)
	}

	// Shrink again: the drawer stays shut until asked for.
	resizeNavTestWindow(w, 390)
	if hasEffectiveID(w, navSearchID) {
		t.Fatalf("narrow again: has %s, want the drawer shut", navSearchID)
	}
}

// The drawer leaves the reader a strip of page when the window has room
// for one, and never resolves to a zero or negative width. A Fixed width
// of 0 falls back to content sizing (#94).
func TestNavDrawerWidth(t *testing.T) {
	tests := []struct {
		window, want float32
	}{
		{950, catalogWidth},          // wide enough: the catalog's own width
		{390, catalogWidth},          // phone: still room for the strip
		{300, 300 - navDrawerGap},    // narrower than catalog + strip
		{100, 100},                   // a strip would leave a sliver: full width
		{navDrawerGap, navDrawerGap}, // the old formula gave 0 here
		{20, 20},                     // the old formula went negative
		{0, 1},                       // a minimized window still gets a width
	}
	for _, tt := range tests {
		if got := drawerWidth(tt.window); got != tt.want {
			t.Errorf("drawerWidth(%v) = %v, want %v", tt.window, got, tt.want)
		}
	}
}

// The drawer is modal: Tab cycles inside it and never reaches the top
// bar or the page behind the backdrop.
func TestNavDrawerKeepsTabInside(t *testing.T) {
	w, _ := newNavTestWindow(t, 390)
	// With the drawer shut, every ID on screen belongs to the page.
	page := make(map[string]bool)
	for _, id := range w.EffectiveIDs() {
		page[id] = true
	}
	if err := w.TestClick(navOpenID); err != nil {
		t.Fatal(err)
	}
	// More presses than the catalog has stops, so the cycle wraps.
	for i := range 2 * len(w.EffectiveIDs()) {
		if err := w.TestKey(w.FocusID(), gui.KeyTab, gui.ModNone); err != nil {
			t.Fatal(err)
		}
		if got := w.FocusID(); got == "" || page[got] {
			t.Fatalf("Tab %d: focus = %q, want a widget in the drawer", i+1, got)
		}
	}
}
