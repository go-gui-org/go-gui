package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State: &App{
			Color:      gui.HSLA{H: 210, S: 0.7, L: 0.55, A: 0.85},
			ShowPacked: true,
		},
		Width:  920,
		Height: 760,
	})
	_ = mainView(w).GenerateLayout(w)
}

// Toggling the packed picker on must keep it inside the window
// width. With the panels in a Row the fourth card lands past the
// right edge (X ~1000 in a 900-wide window): space opens up but the
// picker itself is off-screen.
func TestPackedPickerStaysOnScreen(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewTestWindow(gui.WindowCfg{
		State: &App{
			Color: gui.HSLA{H: 210, S: 0.7, L: 0.55, A: 0.85},
		},
		Width:  900,
		Height: 620,
		OnInit: func(w *gui.Window) { w.SetView(mainView) },
	})
	w.TestRender(nil)
	toggle := w.ResolveID("color_picker_toggle_packed")
	if len(toggle) == 0 {
		t.Fatal("toggle not resolved")
	}
	if err := w.TestClick(toggle[0]); err != nil {
		t.Fatalf("click toggle: %v", err)
	}
	l := w.TestRender(nil)
	packed := w.ResolveID("packed")
	if len(packed) == 0 {
		t.Fatal("packed picker not resolved after toggle")
	}
	ly, ok := l.FindByID(packed[0])
	if !ok {
		t.Fatal("packed picker not found after toggle")
	}
	if ly.Shape.Width <= 0 || ly.Shape.Height <= 0 {
		t.Fatalf("packed size = %.1fx%.1f, want positive",
			ly.Shape.Width, ly.Shape.Height)
	}
	if right := ly.Shape.X + ly.Shape.Width; right > 900 {
		t.Errorf("packed right edge = %.1f, window width 900", right)
	}
}
