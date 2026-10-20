package gui

import (
	"fmt"
	"testing"
)

// touchPressWindow runs the frame pipeline over a 300x400 window: a
// scrollable list "list" of rows buttons (each counts its clicks in
// clicks), and, under it, a button "outside" in no scrollable.
func touchPressWindow(t *testing.T, rows int, clicks map[string]int) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		content := make([]View, 0, rows)
		for i := range rows {
			id := fmt.Sprintf("b%d", i)
			content = append(content, Button(ButtonCfg{
				ID: id, Sizing: FillFit, Label: id,
				OnClick: func(ctx EventCtx) {
					clicks[id]++
					ctx.Consume()
				},
			}))
		}
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "list", Scrollable: true, Sizing: FillFixed, Height: 300,
					Content: content,
				}),
				Button(ButtonCfg{
					ID: "outside", Label: "outside",
					OnClick: func(ctx EventCtx) {
						clicks["outside"]++
						ctx.Consume()
					},
				}),
			},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.viewState.gesture.nowFn = fixedClock(0)
	return w
}

// touchAt sends one single-finger touch event at time t (nanos).
func touchAt(w *Window, typ EventType, t int64, x, y float32) {
	w.viewState.gesture.nowFn = fixedClock(t)
	w.handleTouch(&w.layout, touchEvent(typ, 1, x, y))
}

// rowCenter is the center of the list button id in window coordinates.
func rowCenter(t *testing.T, w *Window, id string) (x, y float32) {
	t.Helper()
	ids := w.ResolveID(id)
	if len(ids) == 0 {
		t.Fatalf("no %q in the frame", id)
	}
	ly, ok := w.layout.FindByID(ids[0])
	if !ok {
		t.Fatalf("no %q in the layout", id)
	}
	return ly.Shape.X + ly.Shape.Width/2, ly.Shape.Y + ly.Shape.Height/2
}

// TestTouchShortPressClicks checks a short press on a row still clicks
// it, once, at the lift: a quick tap, and one that wobbles under the pan
// threshold.
func TestTouchShortPressClicks(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b2")

	touchAt(w, EventTouchesBegan, 0, x, y)
	if clicks["b2"] != 0 {
		t.Fatalf("clicked at touch-down: %v, want the press held", clicks)
	}
	touchAt(w, EventTouchesEnded, 60_000_000, x, y)
	if clicks["b2"] != 1 {
		t.Fatalf("quick tap clicks = %v, want b2 once", clicks)
	}

	touchAt(w, EventTouchesBegan, 1_000_000_000, x, y)
	touchAt(w, EventTouchesMoved, 1_016_000_000, x+4, y+5)
	touchAt(w, EventTouchesEnded, 1_032_000_000, x+4, y+5)
	if clicks["b2"] != 2 {
		t.Fatalf("tap with a wobble clicks = %v, want b2 twice", clicks)
	}
}

// TestTouchPanDoesNotClick checks a scroll that starts on a row scrolls
// the list and never clicks the row.
func TestTouchPanDoesNotClick(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b4")
	touchAt(w, EventTouchesBegan, 0, x, y)
	for i, dy := range []float32{-12, -30, -60} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x, y+dy)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x, y-60)
	if len(clicks) != 0 {
		t.Fatalf("clicks = %v, want none for a scroll", clicks)
	}
	if v, _ := w.scrollY().Get("list"); v >= 0 {
		t.Fatalf("list offset = %v, want it scrolled", v)
	}
}

// TestTouchCrossAxisPanPresses checks a pan the list cannot scroll goes
// to the widget under the finger: the held press goes out at the touch
// point.
func TestTouchCrossAxisPanPresses(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b1")
	touchAt(w, EventTouchesBegan, 0, x, y)
	touchAt(w, EventTouchesMoved, 16_000_000, x+20, y)
	if clicks["b1"] != 1 {
		t.Fatalf("clicks = %v, want b1 pressed by a pan the list cannot scroll", clicks)
	}
}

