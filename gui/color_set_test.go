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

// End to end through the widget: Flat resolves to one color in every
// state, which is what the deleted six-field literal used to spell out.
func TestButtonColorSetResolvesEveryState(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Colors: Flat(Blue)}
	applyButtonDefaults(&cfg)
	c := cfg.Colors
	if c.Base != Blue || c.Hover != Blue || c.Click != Blue ||
		c.Focus != Blue || c.Border != Blue || c.BorderFocus != Blue {
		t.Fatalf("Flat(Blue) did not reach every state: %+v", c)
	}
}

// Theme defaults still apply to whatever the ColorSet left unspecified.
func TestButtonColorSetLeavesThemeDefaultsForUnsetFields(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Colors: ColorSet{Base: Blue}}
	applyButtonDefaults(&cfg)
	if cfg.Colors.Base != Blue {
		t.Fatalf("Base = %v, want Blue", cfg.Colors.Base)
	}
	if cfg.Colors.Border != DefaultButtonStyle.ColorBorder {
		t.Fatalf("Border = %v, want the theme default %v",
			cfg.Colors.Border, DefaultButtonStyle.ColorBorder)
	}
}

// Color survives as the single-color shorthand and outranks Colors.Base.
func TestButtonColorShorthandWinsOverBase(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Color: Red, Colors: ColorSet{Base: Blue}}
	applyButtonDefaults(&cfg)
	if cfg.Colors.Base != Red {
		t.Fatalf("Base = %v, want Red — Color must win", cfg.Colors.Base)
	}
}

// The shorthand must NOT back the interactive states. Setting only a
// background color has never disabled hover and focus feedback, and
// ColorSet must not make it start doing so — Flat(c) is the opt-in for
// a widget that should not react. Regression: an earlier ordering
// folded Color into Base before the state fallback and silently pinned
// all three states, which reds TestButtonAmendLayoutFocus.
func TestButtonColorShorthandLeavesStatesThemed(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Color: RGB(50, 50, 50)}
	applyButtonDefaults(&cfg)
	if cfg.Colors.Base != RGB(50, 50, 50) {
		t.Fatalf("Base = %v, want the assigned color", cfg.Colors.Base)
	}
	if cfg.Colors.Focus != DefaultButtonStyle.ColorFocus {
		t.Fatalf("Focus = %v, want the theme default %v — Color must "+
			"not pin the interactive states",
			cfg.Colors.Focus, DefaultButtonStyle.ColorFocus)
	}
	if cfg.Colors.Focus == cfg.Colors.Base {
		t.Fatal("focus color equals base; focus would be invisible")
	}
}

// A Cfg that touches no color at all is entirely theme-driven.
func TestButtonWithoutAnyColorUsesTheme(t *testing.T) {
	cfg := ButtonCfg{ID: "b"}
	applyButtonDefaults(&cfg)
	if cfg.Colors.Base != DefaultButtonStyle.Color {
		t.Fatalf("Base = %v, want theme default", cfg.Colors.Base)
	}
	if cfg.Colors.Hover != DefaultButtonStyle.ColorHover {
		t.Fatalf("Hover = %v, want theme default", cfg.Colors.Hover)
	}
}

// The same resolution must hold for every widget that adopted ColorSet,
// not just Button — that was the point of widening the scope.
func TestColorSetAdoptedByAllSixWidgets(t *testing.T) {
	sw := SwitchCfg{ID: "s", Colors: Flat(Blue)}
	applySwitchDefaults(&sw)
	tg := ToggleCfg{ID: "t", Colors: Flat(Blue)}
	applyToggleDefaults(&tg)
	rd := RadioCfg{ID: "r", Colors: Flat(Blue)}
	applyRadioDefaults(&rd)
	for name, got := range map[string]ColorSet{
		"Switch": sw.Colors,
		"Toggle": tg.Colors,
		"Radio":  rd.Colors,
	} {
		if got.Hover != Blue || got.BorderFocus != Blue {
			t.Errorf("%s: Flat(Blue) did not resolve: %+v", name, got)
		}
	}
}
