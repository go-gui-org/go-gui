package gui

import (
	"errors"
	"fmt"
)

// This file is the app-testing API (developer-ergonomics spec §4.6).
//
// Why it lives in package gui rather than a gui/guitest subpackage:
// driving a click means reaching Shape.events, which is unexported and
// should stay that way. A sibling package could only get there if the
// field were exported permanently, for the benefit of tests, on the
// single hottest struct in the library. §9 Q1 resolved this as
// "package gui".
//
// Why these drive Window.EventFn rather than calling the callback
// directly: the callback is the easy half. The interesting failures are
// in the other half — the widget never joined focus traversal, an
// overlay swallowed the click, the container clamped the scroll offset.
// Synthesizing an Event and pushing it through real dispatch exercises
// all of that; reaching into Shape.events would test only the part that
// was never broken.
//
// Targeting is by ID (§9 Q2). A coordinate-based TestClickAt is a
// separate future name, not an overload of these.

// Errors reported by the Test* methods. All are returned wrapped with
// the offending ID, so callers should match with errors.Is.
var (
	// ErrTestNoSuchID means no layout in the current tree carries the
	// requested ID. Usually one of: the view never rendered, the ID is
	// misspelled, or the widget is inside a collapsed/hidden branch that
	// the view function did not emit this frame.
	ErrTestNoSuchID = errors.New("gui: no widget with that ID")

	// ErrTestDisabled means the widget exists but is Disabled. Event
	// traversal skips disabled subtrees entirely (isChildEnabled), so
	// synthesizing the event would silently do nothing.
	ErrTestDisabled = errors.New("gui: widget is disabled")

	// ErrTestNotFocusable means the widget cannot hold focus, so a
	// keyboard event addressed to it can never be delivered. Requires
	// both Focusable and a non-empty ID (isFocusedTarget), and tab
	// order additionally requires !FocusSkip.
	ErrTestNotFocusable = errors.New("gui: widget is not focusable")

	// ErrTestNoHandler means the widget has no handler that could
	// respond to the synthesized event.
	ErrTestNoHandler = errors.New("gui: widget has no handler for that event")

	// ErrTestNotVisible means the widget has an empty clip rectangle, so
	// no coordinate hits it. A widget scrolled out of view, sized to
	// zero, or clipped away by an ancestor lands here.
	ErrTestNotVisible = errors.New("gui: widget has no visible area")

	// ErrTestUnhandled means the event was dispatched and reached the
	// end of traversal with nothing claiming it. The widget is present
	// and hittable, but something above it in z-order absorbed the
	// event, or no handler along the way called ctx.Consume().
	ErrTestUnhandled = errors.New("gui: event was not handled by any widget")

	// ErrTestNoScrollRoom means the widget is Scrollable but its
	// content fits inside it on the requested axis, so there is
	// nowhere to scroll. Separated from ErrTestUnhandled because the
	// two call for opposite fixes: this one says the fixture is wrong
	// (give the container more content, or less height), whereas
	// ErrTestUnhandled from a scroll says the container is real but
	// already pinned at its limit.
	ErrTestNoScrollRoom = errors.New("gui: scroll container has no room to scroll")
)

// TabDirection selects forward or backward traversal for TestTab.
type TabDirection int

const (
	// TabForward is plain Tab.
	TabForward TabDirection = iota
	// TabBackward is Shift-Tab.
	TabBackward
)

// testWindowSize is the default viewport for a test window. Arbitrary,
// but large enough that a FillFill root does not clip typical content
// down to nothing — a zero-size window would make every TestClick fail
// with ErrTestNotVisible for reasons that have nothing to do with the
// widget under test.
const testWindowSize = 800

// NewTestWindow builds a Window suitable for headless tests: no
// backend, no platform, no run loop.
//
// Differences from NewWindow, both of which exist because there is no
// backend to do the work:
//
//   - Width and Height default to testWindowSize when zero, so a
//     FillFill root has something to fill.
//   - cfg.OnInit is invoked before returning. The backend normally calls
//     it at window-open time, and an app that installs its view there
//     would otherwise render nothing.
//
// The injected interfaces (TextMeasurer, SvgParser, NativePlatform) stay
// nil, exactly as in the existing package tests. Layout therefore uses
// the no-measurer fallbacks: text extents are approximations, so assert
// on structure, state and focus, not on pixel widths.
func NewTestWindow(cfg WindowCfg) *Window {
	if cfg.Width == 0 {
		cfg.Width = testWindowSize
	}
	if cfg.Height == 0 {
		cfg.Height = testWindowSize
	}
	w := NewWindow(cfg)
	if cfg.OnInit != nil {
		cfg.OnInit(w)
	}
	return w
}

