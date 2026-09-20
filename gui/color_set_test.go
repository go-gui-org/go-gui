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
	// It survives resolution rather than being refilled from the
	// theme: the theme only speaks for a slot nobody claimed.
	got := cs.resolved(Color{}, Flat(Red))
	if got.Base != ColorTransparent {
		t.Fatalf("Base = %v, want ColorTransparent", got.Base)
	}
}

// The precedence rule: the Color shorthand beats Colors.Base, and only
// Base — the other slots are the set's to fill.
func TestColorSetShorthandWinsOverBaseOnly(t *testing.T) {
	got := ColorSet{Base: Blue, Hover: Green}.resolved(Red, ColorSet{})
	if got.Base != Red {
		t.Errorf("Base = %v, want Red — the shorthand must win", got.Base)
	}
	if got.Hover != Green {
		t.Errorf("Hover = %v, want Green — the shorthand must not "+
			"reach past Base", got.Hover)
	}
}

// End to end through the widget: Flat resolves to one color in every
// state, which is what the deleted six-field literal used to spell out.
func TestButtonColorSetResolvesEveryState(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Colors: Flat(Blue)}
	applyButtonDefaults(&cfg, &defaultButtonStyle)
	c := cfg.Colors
	if c.Base != Blue || c.Hover != Blue || c.Click != Blue ||
		c.Focus != Blue || c.Border != Blue || c.BorderFocus != Blue {
		t.Fatalf("Flat(Blue) did not reach every state: %+v", c)
	}
}

// Theme defaults still apply to whatever the ColorSet left unspecified.
func TestButtonColorSetLeavesThemeDefaultsForUnsetFields(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Colors: ColorSet{Base: Blue}}
	applyButtonDefaults(&cfg, &defaultButtonStyle)
	if cfg.Colors.Base != Blue {
		t.Fatalf("Base = %v, want Blue", cfg.Colors.Base)
	}
	if cfg.Colors.Border != defaultButtonStyle.Colors.Border {
		t.Fatalf("Border = %v, want the theme default %v",
			cfg.Colors.Border, defaultButtonStyle.Colors.Border)
	}
}

// Color survives as the single-color shorthand and outranks Colors.Base.
func TestButtonColorShorthandWinsOverBase(t *testing.T) {
	cfg := ButtonCfg{ID: "b", Color: Red, Colors: ColorSet{Base: Blue}}
	applyButtonDefaults(&cfg, &defaultButtonStyle)
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
	applyButtonDefaults(&cfg, &defaultButtonStyle)
	if cfg.Colors.Base != RGB(50, 50, 50) {
		t.Fatalf("Base = %v, want the assigned color", cfg.Colors.Base)
	}
	if cfg.Colors.Focus != defaultButtonStyle.Colors.Focus {
		t.Fatalf("Focus = %v, want the theme default %v — Color must "+
			"not pin the interactive states",
			cfg.Colors.Focus, defaultButtonStyle.Colors.Focus)
	}
	if cfg.Colors.Focus == cfg.Colors.Base {
		t.Fatal("focus color equals base; focus would be invisible")
	}
}

