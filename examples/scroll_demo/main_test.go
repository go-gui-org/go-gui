package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestWindow builds the demo the way main() does, minus the backend.
func newTestWindow(t *testing.T) *gui.Window {
	t.Helper()
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewTestWindow(gui.WindowCfg{
		State:  &App{},
		Width:  400,
		Height: 600,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.SetFocus("scroll-panel")
		},
	})
	w.TestRender(nil)
	return w
}

// The panel scrolls. Written as a state assertion rather than a
// no-panic render: §4.8 of the developer-ergonomics spec tried this
// conversion and abandoned it because headless wrapped text kept its
// single-line height, so the panel never overflowed and TestScroll came
// back ErrTestUnhandled. plainTextHeightNoMeasurer now estimates the
// wrapped height, so the overflow is real in a test too.
func TestScrollPanelScrolls(t *testing.T) {
	w := newTestWindow(t)

	_, y0, err := w.TestScrollOffset("scroll-panel")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if y0 != 0 {
		t.Fatalf("initial offset y = %v, want 0", y0)
	}
	if err = w.TestScroll("scroll-panel", 0, -5); err != nil {
		t.Fatalf("TestScroll = %v, want nil", err)
	}
	_, y1, err := w.TestScrollOffset("scroll-panel")
	if err != nil {
		t.Fatalf("TestScrollOffset after scroll = %v, want nil", err)
	}
	if y1 >= 0 {
		t.Fatalf("offset y = %v after scrolling down, want < 0", y1)
	}
}

// The percentage buttons drive the panel programmatically. This is what
// the duplicate-ID fix bought: before it, all five shared one ID and
// none could be targeted.
func TestPctButtonsJumpToPosition(t *testing.T) {
	w := newTestWindow(t)

	if err := w.TestClick("scroll_demo_pct_button_100"); err != nil {
		t.Fatalf("TestClick(100%%) = %v, want nil", err)
	}
	if got := gui.State[App](w).Pct; got != 1 {
		t.Fatalf("Pct = %v after clicking 100%%, want 1", got)
	}
	_, bottom, err := w.TestScrollOffset("scroll-panel")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if bottom >= 0 {
		t.Fatalf("offset y = %v at 100%%, want < 0", bottom)
	}

	// Back to the top: a distinct button, so a shared ID would show up
	// here as the offset not moving.
	if err = w.TestClick("scroll_demo_pct_button_0"); err != nil {
		t.Fatalf("TestClick(0%%) = %v, want nil", err)
	}
	if got := gui.State[App](w).Pct; got != 0 {
		t.Fatalf("Pct = %v after clicking 0%%, want 0", got)
	}
	_, top, err := w.TestScrollOffset("scroll-panel")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if top != 0 {
		t.Fatalf("offset y = %v at 0%%, want 0", top)
	}
}
