package gui

import (
	"math"
	"testing"
)

// softKeyboardCall records one ShowSoftKeyboard request.
type softKeyboardCall struct {
	kind   KeyboardKind
	secure bool
}

// softKeyboardSpy records soft-keyboard requests so a test can assert
// what the platform was asked to show, and when (issue #770).
type softKeyboardSpy struct {
	noopNativePlatform
	shows []softKeyboardCall
	hides int
	// imeStarts and imeStops count input-method cycles, so a test
	// can check that a keyboard change leaves the IME alone (#156).
	imeStarts, imeStops int
}

func (p *softKeyboardSpy) ShowSoftKeyboard(kind KeyboardKind, secure bool) {
	p.shows = append(p.shows, softKeyboardCall{kind, secure})
}

func (p *softKeyboardSpy) HideSoftKeyboard() { p.hides++ }

func (p *softKeyboardSpy) IMEStart() { p.imeStarts++ }
func (p *softKeyboardSpy) IMEStop()  { p.imeStops++ }

// newSoftKeyboardWindow builds a real window whose view is content in
// a FillFill column, with a spy platform attached. Tests drive it
// through FrameFn so the edit-context gate runs where it runs in an
// app: in the render pass.
func newSoftKeyboardWindow(content ...View) (*Window, *softKeyboardSpy) {
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 300})
	spy := &softKeyboardSpy{}
	w.SetNativePlatform(spy)
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: content})
	}
	return w, spy
}

// frame runs one full frame after marking the layout stale.
func frame(w *Window) {
	w.refreshLayout.Store(true)
	w.FrameFn()
}

// pressCenter sends a left press at the center of the shape whose
// effective identity is effectiveID.
func pressCenter(t *testing.T, w *Window, effectiveID string) {
	t.Helper()
	ly := findLayoutByID(&w.layout, effectiveID)
	if ly == nil {
		t.Fatalf("no shape for %q", effectiveID)
	}
	c := ly.Shape.shapeClip
	w.EventFn(&Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: c.X + c.Width/2, MouseY: c.Y + c.Height/2,
	})
}

func TestSoftKeyboardShownWithFieldKind(t *testing.T) {
	cases := []struct {
		name string
		view View
		id   string
		want softKeyboardCall
	}{
		{"default text", Input(InputCfg{ID: "f"}), "f",
			softKeyboardCall{KeyboardText, false}},
		{"email", Input(InputCfg{ID: "f", Keyboard: KeyboardEmail}), "f",
			softKeyboardCall{KeyboardEmail, false}},
		{"password is secure", Input(InputCfg{ID: "f", IsPassword: true}), "f",
			softKeyboardCall{KeyboardText, true}},
		{"none reaches the platform",
			Input(InputCfg{ID: "f", Keyboard: KeyboardNone}), "f",
			softKeyboardCall{KeyboardNone, false}},
		{"numeric integer", NumericInput(NumericInputCfg{ID: "n"}), "n",
			softKeyboardCall{KeyboardNumber, false}},
		{"numeric decimals",
			NumericInput(NumericInputCfg{ID: "n", Decimals: 2}), "n",
			softKeyboardCall{KeyboardDecimal, false}},
		{"date", InputDate(InputDateCfg{ID: "d"}), ScopeID("d", "input"),
			softKeyboardCall{KeyboardNumber, false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, spy := newSoftKeyboardWindow(c.view)
			w.SetFocus(c.id)
			frame(w)
			if len(spy.shows) != 1 {
				t.Fatalf("shows = %d, want 1", len(spy.shows))
			}
			if spy.shows[0] != c.want {
				t.Fatalf("show = %+v, want %+v", spy.shows[0], c.want)
			}
		})
	}
}

