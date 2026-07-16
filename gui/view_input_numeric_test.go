package gui

import "testing"

func TestNumericInputIDPassthrough(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni1",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "ni1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestNumericInputDisabledFlag(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:       "ni2",
		Disabled: true,
		StepCfg:  NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestNumericInputStepButtonCount(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni3",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Errorf("children: got %d, want 2", len(layout.Children))
	}
}

func TestNumericInputPlaceholder(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:          "ni4",
		Placeholder: "Enter...",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
}

// TestNumericInputReadOnlyForwardsToInput checks that a read-only
// NumericInput (no steppers) forwards ReadOnly to its inner Input,
// which announces AccessStateReadOnly.
func TestNumericInputReadOnlyForwardsToInput(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:       "ni-ro-plain",
		ReadOnly: true,
		Text:     "5",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateReadOnly {
		t.Errorf("A11YState=%d, want ReadOnly (%d)",
			layout.Shape.A11YState, AccessStateReadOnly)
	}
}

// TestNumericInputReadOnlyStepButtonsGated covers #82: a read-only
// numeric input must not increment via its step buttons. The buttons
// are visually disabled, and numericInputApplyStep (the choke point)
// blocks the mutation even when the handler is invoked directly. The
// editable control below proves the probe observes stepping (remove the
// ReadOnly gate in numericInputApplyStep and the read-only assertion
// fails).
func TestNumericInputReadOnlyStepButtonsGated(t *testing.T) {
	step := func(readOnly bool) bool {
		committed := false
		w := &Window{}
		v := NumericInput(NumericInputCfg{
			ID:       "ni-step",
			ReadOnly: readOnly,
			Text:     "5",
			StepCfg:  NumericStepCfg{ShowButtons: true, Step: 1},
			OnValueCommit: func(_ *Layout, _ Opt[float64], _ string, _ *Window) {
				committed = true
			},
		})
		layout := generateViewLayout(v, w)
		up := findShapeByID(&layout, "ni-step_step_up")
		if up == nil {
			t.Fatal("step-up button not found")
		}
		if readOnly && !up.Shape.Disabled {
			t.Error("read-only step-up button should be Disabled")
		}
		if up.Shape.events == nil || up.Shape.events.OnClick == nil {
			t.Fatal("step-up button missing OnClick")
		}
		// Invoke directly, bypassing the dispatch-level Disabled gate,
		// to exercise the choke point.
		up.Shape.events.OnClick(up, &Event{}, w)
		return committed
	}

	if !step(false) {
		t.Error("editable numeric input did not step the value")
	}
	if step(true) {
		t.Error("read-only numeric input stepped the value")
	}
}
