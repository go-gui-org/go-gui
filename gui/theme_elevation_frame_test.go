package gui

import "testing"

// End-to-end checks that the elevation and focus-ring tokens survive
// the real frame pipeline, not just the style structs.
//
// The mechanism they guard is easy to get wrong in a way unit tests
// miss: a popover floats, so its shadow is emitted while some ancestor
// clip is in force, and its blur deliberately extends past the shape's
// own bounds.

// frameCmds runs one real frame under theme and returns the emitted
// commands. Mirrors renderGolden's setup; kept separate because these
// cases assert on command content rather than recording a fingerprint.
func frameCmds(t *testing.T, theme Theme, build func(*Window) View,
	focusID string, setup func(*Window),
) []RenderCmd {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 300})
	w.viewGenerator = func(win *Window) View {
		if setup != nil {
			setup(win)
		}
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{build(win)},
		})
	}
	w.SetTheme(theme)
	if focusID != "" {
		w.SetFocus(focusID)
	}
	w.refreshLayout = true
	w.FrameFn()
	if len(w.renderers) == 0 {
		t.Fatal("pipeline emitted no render commands")
	}
	return w.renderers
}

func countShadows(cmds []RenderCmd) int {
	n := 0
	for _, c := range cmds {
		if c.Kind == RenderShadow {
			n++
		}
	}
	return n
}

// An open Select dropdown under a theme with elevation must emit a
// shadow; under a theme without one it must emit none. The pair is the
// point — the second half is what proves the first is the theme's
// doing and not something the widget now always does. macOS and dark
// both carry elevation since visual-refresh §5.3; a bare baseCfg()
// theme is the flat subject (every registered preset carries elevation
// since the blue taste preset was removed).
func TestOpenDropdownEmitsShadowOnlyWhenThemed(t *testing.T) {
	build := func(*Window) View {
		return Select(SelectCfg{
			ID:      "picker",
			Options: []string{"one", "two", "three"},
		})
	}
	open := func(w *Window) {
		StateMap[string, bool](w, nsSelect, capModerate).Set("picker", true)
	}

	for name, theme := range map[string]Theme{
		"macos": themeMacOS,
		"dark":  ThemeDark,
	} {
		cmds := frameCmds(t, theme, build, "", open)
		if countShadows(cmds) == 0 {
			t.Errorf("%s theme: open dropdown emitted no shadow", name)
		}
	}

	flat := frameCmds(t, ThemeMaker(baseCfg()), build, "", open)
	if got := countShadows(flat); got != 0 {
		t.Errorf("flat theme: got %d shadows, want 0", got)
	}
}

// The dropdown's shadow must not be clipped away by the ancestor whose
// scissor is in force when the float renders, and must be emitted at
// the popover's own geometry rather than collapsed to zero.
func TestDropdownShadowSurvivesAncestorClip(t *testing.T) {
	cmds := frameCmds(t, themeMacOS, func(*Window) View {
		// A clipping, scrollable ancestor is the case that would
		// scissor a naive implementation's shadow away.
		return Column(ContainerCfg{
			ID:         "panel",
			Sizing:     FillFill,
			Clip:       true,
			Scrollable: true,
			Content: []View{
				Select(SelectCfg{
					ID:      "picker",
					Options: []string{"one", "two", "three"},
				}),
			},
		})
	}, "", func(w *Window) {
		StateMap[string, bool](w, nsSelect, capModerate).
			Set("panel:picker", true)
	})

	var shadow *RenderCmd
	for i := range cmds {
		if cmds[i].Kind == RenderShadow {
			shadow = &cmds[i]
			break
		}
	}
	if shadow == nil {
		t.Fatal("no shadow emitted for a dropdown inside a clipping panel")
	}
	if shadow.W <= 0 || shadow.H <= 0 {
		t.Errorf("shadow collapsed: got %vx%v", shadow.W, shadow.H)
	}
	if shadow.Color.A == 0 {
		t.Error("shadow emitted fully transparent")
	}
}

// An input's ring rides inputAmendLayout, a different wiring than the
// button's, so it needs its own guard: a focused input under the macOS
// theme must glow, the same input under the ring-bearing dark preset
// must ring, and under a ringless theme must do neither.
func TestFocusedInputEmitsRingOnlyWhenThemed(t *testing.T) {
	build := func(*Window) View {
		return Input(InputCfg{ID: "field"})
	}

	if got := countShadows(
		frameCmds(t, themeMacOS, build, "field", nil)); got == 0 {
		t.Error("macOS theme: focused input emitted no focus ring")
	}
	if got := countShadows(
		frameCmds(t, themeMacOS, build, "", nil)); got != 0 {
		t.Errorf("macOS theme: unfocused input emitted %d shadows", got)
	}
	if got := countShadows(
		frameCmds(t, ThemeDark, build, "field", nil)); got == 0 {
		t.Error("dark theme: focused input emitted no focus ring")
	}
	if got := countShadows(
		frameCmds(t, ThemeDark, build, "", nil)); got != 0 {
		t.Errorf("dark theme: unfocused input emitted %d shadows", got)
	}
	if got := countShadows(
		frameCmds(t, themeWindows, build, "field", nil)); got != 0 {
		t.Errorf("ringless windows theme: focused input emitted %d shadows", got)
	}
}

// A focused Button under a theme with a ring emits one; the same
// button unfocused, and any button under a ringless theme, emits none.
func TestFocusedButtonEmitsRingOnlyWhenThemed(t *testing.T) {
	build := func(*Window) View {
		return Button(ButtonCfg{
			ID:      "go",
			Content: []View{Text(TextCfg{Text: "Go"})},
			OnClick: func(EventCtx) {},
		})
	}

	if got := countShadows(
		frameCmds(t, themeMacOS, build, "go", nil)); got == 0 {
		t.Error("macOS theme: focused button emitted no focus ring")
	}
	if got := countShadows(
		frameCmds(t, themeMacOS, build, "", nil)); got != 0 {
		t.Errorf("macOS theme: unfocused button emitted %d shadows", got)
	}
	if got := countShadows(
		frameCmds(t, ThemeDark, build, "go", nil)); got == 0 {
		t.Error("dark theme: focused button emitted no focus ring")
	}
	if got := countShadows(
		frameCmds(t, ThemeDark, build, "", nil)); got != 0 {
		t.Errorf("dark theme: unfocused button emitted %d shadows", got)
	}
	if got := countShadows(
		frameCmds(t, themeWindows, build, "go", nil)); got != 0 {
		t.Errorf("ringless windows theme: focused button emitted %d shadows", got)
	}
}
