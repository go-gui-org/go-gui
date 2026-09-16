package gui

import (
	"strings"
	"testing"
)

// Tests for keyboard press state (issue #658). Space on a focused
// ClickOnSpace widget presses it on key down and clicks on key up, the
// way a pointer press clicks on release. The press shows in IsPressed
// while the key is held, and a focus change or a window blur cancels it.

// spaceButton returns a focused ClickOnSpace/ClickOnEnter widget whose
// OnClick counts into *clicks, and a window with focus on it.
func spaceButton(clicks *int) (*Layout, *Window) {
	root := focusedChild("b", &eventHandlers{
		clickOnSpace: true,
		clickOnEnter: true,
		OnClick: func(ctx EventCtx) {
			*clicks++
			ctx.Consume()
		},
	})
	w := newTestWindow()
	w.setFocusLocked("b")
	return root, w
}

func keyDown(root *Layout, w *Window, key KeyCode) *Event {
	e := &Event{Type: EventKeyDown, KeyCode: key}
	w.handleKeyDownEvent(root, e)
	return e
}

func keyUp(root *Layout, w *Window, key KeyCode) *Event {
	e := &Event{Type: EventKeyUp, KeyCode: key}
	w.handleKeyUpEvent(root, e)
	return e
}

func TestSpacePressesOnDownClicksOnUp(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)

	down := keyDown(root, w, KeySpace)
	if !down.IsHandled {
		t.Error("space down on a ClickOnSpace widget must be consumed")
	}
	if clicks != 0 {
		t.Fatalf("clicks after key down = %d, want 0", clicks)
	}
	if !w.IsPressed("b") {
		t.Fatal("IsPressed must be true while space is held")
	}

	up := keyUp(root, w, KeySpace)
	if !up.IsHandled {
		t.Error("space up that clicks must be consumed")
	}
	if clicks != 1 {
		t.Fatalf("clicks after key up = %d, want 1", clicks)
	}
	if w.IsPressed("b") {
		t.Fatal("IsPressed must be false after release")
	}
}

// The char event a backend sends between key down and key up is claimed
// so the space does not type or scroll, but it no longer clicks.
func TestSpaceCharClaimedWithoutClick(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	keyDown(root, w, KeySpace)
	e := &Event{Type: EventChar, CharCode: charSpace}
	charHandler(root, e, w)
	if !e.IsHandled {
		t.Error("space char must stay claimed")
	}
	if clicks != 0 {
		t.Fatalf("space char clicked %d times, want 0", clicks)
	}
}

func TestSpaceKeyRepeatClicksOnce(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	keyDown(root, w, KeySpace)
	for range 3 {
		e := &Event{Type: EventKeyDown, KeyCode: KeySpace, KeyRepeat: true}
		w.handleKeyDownEvent(root, e)
		if !e.IsHandled {
			t.Error("repeat space down must stay consumed")
		}
	}
	keyUp(root, w, KeySpace)
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1", clicks)
	}
}

func TestSpacePressCancelled(t *testing.T) {
	tests := []struct {
		name   string
		cancel func(root *Layout, w *Window)
	}{
		{"focus moves", func(_ *Layout, w *Window) { w.setFocusLocked("other") }},
		{"focus cleared", func(_ *Layout, w *Window) { w.setFocusLocked("") }},
		{"window unfocused", func(_ *Layout, w *Window) { w.handleUnfocusedEvent() }},
		{"escape", func(root *Layout, w *Window) { keyDown(root, w, KeyEscape) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clicks := 0
			root, w := spaceButton(&clicks)
			keyDown(root, w, KeySpace)
			tc.cancel(root, w)
			if w.IsPressed("b") {
				t.Fatal("press must be cancelled")
			}
			// Focus may have come back before the release: still no click.
			w.setFocusLocked("b")
			keyUp(root, w, KeySpace)
			if clicks != 0 {
				t.Fatalf("clicks = %d, want 0", clicks)
			}
		})
	}
}

// Re-asserting focus on the pressed widget is not a focus change.
func TestSpacePressSurvivesFocusReassert(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	keyDown(root, w, KeySpace)
	w.setFocusLocked("b")
	keyUp(root, w, KeySpace)
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1", clicks)
	}
}

// A key up with no matching press, for example a space held down before
// focus arrived, does not click.
func TestSpaceUpWithoutPressIsInert(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	e := keyUp(root, w, KeySpace)
	if clicks != 0 || e.IsHandled {
		t.Fatalf("clicks = %d handled = %v, want 0 false", clicks, e.IsHandled)
	}
}

