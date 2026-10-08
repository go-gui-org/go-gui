package gui

import (
	"math"
	"strings"
	"testing"
)

// Tests for idle pointer moves (#973). A mouse move that changes nothing the
// last arranged frame encodes must not ask for a layout rebuild. A move that
// could change it must still ask for one, or the frame on screen goes stale.

// moveAndFrame sends a mouse move through EventFn and runs one FrameFn. It
// returns how many times the view was generated in that frame.
func moveAndFrame(w *Window, gens *int, x, y float32) int {
	*gens = 0
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.FrameFn()
	return *gens
}

// countingView wraps a view generator so a test can count generations.
func countingView(gens *int, view func(*Window) View) func(*Window) View {
	return func(w *Window) View {
		*gens++
		return view(w)
	}
}

// A move inside a shape with no pointer handlers, with the hover target
// unchanged, generates nothing and counts as skipped.
func TestIdleMoveSkipsRebuild(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	skipped := w.idleMovesSkipped

	if n := moveAndFrame(w, &gens, x+1, y); n != 0 {
		t.Fatalf("idle move: %d generations, want 0", n)
	}
	if w.layoutPending() {
		t.Fatal("idle move left a layout refresh pending")
	}
	if got := w.idleMovesSkipped - skipped; got != 1 {
		t.Fatalf("idleMovesSkipped grew by %d, want 1", got)
	}
	// The pointer position still moves, so the next real pass hit-tests
	// where the pointer is now.
	if w.viewState.mousePosX != x+1 || w.viewState.pointerX != x+1 {
		t.Fatalf("pointer not recorded: mouse %v pointer %v, want %v",
			w.viewState.mousePosX, w.viewState.pointerX, x+1)
	}
}

// A move to another hover target rebuilds, so IsHovered answers anew.
func TestIdleMoveTargetChangeRebuilds(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	bx, by := hitPoint(t, w, "panel:b")
	if n := moveAndFrame(w, &gens, bx, by); n == 0 {
		t.Fatal("move to a new hover target did not rebuild")
	}
	if !w.IsHovered("panel:b") {
		t.Fatal("panel:b not hovered after the move")
	}
}

// An app OnHover may read the pointer position, so a move inside its shape
// rebuilds and the callback runs again.
func TestIdleMoveUserOnHoverRebuilds(t *testing.T) {
	gens, hovers := 0, 0
	w := newInteractionWindow(t, interactionOpts{
		gens:     &gens,
		onHoverA: func(EventCtx) { hovers++ },
	})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	hovers = 0
	if n := moveAndFrame(w, &gens, x+1, y); n == 0 {
		t.Fatal("move inside an OnHover shape did not rebuild")
	}
	if hovers == 0 {
		t.Fatal("OnHover did not run for the move")
	}
}

// Leaving an OnMouseLeave shape rebuilds so the leave fires.
func TestIdleMoveLeaveRebuilds(t *testing.T) {
	gens, leaves := 0, 0
	w := newInteractionWindow(t, interactionOpts{
		gens:     &gens,
		onLeaveA: func(EventCtx) { leaves++ },
	})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	moveAndFrame(w, &gens, 290, 190) // inside panel, outside a and b
	if leaves != 1 {
		t.Fatalf("OnMouseLeave fired %d times, want 1", leaves)
	}
}

// Regression: OnMouseLeave fired only when the pointer had been seen inside
// the shape this frame or the one before. Frames that run no arrange pass (a
// caret blink, an idle move) made the record look stale, and the leave was
// lost.
func TestMouseLeaveSurvivesFramesWithoutArrange(t *testing.T) {
	gens, leaves := 0, 0
	w := newInteractionWindow(t, interactionOpts{
		gens:     &gens,
		onLeaveA: func(EventCtx) { leaves++ },
	})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	for range 5 {
		w.FrameFn() // nothing pending: frameCount moves, nothing arranges
	}
	moveAndFrame(w, &gens, 290, 190)
	if leaves != 1 {
		t.Fatalf("OnMouseLeave fired %d times, want 1", leaves)
	}
}

// A shape under the pointer with OnMouseMove gets the event, and the frame
// rebuilds: the handler may have changed state.
func TestIdleMoveMouseMoveHandlerRebuilds(t *testing.T) {
	gens, moves := 0, 0
	view := countingView(&gens, func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Column(ContainerCfg{
				ID: "m", Sizing: FixedFixed, Width: 80, Height: 60,
				SizeBorder:  NoBorder,
				OnMouseMove: func(EventCtx) { moves++ },
			})},
		})
	})
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(view)
	x, y := hitPoint(t, w, "m")
	moveAndFrame(w, &gens, x, y)
	moves = 0
	if n := moveAndFrame(w, &gens, x+1, y); n == 0 {
		t.Fatal("move over an OnMouseMove shape did not rebuild")
	}
	if moves != 1 {
		t.Fatalf("OnMouseMove ran %d times, want 1", moves)
	}
}

