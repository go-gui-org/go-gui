package gui

import "testing"

// Interaction-level coverage for the splitter: the AmendLayout geometry
// pass, drag resize through the mouse lock, hover feedback, the keyboard
// map and the collapse buttons. The sibling file
// (view_splitter_test.go) covers the pure computation helpers; anything
// that needs a real layout tree or real event dispatch lives here.

const (
	splitterTestW = 400
	splitterTestH = 300
	// Default HandleSize from defaultSplitterStyle. Asserted below so a
	// theme change fails here loudly rather than skewing every geometry
	// expectation by a few pixels.
	splitterTestHandle = 9
)

// splitterHarness renders a splitter as the whole window with the ratio
// and collapse state owned by the test.
//
// The widget stores nothing: it reads cfg.Ratio every frame and reports a
// new one through OnChange. A test that did not write the reported value
// back would be driving a splitter that can never move, so the harness
// models the app half of that contract — feed state in, capture state out,
// re-render.
type splitterHarness struct {
	w       *Window
	state   SplitterState
	changes int
}

func newSplitterHarness(t *testing.T, cfg SplitterCfg) *splitterHarness {
	t.Helper()
	h := &splitterHarness{
		state: SplitterState{
			Ratio:     cfg.Ratio.Get(splitterDefaultRatio),
			Collapsed: cfg.Collapsed,
		},
	}
	h.w = NewTestWindow(WindowCfg{
		Width:  splitterTestW,
		Height: splitterTestH,
	})
	base := cfg
	h.w.TestRender(func(_ *Window) View {
		c := base
		c.Ratio = SomeF(h.state.Ratio)
		c.Collapsed = h.state.Collapsed
		c.OnChange = func(ratio float32, col SplitterCollapsed, _ EventCtx) {
			h.state = SplitterState{Ratio: ratio, Collapsed: col}
			h.changes++
		}
		return splitterFillParent(Splitter(c))
	})
	return h
}

// splitterFillParent wraps a splitter in a Fill parent that fills the
// window exactly. The splitter defaults to FillFill but Canvas has no
// axis, and a Fill root with no parent to fill against resolves to 0x0 —
// so a splitter used as the whole view collapses. Real apps put it inside
// a Row/Column (see examples/showcase), which is what this reproduces.
func splitterFillParent(v View) View {
	return Row(ContainerCfg{
		Sizing:     FillFill,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    []View{v},
	})
}

// find resolves an effective ID in the current tree. Every helper
// re-resolves rather than caching a *Layout: each settled event rebuilds
// the tree, so a pointer captured before an action is stale after it.
func (h *splitterHarness) find(t *testing.T, id string) *Layout {
	t.Helper()
	ly, ok := h.w.layout.FindByID(id)
	if !ok || ly.Shape == nil {
		t.Fatalf("no layout with ID %q", id)
	}
	return ly
}

// parts returns the three children AmendLayout positions, in tree order.
func (h *splitterHarness) parts(t *testing.T, id string) (first, handle, second *Layout) {
	t.Helper()
	root := h.find(t, id)
	if len(root.Children) != 3 {
		t.Fatalf("splitter children = %d, want 3", len(root.Children))
	}
	return &root.Children[0], &root.Children[1], &root.Children[2]
}

// center returns the middle of the shape's clip rect, which is what
// PointInShape tests — the same coordinate the Test* helpers pick.
func center(ly *Layout) (x, y float32) {
	c := ly.Shape.shapeClip
	return c.X + c.Width/2, c.Y + c.Height/2
}

func (h *splitterHarness) send(e *Event) {
	h.w.EventFn(e)
	h.w.settle()
}

func (h *splitterHarness) press(x, y float32) {
	h.send(&Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	})
}

func (h *splitterHarness) move(x, y float32) {
	h.send(&Event{
		Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x, MouseY: y,
	})
}

func (h *splitterHarness) release(x, y float32) {
	h.send(&Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	})
}

// pressHandle presses the left button at the center of the handle, which
// is what arms the drag (OnClick fires on press, not on release).
func (h *splitterHarness) pressHandle(t *testing.T, id string) {
	t.Helper()
	_, handle, _ := h.parts(t, id)
	x, y := center(handle)
	h.press(x, y)
}

func nearF(t *testing.T, label string, got, want, eps float32) {
	t.Helper()
	d := got - want
	if d < 0 {
		d = -d
	}
	if d > eps {
		t.Errorf("%s = %g, want %g (±%g)", label, got, want, eps)
	}
}

func splitterTwoPanes() (first, second SplitterPaneCfg) {
	return SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "L"})}},
		SplitterPaneCfg{Content: []View{Text(TextCfg{Text: "R"})}}
}

// splitterEmptyPanes is for geometry assertions that need a pane able to
// reach zero width. A pane with content cannot: the fit pass floors it at
// the content's own minimum (see
// TestSplitterCollapsedPaneKeepsContentMinimum).
func splitterEmptyPanes() (first, second SplitterPaneCfg) {
	return SplitterPaneCfg{}, SplitterPaneCfg{}
}

// --- AmendLayout geometry ---

func TestSplitterDefaultHandleSize(t *testing.T) {
	if defaultSplitterStyle.HandleSize != splitterTestHandle {
		t.Fatalf("theme HandleSize = %g, want %d — geometry expectations "+
			"in this file assume the default",
			defaultSplitterStyle.HandleSize, splitterTestHandle)
	}
}

func TestSplitterAmendLayoutHorizontal(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.5),
		First:  a,
		Second: b,
	})
	first, handle, second := h.parts(t, "sp")

	avail := float32(splitterTestW - splitterTestHandle)
	wantFirst := avail * 0.5

	nearF(t, "first width", first.Shape.Width, wantFirst, 0.5)
	nearF(t, "handle width", handle.Shape.Width, splitterTestHandle, 0.01)
	nearF(t, "second width", second.Shape.Width, avail-wantFirst, 0.5)

	// Panes tile the main axis with no gap and no overlap.
	nearF(t, "first x", first.Shape.X, 0, 0.01)
	nearF(t, "handle x", handle.Shape.X, wantFirst, 0.5)
	nearF(t, "second x", second.Shape.X,
		wantFirst+splitterTestHandle, 0.5)

	// Cross axis: every child spans the full height.
	for _, ly := range []*Layout{first, handle, second} {
		nearF(t, "child height", ly.Shape.Height, splitterTestH, 0.01)
		nearF(t, "child y", ly.Shape.Y, 0, 0.01)
	}
}

