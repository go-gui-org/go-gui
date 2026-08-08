package gui

import "testing"

func TestColorSetZeroValueIsUnset(t *testing.T) {
	var cs ColorSet
	if cs.IsSet() {
		t.Fatal("zero ColorSet reports IsSet() = true, want false")
	}
}

// Base backs the three interactive states but deliberately not the
// borders — see the field comments on ColorSet.Border.
func TestColorSetBaseBacksStatesNotBorders(t *testing.T) {
	r := ColorSet{Base: Blue}.resolve()
	for name, got := range map[string]Color{
		"Hover": r.Hover,
		"Click": r.Click,
		"Focus": r.Focus,
	} {
		if got != Blue {
			t.Errorf("%s = %v, want Blue (fallback to Base)", name, got)
		}
	}
	if r.Border.IsSet() {
		t.Errorf("Border = %v, want unset — Base must not back it", r.Border)
	}
	if r.BorderFocus.IsSet() {
		t.Errorf("BorderFocus = %v, want unset", r.BorderFocus)
	}
}

func TestColorSetBorderFocusFallsBackToBorder(t *testing.T) {
	r := ColorSet{Border: Red}.resolve()
	if r.BorderFocus != Red {
		t.Fatalf("BorderFocus = %v, want Red", r.BorderFocus)
	}
}

// Flat differs from ColorSet{Base: c} precisely in pinning the borders.
func TestFlatPinsEveryFieldIncludingBorders(t *testing.T) {
	cs := Flat(Green)
	if cs.Base != Green || cs.Hover != Green || cs.Click != Green ||
		cs.Focus != Green || cs.Border != Green ||
		cs.BorderFocus != Green {
		t.Fatalf("Flat(Green) = %+v, want every field Green", cs)
	}
	if base := (ColorSet{Base: Green}).resolve(); base.Border.IsSet() {
		t.Fatal("ColorSet{Base:} resolved a Border; Flat must be the " +
			"only way to pin borders")
	}
}

// An explicitly transparent color is a real choice, not an omission.
// This is the case Opt[Color] was proposed to handle, and the reason
// it is not needed: Color already tracks it.
func TestColorSetTransparentIsNotUnset(t *testing.T) {
	cs := ColorSet{Base: ColorTransparent}
	if !cs.IsSet() {
		t.Fatal("ColorSet{Base: ColorTransparent}.IsSet() = false")
	}
	dst := Color{}
	cs.applyTo(&dst, nil, nil, nil, nil, nil)
	if dst != ColorTransparent {
		t.Fatalf("dst = %v, want ColorTransparent", dst)
	}
}

// The precedence rule: an assigned flat field beats the ColorSet.
func TestColorSetApplyToLetsFlatFieldWin(t *testing.T) {
	flat := Red
	unset := Color{}
	Flat(Blue).applyTo(&flat, &unset, nil, nil, nil, nil)
	if flat != Red {
		t.Errorf("assigned flat field = %v, want Red — ColorSet must "+
			"not overwrite it", flat)
	}
	if unset != Blue {
		t.Errorf("unassigned flat field = %v, want Blue", unset)
	}
}

func TestColorSetApplyToToleratesNilDestinations(t *testing.T) {
	// A widget without, say, a click color passes nil for it.
	Flat(Blue).applyTo(nil, nil, nil, nil, nil, nil)
}

// End to end through the widget: the six-line "don't react" literal
// collapses to one field and produces the same flat colors.
func TestButtonColorSetReachesFlatFields(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Colors: Flat(Blue)}
	applyButtonDefaults(&cfg)
	if cfg.Color != Blue || cfg.ColorHover != Blue ||
		cfg.ColorClick != Blue || cfg.ColorFocus != Blue ||
		cfg.ColorBorder != Blue || cfg.ColorBorderFocus != Blue {
		t.Fatalf("Flat(Blue) did not reach every flat field: %+v", cfg)
	}
}

// Theme defaults still apply to whatever the ColorSet left unspecified.
func TestButtonColorSetLeavesThemeDefaultsForUnsetFields(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Colors: ColorSet{Base: Blue}}
	applyButtonDefaults(&cfg)
	if cfg.Color != Blue {
		t.Fatalf("Color = %v, want Blue", cfg.Color)
	}
	if cfg.ColorBorder != DefaultButtonStyle.ColorBorder {
		t.Fatalf("ColorBorder = %v, want the theme default %v",
			cfg.ColorBorder, DefaultButtonStyle.ColorBorder)
	}
}

// Existing code that sets only flat fields must be unaffected by the
// new field's existence.
func TestButtonWithoutColorSetIsUnchanged(t *testing.T) {
	withSet := ButtonCfg{ID: "b", ColorHover: Red}
	applyButtonDefaults(&withSet)
	if withSet.ColorHover != Red {
		t.Fatalf("ColorHover = %v, want Red", withSet.ColorHover)
	}
	if withSet.Color != DefaultButtonStyle.Color {
		t.Fatalf("Color = %v, want theme default", withSet.Color)
	}
}
