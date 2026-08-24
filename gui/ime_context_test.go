package gui

import "testing"

// imeEditTargetLayout builds the shape tree an Input produces: a
// focusable container claiming the ID, wrapping the text shape that
// only references it through focusOwner.
func imeEditTargetLayout(id string) Layout {
	return Layout{
		Shape: &Shape{},
		Children: []Layout{{
			Shape: &Shape{ID: id, Focusable: true},
			Children: []Layout{{
				Shape: &Shape{
					shapeType:  shapeText,
					focusOwner: id,
					TC:         &shapeTextConfig{Text: "hi"},
				},
			}},
		}},
	}
}

func TestShapeIsIMEEditTarget(t *testing.T) {
	cases := []struct {
		name  string
		shape *Shape
		want  bool
	}{
		{"nil", nil, false},
		{
			"input text shape",
			&Shape{
				shapeType:  shapeText,
				focusOwner: "in",
				TC:         &shapeTextConfig{},
			},
			true,
		},
		{
			"read-only input never commits a composition",
			&Shape{
				shapeType:  shapeText,
				focusOwner: "in",
				TC:         &shapeTextConfig{textReadOnly: true},
			},
			false,
		},
		{
			"selection-only text is focusable but not editable",
			&Shape{
				shapeType: shapeText,
				Focusable: true,
				ID:        "label",
				TC:        &shapeTextConfig{},
			},
			false,
		},
		{
			"focusable term grid consumes typed text",
			&Shape{shapeType: shapeTermGrid, Focusable: true},
			true,
		},
		{
			"read-only term grid",
			&Shape{shapeType: shapeTermGrid},
			false,
		},
		{"button", &Shape{Focusable: true, ID: "b"}, false},
	}
	for _, c := range cases {
		if got := shapeIsIMEEditTarget(c.shape); got != c.want {
			t.Errorf("%s: shapeIsIMEEditTarget = %v, want %v",
				c.name, got, c.want)
		}
	}
}

// Focusing a non-text widget must leave the platform input method
// alone. On macOS an active input context routes every key through
// interpretKeyEvents:, which swallows Option+printable shortcuts as
// dead keys (issue #393).
func TestIMENotStartedForNonTextWidget(t *testing.T) {
	w := newTestWindow()
	spy := &imeSpyPlatform{}
	w.SetNativePlatform(spy)
	w.layout = Layout{
		Shape:    &Shape{},
		Children: []Layout{{Shape: &Shape{ID: "btn", Focusable: true}}},
	}

	w.SetFocus("btn")
	w.syncIMEEditContext()
	if spy.starts != 0 {
		t.Fatalf("IMEStart calls for a focused button = %d, want 0",
			spy.starts)
	}
	if spy.stops != 0 {
		t.Fatalf("IMEStop calls with nothing to stop = %d, want 0",
			spy.stops)
	}
}

// Losing focus entirely stops the input method.
func TestIMEStoppedOnBlur(t *testing.T) {
	w := newTestWindow()
	spy := &imeSpyPlatform{}
	w.SetNativePlatform(spy)
	w.layout = imeEditTargetLayout("in")

	w.SetFocus("in")
	w.syncIMEEditContext()
	if spy.starts != 1 {
		t.Fatalf("IMEStart calls = %d, want 1", spy.starts)
	}
	w.ClearFocus()
	w.syncIMEEditContext()
	if spy.stops != 1 {
		t.Fatalf("IMEStop calls after blur = %d, want 1", spy.stops)
	}
}

// End-to-end through the real widgets and a real frame: the predicate
// must match the shape tree Input actually builds, not just a
// hand-assembled one.
func TestIMEEditContextThroughRealFrame(t *testing.T) {
	cases := []struct {
		name    string
		build   func() View
		focusID string
		want    bool
	}{
		{
			"input",
			func() View { return Input(InputCfg{ID: "field"}) },
			"field",
			true,
		},
		{
			"read-only input",
			func() View {
				return Input(InputCfg{ID: "field", ReadOnly: true})
			},
			"field",
			false,
		},
		{
			"button",
			func() View { return Button(ButtonCfg{ID: "btn", Label: "go"}) },
			"btn",
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{
				State: new(int), Width: 200, Height: 100,
			})
			spy := &imeSpyPlatform{}
			w.SetNativePlatform(spy)
			w.viewGenerator = func(_ *Window) View {
				return Column(ContainerCfg{
					Sizing:  FillFill,
					Content: []View{c.build()},
				})
			}
			w.SetFocus(c.focusID)
			w.refreshLayout = true
			w.FrameFn()

			if got := spy.starts > 0; got != c.want {
				t.Fatalf("IMEStart fired = %v (starts=%d), want %v",
					got, spy.starts, c.want)
			}
		})
	}
}