// TestTouchLongPressPresses checks a long press sends the held press
// before the gesture, as a press at touch-down did.
func TestTouchLongPressPresses(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b3")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if clicks["b3"] != 0 {
		t.Fatalf("clicked at touch-down: %v", clicks)
	}
	gestureLongPressFired(nil, w)
	if clicks["b3"] != 1 {
		t.Fatalf("clicks after the long press = %v, want b3 once", clicks)
	}
	if w.viewState.gesture.gestureType != GestureLongPress {
		t.Fatalf("gesture = %v, want a long press", w.viewState.gesture.gestureType)
	}
	touchAt(w, EventTouchesEnded, 700_000_000, x, y)
	if clicks["b3"] != 1 {
		t.Fatalf("clicks after the lift = %v, want b3 once", clicks)
	}
}

// TestTouchSecondFingerDropsPress checks a pinch that starts on a row
// does not click it.
func TestTouchSecondFingerDropsPress(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b2")
	touchAt(w, EventTouchesBegan, 0, x, y)
	w.handleTouch(&w.layout, twoTouchEvent(EventTouchesBegan, 1, x, y, 2, x+40, y+40))
	w.handleTouch(&w.layout, twoTouchEvent(EventTouchesEnded, 1, x, y, 2, x+40, y+40))
	if len(clicks) != 0 {
		t.Fatalf("clicks = %v, want none for a two-finger touch", clicks)
	}
}

// TestTouchPressNowOutsideScroll checks a press with no scroll to wait
// for goes out at touch-down: outside any scrollable, and in a list
// whose rows all fit.
func TestTouchPressNowOutsideScroll(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "outside")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if clicks["outside"] != 1 {
		t.Fatalf("clicks = %v, want outside pressed at touch-down", clicks)
	}
	touchAt(w, EventTouchesEnded, 50_000_000, x, y)

	fit := map[string]int{}
	w = touchPressWindow(t, 3, fit)
	x, y = rowCenter(t, w, "b0")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if fit["b0"] != 1 {
		t.Fatalf("clicks = %v, want b0 pressed at touch-down in a list that fits", fit)
	}
}

// listThumb is the list's vertical scrollbar thumb.
func listThumb(t *testing.T, w *Window) *Shape {
	t.Helper()
	sc, ok := findScrollLayout(w, "list")
	if !ok {
		t.Fatal("no scrollable list")
	}
	for i := range sc.Children {
		bar := &sc.Children[i]
		if bar.Shape.scrollbarOrientation == scrollbarVertical && len(bar.Children) > 0 {
			return bar.Children[0].Shape
		}
	}
	t.Fatal("no vertical scrollbar")
	return nil
}

// TestTouchDragsScrollbarThumb checks a finger drags the thumb: a pan
// along the bar sends it the press, the list scrolls the way the thumb
// moves, and the content does not pan against it.
func TestTouchDragsScrollbarThumb(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	th := listThumb(t, w)
	x, y := th.X+th.Width/2, th.Y+th.Height/2
	touchAt(w, EventTouchesBegan, 0, x, y)
	if w.mouseIsLocked() {
		t.Fatal("thumb took the press at touch-down, want it held")
	}
	sc, _ := findScrollLayout(w, "list")
	viewH := sc.Shape.Height - sc.Shape.paddingHeight()
	content := contentHeight(sc)
	_, travel := scrollbarThumbGeom(viewH, content, DefaultScrollbarStyle.minThumbSize)
	var last float32
	for i, dy := range []float32{5, 20, 40, 60} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x, y+dy)
		v, _ := w.scrollY().Get("list")
		if v > last {
			t.Fatalf("thumb +%v: offset %v went back from %v", dy, v, last)
		}
		last = v
	}
	// The thumb alone moves the content: 60 px of thumb is 60 px of
	// its travel. A pan under it as well would pull the content back.
	want := -60 * (content - viewH) / travel
	if d := last - want; d > 0.5 || d < -0.5 {
		t.Fatalf("offset after a 60 px thumb drag = %v, want %v", last, want)
	}
	touchAt(w, EventTouchesEnded, 200_000_000, x, y+60)
	if len(clicks) != 0 {
		t.Fatalf("clicks = %v, want none from a thumb drag", clicks)
	}
}

