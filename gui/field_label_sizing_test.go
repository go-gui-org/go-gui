package gui

import "testing"

// The wrapper labelledField puts around a field hardcoded Sizing:
// FitFit, so the caller's sizing was unreachable: a FillFit Input in a
// FillFit Row still arranged at the width of its text, and no labelled
// field in any app built on this toolkit could fill its row. Nine
// widgets route through that function, so the bug was invisible per
// widget and total in aggregate.
//
// These tests pin the pass-through in both directions: a stated Fill
// width reaches the wrapper, and an unstated sizing still gets the Fit
// wrapper it got before, which is what keeps the fix from being a
// visual break for callers that never asked for a width.

const fieldSizingWindow = 400

// arrangeLabelledField renders one labelled field as the only child of
// a filling row and returns the arranged wrapper (the label stack) and
// the field shape itself.
func arrangeLabelledField(t *testing.T, field View, id string) (*Layout, *Layout) {
	t.Helper()
	w := NewTestWindow(WindowCfg{
		Width:  fieldSizingWindow,
		Height: 200,
	})
	root := w.TestRender(func(_ *Window) View {
		return Row(ContainerCfg{
			ID:         "row",
			Sizing:     FillFit,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content:    []View{field},
		})
	})
	if root == nil {
		t.Fatal("TestRender returned nil")
	}
	row, ok := root.FindByID("row")
	if !ok {
		t.Fatal("row not found")
	}
	if len(row.Children) != 1 {
		t.Fatalf("row has %d children, want 1", len(row.Children))
	}
	wrapper := &row.Children[0]
	shape, ok := root.FindByID(ScopeID("row", id))
	if !ok {
		t.Fatalf("field %q not found", ScopeID("row", id))
	}
	return wrapper, shape
}

// A labelled field that asks to fill gets the row's width, and the
// wrapper stays content-height so the label/field stack still hugs.
func TestLabelledFieldFillsRow(t *testing.T) {
	wrapper, field := arrangeLabelledField(t, Input(InputCfg{
		ID:       "name",
		Label:    "Full name",
		Text:     "Mike",
		Sizing:   FillFit,
		MinWidth: 40,
	}), "name")

	if wrapper.Shape.Width != fieldSizingWindow {
		t.Errorf("wrapper width = %f, want %d",
			wrapper.Shape.Width, fieldSizingWindow)
	}
	if field.Shape.Width != fieldSizingWindow {
		t.Errorf("field width = %f, want %d",
			field.Shape.Width, fieldSizingWindow)
	}
	// Fit height, not Fill: the wrapper must not swallow the row.
	if wrapper.Shape.Height >= 200 {
		t.Errorf("wrapper height = %f, want content height",
			wrapper.Shape.Height)
	}
}

// A Cfg that never states a Sizing keeps the Fit wrapper it had, so
// the pass-through is additive. The field itself still takes the
// theme's min-width floor, which is the separate change in the same
// phase — assert the wrapper tracks the field rather than the row.
func TestLabelledFieldUnsizedStaysFit(t *testing.T) {
	wrapper, field := arrangeLabelledField(t, Input(InputCfg{
		ID:    "name",
		Label: "Full name",
		Text:  "Mike",
	}), "name")

	if wrapper.Shape.Width >= fieldSizingWindow {
		t.Errorf("wrapper width = %f, want less than the row",
			wrapper.Shape.Width)
	}
	if wrapper.Shape.Width != field.Shape.Width {
		t.Errorf("wrapper width = %f, field width = %f; a Fit wrapper "+
			"must hug its widest child",
			wrapper.Shape.Width, field.Shape.Width)
	}
}

// fitHeight maps every width mode through the predefined Sizing vars,
// which is what keeps a raw Sizing literal (read as unset) out of
// field_label.go.
func TestFitHeightKeepsWidthPinsHeight(t *testing.T) {
	// Declared, not a literal: a raw Sizing{...} is exactly the
	// mistake the literals audit gates, and the zero value is what
	// this case is about.
	var unset Sizing
	cases := []struct {
		in   Sizing
		want Sizing
	}{
		{unset, FitFit}, // unset: today's wrapper
		{FillFill, FillFit},
		{FillFit, FillFit},
		{FixedFixed, FixedFit},
		{FitFill, FitFit},
	}
	for _, c := range cases {
		if got := fitHeight(c.in); got != c.want {
			t.Errorf("fitHeight(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// The theme states the field min-width floor once, so an Input, a
// Select and a Combobox in one row agree on it. Before, ThemeMaker
// spelled MinWidth 75 / MaxWidth 200 on selectStyle and again on
// comboboxStyle, and Input had no floor at all.
func TestFieldMinWidthFloorIsShared(t *testing.T) {
	floor := guiTheme.SizeFieldMinWidth
	if floor == 0 {
		t.Fatal("theme states no field min width")
	}

	in := InputCfg{ID: "in"}
	applyInputDefaults(&in)
	if in.MinWidth != floor {
		t.Errorf("Input min width = %f, want %f", in.MinWidth, floor)
	}

	sel := SelectCfg{ID: "sel"}
	applySelectDefaults(&sel)
	if sel.MinWidth != floor {
		t.Errorf("Select min width = %f, want %f", sel.MinWidth, floor)
	}
	if sel.MaxWidth != 0 {
		t.Errorf("Select max width = %f, want uncapped", sel.MaxWidth)
	}

	num := NumericInputCfg{ID: "num"}
	applyNumericInputDefaults(&num)
	if num.MinWidth != floor {
		t.Errorf("NumericInput min width = %f, want %f", num.MinWidth, floor)
	}

	date := InputDateCfg{ID: "date"}
	applyInputDateDefaults(&date)
	if date.MinWidth != floor {
		t.Errorf("InputDate min width = %f, want %f", date.MinWidth, floor)
	}

	cb := ComboboxCfg{ID: "cb"}
	applyComboboxDefaults(&cb)
	if cb.MinWidth != floor {
		t.Errorf("Combobox min width = %f, want %f", cb.MinWidth, floor)
	}
	if cb.MaxWidth != 0 {
		t.Errorf("Combobox max width = %f, want uncapped", cb.MaxWidth)
	}
}

// A stated Width answers the question the floor exists for, so it opts
// out — otherwise a narrow field (the ColorFields channel inputs) would
// be forced to the floor and the column would blow apart.
func TestFieldMinWidthFloorYieldsToStatedWidth(t *testing.T) {
	in := InputCfg{ID: "in", Width: 48}
	applyInputDefaults(&in)
	if in.MinWidth != 0 {
		t.Errorf("min width = %f, want 0 for a stated Width", in.MinWidth)
	}
}
