package gui

import "testing"

func TestIsFocusedTargetNilShape(t *testing.T) {
	w := &Window{}
	l := &Layout{}
	if isFocusedTarget(l, w) {
		t.Error("nil Shape should return false")
	}
}

func TestIsFocusedTargetReservedDialog(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{ID: reservedDialogID}}
	if !isFocusedTarget(l, w) {
		t.Error("reservedDialogID should return true")
	}
}

func TestIsFocusedTargetScopedReservedDialogID(t *testing.T) {
	w := &Window{}
	// A scoped widget whose leaf spells the reserved dialog ID
	// addresses "scope:___dialog_reserved_do_not_use___", not the
	// dialog. Matching on the leaf would hand it the dialog's
	// focused dispatch.
	l := &Layout{Shape: &Shape{ID: reservedDialogID, effID: "scope:" + reservedDialogID}}
	if isFocusedTarget(l, w) {
		t.Error("scoped reservedDialogID leaf should not be a focus target")
	}
}

func TestIsFocusedTargetZeroIDFocus(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{Focusable: true, ID: "f0"}}
	if isFocusedTarget(l, w) {
		t.Error("IDFocus 0 should return false")
	}
}

func TestIsFocusedTargetMatches(t *testing.T) {
	w := &Window{}
	w.viewState.focusID.Store("f42")
	l := &Layout{Shape: &Shape{Focusable: true, ID: "f42"}}
	if !isFocusedTarget(l, w) {
		t.Error("matching IDFocus should return true")
	}
}

func TestExecuteFocusCallbackNil(t *testing.T) {
	w := &Window{}
	w.viewState.focusID.Store("f1")
	l := &Layout{Shape: &Shape{ID: reservedDialogID}}
	e := &Event{}
	if executeFocusCallback(l, e, w, nil, evNotify, nil) {
		t.Error("nil callback should return false")
	}
}

// duplicateFocusRoot builds a root with two focusable twins sharing
// one effective ID, each counting its own deliveries.
func duplicateFocusRoot(first, second *int, handlers func(*int) *eventHandlers) *Layout {
	return &Layout{Shape: &Shape{}, Children: []Layout{
		{Shape: &Shape{Focusable: true, ID: "dup", events: handlers(first)}},
		{Shape: &Shape{Focusable: true, ID: "dup", events: handlers(second)}},
	}}
}

// Twins sharing an effective ID are a bug the debug gate reports, but
// the runtime must still deliver once: the first twin in dispatch
// order wins instead of every twin firing. Each twin declines, so a
// second delivery cannot hide behind consumption.
func TestDuplicateFocusIDDedupsKeyDown(t *testing.T) {
	t.Parallel()
	var first, second int
	root := duplicateFocusRoot(&first, &second, func(calls *int) *eventHandlers {
		return &eventHandlers{OnKeyDown: func(EventCtx) { *calls++ }}
	})
	w := &Window{}
	w.SetFocus("dup")
	keydownHandler(root, &Event{Type: EventKeyDown, KeyCode: KeyA}, w)
	if first+second != 1 {
		t.Errorf("key down ran %d times across twins, want 1", first+second)
	}
}

func TestDuplicateFocusIDDedupsChar(t *testing.T) {
	t.Parallel()
	var first, second int
	root := duplicateFocusRoot(&first, &second, func(calls *int) *eventHandlers {
		return &eventHandlers{OnChar: func(EventCtx) { *calls++ }}
	})
	w := &Window{}
	w.SetFocus("dup")
	charHandler(root, &Event{Type: EventChar, CharCode: 'x'}, w)
	if first+second != 1 {
		t.Errorf("char ran %d times across twins, want 1", first+second)
	}
}

func TestDuplicateFocusIDDedupsEnterClick(t *testing.T) {
	t.Parallel()
	var first, second int
	root := duplicateFocusRoot(&first, &second, func(calls *int) *eventHandlers {
		return &eventHandlers{
			clickOnEnter: true,
			OnClick:      func(EventCtx) { *calls++ },
		}
	})
	w := &Window{}
	w.SetFocus("dup")
	keydownHandler(root,
		&Event{Type: EventKeyDown, KeyCode: KeyEnter, Modifiers: ModNone}, w)
	if first+second != 1 {
		t.Errorf("enter-click ran %d times across twins, want 1", first+second)
	}
}