// A Cfg that touches no color at all is entirely theme-driven.
func TestButtonWithoutAnyColorUsesTheme(t *testing.T) {
	cfg := ButtonCfg{ID: "b"}
	applyButtonDefaults(&cfg, &defaultButtonStyle)
	if cfg.Colors.Base != defaultButtonStyle.Colors.Base {
		t.Fatalf("Base = %v, want theme default", cfg.Colors.Base)
	}
	if cfg.Colors.Hover != defaultButtonStyle.Colors.Hover {
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

// The twelve widgets that fanned a caller's ColorSet out into flat
// Color* fields through applyTo now resolve straight into Colors —
// the flats are gone (issue #721). Flat(Blue) reaches every live
// slot of each widget's set.
func TestColorSetAdoptedByTheTwelve(t *testing.T) {
	in := InputCfg{ID: "in", Colors: Flat(Blue)}
	applyInputDefaults(&in)
	ni := NumericInputCfg{ID: "ni", Colors: Flat(Blue)}
	applyNumericInputDefaults(&ni)
	sel := SelectCfg{ID: "sel", Colors: Flat(Blue)}
	applySelectDefaults(&sel)
	cb := ComboboxCfg{ID: "cb", Colors: Flat(Blue)}
	applyComboboxDefaults(&cb)
	lb := ListBoxCfg{ID: "lb", Colors: Flat(Blue)}
	applyListBoxDefaults(&lb)
	tr := TreeCfg{ID: "tr", Colors: Flat(Blue)}
	applyTreeDefaults(&tr)
	sl := SliderCfg{ID: "sl", Colors: Flat(Blue)}
	applySliderDefaults(&sl)
	cm := ContextMenuCfg{Colors: Flat(Blue)}
	applyContextMenuDefaults(&cm)
	mb := MenubarCfg{ID: "mb", Colors: Flat(Blue)}
	applyMenubarDefaults(&mb)
	ep := ExpandPanelCfg{Colors: Flat(Blue)}
	applyExpandPanelDefaults(&ep)

	for name, got := range map[string]ColorSet{
		"Input":        in.Colors,
		"NumericInput": ni.Colors,
		"Select":       sel.Colors,
		"Combobox":     cb.Colors,
		"ListBox":      lb.Colors,
		"Tree":         tr.Colors,
		"Slider":       sl.Colors,
		"ContextMenu":  cm.Colors,
		"Menubar":      mb.Colors,
		"ExpandPanel":  ep.Colors,
	} {
		if got.Base != Blue || got.Hover != Blue || got.Click != Blue ||
			got.Focus != Blue || got.Border != Blue ||
			got.BorderFocus != Blue {
			t.Errorf("%s: Flat(Blue) did not resolve: %+v", name, got)
		}
	}
	// Table has no base fill: Flat(Blue) reaches Hover and Border.
	tb := TableCfg{Colors: Flat(Blue)}
	applyTableDefaults(&tb)
	if tb.Colors.Hover != Blue || tb.Colors.Border != Blue ||
		tb.Colors.BorderFocus != Blue {
		t.Errorf("Table: Flat(Blue) did not resolve: %+v", tb.Colors)
	}
	// VirtualList shares the list-box style and has no hover path:
	// Flat(Blue) reaches Base and the borders.
	vl := VirtualListCfg{ID: "vl", Colors: Flat(Blue)}
	applyVirtualListDefaults(&vl)
	if vl.Colors.Base != Blue || vl.Colors.Border != Blue ||
		vl.Colors.BorderFocus != Blue {
		t.Errorf("VirtualList: Flat(Blue) did not resolve: %+v",
			vl.Colors)
	}

	// Precedence: the Color shorthand wins for Base, the set covers
	// the rest.
	win := InputCfg{ID: "in", Color: Red, Colors: Flat(Blue)}
	applyInputDefaults(&win)
	if win.Colors.Base != Red {
		t.Errorf("Base = %v, want Red — Color must win", win.Colors.Base)
	}
	if win.Colors.Hover != Blue || win.Colors.Border != Blue {
		t.Errorf("Hover/Border = %v/%v, want Blue — unset slots take "+
			"the set", win.Colors.Hover, win.Colors.Border)
	}
}

// pickTestSet gives every slot a distinguishable color, so a pick that
// returns the wrong field cannot pass by aliasing another. Flat would
// hide exactly that class of mistake.
var pickTestSet = ColorSet{
	Base:        RGB(1, 1, 1),
	Hover:       RGB(2, 2, 2),
	Click:       RGB(3, 3, 3),
	Focus:       RGB(4, 4, 4),
	Border:      RGB(5, 5, 5),
	BorderFocus: RGB(6, 6, 6),
}

// TestColorSetPick walks all sixteen flag combinations. The rules under
// test are two, one per channel:
//
//	fill:   disabled > pressed > hovered > focused > base
//	border: disabled > focused > base
func TestColorSetPick(t *testing.T) {
	cs := pickTestSet
	tests := []struct {
		name         string
		flags        stateFlags
		fill, border Color
	}{
		{"resting", stateFlags{}, cs.Base, cs.Border},
		{"hovered", stateFlags{hovered: true}, cs.Hover, cs.Border},
		{"pressed", stateFlags{pressed: true}, cs.Click, cs.Border},
		{"focused", stateFlags{focused: true}, cs.Focus, cs.BorderFocus},

		// The decision this refactor exists to make: the fill goes to
		// the pointer, the border stays with focus.
		{"focused and hovered",
			stateFlags{focused: true, hovered: true},
			cs.Hover, cs.BorderFocus},
		{"focused and pressed",
			stateFlags{focused: true, pressed: true},
			cs.Click, cs.BorderFocus},
		{"pressed beats hovered",
			stateFlags{pressed: true, hovered: true},
			cs.Click, cs.Border},
		{"pressed beats hovered and focused",
			stateFlags{pressed: true, hovered: true, focused: true},
			cs.Click, cs.BorderFocus},

		// The bug history: a disabled widget takes its resting colors
		// whatever else is true of it.
		{"disabled", stateFlags{disabled: true}, cs.Base, cs.Border},
		{"disabled and hovered",
			stateFlags{disabled: true, hovered: true},
			cs.Base, cs.Border},
		{"disabled and pressed",
			stateFlags{disabled: true, pressed: true},
			cs.Base, cs.Border},
		{"disabled and focused",
			stateFlags{disabled: true, focused: true},
			cs.Base, cs.Border},
		{"disabled and hovered and focused",
			stateFlags{disabled: true, hovered: true, focused: true},
			cs.Base, cs.Border},
		{"disabled and pressed and hovered",
			stateFlags{disabled: true, pressed: true, hovered: true},
			cs.Base, cs.Border},
		{"disabled and pressed and focused",
			stateFlags{disabled: true, pressed: true, focused: true},
			cs.Base, cs.Border},
		{"disabled and everything",
			stateFlags{disabled: true, pressed: true, hovered: true,
				focused: true},
			cs.Base, cs.Border},
	}
	if len(tests) != 16 {
		t.Fatalf("table covers %d combinations, want all 16",
			len(tests))
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fill, border := cs.pick(tc.flags)
			if fill != tc.fill {
				t.Errorf("fill = %v, want %v", fill, tc.fill)
			}
			if border != tc.border {
				t.Errorf("border = %v, want %v", border, tc.border)
			}
		})
	}
}

