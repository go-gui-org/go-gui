package gui

import "testing"

// touchDragPage runs the frame pipeline over a 300x400 window: a
// scrollable "page" that overflows both ways and holds the view drag at
// its left, 150 px down. Issue #1226: a pan along a drag control in a
// scrollable that can scroll on that axis went to the scrollable.
func touchDragPage(t *testing.T, drag View) *Window {
	t.Helper()
	return touchDragPageMode(t, ScrollBoth, drag)
}

// touchDragPageMode is touchDragPage with the page's scroll mode.
func touchDragPageMode(t *testing.T, mode ScrollMode, drag View) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			ID: "page", Scrollable: true, ScrollMode: mode, Sizing: FillFill,
			Content: []View{
				// Room above the drag, so a pan up stays in the window.
				Rectangle(RectangleCfg{Width: 10, Height: 150, Sizing: FixedFixed}),
				drag,
				// Wider and taller than the window: the page scrolls
				// on both axes.
				Rectangle(RectangleCfg{Width: 900, Height: 900, Sizing: FixedFixed}),
			},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.viewState.gesture.nowFn = fixedClock(0)
	return w
}

// touchPan sends a press at x, y, moves by dx, dy in four steps and
// lifts.
func touchPan(w *Window, x, y, dx, dy float32) {
	touchAt(w, EventTouchesBegan, 0, x, y)
	for i, f := range []float32{0.25, 0.5, 0.75, 1} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x+dx*f, y+dy*f)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x+dx, y+dy)
}

func touchDragSlider(value *float32) View {
	return Slider(SliderCfg{
		ID: "s", Width: 250, Min: 0, Max: 100, Value: *value,
		OnChange: func(v float32, _ EventCtx) { *value = v },
	})
}

// TestTouchPanAlongSliderDrags checks a pan along a horizontal slider
// drags it, though the page around it scrolls horizontally too, and
// the page does not move.
func TestTouchPanAlongSliderDrags(t *testing.T) {
	var value float32 = 10
	w := touchDragPage(t, touchDragSlider(&value))
	x, y := rowCenter(t, w, "s")
	// The press at x-60 alone sets about 26. A value past 50 needs the
	// later moves to drag the handle on to x+40 (about 66).
	touchPan(w, x-60, y, 100, 0)
	if value <= 50 {
		t.Fatalf("slider value = %v, want it dragged past 50", value)
	}
	if v, _ := w.scrollX().Get("page"); v != 0 {
		t.Fatalf("page x offset = %v, want 0: the slider took the pan", v)
	}
}

// TestTouchPanAcrossSliderScrolls checks a pan across a horizontal
// slider still scrolls the page and leaves the value alone.
func TestTouchPanAcrossSliderScrolls(t *testing.T) {
	var value float32 = 10
	w := touchDragPage(t, touchDragSlider(&value))
	x, y := rowCenter(t, w, "s")
	touchPan(w, x, y, 0, -100)
	if value != 10 {
		t.Fatalf("slider value = %v, want 10: a pan across is a scroll", value)
	}
	if v, _ := w.scrollY().Get("page"); v >= 0 {
		t.Fatalf("page y offset = %v, want it scrolled", v)
	}
}

// TestTouchPanAlongSplitterDrags checks a pan along a splitter handle
// drags it, though the page around it scrolls on that axis.
func TestTouchPanAlongSplitterDrags(t *testing.T) {
	changes := 0
	w := touchDragPage(t, Row(ContainerCfg{
		Width: 250, Height: 200, Sizing: FixedFixed, Padding: NoPadding,
		SizeBorder: NoBorder,
		Content: []View{Splitter(SplitterCfg{
			ID: "sp", Sizing: FillFill,
			Orientation: SplitterHorizontal,
			First:       SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "a"})}},
			Second:      SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "b"})}},
			OnChange: func(float32, SplitterCollapsed, EventCtx) {
				changes++
			},
		})},
	}))
	x, y := rowCenter(t, w, ScopeID(ScopeID("page", "sp"), "handle"))
	touchPan(w, x, y, 50, 0)
	if changes == 0 {
		t.Fatal("splitter did not change, want the handle dragged")
	}
	if v, _ := w.scrollX().Get("page"); v != 0 {
		t.Fatalf("page x offset = %v, want 0: the handle took the pan", v)
	}
}

// TestTouchPanAlongLookSliderDrags checks a slider drawn by a Look
// takes a pan along its track as the stock one does.
func TestTouchPanAlongLookSliderDrags(t *testing.T) {
	var value float32 = 10
	w := touchDragPage(t, Slider(SliderCfg{
		ID: "s", Width: 250, Height: 20, Min: 0, Max: 100, Value: value,
		OnChange: func(v float32, _ EventCtx) { value = v },
		Look: func(SliderLookState) SliderParts {
			return SliderParts{
				Track: Row(ContainerCfg{Sizing: FillFill,
					SizeBorder: NoBorder, Padding: NoPadding}),
				Handle: Column(ContainerCfg{Width: 16, Height: 16,
					Sizing: FixedFixed, SizeBorder: NoBorder, Padding: NoPadding}),
			}
		},
	}))
	x, y := rowCenter(t, w, "s")
	// The press at x-60 alone sets about 26. A value past 50 needs the
	// later moves to drag the handle on to x+40 (about 66).
	touchPan(w, x-60, y, 100, 0)
	if value <= 50 {
		t.Fatalf("slider value = %v, want it dragged past 50", value)
	}
	if v, _ := w.scrollX().Get("page"); v != 0 {
		t.Fatalf("page x offset = %v, want 0: the slider took the pan", v)
	}
}