// The request rides the edit-context transition: re-asserting focus on
// the field that holds it must not re-show (the #156 invariant), and
// leaving the field hides the keyboard.
func TestSoftKeyboardFollowsEditContext(t *testing.T) {
	w, spy := newSoftKeyboardWindow(Input(InputCfg{ID: "f"}))
	w.SetFocus("f")
	frame(w)
	for range 3 {
		w.SetFocus("f")
		frame(w)
	}
	if len(spy.shows) != 1 {
		t.Fatalf("shows after refocus = %d, want 1", len(spy.shows))
	}
	w.ClearFocus()
	frame(w)
	if spy.hides != 1 {
		t.Fatalf("hides after blur = %d, want 1", spy.hides)
	}
	if len(spy.shows) != 1 {
		t.Fatalf("shows after blur = %d, want 1", len(spy.shows))
	}
}

// Moving between two fields re-requests with the new field's kind.
func TestSoftKeyboardMovesBetweenFields(t *testing.T) {
	w, spy := newSoftKeyboardWindow(
		Input(InputCfg{ID: "a"}),
		Input(InputCfg{ID: "b", Keyboard: KeyboardURL}),
	)
	w.SetFocus("a")
	frame(w)
	w.SetFocus("b")
	frame(w)
	if len(spy.shows) != 2 {
		t.Fatalf("shows = %d, want 2", len(spy.shows))
	}
	if got := spy.shows[1].kind; got != KeyboardURL {
		t.Fatalf("second show kind = %v, want KeyboardURL", got)
	}
}

func TestSoftKeyboardNotShownForReadOnlyOrButton(t *testing.T) {
	w, spy := newSoftKeyboardWindow(
		Input(InputCfg{ID: "ro", ReadOnly: true}),
		Button(ButtonCfg{ID: "b", Content: []View{Text(TextCfg{Text: "b"})}}),
	)
	w.SetFocus("ro")
	frame(w)
	w.SetFocus("b")
	frame(w)
	if len(spy.shows) != 0 {
		t.Fatalf("shows = %d, want 0", len(spy.shows))
	}
}

// A tap on the field that already holds focus re-opens a keyboard the
// user dismissed (Android Back) without the field losing focus.
func TestSoftKeyboardReshownOnTapOfFocusedField(t *testing.T) {
	w, spy := newSoftKeyboardWindow(Input(InputCfg{ID: "f"}))
	w.SetFocus("f")
	frame(w)
	pressCenter(t, w, "f")
	if len(spy.shows) != 2 {
		t.Fatalf("shows after tap = %d, want 2", len(spy.shows))
	}
	// A tap that focuses the field for the first time is served by the
	// transition alone, not a second request from the press.
	w.ClearFocus()
	frame(w)
	spy.shows = nil
	pressCenter(t, w, "f")
	frame(w)
	if len(spy.shows) != 1 {
		t.Fatalf("shows after focusing tap = %d, want 1", len(spy.shows))
	}
}

// A custom keypad key is a FocusDisabled button: pressing it must keep
// the field focused and must not pop the OS keyboard back up.
func TestSoftKeyboardKeypadPressKeepsFocus(t *testing.T) {
	typed := 0
	w, spy := newSoftKeyboardWindow(
		Input(InputCfg{ID: "f", Keyboard: KeyboardNone}),
		Button(ButtonCfg{
			ID: "k1", FocusDisabled: true,
			Content: []View{Text(TextCfg{Text: "1"})},
			OnClick: func(ctx EventCtx) {
				typed++
				ctx.Consume()
			},
		}),
	)
	w.SetFocus("f")
	frame(w)
	pressCenter(t, w, "k1")
	frame(w)
	if typed != 1 {
		t.Fatalf("key clicks = %d, want 1", typed)
	}
	if got := w.FocusID(); got != "f" {
		t.Fatalf("focus after key press = %q, want f", got)
	}
	if len(spy.shows) != 1 || spy.hides != 0 {
		t.Fatalf("shows=%d hides=%d, want 1 and 0", len(spy.shows), spy.hides)
	}
}

// sendMouse sends a left-button mouse event at (x, y).
func sendMouse(w *Window, typ EventType, x, y float32) {
	w.EventFn(&Event{Type: typ, MouseButton: MouseLeft, MouseX: x, MouseY: y})
}