// TestRender installs view as the window's view generator and runs one
// full frame: view function, layout arrange, renderer build. It returns
// the composed root layout.
//
// Passing nil re-runs the frame with whatever generator is already
// installed — the usual way to observe the tree after a Test* action
// has changed state.
//
// The returned pointer is only valid until the next frame. Every Test*
// action below settles a frame when the event dirtied the window, which
// rebuilds the tree in place, so a *Layout captured before an action
// must not be read after it. Call TestRender(nil) again instead.
func (w *Window) TestRender(view func(*Window) View) *Layout {
	if view != nil {
		w.UpdateView(view)
	}
	w.markLayoutRefresh()
	w.Update()
	return &w.layout
}

// settle rebuilds the frame if the event just dispatched dirtied the
// window. EventFn always ends with UpdateWindow, and the backend would
// pick that up on the next FrameFn; with no backend running, nothing
// would. Without this the caller observes pre-event geometry, and —
// more subtly — scroll offsets stay unclamped, because the clamp lives
// in layoutAdjustScrollOffsets during arrange, not in the scroll
// handler.
func (w *Window) settle() {
	if w.refreshLayout {
		w.Update()
		return
	}
	if w.refreshRenderOnly {
		w.updateRenderOnly()
	}
}

// testTarget resolves id to a layout and rejects the two states in
// which dispatching any event is meaningless.
func (w *Window) testTarget(id string) (*Layout, error) {
	ly, ok := w.layout.FindByID(id)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrTestNoSuchID, id)
	}
	if ly.Shape == nil {
		return nil, fmt.Errorf("%w: %q", ErrTestNoSuchID, id)
	}
	if ly.Shape.Disabled {
		return nil, fmt.Errorf("%w: %q", ErrTestDisabled, id)
	}
	return ly, nil
}

// testHitPoint returns a coordinate guaranteed to hit ly, or an error
// when no such coordinate exists.
//
// The center of shapeClip, not of the shape: PointInShape tests the
// clip rectangle, so a widget partly scrolled out of its container has
// a shape center that misses and a clip center that does not.
func testHitPoint(ly *Layout, id string) (x, y float32, err error) {
	c := ly.Shape.shapeClip
	if c.Width <= 0 || c.Height <= 0 {
		return 0, 0, fmt.Errorf("%w: %q", ErrTestNotVisible, id)
	}
	return c.X + c.Width/2, c.Y + c.Height/2, nil
}

// TestClick synthesizes a left-button press and release at the center of
// the widget's visible area, dispatched from the window root exactly as
// the backend would.
//
// Hit-testing, not the ID, decides who receives it. The ID only picks
// the coordinate. If a floating overlay covers the target, the overlay
// gets the click — which is the real behavior, and the reason this
// drives dispatch instead of calling the callback.
//
// Known gap: when the overlay consumes the click, this returns nil, not
// an error. Dispatch does not report which shape it delivered to, so
// "handled by the target" and "handled by something on top of the
// target" are indistinguishable from here. ErrTestUnhandled therefore
// catches only total non-delivery. Assert on the state the click was
// supposed to change; do not read a nil return as proof the target
// fired.
//
// OnClick fires on press (see mouseDownHandler), so the release is sent
// only for the benefit of OnMouseUp handlers and drag-end logic.
//
// Returns ErrTestNoHandler when the widget has neither an OnClick nor
// focusability, since a click on such a widget cannot have any effect
// worth asserting on.
func (w *Window) TestClick(id string) error {
	ly, err := w.testTarget(id)
	if err != nil {
		return err
	}
	hasClick := ly.Shape.hasEvents() && ly.Shape.events.OnClick != nil
	if !hasClick && !ly.Shape.Focusable {
		return fmt.Errorf("%w: %q has no OnClick and is not focusable",
			ErrTestNoHandler, id)
	}
	x, y, err := testHitPoint(ly, id)
	if err != nil {
		return err
	}
	down := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&down)
	// Re-resolve rather than reuse ly: the press may have changed state
	// (focus, popup dismissal), and settling rebuilds the tree, so the
	// release must be aimed at the post-press geometry.
	w.settle()
	up := Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&up)
	w.settle()
	if !down.IsHandled {
		return fmt.Errorf("%w: click on %q", ErrTestUnhandled, id)
	}
	return nil
}