func TestSplitterAmendLayoutVertical(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:          "sp",
		Orientation: SplitterVertical,
		Ratio:       SomeF(0.25),
		First:       a,
		Second:      b,
	})
	first, handle, second := h.parts(t, "sp")

	avail := float32(splitterTestH - splitterTestHandle)
	wantFirst := avail * 0.25

	nearF(t, "first height", first.Shape.Height, wantFirst, 0.5)
	nearF(t, "handle height", handle.Shape.Height, splitterTestHandle, 0.01)
	nearF(t, "second height", second.Shape.Height, avail-wantFirst, 0.5)

	nearF(t, "first y", first.Shape.Y, 0, 0.01)
	nearF(t, "handle y", handle.Shape.Y, wantFirst, 0.5)
	nearF(t, "second y", second.Shape.Y,
		wantFirst+splitterTestHandle, 0.5)

	for _, ly := range []*Layout{first, handle, second} {
		nearF(t, "child width", ly.Shape.Width, splitterTestW, 0.01)
	}
}

func TestSplitterAmendLayoutClampsToMinSize(t *testing.T) {
	a, b := splitterTwoPanes()
	a.MinSize = 250
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.05), // would give ~20px without the clamp
		First:  a,
		Second: b,
	})
	first, _, second := h.parts(t, "sp")
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "first width", first.Shape.Width, 250, 0.5)
	nearF(t, "second width", second.Shape.Width, avail-250, 0.5)
}

func TestSplitterAmendLayoutClampsToMaxSize(t *testing.T) {
	a, b := splitterTwoPanes()
	a.MaxSize = 120
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.9),
		First:  a,
		Second: b,
	})
	first, _, second := h.parts(t, "sp")
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "first width", first.Shape.Width, 120, 0.5)
	nearF(t, "second width", second.Shape.Width, avail-120, 0.5)
}

// The second pane's MinSize constrains the first from the other side:
// available - secondMin is the upper bound on the first.
func TestSplitterAmendLayoutSecondMinSizeBoundsFirst(t *testing.T) {
	a, b := splitterTwoPanes()
	b.MinSize = 300
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.9),
		First:  a,
		Second: b,
	})
	first, _, second := h.parts(t, "sp")
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "second width", second.Shape.Width, 300, 0.5)
	nearF(t, "first width", first.Shape.Width, avail-300, 0.5)
}

func TestSplitterAmendLayoutCollapsedFirst(t *testing.T) {
	a, b := splitterEmptyPanes()
	a.collapsible = true
	h := newSplitterHarness(t, SplitterCfg{
		ID:        "sp",
		Collapsed: SplitterCollapseFirst,
		First:     a,
		Second:    b,
	})
	first, handle, second := h.parts(t, "sp")
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "collapsed first width", first.Shape.Width, 0, 0.01)
	nearF(t, "handle x", handle.Shape.X, 0, 0.01)
	nearF(t, "second width", second.Shape.Width, avail, 0.5)
}

// A collapsed pane is only as small as its content allows.
// splitterLayoutChild pins the pane to width 0, but then re-runs the fit
// pass on the subtree, which grows the pane back to its content's
// minimum — so the "collapsed" pane keeps a sliver and draws underneath
// the handle, which sits at x=0. Asserted as-is to pin the behavior;
// see issue #263.
func TestSplitterCollapsedPaneKeepsContentMinimum(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := newSplitterHarness(t, SplitterCfg{
		ID:        "sp",
		Collapsed: SplitterCollapseFirst,
		First:     a,
		Second:    b,
	})
	first, handle, _ := h.parts(t, "sp")
	if first.Shape.Width <= 0 {
		t.Errorf("collapsed pane width = %g: #263 appears fixed, so "+
			"this test and the geometry cases using splitterEmptyPanes "+
			"should be rewritten to expect a true zero-width collapse",
			first.Shape.Width)
	}
	if first.Shape.X != handle.Shape.X {
		t.Errorf("collapsed pane x = %g, handle x = %g: expected the "+
			"residual pane to sit under the handle",
			first.Shape.X, handle.Shape.X)
	}
}

func TestSplitterAmendLayoutCollapsedSecondHonorsCollapsedSize(t *testing.T) {
	a, b := splitterEmptyPanes()
	b.collapsible = true
	b.collapsedSize = 24
	h := newSplitterHarness(t, SplitterCfg{
		ID:        "sp",
		Collapsed: SplitterCollapseSecond,
		First:     a,
		Second:    b,
	})
	first, _, second := h.parts(t, "sp")
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "collapsed second width", second.Shape.Width, 24, 0.5)
	nearF(t, "first width", first.Shape.Width, avail-24, 0.5)
}

// Collapsed is inert unless the pane opted in. The config is reachable
// (SplitterCollapseFirst is exported) while collapsible is not, so an app
// can only ask for a collapse the widget will refuse.
func TestSplitterAmendLayoutCollapseIgnoredWhenNotCollapsible(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:        "sp",
		Ratio:     SomeF(0.5),
		Collapsed: SplitterCollapseFirst,
		First:     a,
		Second:    b,
	})
	first, _, _ := h.parts(t, "sp")
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "first width", first.Shape.Width, avail*0.5, 0.5)
}

