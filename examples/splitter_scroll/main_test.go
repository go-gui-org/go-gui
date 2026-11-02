package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Scrolling the message list as far down as it goes must bring the
// newest message fully into view, with and without the Splitter
// (issue #1214). Before the fix the Splitter pane stopped about 2.8k px
// short of the end.
func TestWheelReachesNewestMessage(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)

	for _, plainRow := range []bool{false, true} {
		app := &App{ratio: 0.3, plainRow: plainRow}
		w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 800, Height: 600})
		w.TestRender(mainView)

		id := w.ResolveID("messages")[0]
		if err := w.TestScroll(id, 0, -100000); err != nil {
			t.Fatalf("plainRow=%v: TestScroll: %v", plainRow, err)
		}
		l := w.TestRender(nil)
		list, ok := l.FindByID(id)
		if !ok {
			t.Fatalf("plainRow=%v: %q not found", plainRow, id)
		}
		newest, ok := l.FindByID(w.ResolveID("newest")[0])
		if !ok {
			t.Fatalf("plainRow=%v: newest message not found", plainRow)
		}
		viewBottom := list.Shape.Y + list.Shape.Height
		if below := newest.Shape.Y + newest.Shape.Height - viewBottom; below > 0.5 {
			t.Errorf("plainRow=%v: newest message ends %.0f px below the viewport",
				plainRow, below)
		}
		if pct := w.ScrollVerticalPct(id); pct < 0.999 {
			t.Errorf("plainRow=%v: scrolled %.1f%%, want 100%%", plainRow, 100*pct)
		}
	}
}
