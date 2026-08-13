package gui

import (
	"errors"
	"strings"
	"testing"
)

// These tests exercise the app-testing API against real dispatch. They
// are deliberately written the way an app author would write them —
// build a view, act, assert on state — because the API's whole claim is
// that this style now works. §4.6 measured 63 example test functions of
// which zero assert that clicking changed anything.

// counterState is a minimal app state: one typed slot per window.
type counterState struct {
	count int
	text  string
	keys  []KeyCode
}

// newCounterWindow builds a window whose view is two buttons and an
// input, all with IDs. Returns the window with one frame already
// rendered.
func newCounterWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(w *Window) View {
		app := State[counterState](w)
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Button(ButtonCfg{
					ID:      "inc",
					Content: []View{Text(TextCfg{Text: "Increment"})},
					OnClick: func(ctx EventCtx) {
						State[counterState](ctx.Window).count++
					},
				}),
				Button(ButtonCfg{
					ID:      "dec",
					Content: []View{Text(TextCfg{Text: "Decrement"})},
					OnClick: func(ctx EventCtx) {
						State[counterState](ctx.Window).count--
					},
				}),
				Input(InputCfg{
					ID:   "name",
					Text: app.text,
					OnKeyDown: func(ctx EventCtx) {
						a := State[counterState](ctx.Window)
						a.keys = append(a.keys, ctx.Event.KeyCode)
					},
				}),
			},
		})
	})
	return w
}

func TestTestClickInvokesOnClick(t *testing.T) {
	w := newCounterWindow(t)
	app := State[counterState](w)

	if err := w.TestClick("inc"); err != nil {
		t.Fatalf("TestClick(inc) = %v, want nil", err)
	}
	if app.count != 1 {
		t.Fatalf("count = %d after one click, want 1", app.count)
	}
	// Twice more, to prove the frame settled between actions rather
	// than the first click happening to work on a stale tree.
	if err := w.TestClick("inc"); err != nil {
		t.Fatalf("TestClick(inc) second = %v, want nil", err)
	}
	if err := w.TestClick("dec"); err != nil {
		t.Fatalf("TestClick(dec) = %v, want nil", err)
	}
	if app.count != 1 {
		t.Fatalf("count = %d after +1+1-1, want 1", app.count)
	}
}

func TestTestClickUnknownID(t *testing.T) {
	w := newCounterWindow(t)
	err := w.TestClick("nope")
	if !errors.Is(err, errTestNoSuchID) {
		t.Fatalf("TestClick(nope) = %v, want ErrTestNoSuchID", err)
	}
}

func TestTestClickDisabled(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		return Button(ButtonCfg{
			ID:       "off",
			Disabled: true,
			Content:  []View{Text(TextCfg{Text: "Off"})},
			OnClick:  func(_ EventCtx) { t.Error("disabled button fired") },
		})
	})
	err := w.TestClick("off")
	if !errors.Is(err, errTestDisabled) {
		t.Fatalf("TestClick(off) = %v, want ErrTestDisabled", err)
	}
}

func TestTestClickNoHandler(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		// A plain container: no OnClick, not focusable. Clicking it
		// cannot do anything, so asking to click it is a test bug.
		return Column(ContainerCfg{
			ID:      "plain",
			Sizing:  FixedFixed,
			Width:   100,
			Height:  50,
			Content: []View{Text(TextCfg{Text: "inert"})},
		})
	})
	err := w.TestClick("plain")
	if !errors.Is(err, errTestNoHandler) {
		t.Fatalf("TestClick(plain) = %v, want ErrTestNoHandler", err)
	}
}

// A widget behind an overlay does not receive the click — the overlay
// does. This separates driving real dispatch from calling
// Shape.events.OnClick directly: the latter would fire the buried
// button and report success on an app the user cannot operate.
//
// Also pins TestClick's documented gap: the return is nil, because
// dispatch does not say who it delivered to and something did handle
// the event. A test must assert on state, not on the nil.
func TestTestClickBlockedByOverlay(t *testing.T) {
	var fired, overFired bool
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FixedFixed,
			Width:  400,
			Height: 400,
			Content: []View{
				Button(ButtonCfg{
					ID:      "under",
					Sizing:  FillFill,
					Content: []View{Text(TextCfg{Text: "Under"})},
					OnClick: func(_ EventCtx) { fired = true },
				}),
				// Float + OverDraw lifts this out of the column's
				// sizing and draws it on top. Sized explicitly: a
				// floating shape has no parent to Fill against, so
				// FillFill would collapse it to nothing and it would
				// cover the button in name only.
				Column(ContainerCfg{
					ID:       "over",
					Float:    true,
					OverDraw: true,
					Sizing:   FixedFixed,
					Width:    400,
					Height:   400,
					OnClick: func(ctx EventCtx) {
						overFired = true
						// An overlay absorbs what lands on it. Under
						// the one-rule model that is a Consume, not a
						// side effect of being consume-class.
						ctx.Consume()
					},
					Color: Hex(0x000000),
				}),
			},
		})
	})
	if err := w.TestClick("under"); err != nil {
		t.Fatalf("TestClick(under) = %v, want nil (the overlay handled it)", err)
	}
	if fired {
		t.Fatal("covered button fired; overlay did not absorb the click")
	}
	if !overFired {
		t.Fatal("overlay did not receive the click either")
	}
}