// A handle wider than the window collapses both panes to zero rather than
// producing negative sizes.
func TestSplitterAmendLayoutOversizedHandle(t *testing.T) {
	a, b := splitterEmptyPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:         "sp",
		HandleSize: SomeF(splitterTestW + 200),
		First:      a,
		Second:     b,
	})
	first, handle, second := h.parts(t, "sp")
	nearF(t, "handle width", handle.Shape.Width, splitterTestW, 0.01)
	nearF(t, "first width", first.Shape.Width, 0, 0.01)
	nearF(t, "second width", second.Shape.Width, 0, 0.01)
}

// AmendLayout indexes Children[0..2] unconditionally after the length
// guard; a truncated tree must be left alone rather than panic.
func TestSplitterAmendLayoutShortCircuitsOnMissingChildren(t *testing.T) {
	core := &splitterCore{id: "sp", handleSize: 9}
	layout := Layout{Shape: &Shape{ID: "sp", Width: 100, Height: 50}}
	splitterAmendLayout(core, &layout, NewTestWindow(WindowCfg{}))
	if core.id != "sp" {
		t.Errorf("core.id = %q, want unchanged", core.id)
	}
}

// AmendLayout stamps the effective ID onto the core, which is what every
// later handler looks the splitter up by. Nesting the splitter inside an
// ID-bearing container is the case that breaks if the stamp is missing:
// the leaf "sp" is no longer the identity.
func TestSplitterAmendLayoutStampsEffectiveID(t *testing.T) {
	a, b := splitterTwoPanes()
	w := NewTestWindow(WindowCfg{Width: splitterTestW, Height: splitterTestH})
	var got float32 = -1
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:         "outer",
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content: []View{
				Splitter(SplitterCfg{
					ID:     "sp",
					Ratio:  SomeF(0.5),
					First:  a,
					Second: b,
					OnChange: func(r float32, _ SplitterCollapsed, _ EventCtx) {
						got = r
					},
				}),
			},
		})
	})
	if _, ok := w.layout.FindByID("outer:sp"); !ok {
		t.Fatal("nested splitter did not resolve to outer:sp")
	}
	// The inner IDs are composed eagerly in the factory, with no Window
	// in hand, so they already contain ':' and resolveShapeIDs treats
	// them as absolute — "sp:handle" stays window-global rather than
	// becoming "outer:sp:handle". Asserted as-is; see issue #264.
	ly, ok := w.layout.FindByID("sp:handle")
	if !ok {
		t.Fatal("handle did not resolve to the unscoped sp:handle")
	}
	if _, nested := w.layout.FindByID("outer:sp:handle"); nested {
		t.Error("handle now scopes under its ancestor: #264 appears " +
			"fixed, so this test should expect outer:sp:handle")
	}
	x, y := center(ly)
	down := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&down)
	w.settle()
	mv := Event{Type: EventMouseMove, MouseX: 100, MouseY: y}
	w.EventFn(&mv)
	w.settle()
	if got < 0 {
		t.Fatal("drag produced no OnChange; core.id was never stamped")
	}
}

// --- Handle click and drag ---

func TestSplitterHandleClickLocksMouseAndFocuses(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:        "sp",
		Focusable: true,
		First:     a,
		Second:    b,
	})
	h.pressHandle(t, "sp")
	if !h.w.mouseIsLocked() {
		t.Error("press on handle did not lock the mouse")
	}
	if h.w.FocusID() != "sp" {
		t.Errorf("focus = %q, want %q", h.w.FocusID(), "sp")
	}
	if h.w.viewState.mouseCursor != CursorResizeEW {
		t.Errorf("cursor = %v, want CursorResizeEW", h.w.viewState.mouseCursor)
	}
}

func TestSplitterHandleReleaseUnlocksMouse(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:        "sp",
		Focusable: true,
		First:     a,
		Second:    b,
	})
	h.pressHandle(t, "sp")
	h.release(100, 100)
	if h.w.mouseIsLocked() {
		t.Error("release did not unlock the mouse")
	}
	if h.w.FocusID() != "sp" {
		t.Errorf("focus after release = %q, want %q", h.w.FocusID(), "sp")
	}
}

func TestSplitterHandleClickDisabledIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "sp", disabled: true, handleSize: 9}
	e := Event{Type: EventMouseDown, MouseButton: MouseLeft}
	splitterOnHandleClick(core, &e, h.w)
	if h.w.mouseIsLocked() {
		t.Error("disabled splitter armed a drag")
	}
	if e.IsHandled {
		t.Error("disabled splitter consumed the press")
	}
}

func TestSplitterDragResizesPanes(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.5),
		First:  a,
		Second: b,
	})
	h.pressHandle(t, "sp")
	h.move(100, 150)
	h.release(100, 150)

	avail := float32(splitterTestW - splitterTestHandle)
	// The handler centers the handle on the cursor, so the first pane
	// ends one half-handle short of the cursor x.
	wantFirst := 100 - float32(splitterTestHandle)/2
	nearF(t, "reported ratio", h.state.Ratio, wantFirst/avail, 0.01)

	first, handle, _ := h.parts(t, "sp")
	nearF(t, "first width after drag", first.Shape.Width, wantFirst, 1)
	nearF(t, "handle x after drag", handle.Shape.X, wantFirst, 1)
}

func TestSplitterDragVertical(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:          "sp",
		Orientation: SplitterVertical,
		Ratio:       SomeF(0.5),
		First:       a,
		Second:      b,
	})
	h.pressHandle(t, "sp")
	if h.w.viewState.mouseCursor != CursorResizeNS {
		t.Errorf("cursor = %v, want CursorResizeNS", h.w.viewState.mouseCursor)
	}
	h.move(200, 60)

	avail := float32(splitterTestH - splitterTestHandle)
	wantFirst := 60 - float32(splitterTestHandle)/2
	nearF(t, "reported ratio", h.state.Ratio, wantFirst/avail, 0.01)

	first, _, _ := h.parts(t, "sp")
	nearF(t, "first height after drag", first.Shape.Height, wantFirst, 1)
}