func TestDuplicateFocusIDDedupsKeyUp(t *testing.T) {
	t.Parallel()
	var first, second int
	root := duplicateFocusRoot(&first, &second, func(calls *int) *eventHandlers {
		return &eventHandlers{OnKeyUp: func(EventCtx) { *calls++ }}
	})
	w := &Window{}
	w.SetFocus("dup")
	keyupHandler(root, &Event{Type: EventKeyUp, KeyCode: KeyA}, w)
	if first+second != 1 {
		t.Errorf("key up ran %d times across twins, want 1", first+second)
	}
}

func TestDuplicateFocusIDDedupsSpaceClick(t *testing.T) {
	t.Parallel()
	var first, second int
	root := duplicateFocusRoot(&first, &second, func(calls *int) *eventHandlers {
		return &eventHandlers{
			clickOnSpace: true,
			OnClick:      func(EventCtx) { *calls++ },
		}
	})
	w := &Window{}
	w.SetFocus("dup")
	keydownHandler(root, &Event{Type: EventKeyDown, KeyCode: KeySpace}, w)
	keyupHandler(root, &Event{Type: EventKeyUp, KeyCode: KeySpace}, w)
	if first+second != 1 {
		t.Errorf("space-click ran %d times across twins, want 1", first+second)
	}
}

// The dedup scope is one dispatch: a second event must deliver again.
func TestDuplicateFocusIDResetsPerEvent(t *testing.T) {
	t.Parallel()
	var first, second int
	root := duplicateFocusRoot(&first, &second, func(calls *int) *eventHandlers {
		return &eventHandlers{OnKeyDown: func(EventCtx) { *calls++ }}
	})
	w := &Window{focused: true}
	w.layout = *root
	w.SetFocus("dup")
	w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyA})
	w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyA})
	if first+second != 2 {
		t.Errorf("two events ran %d times across twins, want 2", first+second)
	}
}

func TestExecuteMouseCallbackOutside(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{
		shapeClip: drawClip{X: 10, Y: 10, Width: 50, Height: 50},
	}}
	e := &Event{MouseX: 0, MouseY: 0}
	called := false
	cb := func(ctx EventCtx) { called = true }
	if executeMouseCallback(l, e, w, cb, evNotify) {
		t.Error("outside mouse should return false")
	}
	if called {
		t.Error("callback should not fire")
	}
}

func TestExecuteMouseCallbackInside(t *testing.T) {
	w := &Window{}
	s := &Shape{
		shapeClip: drawClip{X: 10, Y: 20, Width: 50, Height: 50},
	}
	s.X = 10
	s.Y = 20
	l := &Layout{Shape: s}
	e := &Event{MouseX: 30, MouseY: 40}
	var relX, relY float32
	cb := func(ctx EventCtx) {
		relX = ctx.Event.MouseX
		relY = ctx.Event.MouseY
		ctx.Consume()
	}
	if !executeMouseCallback(l, e, w, cb, evNotify) {
		t.Error("inside mouse should return true")
	}
	if relX != 20 || relY != 20 {
		t.Errorf("relative coords = (%f,%f), want (20,20)", relX, relY)
	}
}

func TestExecuteMouseCallbackRestoresEvent(t *testing.T) {
	w := &Window{}
	s := &Shape{
		shapeClip: drawClip{X: 10, Y: 20, Width: 50, Height: 50},
	}
	s.X = 10
	s.Y = 20
	l := &Layout{Shape: s}
	e := &Event{MouseX: 30, MouseY: 40}
	cb := func(ctx EventCtx) {
		ctx.Consume()
	}
	executeMouseCallback(l, e, w, cb, evNotify)
	if e.MouseX != 30 || e.MouseY != 40 {
		t.Errorf("event coords = (%f,%f), want (30,40)", e.MouseX, e.MouseY)
	}
}

func TestIsChildEnabledTraversal(t *testing.T) {
	if isChildEnabled(&Layout{}) {
		t.Error("nil Shape should return false")
	}
	if isChildEnabled(&Layout{Shape: &Shape{Disabled: true}}) {
		t.Error("Disabled should return false")
	}
	if !isChildEnabled(&Layout{Shape: &Shape{}}) {
		t.Error("normal shape should return true")
	}
}
