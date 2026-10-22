package gui

import (
	"fmt"
	"slices"
	"testing"
)

// longPressDragPage renders list on a page that scrolls vertically, the
// layout where a touch press waits for the finger (touch_press.go).
func longPressDragPage(t *testing.T, list View) (*Window, []*Layout) {
	t.Helper()
	return longPressDragWindow(t, list, true)
}

// longPressDragWindow renders list on a page that scrolls when scrolls is
// set. On a page that does not, a touch press goes out at touch-down.
func longPressDragWindow(t *testing.T, list View, scrolls bool) (*Window, []*Layout) {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 600})
	w.TestRender(func(*Window) View {
		filler := float32(0)
		if scrolls {
			filler = 2000
		}
		return Column(ContainerCfg{
			ID: "page", Scrollable: scrolls, Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				list,
				Rectangle(RectangleCfg{Sizing: FillFixed, Height: filler}),
			},
		})
	})
	var rows []*Layout
	xfCollectRows(&w.layout, &rows)
	if len(rows) < 4 {
		t.Fatalf("found %d rows, want 4", len(rows))
	}
	return w, rows
}

// touchDragAt is touchAt, then a settled frame: a drag reads the rows
// the last move laid out.
func touchDragAt(w *Window, typ EventType, t int64, x, y float32) {
	touchAt(w, typ, t, x, y)
	w.settle()
}

// longPressHoldDrag holds the finger on from until the long press
// fires, then drags it to the lower part of to, still down. It returns
// where the finger is.
func longPressHoldDrag(t *testing.T, w *Window, from, to *Layout) (x, y float32) {
	t.Helper()
	sx, sy, ok := shapeHitPoint(from)
	if !ok {
		t.Fatal("source row has no hit point")
	}
	ey := to.Shape.Y + to.Shape.Height*0.75
	touchDragAt(w, EventTouchesBegan, 0, sx, sy)
	w.viewState.gesture.nowFn = fixedClock(600_000_000)
	gestureLongPressFired(nil, w)
	w.settle()
	const steps = 8
	for i := 1; i <= steps; i++ {
		touchDragAt(w, EventTouchesMoved, 600_000_000+int64(i)*16_000_000,
			sx, sy+(ey-sy)*float32(i)/steps)
	}
	return sx, ey
}

// longPressDrag is longPressHoldDrag, then the lift.
func longPressDrag(t *testing.T, w *Window, from, to *Layout) {
	t.Helper()
	x, y := longPressHoldDrag(t, w, from, to)
	touchDragAt(w, EventTouchesEnded, 900_000_000, x, y)
}

// On a page that scrolls, a vertical drag that starts on a reorderable
// row scrolls the page (touch_press.go). A long press first arms the
// drag, and the finger's moves after it reorder the row.
func TestTouchLongPressDragReorders(t *testing.T) {
	treeNodes := make([]TreeNodeCfg, 4)
	for i := range treeNodes {
		treeNodes[i] = TreeNodeCfg{ID: fmt.Sprintf("id-%d", i), Text: fmt.Sprintf("Name %d", i)}
	}
	for _, tc := range []struct {
		name string
		list func(onReorder func(string, string, EventCtx)) View
	}{
		{"list box", func(on func(string, string, EventCtx)) View {
			return ListBox(ListBoxCfg{
				ID: "lb", Data: listBoxTestData(4), Reorderable: true, OnReorder: on,
			})
		}},
		{"tree", func(on func(string, string, EventCtx)) View {
			return Tree(TreeCfg{
				ID: "tree", Nodes: treeNodes, Reorderable: true, OnReorder: on,
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var moved, before string
			w, rows := longPressDragPage(t, tc.list(func(m, b string, _ EventCtx) {
				moved, before = m, b
			}))
			longPressDrag(t, w, rows[0], rows[2])
			if moved != "id-0" || before != "id-3" {
				t.Fatalf("OnReorder(%q, %q), want (\"id-0\", \"id-3\")", moved, before)
			}
			if v, _ := w.scrollY().Get("page"); v != 0 {
				t.Fatalf("page offset = %v, want 0: the drag must not scroll", v)
			}
		})
	}
}

// A drag the finger never released is cancelled, not dropped: a touch
// the platform cancels, or one a second finger takes over, must not
// reorder. A drop would commit the row where the finger happened to be.
func TestTouchLongPressDragAbortDoesNotReorder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		abort func(w *Window, x, y float32)
	}{
		{"cancelled", func(w *Window, x, y float32) {
			touchDragAt(w, EventTouchesCancelled, 900_000_000, x, y)
		}},
		{"second finger", func(w *Window, x, y float32) {
			w.viewState.gesture.nowFn = fixedClock(900_000_000)
			e := twoTouchEvent(EventTouchesBegan, 1, x, y, 2, x+80, y)
			e.Touches[0].Changed = false
			w.EventFn(e)
			w.settle()
			touchDragAt(w, EventTouchesEnded, 1_000_000_000, x, y)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reordered := false
			w, rows := longPressDragPage(t, ListBox(ListBoxCfg{
				ID: "lb", Data: listBoxTestData(4), Reorderable: true,
				OnReorder: func(string, string, EventCtx) { reordered = true },
			}))
			x, y := longPressHoldDrag(t, w, rows[0], rows[2])
			if !w.mouseIsLocked() {
				t.Fatal("the drag holds no mouse lock before the abort")
			}
			tc.abort(w, x, y)
			if reordered {
				t.Fatal("an aborted touch reordered the list")
			}
			if w.mouseIsLocked() {
				t.Fatal("the mouse is still locked after the abort")
			}
		})
	}
}

