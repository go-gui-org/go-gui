package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestWindow builds the demo the way main() does, minus the backend.
func newTestWindow(t *testing.T) *gui.Window {
	t.Helper()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewTestWindow(gui.WindowCfg{
		State:  &App{},
		Width:  400,
		Height: 600,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
			w.SetFocus(gui.ScopeID("scroll-panel", "scroll-text"))
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

// The panel keeps both scrollbar views (X auto + Y auto) and the text
// overflows, so a thumb is drawn. Before the fix the Y config was
// ScrollbarHidden until the panel held focus, but the focus check
// compared the bare leaf "scroll-panel" while the focused text resolves
// to "scroll-panel:scroll-text" — so the Y bar was never appended.
func TestScrollPanelHasScrollbar(t *testing.T) {
	w := newTestWindow(t)

	wantFocus := gui.ScopeID("scroll-panel", "scroll-text")
	if got := w.FocusID(); got != wantFocus {
		t.Fatalf("FocusID = %q, want %q", got, wantFocus)
	}

	overflow, ok := w.ScrollOverflowY("scroll-panel")
	if !ok {
		t.Fatalf("ScrollOverflowY(scroll-panel) not found")
	}
	if overflow <= 0 {
		t.Fatalf("ScrollOverflowY = %v, want > 0", overflow)
	}

	root := w.TestRender(nil)
	panel, ok := root.FindByID("scroll-panel")
	if !ok {
		t.Fatalf("FindByID(scroll-panel) not found")
	}
	// Content is the text plus the two appended scrollbar views. With
	// the old Hidden overflow the Y bar was dropped and this was 2.
	if len(panel.Children) != 3 {
		t.Fatalf("scroll-panel children = %d, want 3 (text + X + Y scrollbars)", len(panel.Children))
	}
}

// The percentage buttons drive the panel programmatically. This is what
// the duplicate-ID fix bought: before it, all five shared one ID and
// none could be targeted.
func TestPctButtonsJumpToPosition(t *testing.T) {
	w := newTestWindow(t)

	if err := w.TestClick("scroll_demo:pct_button:100"); err != nil {
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
	if err = w.TestClick("scroll_demo:pct_button:0"); err != nil {
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
