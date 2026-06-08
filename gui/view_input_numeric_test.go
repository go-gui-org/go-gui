package gui

import "testing"

func TestNumericInputIDPassthrough(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni1",
		IDFocus: 100,
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
		IDFocus:  101,
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
		IDFocus: 102,
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
		IDFocus:     103,
		Placeholder: "Enter...",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
}