// TestTouchPassiveOverlayHoldsPress checks a list under a passive
// overlay still holds the press back. The press walk falls through the
// overlay, which takes no press, to the row under it, so a scroll that
// started there clicked it.
func TestTouchPassiveOverlayHoldsPress(t *testing.T) {
	clicks := 0
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		rows := make([]View, 0, 40)
		for i := range 40 {
			rows = append(rows, Button(ButtonCfg{
				ID: fmt.Sprintf("b%d", i), Sizing: FillFit, Label: "row",
				OnClick: func(ctx EventCtx) {
					clicks++
					ctx.Consume()
				},
			}))
		}
		return Stack(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{ID: "list", Scrollable: true, Sizing: FillFill, Content: rows}),
				Column(ContainerCfg{Sizing: FillFill, SizeBorder: NoBorder}),
			},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	x, y := rowCenter(t, w, "b3")
	touchAt(w, EventTouchesBegan, 0, x, y)
	for i, dy := range []float32{-12, -30, -60} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x, y+dy)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x, y-60)
	if clicks != 0 {
		t.Fatalf("clicks = %d, want none for a scroll under a passive overlay", clicks)
	}
	if v, _ := w.scrollY().Get("list"); v >= 0 {
		t.Fatalf("list offset = %v, want it scrolled", v)
	}
	touchAt(w, EventTouchesBegan, 1_000_000_000, x, y)
	touchAt(w, EventTouchesEnded, 1_050_000_000, x, y)
	if clicks != 1 {
		t.Fatalf("clicks after a tap = %d, want 1", clicks)
	}
}

// TestTouchHeldPressMovesFromFinger checks a held press makes the finger
// the pointer: the moves after it carry deltas from the touch, not from
// where a mouse last was.
func TestTouchHeldPressMovesFromFinger(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	w.EventFn(&Event{Type: EventMouseMove, MouseX: 5, MouseY: 5})
	x, y := rowCenter(t, w, "b4")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if !w.viewState.gesture.pressHeld {
		t.Fatal("press not held")
	}
	touchAt(w, EventTouchesMoved, 16_000_000, x, y-12)
	touchAt(w, EventTouchesMoved, 32_000_000, x, y-30)
	e := w.scratch.gestureEvent
	if e.Type != EventMouseMove || e.MouseDX != 0 || e.MouseDY != -30 {
		t.Fatalf("pan move = %v dx %v dy %v, want a move of dy -30 from the touch",
			e.Type, e.MouseDX, e.MouseDY)
	}
}

// wideRowWindow runs the frame pipeline over a page "page" that scrolls
// vertically, holding a row "wide" too wide for it at the top: the
// row scrolls horizontally and shows a horizontal bar, as the
// showcase's data grid does on a phone.
func wideRowWindow(t *testing.T) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		cells := make([]View, 0, 10)
		for i := range 10 {
			cells = append(cells, Button(ButtonCfg{
				ID: fmt.Sprintf("c%d", i), Width: 80, Sizing: FixedFit, Label: "cell",
			}))
		}
		rows := []View{Row(ContainerCfg{
			ID: "wide", Scrollable: true, ScrollMode: ScrollHorizontalOnly,
			Sizing: FillFixed, Height: 80, Content: cells,
		})}
		for i := range 30 {
			rows = append(rows, Button(ButtonCfg{ID: fmt.Sprintf("b%d", i), Sizing: FillFit, Label: "row"}))
		}
		return Column(ContainerCfg{ID: "page", Scrollable: true, Sizing: FillFill, Content: rows})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	w.viewState.gesture.nowFn = fixedClock(0)
	return w
}

// barOf returns the scrollbar of the scrollable id with the orientation.
func barOf(t *testing.T, w *Window, id string, o ScrollbarOrientation) *Layout {
	t.Helper()
	sc, ok := findScrollLayout(w, id)
	if !ok {
		t.Fatalf("no scrollable %q", id)
	}
	for i := range sc.Children {
		if sc.Children[i].Shape.scrollbarOrientation == o {
			return &sc.Children[i]
		}
	}
	t.Fatalf("no scrollbar %v on %q", o, id)
	return nil
}

