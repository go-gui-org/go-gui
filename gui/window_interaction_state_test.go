package gui

import "testing"

// Tests for build-time interaction state: Window.IsHovered and
// Window.IsPressed (issue #587). Each drives the real pipeline — a
// backend-shaped event through EventFn, then a settled frame — because
// the hover target is recorded by layoutArrange, not by the event.

// interactionView is the shared fixture. Main layer: an ID-less root
// holding panel (ID "panel") with two children "a" and "b", so their
// effective IDs are "panel:a" and "panel:b". Options add a covering
// float, disable "a", or swap "a" for a view that reads hover while it
// is built.
type interactionOpts struct {
	coverFloat bool
	innerFloat bool
	disableA   bool
	onHoverA   func(EventCtx)
	onLeaveA   func(EventCtx)
	buildTimeA bool
	gens       *int
}

func interactionView(o interactionOpts) func(*Window) View {
	return func(w *Window) View {
		if o.gens != nil {
			*o.gens++
		}
		var a View
		if o.buildTimeA {
			a = hoverPadView{}
		} else {
			a = leaveHookView{
				cfg: ContainerCfg{
					ID: "a", Sizing: FixedFixed, Width: 80, Height: 60,
					SizeBorder: NoBorder, Disabled: o.disableA,
					OnHover: o.onHoverA,
				},
				onLeave: o.onLeaveA,
			}
		}
		panelContent := []View{
			a,
			Column(ContainerCfg{
				ID: "b", Sizing: FixedFixed, Width: 80, Height: 60,
				SizeBorder: NoBorder,
			}),
		}
		if o.innerFloat {
			// No ID, lifted from inside panel: a switch knob that floats
			// over its track (#661). It covers all of panel.
			panelContent = append(panelContent, Column(ContainerCfg{
				Float: true, FloatAnchor: FloatTopLeft, FloatTieOff: FloatTopLeft,
				Sizing: FixedFixed, Width: 300, Height: 200, SizeBorder: NoBorder,
			}))
		}
		content := []View{
			Row(ContainerCfg{
				ID: "panel", Sizing: FixedFixed, Width: 300, Height: 200,
				Padding: PadAll(20), SizeBorder: NoBorder, Spacing: SomeF(10),
				Content: panelContent,
			}),
		}
		if o.coverFloat {
			// No ID, no handlers: purely a covering shape over panel.
			content = append(content, Column(ContainerCfg{
				Float: true, FloatAnchor: FloatTopLeft, FloatTieOff: FloatTopLeft,
				Sizing: FixedFixed, Width: 300, Height: 200, SizeBorder: NoBorder,
			}))
		}
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: content,
		})
	}
}

// leaveHookView builds a container and installs OnMouseLeave on its
// shape, which ContainerCfg does not expose.
type leaveHookView struct {
	cfg     ContainerCfg
	onLeave func(EventCtx)
}

func (v leaveHookView) Content() []View { return nil }

func (v leaveHookView) GenerateLayout(w *Window) Layout {
	cfg := v.cfg
	if v.onLeave != nil && cfg.OnHover == nil {
		// OnHover forces the events block to exist.
		cfg.OnHover = func(EventCtx) {}
	}
	ly := Column(cfg).GenerateLayout(w)
	if v.onLeave != nil {
		ly.Shape.events.OnMouseLeave = v.onLeave
	}
	return ly
}

// hoverPadView reads its own hover state while it is built and grows
// its inner padding when hovered. The outer size is fixed, so the look
// changes inside the bounds only — the rule the IsHovered doc states.
type hoverPadView struct{}

func (hoverPadView) Content() []View { return nil }

func (hoverPadView) GenerateLayout(w *Window) Layout {
	pad := PadAll(2)
	if w.IsHovered(w.EffID("a")) {
		pad = PadAll(6)
	}
	return Column(ContainerCfg{
		ID: "a", Sizing: FixedFixed, Width: 80, Height: 60,
		Padding: pad, SizeBorder: NoBorder,
		Content: []View{Column(ContainerCfg{Sizing: FillFill, SizeBorder: NoBorder})},
	}).GenerateLayout(w)
}

func newInteractionWindow(t *testing.T, o interactionOpts) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	w.TestRender(interactionView(o))
	return w
}

// hitPoint returns a point inside the shape with effective ID id
// without moving the pointer.
func hitPoint(t *testing.T, w *Window, id string) (float32, float32) {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape with effective ID %s", id)
	}
	x, y, err := testHitPoint(ly, id)
	if err != nil {
		t.Fatal(err)
	}
	return x, y
}

