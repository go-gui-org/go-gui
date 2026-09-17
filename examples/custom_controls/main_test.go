package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/examples/custom_controls/internal/look"
	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp renders the example once on the given tab.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T, tab string) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	app := newApp()
	app.tab = tab
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 960, Height: 780})
	w.TestRender(mainView)
	return app, w
}

// Every tab has a label, and every page renders with clean IDs.
func TestEveryTabRendersClean(t *testing.T) {
	for _, id := range tabIDs {
		t.Run(id, func(t *testing.T) {
			if tabLabels[id] == "" {
				t.Fatalf("tab %q has no label", id)
			}
			_, w := newTestApp(t, id)
			ly := w.TestRender(nil)
			if len(ly.Children) == 0 {
				t.Fatal("empty window")
			}
			if d := w.TestDuplicateIDs(); len(d) != 0 {
				t.Fatalf("duplicate IDs: %v", d)
			}
			if found := w.TestFindings(gui.DebugMissingIDs | gui.DebugDuplicates); len(found) != 0 {
				t.Fatalf("findings: %v", found)
			}
		})
	}
}

// A click on a tab selects it and shows its page.
func TestClickTabSwitchesPage(t *testing.T) {
	app, w := newTestApp(t, "buttons")
	if err := w.TestClick(gui.ScopeID("tabs", "tab", "sliders")); err != nil {
		t.Fatal(err)
	}
	if app.tab != "sliders" {
		t.Fatalf("tab %q after click, want sliders", app.tab)
	}
	w.TestRender(nil)
	if ids := w.ResolveID("apple"); len(ids) == 0 {
		t.Fatalf("sliders page not shown; frame IDs %v", w.EffectiveIDs())
	}
}

// A page paints its own light background, so its text must stay dark under a
// dark app theme. Before, the title took the app theme's text color: light
// text on a light page.
func TestPageTextStaysDarkUnderDarkTheme(t *testing.T) {
	want := look.Light.Title.Color
	if want == gui.ThemeDark.B2.Color {
		t.Fatal("light and dark title colors are equal; the test proves nothing")
	}
	for _, id := range tabIDs {
		t.Run(id, func(t *testing.T) {
			_, w := newTestApp(t, id)
			gui.SetTheme(gui.ThemeDark)
			t.Cleanup(func() { gui.SetTheme(gui.ThemeLight) })
			ly := w.TestRender(nil)
			ids := w.ResolveID("page")
			if len(ids) != 1 {
				t.Fatalf("page IDs %v, want one", ids)
			}
			page, ok := ly.FindByID(ids[0])
			if !ok || len(page.Children) == 0 {
				t.Fatalf("no page content at %q", ids[0])
			}
			title := page.Children[0].Shape
			if title.TC == nil || title.TC.TextStyle == nil {
				t.Fatal("first page child is not text")
			}
			if got := title.TC.TextStyle.Color; got != want {
				t.Fatalf("title color %+v under ThemeDark, want light-theme %+v", got, want)
			}
		})
	}
}
