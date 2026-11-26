package gui

import "testing"

// Issue #1359: under RTL the tab strip runs right to left, but the drag
// worked out the drop slot as if it ran left to right, so a tab dragged
// past its neighbours did not move. The arrow keys had the same fault:
// Left went to the previous tab, which under RTL sits to the right.
// The drop slot follows the pointer and the arrows go the way they point,
// in both directions.

// tabRTLLocale sets the locale direction for one test and puts the old
// locale back after it.
func tabRTLLocale(t *testing.T, dir TextDirection) {
	t.Helper()
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	loc := saved
	loc.TextDir = dir
	SetLocale(loc)
}

// tabRTLView is a reorderable strip of three tabs with "b" selected. Each
// callback appends to *got, so a test reads what happened as one string.
func tabRTLView(got *string) func(*Window) View {
	return func(*Window) View {
		return TabControl(TabControlCfg{
			ID: "tc", Reorderable: true, Focusable: true, Selected: "b",
			Sizing: FillFill,
			Items: []TabItemCfg{
				{ID: "a", Label: "Aaaa"},
				{ID: "b", Label: "Bbbb"},
				{ID: "c", Label: "Cccc"},
			},
			OnSelect: func(id string, _ EventCtx) { *got += " select " + id },
			OnReorder: func(id, before string, _ EventCtx) {
				*got += " move " + id + " before [" + before + "]"
			},
		})
	}
}