// 2.1: target and its ID-bearing ancestor hover; a sibling does not;
// nothing hovers before the pointer has moved.
func TestInteractionStateHoverTargetAndAncestor(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	// The pointer starts at (0,0), which is inside panel. No move has
	// arrived, so no shape may read as hovered.
	if w.IsHovered("panel") || w.IsHovered("panel:a") {
		t.Fatal("hovered before any mouse move")
	}
	hoverOver(t, w, "panel:a")
	if !w.IsHovered("panel:a") {
		t.Fatal(`IsHovered("panel:a") = false, want true`)
	}
	if !w.IsHovered("panel") {
		t.Fatal(`IsHovered("panel") = false, want true (ancestor)`)
	}
	if w.IsHovered("panel:b") {
		t.Fatal(`IsHovered("panel:b") = true, want false (sibling)`)
	}
	// A prefix that is not a whole segment must not match.
	if w.IsHovered("pan") {
		t.Fatal(`IsHovered("pan") = true, want false`)
	}
}

// 2.2: the leaf is not the effective ID.
func TestInteractionStateLeafIsNotEffectiveID(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	hoverOver(t, w, "panel:a")
	if w.IsHovered("a") {
		t.Fatal(`IsHovered("a") = true, want false (leaf, not effective ID)`)
	}
}

// 2.3: a visible dialog blocks shapes behind it.
func TestInteractionStateDialogBlocks(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hitPoint(t, w, "panel:a")
	w.Dialog(DialogCfg{Title: "t", Body: "b"})
	w.TestRender(nil)
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.settle()
	if w.IsHovered("panel:a") || w.IsHovered("panel") {
		t.Fatal("shape behind a visible dialog reads as hovered")
	}
}

// 2.4 + 2.12: a handler-less float over a shape blocks IsHovered, while
// OnHover still fires underneath it. The split is deliberate (semantic
// 11): OnHover is dispatch, IsHovered is visible state.
func TestInteractionStateFloatBlocksButOnHoverFires(t *testing.T) {
	fired := 0
	w := newInteractionWindow(t, interactionOpts{
		coverFloat: true,
		onHoverA:   func(EventCtx) { fired++ },
	})
	hoverOver(t, w, "panel:a")
	if fired == 0 {
		t.Fatal("OnHover did not fire under a handler-less float")
	}
	if w.IsHovered("panel:a") || w.IsHovered("panel") {
		t.Fatal("shape under a covering float reads as hovered")
	}
}

// #661: an ID-less float lifted from inside an ID-bearing widget hovers
// and presses that widget, the shape a click reaches. It still blocks
// the widget's own children below it.
func TestInteractionStateIDLessFloatTargetsLiftedFromAncestor(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{innerFloat: true})
	x, y := hoverOver(t, w, "panel:a")
	if !w.IsHovered("panel") {
		t.Fatal(`IsHovered("panel") = false under an ID-less float lifted from panel`)
	}
	if w.IsHovered("panel:a") {
		t.Fatal(`IsHovered("panel:a") = true, want false (covered by the float)`)
	}
	pressAt(w, MouseLeft, x, y)
	if !w.IsPressed("panel") {
		t.Fatal(`IsPressed("panel") = false under an ID-less float lifted from panel`)
	}
}

// 2.5: a disabled shape never hovers; its enabled ancestor does.
func TestInteractionStateDisabledNeverHovers(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{disableA: true})
	x, y := hitPoint(t, w, "panel:a")
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.settle()
	if w.IsHovered("panel:a") {
		t.Fatal("disabled shape reads as hovered")
	}
	if !w.IsHovered("panel") {
		t.Fatal("enabled ancestor of a disabled shape not hovered")
	}
}

// 2.6: a target change asks for one more layout pass, which FrameFn
// runs in the same frame; an unchanged target asks for nothing.
func TestInteractionStateTargetChangeInvalidatesOnce(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")

	gens = 0
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.FrameFn()
	if gens != 2 {
		t.Fatalf("hover change: %d generations in one FrameFn, want 2", gens)
	}

	gens = 0
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x + 1, MouseY: y})
	w.FrameFn()
	if gens != 1 {
		t.Fatalf("same target: %d generations in one FrameFn, want 1", gens)
	}
	if w.refreshLayout {
		t.Fatal("refreshLayout still set with an unchanged target")
	}
}

