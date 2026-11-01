package gui

import "testing"

// datePickerFocusDisabledView puts an input above a FocusDisabled
// DatePicker, so a focus repair would land on the input.
func datePickerFocusDisabledView(w *Window) View {
	return Column(ContainerCfg{Sizing: FillFill, Content: []View{
		Input(InputCfg{ID: "name"}),
		DatePicker(DatePickerCfg{ID: "dp", FocusDisabled: true}),
	}})
}

// A FocusDisabled picker opts out of focus as a whole: its header and
// day buttons take none, and their handlers move none. Before, a press
// on an arrow focused the picker, a shape that cannot hold focus, and
// the window's repair moved focus to the first tab stop (#1212).
func TestDatePickerFocusDisabledTakesNoFocus(t *testing.T) {
	for _, target := range []string{
		ScopeID("dp", "next"),
		ScopeID("dp", "prev"),
		ScopeIDN("dp", "day", 15),
	} {
		t.Run(target, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(datePickerFocusDisabledView)
			if err := w.TestClick(target); err != nil {
				t.Fatalf("TestClick(%s): %v", target, err)
			}
			if got := w.FocusID(); got != "" {
				t.Errorf("focus = %q, want none", got)
			}
		})
	}
}

// The month label opens the roller and Done closes it, both without
// focus for a FocusDisabled picker.
func TestDatePickerFocusDisabledRollerTakesNoFocus(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(datePickerFocusDisabledView)
	if err := w.TestClick(ScopeID("dp", "month")); err != nil {
		t.Fatalf("TestClick(month): %v", err)
	}
	s, _ := StateMap[string, datePickerState](w, nsDatePicker, capModerate).Get("dp")
	if !s.ShowYearMonthPicker {
		t.Fatal("month press did not open the roller")
	}
	if err := w.TestClick(ScopeID("dp", "done")); err != nil {
		t.Fatalf("TestClick(done): %v", err)
	}
	s, _ = StateMap[string, datePickerState](w, nsDatePicker, capModerate).Get("dp")
	if s.ShowYearMonthPicker {
		t.Error("done press left the roller open")
	}
	if got := w.FocusID(); got != "" {
		t.Errorf("focus = %q, want none", got)
	}
}

// A FocusDisabled picker has no tab stops at all.
func TestDatePickerFocusDisabledNoTabStops(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	ly := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			DatePicker(DatePickerCfg{ID: "dp", FocusDisabled: true}),
		}})
	})
	if got := autoFocusables(ly); len(got) != 0 {
		t.Errorf("FocusDisabled DatePicker has tab stop %q", got[0].idKey())
	}
}
