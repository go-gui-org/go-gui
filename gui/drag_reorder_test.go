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
		dragReorderVertical, []string{"a", "b", "c"}, w)
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
		dragReorderVertical, []string{"a", "missing"}, w)
	if ok {
		t.Error("expected false for missing layout ID")
	}
	_, ok = dragReorderItemMidsFromLayouts(
		dragReorderVertical, []string{"a", ""}, w)
	if ok {
		t.Error("expected false for empty layout ID")
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

// Issue #110: losing mouse capture mid-drag must unwind the reorder,
// not commit it. The same setup is run twice — once released, once
// cancelled — so the assertion is that the two differ, not just that
// the cancel path is quiet.
func TestDragReorderCaptureLossDoesNotReorder(t *testing.T) {
	setup := func(w *Window, dragKey string, fired *bool) {
		w.layout = Layout{Shape: &Shape{ID: "root"}}
		dragReorderStart(dragReorderStartCfg{
			DragKey: dragKey, Index: 0, ItemID: "a",
			Axis: dragReorderVertical, ItemIDs: []string{"a", "b", "c"},
			ItemLayoutIDs: []string{"a", "b", "c"},
			OnReorder:     func(string, string, EventCtx) { *fired = true },
			Layout:        &Layout{Shape: &Shape{ID: "a"}},
			Event:         &Event{},
		}, w)
		// A drag that has moved far enough to commit on release.
		state := dragReorderGet(w, dragKey)
		state.active = true
		state.currentIndex = 2
		dragReorderSet(w, dragKey, state)
	}

	wUp := &Window{}
	upFired := false
	setup(wUp, "drag_release", &upFired)
	mouseUpHandler(&Layout{Shape: &Shape{}}, &Event{}, wUp)
	if !upFired {
		t.Fatal("setup does not commit on release; the cancel case proves nothing")
	}

	w := &Window{}
	fired := false
	setup(w, "drag_capture_loss", &fired)
	w.MouseCancel()
	if fired {
		t.Error("OnReorder fired for a drag the user never released")
	}
	if w.mouseIsLocked() {
		t.Error("mouse still locked after capture loss")
	}
	if state := dragReorderGet(w, "drag_capture_loss"); state.active {
		t.Error("drag state left active; ghost and gap would keep rendering")
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
	noop := func(string, string, EventCtx) {}

	dragKeyOK := "drag_layout_ok"
	dragReorderStart(dragReorderStartCfg{
		DragKey: dragKeyOK, Index: 0, ItemID: "a",
		Axis: dragReorderVertical, ItemIDs: []string{"a", "b"},
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
		Axis: dragReorderVertical, ItemIDs: []string{"a", "b"},
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
		KeyDown, ModNone, dragReorderVertical, 1,
		[]string{"a", "b", "c"},
		func(string, string, EventCtx) { called = true },
		SoundNone, w)
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
		KeyRight, ModAlt, dragReorderHorizontal, 1,
		[]string{"a", "b", "c", "d"},
		func(m, b string, _ EventCtx) {
			called = true
			moved = m
			before = b
		}, SoundNone, w)
	if !handled || !called {
		t.Error("Alt+Right should be handled")
	}
	if moved != "b" || before != "d" {
		t.Errorf("got (%q,%q) want (b,d)", moved, before)
	}

	// Boundary: Alt+Left at index 0 is a no-op.
	boundaryCalled := false
	handled = dragReorderKeyboardMove(
		KeyLeft, ModAlt, dragReorderHorizontal, 0,
		[]string{"a", "b", "c"},
		func(string, string, EventCtx) { boundaryCalled = true },
		SoundNone, w)
	if handled || boundaryCalled {
		t.Error("Alt+Left at 0 should be a no-op")
	}
}

func TestDragReorderCalcIndexWithScrollDelta(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_scroll"
	idScroll := "100"

	sy := w.scrollY()
	sy.Set(idScroll, -10.0)

	state := dragReorderState{
		active:       true,
		itemY:        100.0,
		itemHeight:   20.0,
		sourceIndex:  0,
		itemCount:    5,
		scrollID:     idScroll,
		startScrollY: -10.0,
	}
	dragReorderSet(w, dragKey, state)

	// Container scrolls down to -20 (delta = -10).
	sy.Set(idScroll, -20.0)

	dragReorderOnMouseMove(dragKey, dragReorderVertical, 0, 115, w)

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
	idScroll := "100"

	state := dragReorderState{
		active:         true,
		itemY:          0.0,
		itemHeight:     20.0,
		sourceIndex:    0,
		itemCount:      5,
		scrollID:       idScroll,
		containerStart: 0.0,
		containerEnd:   100.0,
	}
	dragReorderSet(w, dragKey, state)

	// Mouse near start edge (within scroll zone).
	dragReorderOnMouseMove(dragKey, dragReorderVertical, 0, 5, w)

	newState := dragReorderGet(w, dragKey)
	if !newState.scrollTimerActive {
		t.Error("scroll timer should be active")
	}
	if !w.HasAnimation(dragReorderScrollAnimID) {
		t.Error("scroll animation should exist")
	}

	// Mouse moves away from scroll zone.
	dragReorderOnMouseMove(dragKey, dragReorderVertical, 0, 50, w)
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
		Axis: dragReorderVertical, ItemIDs: []string{"a", "b", "c"},
		OnReorder:     func(_ string, _ string, ctx EventCtx) { called = true },
		ItemLayoutIDs: []string{"a", "b", "c"},
		Layout:        item, Event: e,
	}, w)
	dragReorderIDsMetaSet(w, dragKey, []string{"a", "b", "c"})

	// Simulate list mutation before mouse-up.
	dragReorderIDsMetaSet(w, dragKey, []string{"a", "c"})
	dragReorderOnMouseUp(dragKey, []string{"a", "c"},
		func(string, string, EventCtx) { called = true },
		w)
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
		dragKey, dragReorderVertical, 0, 0, w)

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

	vGap := dragReorderGapView(state, dragReorderVertical)
	vLy := generateViewLayout(vGap, w)
	if vLy.Shape.Width != 120 || vLy.Shape.Height != 40 {
		t.Errorf("vertical gap = %vx%v, want 120x40",
			vLy.Shape.Width, vLy.Shape.Height)
	}
	if vLy.Shape.Sizing != FillFixed {
		t.Errorf("vertical gap sizing = %v, want FillFixed",
			vLy.Shape.Sizing)
	}

	hGap := dragReorderGapView(state, dragReorderHorizontal)
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