// TestTouchPanAcrossScrollbarScrolls checks a scroll that starts on a
// bar it crosses scrolls the content: a vertical pan from a horizontal
// bar moves the page, and the bar neither jumps nor holds the finger.
func TestTouchPanAcrossScrollbarScrolls(t *testing.T) {
	w := wideRowWindow(t)
	bar := barOf(t, w, ScopeID("page", "wide"), scrollbarHorizontal)
	// The middle of the track, right of the thumb at the start.
	x := bar.Shape.X + bar.Shape.Width*0.7
	y := bar.Shape.Y + bar.Shape.Height/2
	touchAt(w, EventTouchesBegan, 0, x, y)
	for i, dy := range []float32{-12, -30, -60} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x, y+dy)
	}
	if v, _ := w.scrollY().Get("page"); v >= 0 {
		t.Fatalf("page offset = %v, want a vertical pan from the bar to scroll it", v)
	}
	if v, _ := w.scrollX().Get(ScopeID("page", "wide")); v != 0 {
		t.Fatalf("row offset = %v, want the bar not to jump", v)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x, y-60)
	if w.mouseIsLocked() {
		t.Fatal("mouse still locked after the lift")
	}
}

// TestTouchTapOnTrackJumps checks a tap on a scrollbar track still jumps
// there, at the lift.
func TestTouchTapOnTrackJumps(t *testing.T) {
	w := wideRowWindow(t)
	bar := barOf(t, w, ScopeID("page", "wide"), scrollbarHorizontal)
	x := bar.Shape.X + bar.Shape.Width*0.7
	y := bar.Shape.Y + bar.Shape.Height/2
	touchAt(w, EventTouchesBegan, 0, x, y)
	touchAt(w, EventTouchesEnded, 60_000_000, x, y)
	if v, _ := w.scrollX().Get(ScopeID("page", "wide")); v >= 0 {
		t.Fatalf("row offset = %v, want the tap on the track to jump", v)
	}
	if w.mouseIsLocked() {
		t.Fatal("mouse still locked after the lift")
	}
}

// dragStripPage runs the frame pipeline over a page "page" that scrolls
// vertically, holding at the top a DragScroll strip "strip" 100 px high.
// A horizontal strip holds 20 cards 100 px wide and scrolls sideways
// only; a vertical one holds 20 rows and scrolls up and down only.
func dragStripPage(t *testing.T, horizontal bool) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		items := make([]View, 0, 20)
		for i := range 20 {
			items = append(items, Button(ButtonCfg{
				ID: fmt.Sprintf("c%d", i), Label: "card", Sizing: FixedFill, Width: 100,
			}))
		}
		strip := ContainerCfg{
			ID: "strip", Scrollable: true, DragScroll: true,
			Sizing: FillFixed, Height: 100, Content: items,
		}
		var top View
		if horizontal {
			strip.ScrollMode = ScrollHorizontalOnly
			top = Row(strip)
		} else {
			strip.ScrollMode = ScrollVerticalOnly
			for i := range items {
				items[i] = Button(ButtonCfg{
					ID: fmt.Sprintf("c%d", i), Label: "item", Sizing: FillFit,
				})
			}
			top = Column(strip)
		}
		rows := []View{top}
		for i := range 40 {
			rows = append(rows, Button(ButtonCfg{
				ID: fmt.Sprintf("b%d", i), Sizing: FillFit, Label: "row",
			}))
		}
		return Column(ContainerCfg{ID: "page", Scrollable: true, Sizing: FillFill, Content: rows})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	return w
}

// stripSwipe touches the strip's center, moves the finger by each step
// of (dxs[i], dys[i]) from there, and lifts it.
func stripSwipe(t *testing.T, w *Window, dxs, dys []float32) {
	t.Helper()
	x, y := rowCenter(t, w, "strip")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if !w.viewState.dragPan.active {
		t.Fatal("strip did not take its press at touch-down")
	}
	for i := range dxs {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x+dxs[i], y+dys[i])
	}
	last := len(dxs) - 1
	touchAt(w, EventTouchesEnded, 300_000_000, x+dxs[last], y+dys[last])
}