func TestSplitterDragClampsAtMinSizes(t *testing.T) {
	a, b := splitterTwoPanes()
	a.MinSize = 80
	b.MinSize = 120
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.5),
		First:  a,
		Second: b,
	})
	avail := float32(splitterTestW - splitterTestHandle)

	// Drag well past the left edge: the first pane stops at its MinSize.
	h.pressHandle(t, "sp")
	h.move(-500, 150)
	nearF(t, "ratio at low clamp", h.state.Ratio, 80/avail, 0.01)
	first, _, _ := h.parts(t, "sp")
	nearF(t, "first width at low clamp", first.Shape.Width, 80, 1)

	// And past the right edge: the second pane's MinSize now binds.
	h.move(splitterTestW+500, 150)
	nearF(t, "ratio at high clamp", h.state.Ratio, (avail-120)/avail, 0.01)
	_, _, second := h.parts(t, "sp")
	nearF(t, "second width at high clamp", second.Shape.Width, 120, 1)
	h.release(0, 0)
}

// Every move during a drag reports, so a drag that does not move the
// handle still emits — but it must report the same ratio, not drift.
func TestSplitterDragIsIdempotentAtSamePoint(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.5),
		First:  a,
		Second: b,
	})
	h.pressHandle(t, "sp")
	h.move(120, 150)
	firstRatio := h.state.Ratio
	h.move(120, 150)
	if h.state.Ratio != firstRatio {
		t.Errorf("ratio drifted on repeat move: %g then %g",
			firstRatio, h.state.Ratio)
	}
}

// available <= 0 leaves nothing to divide, so the drag must report
// nothing rather than a 0/0 ratio.
func TestSplitterDragNoOpWhenNoSpace(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:         "sp",
		HandleSize: SomeF(splitterTestW + 200),
		First:      a,
		Second:     b,
	})
	h.pressHandle(t, "sp")
	before := h.changes
	h.move(50, 150)
	if h.changes != before {
		t.Errorf("changes = %d, want %d (no room to drag)",
			h.changes, before)
	}
}

func TestSplitterDragDisabledIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "sp", disabled: true, handleSize: 9}
	e := Event{Type: EventMouseMove, MouseX: 50}
	splitterOnDragMove(core, &e, h.w)
	if e.IsHandled {
		t.Error("disabled splitter consumed a drag move")
	}
}

// The core carries the ID of a splitter that is no longer in the tree
// (the view stopped emitting it mid-drag). Resolution fails and the
// handler must bail rather than dereference a missing layout.
func TestSplitterDragMissingLayoutIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "gone", handleSize: 9}
	e := Event{Type: EventMouseMove, MouseX: 50}
	splitterOnDragMove(core, &e, h.w)
	if e.IsHandled {
		t.Error("drag on a missing splitter consumed the event")
	}
}

// --- Hover ---

func TestSplitterHandleHoverPaintsHoverColor(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		First:  a,
		Second: b,
	})
	_, handle, _ := h.parts(t, "sp")
	x, y := center(handle)
	h.move(x, y)

	_, handle, _ = h.parts(t, "sp")
	if handle.Shape.Color != defaultSplitterStyle.colorHandleHover {
		t.Errorf("hovered handle color = %v, want %v",
			handle.Shape.Color, defaultSplitterStyle.colorHandleHover)
	}
	if h.w.viewState.mouseCursor != CursorResizeEW {
		t.Errorf("cursor = %v, want CursorResizeEW", h.w.viewState.mouseCursor)
	}
}

func TestSplitterHandleHoverAwayKeepsBaseColor(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID:     "sp",
		First:  a,
		Second: b,
	})
	h.move(5, 5)
	_, handle, _ := h.parts(t, "sp")
	if handle.Shape.Color != defaultSplitterStyle.colorHandle {
		t.Errorf("unhovered handle color = %v, want %v",
			handle.Shape.Color, defaultSplitterStyle.colorHandle)
	}
}

// The active (button-held) branch is exercised directly. layoutHover
// bails while the mouse is locked and always synthesizes its hover event
// with MouseButton: MouseInvalid, so no real frame can reach this branch
// — see issue #265.
func TestSplitterHandleHoverActiveColor(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	layout := Layout{Shape: &Shape{}}
	hover := RGBA(1, 2, 3, 255)
	active := RGBA(4, 5, 6, 255)
	e := Event{Type: EventMouseMove, MouseButton: MouseLeft}
	splitterOnHandleHover(SplitterVertical, hover, active, &layout, &e, h.w)
	if layout.Shape.Color != active {
		t.Errorf("color = %v, want active %v", layout.Shape.Color, active)
	}
	if !e.IsHandled {
		t.Error("hover did not consume the event")
	}
	if h.w.viewState.mouseCursor != CursorResizeNS {
		t.Errorf("cursor = %v, want CursorResizeNS", h.w.viewState.mouseCursor)
	}
}

// --- Keyboard ---

func splitterKeyHarness(t *testing.T, cfg SplitterCfg) *splitterHarness {
	t.Helper()
	cfg.Focusable = true
	return newSplitterHarness(t, cfg)
}

func (h *splitterHarness) key(t *testing.T, id string, k KeyCode, mods Modifier) {
	t.Helper()
	if err := h.w.TestKey(id, k, mods); err != nil {
		t.Fatalf("TestKey(%q): %v", id, err)
	}
}

func TestSplitterKeyArrowsAdjustRatio(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyRight, ModNone)
	nearF(t, "ratio after Right", h.state.Ratio,
		0.5+defaultSplitterStyle.dragStep, 0.001)
	h.key(t, "sp", KeyLeft, ModNone)
	nearF(t, "ratio after Left", h.state.Ratio, 0.5, 0.001)
}

func TestSplitterKeyShiftUsesLargeStep(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyRight, ModShift)
	nearF(t, "ratio after Shift-Right", h.state.Ratio,
		0.5+defaultSplitterStyle.dragStepLarge, 0.001)
}

// A horizontal splitter ignores the vertical arrows entirely, so Up/Down
// stay available to whatever else wants them.
func TestSplitterKeyWrongAxisArrowsIgnored(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyUp, ModNone)
	h.key(t, "sp", KeyDown, ModNone)
	if h.changes != 0 {
		t.Errorf("changes = %d, want 0 for cross-axis arrows", h.changes)
	}
}

