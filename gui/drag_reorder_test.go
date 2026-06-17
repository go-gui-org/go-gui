package gui

import "testing"

func TestReorderIndices(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}

	from, to := ReorderIndices(ids, "c", "b")
	if from != 2 || to != 1 {
		t.Errorf("move c before b: got (%d,%d) want (2,1)", from, to)
	}

	from, to = ReorderIndices(ids, "b", "d")
	if from != 1 || to != 2 {
		t.Errorf("move b before d: got (%d,%d) want (1,2)", from, to)
	}

	from, to = ReorderIndices(ids, "b", "")
	if from != 1 || to != 3 {
		t.Errorf("move b to end: got (%d,%d) want (1,3)", from, to)
	}

	// No-op: b before c is the same position.
	from, to = ReorderIndices(ids, "b", "c")
	if from != -1 || to != -1 {
		t.Errorf("no-op b before c: got (%d,%d) want (-1,-1)", from, to)
	}

	// Not found: z.
	from, to = ReorderIndices(ids, "z", "a")
	if from != -1 || to != -1 {
		t.Errorf("missing z: got (%d,%d) want (-1,-1)", from, to)
	}

	// Missing before_id.
	from, to = ReorderIndices(ids, "b", "missing")
	if from != -1 || to != -1 {
		t.Errorf("missing before: got (%d,%d) want (-1,-1)", from, to)
	}
}

func TestDragReorderCalcIndex(t *testing.T) {
	// 5 items, each 20px, source at index 2 (item_start=40).
	idx := dragReorderCalcIndex(45, 40, 20, 2, 5)
	if idx != 2 {
		t.Errorf("mid-item: got %d want 2", idx)
	}

	// Before first item.
	idx = dragReorderCalcIndex(-5, 40, 20, 2, 5)
	if idx != 0 {
		t.Errorf("before first: got %d want 0", idx)
	}

	// After last item.
	idx = dragReorderCalcIndex(200, 40, 20, 2, 5)
	if idx != 5 {
		t.Errorf("after last: got %d want 5", idx)
	}

	// Single item.
	idx = dragReorderCalcIndex(50, 0, 20, 0, 1)
	if idx != 0 {
		t.Errorf("single item: got %d want 0", idx)
	}
}

func TestDragReorderCalcIndexFromMids(t *testing.T) {
	mids := []float32{5, 25, 45}

	idx, ok := dragReorderCalcIndexFromMids(6, mids)
	if !ok || idx != 1 {
		t.Errorf("after first mid: got (%d,%v) want (1,true)", idx, ok)
	}

	idx, ok = dragReorderCalcIndexFromMids(26, mids)
	if !ok || idx != 2 {
		t.Errorf("after second mid: got (%d,%v) want (2,true)", idx, ok)
	}

	idx, ok = dragReorderCalcIndexFromMids(90, mids)
	if !ok || idx != 3 {
		t.Errorf("past all mids: got (%d,%v) want (3,true)", idx, ok)
	}

	_, ok = dragReorderCalcIndexFromMids(10, nil)
	if ok {
		t.Error("empty mids should return false")
	}
}

func TestDragReorderIDsSignature(t *testing.T) {
	a := dragReorderIDsSignature([]string{"a", "b", "c"})
	b := dragReorderIDsSignature([]string{"a", "b", "c"})
	if a != b {
		t.Error("same input should produce same hash")
	}

	c := dragReorderIDsSignature([]string{"a", "c"})
	if a == c {
		t.Error("different input should produce different hash")
	}
}

func TestDragReorderItemMidsFromLayouts(t *testing.T) {
	w := &Window{}
	w.layout = Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "a", X: 0, Y: 0, Width: 100, Height: 10}},
			{Shape: &Shape{ID: "b", X: 0, Y: 10, Width: 100, Height: 30}},
			{Shape: &Shape{ID: "c", X: 0, Y: 40, Width: 100, Height: 10}},
		},
	}
	mids, ok := dragReorderItemMidsFromLayouts(
		DragReorderVertical, []string{"a", "b", "c"}, w)
	if !ok || len(mids) != 3 {
		t.Fatalf("expected 3 mids, got %d ok=%v", len(mids), ok)
	}
	if mids[0] != 5 || mids[1] != 25 || mids[2] != 45 {
		t.Errorf("mids = %v, want [5 25 45]", mids)
	}
}

