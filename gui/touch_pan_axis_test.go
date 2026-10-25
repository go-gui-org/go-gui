package gui

import (
	"fmt"
	"testing"
)

// panAxisPage runs the frame pipeline over a page "page" that scrolls
// vertically, holding at the top a row "wide" 250 px high whose cells
// are too wide for it: the row scrolls sideways only, as a Markdown
// table does on a phone (#1118). onGesture, when set, goes on a wrapper
// between the row and the page.
func panAxisPage(t *testing.T, onGesture func(EventCtx)) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		cells := make([]View, 0, 10)
		for i := range 10 {
			cells = append(cells, Button(ButtonCfg{
				ID: fmt.Sprintf("c%d", i), Width: 80, Sizing: FixedFill, Label: "cell",
			}))
		}
		top := Row(ContainerCfg{
			ID: "wide", Scrollable: true, ScrollMode: ScrollHorizontalOnly,
			Sizing: FillFixed, Height: 250, Content: cells,
		})
		if onGesture != nil {
			top = Column(ContainerCfg{
				Sizing: FillFit, Padding: PaddingNone, SizeBorder: NoBorder,
				OnGesture: onGesture, Content: []View{top},
			})
		}
		rows := []View{top}
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

// panAxisRowID is the wide row's effective ID.
var panAxisRowID = ScopeID("page", "wide")

// panAxisStart is a touch point on a cell near the bottom of the wide
// row, above its scrollbar (a pan along a bar drags the thumb), with
// room above it for a pan that stays over the row.
func panAxisStart(t *testing.T, w *Window) (x, y float32) {
	t.Helper()
	row, ok := findScrollLayout(w, panAxisRowID)
	if !ok {
		t.Fatalf("no scrollable %q", panAxisRowID)
	}
	return row.Shape.X + row.Shape.Width/2, row.Shape.Y + row.Shape.Height - 40
}

// panAxisSwipe touches x, y, moves the finger n times by sx, sy, one
// move per 16 ms, and returns the time of the last move. The frame is
// not rebuilt between moves, so the finger stays over the row.
func panAxisSwipe(w *Window, x, y, sx, sy float32, n int) int64 {
	touchAt(w, EventTouchesBegan, 0, x, y)
	var t int64
	for i := 1; i <= n; i++ {
		t = int64(i) * 16_000_000
		touchAt(w, EventTouchesMoved, t, x+sx*float32(i), y+sy*float32(i))
	}
	return t
}

func panAxisOffsets(w *Window) (pageY, rowX float32) {
	pageY, _ = w.scrollY().Get("page")
	rowX, _ = w.scrollX().Get(panAxisRowID)
	return pageY, rowX
}

// panAxisFling reports a fling armed on id's axis.
func panAxisFling(w *Window, id string, axis scrollAxis) bool {
	if w.scrollSmooth == nil {
		return false
	}
	e := w.scrollSmooth.findEntry(id, axis)
	return e != nil && e.active && e.tau > 0
}

func TestPanAxisLock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dx, dy float32
		want   panAxis
	}{
		{0, -11, panAxisVertical},
		{2, -16, panAxisVertical},
		{-11, 0, panAxisHorizontal},
		{16, 2, panAxisHorizontal},
		{8, 8, panAxisFree},
		// Exactly the ratio stays free: touchPressOnPan treats it as a
		// scroll along the list, so the list must still scroll.
		{10, 5, panAxisFree},
		{5, 10, panAxisFree},
	}
	for _, c := range cases {
		if got := panAxisLock(c.dx, c.dy); got != c.want {
			t.Errorf("panAxisLock(%v, %v) = %v, want %v", c.dx, c.dy, got, c.want)
		}
	}
}

// TestTouchVerticalPanOverSidewaysRowScrollsPage checks a vertical pan
// that drifts sideways over a row that scrolls sideways scrolls the page
// by the finger's travel, and leaves the row where it is (#1118). The
// row took each move's drift, and the page moved 36 px for 96. The drift
// runs the way the row can scroll: from its start, content moving right
// rests on its edge and was never taken.
func TestTouchVerticalPanOverSidewaysRowScrollsPage(t *testing.T) {
	w := panAxisPage(t, nil)
	x, y := panAxisStart(t, w)
	panAxisSwipe(w, x, y, -1, -8, 12)
	pageY, rowX := panAxisOffsets(w)
	// 96 px of finger, less the threshold distance (#1117).
	if pageY > -85 || pageY < -87 {
		t.Fatalf("page offset = %v, want about -86 for a 96 px pan", pageY)
	}
	if rowX != 0 {
		t.Fatalf("row offset = %v, want the vertical pan not to move it", rowX)
	}
}