func TestTabControlDragReorderFollowsDirection(t *testing.T) {
	for _, tc := range []struct {
		dir    TextDirection
		target string
		want   string
	}{
		// `before []` means at the end. The press selects the tab.
		{TextDirLTR, "b", " select a move a before [c]"},
		{TextDirLTR, "c", " select a move a before []"},
		{TextDirRTL, "b", " select a move a before [c]"},
		{TextDirRTL, "c", " select a move a before []"},
	} {
		name := "LTR"
		// past is a step just past a tab's middle, in reading order.
		past := float32(10)
		if tc.dir == TextDirRTL {
			name, past = "RTL", -10
		}
		t.Run(name+" past "+tc.target, func(t *testing.T) {
			tabRTLLocale(t, tc.dir)
			var got string
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 200})
			w.TestRender(tabRTLView(&got))
			l := w.TestRender(nil)
			mid := func(id string) (x, y float32) {
				s, ok := l.FindByID(ScopeID("tc", "tab", id))
				if !ok {
					t.Fatalf("tab %q not found", id)
				}
				return s.Shape.X + s.Shape.Width/2, s.Shape.Y + s.Shape.Height/2
			}
			from, y := mid("a")
			to, _ := mid(tc.target)
			send := func(ty EventType, x float32) {
				w.EventFn(&Event{Type: ty, MouseX: x, MouseY: y, MouseButton: MouseLeft})
				w.TestRender(nil)
			}
			send(EventMouseDown, from)
			send(EventMouseMove, from+past)
			send(EventMouseMove, to+past)
			send(EventMouseUp, to+past)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTabControlArrowsFollowDirection(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  TextDirection
		key  KeyCode
		mod  Modifier
		want string
	}{
		{"LTR Left", TextDirLTR, KeyLeft, ModNone, " select a"},
		{"LTR Right", TextDirLTR, KeyRight, ModNone, " select c"},
		{"LTR Alt+Left", TextDirLTR, KeyLeft, ModAlt, " move b before [a]"},
		{"LTR Alt+Right", TextDirLTR, KeyRight, ModAlt, " move b before []"},
		// Under RTL "a" is the rightmost tab, so Right reaches it.
		{"RTL Left", TextDirRTL, KeyLeft, ModNone, " select c"},
		{"RTL Right", TextDirRTL, KeyRight, ModNone, " select a"},
		{"RTL Alt+Left", TextDirRTL, KeyLeft, ModAlt, " move b before []"},
		{"RTL Alt+Right", TextDirRTL, KeyRight, ModAlt, " move b before [a]"},
		// Up and Down follow reading order, not the screen.
		{"RTL Up", TextDirRTL, KeyUp, ModNone, " select a"},
		{"RTL Down", TextDirRTL, KeyDown, ModNone, " select c"},
		{"RTL Home", TextDirRTL, KeyHome, ModNone, " select a"},
		{"RTL End", TextDirRTL, KeyEnd, ModNone, " select c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tabRTLLocale(t, tc.dir)
			var got string
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 200})
			w.TestRender(tabRTLView(&got))
			w.TestRender(nil)
			if err := w.TestKey("tc", tc.key, tc.mod); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The segmented control shares the tab strip's key handler, so its
// arrows follow the direction too.
func TestSegmentedControlArrowsFollowDirection(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  TextDirection
		key  KeyCode
		want string
	}{
		{"LTR Left", TextDirLTR, KeyLeft, "a"},
		{"LTR Right", TextDirLTR, KeyRight, "c"},
		{"RTL Left", TextDirRTL, KeyLeft, "c"},
		{"RTL Right", TextDirRTL, KeyRight, "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tabRTLLocale(t, tc.dir)
			var got string
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 200})
			w.TestRender(func(*Window) View {
				return SegmentedControl(SegmentedControlCfg{
					ID: "seg", Value: "b", Items: []string{"a", "b", "c"},
					OnSelect: func(v string, _ EventCtx) { got = v },
				})
			})
			w.TestRender(nil)
			if err := w.TestKey("seg", tc.key, ModNone); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The uniform-size fallback, used when the item midpoints are not known,
// counts slots from the start edge of the row. Under RTL that is the
// right edge, which the caller gives by mirroring the axis.
func TestDragReorderCalcIndexMirrored(t *testing.T) {
	// Three 100 px items laid out right to left in [0, 300]: index 0 at
	// [200, 300], index 1 at [100, 200], index 2 at [0, 100]. The source
	// is index 0. Mirrored, its start is -300.
	for _, tc := range []struct {
		mouseX float32
		want   int
	}{{290, 0}, {150, 1}, {50, 2}, {-10, 3}} {
		got := dragReorderCalcIndex(-tc.mouseX, -300, 100, 0, 3)
		if got != tc.want {
			t.Errorf("mouseX %v: got %d, want %d", tc.mouseX, got, tc.want)
		}
	}
}

// Only a row that resolves RTL places items right to left. A column, an
// LTR row and an item with no parent do not mirror the drag.
func TestDragReorderRowRTL(t *testing.T) {
	rtl := &Shape{Axis: axisLeftToRight, TextDir: TextDirRTL}
	ltr := &Shape{Axis: axisLeftToRight, TextDir: TextDirLTR}
	col := &Shape{Axis: axisTopToBottom, TextDir: TextDirRTL}
	for _, tc := range []struct {
		name   string
		parent *Layout
		want   bool
	}{
		{"RTL row", &Layout{Shape: rtl}, true},
		{"LTR row", &Layout{Shape: ltr}, false},
		{"RTL column", &Layout{Shape: col}, false},
		{"parent without shape", &Layout{}, false},
		{"no parent", nil, false},
	} {
		item := &Layout{Shape: &Shape{}, Parent: tc.parent}
		if got := dragReorderRowRTL(item); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Without valid midpoints the move falls back to uniform slots. Mirrored,
// the slots count from the source item's right edge.
func TestDragReorderMoveMirroredFallback(t *testing.T) {
	// Three 100 px items right to left in [0, 300]; the source is index 0
	// at [200, 300].
	for _, tc := range []struct {
		mouseX float32
		want   int
	}{{150, 1}, {50, 2}} {
		w := NewTestWindow(t, WindowCfg{Width: 600, Height: 200})
		dragReorderSet(w, "k", dragReorderState{
			started: true, active: true, mirrored: true,
			sourceIndex: 0, currentIndex: 0, itemCount: 3,
			itemX: 200, itemWidth: 100, itemHeight: 20,
			startMouseX: 250, mouseX: 250,
		})
		dragReorderOnMouseMove("k", dragReorderHorizontal, tc.mouseX, 10, w)
		if got := dragReorderGet(w, "k").currentIndex; got != tc.want {
			t.Errorf("mouseX %v: got %d, want %d", tc.mouseX, got, tc.want)
		}
	}
}

// dragGhostX returns the X of the drag ghost in the last frame.
func dragGhostX(t *testing.T, w *Window) float32 {
	t.Helper()
	ly, ok := w.layout.findLayoutDepth(func(l Layout) bool {
		return l.Shape != nil && l.Shape.ID == "drag_reorder_ghost"
	}, 0)
	if !ok {
		t.Fatal("no drag ghost")
	}
	return ly.Shape.X
}

// Under RTL a float in an RTL parent is anchored to the parent's right
// edge and its X offset is mirrored. The ghost gave its offset from the
// left edge, so during a drag it was drawn on the far side of the strip,
// although the gap and the drop were right. The ghost keeps the grab
// point under the pointer in both directions, for a horizontal strip and
// for a vertical list.
func TestDragReorderGhostFollowsPointerRTL(t *testing.T) {
	for _, dir := range []TextDirection{TextDirLTR, TextDirRTL} {
		name := "LTR"
		if dir == TextDirRTL {
			name = "RTL"
		}
		t.Run(name+" tabs", func(t *testing.T) {
			tabRTLLocale(t, dir)
			var got string
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 200})
			w.TestRender(tabRTLView(&got))
			l := w.TestRender(nil)
			s, ok := l.FindByID(ScopeID("tc", "tab", "a"))
			if !ok {
				t.Fatal("tab a not found")
			}
			x, y := s.Shape.X+s.Shape.Width/2, s.Shape.Y+s.Shape.Height/2
			// A small step stays inside tab a, so no slot change and no
			// FLIP animation moves the ghost.
			step := float32(8)
			for _, ev := range []Event{
				{Type: EventMouseDown, MouseX: x, MouseY: y, MouseButton: MouseLeft},
				{Type: EventMouseMove, MouseX: x + step, MouseY: y},
				{Type: EventMouseMove, MouseX: x + 2*step, MouseY: y},
			} {
				w.EventFn(&ev)
				w.TestRender(nil)
			}
			want := s.Shape.X + 2*step
			if gx := dragGhostX(t, w); !xfNear(gx, want) {
				t.Fatalf("ghost X %v, want %v", gx, want)
			}
		})
		t.Run(name+" listbox", func(t *testing.T) {
			tabRTLLocale(t, dir)
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 600})
			w.TestRender(func(*Window) View {
				return ListBox(ListBoxCfg{
					ID: "lb", Data: listBoxTestData(4), Reorderable: true,
					Sizing:    FillFit,
					OnReorder: func(_, _ string, _ EventCtx) {},
				})
			})
			var rows []*Layout
			xfCollectRows(&w.layout, &rows)
			if len(rows) < 4 {
				t.Fatalf("found %d rows, want 4", len(rows))
			}
			row := rows[1].Shape
			x, y := row.X+20, row.Y+row.Height/2
			step := float32(3)
			for _, ev := range []Event{
				{Type: EventMouseDown, MouseX: x, MouseY: y, MouseButton: MouseLeft},
				{Type: EventMouseMove, MouseX: x + step, MouseY: y + step},
				{Type: EventMouseMove, MouseX: x + 2*step, MouseY: y + 2*step},
			} {
				w.EventFn(&ev)
				w.settle()
			}
			want := row.X + 2*step
			if gx := dragGhostX(t, w); !xfNear(gx, want) {
				t.Fatalf("ghost X %v, want %v", gx, want)
			}
		})
	}
}