// centerOf returns the center of the shape with effective ID id.
func centerOf(t *testing.T, w *Window, id string) (x, y float32) {
	t.Helper()
	ly := findLayoutByID(&w.layout, id)
	if ly == nil {
		t.Fatalf("no shape for %q", id)
	}
	c := ly.Shape.shapeClip
	return c.X + c.Width/2, c.Y + c.Height/2
}

// dragScrollPad wraps content in a DragScroll column, the common
// touch layout: every press inside it is first claimed for a pan.
func dragScrollPad(content ...View) View {
	return Column(ContainerCfg{
		ID: "pad", Scrollable: true, DragScroll: true,
		Sizing: FillFill, Content: content,
	})
}

// A keypad key inside a DragScroll container: the pan claim holds the
// press, and the tap replay must still keep the field focused.
func TestSoftKeyboardKeypadInDragScrollKeepsFocus(t *testing.T) {
	typed := 0
	w, spy := newSoftKeyboardWindow(
		Input(InputCfg{ID: "f", Keyboard: KeyboardNone}),
		dragScrollPad(Button(ButtonCfg{
			ID: "k1", FocusDisabled: true,
			Content: []View{Text(TextCfg{Text: "1"})},
			OnClick: func(ctx EventCtx) { typed++; ctx.Consume() },
		})),
	)
	w.SetFocus("f")
	frame(w)
	x, y := centerOf(t, w, "pad:k1")
	sendMouse(w, EventMouseDown, x, y)
	sendMouse(w, EventMouseUp, x, y)
	frame(w)
	if typed != 1 {
		t.Fatalf("key clicks = %d, want 1", typed)
	}
	if got := w.FocusID(); got != "f" {
		t.Fatalf("focus after key tap = %q, want f", got)
	}
	if len(spy.shows) != 1 || spy.hides != 0 {
		t.Fatalf("shows=%d hides=%d, want 1 and 0", len(spy.shows), spy.hides)
	}
}

// A pan that starts on the focused field scrolls; it must not re-open
// a keyboard the user dismissed. A tap on the same field does.
func TestSoftKeyboardPanOnFocusedFieldNoReshow(t *testing.T) {
	w, spy := newSoftKeyboardWindow(dragScrollPad(Input(InputCfg{ID: "f"})))
	w.SetFocus("pad:f")
	frame(w)
	if len(spy.shows) != 1 {
		t.Fatalf("shows after focus = %d, want 1", len(spy.shows))
	}
	x, y := centerOf(t, w, "pad:f")
	sendMouse(w, EventMouseDown, x, y)
	sendMouse(w, EventMouseMove, x, y+60)
	sendMouse(w, EventMouseUp, x, y+60)
	frame(w)
	if len(spy.shows) != 1 {
		t.Fatalf("shows after pan = %d, want 1", len(spy.shows))
	}
	if got := w.FocusID(); got != "pad:f" {
		t.Fatalf("focus after pan from field = %q, want pad:f", got)
	}
	x, y = centerOf(t, w, "pad:f")
	sendMouse(w, EventMouseDown, x, y)
	sendMouse(w, EventMouseUp, x, y)
	if len(spy.shows) != 2 {
		t.Fatalf("shows after tap = %d, want 2", len(spy.shows))
	}
}

func TestSoftKeyboardShowHideAPI(t *testing.T) {
	w, spy := newSoftKeyboardWindow(Input(InputCfg{ID: "f", Keyboard: KeyboardPhone}))

	// Nothing editable is focused: both calls are inert.
	frame(w)
	w.ShowSoftKeyboard()
	w.HideSoftKeyboard()
	if len(spy.shows) != 0 || spy.hides != 0 {
		t.Fatalf("idle shows=%d hides=%d, want 0 and 0",
			len(spy.shows), spy.hides)
	}

	w.SetFocus("f")
	frame(w)
	w.HideSoftKeyboard()
	if spy.hides != 1 {
		t.Fatalf("hides = %d, want 1", spy.hides)
	}
	if got := w.FocusID(); got != "f" {
		t.Fatalf("focus after hide = %q, want f", got)
	}
	w.ShowSoftKeyboard()
	if len(spy.shows) != 2 || spy.shows[1].kind != KeyboardPhone {
		t.Fatalf("shows = %+v, want second with KeyboardPhone", spy.shows)
	}
}