// A move during a drag goes to the lock's handler and rebuilds.
func TestIdleMoveMouseLockRebuilds(t *testing.T) {
	gens, moves := 0, 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) { moves++ }})
	if n := moveAndFrame(w, &gens, x+1, y); n == 0 {
		t.Fatal("locked move did not rebuild")
	}
	if moves != 1 {
		t.Fatalf("lock MouseMove ran %d times, want 1", moves)
	}
}

// buttonWindow renders one clickable Button, with or without an app OnHover.
func buttonWindow(t *testing.T, gens *int, onHover func(EventCtx)) *Window {
	t.Helper()
	view := countingView(gens, func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Button(ButtonCfg{
				ID: "btn", Sizing: FixedFixed, Width: 120, Height: 40,
				OnClick: func(EventCtx) {},
				OnHover: onHover,
			})},
		})
	})
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(view)
	return w
}

// A Button's own hover look depends only on being hovered, not on where the
// pointer is inside it: moving inside it skips the rebuild and keeps the
// hover fill and the pointing-hand cursor.
func TestIdleMoveInsideButtonSkips(t *testing.T) {
	gens := 0
	w := buttonWindow(t, &gens, nil)
	x, y := hitPoint(t, w, "btn")
	moveAndFrame(w, &gens, x, y)
	ly, _ := w.layout.FindByID("btn")
	hoverFill := ly.Shape.Color
	if n := moveAndFrame(w, &gens, x+1, y); n != 0 {
		t.Fatalf("move inside a Button: %d generations, want 0", n)
	}
	if w.viewState.mouseCursor != CursorPointingHand {
		t.Fatalf("cursor = %v, want pointing hand", w.viewState.mouseCursor)
	}
	ly, _ = w.layout.FindByID("btn")
	if ly.Shape.Color != hoverFill {
		t.Fatalf("hover fill lost: %v, want %v", ly.Shape.Color, hoverFill)
	}
}

// An app OnHover on a Button may read the pointer, so the Button rebuilds.
func TestIdleMoveInsideButtonWithUserHoverRebuilds(t *testing.T) {
	gens := 0
	w := buttonWindow(t, &gens, func(EventCtx) {})
	x, y := hitPoint(t, w, "btn")
	moveAndFrame(w, &gens, x, y)
	if n := moveAndFrame(w, &gens, x+1, y); n == 0 {
		t.Fatal("move inside a Button with OnHover did not rebuild")
	}
}

// Moving into a WithTooltip wrapper rebuilds, so its amend hook starts the
// tooltip timer. Neither the wrapper nor its content carries an ID, so the
// hover target stays "" and only the pointerAmend mark forces the rebuild.
func TestIdleMoveIntoTooltipRebuilds(t *testing.T) {
	gens := 0
	view := countingView(&gens, func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Spacing: SpacingPx(0),
			Content: []View{
				Column(ContainerCfg{
					Sizing: FixedFixed, Width: 80, Height: 60,
					SizeBorder: NoBorder,
				}),
				WithTooltip(w, WithTooltipCfg{
					ID: "tip", Text: "hello",
					Content: []View{Column(ContainerCfg{
						Sizing: FixedFixed, Width: 80, Height: 60,
						SizeBorder: NoBorder,
					})},
				}),
			},
		})
	})
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(view)
	moveAndFrame(w, &gens, 10, 10) // the plain block above the wrapper
	if w.viewState.hoverTargetID != "" {
		t.Fatalf("hover target = %q, want none", w.viewState.hoverTargetID)
	}
	moveAndFrame(w, &gens, 10, 70) // inside the wrapper's content
	if w.viewState.tooltip.hoverID == "" {
		t.Fatal("tooltip hover not started: the move into the wrapper was skipped")
	}
}

// A keyboard-driven menu shows its highlight until the mouse moves; the
// first move clears the key-nav mode, which the frame must show.
func TestIdleMoveEndsMenuKeyNav(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	w.viewState.menuKeyNav = true
	if n := moveAndFrame(w, &gens, x+1, y); n == 0 {
		t.Fatal("move ending menu key-nav did not rebuild")
	}
	if w.viewState.menuKeyNav {
		t.Fatal("menuKeyNav still set")
	}
}

// A one-finger touch drag under the pan threshold, over a shape with no
// handlers, is idle too.
func TestIdleTouchMoveSkipsRebuild(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	w.EventFn(touchEvent(EventTouchesBegan, 1, x, y))
	w.settle()
	skipped := w.idleMovesSkipped
	gens = 0
	w.EventFn(touchEvent(EventTouchesMoved, 1, x+1, y))
	w.FrameFn()
	if gens != 0 {
		t.Fatalf("idle touch move: %d generations, want 0", gens)
	}
	if w.idleMovesSkipped-skipped != 1 {
		t.Fatal("idle touch move not counted")
	}
	if w.viewState.pointerX != x+1 {
		t.Fatalf("pointerX = %v, want %v", w.viewState.pointerX, x+1)
	}
}