// TestColorSetPickUnresolvedStaysUnset pins the precondition in pick's
// doc comment: pick does no fallback, so a set that never went through
// resolved() answers with unset colors rather than with Base. A widget
// that forgets to resolve gets a blank shape, not a plausible one.
func TestColorSetPickUnresolvedStaysUnset(t *testing.T) {
	cs := ColorSet{Base: RGB(1, 1, 1)}
	if fill, _ := cs.pick(stateFlags{hovered: true}); fill.IsSet() {
		t.Errorf("unresolved hover fill = %v, want unset", fill)
	}
	if fill, _ := cs.pick(stateFlags{}); fill != cs.Base {
		t.Errorf("unresolved resting fill = %v, want Base", fill)
	}
}

// pickFillSink and pickBorderSink keep the pick results reachable, so
// the compiler cannot delete the call the allocation gate measures.
var pickFillSink, pickBorderSink Color

// TestColorSetPickAllocFree gates what makes pick usable on the view
// path at all: it runs for every styled widget in both the amend and
// the hover pass, every frame.
func TestColorSetPickAllocFree(t *testing.T) {
	cs := pickTestSet
	s := stateFlags{hovered: true, focused: true}
	allocs := testing.AllocsPerRun(100, func() {
		pickFillSink, pickBorderSink = cs.pick(s)
	})
	if allocs != 0 {
		t.Errorf("pick allocated %v times per run, want 0", allocs)
	}
}
