package gui

import "testing"

// Roles must track the theme's own text style, or a theme that changes
// its type scale silently leaves them behind.
func TestTextRolesTrackTextStyleDef(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorBackground = RGB(48, 48, 48)
	cfg.TextStyleDef = TextStyle{
		Family: "TestFace",
		Color:  RGB(200, 210, 220),
		Size:   19,
	}
	th := ThemeMaker(cfg)

	roles := map[string]TextStyle{
		"secondary":   th.TextStyleSecondary,
		"disabled":    th.TextStyleDisabled,
		"placeholder": th.TextStylePlaceholder,
		"label":       th.TextStyleLabel,
	}
	for name, r := range roles {
		if r.Family != "TestFace" {
			t.Errorf("%s: Family = %q, want TestFace", name, r.Family)
		}
		if r.Color.R != 200 || r.Color.G != 210 || r.Color.B != 220 {
			t.Errorf("%s: color hue = %v, want the theme's own",
				r.Color, name)
		}
		if r.Color.A == 255 {
			t.Errorf("%s: alpha 255, role is not de-emphasized", name)
		}
	}

	// Only the label steps size; it names a value rather than being
	// one, so it sits a rung down.
	for name, r := range map[string]TextStyle{
		"secondary":   th.TextStyleSecondary,
		"disabled":    th.TextStyleDisabled,
		"placeholder": th.TextStylePlaceholder,
	} {
		if r.Size != 19 {
			t.Errorf("%s: Size = %v, want 19", name, r.Size)
		}
	}
	if th.TextStyleLabel.Size != cfg.SizeTextXSmall {
		t.Errorf("label Size = %v, want %v",
			th.TextStyleLabel.Size, cfg.SizeTextXSmall)
	}
}

// The ladder is ordered: a placeholder is quieter than disabled text,
// which is quieter than secondary, which is quieter than a label. If
// that ordering breaks, the roles stop being distinguishable on screen.
func TestTextRolesOrdered(t *testing.T) {
	for _, th := range []Theme{ThemeDark, ThemeLight} {
		a := []struct {
			name  string
			alpha uint8
		}{
			{"placeholder", th.TextStylePlaceholder.Color.A},
			{"disabled", th.TextStyleDisabled.Color.A},
			{"secondary", th.TextStyleSecondary.Color.A},
			{"label", th.TextStyleLabel.Color.A},
		}
		for i := 1; i < len(a); i++ {
			if a[i].alpha <= a[i-1].alpha {
				t.Errorf("%s theme: %s (%d) not quieter than %s (%d)",
					th.Name, a[i-1].name, a[i-1].alpha,
					a[i].name, a[i].alpha)
			}
		}
	}
}

// The point of per-theme ladders: the light theme must not simply reuse
// the dark alphas, because the same alpha yields lower contrast against
// a light ground. This is the regression that would silently undo the
// contrast matching.
func TestTextRolesDifferPerTheme(t *testing.T) {
	if ThemeDark.TextStyleDisabled.Color.A ==
		ThemeLight.TextStyleDisabled.Color.A {
		t.Error("dark and light disabled alpha are equal; " +
			"the light ladder is not contrast-matched")
	}
	if ThemeLight.TextStyleDisabled.Color.A <
		ThemeDark.TextStyleDisabled.Color.A {
		t.Error("light disabled alpha is lower than dark; " +
			"dark text on a light ground needs more, not less")
	}
}

// Polarity is derived, not configured, so a theme an app builds itself
// still gets the right ladder.
func TestTextRolesPolarityDerived(t *testing.T) {
	onDark := textRolesFor(RGB(225, 225, 225), RGB(48, 48, 48))
	if onDark != textRolesOnDark {
		t.Error("light text over dark background: want the dark ladder")
	}
	onLight := textRolesFor(RGB(32, 32, 32), RGB(225, 225, 225))
	if onLight != textRolesOnLight {
		t.Error("dark text over light background: want the light ladder")
	}
}