// 2.7: IsPressed follows the left button only and clears on release,
// on MouseCancel, and on SetView; a touch press counts.
func TestInteractionStatePressed(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hoverOver(t, w, "panel:a")

	pressAt(w, MouseLeft, x, y)
	if !w.IsPressed("panel:a") || !w.IsPressed("panel") {
		t.Fatal("left press not recorded on target and ancestor")
	}
	if w.IsPressed("panel:b") {
		t.Fatal("sibling reads as pressed")
	}
	releaseAt(w, MouseLeft, x, y)
	if w.IsPressed("panel:a") {
		t.Fatal("still pressed after release")
	}

	pressAt(w, MouseRight, x, y)
	if w.IsPressed("panel:a") {
		t.Fatal("right button reads as pressed")
	}
	releaseAt(w, MouseRight, x, y)

	pressAt(w, MouseLeft, x, y)
	w.MouseCancel()
	if w.IsPressed("panel:a") {
		t.Fatal("still pressed after MouseCancel")
	}

	pressAt(w, MouseLeft, x, y)
	w.SetView(interactionView(interactionOpts{}))
	if w.IsPressed("panel:a") {
		t.Fatal("still pressed after SetView")
	}
	w.TestRender(nil)

	w.EventFn(touchEvent(EventTouchesBegan, 1, x, y))
	w.settle()
	if !w.IsPressed("panel:a") {
		t.Fatal("touch press not recorded")
	}
	w.EventFn(touchEvent(EventTouchesEnded, 1, x, y))
	w.settle()
	if w.IsPressed("panel:a") {
		t.Fatal("still pressed after touch end")
	}
}

// 2.8: a press persists while the pointer moves off, with no lock.
func TestInteractionStatePressPersistsOffShape(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hoverOver(t, w, "panel:a")
	pressAt(w, MouseLeft, x, y)
	bx, by := hitPoint(t, w, "panel:b")
	w.EventFn(&Event{Type: EventMouseMove, MouseX: bx, MouseY: by, MouseButton: MouseLeft})
	w.settle()
	if !w.IsPressed("panel:a") {
		t.Fatal("press lost when the pointer moved off")
	}
	if w.IsHovered("panel:a") {
		t.Fatal("pressed shape still hovered after the pointer moved off")
	}
	if !w.IsHovered("panel:b") {
		t.Fatal("new shape under the pointer not hovered")
	}
}

// Semantic 3: under a mouse lock the hover target is frozen.
func TestInteractionStateMouseLockFreezesHover(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	hoverOver(t, w, "panel:a")
	bx, by := hitPoint(t, w, "panel:b")
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) {}})
	w.EventFn(&Event{Type: EventMouseMove, MouseX: bx, MouseY: by})
	w.settle()
	if !w.IsHovered("panel:a") || w.IsHovered("panel:b") {
		t.Fatal("hover target changed under a mouse lock")
	}
	w.MouseUnlock()
}

// 2.9 + 2.11: a view reads its hover state while it is built and the
// arranged tree shows the new look; the change stays inside the bounds,
// so the frame settles instead of looping.
func TestInteractionStateBuildTimeReadSettles(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{buildTimeA: true})
	if got := mustShape(t, w, "panel:a").Padding.Top; got != 2 {
		t.Fatalf("unhovered padding = %v, want 2", got)
	}
	hoverOver(t, w, "panel:a")
	// hoverOver settled one pass, which recorded the new target and
	// asked for a second.
	if !w.refreshLayout {
		t.Fatal("target change did not ask for another pass")
	}
	w.settle()
	if got := mustShape(t, w, "panel:a").Padding.Top; got != 6 {
		t.Fatalf("hovered padding = %v, want 6", got)
	}
	if w.refreshLayout {
		t.Fatal("inner-padding look did not settle: refreshLayout still set")
	}
}

// 2.10: the queries do not allocate.
func TestInteractionStateQueriesDoNotAllocate(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hoverOver(t, w, "panel:a")
	pressAt(w, MouseLeft, x, y)
	allocs := testing.AllocsPerRun(100, func() {
		_ = w.IsHovered("panel")
		_ = w.IsHovered("panel:a")
		_ = w.IsPressed("panel")
		_ = w.IsPressed("panel:b")
	})
	if allocs != 0 {
		t.Fatalf("queries allocated %v times per run, want 0", allocs)
	}
}

