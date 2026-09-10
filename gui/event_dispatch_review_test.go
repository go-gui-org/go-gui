package gui

import "testing"

// Regressions for the event-dispatch review. Each test fails against the
// code as it stood before its fix.

// --- OnMouseScroll coordinate space ---

// runScrollProbe dispatches one scroll at screen (150,80) over a shape
// placed at (100,50) and returns the coordinates OnMouseScroll was
// handed. focused selects which dispatch path reaches the handler.
func runScrollProbe(focused bool) (gotX, gotY float32) {
	w := &Window{focused: true}
	sh := &Shape{
		ID: "sc", Focusable: true,
		X: 100, Y: 50, Width: 200, Height: 100,
		shapeClip: drawClip{X: 100, Y: 50, Width: 200, Height: 100},
	}
	sh.events = &eventHandlers{
		OnMouseScroll: func(ctx EventCtx) {
			gotX, gotY = ctx.Event.MouseX, ctx.Event.MouseY
			ctx.Consume()
		},
	}
	root := &Layout{
		Shape:    &Shape{ID: "root", shapeClip: drawClip{Width: 800, Height: 600}},
		Children: []Layout{{Shape: sh}},
	}
	root.Children[0].Parent = root
	if focused {
		w.viewState.focusID = "sc"
	}
	mouseScrollHandler(root,
		&Event{Type: EventMouseScroll, MouseX: 150, MouseY: 80, ScrollY: 3}, w)
	return
}

// One callback must not see two coordinate spaces. The focused-target
// path went through callRelative and handed over shape-relative
// coordinates; the fallback path called the callback directly and handed
// over screen-space ones. Whether a DrawCanvas zoomed at the cursor or
// at the wrong point came down to whether it held focus.
func TestScrollHandlerCoordsMatchOnBothPaths(t *testing.T) {
	fx, fy := runScrollProbe(true)
	ux, uy := runScrollProbe(false)
	if fx != ux || fy != uy {
		t.Errorf("focused path got (%v,%v), fallback got (%v,%v); "+
			"one callback must see one coordinate space", fx, fy, ux, uy)
	}
	// Shape-relative is the convention every other pointer callback
	// follows, so pin the value too, not only the agreement.
	if fx != 50 || fy != 30 {
		t.Errorf("got (%v,%v), want shape-relative (50,30)", fx, fy)
	}
}

// --- callRelative save/restore width ---

// callRelative used to copy the whole Event out and back, which reverted
// every field a callback wrote, with IsHandled reinstated by hand as the
// only exception. Dispatch owns the two coordinates and nothing else.
func TestCallRelativePreservesCallbackWrites(t *testing.T) {
	w := &Window{focused: true}
	sh := &Shape{
		X: 10, Y: 20, Width: 100, Height: 100,
		shapeClip: drawClip{X: 10, Y: 20, Width: 100, Height: 100},
	}
	l := &Layout{Shape: sh}
	e := &Event{Type: EventMouseDown, MouseX: 60, MouseY: 70}
	cb := func(ctx EventCtx) {
		// A field dispatch does not own. Writing it must stick.
		ctx.Event.IMEText = "written by the callback"
	}
	callRelative(l, e, w, cb, evNotify)
	if e.IMEText != "written by the callback" {
		t.Errorf("callback write reverted: IMEText = %q", e.IMEText)
	}
	// The coordinates dispatch does own are restored.
	if e.MouseX != 60 || e.MouseY != 70 {
		t.Errorf("coords not restored: got (%v,%v), want (60,70)",
			e.MouseX, e.MouseY)
	}
}

// --- modifier masking ---

// Two backends OR the held mouse buttons into Event.Modifiers. Matched
// unmasked, a wheel turn during a drag equalled neither ModNone nor
// ModShift, so scroll-while-dragging silently did nothing on Windows and
// the web while working on macOS.
// scrollableProbeTree builds a 100x100 scroll container holding a
// 500x500 child, so there is room to scroll on both axes, and returns the
// root plus the container.
func scrollableProbeTree() (root, sc *Layout) {
	// shapeRectangle, not the zero shapeType: skipLayoutChild drops a
	// shapeNone child, which would leave the container no content to
	// scroll and scrollVertical refusing the move for the wrong reason.
	inner := &Shape{
		shapeType: shapeRectangle,
		X:         0, Y: 0, Width: 500, Height: 500,
		shapeClip: drawClip{Width: 500, Height: 500},
	}
	container := &Shape{
		shapeType: shapeRectangle,
		ID:        "sc", Scrollable: true, Axis: axisTopToBottom,
		X: 0, Y: 0, Width: 100, Height: 100,
		shapeClip: drawClip{Width: 100, Height: 100},
	}
	root = &Layout{
		Shape: &Shape{shapeType: shapeRectangle,
			shapeClip: drawClip{Width: 200, Height: 200}},
		Children: []Layout{{
			Shape:    container,
			Children: []Layout{{Shape: inner}},
		}},
	}
	sc = &root.Children[0]
	sc.Parent = root
	sc.Children[0].Parent = sc
	return
}