// The focus ID is the effective one, and Input's inner text shape
// carries only the leaf in focusOwner. resolveShapeIDs reconciles the
// two; assert the edit context still resolves under a scope.
func TestIMEEditContextUnderIDScope(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 100})
	spy := &imeSpyPlatform{}
	w.SetNativePlatform(spy)
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			ID:      "panel",
			Sizing:  FillFill,
			Content: []View{Input(InputCfg{ID: "field"})},
		})
	}
	w.SetFocus("panel:field")
	w.refreshLayout = true
	w.FrameFn()

	if spy.starts != 1 {
		t.Fatalf("IMEStart calls for a scoped input = %d, want 1",
			spy.starts)
	}
}

// Moving from one text field to another must cycle the platform input
// method. IMEStop is what cancels a composition still live inside the
// engine (ibus FocusOut, the IMM context detach, the web hidden input
// being removed); without it a half-typed word commits into the field
// that just took focus.
func TestIMECycledBetweenTextFields(t *testing.T) {
	w := newTestWindow()
	spy := &imeSpyPlatform{}
	w.SetNativePlatform(spy)

	w.layout = imeEditTargetLayout("a")
	w.SetFocus("a")
	w.syncIMEEditContext()
	if spy.starts != 1 || spy.stops != 0 {
		t.Fatalf("first field: starts=%d stops=%d, want 1 and 0",
			spy.starts, spy.stops)
	}

	w.layout = imeEditTargetLayout("b")
	w.SetFocus("b")
	w.syncIMEEditContext()
	if spy.stops != 1 || spy.starts != 2 {
		t.Fatalf("second field: starts=%d stops=%d, want 2 and 1",
			spy.starts, spy.stops)
	}
}

// shapeDrawsCaret covers exactly the shapes renderInputCursor will
// draw for. It is wider than shapeIsIMEEditTarget: a read-only input
// keeps its caret (only the IME gate excludes it), and a
// selection-only focusable Text draws one too. A terminal grid is not
// a caret target — its cursor is the consumer's own TermCursor.Visible
// flag, which the blink animation does not drive (issue #403).
func TestShapeDrawsCaret(t *testing.T) {
	cases := []struct {
		name  string
		shape *Shape
		want  bool
	}{
		{"nil", nil, false},
		{
			"input text shape",
			&Shape{
				shapeType:  shapeText,
				focusOwner: "in",
				TC:         &shapeTextConfig{},
			},
			true,
		},
		{
			"read-only input keeps a caret",
			&Shape{
				shapeType:  shapeText,
				focusOwner: "in",
				TC:         &shapeTextConfig{textReadOnly: true},
			},
			true,
		},
		{
			"selection-only text draws a caret",
			&Shape{
				shapeType: shapeText,
				Focusable: true,
				ID:        "label",
				TC:        &shapeTextConfig{},
			},
			true,
		},
		{
			"term grid owns its own cursor",
			&Shape{shapeType: shapeTermGrid, Focusable: true},
			false,
		},
		{"button", &Shape{Focusable: true, ID: "b"}, false},
		{
			"plain text renders no caret",
			&Shape{shapeType: shapeText, ID: "t", TC: &shapeTextConfig{}},
			false,
		},
	}
	for _, c := range cases {
		if got := shapeDrawsCaret(c.shape); got != c.want {
			t.Errorf("%s: shapeDrawsCaret = %v, want %v",
				c.name, got, c.want)
		}
	}
}

// Focusing a non-text widget must not start the caret-blink
// animation: every 600 ms tick queues a window-wide render-only
// refresh for a caret no widget draws (issue #403).
func TestBlinkCursorNotAddedForNonTextWidget(t *testing.T) {
	w := newTestWindow()
	w.layout = Layout{
		Shape:    &Shape{},
		Children: []Layout{{Shape: &Shape{ID: "btn", Focusable: true}}},
	}

	w.SetFocus("btn")
	w.syncBlinkCursor()
	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation active for a focused button")
	}
}