// 2.13: a touch lift clears hover left by an earlier pointer.
func TestInteractionStateTouchLiftClearsHover(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hoverOver(t, w, "panel:a")
	w.EventFn(touchEvent(EventTouchesBegan, 1, x, y))
	w.settle()
	w.EventFn(touchEvent(EventTouchesEnded, 1, x, y))
	w.settle()
	w.settle()
	if w.IsHovered("panel:a") || w.IsHovered("panel") {
		t.Fatal("hover stuck after touch lift")
	}
}

// 2.14: the pointer leaving the window clears hover, fires
// OnMouseLeave, and stops OnHover — focused or not.
func TestInteractionStateWindowExitClearsHover(t *testing.T) {
	for _, focused := range []bool{true, false} {
		hovers, leaves := 0, 0
		w := newInteractionWindow(t, interactionOpts{
			onHoverA: func(EventCtx) { hovers++ },
			onLeaveA: func(EventCtx) { leaves++ },
		})
		hoverOver(t, w, "panel:a")
		w.settle()
		if !focused {
			w.EventFn(&Event{Type: EventUnfocused})
			w.settle()
		}
		before := hovers
		w.EventFn(&Event{Type: EventMouseLeave})
		w.settle()
		w.settle()
		if w.IsHovered("panel:a") || w.IsHovered("panel") {
			t.Fatalf("focused=%v: hover stuck after window exit", focused)
		}
		if leaves == 0 {
			t.Fatalf("focused=%v: OnMouseLeave did not fire on window exit", focused)
		}
		if hovers != before {
			t.Fatalf("focused=%v: OnHover fired %d times after window exit",
				focused, hovers-before)
		}
	}
}

// A finger held on a shape hovers it, with no mouse move ever sent, so
// the "armed" look (IsPressed && IsHovered) shows on a touch screen too.
// Lifting the finger clears the hover.
func TestInteractionStateTouchHeldHovers(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hitPoint(t, w, "panel:a")
	w.EventFn(touchEvent(EventTouchesBegan, 1, x, y))
	w.settle()
	if !w.IsPressed("panel:a") || !w.IsHovered("panel:a") {
		t.Fatalf("held touch: pressed=%v hovered=%v, want both true",
			w.IsPressed("panel:a"), w.IsHovered("panel:a"))
	}
	w.EventFn(touchEvent(EventTouchesEnded, 1, x, y))
	w.settle()
	if w.IsHovered("panel:a") {
		t.Fatal("hover stuck after touch lift")
	}
}

// A press while the mouse is locked belongs to the drag and records
// nothing.
func TestInteractionStatePressUnderLockRecordsNothing(t *testing.T) {
	w := newInteractionWindow(t, interactionOpts{})
	x, y := hoverOver(t, w, "panel:a")
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) {}})
	pressAt(w, MouseLeft, x, y)
	if w.IsPressed("panel:a") {
		t.Fatal("press under a mouse lock was recorded")
	}
	w.MouseUnlock()
}

func TestTargetWithin(t *testing.T) {
	for _, c := range []struct {
		target, id string
		want       bool
	}{
		{"a:b", "a:b", true},
		{"a:b:c", "a:b", true},
		{"a:bc", "a:b", false},
		{"a", "a:b", false},
		{"", "", false},
		{"a", "", false},
	} {
		if got := targetWithin(c.target, c.id); got != c.want {
			t.Errorf("targetWithin(%q, %q) = %v, want %v", c.target, c.id, got, c.want)
		}
	}
}

// liftedFromTarget climbs past ID-less and disabled ancestors, yields ""
// for a root with no Parent, and stops on a Parent cycle.
func TestLiftedFromTarget(t *testing.T) {
	root := &Layout{Shape: &Shape{ID: "root"}}
	disabled := &Layout{Shape: &Shape{ID: "off", Disabled: true}, Parent: root}
	plain := &Layout{Shape: &Shape{}, Parent: disabled}
	noShape := &Layout{Parent: plain}

	if got := liftedFromTarget(&Layout{Shape: &Shape{}}); got != "" {
		t.Fatalf("nil Parent: got %q, want \"\"", got)
	}
	if got := liftedFromTarget(&Layout{Shape: &Shape{}, Parent: noShape}); got != "root" {
		t.Fatalf("climb: got %q, want \"root\"", got)
	}

	// A cycle of ID-less shapes must end at the depth cap.
	a := &Layout{Shape: &Shape{}}
	b := &Layout{Shape: &Shape{}, Parent: a}
	a.Parent = b
	if got := liftedFromTarget(&Layout{Shape: &Shape{}, Parent: a}); got != "" {
		t.Fatalf("cycle: got %q, want \"\"", got)
	}
}