// testFocusable resolves id and rejects it unless it can actually hold
// focus. Both conditions are required by isFocusedTarget; a widget with
// Focusable set but no ID is the silent no-op the requiredid analyzer
// and the RequireID panic exist to prevent.
func (w *Window) testFocusable(id string) (*Layout, error) {
	ly, err := w.testTarget(id)
	if err != nil {
		return nil, err
	}
	if !ly.Shape.Focusable {
		return nil, fmt.Errorf("%w: %q", ErrTestNotFocusable, id)
	}
	return ly, nil
}

// TestFocus moves keyboard focus to the widget, as clicking it or
// tabbing to it would.
//
// FocusSkip is not rejected: a skipped widget is out of tab order but
// can still be focused programmatically, which is what this does.
func (w *Window) TestFocus(id string) error {
	if _, err := w.testFocusable(id); err != nil {
		return err
	}
	w.SetFocus(id)
	w.settle()
	return nil
}

// TestKey focuses the widget and delivers one key press and release.
//
// Unlike TestClick this does not report ErrTestUnhandled. A focused
// widget legitimately ignores most keys, and keydownHandler also feeds
// global commands, tab traversal and container scrolling — "nothing
// consumed it" is normal, not a defect. Assert on the state change
// instead.
func (w *Window) TestKey(id string, key KeyCode, mods Modifier) error {
	if err := w.TestFocus(id); err != nil {
		return err
	}
	down := Event{Type: EventKeyDown, KeyCode: key, Modifiers: mods}
	w.EventFn(&down)
	w.settle()
	up := Event{Type: EventKeyUp, KeyCode: key, Modifiers: mods}
	w.EventFn(&up)
	w.settle()
	return nil
}

// TestType focuses the widget and delivers text one rune at a time as
// character events, the same granularity the backend produces.
//
// Character events, not key events: typing is EventChar, and an input
// that only handled EventKeyDown would wrongly appear to work if this
// sent key codes. Empty text focuses the widget and delivers nothing.
func (w *Window) TestType(id string, text string) error {
	if err := w.TestFocus(id); err != nil {
		return err
	}
	for _, r := range text {
		e := Event{Type: EventChar, CharCode: uint32(r)}
		w.EventFn(&e)
		w.settle()
	}
	return nil
}

// TestTab presses Tab (or Shift-Tab) and returns the ID that holds focus
// afterwards.
//
// Sent as a real key event rather than by calling NextFocusable, so a
// widget whose OnKeyDown swallows Tab is observed swallowing it: the
// returned ID is then unchanged, which is the bug the test is looking
// for.
//
// An empty returned ID means nothing is focused — typically no widget in
// the tree qualifies for tab order (Focusable, non-empty ID, !FocusSkip,
// !Disabled).
func (w *Window) TestTab(dir TabDirection) (focusedID string, err error) {
	mods := Modifier(0)
	if dir == TabBackward {
		mods = ModShift
	}
	e := Event{Type: EventKeyDown, KeyCode: KeyTab, Modifiers: mods}
	w.EventFn(&e)
	w.settle()
	return w.FocusID(), nil
}

