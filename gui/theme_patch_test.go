package gui

import (
	"math"
	"strings"
	"testing"
)

// Per-widget theme overrides (issue #754). A patch changes one widget
// class, keeps a fresh theme id, and survives every rebuild path.

func TestThemeWithButtonRadius(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	patched := base.With(ButtonPatch{Radius: SomeF(2)})

	if patched.id == base.id {
		t.Error("With reused the parent theme id")
	}
	for name, got := range map[string]float32{
		"buttonStyle":        patched.buttonStyle.Radius,
		"buttonStylePrimary": patched.buttonStylePrimary.Radius,
		"buttonStyleGhost":   patched.buttonStyleGhost.Radius,
		"buttonStyleDanger":  patched.buttonStyleDanger.Radius,
	} {
		if got != 2 {
			t.Errorf("%s.Radius = %v, want 2", name, got)
		}
	}
}

func TestThemeWithIsolation(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	patched := base.With(ButtonPatch{Radius: SomeF(2)})

	if patched.inputStyle != base.inputStyle {
		t.Error("ButtonPatch moved inputStyle")
	}
	if patched.selectStyle != base.selectStyle {
		t.Error("ButtonPatch moved selectStyle")
	}
	if patched.dialogStyle != base.dialogStyle {
		t.Error("ButtonPatch moved dialogStyle")
	}
	if patched.containerStyle != base.containerStyle {
		t.Error("ButtonPatch moved containerStyle")
	}
	if patched.inputStyle.Radius != base.inputStyle.Radius {
		t.Errorf("input radius = %v, want %v",
			patched.inputStyle.Radius, base.inputStyle.Radius)
	}
}

func TestThemeWithZeroPatchKeepsStyles(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	patched := base.With(ButtonPatch{})

	if patched.buttonStyle != base.buttonStyle {
		t.Error("zero ButtonPatch moved buttonStyle")
	}
	if patched.id == base.id {
		t.Error("With reused the parent theme id")
	}
}

func TestThemeWithPanicsOnUnknown(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("With did not panic on unknown patch type")
		}
	}()
	ThemeMaker(themeDarkCfg).With(themeExtSample{Tag: "x"})
}

// The panic names the wrong type, so the message points at the
// misspelled override. A nil patch panics too, not a nil deref.
func TestThemeWithPanicNamesType(t *testing.T) {
	for _, tc := range []struct {
		patch any
		want  string
	}{
		{themeExtSample{Tag: "x"}, "gui.themeExtSample"},
		{&ButtonPatch{}, "*gui.ButtonPatch"},
		{nil, "<nil>"},
	} {
		func() {
			defer func() {
				msg, _ := recover().(string)
				if !strings.Contains(msg, tc.want) {
					t.Errorf("panic = %q, want it to name %s", msg, tc.want)
				}
			}()
			ThemeMaker(themeDarkCfg).With(tc.patch)
		}()
	}
}

// A NaN, infinite or negative length becomes 0, both in the applied
// style and in the stored patch that rebuild paths re-apply.
func TestThemeWithSanitizesLengths(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	patched := ThemeMaker(themeDarkCfg).With(InputPatch{
		Padding:    NewPadding(nan, -4, inf, 5),
		SizeBorder: SomeF(-1),
		Radius:     SomeF(nan),
	})
	want := NewPadding(0, 0, 0, 5)
	for name, theme := range map[string]Theme{
		"With":        patched,
		"WithBorders": patched.WithBorders(true),
	} {
		st := theme.inputStyle
		if st.Padding != want {
			t.Errorf("%s: padding = %v, want %v", name, st.Padding, want)
		}
		if st.SizeBorder != 0 {
			t.Errorf("%s: border = %v, want 0", name, st.SizeBorder)
		}
		if st.Radius != 0 {
			t.Errorf("%s: radius = %v, want 0", name, st.Radius)
		}
	}
	stored, ok := Ext[InputPatch](patched)
	if !ok {
		t.Fatal("InputPatch not stored")
	}
	if r, _ := stored.Radius.Value(); r != 0 {
		t.Errorf("stored radius = %v, want 0", r)
	}
}

func TestThemeWithOverwriteSameType(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	first := base.With(ButtonPatch{Radius: SomeF(2)})
	second := first.With(ButtonPatch{Radius: SomeF(5)})

	if second.buttonStyle.Radius != 5 {
		t.Errorf("second radius = %v, want 5", second.buttonStyle.Radius)
	}
	if first.buttonStyle.Radius != 2 {
		t.Errorf("first radius = %v, want 2 (clone on write)", first.buttonStyle.Radius)
	}
}

func TestThemeWithCoexistingTypes(t *testing.T) {
	patched := ThemeMaker(themeDarkCfg).With(ButtonPatch{Radius: SomeF(2)}).
		With(InputPatch{Radius: SomeF(7)})

	if patched.buttonStyle.Radius != 2 {
		t.Errorf("button radius = %v, want 2", patched.buttonStyle.Radius)
	}
	if patched.inputStyle.Radius != 7 {
		t.Errorf("input radius = %v, want 7", patched.inputStyle.Radius)
	}
}

