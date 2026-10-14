package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newWindow(t *testing.T, a *app) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{
		State:  a,
		Width:  720,
		Height: 280,
	})
	w.TestRender(mainView)
	return w
}

func newApp() *app { return &app{flipped: make([]bool, len(cards))} }

func TestMainViewNoFindings(t *testing.T) {
	w := newWindow(t, newApp())
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// A click turns one card over and leaves the others alone; a second
// click turns it back.
func TestClickFlipsOneCard(t *testing.T) {
	a := newApp()
	w := newWindow(t, a)
	if err := w.TestClick("planet"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if !a.flipped[1] || a.flipped[0] || a.flipped[2] {
		t.Fatalf("flipped = %v, want only card 1", a.flipped)
	}
	if err := w.TestClick("planet"); err != nil {
		t.Fatalf("second TestClick: %v", err)
	}
	if a.flipped[1] {
		t.Fatal("second click did not turn the card back")
	}
}

// Space turns the focused card over.
func TestSpaceFlipsFocusedCard(t *testing.T) {
	a := newApp()
	w := newWindow(t, a)
	if err := w.TestKey("boil", gui.KeySpace, 0); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if !a.flipped[2] {
		t.Fatal("Space did not turn the card over")
	}
}

// A card first seen flipped rests on its back face, and a click there
// turns it back to the question.
func TestClickOnBackTurnsCardBack(t *testing.T) {
	a := newApp()
	a.flipped[0] = true
	w := newWindow(t, a)
	if err := w.TestClick("capital"); err != nil {
		t.Fatalf("TestClick on the flipped card: %v", err)
	}
	if a.flipped[0] {
		t.Fatal("a click on the back face did not turn the card back")
	}
}