// A long press whose press takes no mouse lock lets the finger go: a
// finger that rested on a row and then drags scrolls the list (#1117).
// The row's press went out when the long press fired, and the pan
// releases it.
func TestTouchLongPressThenPanScrolls(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 40, clicks)
	x, y := rowCenter(t, w, "b3")
	touchAt(w, EventTouchesBegan, 0, x, y)
	w.viewState.gesture.nowFn = fixedClock(600_000_000)
	gestureLongPressFired(nil, w)
	if w.mouseIsLocked() {
		t.Fatal("a button's press took the mouse lock")
	}
	for i, dy := range []float32{-20, -40, -60} {
		touchAt(w, EventTouchesMoved, 600_000_000+int64(i+1)*16_000_000, x, y+dy)
	}
	if v, _ := w.scrollY().Get("list"); v >= 0 {
		t.Fatalf("list offset = %v, want it scrolled while the finger is down", v)
	}
	if w.viewState.mouseButtonsHeld != 0 {
		t.Fatal("the long press's press is still held after the pan took over")
	}
	touchAt(w, EventTouchesEnded, 800_000_000, x, y-60)
	if clicks["b3"] != 1 {
		t.Fatalf("clicks = %v, want b3 once (the long press)", clicks)
	}
}

// longPressGestureWindow runs the frame pipeline over a scrollable list
// "list" of rows whose OnGesture records each long-press phase in
// phases, and consumes the long press when take is set.
func longPressGestureWindow(t *testing.T, take bool, phases *[]GesturePhase) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		rows := make([]View, 0, 40)
		for i := range 40 {
			rows = append(rows, Button(ButtonCfg{
				ID: fmt.Sprintf("b%d", i), Sizing: FillFit, Label: "row",
			}))
		}
		return Column(ContainerCfg{
			ID: "list", Scrollable: true, Sizing: FillFill, Content: rows,
			OnGesture: func(ctx EventCtx) {
				if ctx.Event.GestureType != GestureLongPress {
					return
				}
				*phases = append(*phases, ctx.Event.GesturePhase)
				if take {
					ctx.Consume()
				}
			},
		})
	}
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	return w
}

