package gui

import "testing"

// TestNumericInputStepTriangleBounds locks the named bounds of the step
// triangle (issue #335 §2): it sits 4pt below the field text, never
// below the ladder's bottom rung (N6, tiny) and never above the text it
// decorates. The old widget-local floor was 8 — below every named rung.
func TestNumericInputStepTriangleBounds(t *testing.T) {
	// The text sizes rendered in the generated layout, triangle included.
	textSizes := func(cfg NumericInputCfg) map[float32]bool {
		w := &Window{}
		v := NumericInput(cfg)
		layout := generateViewLayout(v, w)
		sizes := map[float32]bool{}
		var walk func(l Layout)
		walk = func(l Layout) {
			if l.Shape != nil && l.Shape.TC != nil {
				sizes[l.Shape.TC.TextStyle.Size] = true
			}
			for _, c := range l.Children {
				walk(c)
			}
		}
		walk(layout)
		return sizes
	}

	// The lowest size anywhere in the layout, which is the triangle
	// whenever it is stepped below the field text.
	minSize := func(sizes map[float32]bool) float32 {
		var lo float32
		set := false
		for s := range sizes {
			if !set || s < lo {
				lo, set = s, true
			}
		}
		return lo
	}

	// A small field text (12) steps to 8 — the old widget-local floor,
	// and below every named rung. The named floor lifts it to N6 (10),
	// a value nothing else in this layout renders, so the assertion is
	// not satisfied by the field text itself.
	small := DefaultTextStyle
	small.Size = 12
	smallSizes := textSizes(NumericInputCfg{
		ID:        "ni-small",
		TextStyle: small,
		StepCfg:   NumericStepCfg{ShowButtons: true, Step: 1},
	})
	if got := minSize(smallSizes); got != guiTheme.N6.Size {
		t.Errorf("small layout floors at %v, want N6 (%v); sizes = %v",
			got, guiTheme.N6.Size, smallSizes)
	}

	// A default field steps clear of both bounds.
	defSizes := textSizes(NumericInputCfg{
		ID:      "ni-def",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	if got := minSize(defSizes); got != DefaultTextStyle.Size-4 {
		t.Errorf("default layout triangle is %v, want %v; sizes = %v",
			got, DefaultTextStyle.Size-4, defSizes)
	}

	// The ceiling: a field text below the bottom rung (8 against N6's
	// 10) would have the floor lift the triangle *above* the text it
	// decorates. The clamp keeps it at the text size. This is the bound
	// a theme with a large SizeTextTiny crosses at ordinary text sizes.
	belowFloor := DefaultTextStyle
	belowFloor.Size = guiTheme.N6.Size - 2
	belowSizes := textSizes(NumericInputCfg{
		ID:        "ni-below",
		TextStyle: belowFloor,
		StepCfg:   NumericStepCfg{ShowButtons: true, Step: 1},
	})
	if got := minSize(belowSizes); got != belowFloor.Size {
		t.Errorf("triangle is %v, want the field text %v; sizes = %v",
			got, belowFloor.Size, belowSizes)
	}
	if belowSizes[guiTheme.N6.Size] {
		t.Errorf("floor lifted the triangle to N6 (%v) above the field text %v; sizes = %v",
			guiTheme.N6.Size, belowFloor.Size, belowSizes)
	}
}

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

// TestNumericInputBorderFollowsTheme covers issue #300: the numeric
// input used to resolve its fallback border/radius against a private
// literal that no theme could reach. It must take them from the
// theme's input style, and an explicit cfg value must still win.
func TestNumericInputBorderFollowsTheme(t *testing.T) {
	restoreTheme(t)
	th := ThemeDark.withInputStyle(InputStyle{SizeBorder: 7, Radius: 9})
	SetTheme(th)

	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni-theme-border",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 7 {
		t.Errorf("size_border = %v, want 7 (input style)", layout.Shape.SizeBorder)
	}
	if layout.Shape.Radius != 9 {
		t.Errorf("radius = %v, want 9 (input style)", layout.Shape.Radius)
	}

	v = NumericInput(NumericInputCfg{
		ID:         "ni-theme-border-override",
		StepCfg:    NumericStepCfg{ShowButtons: true, Step: 1},
		SizeBorder: SomeF(3),
	})
	layout = generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 3 {
		t.Errorf("size_border = %v, want 3 (cfg wins)", layout.Shape.SizeBorder)
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
			OnValueCommit: func(_ Opt[float64], _ string, ctx EventCtx) {
				committed = true
			},
		})
		layout := generateViewLayout(v, w)
		up := findShapeByID(&layout, ScopeID("ni-step", "step_up"))
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
		up.Shape.events.OnClick(EventCtx{up, &Event{}, w})
		return committed
	}

	if !step(false) {
		t.Error("editable numeric input did not step the value")
	}
	if step(true) {
		t.Error("read-only numeric input stepped the value")
	}
}