// TestTouchPanAlongVerticalSliderDrags checks a vertical slider takes a
// vertical pan in a page that scrolls vertically.
func TestTouchPanAlongVerticalSliderDrags(t *testing.T) {
	var value float32 = 50
	w := touchDragPage(t, Slider(SliderCfg{
		ID: "s", Vertical: true, Height: 200, Min: 0, Max: 100, Value: value,
		OnChange: func(v float32, _ EventCtx) { value = v },
	}))
	x, y := rowCenter(t, w, "s")
	touchPan(w, x, y, 0, -60)
	if value == 50 {
		t.Fatal("slider value = 50, want it dragged")
	}
	if v, _ := w.scrollY().Get("page"); v != 0 {
		t.Fatalf("page y offset = %v, want 0: the slider took the pan", v)
	}
}

// TestTouchDiagonalPanOnSliderInVerticalListScrolls checks the drag rule
// leaves a list that scrolls only across the slider alone: a diagonal
// pan, more along the slider than down but within the cross ratio, still
// scrolls the list (#1116) and does not drag the slider.
func TestTouchDiagonalPanOnSliderInVerticalListScrolls(t *testing.T) {
	var value float32 = 10
	w := touchDragPageMode(t, ScrollVerticalOnly, touchDragSlider(&value))
	x, y := rowCenter(t, w, "s")
	touchPan(w, x-40, y, 80, -50)
	if value != 10 {
		t.Fatalf("slider value = %v, want 10: the pan is within the cross ratio", value)
	}
	if v, _ := w.scrollY().Get("page"); v >= 0 {
		t.Fatalf("page y offset = %v, want it scrolled", v)
	}
}

// TestTouchPanFromCollapseButtonScrolls checks a pan that starts on a
// button inside a drag control is not the control's drag: the button
// takes the press, and the press is its click. A pan along the
// splitter handle from a collapse button scrolls the page and does not
// collapse a pane.
func TestTouchPanFromCollapseButtonScrolls(t *testing.T) {
	collapsed := SplitterCollapseNone
	w := touchDragPage(t, Row(ContainerCfg{
		Width: 250, Height: 200, Sizing: FixedFixed, Padding: NoPadding,
		SizeBorder: NoBorder,
		Content: []View{Splitter(SplitterCfg{
			ID: "sp", Sizing: FillFill,
			Orientation:         SplitterHorizontal,
			ShowCollapseButtons: true,
			First: SplitterPaneCfg{Collapsible: true,
				Content: []View{Text(TextCfg{Text: "a"})}},
			Second: SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "b"})}},
			OnChange: func(_ float32, c SplitterCollapsed, _ EventCtx) {
				collapsed = c
			},
		})},
	}))
	x, y := rowCenter(t, w, "page:sp"+splitterButtonSuffix[SplitterCollapseFirst])
	touchPan(w, x, y, -60, 0)
	if collapsed != SplitterCollapseNone {
		t.Fatalf("collapsed = %v, want none: a pan is not a click", collapsed)
	}
	if v, _ := w.scrollX().Get("page"); v >= 0 {
		t.Fatalf("page x offset = %v, want it scrolled", v)
	}
}

// TestTouchPanAlongDisabledSliderScrolls checks a disabled slider is not
// a drag control: a pan along it scrolls the page that scrolls on that
// axis.
func TestTouchPanAlongDisabledSliderScrolls(t *testing.T) {
	var value float32 = 10
	w := touchDragPage(t, Slider(SliderCfg{
		ID: "s", Width: 250, Min: 0, Max: 100, Value: value, Disabled: true,
		OnChange: func(v float32, _ EventCtx) { value = v },
	}))
	x, y := rowCenter(t, w, "s")
	touchPan(w, x+60, y, -100, 0)
	if value != 10 {
		t.Fatalf("slider value = %v, want 10: the slider is disabled", value)
	}
	if v, _ := w.scrollX().Get("page"); v >= 0 {
		t.Fatalf("page x offset = %v, want it scrolled", v)
	}
}

// TestTouchPanAt45DegreesOnVerticalSliderScrolls checks a pan with equal
// x and y on a vertical slider is not along the slider: it scrolls the
// page, as the same pan does on a horizontal slider.
func TestTouchPanAt45DegreesOnVerticalSliderScrolls(t *testing.T) {
	var value float32 = 50
	w := touchDragPage(t, Slider(SliderCfg{
		ID: "s", Vertical: true, Height: 200, Min: 0, Max: 100, Value: value,
		OnChange: func(v float32, _ EventCtx) { value = v },
	}))
	x, y := rowCenter(t, w, "s")
	touchPan(w, x, y, -40, -40)
	if value != 50 {
		t.Fatalf("slider value = %v, want 50: a 45 degree pan is a scroll", value)
	}
	if v, _ := w.scrollY().Get("page"); v >= 0 {
		t.Fatalf("page y offset = %v, want it scrolled", v)
	}
}
