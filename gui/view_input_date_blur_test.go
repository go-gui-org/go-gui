package gui

import "testing"

// inputDateBlurView is the form from issue #1212: an input above a
// "from" and a "to" date field side by side. The input sits above the
// row so the calendar, which floats below its field, cannot cover the
// point TestClick presses.
func inputDateBlurView(w *Window) View {
	return Column(ContainerCfg{
		Sizing: FillFill,
		Content: []View{
			Input(InputCfg{ID: "name"}),
			Row(ContainerCfg{
				Sizing: FillFit,
				Content: []View{
					InputDate(InputDateCfg{ID: "from"}),
					InputDate(InputDateCfg{ID: "to"}),
				},
			}),
		},
	})
}

// openInputDate opens the calendar of the date field id through its
// calendar button and checks that it stays open once the frame
// settles: the button takes focus, which sits inside the field.
func openInputDate(t *testing.T, w *Window, id string) {
	t.Helper()
	if err := w.TestClick(ScopeID(id, "calendar")); err != nil {
		t.Fatalf("TestClick(%s calendar): %v", id, err)
	}
	if !StateReadOr(w, nsInputDate, id, false) {
		t.Fatalf("calendar of %s closed right after it opened (focus %q)",
			id, w.FocusID())
	}
}

// Opening the second date field's calendar closes the first, rather
// than leaving both open on top of each other (issue #1212).
func TestInputDateClosesWhenOtherFieldOpens(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	openInputDate(t, w, "to")

	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("first calendar still open after the second opened")
	}
	ly := w.TestRender(nil)
	if _, ok := ly.findByID(ScopeID("from", "picker")); ok {
		t.Error("first calendar still in the layout")
	}
	if _, ok := ly.findByID(ScopeID("to", "picker")); !ok {
		t.Error("second calendar not in the layout")
	}
}

// A press on another focusable widget moves focus off the date field;
// the calendar closes with it (issue #1212).
func TestInputDateClosesWhenFocusMoves(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	if err := w.TestClick("name"); err != nil {
		t.Fatalf("TestClick(name): %v", err)
	}
	if !w.IsFocus("name") {
		t.Fatalf("focus = %q, want name", w.FocusID())
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("calendar still open after focus moved to the input")
	}
}

// A press on empty window space clears focus, which closes the
// calendar too: the click-outside backdrop covers only the field
// (issue #1212).
func TestInputDateClosesOnEmptySpacePress(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	down := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: testWindowSize - 2, MouseY: testWindowSize - 2,
	}
	w.EventFn(&down)
	w.settle()
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Errorf("calendar still open after an empty-space press (focus %q)",
			w.FocusID())
	}
}

// Focus that moves inside the field must not count as leaving it: the
// text field, and the picker's own buttons, are scoped by the field's
// ID (issue #1212).
func TestInputDateFocusInsideKeepsOpen(t *testing.T) {
	for _, target := range []string{
		ScopeID("from", "input"),
		ScopeID("from", "picker", "next"),
		ScopeID("from", "picker", "month"),
	} {
		t.Run(target, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(inputDateBlurView)

			openInputDate(t, w, "from")
			if err := w.TestClick(target); err != nil {
				t.Fatalf("TestClick(%s): %v", target, err)
			}
			if !StateReadOr(w, nsInputDate, "from", false) {
				t.Errorf("calendar closed after a press on %s (focus %q)",
					target, w.FocusID())
			}
		})
	}
}

// A FocusDisabled date field never holds focus, so the blur dismiss
// skips it, or its calendar would close the frame after it opened.
func TestInputDateFocusDisabledKeepsOpen(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			InputDate(InputDateCfg{ID: "due", FocusDisabled: true}),
		}})
	})
	openInputDate(t, w, "due")
	w.TestRender(nil)
	if !StateReadOr(w, nsInputDate, "due", false) {
		t.Error("FocusDisabled calendar closed without a press")
	}
}