func TestDragReorderItemMidsFromLayoutsMissing(t *testing.T) {
	w := &Window{}
	w.layout = Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "a", Width: 10, Height: 10}},
		},
	}
	_, ok := dragReorderItemMidsFromLayouts(
		DragReorderVertical, []string{"a", "missing"}, w)
	if ok {
		t.Error("expected false for missing layout ID")
	}
}

func TestDragReorderEscapeCancelsStartedDrag(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_escape"
	dragReorderSet(w, dragKey, dragReorderState{started: true})

	handled := dragReorderEscape(dragKey, KeyEscape, w)
	if !handled {
		t.Error("escape should be handled")
	}
	state := dragReorderGet(w, dragKey)
	if state.started {
		t.Error("state should be cleared after escape")
	}
}

func TestDragReorderStartSetsLayoutValidity(t *testing.T) {
	w := &Window{}
	w.layout = Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "a", X: 0, Y: 0, Width: 10, Height: 10}},
			{Shape: &Shape{ID: "b", X: 0, Y: 10, Width: 10, Height: 10}},
		},
	}
	parent := &Layout{
		Shape: &Shape{
			ID: "parent", X: 0, Y: 0,
			Width: 100, Height: 100,
		},
	}
	item := &Layout{
		Shape:  &Shape{ID: "a", X: 0, Y: 0, Width: 10, Height: 10},
		Parent: parent,
	}
	e := &Event{MouseX: 1, MouseY: 1}
	noop := func(string, string, *Window) {}

	dragKeyOK := "drag_layout_ok"
	dragReorderStart(dragReorderStartCfg{
		DragKey: dragKeyOK, Index: 0, ItemID: "a",
		Axis: DragReorderVertical, ItemIDs: []string{"a", "b"},
		OnReorder: noop, ItemLayoutIDs: []string{"a", "b"},
		Layout: item, Event: e,
	}, w)
	stateOK := dragReorderGet(w, dragKeyOK)
	if !stateOK.started || !stateOK.layoutsValid {
		t.Error("valid layouts should set layoutsValid=true")
	}

	w.MouseUnlock()
	dragKeyMissing := "drag_layout_missing"
	dragReorderStart(dragReorderStartCfg{
		DragKey: dragKeyMissing, Index: 0, ItemID: "a",
		Axis: DragReorderVertical, ItemIDs: []string{"a", "b"},
		OnReorder: noop, ItemLayoutIDs: []string{"a", "missing"},
		Layout: item, Event: e,
	}, w)
	stateMissing := dragReorderGet(w, dragKeyMissing)
	if !stateMissing.started || stateMissing.layoutsValid {
		t.Error("missing layout should set layoutsValid=false")
	}
}

func TestDragReorderKeyboardMoveRequiresAlt(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	called := false
	handled := dragReorderKeyboardMove(
		KeyDown, ModNone, DragReorderVertical, 1,
		[]string{"a", "b", "c"},
		func(string, string, *Window) { called = true }, w)
	if handled || called {
		t.Error("should not handle without Alt modifier")
	}
}

func TestDragReorderKeyboardMovePayloadAndBoundary(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	var moved, before string
	called := false
	handled := dragReorderKeyboardMove(
		KeyRight, ModAlt, DragReorderHorizontal, 1,
		[]string{"a", "b", "c", "d"},
		func(m, b string, _ *Window) {
			called = true
			moved = m
			before = b
		}, w)
	if !handled || !called {
		t.Error("Alt+Right should be handled")
	}
	if moved != "b" || before != "d" {
		t.Errorf("got (%q,%q) want (b,d)", moved, before)
	}

	// Boundary: Alt+Left at index 0 is a no-op.
	boundaryCalled := false
	handled = dragReorderKeyboardMove(
		KeyLeft, ModAlt, DragReorderHorizontal, 0,
		[]string{"a", "b", "c"},
		func(string, string, *Window) { boundaryCalled = true }, w)
	if handled || boundaryCalled {
		t.Error("Alt+Left at 0 should be a no-op")
	}
}