func TestTestClickFocusesFocusableWidget(t *testing.T) {
	w := newCounterWindow(t)
	if err := w.TestClick("name"); err != nil {
		t.Fatalf("TestClick(name) = %v, want nil", err)
	}
	if got := w.FocusID(); got != "name" {
		t.Fatalf("FocusID = %q after clicking the input, want %q", got, "name")
	}
}

func TestTestFocusMovesFocus(t *testing.T) {
	w := newCounterWindow(t)
	if err := w.testFocus("name"); err != nil {
		t.Fatalf("TestFocus(name) = %v, want nil", err)
	}
	if got := w.FocusID(); got != "name" {
		t.Fatalf("FocusID = %q, want %q", got, "name")
	}
}

// KeyF5 rather than a navigation key: Input consumes every key it
// handles itself (Home, End, arrows, editing keys) and only forwards
// the leftovers to Cfg.OnKeyDown. Testing with Home would assert
// nothing about delivery.
func TestTestKeyReachesOnKeyDown(t *testing.T) {
	w := newCounterWindow(t)
	app := State[counterState](w)
	if err := w.TestKey("name", KeyF5, 0); err != nil {
		t.Fatalf("TestKey = %v, want nil", err)
	}
	if len(app.keys) != 1 || app.keys[0] != KeyF5 {
		t.Fatalf("keys = %v, want [KeyF5]", app.keys)
	}
}

// The complement: a key the widget handles internally must drive the
// widget's own state, not the app callback. This is what proves TestKey
// goes through real dispatch rather than poking a callback.
func TestTestKeyDrivesWidgetState(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(w *Window) View {
		app := State[counterState](w)
		return Input(InputCfg{
			ID:     "name",
			Sizing: FillFit,
			Text:   app.text,
			OnTextChanged: func(s string, ctx EventCtx) {
				State[counterState](ctx.Window).text = s
			},
		})
	})
	if err := w.TestType("name", "abc"); err != nil {
		t.Fatalf("TestType = %v, want nil", err)
	}
	if got := getInputState(w, "name").CursorPos; got != 3 {
		t.Fatalf("CursorPos = %d after typing \"abc\", want 3", got)
	}
	if err := w.TestKey("name", KeyHome, 0); err != nil {
		t.Fatalf("TestKey(Home) = %v, want nil", err)
	}
	if got := getInputState(w, "name").CursorPos; got != 0 {
		t.Fatalf("CursorPos = %d after Home, want 0", got)
	}
}

func TestTestFocusNotFocusable(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:      "plain",
			Sizing:  FillFill,
			Content: []View{Text(TextCfg{Text: "x"})},
		})
	})
	err := w.testFocus("plain")
	if !errors.Is(err, errTestNotFocusable) {
		t.Fatalf("TestFocus(plain) = %v, want ErrTestNotFocusable", err)
	}
}

func TestTestTypeEntersText(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(w *Window) View {
		app := State[counterState](w)
		return Input(InputCfg{
			ID:     "name",
			Sizing: FillFit,
			Text:   app.text,
			OnTextChanged: func(s string, ctx EventCtx) {
				State[counterState](ctx.Window).text = s
			},
		})
	})
	if err := w.TestType("name", "héllo"); err != nil {
		t.Fatalf("TestType = %v, want nil", err)
	}
	app := State[counterState](w)
	if app.text != "héllo" {
		t.Fatalf("text = %q, want %q", app.text, "héllo")
	}
}

