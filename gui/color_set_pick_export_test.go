package gui

import "testing"

// Tests for the exported ColorSet.Pick and ColorSet.Resolved (#741).
// They are the seam a widget outside gui/ (datagrid) uses to get the
// same state order as the widgets inside it.

// Pick must answer exactly what pick answers for every state, so an
// outside widget cannot drift from an inside one.
func TestColorSetPickMatchesPick(t *testing.T) {
	cs := selectedTestSet
	tests := []struct {
		name  string
		state PickState
		flags stateFlags
	}{
		{"resting", PickState{}, stateFlags{}},
		{"hovered", PickState{Hovered: true}, stateFlags{hovered: true}},
		{"pressed", PickState{Pressed: true}, stateFlags{pressed: true}},
		{"focused", PickState{Focused: true}, stateFlags{focused: true}},
		{"selected", PickState{Selected: true}, stateFlags{selected: true}},
		{"selected hovered", PickState{Selected: true, Hovered: true},
			stateFlags{selected: true, hovered: true}},
		{"selected focused", PickState{Selected: true, Focused: true},
			stateFlags{selected: true, focused: true}},
		{"disabled", PickState{Disabled: true, Hovered: true},
			stateFlags{disabled: true, hovered: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fill, border := cs.Pick(tc.state)
			wantFill, wantBorder := cs.pick(tc.flags)
			if fill != wantFill || border != wantBorder {
				t.Errorf("Pick = (%v, %v), want (%v, %v)",
					fill, border, wantFill, wantBorder)
			}
		})
	}
}

// Resolved applies the set's own fallbacks, then the theme, and leaves
// no shorthand in the way: Base comes from the caller or the theme.
func TestColorSetResolved(t *testing.T) {
	theme := selectedTestSet
	got := ColorSet{Base: Red}.Resolved(theme)
	// Hover, Click and Focus fall back to the caller's Base before the
	// theme, so a caller who sets only Base gets a flat fill.
	if got.Base != Red || got.Hover != Red || got.Click != Red || got.Focus != Red {
		t.Errorf("interactive slots = %v %v %v %v, want all Red",
			got.Base, got.Hover, got.Click, got.Focus)
	}
	if got.Selected != theme.Selected || got.Disabled != theme.Disabled {
		t.Errorf("Selected, Disabled = %v, %v, want the theme's %v, %v",
			got.Selected, got.Disabled, theme.Selected, theme.Disabled)
	}
	if got.Border != theme.Border || got.BorderFocus != theme.BorderFocus {
		t.Errorf("borders = %v, %v, want the theme's", got.Border, got.BorderFocus)
	}

	// A zero set takes every slot from the theme.
	if got = (ColorSet{}).Resolved(theme); got != theme {
		t.Errorf("zero set resolved = %+v, want the theme %+v", got, theme)
	}
}

// Pick does no fallback of its own. A set not passed through Resolved
// returns the zero Color for a slot the caller left unset. This is the
// documented misuse; the test pins it so the doc stays true.
func TestColorSetPickUnresolved(t *testing.T) {
	cs := ColorSet{Base: Red}
	if fill, _ := cs.Pick(PickState{Hovered: true}); fill.IsSet() {
		t.Errorf("unresolved hover fill = %v, want the zero Color", fill)
	}
	if fill, _ := cs.Resolved(ColorSet{}).Pick(PickState{Hovered: true}); fill != Red {
		t.Errorf("resolved hover fill = %v, want Red", fill)
	}
}