// An accessibility press (VoiceOver, Narrator, AT-SPI) runs the
// calendar button's OnClick without the mouse press walk, so nothing
// else moves focus into the field. The open must take focus itself, or
// the blur dismiss closes the calendar the next frame (#1212).
func TestInputDateA11yPressOpensWithFocus(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)
	w.SetFocus("name")
	ly := w.TestRender(nil)

	btnID := ScopeID("from", "calendar")
	btn, ok := ly.findByID(btnID)
	if !ok {
		t.Fatal("calendar button not in the layout")
	}
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, "", &live)
	idx := -1
	for i, n := range w.a11y.nodes {
		if n.Role == AccessRoleButton &&
			n.X == btn.Shape.X && n.Y == btn.Shape.Y {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("no button node for the calendar button")
	}
	a11yActionCallback(w, A11yActionPress, idx)
	w.TestRender(nil)
	w.TestRender(nil)

	if !w.IsFocus(btnID) {
		t.Errorf("focus = %q, want %s", w.FocusID(), btnID)
	}
	if !StateReadOr(w, nsInputDate, "from", false) {
		t.Error("calendar closed after an accessibility press opened it")
	}
}

// inputDateDueView adds a FocusDisabled "due" field beside "to", the
// form from the manual test of #1212.
func inputDateDueView(w *Window) View {
	return Column(ContainerCfg{
		Sizing: FillFill,
		Content: []View{
			Input(InputCfg{ID: "name"}),
			Row(ContainerCfg{
				Sizing: FillFit,
				Content: []View{
					InputDate(InputDateCfg{ID: "to"}),
					InputDate(InputDateCfg{ID: "due", FocusDisabled: true}),
				},
			}),
		},
	})
}

// A FocusDisabled field opens without taking focus, so the blur
// dismiss alone cannot close the calendar it opens over. Opening one
// calendar closes every other, either way round (#1212).
func TestInputDateOneCalendarOpen(t *testing.T) {
	for _, tc := range []struct{ first, second string }{
		{"to", "due"},
		{"due", "to"},
	} {
		t.Run(tc.first+" then "+tc.second, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(inputDateDueView)

			openInputDate(t, w, tc.first)
			openInputDate(t, w, tc.second)
			if StateReadOr(w, nsInputDate, tc.first, false) {
				t.Errorf("%s calendar still open after %s opened",
					tc.first, tc.second)
			}
		})
	}
}

// A FocusDisabled field skips the blur dismiss, so an empty-space
// press closes its calendar through the unhandled-press cleanup, as
// for a FocusDisabled Select (#1212).
func TestInputDateFocusDisabledClosesOnEmptySpacePress(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateDueView)

	openInputDate(t, w, "due")
	down := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: testWindowSize - 2, MouseY: testWindowSize - 2,
	}
	w.EventFn(&down)
	w.settle()
	if StateReadOr(w, nsInputDate, "due", false) {
		t.Error("calendar still open after an empty-space press")
	}
}

// Window unfocus closes an open calendar, as it does a Select or
// Combobox list (#1212).
func TestInputDateClosesOnWindowBlur(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	dismissFieldPopups(w)
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("dismissFieldPopups left the calendar open")
	}
}

// A window blur with focus inside the picker closes the calendar and
// puts focus back on its button. Left on the picker, focus would sit
// on a shape that leaves the tree, and the window's focus repair would
// move it to the first tab stop (the input above) (#1212).
func TestInputDateWindowBlurFromPickerFocusesButton(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	if err := w.TestClick(ScopeID("from", "picker", "next")); err != nil {
		t.Fatalf("TestClick(next): %v", err)
	}
	if !targetWithin(w.FocusID(), ScopeID("from", "picker")) {
		t.Fatalf("focus = %q, want inside the picker", w.FocusID())
	}
	blur := Event{Type: EventUnfocused}
	w.EventFn(&blur)
	w.settle()
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("window blur left the calendar open")
	}
	if want := ScopeID("from", "calendar"); !w.IsFocus(want) {
		t.Errorf("focus = %q, want %s", w.FocusID(), want)
	}
}