// TestTouchPanAroundDragScrollScrolls checks a vertical swipe that starts
// on a horizontal DragScroll strip scrolls the page around it. The strip
// takes the mouse lock at touch-down, and the pan fallback stood down for
// every lock, so the page did not move.
func TestTouchPanAroundDragScrollScrolls(t *testing.T) {
	w := dragStripPage(t, true)
	stripSwipe(t, w, []float32{0, 0, 0}, []float32{-12, -30, -60})
	if v, _ := w.scrollY().Get("page"); v >= 0 {
		t.Fatalf("page offset = %v, want the page scrolled by a swipe on the strip", v)
	}
}

// TestTouchSidewaysSwipeOnStripHoldsPage checks a sideways swipe on a
// horizontal strip moves the strip and not the page, though the finger
// drifts up as it goes: the strip owns the pan's axis.
func TestTouchSidewaysSwipeOnStripHoldsPage(t *testing.T) {
	w := dragStripPage(t, true)
	stripSwipe(t, w, []float32{-12, -40, -80}, []float32{-3, -8, -15})
	if v, _ := w.scrollX().Get(w.ResolveID("strip")[0]); v >= 0 {
		t.Fatalf("strip offset = %v, want it scrolled", v)
	}
	if v, _ := w.scrollY().Get("page"); v != 0 {
		t.Fatalf("page offset = %v, want 0 for a sideways swipe", v)
	}
}

// TestTouchSameAxisDragScrollMovesOnce checks a vertical swipe on a
// vertical DragScroll strip in a vertical page moves the strip alone.
// Both moved, so the content went twice as far as the finger.
func TestTouchSameAxisDragScrollMovesOnce(t *testing.T) {
	w := dragStripPage(t, false)
	stripSwipe(t, w, []float32{0, 0, 0}, []float32{-12, -30, -60})
	if v, _ := w.scrollY().Get(w.ResolveID("strip")[0]); v >= 0 {
		t.Fatalf("strip offset = %v, want it scrolled", v)
	}
	if v, _ := w.scrollY().Get("page"); v != 0 {
		t.Fatalf("page offset = %v, want 0: the strip owns the axis", v)
	}
}

// TestTouchOverlayControlTakesPress checks a control in an overlay over a
// list gets its press at touch-down. The scan fell through the overlay
// to the list, held the press, and a vertical drag dropped it, so a
// vertical slider in the overlay did not move.
func TestTouchOverlayControlTakesPress(t *testing.T) {
	var value float32 = 50
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		rows := make([]View, 0, 40)
		for i := range 40 {
			rows = append(rows, Button(ButtonCfg{
				ID: fmt.Sprintf("b%d", i), Sizing: FillFit, Label: "row",
			}))
		}
		return Stack(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{ID: "list", Scrollable: true, Sizing: FillFill, Content: rows}),
				Column(ContainerCfg{
					Sizing: FixedFixed, Width: 80, Height: 300,
					Content: []View{Slider(SliderCfg{
						ID: "sl", Vertical: true, Height: 260, Value: value, Min: 0, Max: 100,
						OnChange: func(v float32, ctx EventCtx) {
							value = v
							ctx.Consume()
						},
					})},
				}),
			},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	x, y := rowCenter(t, w, "sl")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if w.viewState.gesture.pressHeld {
		t.Fatal("press held for a slider in an overlay, want it sent at touch-down")
	}
	for i, dy := range []float32{12, 30, 60} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x, y+dy)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x, y+60)
	if value == 50 {
		t.Fatal("slider value = 50, want it moved by the drag")
	}
}

// A shape in an overlay whose OnClick fires only for the right button does
// not take a touch press, which is a left press: the press walk passes it
// to the list under it, so the list still holds the press back.
func TestTouchRightClickOverlayPassesPress(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		rows := make([]View, 0, 40)
		for i := range 40 {
			rows = append(rows, Button(ButtonCfg{
				ID: fmt.Sprintf("b%d", i), Sizing: FillFit, Label: "row",
			}))
		}
		return Stack(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{ID: "list", Scrollable: true, Sizing: FillFill, Content: rows}),
				Column(ContainerCfg{
					ID: "menu", Sizing: FixedFixed, Width: 80, Height: 300,
					OnAnyClick:  func(EventCtx) {},
					clickButton: clickRightOnly,
				}),
			},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	x, y := rowCenter(t, w, "menu")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if !w.viewState.gesture.pressHeld {
		t.Fatal("press sent at touch-down, want it held for the list under a right-click-only overlay")
	}
}

