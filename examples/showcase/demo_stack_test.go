package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// stackPageWindow renders the showcase at the Stack page.
func stackPageWindow(t *testing.T) *gui.Window {
	t.Helper()
	app := newShowcaseApp()
	app.SelectedGroup = groupLayout
	app.SelectedComponent = "stack"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 1200})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

func TestStackPageNoFindings(t *testing.T) {
	w := stackPageWindow(t)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// Green covers the center of Red. A click there goes to Green, the upper
// layer, so Green comes to the front, not Red.
func TestStackPageClickGoesToUpperCard(t *testing.T) {
	w := stackPageWindow(t)
	id := gui.ScopeID("detail", gui.ScopeID("stack-cards", "red"))
	if err := w.TestClick(id); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if got, want := appState(w).StackOrder, [3]int{0, 2, 1}; got != want {
		t.Fatalf("order = %v, want %v (green on top)", got, want)
	}
}

func TestStackDemoRaise(t *testing.T) {
	app := newShowcaseApp()
	stackDemoRaise(app, 0)
	if want := [3]int{1, 2, 0}; app.StackOrder != want {
		t.Fatalf("order = %v, want %v", app.StackOrder, want)
	}
}

// A card index not in the order leaves the order unchanged.
func TestStackDemoRaiseUnknownCard(t *testing.T) {
	app := newShowcaseApp()
	stackDemoRaise(app, 7)
	if want := [3]int{0, 1, 2}; app.StackOrder != want {
		t.Fatalf("order = %v, want %v", app.StackOrder, want)
	}
}