// TestTouchSidewaysPanOverRowDoesNotDriftPage checks a sideways pan that
// drifts vertically scrolls the row and not the page. Per-axis routing
// alone would hand each move's drift to the page; the lock keeps it out.
func TestTouchSidewaysPanOverRowDoesNotDriftPage(t *testing.T) {
	w := panAxisPage(t, nil)
	x, y := panAxisStart(t, w)
	panAxisSwipe(w, x, y, -8, -1, 12)
	pageY, rowX := panAxisOffsets(w)
	if rowX > -85 || rowX < -87 {
		t.Fatalf("row offset = %v, want about -86 for a 96 px pan", rowX)
	}
	if pageY != 0 {
		t.Fatalf("page offset = %v, want the sideways pan not to move it", pageY)
	}
}

// TestTouchDiagonalPanSplitsAxes checks a diagonal pan over the row
// scrolls the row sideways and the page vertically: each axis goes to
// the innermost scrollable that moves on it.
func TestTouchDiagonalPanSplitsAxes(t *testing.T) {
	w := panAxisPage(t, nil)
	x, y := panAxisStart(t, w)
	panAxisSwipe(w, x, y, -6, -6, 12)
	pageY, rowX := panAxisOffsets(w)
	if rowX > -60 {
		t.Fatalf("row offset = %v, want the diagonal pan to scroll it", rowX)
	}
	if pageY > -60 {
		t.Fatalf("page offset = %v, want the diagonal pan to scroll it", pageY)
	}
}

// TestTouchDiagonalFlickFlingsBoth checks the lift of a diagonal flick
// flings each container on the axis it moved.
func TestTouchDiagonalFlickFlingsBoth(t *testing.T) {
	w := panAxisPage(t, nil)
	x, y := panAxisStart(t, w)
	last := panAxisSwipe(w, x, y, -25, -25, 5)
	touchAt(w, EventTouchesEnded, last, x-125, y-125)
	if !panAxisFling(w, "page", scrollAxisY) {
		t.Fatal("no vertical fling on the page")
	}
	if !panAxisFling(w, panAxisRowID, scrollAxisX) {
		t.Fatal("no sideways fling on the row")
	}
}

// TestTouchVerticalFlickFlingsOnlyPage checks the lift of a vertical
// flick that drifts sideways flings the page, not the row.
func TestTouchVerticalFlickFlingsOnlyPage(t *testing.T) {
	w := panAxisPage(t, nil)
	x, y := panAxisStart(t, w)
	last := panAxisSwipe(w, x, y, -2, -30, 5)
	touchAt(w, EventTouchesEnded, last, x-10, y-150)
	if !panAxisFling(w, "page", scrollAxisY) {
		t.Fatal("no vertical fling on the page")
	}
	if panAxisFling(w, panAxisRowID, scrollAxisX) {
		t.Fatal("the vertical flick flung the row sideways")
	}
}

// TestTouchPanLockKeepsRawDeltas checks the axis lock applies to
// scrolling only: a handler around the row still reads the drift of a
// vertical-locked pan.
func TestTouchPanLockKeepsRawDeltas(t *testing.T) {
	var sumX, sumY float32
	w := panAxisPage(t, func(ctx EventCtx) {
		if e := ctx.Event; e.GestureType == GesturePan && e.GesturePhase == GesturePhaseChanged {
			sumX += e.GestureDX
			sumY += e.GestureDY
		}
	})
	x, y := panAxisStart(t, w)
	panAxisSwipe(w, x, y, -1, -8, 12)
	if sumX >= 0 {
		t.Fatalf("handler read dx %v, want the finger's sideways drift", sumX)
	}
	if sumY >= 0 {
		t.Fatalf("handler read dy %v, want the finger's vertical travel", sumY)
	}
}

// TestTouchPanTakenAxisHiddenFromAncestors checks a handler around the
// row reads only what the row left of a diagonal pan: the row took the
// sideways part, the vertical part is still to scroll.
func TestTouchPanTakenAxisHiddenFromAncestors(t *testing.T) {
	var sumX, sumY float32
	w := panAxisPage(t, func(ctx EventCtx) {
		if e := ctx.Event; e.GestureType == GesturePan && e.GesturePhase == GesturePhaseChanged {
			sumX += e.GestureDX
			sumY += e.GestureDY
		}
	})
	x, y := panAxisStart(t, w)
	panAxisSwipe(w, x, y, -6, -6, 12)
	if sumX != 0 {
		t.Fatalf("handler read dx %v, want 0: the row took it", sumX)
	}
	if sumY >= 0 {
		t.Fatalf("handler read dy %v, want the vertical travel no scrollable inside took", sumY)
	}
}