func TestDragReorderCalcIndexWithScrollDelta(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_scroll"
	idScroll := uint32(100)

	sy := w.scrollY()
	sy.Set(idScroll, -10.0)

	state := dragReorderState{
		active:       true,
		itemY:        100.0,
		itemHeight:   20.0,
		sourceIndex:  0,
		itemCount:    5,
		idScroll:     idScroll,
		startScrollY: -10.0,
	}
	dragReorderSet(w, dragKey, state)

	// Container scrolls down to -20 (delta = -10).
	sy.Set(idScroll, -20.0)

	dragReorderOnMouseMove(dragKey, DragReorderVertical, 0, 115, w)

	newState := dragReorderGet(w, dragKey)
	if newState.currentIndex != 1 {
		t.Errorf("scroll-adjusted index: got %d want 1",
			newState.currentIndex)
	}
}

func TestDragReorderAutoScrollTimerActivation(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_timer"
	idScroll := uint32(100)

	state := dragReorderState{
		active:         true,
		itemY:          0.0,
		itemHeight:     20.0,
		sourceIndex:    0,
		itemCount:      5,
		idScroll:       idScroll,
		containerStart: 0.0,
		containerEnd:   100.0,
	}
	dragReorderSet(w, dragKey, state)

	// Mouse near start edge (within scroll zone).
	dragReorderOnMouseMove(dragKey, DragReorderVertical, 0, 5, w)

	newState := dragReorderGet(w, dragKey)
	if !newState.scrollTimerActive {
		t.Error("scroll timer should be active")
	}
	if !w.HasAnimation(dragReorderScrollAnimID) {
		t.Error("scroll animation should exist")
	}

	// Mouse moves away from scroll zone.
	dragReorderOnMouseMove(dragKey, DragReorderVertical, 0, 50, w)
	stateAfter := dragReorderGet(w, dragKey)
	if stateAfter.scrollTimerActive {
		t.Error("scroll timer should be inactive")
	}
	if w.HasAnimation(dragReorderScrollAnimID) {
		t.Error("scroll animation should be removed")
	}
}

func TestDragReorderCancelsOnMidDragMutation(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	parent := &Layout{
		Shape: &Shape{
			ID: "parent", X: 0, Y: 0,
			Width: 100, Height: 100,
		},
	}
	item := &Layout{
		Shape:  &Shape{ID: "a", X: 0, Y: 0, Width: 10, Height: 10},
		Parent: parent,
	}
	e := &Event{MouseX: 1, MouseY: 1}
	dragKey := "drag_mutation"
	called := false
	dragReorderStart(dragReorderStartCfg{
		DragKey: dragKey, Index: 0, ItemID: "a",
		Axis: DragReorderVertical, ItemIDs: []string{"a", "b", "c"},
		OnReorder:     func(string, string, *Window) { called = true },
		ItemLayoutIDs: []string{"a", "b", "c"},
		Layout:        item, Event: e,
	}, w)
	dragReorderIDsMetaSet(w, dragKey, []string{"a", "b", "c"})

	// Simulate list mutation before mouse-up.
	dragReorderIDsMetaSet(w, dragKey, []string{"a", "c"})
	dragReorderOnMouseUp(dragKey, []string{"a", "c"},
		func(string, string, *Window) { called = true }, w)
	if called {
		t.Error("callback should not fire on mutation")
	}
	state := dragReorderGet(w, dragKey)
	if state.started || state.active {
		t.Error("state should be cleared after mutation cancel")
	}
}

