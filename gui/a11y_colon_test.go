package gui

import "testing"

// a11yLabelFor generates v and returns the accessible name of the
// shape with the given ID. It fails the test when the shape or its
// a11y node is missing, so a regression reads as a name mismatch
// rather than a nil panic.
func a11yLabelFor(t *testing.T, v View, id string) string {
	t.Helper()
	w := newTestWindow()
	layout := generateViewLayout(v, w)
	found, ok := layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape %q", id)
	}
	if found.Shape.a11Y == nil {
		t.Fatalf("%s: no a11y node", id)
	}
	return found.Shape.a11Y.Label
}

// A Label is prose, not an ID path: a colon in it is part of the
// name (issue #876). Each widget below fell back through a11yLabel,
// which kept only the text after the last IDSep.
func TestA11YLabelKeepsColonPerWidget(t *testing.T) {
	const want = "Price: USD"
	cases := []struct {
		name string
		view View
		id   string
	}{
		{"Input", Input(InputCfg{ID: "in", Label: want}), "in"},
		{"Select", Select(SelectCfg{ID: "sel", Label: want}), "sel"},
		{"Combobox", Combobox(ComboboxCfg{ID: "cb", Label: want}), "cb"},
		{"NumericInput", NumericInput(NumericInputCfg{ID: "ni", Label: want,
			StepCfg: NumericStepCfg{ShowButtons: true, Step: 1}}), "ni"},
		{"NumericInputNoSteppers", NumericInput(NumericInputCfg{ID: "ni", Label: want}), "ni"},
		{"InputDate", InputDate(InputDateCfg{ID: "id", Label: want}), "id"},
		{"DatePicker", DatePicker(DatePickerCfg{ID: "dp", Label: want}), "dp"},
		{"ColorPicker", ColorPicker(ColorPickerCfg{ID: "cp", Label: want}), "cp"},
		{"Slider", Slider(SliderCfg{ID: "sl", Label: want}), "sl"},
		{"Radio", Radio(RadioCfg{ID: "ra", Label: want}), "ra"},
		{"Switch", Switch(SwitchCfg{ID: "sw", Label: want}), "sw"},
		{"Toggle", Toggle(ToggleCfg{ID: "tg", Label: want}), "tg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := a11yLabelFor(t, tc.view, tc.id); got != want {
				t.Errorf("a11y label = %q, want %q", got, want)
			}
		})
	}
}

// A Placeholder is display text, not an ID path, so it reaches the
// accessible name unchanged (issue #876).
func TestA11YPlaceholderKeepsColon(t *testing.T) {
	const want = "Search: all"
	cases := []struct {
		name string
		view View
		id   string
	}{
		{"Input", Input(InputCfg{ID: "in", Placeholder: want}), "in"},
		{"Select", Select(SelectCfg{ID: "sel", Placeholder: want}), "sel"},
		{"Combobox", Combobox(ComboboxCfg{ID: "cb", Placeholder: want}), "cb"},
		{"NumericInput", NumericInput(NumericInputCfg{ID: "ni", Placeholder: want}), "ni"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := a11yLabelFor(t, tc.view, tc.id); got != want {
				t.Errorf("a11y label = %q, want %q", got, want)
			}
		})
	}
}

// Menu item text, progress text and badge labels are prose, not ID
// paths (issue #876). Text is the same bug class: its fallback is
// the shown string.
func TestA11YDisplayTextKeepsColon(t *testing.T) {
	t.Run("MenuItem", func(t *testing.T) {
		w := newTestWindow()
		layout := generateViewLayout(
			menuItem(MenubarCfg{}, MenuItemText("m", "File: New")), w)
		if layout.Shape.a11Y == nil || layout.Shape.a11Y.Label != "File: New" {
			t.Errorf("a11y %+v, want label %q", layout.Shape.a11Y, "File: New")
		}
	})
	t.Run("ProgressBar", func(t *testing.T) {
		if got := a11yLabelFor(t,
			ProgressBar(ProgressBarCfg{ID: "pb", Text: "Step: 1/2"}),
			"pb"); got != "Step: 1/2" {
			t.Errorf("a11y label = %q, want %q", got, "Step: 1/2")
		}
	})
	t.Run("Badge", func(t *testing.T) {
		layout := generateViewLayout(Badge(BadgeCfg{Label: "Note: new"}), newTestWindow())
		if layout.Shape.a11Y == nil || layout.Shape.a11Y.Label != "Note: new" {
			t.Errorf("a11y %+v, want label %q", layout.Shape.a11Y, "Note: new")
		}
	})
	t.Run("Text", func(t *testing.T) {
		layout := generateViewLayout(
			Text(TextCfg{ID: "t", Text: "Time: now"}), newTestWindow())
		if layout.Shape.a11Y == nil || layout.Shape.a11Y.Label != "Time: now" {
			t.Errorf("a11y %+v, want label %q", layout.Shape.a11Y, "Time: now")
		}
	})
}

// The prose fallback uses the text as written; an explicit label
// still wins over it.
func TestA11YProseLabelFallback(t *testing.T) {
	if got := a11yProseLabel("", "Price: USD"); got != "Price: USD" {
		t.Errorf("fallback = %q, want %q", got, "Price: USD")
	}
	if got := a11yProseLabel("explicit", "Price: USD"); got != "explicit" {
		t.Errorf("explicit = %q, want explicit", got)
	}
	if info := (A11YCfg{}).a11yInfoProse("Price: USD"); info == nil ||
		info.Label != "Price: USD" {
		t.Errorf("a11yInfoProse = %+v, want label %q", info, "Price: USD")
	}
}