func TestSplitterKeyVerticalArrows(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Orientation: SplitterVertical,
		Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyDown, ModNone)
	nearF(t, "ratio after Down", h.state.Ratio,
		0.5+defaultSplitterStyle.dragStep, 0.001)
	h.key(t, "sp", KeyLeft, ModNone)
	nearF(t, "ratio after cross-axis Left", h.state.Ratio,
		0.5+defaultSplitterStyle.dragStep, 0.001)
}

func TestSplitterKeyArrowsRespectMinSize(t *testing.T) {
	a, b := splitterTwoPanes()
	a.MinSize = 190 // ~0.486 of the 391px available
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	avail := float32(splitterTestW - splitterTestHandle)
	for range 10 {
		h.key(t, "sp", KeyLeft, ModNone)
	}
	nearF(t, "clamped ratio", h.state.Ratio, 190/avail, 0.001)
}

// Ctrl/Alt-modified arrows belong to the app, not the splitter.
func TestSplitterKeyModifiedArrowsIgnored(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyRight, ModCtrl)
	if h.changes != 0 {
		t.Errorf("changes = %d, want 0 for Ctrl-Right", h.changes)
	}
}

func TestSplitterKeyHomeCollapsesFirst(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyHome, ModNone)
	if h.state.Collapsed != SplitterCollapseFirst {
		t.Errorf("collapsed = %v, want first", h.state.Collapsed)
	}
	// The collapse reaches layout, not just the reported state: the
	// second pane is assigned the whole available span. That says
	// nothing about the first pane, whose residual sliver overlaps the
	// handle rather than shrinking the second (#263).
	_, _, second := h.parts(t, "sp")
	nearF(t, "second width", second.Shape.Width,
		splitterTestW-splitterTestHandle, 0.5)
}

func TestSplitterKeyEndCollapsesSecond(t *testing.T) {
	a, b := splitterTwoPanes()
	b.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyEnd, ModNone)
	if h.state.Collapsed != SplitterCollapseSecond {
		t.Errorf("collapsed = %v, want second", h.state.Collapsed)
	}
}

func TestSplitterKeyHomeEndInertWhenNotCollapsible(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyHome, ModNone)
	h.key(t, "sp", KeyEnd, ModNone)
	if h.changes != 0 {
		t.Errorf("changes = %d, want 0 with no collapsible pane", h.changes)
	}
}

func TestSplitterKeyEnterTogglesCollapse(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyEnter, ModNone)
	if h.state.Collapsed != SplitterCollapseFirst {
		t.Fatalf("collapsed = %v, want first", h.state.Collapsed)
	}
	h.key(t, "sp", KeyEnter, ModNone)
	if h.state.Collapsed != splitterCollapseNone {
		t.Errorf("collapsed after second Enter = %v, want none",
			h.state.Collapsed)
	}
}

// An arrow after a collapse restores the split, so the pane cannot be
// left collapsed and simultaneously reporting a fresh ratio.
func TestSplitterKeyArrowClearsCollapse(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyHome, ModNone)
	h.key(t, "sp", KeyRight, ModNone)
	if h.state.Collapsed != splitterCollapseNone {
		t.Errorf("collapsed = %v, want none after arrow", h.state.Collapsed)
	}
	first, _, _ := h.parts(t, "sp")
	if first.Shape.Width <= 0 {
		t.Error("first pane still collapsed after arrow key")
	}
}

// Space toggles collapse, driven the way a backend drives it: an
// EventKeyDown carrying KeyCode == KeySpace and CharCode == 0. The
// handler used to test CharCode instead, which no backend sets on a
// keydown (only EventChar carries it), so the spacebar did nothing.
func TestSplitterKeySpaceTogglesCollapse(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeySpace, ModNone)
	if h.state.Collapsed != SplitterCollapseFirst {
		t.Fatalf("collapsed = %v, want first", h.state.Collapsed)
	}
	h.key(t, "sp", KeySpace, ModNone)
	if h.state.Collapsed != splitterCollapseNone {
		t.Errorf("collapsed after second Space = %v, want none",
			h.state.Collapsed)
	}
}

// A space keydown never carries CharCode, so nothing may depend on it.
func TestSplitterKeySpaceIgnoresCharCode(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	if err := h.w.testFocus("sp"); err != nil {
		t.Fatalf("focus: %v", err)
	}
	h.send(&Event{Type: EventKeyDown, CharCode: charSpace})
	if h.changes != 0 {
		t.Errorf("changes = %d, want 0: a keydown with no KeyCode "+
			"must not toggle", h.changes)
	}
}

func TestSplitterKeySpaceWithModifierIgnored(t *testing.T) {
	a, b := splitterTwoPanes()
	a.collapsible = true
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeySpace, ModCtrl)
	if h.changes != 0 {
		t.Errorf("changes = %d, want 0 for Ctrl-Space", h.changes)
	}
}

func TestSplitterKeyUnrelatedKeyIgnored(t *testing.T) {
	a, b := splitterTwoPanes()
	h := splitterKeyHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	h.key(t, "sp", KeyEscape, ModNone)
	if h.changes != 0 {
		t.Errorf("changes = %d, want 0 for Escape", h.changes)
	}
}

func TestSplitterKeyDisabledIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "sp", disabled: true, handleSize: 9}
	e := Event{Type: EventKeyDown, KeyCode: KeyRight}
	splitterOnKeydown(core, &e, h.w)
	if e.IsHandled {
		t.Error("disabled splitter consumed a key")
	}
}

func TestSplitterKeyMissingLayoutIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "gone", handleSize: 9}
	e := Event{Type: EventKeyDown, KeyCode: KeyRight}
	splitterOnKeydown(core, &e, h.w)
	if e.IsHandled {
		t.Error("key on a missing splitter consumed the event")
	}
}