func TestTestTabTraversesInOrder(t *testing.T) {
	w := newCounterWindow(t)
	// Nothing focused yet, so the first Tab lands on the first
	// candidate in DFS order.
	got, err := w.testTab(tabForward)
	if err != nil {
		t.Fatalf("TestTab = %v, want nil", err)
	}
	if got != "inc" {
		t.Fatalf("first Tab focused %q, want %q", got, "inc")
	}
	if got, _ = w.testTab(tabForward); got != "dec" {
		t.Fatalf("second Tab focused %q, want %q", got, "dec")
	}
	if got, _ = w.testTab(tabForward); got != "name" {
		t.Fatalf("third Tab focused %q, want %q", got, "name")
	}
	// Shift-Tab walks back.
	if got, _ = w.testTab(tabBackward); got != "dec" {
		t.Fatalf("Shift-Tab focused %q, want %q", got, "dec")
	}
}

// A widget with Focusable but no ID never joins tab order. This is the
// silent no-op phase 1 made impossible for the nine input factories;
// containers can still express it, and TestTab must show it.
func TestTestTabSkipsIDlessFocusable(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					Focusable: true, // no ID: inert
					Sizing:    FixedFixed,
					Width:     50,
					Height:    20,
				}),
				Button(ButtonCfg{
					ID:      "real",
					Content: []View{Text(TextCfg{Text: "Real"})},
					OnClick: func(_ EventCtx) {},
				}),
			},
		})
	})
	if got, _ := w.testTab(tabForward); got != "real" {
		t.Fatalf("Tab focused %q, want %q — ID-less focusable joined tab order",
			got, "real")
	}
}

// newScrollWindow builds a short scrollable container holding content
// taller than it, so scrolling has somewhere to go.
func newScrollWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		rows := make([]View, 0, 40)
		for range 40 {
			rows = append(rows, Column(ContainerCfg{
				Sizing: FillFixed,
				Height: 20,
			}))
		}
		return Column(ContainerCfg{
			ID:         "list",
			Scrollable: true,
			Sizing:     FixedFixed,
			Width:      200,
			Height:     100,
			Content:    rows,
		})
	})
	return w
}

func TestTestScrollMovesOffset(t *testing.T) {
	w := newScrollWindow(t)
	x, y, err := w.TestScrollOffset("list")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if x != 0 || y != 0 {
		t.Fatalf("initial offset = (%v, %v), want (0, 0)", x, y)
	}
	// Negative dy scrolls down; the stored offset goes negative.
	if err = w.TestScroll("list", 0, -3); err != nil {
		t.Fatalf("TestScroll = %v, want nil", err)
	}
	_, y, err = w.TestScrollOffset("list")
	if err != nil {
		t.Fatalf("TestScrollOffset after scroll = %v, want nil", err)
	}
	if y >= 0 {
		t.Fatalf("offset y = %v after scrolling down, want < 0", y)
	}
}

// Scrolling past the end must stop at the content bound. Asserted as a
// fixed point rather than against a hand-computed offset: the exact
// bound depends on row spacing and container padding, so hard-coding it
// would test the arithmetic in this test rather than the clamp.
func TestTestScrollClampsAtEnd(t *testing.T) {
	w := newScrollWindow(t)
	for range 200 {
		// Ignore the error: once pinned at the limit the handler stops
		// consuming, which is the state this test is driving toward.
		_ = w.TestScroll("list", 0, -30)
	}
	_, end, err := w.TestScrollOffset("list")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if end >= 0 {
		t.Fatalf("offset y = %v after scrolling to the end, want < 0", end)
	}
	// Already pinned: more scrolling must be a no-op, and the handler
	// must stop claiming the event so it can cascade to an ancestor.
	err = w.TestScroll("list", 0, -30)
	if !errors.Is(err, errTestUnhandled) {
		t.Fatalf("TestScroll past the end = %v, want ErrTestUnhandled", err)
	}
	_, after, err := w.TestScrollOffset("list")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if after != end {
		t.Fatalf("offset y moved from %v to %v past the end, want clamped",
			end, after)
	}
	// Scrolling back up is not clamped away.
	if err = w.TestScroll("list", 0, 30); err != nil {
		t.Fatalf("TestScroll back up = %v, want nil", err)
	}
	_, up, err := w.TestScrollOffset("list")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if up <= end {
		t.Fatalf("offset y = %v after scrolling up from %v, want greater",
			up, end)
	}
}

