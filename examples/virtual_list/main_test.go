package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	msgs := make([]message, 200)
	for i := range msgs {
		msgs[i] = newMessage(i)
	}
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Messages: msgs},
		Width:  520,
		Height: 620,
	})
	_ = mainView(w).GenerateLayout(w)
}

// TestJumpToIndexIsAddressable pins the half of the example that the
// uniform widgets cannot do: addressing a row that was never built.
func TestJumpToIndexIsAddressable(t *testing.T) {
	msgs := make([]message, 5_000)
	for i := range msgs {
		msgs[i] = newMessage(i)
	}
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Messages: msgs},
		Width:  520,
		Height: 620,
	})
	// A full frame, not a bare GenerateLayout: the index-addressed
	// scroll is converted inside the layout pipeline, after the pass
	// that would otherwise zero-fill a never-scrolled list.
	w.TestRender(mainView)
	w.ScrollToIndex(listID, 4_000)
	w.TestRender(nil)
	// Default 0: absent entry means unscrolled, which is the failure.
	if off := w.ScrollY().GetOr(listID, 0); off >= 0 {
		t.Fatalf("offset = %v, want a scrolled list", off)
	}
}
