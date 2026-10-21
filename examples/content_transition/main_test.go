package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newWindow(t *testing.T, a *app) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{
		State:  a,
		Width:  520,
		Height: 300,
	})
	w.TestRender(mainView)
	return w
}

func TestMainViewNoFindings(t *testing.T) {
	w := newWindow(t, &app{count: 98})
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// The buttons change the counter, which is what starts each roll.
func TestButtonsChangeCount(t *testing.T) {
	a := &app{count: 98}
	w := newWindow(t, a)
	if err := w.TestClick("inc"); err != nil {
		t.Fatalf("TestClick inc: %v", err)
	}
	if err := w.TestClick("inc"); err != nil {
		t.Fatalf("TestClick inc: %v", err)
	}
	if a.count != 100 {
		t.Fatalf("count = %d, want 100", a.count)
	}
	if err := w.TestClick("dec"); err != nil {
		t.Fatalf("TestClick dec: %v", err)
	}
	if a.count != 99 {
		t.Fatalf("count = %d, want 99", a.count)
	}
}

func TestLikeToggles(t *testing.T) {
	a := &app{}
	w := newWindow(t, a)
	if err := w.TestClick("like"); err != nil {
		t.Fatalf("TestClick like: %v", err)
	}
	if !a.liked {
		t.Fatal("like did not toggle")
	}
}

func TestNextCyclesStatus(t *testing.T) {
	a := &app{}
	w := newWindow(t, a)
	for i := range len(statuses) {
		if err := w.TestClick("next"); err != nil {
			t.Fatalf("TestClick next: %v", err)
		}
		if want := (i + 1) % len(statuses); a.status != want {
			t.Fatalf("status = %d, want %d", a.status, want)
		}
	}
}
