package scrollbars

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestWindow renders the example once.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestWindow(t *testing.T) *gui.Window {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	w := gui.NewTestWindow(gui.WindowCfg{Width: 960, Height: 640})
	w.TestRender(func(*gui.Window) gui.View { return View() })
	return w
}

// thumbOf returns the thumb of the panel's vertical scrollbar. With a Track
// hook the track is the bar's first child and the thumb its second.
func thumbOf(t *testing.T, w *gui.Window, panel string) *gui.Shape {
	t.Helper()
	ly, ok := w.TestRender(nil).FindByID(gui.ScopeID(panel, "scrollbar-y"))
	if !ok || len(ly.Children) < 2 {
		t.Fatalf("no hook scrollbar in %q", panel)
	}
	return ly.Children[1].Shape
}

// Each custom thumb has a real size, and a drag on it scrolls its panel.
func TestThumbDragScrolls(t *testing.T) {
	w := newTestWindow(t)
	for _, panel := range []string{"page:classic", "page:win98", "page:blue"} {
		th := thumbOf(t, w, panel)
		if th.Width != barSize || th.Height < 28 {
			t.Fatalf("%s: thumb %vx%v, want %v wide and at least 28 tall", panel, th.Width, th.Height, barSize)
		}
		// Copy what is needed: the shape is rebuilt in place by the next frame.
		x, y, top := th.X+th.Width/2, th.Y+th.Height/2, th.Y
		w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
		w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: x, MouseY: y + 40, MouseDY: 40})
		w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y + 40})
		w.TestRender(nil)
		if _, oy, err := w.TestScrollOffset(panel); err != nil || oy >= 0 {
			t.Fatalf("%s: offset %v err %v after drag, want scrolled", panel, oy, err)
		}
		if moved := thumbOf(t, w, panel); moved.Y <= top {
			t.Fatalf("%s: thumb y %v after drag, want below %v", panel, moved.Y, top)
		}
	}
}

// The wheel over a panel moves its custom thumb.
func TestWheelMovesThumb(t *testing.T) {
	w := newTestWindow(t)
	top := thumbOf(t, w, "page:blue").Y
	if err := w.TestScroll("page:blue", 0, -200); err != nil {
		t.Fatal(err)
	}
	if y := thumbOf(t, w, "page:blue").Y; y <= top {
		t.Fatalf("thumb y %v after wheel, want below %v", y, top)
	}
}

// The window has no duplicate IDs or debug findings.
func TestLooksClean(t *testing.T) {
	w := newTestWindow(t)
	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs: %v", dups)
	}
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("debug findings: %v", found)
	}
}
