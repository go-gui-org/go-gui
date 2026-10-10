package gui

import "testing"

// stockList is a scrolling column of rows with the stock scrollbars.
func stockList(rows int) func(*Window) View {
	return func(*Window) View {
		content := make([]View, rows)
		for i := range content {
			content[i] = Column(ContainerCfg{Sizing: FillFixed, Height: 20})
		}
		return Column(ContainerCfg{
			ID:         "list",
			Sizing:     FixedFixed,
			Width:      200,
			Height:     100,
			Scrollable: true,
			ScrollMode: ScrollVerticalOnly,
			Content:    content,
		})
	}
}

// stockBarY returns the vertical bar of stockList's container. The
// container appends the horizontal bar, then the vertical one, after
// the content, so the vertical bar is the last child.
func stockBarY(t *testing.T, w *Window) *Layout {
	t.Helper()
	sc, ok := w.layout.FindByID("list")
	if !ok {
		t.Fatal("list not found")
	}
	bar := &sc.Children[len(sc.Children)-1]
	if bar.Shape.scrollbarOrientation != scrollbarVertical {
		t.Fatalf("last child orientation %v, want vertical bar", bar.Shape.scrollbarOrientation)
	}
	return bar
}

// A stock scrollbar is built every frame for every scrollable container,
// both axes, so its cost is paid many times over. It costs one
// allocation, the view itself: the handlers are shared functions that
// read the settings from the shape, not closures holding a copy.
func TestScrollbarStockAllocs(t *testing.T) {
	w := newTestWindow()
	cfg := ScrollbarCfg{Orientation: scrollbarVertical, scrollID: "list"}
	got := testing.AllocsPerRun(200, func() {
		w.scratch.resetViewPools()
		generateViewLayout(scrollbar(cfg), w)
	})
	if got > 1 {
		t.Fatalf("stock scrollbar costs %v allocs per frame, want at most 1", got)
	}
}

// The stock bar still places its thumb, shows it on hover, scrolls on a
// thumb drag, moves the thumb with the offset and jumps on a gutter
// press.
func TestScrollbarStockBehaviour(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(stockList(20))
	w.TestRender(nil)

	thumb := stockBarY(t, w).Children[thumbIndex].Shape
	if thumb.Height <= 0 || thumb.Height >= 100 {
		t.Fatalf("thumb height %v, want a share of the 100 px track", thumb.Height)
	}
	top := thumb.Y

	// Hover over the thumb keeps it painted in the thumb color.
	x, y := thumb.X+thumb.Width/2, thumb.Y+thumb.Height/2
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.TestRender(nil)
	thumb = stockBarY(t, w).Children[thumbIndex].Shape
	if thumb.Color != DefaultScrollbarStyle.colorThumb {
		t.Fatalf("hovered thumb color %v, want %v", thumb.Color, DefaultScrollbarStyle.colorThumb)
	}

	// A drag on the thumb scrolls further.
	_, before, _ := w.TestScrollOffset("list")
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y})
	if !w.mouseIsLocked() {
		t.Fatal("press on the thumb did not start a drag")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y + 20, MouseDY: 20})
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft, MouseX: x, MouseY: y + 20})
	w.TestRender(nil)
	_, after, _ := w.TestScrollOffset("list")
	if after >= before {
		t.Fatalf("drag: offset %v, want below %v", after, before)
	}
	// The thumb follows the new offset.
	if moved := stockBarY(t, w).Children[thumbIndex].Shape.Y; moved <= top {
		t.Fatalf("thumb y %v after drag, want below %v", moved, top)
	}
	if w.mouseIsLocked() {
		t.Fatal("drag still locked after release")
	}

	// A press on the gutter near the top jumps back up.
	bar := stockBarY(t, w).Shape
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: bar.X + bar.Width/2, MouseY: bar.Y + 1})
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: bar.X + bar.Width/2, MouseY: bar.Y + 1})
	w.TestRender(nil)
	if _, jumped, _ := w.TestScrollOffset("list"); jumped <= after {
		t.Fatalf("gutter press: offset %v, want above %v", jumped, after)
	}
}