// TestScroll synthesizes a scroll over the widget and applies it
// immediately. Positive dy scrolls up. The delta is multiplied by the
// theme's ScrollMultiplier, exactly as a real scroll is.
//
// Sent as a precise/trackpad scroll (ScrollPrecise true), not a discrete
// wheel, and that is not a detail a test can ignore: the discrete path
// does not move the offset at all. It arms an exponential ease
// (scrollSmoothBy) that lands over subsequent frames driven by the
// animation goroutine, which no headless test runs — and even if one
// did, clearHotMaps calls scrollSmoothReset on every view rebuild, so
// settling a frame would discard the in-flight ease. The precise path
// (scrollVertical/scrollHorizontal) writes the offset synchronously,
// which is the only behavior a test can assert on. A widget that
// branches on Event.ScrollPrecise will therefore see the trackpad
// branch here.
//
// The widget under the cursor is not guaranteed to receive it.
// mouseScrollHandler offers the event to the focused element's
// OnMouseScroll first and only then falls back to the shape under the
// cursor, so a focused scrollable elsewhere in the tree wins. Real
// behavior, faithfully reproduced; clear focus first if the test means
// to isolate the container.
//
// Returns ErrTestNoHandler when the widget is neither Scrollable nor
// has an OnMouseScroll, ErrTestNoScrollRoom when it is a scroll
// container whose content already fits, and ErrTestUnhandled when the
// event reached no one — including the case where it fell through to a
// scrollable already pinned at its limit.
func (w *Window) TestScroll(id string, dx, dy float32) error {
	ly, err := w.testTarget(id)
	if err != nil {
		return err
	}
	hasScroll := ly.Shape.hasEvents() && ly.Shape.events.OnMouseScroll != nil
	if !ly.Shape.Scrollable && !hasScroll {
		return fmt.Errorf("%w: %q is not Scrollable and has no OnMouseScroll",
			ErrTestNoHandler, id)
	}
	x, y, err := testHitPoint(ly, id)
	if err != nil {
		return err
	}
	e := Event{
		Type:          EventMouseScroll,
		ScrollPrecise: true, // instant path; see the doc comment
		MouseX:        x, MouseY: y, ScrollX: dx, ScrollY: dy,
	}
	w.EventFn(&e)
	w.settle()
	if !e.IsHandled {
		// Re-resolve: settle() rebuilt the tree, so the ly captured
		// above is stale (see TestRender's doc comment).
		if cur, rerr := w.testTarget(id); rerr == nil {
			if err = testScrollRoomErr(cur, id, dx, dy); err != nil {
				return err
			}
		}
		return fmt.Errorf("%w: scroll on %q", ErrTestUnhandled, id)
	}
	return nil
}

// testScrollRoomErr reports ErrTestNoScrollRoom when ly is a scroll
// container whose content fits on every axis the caller asked to
// scroll. Returns nil when there is room, leaving the caller to report
// the generic ErrTestUnhandled.
//
// Room is measured the same way layoutAdjustScrollOffsets clamps —
// viewport minus content — so this cannot disagree with the clamp that
// actually swallowed the scroll.
func testScrollRoomErr(ly *Layout, id string, dx, dy float32) error {
	if !ly.Shape.Scrollable {
		return nil
	}
	roomX := contentWidth(ly) - (ly.Shape.Width - ly.Shape.paddingWidth())
	roomY := contentHeight(ly) - (ly.Shape.Height - ly.Shape.paddingHeight())
	if dx != 0 && roomX > 0 {
		return nil
	}
	if dy != 0 && roomY > 0 {
		return nil
	}
	return fmt.Errorf(
		"%w: %q content is %.0fx%.0f inside a %.0fx%.0f viewport",
		ErrTestNoScrollRoom, id,
		contentWidth(ly), contentHeight(ly),
		ly.Shape.Width-ly.Shape.paddingWidth(),
		ly.Shape.Height-ly.Shape.paddingHeight(),
	)
}

// TestScrollOffset reads a scroll container's current offset.
//
// Values are the raw stored offsets, which are zero or negative: the
// offset is added to child positions, so scrolling down moves content up
// and y goes negative. layoutAdjustScrollOffsets clamps them during
// arrange, so what this returns is already bounded by content size.
//
// Returns ErrTestNoHandler when the widget is not Scrollable, since a
// non-scrollable widget has no offset rather than an offset of zero, and
// silently reporting 0 would make an over-scroll assertion pass for the
// wrong reason.
func (w *Window) TestScrollOffset(id string) (x, y float32, err error) {
	ly, err := w.testTarget(id)
	if err != nil {
		return 0, 0, err
	}
	if !ly.Shape.Scrollable {
		return 0, 0, fmt.Errorf("%w: %q is not Scrollable", ErrTestNoHandler, id)
	}
	// Read-only accessors: a query must not allocate the state maps as a
	// side effect of being asked about a container that never scrolled.
	if sx := w.scrollXRead(); sx != nil {
		x = sx.GetOr(id, 0)
	}
	if sy := w.scrollYRead(); sy != nil {
		y = sy.GetOr(id, 0)
	}
	return x, y, nil
}