// DebugRebuilds reports the moves it skipped in front of the next pass.
func TestDebugRebuildsCountsSkippedMoves(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	buf := captureDebugMask(t, DebugRebuilds)
	moveAndFrame(w, &gens, x+1, y)
	moveAndFrame(w, &gens, x+2, y)
	moveAndFrame(w, &gens, x+3, y)
	bx, by := hitPoint(t, w, "panel:b")
	moveAndFrame(w, &gens, bx, by)
	if !strings.Contains(buf.String(), "gui: skipped 3 idle moves\n") {
		t.Fatalf("log missing skip count:\n%s", buf.String())
	}
}

// The idle check runs on every pointer move, so it must not allocate.
func TestIdleMoveZeroAlloc(t *testing.T) {
	gens := 0
	w := buttonWindow(t, &gens, nil)
	x, y := hitPoint(t, w, "btn")
	moveAndFrame(w, &gens, x, y)
	// One event, reset per run: EventFn hands it to callbacks, so a
	// fresh local per run would be the test's own allocation.
	e := &Event{}
	allocs := testing.AllocsPerRun(100, func() {
		*e = Event{Type: EventMouseMove, MouseX: x + 1, MouseY: y}
		w.EventFn(e)
	})
	if w.layoutPending() {
		t.Fatal("idle move asked for a rebuild")
	}
	if allocs != 0 {
		t.Fatalf("idle move allocated %v times, want 0", allocs)
	}
}

// moveCursorWindow renders a shape whose OnMouseMove sets the pointing
// hand at event time, the way rtfMouseMove does over a link, beside empty
// space with no handlers. No ID anywhere, so the hover target stays "" and
// only the cursor tells the two positions apart.
func moveCursorWindow(t *testing.T, gens *int) *Window {
	t.Helper()
	view := countingView(gens, func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Column(ContainerCfg{
				Sizing: FixedFixed, Width: 80, Height: 60,
				SizeBorder: NoBorder,
				OnMouseMove: func(ctx EventCtx) {
					ctx.Window.SetMouseCursorPointingHand()
				},
			})},
		})
	})
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(view)
	return w
}

// Regression: a cursor an OnMouseMove set at event time stuck after the
// pointer left its shape, because the move out counted as idle and the
// arrow reset only runs for moves that are not idle.
func TestIdleMoveResetsEventTimeCursor(t *testing.T) {
	gens := 0
	w := moveCursorWindow(t, &gens)
	moveAndFrame(w, &gens, 10, 10) // inside the shape at the top left
	if w.viewState.mouseCursor != CursorPointingHand {
		t.Fatalf("over m: cursor = %v, want pointing hand", w.viewState.mouseCursor)
	}
	moveAndFrame(w, &gens, 300, 300) // empty space
	if w.viewState.mouseCursor != CursorArrow {
		t.Fatalf("after leaving m: cursor = %v, want arrow", w.viewState.mouseCursor)
	}
}

// Regression: a cursor a drag set stuck after the release, because the
// next move counted as idle.
func TestIdleMoveResetsDragCursor(t *testing.T) {
	gens := 0
	w := newInteractionWindow(t, interactionOpts{gens: &gens})
	x, y := hitPoint(t, w, "panel:a")
	moveAndFrame(w, &gens, x, y)
	w.MouseLock(MouseLockCfg{MouseMove: func(ctx EventCtx) {
		ctx.Window.SetMouseCursorEW()
	}})
	moveAndFrame(w, &gens, x+1, y)
	w.MouseUnlock()
	w.EventFn(&Event{Type: EventMouseUp, MouseX: x + 1, MouseY: y, MouseButton: MouseLeft})
	w.FrameFn()
	moveAndFrame(w, &gens, x+2, y)
	if w.viewState.mouseCursor != CursorArrow {
		t.Fatalf("after drag: cursor = %v, want arrow", w.viewState.mouseCursor)
	}
}

// A NaN position misses every shape at both points, so without a guard it
// would pass as idle and store NaN with no rebuild to follow.
func TestIdleMoveNaNRebuilds(t *testing.T) {
	gens := 0
	w := emptyWindow(t, &gens)
	moveAndFrame(w, &gens, 10, 10)
	// Control: the same window skips an ordinary move.
	if n := moveAndFrame(w, &gens, 20, 20); n != 0 {
		t.Fatalf("control move: %d generations, want 0", n)
	}
	nan := float32(math.NaN())
	if n := moveAndFrame(w, &gens, nan, nan); n != 1 {
		t.Fatalf("NaN move: %d generations, want 1", n)
	}
}