// An explicit ColorText* wins over the derived ladder — that is what
// makes a role a per-theme value rather than a fixed multiplier.
func TestTextRolesExplicitOverride(t *testing.T) {
	cfg := baseCfg()
	cfg.ColorBackground = RGB(48, 48, 48)
	cfg.TextStyleDef = DefaultTextStyle
	want := RGBA(10, 20, 30, 200)
	cfg.ColorTextSecondary = want

	th := ThemeMaker(cfg)
	if !th.TextStyleSecondary.Color.eq(want) {
		t.Errorf("secondary = %v, want the configured %v",
			th.TextStyleSecondary.Color, want)
	}
	// The roles that were not overridden still derive.
	if th.TextStyleDisabled.Color.eq(want) {
		t.Error("overriding secondary must not move disabled")
	}
}

// The disabled role carries the marker that exempts it from
// renderText's second dim; no other role may, or ordinary quiet text
// inside a disabled control would stop dimming with it.
func TestTextRolesDisabledMarker(t *testing.T) {
	if !ThemeDark.TextStyleDisabled.disabledRole {
		t.Error("disabled role must set disabledRole")
	}
	for name, r := range map[string]TextStyle{
		"secondary":   ThemeDark.TextStyleSecondary,
		"placeholder": ThemeDark.TextStylePlaceholder,
		"label":       ThemeDark.TextStyleLabel,
		"default":     ThemeDark.TextStyleDef,
	} {
		if r.disabledRole {
			t.Errorf("%s must not set disabledRole", name)
		}
	}
}

// withRoleAlpha shares the amount of de-emphasis without taking the
// hue, which is what lets a caller keep its own text color.
func TestWithRoleAlphaKeepsHue(t *testing.T) {
	base := TextStyle{Color: RGBA(10, 20, 30, 255), Size: 14}
	got := withRoleAlpha(base, ThemeDark.TextStyleSecondary)

	if got.Color.R != 10 || got.Color.G != 20 || got.Color.B != 30 {
		t.Errorf("hue = %v, want the base's own", got.Color)
	}
	if got.Color.A != ThemeDark.TextStyleSecondary.Color.A {
		t.Errorf("alpha = %d, want the role's %d",
			got.Color.A, ThemeDark.TextStyleSecondary.Color.A)
	}
	if got.Size != 14 {
		t.Errorf("Size = %v, want the base's 14", got.Size)
	}
	if got.disabledRole {
		t.Error("secondary role must not mark the result disabled")
	}
	if !withRoleAlpha(base, ThemeDark.TextStyleDisabled).disabledRole {
		t.Error("disabled role must carry its marker through")
	}
}

// disabledTextColor is the render-time companion to withRoleAlpha: the
// renderer asks the theme for the disabled amount of a caller-supplied
// base color. It must keep the hue, take the role's alpha, and reflect
// an explicit ColorTextDisabled override — and the light theme must ask
// for more, not less, than the dark one.
func TestDisabledTextColor(t *testing.T) {
	base := RGBA(200, 50, 5, 255)
	for _, th := range []Theme{ThemeDark, ThemeLight} {
		got := th.disabledTextColor(base)
		if got.R != 200 || got.G != 50 || got.B != 5 {
			t.Errorf("%s: hue = %v, want the base's own", th.Name, got)
		}
		if got.A != th.TextStyleDisabled.Color.A {
			t.Errorf("%s: alpha = %d, want the role's %d",
				th.Name, got.A, th.TextStyleDisabled.Color.A)
		}
	}
	if ThemeLight.disabledTextColor(base).A <=
		ThemeDark.disabledTextColor(base).A {
		t.Error("light asks for no more alpha than dark; " +
			"the light-theme contrast gap of #341 is back")
	}

	cfg := baseCfg()
	cfg.ColorBackground = RGB(48, 48, 48)
	cfg.TextStyleDef = DefaultTextStyle
	want := RGBA(10, 20, 30, 200)
	cfg.ColorTextDisabled = want
	th := ThemeMaker(cfg)
	if got := th.disabledTextColor(base); got.A != 200 {
		t.Errorf("override: alpha = %d, want the configured 200", got.A)
	}
}