func TestDragReorderStartNilGuards(t *testing.T) {
	newWindow := func() *Window {
		w := &Window{}
		w.layout = Layout{Shape: &Shape{ID: "root"}}
		return w
	}
	shape := &Layout{Shape: &Shape{ID: "a"}}
	noop := func(string, string, EventCtx) {}

	// Nil layout: no state, no lock.
	w := newWindow()
	dragReorderStart(dragReorderStartCfg{
		DragKey: "drag_nil_layout", Index: 0, ItemID: "a",
		Axis: dragReorderVertical, ItemIDs: []string{"a"},
		OnReorder: noop, Event: &Event{},
	}, w)
	if w.mouseIsLocked() {
		t.Error("nil layout must not lock the mouse")
	}
	if state := dragReorderGet(w, "drag_nil_layout"); state.started {
		t.Error("nil layout must not start a drag")
	}

	// Nil event: same expectation.
	w = newWindow()
	dragReorderStart(dragReorderStartCfg{
		DragKey: "drag_nil_event", Index: 0, ItemID: "a",
		Axis: dragReorderVertical, ItemIDs: []string{"a"},
		OnReorder: noop, Layout: shape,
	}, w)
	if w.mouseIsLocked() {
		t.Error("nil event must not lock the mouse")
	}
	if state := dragReorderGet(w, "drag_nil_event"); state.started {
		t.Error("nil event must not start a drag")
	}

	// Nil shape: same expectation.
	w = newWindow()
	dragReorderStart(dragReorderStartCfg{
		DragKey: "drag_nil_shape", Index: 0, ItemID: "a",
		Axis: dragReorderVertical, ItemIDs: []string{"a"},
		OnReorder: noop, Layout: &Layout{}, Event: &Event{},
	}, w)
	if w.mouseIsLocked() {
		t.Error("nil shape must not lock the mouse")
	}
	if state := dragReorderGet(w, "drag_nil_shape"); state.started {
		t.Error("nil shape must not start a drag")
	}

	// Nil event on a locked move: no panic, no state change.
	w = newWindow()
	lock := dragReorderMakeLock(
		"drag_nil_move", dragReorderVertical,
		[]string{"a"}, noop)
	lock.MouseMove(EventCtx{&w.layout, nil, w})
}