// The first move after the pointer leaves the window starts hover again,
// so it is never idle, even over a shape that reacts to nothing.
func TestIdleMoveAfterWindowLeaveRebuilds(t *testing.T) {
	gens := 0
	w := emptyWindow(t, &gens)
	moveAndFrame(w, &gens, 10, 10)
	w.EventFn(&Event{Type: EventMouseLeave})
	w.FrameFn()
	if n := moveAndFrame(w, &gens, 20, 20); n != 1 {
		t.Fatalf("move back into the window: %d generations, want 1", n)
	}
}

// An app OnEvent sees every unhandled event and may act on a move, so no
// move is idle while one is set.
func TestIdleMoveWithOnEventRebuilds(t *testing.T) {
	gens, seen := 0, 0
	w := emptyWindow(t, &gens)
	w.OnEvent = func(*Event, *Window) { seen++ }
	moveAndFrame(w, &gens, 10, 10)
	if n := moveAndFrame(w, &gens, 20, 20); n != 1 {
		t.Fatalf("move with OnEvent set: %d generations, want 1", n)
	}
	if seen == 0 {
		t.Fatal("OnEvent did not see the move")
	}
}

// Children of a rotated box are tested in its unrotated frame, as hover
// dispatch does. Tested in screen space, a move into the rotated child
// misses it at both points and would pass as idle.
func TestIdleMoveIntoRotatedHoverRebuilds(t *testing.T) {
	gens, hovers := 0, 0
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(countingView(&gens, func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{RotatedBox(RotatedBoxCfg{
				// A half turn keeps the box's size, so the hover child
				// at its left end shows at its right end on screen.
				QuarterTurns: 2,
				Content: Row(ContainerCfg{
					Sizing:  FixedFixed,
					Width:   200,
					Height:  40,
					Padding: NoPadding,
					Content: []View{Column(ContainerCfg{
						Sizing:  FixedFixed,
						Width:   40,
						Height:  40,
						OnHover: func(EventCtx) { hovers++ },
					})},
				}),
			})},
		})
	}))
	box := &w.layout.Children[0].Children[0]
	child := &box.Children[0].Children[0]
	if box.Shape.QuarterTurns != 2 || child.Shape.events.OnHover == nil {
		t.Fatal("layout is not the rotated box holding the hover child")
	}
	// The child's center in its own frame, carried to the screen. A half
	// turn is its own inverse.
	c := child.Shape.shapeClip
	x, y := rotateCoordsInverse(box.Shape, c.X+c.Width/2, c.Y+c.Height/2)
	if child.Shape.PointInShape(x, y) {
		t.Fatalf("(%v, %v) is in the child unrotated too; the test proves nothing", x, y)
	}
	moveAndFrame(w, &gens, 700, 700)
	hovers = 0
	if n := moveAndFrame(w, &gens, x, y); n != 1 {
		t.Fatalf("move into rotated child: %d generations, want 1", n)
	}
	if hovers == 0 {
		t.Fatal("OnHover did not fire in the rotated child")
	}
}

// emptyWindow renders one plain column that reacts to nothing, so every
// move over it is idle unless something else in the window says not.
func emptyWindow(t *testing.T, gens *int) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(countingView(gens, func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	}))
	return w
}

// quietTB is a GoldenTB that drops every report: the test below wants the
// window state TestGolden leaves behind, not the file comparison.
type quietTB struct{}

func (quietTB) Helper()               {}
func (quietTB) Logf(string, ...any)   {}
func (quietTB) Errorf(string, ...any) {}
func (quietTB) Fatalf(string, ...any) {}

// TestGolden arranges a frame hovered at HoverX, then puts the hover
// target back. The tree on screen is still the hovered one, so the next
// move must rebuild even where nothing reacts to it: a view that paints
// from IsHovered has no handler for the idle check to find. What covers it
// today is TestGolden's theme restore (pinTheme calls InvalidateLayout);
// this pins that, should the restore stop invalidating.
func TestIdleMoveAfterGoldenHoverRebuilds(t *testing.T) {
	gens := 0
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(countingView(&gens, func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Column(ContainerCfg{
				ID: "panel", Sizing: FixedFixed, Width: 100, Height: 40,
			})},
		})
	}))
	moveAndFrame(w, &gens, 300, 300)
	w.TestGolden(quietTB{}, GoldenCfg{Dir: t.TempDir(), Name: "hover", HoverX: 10, HoverY: 10})
	if n := moveAndFrame(w, &gens, 310, 300); n != 1 {
		t.Fatalf("move after a hovered golden frame: %d generations, want 1", n)
	}
}