// A scroll container whose content fits reports that specifically,
// rather than the generic ErrTestUnhandled a clamped-at-the-limit
// container gets. The two mean opposite things to whoever is reading
// the failure: this one says the fixture never had anything to scroll.
func TestTestScrollNoRoom(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:         "roomy",
			Scrollable: true,
			Sizing:     FixedFixed,
			Width:      200,
			Height:     400,
			Content: []View{
				Column(ContainerCfg{Sizing: FillFixed, Height: 20}),
			},
		})
	})
	err := w.TestScroll("roomy", 0, -3)
	if !errors.Is(err, errTestNoScrollRoom) {
		t.Fatalf("TestScroll on a fitting container = %v, want ErrTestNoScrollRoom",
			err)
	}
}

// The gap this closes: wrapped text used to keep its single-line
// estimate headlessly, so a scroll container around a wall of text
// never overflowed and TestScroll failed for a reason that had nothing
// to do with the widget. This is the shape scroll_demo has, which §4.8
// tried and abandoned.
func TestTestScrollOverWrappedText(t *testing.T) {
	long := strings.Repeat("word ", 300)
	w := NewTestWindow(WindowCfg{State: &counterState{}})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			ID:         "panel",
			Scrollable: true,
			Sizing:     FixedFixed,
			Width:      200,
			Height:     100,
			Content: []View{
				Text(TextCfg{Text: long, Mode: TextModeWrap}),
			},
		})
	})
	if err := w.TestScroll("panel", 0, -3); err != nil {
		t.Fatalf("TestScroll over wrapped text = %v, want nil", err)
	}
	_, y, err := w.TestScrollOffset("panel")
	if err != nil {
		t.Fatalf("TestScrollOffset = %v, want nil", err)
	}
	if y >= 0 {
		t.Fatalf("offset y = %v after scrolling down, want < 0", y)
	}
}

func TestTestScrollOffsetNotScrollable(t *testing.T) {
	w := newCounterWindow(t)
	_, _, err := w.TestScrollOffset("inc")
	if !errors.Is(err, errTestNoHandler) {
		t.Fatalf("TestScrollOffset(inc) = %v, want ErrTestNoHandler", err)
	}
}

func TestTestScrollNotScrollable(t *testing.T) {
	w := newCounterWindow(t)
	err := w.TestScroll("inc", 0, -3)
	if !errors.Is(err, errTestNoHandler) {
		t.Fatalf("TestScroll(inc) = %v, want ErrTestNoHandler", err)
	}
}

func TestNewTestWindowRunsOnInit(t *testing.T) {
	var called bool
	w := NewTestWindow(WindowCfg{
		State:  &counterState{},
		OnInit: func(_ *Window) { called = true },
	})
	if !called {
		t.Fatal("OnInit not called by NewTestWindow")
	}
	if w.windowWidth != testWindowSize || w.windowHeight != testWindowSize {
		t.Fatalf("size = %dx%d, want %dx%d",
			w.windowWidth, w.windowHeight, testWindowSize, testWindowSize)
	}
}

func TestTestRenderNilReusesGenerator(t *testing.T) {
	w := newCounterWindow(t)
	if err := w.TestClick("inc"); err != nil {
		t.Fatalf("TestClick = %v, want nil", err)
	}
	root := w.TestRender(nil)
	if root == nil {
		t.Fatal("TestRender(nil) returned nil")
	}
	if _, ok := root.FindByID("inc"); !ok {
		t.Fatal("TestRender(nil) lost the tree; generator was not reused")
	}
}

// --- Hover pressed-state helpers ---
//
// These drive the real pipeline: a mouse event, then a settled frame,
// which is where layoutHover fires (layoutArrange runs it after the
// layout passes). They are what the hover pressed-state widget tests
// are built on — asserting colors after feeding a mouse down/up.

// hoverOver moves the pointer to the clip center of the shape with
// effective ID id and settles a frame, so the hover pass runs at that
// point. Returns the hit point for subsequent press/release events.
func hoverOver(t *testing.T, w *Window, id string) (float32, float32) {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape with effective ID %s", id)
	}
	x, y, err := testHitPoint(ly, id)
	if err != nil {
		t.Fatal(err)
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.settle()
	return x, y
}

// pressAt feeds a button press at the given point and settles a frame;
// releaseAt feeds the release. The settle runs the layout pass, which
// is where hover fires with the window's held-button state.
func pressAt(w *Window, btn MouseButton, x, y float32) {
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: btn, MouseX: x, MouseY: y})
	w.settle()
}

func releaseAt(w *Window, btn MouseButton, x, y float32) {
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: btn, MouseX: x, MouseY: y})
	w.settle()
}

// mustShape resolves id against the current layout tree.
func mustShape(t *testing.T, w *Window, id string) *Shape {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape with effective ID %s", id)
	}
	return ly.Shape
}