func TestScrollWorksWhileMouseButtonHeld(t *testing.T) {
	for _, tc := range []struct {
		name string
		mods Modifier
	}{
		{"no buttons", ModNone},
		{"left held", ModLMB},
		{"right held", ModRMB},
		{"middle held", ModMMB},
		{"left held plus shift", ModLMB | ModShift},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &Window{focused: true}
			// No OnMouseScroll: the assertion is on the Scrollable
			// branch, which is the one that matched modifiers exactly.
			root, _ := scrollableProbeTree()
			// Both deltas are set: masked to ModShift the dispatch
			// reads ScrollX, masked to ModNone it reads ScrollY.
			e := &Event{
				Type: EventMouseScroll, MouseX: 50, MouseY: 50,
				ScrollX: -10, ScrollY: -10,
				ScrollPrecise: true, Modifiers: tc.mods,
			}
			mouseScrollHandler(root, e, w)
			if !e.IsHandled {
				t.Errorf("modifiers %#x: scroll not handled; the "+
					"held-button bits must not defeat the match", tc.mods)
			}
		})
	}
}

// keyDownScrollHandler matches modifiers the same way and needs the same
// mask: arrow-key scrolling must survive a held mouse button.
func TestKeyScrollWorksWhileMouseButtonHeld(t *testing.T) {
	w := &Window{focused: true}
	_, sc := scrollableProbeTree()
	e := &Event{Type: EventKeyDown, KeyCode: KeyDown, Modifiers: ModLMB}
	keyDownScrollHandler(sc, e, w)
	if !e.IsHandled {
		t.Error("arrow scroll dropped while a mouse button was held")
	}
}

// --- synthetic mouse events carry a frame stamp ---

// EventFn stamps FrameCount on every backend event; the touch-to-mouse
// synthesis path did not, so consumers timing multi-click gestures by
// differencing it saw frame 0 and never recognized a double tap.
func TestSynthMouseStampsFrameCount(t *testing.T) {
	w := &Window{focused: true, scratch: newScratchPools()}
	w.frameCount = 4242
	root := &Layout{Shape: &Shape{shapeClip: drawClip{Width: 200, Height: 200}}}
	synthMouse(EventMouseDown, 10, 10, MouseLeft, root, w)
	if got := w.scratch.gestureEvent.FrameCount; got != 4242 {
		t.Errorf("FrameCount = %d, want the window's 4242", got)
	}
}

// --- nil-Shape tolerance in the ID walks ---

// findLayoutByFocusID runs on every scroll event through
// focusedScrollTarget, and findLayoutByScrollID on every scrollbar
// build. Both dereferenced layout.Shape unguarded while the rest of the
// package treats a nil Shape as an ordinary state.
func TestFindLayoutWalksToleratesNilShape(t *testing.T) {
	root := &Layout{Children: []Layout{
		{}, // nil Shape
		{Shape: &Shape{ID: "f", Focusable: true}},  //
		{Shape: &Shape{ID: "s", Scrollable: true}}, //
	}}
	for i := range root.Children {
		root.Children[i].Parent = root
	}
	if ly, ok := findLayoutByFocusID(root, "f"); !ok || ly == nil {
		t.Error("focus walk did not find the focusable past a nil Shape")
	}
	if ly, ok := findLayoutByScrollID(root, "s"); !ok || ly == nil {
		t.Error("scroll walk did not find the scrollable past a nil Shape")
	}
}

// --- nil root at every dispatch entry point ---

// keyupHandler has tolerated a nil root since it was written, and the
// guard sat in its recursive half where it was re-evaluated per node
// while no other walk had it at all. The tolerance now lives at every
// entry point and in none of the recursive halves.
func TestDispatchEntryPointsTolerateNilRoot(t *testing.T) {
	w := &Window{focused: true}
	for name, call := range map[string]func(){
		"char":     func() { charHandler(nil, &Event{CharCode: 'a'}, w) },
		"keydown":  func() { keydownHandler(nil, &Event{KeyCode: KeyEnter}, w) },
		"keyup":    func() { keyupHandler(nil, &Event{KeyCode: KeyEnter}, w) },
		"mousedn":  func() { mouseDownHandler(nil, false, &Event{}, w) },
		"mousemv":  func() { mouseMoveHandler(nil, &Event{}, w) },
		"mouseup":  func() { mouseUpHandler(nil, &Event{}, w) },
		"scroll":   func() { mouseScrollHandler(nil, &Event{}, w) },
		"filedrop": func() { fileDropHandler(nil, &Event{}, w) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panicked on a nil root: %v", r)
				}
			}()
			call()
		})
	}
}

// --- nil-Shape intermediates in the remaining walks ---

// findByID used to stop at a shape-less node without searching below
// it, while the focus and scroll ID walks recurse past one. A
// hand-built Layout with a shape-less intermediate must still resolve
// the descendant it contains.
func TestFindByIDSearchesPastNilShape(t *testing.T) {
	root := &Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{{
			Children: []Layout{{Shape: &Shape{ID: "target"}}},
		}},
	}
	if _, ok := root.FindByID("target"); !ok {
		t.Error("FindByID did not find the descendant past a nil Shape")
	}
}

// collectFocusCandidates dereferenced Layout.Shape unguarded, so a
// shape-less intermediate panicked tab traversal while the ID walks
// beside it tolerated one.
func TestFocusTraversalPastNilShape(t *testing.T) {
	root := &Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{{
			Children: []Layout{{Shape: &Shape{ID: "f", Focusable: true}}},
		}},
	}
	if s, ok := root.nextFocusable(nil); !ok || s == nil || s.ID != "f" {
		t.Error("tab traversal did not reach the focusable past a nil Shape")
	}
}