// A mouse release must not clear a held space, and a released space must
// not clear a held mouse press.
func TestKeyAndMousePressIndependent(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	keyDown(root, w, KeySpace)
	w.handleMouseUpEvent(root, &Event{Type: EventMouseUp, MouseButton: MouseLeft})
	if !w.IsPressed("b") {
		t.Fatal("mouse up cleared the key press")
	}
	w.viewState.pressTargetID = "b"
	keyUp(root, w, KeySpace)
	if !w.IsPressed("b") {
		t.Fatal("space up cleared the mouse press")
	}
}

func TestSpaceWithCommandModifierIgnored(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	e := &Event{Type: EventKeyDown, KeyCode: KeySpace, Modifiers: ModCtrl}
	w.handleKeyDownEvent(root, e)
	if w.IsPressed("b") {
		t.Fatal("ctrl+space must not press")
	}
}

func TestEnterClicksOnDownWithoutPress(t *testing.T) {
	clicks := 0
	root, w := spaceButton(&clicks)
	keyDown(root, w, KeyEnter)
	if clicks != 1 {
		t.Fatalf("clicks after enter down = %d, want 1", clicks)
	}
	if w.IsPressed("b") {
		t.Fatal("enter must not leave a press")
	}
	keyUp(root, w, KeyEnter)
	if clicks != 1 {
		t.Fatalf("clicks after enter up = %d, want 1", clicks)
	}
}

// End to end through EventFn on a real Button: TestKey sends down then up.
func TestButtonSpaceThroughEventFn(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	clicks := 0
	w.TestRender(func(_ *Window) View {
		return Button(ButtonCfg{ID: "ok", OnClick: func(ctx EventCtx) {
			clicks++
			ctx.Consume()
		}})
	})
	if err := w.TestKey("ok", KeySpace, ModNone); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1", clicks)
	}
}

func TestInteractiveArmedByKeyPress(t *testing.T) {
	w := newTestWindow()
	w.setFocusLocked("ok")
	w.viewState.keyPressTargetID = "ok"
	var got InteractionState
	generateViewLayout(interactiveProbe("ok", "ok", &got), w)
	want := InteractionState{Pressed: true, Armed: true, Focused: true, FocusWithin: true}
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
}

func TestInteractiveReportsMissingKeyboardFields(t *testing.T) {
	onClick := func(ctx EventCtx) {}
	full := ContainerCfg{ID: "ok", SizeBorder: NoBorder, Focusable: true,
		ClickOnSpace: true, ClickOnEnter: true}
	tests := []struct {
		name    string
		cfg     func(c *ContainerCfg)
		missing string
	}{
		{"complete", func(*ContainerCfg) {}, ""},
		{"no click handler", func(c *ContainerCfg) {
			c.Focusable, c.ClickOnSpace, c.ClickOnEnter = false, false, false
		}, ""},
		{"no focusable", func(c *ContainerCfg) { c.Focusable = false }, "Focusable"},
		{"no space", func(c *ContainerCfg) { c.ClickOnSpace = false }, "ClickOnSpace"},
		{"no enter", func(c *ContainerCfg) { c.ClickOnEnter = false }, "ClickOnEnter"},
		// A root with its own OnKeyDown, such as a slider, handles the
		// keyboard itself: it is not a button missing Space and Enter.
		{"own key handler", func(c *ContainerCfg) {
			c.ClickOnSpace, c.ClickOnEnter = false, false
			c.A11YRole = AccessRoleSlider
			c.OnKeyDown = func(EventCtx) {}
		}, ""},
		// Its own key handler does not excuse a root that cannot take focus.
		{"own key handler, no focusable", func(c *ContainerCfg) {
			c.Focusable = false
			c.OnKeyDown = func(EventCtx) {}
		}, "Focusable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := full
			if tc.name != "no click handler" {
				cfg.OnClick = onClick
			}
			tc.cfg(&cfg)
			buf := captureDebugMask(t, DebugMissingIDs)
			w := newTestWindow()
			generateViewLayout(Interactive("ok", func(InteractionState) View {
				return Row(cfg)
			}), w)
			out := buf.String()
			if tc.missing == "" {
				if out != "" {
					t.Fatalf("unexpected report: %s", out)
				}
				return
			}
			if !strings.Contains(out, tc.missing) {
				t.Fatalf("report %q does not name %s", out, tc.missing)
			}
		})
	}
}
