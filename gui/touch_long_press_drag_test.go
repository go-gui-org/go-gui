package gui

import (
	"fmt"
	"testing"
)

// longPressDragPage renders list on a page that scrolls vertically, the
// layout where a touch press waits for the finger (touch_press.go).
func longPressDragPage(t *testing.T, list View) (*Window, []*Layout) {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 600})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			ID: "page", Scrollable: true, Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				list,
				Rectangle(RectangleCfg{Sizing: FillFixed, Height: 2000}),
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

// touchDragAt sends one single-finger touch event at time t (nanos),
// then settles the frame.
func touchDragAt(w *Window, typ EventType, t int64, x, y float32) {
	w.viewState.gesture.nowFn = fixedClock(t)
	w.EventFn(touchEvent(typ, 1, x, y))
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

// A long press whose press takes no mouse lock keeps the finger as
// before: its moves neither scroll the list nor reach the widget.
func TestTouchLongPressWithoutDragMovesNothing(t *testing.T) {
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
	touchAt(w, EventTouchesEnded, 800_000_000, x, y-60)
	if v, _ := w.scrollY().Get("list"); v != 0 {
		t.Fatalf("list offset = %v, want 0 after a long press", v)
	}
	if clicks["b3"] != 1 {
		t.Fatalf("clicks = %v, want b3 once (the long press)", clicks)
	}
}