// A drag records the item midpoints and the ghost anchor when it starts.
// A window resize moves the items (under RTL the strip sits against the
// right edge), so after a resize that geometry is stale: the ghost
// drifts from the pointer and the drop lands on the wrong slot. A resize
// that changes the size cancels the drag. One that keeps the size does
// not.
func TestDragReorderResizeCancelsDrag(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dir   TextDirection
		width int
		want  string
	}{
		{"LTR resized", TextDirLTR, 700, " select a"},
		{"RTL resized", TextDirRTL, 700, " select a"},
		{"RTL same size", TextDirRTL, 600, " select a move a before [c]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tabRTLLocale(t, tc.dir)
			var got string
			w := NewTestWindow(t, WindowCfg{Width: 600, Height: 200})
			w.TestRender(tabRTLView(&got))
			l := w.TestRender(nil)
			mid := func(id string) (x, y float32) {
				s, ok := l.FindByID(ScopeID("tc", "tab", id))
				if !ok {
					t.Fatalf("tab %q not found", id)
				}
				return s.Shape.X + s.Shape.Width/2, s.Shape.Y + s.Shape.Height/2
			}
			past := float32(10)
			if tc.dir == TextDirRTL {
				past = -10
			}
			from, y := mid("a")
			to, _ := mid("b")
			send := func(ty EventType, x float32) {
				w.EventFn(&Event{Type: ty, MouseX: x, MouseY: y, MouseButton: MouseLeft})
				w.TestRender(nil)
			}
			send(EventMouseDown, from)
			send(EventMouseMove, from+past)
			send(EventMouseMove, to+past)
			w.EventFn(&Event{Type: EventResized, WindowWidth: tc.width, WindowHeight: 200})
			w.TestRender(nil)
			send(EventMouseUp, to+past)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