func TestThemeWithSurvivesRebuilds(t *testing.T) {
	base := ThemeMaker(themeDarkCfg).With(ButtonPatch{Radius: SomeF(2)})

	sized, err := base.AdjustFontSize(1, 1, 100)
	if err != nil {
		t.Fatalf("AdjustFontSize = %v", err)
	}
	rebuilt := []struct {
		name  string
		theme Theme
	}{
		{"WithColors", base.WithColors(ColorOverrides{})},
		{"WithPadding", base.WithPadding(false)},
		{"WithBorders", base.WithBorders(false)},
		{"AdjustFontSize", sized},
	}

	for _, r := range rebuilt {
		if r.theme.buttonStyle.Radius != 2 {
			t.Errorf("%s button radius = %v, want 2",
				r.name, r.theme.buttonStyle.Radius)
		}
		if r.theme.id == base.id {
			t.Errorf("%s reused the parent theme id", r.name)
		}
	}
}

func TestThemeWithSurvivesPaddingRestore(t *testing.T) {
	base := ThemeMaker(themeDarkCfg).With(ButtonPatch{Radius: SomeF(2)})
	restored := base.WithPadding(false).WithPadding(true)

	if restored.buttonStyle.Radius != 2 {
		t.Errorf("restored button radius = %v, want 2",
			restored.buttonStyle.Radius)
	}
}

func TestThemeWithContainerPatch(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)
	patched := base.With(ContainerPatch{
		Padding:    PadAll(3),
		SizeBorder: NoBorder,
		Radius:     SomeF(9),
	})

	if patched.containerStyle.Padding != PadAll(3) {
		t.Errorf("container padding = %v, want %v",
			patched.containerStyle.Padding, PadAll(3))
	}
	if patched.containerStyle.SizeBorder != 0 {
		t.Errorf("container border = %v, want 0", patched.containerStyle.SizeBorder)
	}
	if patched.containerStyle.Radius != 9 {
		t.Errorf("container radius = %v, want 9", patched.containerStyle.Radius)
	}
	if patched.buttonStyle != base.buttonStyle {
		t.Error("ContainerPatch moved buttonStyle")
	}
}

func TestThemeWithOtherPatches(t *testing.T) {
	base := ThemeMaker(themeDarkCfg)

	patchedInput := base.With(InputPatch{
		Padding:    PadAll(3),
		SizeBorder: NoBorder,
		Radius:     SomeF(7),
	})
	if patchedInput.inputStyle.Padding != PadAll(3) {
		t.Errorf("input padding = %v, want %v",
			patchedInput.inputStyle.Padding, PadAll(3))
	}
	if patchedInput.inputStyle.SizeBorder != 0 {
		t.Errorf("input border = %v, want 0", patchedInput.inputStyle.SizeBorder)
	}
	if patchedInput.inputStyle.Radius != 7 {
		t.Errorf("input radius = %v, want 7", patchedInput.inputStyle.Radius)
	}
	if patchedInput.buttonStyle != base.buttonStyle {
		t.Error("InputPatch moved buttonStyle")
	}

	patchedSelect := base.With(SelectPatch{Radius: SomeF(4)})
	if patchedSelect.selectStyle.Radius != 4 {
		t.Errorf("select radius = %v, want 4", patchedSelect.selectStyle.Radius)
	}
	if patchedSelect.inputStyle != base.inputStyle {
		t.Error("SelectPatch moved inputStyle")
	}

	patchedDialog := base.With(DialogPatch{Radius: SomeF(6)})
	if patchedDialog.dialogStyle.Radius != 6 {
		t.Errorf("dialog radius = %v, want 6", patchedDialog.dialogStyle.Radius)
	}
	if patchedDialog.containerStyle != base.containerStyle {
		t.Error("DialogPatch moved containerStyle")
	}
}

// TestGoldenWidgetPatch records a patched button in both themes. The
// radius override must reach the rendered commands under dark and
// light alike.
func TestGoldenWidgetPatch(t *testing.T) {
	// renderGolden installs each theme globally through the frame
	// pipeline, so restore the theme afterwards: later tests read
	// the installed mirrors against ThemeDark.
	restoreTheme(t)
	build := func(*Window) View {
		return Button(ButtonCfg{
			ID:      "btn",
			Content: []View{Text(TextCfg{Text: "Save"})},
			OnClick: func(EventCtx) {},
		})
	}
	c := goldenCase{name: "button_patch", build: build}
	for _, th := range goldenThemes() {
		patched := th.theme.With(ButtonPatch{Radius: SomeF(2)})
		name := c.name + "." + th.name
		t.Run(name, func(t *testing.T) {
			got := renderGolden(t, patched, c)
			checkGolden(t, name, got)
		})
	}
}