// A long press that lets the finger go to a pan is cancelled, not ended.
// One an OnGesture handler consumed keeps the finger: the drag scrolls
// nothing and the lift ends it. A wobble under the pan distance after
// the long press keeps it either way.
func TestTouchLongPressYieldsUnlessTaken(t *testing.T) {
	for _, tc := range []struct {
		name     string
		take     bool
		travel   float32
		phases   []GesturePhase
		scrolled bool
	}{
		{"untaken drag", false, -60, []GesturePhase{GesturePhaseBegan, GesturePhaseCancelled}, true},
		{"untaken wobble", false, -6, []GesturePhase{GesturePhaseBegan, GesturePhaseEnded}, false},
		{"taken drag", true, -60, []GesturePhase{GesturePhaseBegan, GesturePhaseEnded}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var phases []GesturePhase
			w := longPressGestureWindow(t, tc.take, &phases)
			x, y := rowCenter(t, w, "b3")
			touchAt(w, EventTouchesBegan, 0, x, y)
			w.viewState.gesture.nowFn = fixedClock(600_000_000)
			gestureLongPressFired(nil, w)
			for i := 1; i <= 3; i++ {
				touchAt(w, EventTouchesMoved, 600_000_000+int64(i)*16_000_000,
					x, y+tc.travel*float32(i)/3)
			}
			touchAt(w, EventTouchesEnded, 800_000_000, x, y+tc.travel)
			if !slices.Equal(phases, tc.phases) {
				t.Fatalf("long-press phases = %v, want %v", phases, tc.phases)
			}
			v, _ := w.scrollY().Get("list")
			if scrolled := v < 0; scrolled != tc.scrolled {
				t.Fatalf("list offset = %v, scrolled = %v, want %v", v, scrolled, tc.scrolled)
			}
		})
	}
}

// On a page that does not scroll, a reorderable row's press goes out at
// touch-down and its drag takes the lock then. A finger that rests past
// the long press before it drags must still reorder the row.
func TestTouchLongPressDragReordersWithoutScroll(t *testing.T) {
	var moved, before string
	w, rows := longPressDragWindow(t, ListBox(ListBoxCfg{
		ID: "lb", Data: listBoxTestData(4), Reorderable: true,
		OnReorder: func(m, b string, _ EventCtx) { moved, before = m, b },
	}), false)
	longPressDrag(t, w, rows[0], rows[2])
	if moved != "id-0" || before != "id-3" {
		t.Fatalf("OnReorder(%q, %q), want (\"id-0\", \"id-3\")", moved, before)
	}
}

// A lock this touch's press took at touch-down gets the finger's moves
// after a long press: an app's own drag, held and then dragged, drives
// its drag, as a reorder row does.
func TestTouchLongPressMovesReachTouchDownLock(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 4, clicks)
	x, y := rowCenter(t, w, "outside")
	touchAt(w, EventTouchesBegan, 0, x, y)
	// Stands in for an app drag that locks the mouse on its press, which
	// went out at touch-down: "outside" is not in a scrollable.
	moves := 0
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) { moves++ }})
	w.viewState.gesture.nowFn = fixedClock(600_000_000)
	gestureLongPressFired(nil, w)
	for i, dy := range []float32{-20, -40, -60} {
		touchAt(w, EventTouchesMoved, 600_000_000+int64(i+1)*16_000_000, x, y+dy)
	}
	if moves != 3 {
		t.Fatalf("touch-down lock got %d moves after the long press, want 3", moves)
	}
}

// A lock already held when the finger lands is another drag's: it gets
// none of this finger's moves after a long press.
func TestTouchLongPressMovesSkipForeignLock(t *testing.T) {
	clicks := map[string]int{}
	w := touchPressWindow(t, 4, clicks)
	x, y := rowCenter(t, w, "outside")
	moves := 0
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) { moves++ }})
	touchAt(w, EventTouchesBegan, 0, x, y)
	w.viewState.gesture.nowFn = fixedClock(600_000_000)
	gestureLongPressFired(nil, w)
	for i, dy := range []float32{-20, -40, -60} {
		touchAt(w, EventTouchesMoved, 600_000_000+int64(i+1)*16_000_000, x, y+dy)
	}
	if moves != 0 {
		t.Fatalf("foreign lock got %d moves after the long press, want 0", moves)
	}
}

// A lock already held when the finger lands belongs to another drag. An
// abort of this touch must not cancel it: it gets the release, as before.
func TestTouchAbortSparesForeignLock(t *testing.T) {
	for _, tc := range touchAborts {
		t.Run(tc.name, func(t *testing.T) {
			clicks := map[string]int{}
			w := touchPressWindow(t, 4, clicks)
			x, y := rowCenter(t, w, "outside")
			cancelled, ups := 0, 0
			w.MouseLock(MouseLockCfg{
				MouseUp: func(EventCtx) { ups++ },
				Cancel:  func(*Window) { cancelled++ },
			})
			touchAt(w, EventTouchesBegan, 0, x, y)
			tc.abort(w, x, y)
			if cancelled != 0 {
				t.Fatalf("Cancel ran %d times on a lock this touch did not take", cancelled)
			}
			if ups != 1 {
				t.Fatalf("MouseUp ran %d times, want 1", ups)
			}
		})
	}
}