// Form density: the point of the field-inset tier is that controls in
// one row line up. Input, Select and Combobox each picked their own
// inset, which made a Select six pixels taller than the Input beside it
// (issue #335, audit section 7).
//
// Asserted on the *arranged* height, not on the configured padding.
// Equal insets were not enough: Input's inner row left SizeBorder unset
// and so inherited the theme's container border, reserving 3px of
// height for a border it never painted. Only running the real pipeline
// showed it.
func TestFieldControlsShareHeight(t *testing.T) {
	arrangedHeight := func(t *testing.T, build func() View) float32 {
		t.Helper()
		w := NewWindow(WindowCfg{State: new(int), Width: 320, Height: 240})
		w.viewGenerator = func(_ *Window) View {
			return Column(ContainerCfg{
				Sizing:  FillFill,
				Content: []View{build()},
			})
		}
		w.refreshLayout = true
		w.FrameFn()
		// Root -> filling wrapper -> the control.
		field := &w.layout.Children[0].Children[0]
		return field.Shape.Height
	}

	in := arrangedHeight(t, func() View {
		return Input(InputCfg{ID: "in", Text: "x"})
	})
	sel := arrangedHeight(t, func() View {
		return Select(SelectCfg{ID: "sel", Options: []string{"x"}})
	})
	cb := arrangedHeight(t, func() View {
		return Combobox(ComboboxCfg{ID: "cb", Options: []string{"x"}})
	})

	if in != sel || in != cb {
		t.Errorf("field heights differ: Input %v, Select %v, Combobox %v"+
			" -- a form row will not line up", in, sel, cb)
	}
}

// The theme's Input padding used to be dead: view_input.go fell back to
// a hardcoded inset instead of the style's, so editing the theme moved
// every other control but not Input.
func TestInputPaddingComesFromTheme(t *testing.T) {
	if !defaultInputStyle.Padding.IsSet() {
		t.Fatal("InputStyle.Padding unset; the theme is not seeding it")
	}
	got := Input(InputCfg{ID: "in"}).GenerateLayout(&Window{}).Shape.Padding
	if got != defaultInputStyle.Padding {
		t.Errorf("Input padding = %+v, want the theme's %+v",
			got, defaultInputStyle.Padding)
	}
}

// The field inset is one tier every text-bearing control shares, so a
// theme that rescales it moves all of them together.
func TestFieldInsetIsThemed(t *testing.T) {
	cfg := baseCfg()
	cfg.TextStyleDef = DefaultTextStyle
	cfg.PaddingField = PadAll(9)
	th := ThemeMaker(cfg)

	for name, got := range map[string]Padding{
		"input":    th.InputStyle.Padding,
		"select":   th.selectStyle.Padding,
		"combobox": th.comboboxStyle.Padding,
		"theme":    th.PaddingField,
	} {
		if got != PadAll(9) {
			t.Errorf("%s padding = %+v, want the configured %+v",
				name, got, PadAll(9))
		}
	}
}

// An unset Label must produce no wrapper and no extra shape, or adding
// the field to eight Cfgs would be a visual break rather than an
// addition (issue #335, audit section 3).
func TestFieldLabelEmptyIsInert(t *testing.T) {
	field := Input(InputCfg{ID: "in"})
	if got := labelledField("", TextStyle{}, HAlignLeft, FillFit, field); got != field {
		t.Error("empty label must return the field untouched")
	}
}

// Label fills A11YLabel when the caller left it unset, and never
// overrides one the caller supplied.
func TestFieldLabelFeedsA11Y(t *testing.T) {
	w := &Window{}
	derived := Select(SelectCfg{ID: "s", Label: "Variant"}).
		GenerateLayout(w)
	// The wrapper is the label stack; the control is its second child.
	got := derived.Children[1].Shape.a11Y.Label
	if got != "Variant" {
		t.Errorf("A11YLabel = %q, want it derived from Label", got)
	}

	explicit := Select(SelectCfg{
		ID:      "s",
		Label:   "Variant",
		A11YCfg: A11YCfg{A11YLabel: "Chosen variant"},
	}).GenerateLayout(w)
	if got = explicit.Children[1].Shape.a11Y.Label; got != "Chosen variant" {
		t.Errorf("A11YLabel = %q, want the caller's own", got)
	}
}

// The inspector's help text is ordinary supporting text, so it must
// follow the secondary role rather than the fixed literal
// defaultInspectorStyle carries.
func TestInspectorHelpFromSecondaryRole(t *testing.T) {
	th := ThemeMaker(baseCfg())
	s := inspectorStyleFor(th.TextStyleSecondary)
	if !s.colorTextHelp.eq(th.TextStyleSecondary.Color) {
		t.Errorf("help color = %v, want secondary %v",
			s.colorTextHelp, th.TextStyleSecondary.Color)
	}
}