func TestDragReorderCancelsOnMidDragMoveMutation(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_move_mutation"

	state := dragReorderState{
		active:       true,
		sourceIndex:  0,
		currentIndex: 0,
		itemCount:    3,
		idsLen:       3,
		idsHash:      dragReorderIDsSignature([]string{"a", "b", "c"}),
	}
	dragReorderSet(w, dragKey, state)
	dragReorderIDsMetaSet(w, dragKey, []string{"a", "b", "c"})

	// Mutate IDs before move.
	dragReorderIDsMetaSet(w, dragKey, []string{"a", "c"})
	dragReorderOnMouseMove(
		dragKey, DragReorderVertical, 0, 0, w)

	newState := dragReorderGet(w, dragKey)
	if newState.started || newState.active {
		t.Error("state should be cleared after move mutation")
	}
}

func TestDragReorderGhostViewOffset(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	state := dragReorderState{
		startMouseX: 50, startMouseY: 100,
		mouseX: 70, mouseY: 130,
		itemX: 10, itemY: 80,
		itemWidth: 200, itemHeight: 30,
		parentX: 5, parentY: 5,
	}
	ghost := dragReorderGhostView(state, Rectangle(RectangleCfg{}))
	ly := generateViewLayout(ghost, w)

	// ghostX = mouseX - (startMouseX - itemX) = 70 - (50-10) = 30
	// floatOffsetX = ghostX - parentX = 30 - 5 = 25
	wantOffX := float32(25)
	if ly.Shape.FloatOffsetX != wantOffX {
		t.Errorf("FloatOffsetX = %v, want %v",
			ly.Shape.FloatOffsetX, wantOffX)
	}

	// ghostY = mouseY - (startMouseY - itemY) = 130 - (100-80) = 110
	// floatOffsetY = ghostY - parentY = 110 - 5 = 105
	wantOffY := float32(105)
	if ly.Shape.FloatOffsetY != wantOffY {
		t.Errorf("FloatOffsetY = %v, want %v",
			ly.Shape.FloatOffsetY, wantOffY)
	}

	if ly.Shape.Width != 200 || ly.Shape.Height != 30 {
		t.Errorf("ghost size = %vx%v, want 200x30",
			ly.Shape.Width, ly.Shape.Height)
	}
}

func TestDragReorderGapViewSizing(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	state := dragReorderState{
		itemWidth: 120, itemHeight: 40,
	}

	vGap := dragReorderGapView(state, DragReorderVertical)
	vLy := generateViewLayout(vGap, w)
	if vLy.Shape.Width != 120 || vLy.Shape.Height != 40 {
		t.Errorf("vertical gap = %vx%v, want 120x40",
			vLy.Shape.Width, vLy.Shape.Height)
	}
	if vLy.Shape.Sizing != FillFixed {
		t.Errorf("vertical gap sizing = %v, want FillFixed",
			vLy.Shape.Sizing)
	}

	hGap := dragReorderGapView(state, DragReorderHorizontal)
	hLy := generateViewLayout(hGap, w)
	if hLy.Shape.Width != 120 || hLy.Shape.Height != 40 {
		t.Errorf("horizontal gap = %vx%v, want 120x40",
			hLy.Shape.Width, hLy.Shape.Height)
	}
	if hLy.Shape.Sizing != FixedFit {
		t.Errorf("horizontal gap sizing = %v, want FixedFit",
			hLy.Shape.Sizing)
	}
}

func TestDragReorderScrollChangeUsesUniformEstimate(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_scroll_uniform"
	idScroll := uint32(200)

	sy := w.scrollY()
	sy.Set(idScroll, 10.0)

	state := dragReorderState{
		active:       true,
		itemY:        0.0,
		itemHeight:   10.0,
		sourceIndex:  0,
		itemCount:    5,
		idScroll:     idScroll,
		startScrollY: 0.0,
		itemMids:     []float32{25, 35},
		midsOffset:   2,
		layoutsValid: true,
	}
	dragReorderSet(w, dragKey, state)

	// With scroll change, mids are invalid; uniform should yield 0.
	dragReorderOnMouseMove(
		dragKey, DragReorderVertical, 0, 15, w)

	newState := dragReorderGet(w, dragKey)
	if newState.currentIndex != 0 {
		t.Errorf("uniform fallback: got %d want 0",
			newState.currentIndex)
	}
}