// Escape closes the calendar wherever focus sits inside the field.
// Only the text field had an Escape handler, and a click on the
// calendar button or in the picker leaves focus elsewhere (#1212).
// Focus goes back to the calendar button, not to a picker part that
// is gone.
func TestInputDateEscapeCloses(t *testing.T) {
	for _, target := range []string{
		ScopeID("from", "calendar"),
		ScopeID("from", "picker"),
		ScopeID("from", "picker", "next"),
	} {
		t.Run(target, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(inputDateBlurView)

			openInputDate(t, w, "from")
			if err := w.TestKey(target, KeyEscape, ModNone); err != nil {
				t.Fatalf("TestKey(%s): %v", target, err)
			}
			if StateReadOr(w, nsInputDate, "from", false) {
				t.Errorf("Escape on %s left the calendar open", target)
			}
			if want := ScopeID("from", "calendar"); !w.IsFocus(want) {
				t.Errorf("focus = %q, want %s", w.FocusID(), want)
			}
		})
	}
}

// Escape from the text field closes the calendar and leaves focus in
// the field, through the field's own handler.
func TestInputDateEscapeFromInput(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	input := ScopeID("from", "input")
	if err := w.TestKey(input, KeyEscape, ModNone); err != nil {
		t.Fatalf("TestKey(input): %v", err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("Escape in the text field left the calendar open")
	}
	if !w.IsFocus(input) {
		t.Errorf("focus = %q, want %s", w.FocusID(), input)
	}
}

// With the month/year roller up, the first Escape closes only the
// roller; the next one closes the calendar.
func TestInputDateEscapeClosesRollerFirst(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	picker := ScopeID("from", "picker")
	if err := w.TestClick(ScopeID(picker, "month")); err != nil {
		t.Fatalf("TestClick(month): %v", err)
	}
	s, _ := StateMap[string, datePickerState](
		w, nsDatePicker, capModerate).Get(picker)
	if !s.ShowYearMonthPicker {
		t.Fatal("month click did not open the roller")
	}
	if err := w.TestKey(picker, KeyEscape, ModNone); err != nil {
		t.Fatalf("TestKey(picker): %v", err)
	}
	s, _ = StateMap[string, datePickerState](
		w, nsDatePicker, capModerate).Get(picker)
	if s.ShowYearMonthPicker {
		t.Error("first Escape left the roller up")
	}
	if !StateReadOr(w, nsInputDate, "from", false) {
		t.Fatal("first Escape closed the calendar, want only the roller")
	}
	if err := w.TestKey(picker, KeyEscape, ModNone); err != nil {
		t.Fatalf("TestKey(picker): %v", err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("second Escape left the calendar open")
	}
}

// A FocusDisabled field opts out of focus as a whole, its calendar
// included. Presses in the calendar, the arrow and a day, leave focus
// with the input that held it: a picker that took focus would hold it
// inside a field that skips the blur dismiss, and the calendar would
// stay open once focus left it (#1212).
func TestInputDateFocusDisabledCalendarKeepsFocus(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateDueView)
	w.SetFocus("name")
	w.TestRender(nil)

	openInputDate(t, w, "due")
	next := ScopeID("due", "picker", "next")
	if err := w.TestClick(next); err != nil {
		t.Fatalf("TestClick(%s): %v", next, err)
	}
	if !w.IsFocus("name") {
		t.Errorf("after the arrow: focus = %q, want name", w.FocusID())
	}
	if !StateReadOr(w, nsInputDate, "due", false) {
		t.Fatal("arrow press closed the calendar")
	}
	day := ScopeIDN(ScopeID("due", "picker"), "day", 15)
	if err := w.TestClick(day); err != nil {
		t.Fatalf("TestClick(%s): %v", day, err)
	}
	if StateReadOr(w, nsInputDate, "due", false) {
		t.Error("pick left the calendar open")
	}
	if !w.IsFocus("name") {
		t.Errorf("after the pick: focus = %q, want name", w.FocusID())
	}
	if got := w.TestFindings(DebugUnknownFocus); len(got) != 0 {
		t.Errorf("findings: %v", got)
	}
}

// A pointer pick closes the calendar and the picked day with it. Focus
// must not fall to the window's first tab stop (the input above), as
// the focus repair for a vanished shape does; it is cleared (#1212).
func TestInputDatePointerPickClearsFocus(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	day := ScopeIDN(ScopeID("from", "picker"), "day", 15)
	if err := w.TestClick(day); err != nil {
		t.Fatalf("TestClick(%s): %v", day, err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Fatal("pick left the calendar open")
	}
	if got := w.FocusID(); got != "" {
		t.Errorf("focus = %q, want none", got)
	}
}

// A keyboard pick (Enter on the picker grid) returns focus to the
// calendar button, so a keyboard user keeps their place.
func TestInputDateKeyboardPickFocusesButton(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	picker := ScopeID("from", "picker")
	if err := w.TestKey(picker, KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey(%s): %v", picker, err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Fatal("Enter left the calendar open")
	}
	if want := ScopeID("from", "calendar"); !w.IsFocus(want) {
		t.Errorf("focus = %q, want %s", w.FocusID(), want)
	}
}

// Space on a day button picks through the button's click, which fires
// on key up. That is a keyboard pick too: focus returns to the
// calendar button.
func TestInputDateSpacePickFocusesButton(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	day := ScopeIDN(ScopeID("from", "picker"), "day", 15)
	if err := w.TestKey(day, KeySpace, ModNone); err != nil {
		t.Fatalf("TestKey(%s): %v", day, err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Fatal("Space left the calendar open")
	}
	if want := ScopeID("from", "calendar"); !w.IsFocus(want) {
		t.Errorf("focus = %q, want %s", w.FocusID(), want)
	}
}

// A closed calendar leaves no entry behind, so fixupPopupMaps, which
// runs every layout frame, skips its key walk once nothing is open.
func TestInputDateCloseLeavesNoEntry(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	inputDateClose("from", w)
	if n := StateMap[string, bool](w, nsInputDate, capModerate).Len(); n != 0 {
		t.Errorf("open-state entries after close = %d, want 0", n)
	}
}

// A calendar closed from outside while its month and year roller is up
// reopens on the day grid: closing the calendar closes the roller with
// it (#1212).
func TestInputDateReopensWithoutRoller(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	picker := ScopeID("from", "picker")
	if err := w.TestClick(ScopeID(picker, "month")); err != nil {
		t.Fatalf("TestClick(month): %v", err)
	}
	if err := w.TestClick("name"); err != nil {
		t.Fatalf("TestClick(name): %v", err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Fatal("calendar still open after focus moved")
	}

	openInputDate(t, w, "from")
	s, _ := StateMap[string, datePickerState](
		w, nsDatePicker, capModerate).Get(picker)
	if s.ShowYearMonthPicker {
		t.Error("calendar reopened with the roller up")
	}
	if _, ok := w.TestRender(nil).findByID(ScopeIDN(picker, "day", 15)); !ok {
		t.Error("reopened calendar shows no day grid")
	}
}

// Opening a FocusDisabled field's calendar closes the other one, but
// its button takes no focus. Focus inside the closing calendar's
// picker goes back to that field's button rather than to the window's
// first tab stop (#1212).
func TestInputDateFocusDisabledOpenSettlesFocus(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateDueView)

	openInputDate(t, w, "to")
	if err := w.TestClick(ScopeID("to", "picker", "next")); err != nil {
		t.Fatalf("TestClick(next): %v", err)
	}
	if !targetWithin(w.FocusID(), ScopeID("to", "picker")) {
		t.Fatalf("focus = %q, want inside the picker", w.FocusID())
	}
	openInputDate(t, w, "due")
	if StateReadOr(w, nsInputDate, "to", false) {
		t.Error("to calendar still open after due opened")
	}
	if want := ScopeID("to", "calendar"); !w.IsFocus(want) {
		t.Errorf("focus = %q, want %s", w.FocusID(), want)
	}
}

// A press on an Input beside a FocusDisabled field closes its calendar.
// Focus never sat in the field, so the blur dismiss skips it, and the
// Input handles the press, so the unhandled-press cleanup does not run
// either (#1212).
func TestInputDateFocusDisabledClosesOnInputPress(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateDueView)

	openInputDate(t, w, "due")
	if err := w.TestClick("name"); err != nil {
		t.Fatalf("TestClick(name): %v", err)
	}
	if StateReadOr(w, nsInputDate, "due", false) {
		t.Error("calendar still open after a press on the input")
	}
	if !w.IsFocus("name") {
		t.Errorf("focus = %q, want name", w.FocusID())
	}
}

// inputDateToolView adds a FocusDisabled button, which handles a press
// without taking focus, beside a focusable date field.
func inputDateToolView(w *Window) View {
	return Column(ContainerCfg{
		Sizing: FillFill,
		Content: []View{
			Button(ButtonCfg{
				ID:            "tool",
				Label:         "Tool",
				FocusDisabled: true,
				OnClick:       func(ctx EventCtx) { ctx.Consume() },
			}),
			InputDate(InputDateCfg{ID: "from"}),
		},
	})
}

// A press on a FocusDisabled button leaves focus inside the date
// field, yet lands outside it: the calendar closes. Focus on the
// closing picker goes back to the calendar button, not to the
// window's first tab stop (#1212).
func TestInputDateClosesOnFocusDisabledButtonPress(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateToolView)

	openInputDate(t, w, "from")
	if err := w.TestClick(ScopeID("from", "picker", "next")); err != nil {
		t.Fatalf("TestClick(next): %v", err)
	}
	if err := w.TestClick("tool"); err != nil {
		t.Fatalf("TestClick(tool): %v", err)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("calendar still open after a press outside the field")
	}
	if want := ScopeID("from", "calendar"); !w.IsFocus(want) {
		t.Errorf("focus = %q, want %s", w.FocusID(), want)
	}
}

// With the roller up, focus can sit on its done button, which has no
// Escape of its own. The first Escape closes only the roller, and
// focus goes to the picker rather than stay on the done button, which
// leaves with the roller (#1212).
func TestInputDateEscapeOnDoneClosesRollerFirst(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(inputDateBlurView)

	openInputDate(t, w, "from")
	picker := ScopeID("from", "picker")
	if err := w.TestClick(ScopeID(picker, "month")); err != nil {
		t.Fatalf("TestClick(month): %v", err)
	}
	done := ScopeID(picker, "done")
	if err := w.TestKey(done, KeyEscape, ModNone); err != nil {
		t.Fatalf("TestKey(%s): %v", done, err)
	}
	s, _ := StateMap[string, datePickerState](
		w, nsDatePicker, capModerate).Get(picker)
	if s.ShowYearMonthPicker {
		t.Error("Escape on done left the roller up")
	}
	if !StateReadOr(w, nsInputDate, "from", false) {
		t.Fatal("first Escape closed the calendar, want only the roller")
	}
	if !w.IsFocus(picker) {
		t.Errorf("focus = %q, want %s", w.FocusID(), picker)
	}
}

// A touch that starts outside the field on a scrollable and turns into
// a scroll never sends a press: the scrollable holds it, then drops it.
// The calendar closes at touch-down all the same (#1212).
func TestInputDateClosesOnTouchPanOutside(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 400})
	rows := make([]View, 40)
	for i := range rows {
		rows[i] = Button(ButtonCfg{
			ID: ScopeIDN("row", "", i), Label: "row", Sizing: FillFit,
		})
	}
	w.TestRender(func(_ *Window) View {
		return Row(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID: "list", Scrollable: true,
					Sizing: FixedFixed, Width: 200, Height: 300,
					Content: rows,
				}),
				Rectangle(RectangleCfg{Sizing: FixedFit, Width: 300}),
				InputDate(InputDateCfg{ID: "from", Width: 200}),
			},
		})
	})
	openInputDate(t, w, "from")

	ly, ok := w.layout.FindByID("list")
	if !ok {
		t.Fatal("no list in the layout")
	}
	x := ly.Shape.X + ly.Shape.Width/2
	y := ly.Shape.Y + ly.Shape.Height/2
	touchAt(w, EventTouchesBegan, 0, x, y)
	for i, dy := range []float32{-12, -30, -60} {
		touchAt(w, EventTouchesMoved, int64(i+1)*16_000_000, x, y+dy)
	}
	touchAt(w, EventTouchesEnded, 300_000_000, x, y-60)
	if v, _ := w.scrollY().Get("list"); v >= 0 {
		t.Fatalf("list offset = %v, want it scrolled", v)
	}
	if StateReadOr(w, nsInputDate, "from", false) {
		t.Error("calendar still open after a touch scroll outside the field")
	}
}