// touchAborts are the two ways a touch ends without a lift: the platform
// cancels it, or a second finger takes it over.
var touchAborts = []struct {
	name  string
	abort func(w *Window, x, y float32)
}{
	{"cancelled", func(w *Window, x, y float32) {
		touchAt(w, EventTouchesCancelled, 100_000_000, x, y)
	}},
	{"second finger", func(w *Window, x, y float32) {
		w.viewState.gesture.nowFn = fixedClock(100_000_000)
		e := twoTouchEvent(EventTouchesBegan, 1, x, y, 2, x+80, y)
		e.Touches[0].Changed = false
		w.handleTouch(&w.layout, e)
	}},
}

// A dock drag the touch's press started is cancelled on an abort, not
// dropped: a drop would dock the panel where the finger happened to be.
// The lift is run too, so the contrast, not only the silence, is
// asserted.
func TestTouchAbortDoesNotDockPanel(t *testing.T) {
	setup := func(t *testing.T, dropped *bool) *Window {
		w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
		w.layout = Layout{Shape: &Shape{ID: "dock"}}
		touchAt(w, EventTouchesBegan, 0, 50, 50)
		// The press takes the lock after touch-down, as a tab's would.
		root := DockPanelGroup("g1", []string{"p1", "p2"}, "p1")
		dockDragStart("dock", "p1", "g1", root,
			func(*DockNode, EventCtx) { *dropped = true },
			SoundNone,
			&Layout{Shape: &Shape{ID: "tab"}}, &Event{}, w)
		state := dockDragGet(w, "dock")
		state.active = true
		state.hoverZone = dockDropRight
		state.hoverGroupID = "g1"
		dockDragSet(w, "dock", state)
		return w
	}

	liftDropped := false
	wLift := setup(t, &liftDropped)
	touchAt(wLift, EventTouchesEnded, 100_000_000, 50, 50)
	if !liftDropped {
		t.Fatal("setup does not drop on a lift; the abort cases prove nothing")
	}

	for _, tc := range touchAborts {
		t.Run(tc.name, func(t *testing.T) {
			dropped := false
			w := setup(t, &dropped)
			tc.abort(w, 50, 50)
			if dropped {
				t.Fatal("an aborted touch docked the panel")
			}
			if w.mouseIsLocked() {
				t.Fatal("the mouse is still locked after the abort")
			}
			if dockDragGet(w, "dock").active {
				t.Fatal("drag state left active; the drop-zone overlay would persist")
			}
		})
	}
}

// A splitter handle pressed by a touch runs its Cancel hook on an abort:
// the lock goes and the handle drops its pressed color.
func TestTouchAbortReleasesSplitterHandle(t *testing.T) {
	for _, tc := range touchAborts {
		t.Run(tc.name, func(t *testing.T) {
			a, b := splitterTwoPanes()
			h := newSplitterHarness(t, SplitterCfg{ID: "sp", First: a, Second: b})
			_, handle, _ := h.parts(t, "sp")
			x, y := center(handle)
			touchDragAt(h.w, EventTouchesBegan, 0, x, y)
			_, handle, _ = h.parts(t, "sp")
			if handle.Shape.Color != defaultSplitterStyle.ColorsHandle.Click {
				t.Fatalf("handle color after touch = %v, want active %v",
					handle.Shape.Color, defaultSplitterStyle.ColorsHandle.Click)
			}
			tc.abort(h.w, x, y)
			h.w.settle()
			if h.w.mouseIsLocked() {
				t.Fatal("the mouse is still locked after the abort")
			}
			_, handle, _ = h.parts(t, "sp")
			if handle.Shape.Color != defaultSplitterStyle.ColorsHandle.Base {
				t.Fatalf("handle color after abort = %v, want base %v",
					handle.Shape.Color, defaultSplitterStyle.ColorsHandle.Base)
			}
		})
	}
}