// --- Collapse buttons ---

func splitterCollapsibleCfg(id string) SplitterCfg {
	a, b := splitterTwoPanes()
	a.collapsible = true
	b.collapsible = true
	return SplitterCfg{
		ID:                  id,
		Ratio:               SomeF(0.5),
		ShowCollapseButtons: true,
		First:               a,
		Second:              b,
	}
}

func TestSplitterCollapseButtonsRendered(t *testing.T) {
	h := newSplitterHarness(t, splitterCollapsibleCfg("sp"))
	for _, id := range []string{"sp:button:1", "sp:button:2"} {
		if _, ok := h.w.layout.FindByID(id); !ok {
			t.Errorf("collapse button %q not rendered", id)
		}
	}
}

// Without ShowCollapseButtons the handle carries only the grip, so the
// button IDs must not exist — a stale one would collide on a second
// splitter in the same window.
func TestSplitterCollapseButtonsHiddenByDefault(t *testing.T) {
	cfg := splitterCollapsibleCfg("sp")
	cfg.ShowCollapseButtons = false
	h := newSplitterHarness(t, cfg)
	if _, ok := h.w.layout.FindByID("sp:button:1"); ok {
		t.Error("collapse button rendered with ShowCollapseButtons false")
	}
}

// ShowCollapseButtons alone is not enough: a pane must also be
// collapsible, otherwise the button would toggle nothing.
func TestSplitterCollapseButtonsNeedCollapsiblePane(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID: "sp", ShowCollapseButtons: true, First: a, Second: b,
	})
	if _, ok := h.w.layout.FindByID("sp:button:1"); ok {
		t.Error("collapse button rendered for a non-collapsible pane")
	}
}

func TestSplitterCollapseButtonOnlyForCollapsiblePane(t *testing.T) {
	cfg := splitterCollapsibleCfg("sp")
	cfg.Second.collapsible = false
	h := newSplitterHarness(t, cfg)
	if _, ok := h.w.layout.FindByID("sp:button:1"); !ok {
		t.Error("first button missing")
	}
	if _, ok := h.w.layout.FindByID("sp:button:2"); ok {
		t.Error("second button rendered for a non-collapsible pane")
	}
}

func TestSplitterCollapseButtonClickCollapsesAndRestores(t *testing.T) {
	h := newSplitterHarness(t, splitterCollapsibleCfg("sp"))
	if err := h.w.TestClick("sp:button:1"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if h.state.Collapsed != SplitterCollapseFirst {
		t.Fatalf("collapsed = %v, want first", h.state.Collapsed)
	}
	_, _, second := h.parts(t, "sp")
	nearF(t, "second width", second.Shape.Width,
		splitterTestW-splitterTestHandle, 0.5)

	// Clicking the same button again restores the pre-collapse ratio,
	// which the handler carried through untouched.
	if err := h.w.TestClick("sp:button:1"); err != nil {
		t.Fatalf("TestClick (restore): %v", err)
	}
	if h.state.Collapsed != splitterCollapseNone {
		t.Errorf("collapsed = %v, want none", h.state.Collapsed)
	}
	nearF(t, "restored ratio", h.state.Ratio, 0.5, 0.01)
}

func TestSplitterCollapseButtonSecondPane(t *testing.T) {
	h := newSplitterHarness(t, splitterCollapsibleCfg("sp"))
	if err := h.w.TestClick("sp:button:2"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if h.state.Collapsed != SplitterCollapseSecond {
		t.Errorf("collapsed = %v, want second", h.state.Collapsed)
	}
	first, _, _ := h.parts(t, "sp")
	nearF(t, "first width", first.Shape.Width,
		splitterTestW-splitterTestHandle, 0.5)
}

// Collapsing the first pane while the second is collapsed switches sides
// rather than clearing the state.
func TestSplitterCollapseButtonSwitchesSides(t *testing.T) {
	h := newSplitterHarness(t, splitterCollapsibleCfg("sp"))
	if err := h.w.TestClick("sp:button:2"); err != nil {
		t.Fatalf("TestClick(second): %v", err)
	}
	if err := h.w.TestClick("sp:button:1"); err != nil {
		t.Fatalf("TestClick(first): %v", err)
	}
	if h.state.Collapsed != SplitterCollapseFirst {
		t.Errorf("collapsed = %v, want first", h.state.Collapsed)
	}
}

func TestSplitterButtonClickNonCollapsibleIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "sp", handleSize: 9}
	e := Event{Type: EventMouseDown}
	splitterOnButtonClick(core, SplitterCollapseFirst, &e, h.w)
	if e.IsHandled {
		t.Error("button click consumed for a non-collapsible pane")
	}
}

func TestSplitterButtonClickDisabledIsInert(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{
		id: "sp", disabled: true, handleSize: 9,
		first: splitterPaneCore{collapsible: true},
	}
	e := Event{Type: EventMouseDown}
	splitterOnButtonClick(core, SplitterCollapseFirst, &e, h.w)
	if e.IsHandled {
		t.Error("disabled splitter consumed a button click")
	}
}

// --- Button icons ---