// The blink animation follows the caret target: registered while an
// input holds focus, removed when focus leaves it.
func TestBlinkCursorAddedAndRemovedWithCaretTarget(t *testing.T) {
	w := newTestWindow()
	w.layout = imeEditTargetLayout("in")

	w.SetFocus("in")
	w.syncBlinkCursor()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation missing for a focused input")
	}

	w.ClearFocus()
	w.syncBlinkCursor()
	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation still active after blur")
	}
}

// A Pulsar blinks on the same inputCursorOn state with no focused
// input at all; its blink registration must survive syncBlinkCursor.
func TestBlinkCursorKeptWhilePulsarActive(t *testing.T) {
	w := newTestWindow()
	w.layout = Layout{
		Shape:    &Shape{},
		Children: []Layout{{Shape: &Shape{ID: "btn", Focusable: true}}},
	}
	w.AnimationAdd(newBlinkCursorAnimation())
	w.animationAddViewBound(&Animate{
		AnimID: pulsarAnimationID,
		Delay:  blinkCursorAnimationDelay,
		Repeat: true,
	})

	w.SetFocus("btn")
	w.syncBlinkCursor()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation removed while a Pulsar is active")
	}
}

// End-to-end through the real widgets and a real frame: the blink
// gate must match the caret each widget actually draws.
func TestBlinkCursorThroughRealFrame(t *testing.T) {
	cases := []struct {
		name    string
		build   func() View
		focusID string
		want    bool
	}{
		{
			"input",
			func() View { return Input(InputCfg{ID: "field"}) },
			"field",
			true,
		},
		{
			"read-only input keeps its caret",
			func() View {
				return Input(InputCfg{ID: "field", ReadOnly: true})
			},
			"field",
			true,
		},
		{
			"button",
			func() View { return Button(ButtonCfg{ID: "btn", Label: "go"}) },
			"btn",
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{
				State: new(int), Width: 200, Height: 100,
			})
			w.viewGenerator = func(_ *Window) View {
				return Column(ContainerCfg{
					Sizing:  FillFill,
					Content: []View{c.build()},
				})
			}
			w.SetFocus(c.focusID)
			w.refreshLayout = true
			w.FrameFn()

			if got := w.HasAnimation(blinkCursorAnimationID); got != c.want {
				t.Fatalf("blink animation present = %v, want %v", got, c.want)
			}
		})
	}
}

// A background window blinks nothing: the animation is retired while
// the window is unfocused so the animation ticker can park, and comes
// back when the OS returns focus.
func TestBlinkCursorSuspendedWhileWindowUnfocused(t *testing.T) {
	w := newTestWindow()
	w.layout = imeEditTargetLayout("in")

	w.SetFocus("in")
	w.syncBlinkCursor()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation missing for a focused input")
	}

	w.handleUnfocusedEvent()
	w.syncBlinkCursor()
	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation still active while window unfocused")
	}
	// A blink tick may have left the caret off at the moment focus was
	// lost; the animation is gone, so nothing will turn it back on.
	// Refocus must restore it solid regardless of the phase.
	w.viewState.inputCursorOn.Store(false)

	w.handleFocusedEvent()
	w.syncBlinkCursor()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation not restored when window refocused")
	}
	if !w.inputCursorOn() {
		t.Fatal("caret not made visible again on window refocus")
	}
}

// The Pulsar exemption still holds while the window is unfocused: the
// blink registration is the Pulsar's, not the caret's.
func TestBlinkCursorKeptWhilePulsarActiveUnfocused(t *testing.T) {
	w := newTestWindow()
	w.layout = imeEditTargetLayout("in")
	w.AnimationAdd(newBlinkCursorAnimation())
	w.animationAddViewBound(&Animate{
		AnimID: pulsarAnimationID,
		Delay:  blinkCursorAnimationDelay,
		Repeat: true,
	})

	w.SetFocus("in")
	w.handleUnfocusedEvent()
	w.syncBlinkCursor()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation removed while a Pulsar is active")
	}
}

// End-to-end through a real frame: losing OS focus retires the blink
// animation, which is what lets animationLoop park its ticker.
func TestBlinkCursorWindowFocusThroughRealFrame(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 100})
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{Input(InputCfg{ID: "field"})},
		})
	}
	w.SetFocus("field")
	w.refreshLayout = true
	w.FrameFn()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation missing for a focused input")
	}

	w.EventFn(&Event{Type: EventUnfocused})
	w.FrameFn()
	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation survived the window losing focus")
	}

	w.EventFn(&Event{Type: EventFocused})
	w.FrameFn()
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("blink animation not restored when the window refocused")
	}
}