// Nil platform (tests, headless): every entry point is a no-op.
func TestSoftKeyboardNilPlatform(t *testing.T) {
	w := newTestWindow()
	w.ShowSoftKeyboard()
	w.HideSoftKeyboard()
	if got := w.SoftKeyboardInset(); got != 0 {
		t.Fatalf("inset = %v, want 0", got)
	}
}

func TestSoftKeyboardInsetEvent(t *testing.T) {
	w := newEventTestWindow()
	var seen float32
	w.OnEvent = func(e *Event, _ *Window) {
		if e.Type == EventSoftKeyboard {
			seen = e.SoftKeyboardInset
		}
	}
	w.EventFn(&Event{Type: EventSoftKeyboard, SoftKeyboardInset: 280})
	if got := w.SoftKeyboardInset(); got != 280 {
		t.Fatalf("inset = %v, want 280", got)
	}
	if seen != 280 {
		t.Fatalf("OnEvent saw %v, want 280", seen)
	}
	// A backend reporting garbage must not push layout off-screen.
	for _, bad := range []float32{-5, float32(math.NaN()), float32(math.Inf(1))} {
		w.EventFn(&Event{Type: EventSoftKeyboard, SoftKeyboardInset: bad})
		if got := w.SoftKeyboardInset(); got != 0 {
			t.Fatalf("inset for %v = %v, want 0", bad, got)
		}
	}
	// Larger than the window: clamp to the window height.
	w.EventFn(&Event{Type: EventSoftKeyboard, SoftKeyboardInset: 5000})
	if got := w.SoftKeyboardInset(); got != 600 {
		t.Fatalf("inset = %v, want 600", got)
	}
	// The event arrives while the window is in the background (the
	// keyboard closes as the app loses focus).
	w.focused = false
	w.EventFn(&Event{Type: EventSoftKeyboard, SoftKeyboardInset: 0})
	if got := w.SoftKeyboardInset(); got != 0 {
		t.Fatalf("background inset = %v, want 0", got)
	}
}

// A Keyboard or IsPassword change on the focused field re-requests the
// keyboard with the new kind, and leaves the input method running: a
// stop and start would drop a composition in progress (#156).
func TestSoftKeyboardKindChangeOnSameField(t *testing.T) {
	kind := KeyboardText
	secure := false
	w, spy := newSoftKeyboardWindow()
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{ID: "f", Keyboard: kind, IsPassword: secure}),
		}})
	}
	w.SetFocus("f")
	frame(w)
	kind = KeyboardEmail
	frame(w)
	secure = true
	frame(w)
	frame(w) // no change: no request
	want := []softKeyboardCall{
		{KeyboardText, false}, {KeyboardEmail, false}, {KeyboardEmail, true},
	}
	if len(spy.shows) != len(want) {
		t.Fatalf("shows = %+v, want %+v", spy.shows, want)
	}
	for i := range want {
		if spy.shows[i] != want[i] {
			t.Fatalf("show %d = %+v, want %+v", i, spy.shows[i], want[i])
		}
	}
	if spy.imeStarts != 1 || spy.imeStops != 0 || spy.hides != 0 {
		t.Fatalf("imeStarts=%d imeStops=%d hides=%d, want 1 0 0",
			spy.imeStarts, spy.imeStops, spy.hides)
	}
}

func TestKeyboardKindString(t *testing.T) {
	for k, want := range map[KeyboardKind]string{
		KeyboardText: "Text", KeyboardNumber: "Number",
		KeyboardDecimal: "Decimal", KeyboardPhone: "Phone",
		KeyboardEmail: "Email", KeyboardURL: "URL", KeyboardNone: "None",
		KeyboardKind(200): "Unknown",
	} {
		if got := k.String(); got != want {
			t.Errorf("KeyboardKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