// TestTouchTapCatchingFlingDoesNotClick checks a tap that stops a fling
// only stops it: the row under the finger is not clicked, as on iOS and
// Android.
func TestTouchTapCatchingFlingDoesNotClick(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	if !scrollMomentumStart(w, "list", scrollAxisY, -3000) {
		t.Fatal("fling did not start")
	}
	x, y := rowCenter(t, w, "b2")
	touchAt(w, EventTouchesBegan, 0, x, y)
	touchAt(w, EventTouchesEnded, 50_000_000, x, y)
	if len(clicks) != 0 {
		t.Fatalf("clicks = %v, want none for a tap that caught a fling", clicks)
	}
	// The next tap, with nothing gliding, clicks as usual.
	touchAt(w, EventTouchesBegan, 400_000_000, x, y)
	touchAt(w, EventTouchesEnded, 450_000_000, x, y)
	if clicks["b2"] != 1 {
		t.Fatalf("clicks = %v, want b2 clicked by the tap after the catch", clicks)
	}
}

// TestTouchDiagonalPanScrolls checks a diagonal swipe on a vertical list
// is a scroll, not a sideways pan: the finger moves a bit more sideways
// than down, and no row is pressed.
func TestTouchDiagonalPanScrolls(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b4")
	touchAt(w, EventTouchesBegan, 0, x, y)
	for i, d := range []float32{-9, -24, -48} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x-d*1.2, y+d)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x+58, y-48)
	if len(clicks) != 0 {
		t.Fatalf("clicks = %v, want none for a diagonal scroll", clicks)
	}
	if v, _ := w.scrollY().Get("list"); v >= 0 {
		t.Fatalf("list offset = %v, want it scrolled", v)
	}
}

// TestTouchTapBesideFlingClicks checks a tap stops a fling without a click
// only on the content that was gliding: a fling in one list does not
// swallow a tap on a row of another.
func TestTouchTapBesideFlingClicks(t *testing.T) {
	clicks := map[string]int{}
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	list := func(id string) View {
		rows := make([]View, 0, 40)
		for i := range 40 {
			row := fmt.Sprintf("%s%d", id, i)
			rows = append(rows, Button(ButtonCfg{
				ID: row, Sizing: FillFit, Label: row,
				OnClick: func(ctx EventCtx) {
					clicks[row]++
					ctx.Consume()
				},
			}))
		}
		return Column(ContainerCfg{ID: id, Scrollable: true, Sizing: FillFill, Content: rows})
	}
	w.viewGenerator = func(*Window) View {
		return Row(ContainerCfg{Sizing: FillFill, Content: []View{list("left"), list("right")}})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	if !scrollMomentumStart(w, "left", scrollAxisY, -3000) {
		t.Fatal("fling did not start")
	}
	x, y := rowCenter(t, w, "right2")
	touchAt(w, EventTouchesBegan, 0, x, y)
	touchAt(w, EventTouchesEnded, 50_000_000, x, y)
	if clicks["right2"] != 1 {
		t.Fatalf("clicks = %v, want right2 clicked: the fling was in the other list", clicks)
	}
}

// TestTouchSecondFingerReleasesDragPan checks a second finger ends a
// DragScroll pan: its mouse release drops the drag lock, so the pan that
// follows a pinch never runs under a DragScroll lock.
func TestTouchSecondFingerReleasesDragPan(t *testing.T) {
	w := dragStripPage(t, true)
	x, y := rowCenter(t, w, "strip")
	touchAt(w, EventTouchesBegan, 0, x, y)
	if !w.viewState.dragPan.active {
		t.Fatal("strip did not take the press")
	}
	w.handleTouch(&w.layout, &Event{
		Type: EventTouchesBegan, NumTouches: 2,
		Touches: [8]TouchPoint{
			{Identifier: 1, PosX: x, PosY: y},
			{Identifier: 2, PosX: x + 40, PosY: y, Changed: true},
		},
	})
	if w.viewState.dragPan.active || w.mouseIsLocked() {
		t.Fatal("drag lock still held after a second finger")
	}
}