func TestDragReorderDropUsesSnapshotIDs(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_snapshot"
	itemIDs := []string{"a", "b", "c"}
	var moved, before string
	dragReorderStart(dragReorderStartCfg{
		DragKey: dragKey, Index: 0, ItemID: "a",
		Axis: dragReorderVertical, ItemIDs: itemIDs,
		OnReorder: func(m, b string, _ EventCtx) {
			moved, before = m, b
		},
		ItemLayoutIDs: []string{"a", "b", "c"},
		Layout:        &Layout{Shape: &Shape{ID: "a"}},
		Event:         &Event{},
	}, w)

	// Caller mutates its slice after start; the drop must use the
	// snapshot taken in Start, not the aliased backing array.
	itemIDs[2] = "zzz"
	state := dragReorderGet(w, dragKey)
	state.active = true
	state.currentIndex = 2
	dragReorderSet(w, dragKey, state)

	w.viewState.mouseLock.MouseUp(EventCtx{nil, &Event{}, w})
	if moved != "a" || before != "c" {
		t.Errorf("got (%q,%q) want (a,c)", moved, before)
	}
}

func TestDragReorderMidsOffsetClamped(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_clamp"
	dragReorderSet(w, dragKey, dragReorderState{
		active:       true,
		sourceIndex:  0,
		itemCount:    3,
		itemMids:     []float32{10, 20, 30},
		midsOffset:   5,
		layoutsValid: true,
	})
	dragReorderOnMouseMove(dragKey, dragReorderVertical, 0, 25, w)
	if got := dragReorderGet(w, dragKey).currentIndex; got != 3 {
		t.Errorf("offset index: got %d want 3", got)
	}
}

func TestDragReorderMoveDoesNotAllocScrollMaps(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_noalloc"
	dragReorderSet(w, dragKey, dragReorderState{
		active:       true,
		itemY:        0,
		itemHeight:   20,
		sourceIndex:  0,
		itemCount:    5,
		scrollID:     "s",
		startScrollY: 0,
	})
	// Outside the scroll zone, so no scroll write happens.
	dragReorderOnMouseMove(dragKey, dragReorderVertical, 0, 50, w)
	if w.scrollXRead() != nil || w.scrollYRead() != nil {
		t.Error("hot-path scroll read must not allocate scroll maps")
	}
}

func TestDragReorderStartParentNilShape(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	noop := func(string, string, EventCtx) {}
	// A parent without a shape carries no geometry; Start must
	// not dereference it when resolving the scroll container.
	layout := &Layout{
		Shape:  &Shape{ID: "a", Width: 10, Height: 10},
		Parent: &Layout{},
	}
	dragReorderStart(dragReorderStartCfg{
		DragKey: "drag_parent_nil_shape", Index: 0, ItemID: "a",
		Axis: dragReorderVertical, ItemIDs: []string{"a"},
		OnReorder: noop, Layout: layout, Event: &Event{},
		ScrollID: "s",
	}, w)
	if state := dragReorderGet(w, "drag_parent_nil_shape"); !state.started {
		t.Error("parent with nil shape must still start a drag")
	}
}

func TestDragReorderScrollChangeUsesUniformEstimate(t *testing.T) {
	w := &Window{}
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	dragKey := "drag_scroll_uniform"
	idScroll := "200"

	sy := w.scrollY()
	sy.Set(idScroll, 10.0)

	state := dragReorderState{
		active:       true,
		itemY:        0.0,
		itemHeight:   10.0,
		sourceIndex:  0,
		itemCount:    5,
		scrollID:     idScroll,
		startScrollY: 0.0,
		itemMids:     []float32{25, 35},
		midsOffset:   2,
		layoutsValid: true,
	}
	dragReorderSet(w, dragKey, state)

	// With scroll change, mids are invalid; uniform should yield 0.
	dragReorderOnMouseMove(
		dragKey, dragReorderVertical, 0, 15, w)

	newState := dragReorderGet(w, dragKey)
	if newState.currentIndex != 0 {
		t.Errorf("uniform fallback: got %d want 0",
			newState.currentIndex)
	}
}