func TestSplitterButtonIconAllStates(t *testing.T) {
	both := splitterPaneCore{collapsible: true}
	tests := []struct {
		name      string
		orient    splitterOrientation
		collapsed SplitterCollapsed
		target    SplitterCollapsed
		want      string
	}{
		{"h/open/first", SplitterHorizontal, splitterCollapseNone,
			SplitterCollapseFirst, "◀"},
		{"h/open/second", SplitterHorizontal, splitterCollapseNone,
			SplitterCollapseSecond, "▶"},
		{"h/first-collapsed/first", SplitterHorizontal,
			SplitterCollapseFirst, SplitterCollapseFirst, "▶"},
		{"h/second-collapsed/second", SplitterHorizontal,
			SplitterCollapseSecond, SplitterCollapseSecond, "◀"},
		{"v/open/first", SplitterVertical, splitterCollapseNone,
			SplitterCollapseFirst, "▲"},
		{"v/open/second", SplitterVertical, splitterCollapseNone,
			SplitterCollapseSecond, "▼"},
		{"v/first-collapsed/first", SplitterVertical,
			SplitterCollapseFirst, SplitterCollapseFirst, "▼"},
		{"v/second-collapsed/second", SplitterVertical,
			SplitterCollapseSecond, SplitterCollapseSecond, "▲"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core := &splitterCore{
				orientation: tt.orient,
				collapsed:   tt.collapsed,
				first:       both,
				second:      both,
			}
			if got := splitterButtonIcon(core, tt.target); got != tt.want {
				t.Errorf("icon = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Layout query helpers ---

func TestSplitterMainSizePicksAxis(t *testing.T) {
	layout := Layout{Shape: &Shape{Width: 120, Height: 40}}
	if got := splitterMainSize(&layout, SplitterHorizontal); got != 120 {
		t.Errorf("horizontal main = %g, want 120", got)
	}
	if got := splitterMainSize(&layout, SplitterVertical); got != 40 {
		t.Errorf("vertical main = %g, want 40", got)
	}
}

func TestSplitterHandleSizeFromLayout(t *testing.T) {
	withHandle := Layout{
		Shape: &Shape{},
		Children: []Layout{
			{Shape: &Shape{}},
			{Shape: &Shape{Width: 7, Height: 11}},
		},
	}
	if got := splitterHandleSizeFromLayout(&withHandle,
		SplitterHorizontal, 99); got != 7 {
		t.Errorf("horizontal = %g, want 7", got)
	}
	if got := splitterHandleSizeFromLayout(&withHandle,
		SplitterVertical, 99); got != 11 {
		t.Errorf("vertical = %g, want 11", got)
	}
	// A tree without the handle child falls back to the configured size.
	bare := Layout{Shape: &Shape{}}
	if got := splitterHandleSizeFromLayout(&bare,
		SplitterHorizontal, 99); got != 99 {
		t.Errorf("fallback = %g, want 99", got)
	}
}

func TestSplitterHandleSizeClamps(t *testing.T) {
	if got := splitterHandleSize(0, 100); got != 1 {
		t.Errorf("zero handle = %g, want 1 (never invisible)", got)
	}
	if got := splitterHandleSize(20, 10); got != 10 {
		t.Errorf("oversized handle = %g, want 10", got)
	}
	if got := splitterHandleSize(20, 0); got != 20 {
		t.Errorf("unsized parent = %g, want 20", got)
	}
}

func TestSplitterLimitMaxTreatsNonPositiveAsUnbounded(t *testing.T) {
	if got := splitterLimitMax(0, 50); got != 50 {
		t.Errorf("zero max = %g, want available 50", got)
	}
	if got := splitterLimitMax(-5, 50); got != 50 {
		t.Errorf("negative max = %g, want available 50", got)
	}
	if got := splitterLimitMax(500, 50); got != 50 {
		t.Errorf("max beyond available = %g, want 50", got)
	}
}

func TestSplitterStepFallback(t *testing.T) {
	if got := splitterStep(0); got != 0.02 {
		t.Errorf("zero step = %g, want the 0.02 safety net", got)
	}
	if got := splitterStep(0.5); got != 0.5 {
		t.Errorf("step = %g, want 0.5", got)
	}
}

func TestSplitterCurrentRatioUsesLiveLayout(t *testing.T) {
	a, b := splitterTwoPanes()
	a.MinSize = 200
	h := newSplitterHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.1), First: a, Second: b,
	})
	core := &splitterCore{
		id: "sp", ratio: 0.1, handleSize: splitterTestHandle,
		first: splitterPaneCore{minSize: 200},
	}
	avail := float32(splitterTestW - splitterTestHandle)
	nearF(t, "current ratio", splitterCurrentRatio(core, h.w),
		200/avail, 0.001)
}

// With no layout to measure there is nothing to clamp against, so the
// stored ratio is returned normalized rather than guessed at.
func TestSplitterCurrentRatioFallsBackToStored(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	core := &splitterCore{id: "gone", ratio: 1.8}
	if got := splitterCurrentRatio(core, h.w); got != 1 {
		t.Errorf("ratio = %g, want 1 (normalized)", got)
	}
}

func TestSplitterSetCursorPerOrientation(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	splitterSetCursor(SplitterHorizontal, w)
	if w.viewState.mouseCursor != CursorResizeEW {
		t.Errorf("horizontal cursor = %v, want EW", w.viewState.mouseCursor)
	}
	splitterSetCursor(SplitterVertical, w)
	if w.viewState.mouseCursor != CursorResizeNS {
		t.Errorf("vertical cursor = %v, want NS", w.viewState.mouseCursor)
	}
}

// An ID-less splitter cannot be focused, so emitting a change must not
// try to move focus to the empty string.
func TestSplitterFocusSkippedWithoutID(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.SetFocus("elsewhere")
	splitterFocus(&splitterCore{}, w)
	if w.FocusID() != "elsewhere" {
		t.Errorf("focus = %q, want it left alone", w.FocusID())
	}
}

func TestSplitterEmitChangeNormalizesAndConsumes(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	var gotRatio float32
	var gotCollapsed SplitterCollapsed
	core := &splitterCore{
		onChange: func(r float32, c SplitterCollapsed, _ EventCtx) {
			gotRatio, gotCollapsed = r, c
		},
	}
	e := Event{Type: EventKeyDown}
	splitterEmitChange(core, 3.5, SplitterCollapsed(42), &e, w)
	if gotRatio != 1 {
		t.Errorf("ratio = %g, want 1", gotRatio)
	}
	if gotCollapsed != splitterCollapseNone {
		t.Errorf("collapsed = %v, want none", gotCollapsed)
	}
	if !e.IsHandled {
		t.Error("emit did not consume the event")
	}
}

// A splitter with no OnChange is legal (a fixed 50/50 split); driving it
// must not panic on the nil callback.
func TestSplitterEmitChangeNilCallback(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	e := Event{Type: EventKeyDown}
	splitterEmitChange(&splitterCore{}, 0.5, splitterCollapseNone, &e, w)
	if !e.IsHandled {
		t.Error("emit did not consume the event")
	}
}

// --- Style defaults ---

// The handle takes its border and radius from the theme. These were the
// only style fields applySplitterDefaults skipped, so the theme built
// them and nothing ever read them.
func TestSplitterHandleTakesBorderAndRadiusFromStyle(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	_, handle, _ := h.parts(t, "sp")
	nearF(t, "handle border", handle.Shape.SizeBorder,
		defaultSplitterStyle.SizeBorder, 0.01)
	nearF(t, "handle radius", handle.Shape.Radius,
		defaultSplitterStyle.Radius, 0.01)
	if handle.Shape.ColorBorder != defaultSplitterStyle.colorHandleBorder {
		t.Errorf("handle border color = %v, want %v",
			handle.Shape.ColorBorder,
			defaultSplitterStyle.colorHandleBorder)
	}
}

// An explicit zero still means "no border" — the default must not
// overwrite a caller who asked for a flat handle.
func TestSplitterExplicitBorderWinsOverStyle(t *testing.T) {
	a, b := splitterTwoPanes()
	h := newSplitterHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
		SizeBorder: SomeF(0),
		Radius:     SomeF(0),
	})
	_, handle, _ := h.parts(t, "sp")
	nearF(t, "handle border", handle.Shape.SizeBorder, 0, 0.01)
	nearF(t, "handle radius", handle.Shape.Radius, 0, 0.01)
}