// TestTouchPanLeftoverSkipsLowerLayers checks the axis a scrollable did
// not take goes to its ancestors only, never to a layer under it: a
// diagonal pan on a vertical list floating over a sideways table
// scrolls the list and leaves the table alone, as the first move to
// scroll anything used to stop the walk.
func TestTouchPanLeftoverSkipsLowerLayers(t *testing.T) {
	w := &Window{}
	w.animations = make(map[string]Animation)
	clip := drawClip{X: 0, Y: 0, Width: 400, Height: 200}
	w.layout = Layout{
		Shape: &Shape{
			Width: 400, Height: 400,
			shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
		},
		Children: []Layout{
			{
				Shape: &Shape{
					Scrollable: true, ScrollMode: ScrollHorizontalOnly, ID: "under",
					Width: 400, Height: 200, Axis: axisLeftToRight, shapeClip: clip,
				},
				Children: []Layout{{Shape: &Shape{shapeType: shapeRectangle, Width: 1000, Height: 200}}},
			},
			{
				Shape: &Shape{
					Scrollable: true, ScrollMode: ScrollVerticalOnly, ID: "over",
					Width: 400, Height: 200, Axis: axisTopToBottom, shapeClip: clip,
				},
				Children: []Layout{{Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 1000}}},
			},
		},
	}
	w.viewState.gesture.nowFn = fixedClock(0)
	panAxisSwipe(w, 200, 150, -6, -6, 12)
	if v, _ := w.scrollY().Get("over"); v >= 0 {
		t.Fatalf("list offset = %v, want the pan to scroll the list on top", v)
	}
	if v, _ := w.scrollX().Get("under"); v != 0 {
		t.Fatalf("table offset = %v, want the layer under the list left alone", v)
	}
}

// TestTouchPanAfterPinchScrollsBothAxes checks the finger left after a
// pinch pans free: a first pan locked vertical, so the transition must
// drop that lock, or the diagonal pan after the pinch would scroll only
// vertically.
func TestTouchPanAfterPinchScrollsBothAxes(t *testing.T) {
	w := &Window{}
	w.animations = make(map[string]Animation)
	w.layout = Layout{
		Shape: &Shape{
			Width: 400, Height: 400,
			shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
		},
		Children: []Layout{{
			Shape: &Shape{
				Scrollable: true, ID: "grid", Width: 400, Height: 400,
				Axis:      axisTopToBottom,
				shapeClip: drawClip{X: 0, Y: 0, Width: 400, Height: 400},
			},
			Children: []Layout{{Shape: &Shape{shapeType: shapeRectangle, Width: 1000, Height: 1000}}},
		}},
	}
	gs := &w.viewState.gesture
	at := func(tm int64, e *Event) {
		gs.nowFn = fixedClock(tm)
		w.handleTouch(&w.layout, e)
	}
	// Finger 1 pans up: the pan locks vertical.
	at(0, touchEvent(EventTouchesBegan, 1, 200, 300))
	at(16_000_000, touchEvent(EventTouchesMoved, 1, 200, 280))
	if gs.panAxis != panAxisVertical {
		t.Fatalf("setup: pan axis = %v, want vertical", gs.panAxis)
	}
	// Finger 2 lands and spreads: a pinch. Then it lifts.
	at(32_000_000, twoTouchEvent(EventTouchesBegan, 1, 200, 280, 2, 200, 100))
	at(48_000_000, twoTouchEvent(EventTouchesMoved, 1, 200, 280, 2, 200, 60))
	if !gs.pinchBegan {
		t.Fatal("setup: no pinch")
	}
	at(64_000_000, &Event{
		Type: EventTouchesEnded, NumTouches: 1,
		Touches: [8]TouchPoint{{Identifier: 2, PosX: 200, PosY: 60,
			ToolType: TouchToolFinger, Changed: true}},
	})
	y0, _ := w.scrollY().Get("grid")
	// Finger 1 pans diagonally.
	for i := 1; i <= 8; i++ {
		at(64_000_000+int64(i)*16_000_000,
			touchEvent(EventTouchesMoved, 1, 200-6*float32(i), 280-6*float32(i)))
	}
	if x, _ := w.scrollX().Get("grid"); x > -40 {
		t.Fatalf("grid x offset = %v, want the pan after the pinch to scroll sideways", x)
	}
	if y, _ := w.scrollY().Get("grid"); y > y0-40 {
		t.Fatalf("grid y offset = %v (was %v), want the pan after the pinch to scroll vertically", y, y0)
	}
}

// TestTouchLockedPanOnStripScrollsPage checks a vertical swipe that drifts
// sideways on a horizontal DragScroll strip scrolls the page by the
// finger's vertical travel. The lock and the strip's own axis
// (dragPanOuterDeltas) agree: the page gets the vertical part only.
func TestTouchLockedPanOnStripScrollsPage(t *testing.T) {
	w := dragStripPage(t, true)
	stripSwipe(t, w, []float32{-2, -5, -10}, []float32{-12, -30, -60})
	// 60 px of finger, less the threshold distance.
	if v, _ := w.scrollY().Get("page"); v > -48 || v < -52 {
		t.Fatalf("page offset = %v, want about -50", v)
	}
	if v, _ := w.scrollX().Get("page"); v != 0 {
		t.Fatalf("page x offset = %v, want 0", v)
	}
}