func TestSplitterOrientationMarshalTextUnknown(t *testing.T) {
	if _, err := splitterOrientation(9).MarshalText(); err == nil {
		t.Error("expected error for an out-of-range orientation")
	}
}

// Two minimums that cannot both be satisfied invert the bounds
// (lower > upper). The clamp must still return a value inside the
// available space rather than propagate the inversion.
func TestSplitterClampFirstSizeWithImpossibleMinimums(t *testing.T) {
	core := &splitterCore{
		first:  splitterPaneCore{minSize: 80},
		second: splitterPaneCore{minSize: 80},
	}
	const available = 100
	for _, target := range []float32{0, 50, 100} {
		got := splitterClampFirstSize(core, available, target)
		if got < 0 || got > available {
			t.Errorf("target %g → %g, outside [0,%d]",
				target, got, available)
		}
	}
	// Bounds are (20, 80) inverted; the clamp swaps them.
	nearF(t, "below range",
		splitterClampFirstSize(core, available, 0), 20, 0.01)
	nearF(t, "above range",
		splitterClampFirstSize(core, available, 100), 80, 0.01)
}

// The pane re-layout rebases child positions relative to the pane. A
// child of an axis-less container keeps its offset from the old parent
// origin instead of being reset to zero, which is the branch a pane
// containing a Canvas exercises.
func TestSplitterAmendLayoutRebasesAxislessDescendants(t *testing.T) {
	a, b := splitterTwoPanes()
	a.Content = []View{
		Canvas(ContainerCfg{
			ID:      "inner",
			Sizing:  FixedFixed,
			Width:   40,
			Height:  20,
			Padding: NoPadding,
			Content: []View{Rectangle(RectangleCfg{
				Width: 10, Height: 10, Sizing: FixedFixed,
			})},
		}),
	}
	h := newSplitterHarness(t, SplitterCfg{
		ID: "sp", Ratio: SomeF(0.5), First: a, Second: b,
	})
	inner := h.find(t, "sp:pane:first:inner")
	if len(inner.Children) != 1 {
		t.Fatalf("inner children = %d, want 1", len(inner.Children))
	}
	// Positions are absolute, so the child must land inside the canvas.
	// A missed rebase leaves it at a stale coordinate outside the box.
	nearF(t, "canvas x", inner.Shape.X, 0, 0.01)
	child := inner.Children[0].Shape
	if child.X < inner.Shape.X ||
		child.X+child.Width > inner.Shape.X+inner.Shape.Width {
		t.Errorf("canvas child spans %g..%g, outside the canvas %g..%g",
			child.X, child.X+child.Width,
			inner.Shape.X, inner.Shape.X+inner.Shape.Width)
	}
}

// An ID-less splitter cannot take focus, so the drag's mouse-up must
// unlock without trying to focus the empty string.
func TestSplitterHandleReleaseWithoutIDStillUnlocks(t *testing.T) {
	h := newSplitterHarness(t, SplitterCfg{ID: "sp"})
	h.w.SetFocus("elsewhere")
	core := &splitterCore{id: "sp", handleSize: splitterTestHandle}
	e := Event{Type: EventMouseDown, MouseButton: MouseLeft}
	splitterOnHandleClick(core, &e, h.w)
	if !h.w.mouseIsLocked() {
		t.Fatal("handle click did not lock the mouse")
	}
	up := Event{Type: EventMouseUp, MouseButton: MouseLeft}
	h.w.viewState.mouseLock.MouseUp(EventCtx{nil, &up, h.w})
	if h.w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	if h.w.FocusID() != "elsewhere" {
		t.Errorf("focus = %q, want it left alone", h.w.FocusID())
	}
}

// --- Identity ---

// Two splitters in one window must not collide: every inner ID is scoped
// under the splitter's own ID.
func TestSplitterIDsAreUniquePerInstance(t *testing.T) {
	a, b := splitterTwoPanes()
	w := NewTestWindow(WindowCfg{Width: splitterTestW, Height: splitterTestH})
	w.TestRender(func(_ *Window) View {
		mk := func(id string) View {
			cfg := splitterCollapsibleCfg(id)
			cfg.First, cfg.Second = a, b
			cfg.First.collapsible = true
			cfg.Second.collapsible = true
			return Splitter(cfg)
		}
		return Column(ContainerCfg{
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content:    []View{mk("top"), mk("bottom")},
		})
	})
	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Errorf("duplicate IDs: %v", dups)
	}
}
