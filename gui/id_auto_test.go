package gui

import (
	"strings"
	"testing"
)

// Auto widget identity (#881, docs/specs/auto-widget-identity.md).
//
// A widget with an empty ID gets a generated leaf: the nearest scope,
// the widget kind, and the count of earlier widgets of that kind in
// the scope. These tests pin the properties the spec promises.

// autoFocusables returns the focusable shapes of one frame in DFS
// order, which is also the tab order.
func autoFocusables(layout *Layout) []*Shape {
	var out []*Shape
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if s := l.Shape; s != nil && s.ID != "" && s.canTakeFocus() {
			out = append(out, s)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(layout)
	return out
}

// autoFocusKeyByLabel returns the identity of the focusable shape
// whose a11y label is label, or "" when no shape has it.
func autoFocusKeyByLabel(layout *Layout, label string) string {
	for _, s := range autoFocusables(layout) {
		if s.a11Y != nil && s.a11Y.Label == label {
			return s.idKey()
		}
	}
	return ""
}

func labelledInput(label string) View {
	return Input(InputCfg{A11YCfg: A11YCfg{A11YLabel: label}})
}

func TestAutoIDGivesIDLessInputsDistinctStableKeys(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	view := func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{labelledInput("a"), labelledInput("b")},
		})
	}
	first := autoFocusables(w.TestRender(view))
	if len(first) != 2 {
		t.Fatalf("got %d focusable inputs, want 2", len(first))
	}
	a, b := first[0].idKey(), first[1].idKey()
	if a == "" || b == "" || a == b {
		t.Fatalf("auto keys %q and %q must be distinct and non-empty", a, b)
	}
	second := autoFocusables(w.TestRender(view))
	if second[0].idKey() != a || second[1].idKey() != b {
		t.Fatalf("keys moved between frames: %q,%q then %q,%q",
			a, b, second[0].idKey(), second[1].idKey())
	}
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("auto IDs collided: %v", dups)
	}
}

// A Text that appears above a form is a different kind, so it must not
// shift the keys of the inputs — the focus stays on the same field.
func TestAutoIDOtherKindInsertKeepsFocus(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	showError := false
	view := func(_ *Window) View {
		content := []View{}
		if showError {
			content = append(content, Text(TextCfg{Text: "bad input"}))
		}
		content = append(content, labelledInput("a"), labelledInput("b"))
		return Column(ContainerCfg{Sizing: FillFill, Content: content})
	}
	root := w.TestRender(view)
	keyB := autoFocusKeyByLabel(root, "b")
	if keyB == "" {
		t.Fatal("input b has no auto key")
	}
	w.SetFocus(keyB)
	showError = true
	root = w.TestRender(view)
	if got := autoFocusKeyByLabel(root, "b"); got != keyB {
		t.Fatalf("input b moved from %q to %q after a Text insert", keyB, got)
	}
	if w.FocusID() != keyB {
		t.Fatalf("focus moved to %q, want %q", w.FocusID(), keyB)
	}
}

// An explicit ID on a container is a firewall: an input inserted
// outside it does not shift the keys inside it.
func TestAutoIDExplicitContainerIsAFirewall(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	extra := false
	view := func(_ *Window) View {
		content := []View{}
		if extra {
			content = append(content, labelledInput("extra"))
		}
		content = append(content, Column(ContainerCfg{
			ID:      "form",
			Content: []View{labelledInput("name")},
		}))
		return Column(ContainerCfg{Sizing: FillFill, Content: content})
	}
	before := autoFocusKeyByLabel(w.TestRender(view), "name")
	if !strings.HasPrefix(before, "form"+IDSep) {
		t.Fatalf("key %q is not scoped under the form", before)
	}
	extra = true
	after := autoFocusKeyByLabel(w.TestRender(view), "name")
	if after != before {
		t.Fatalf("key moved from %q to %q across an explicit scope",
			before, after)
	}
}

// An explicit ID always wins over the auto leaf.
func TestAutoIDExplicitIDWins(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Input(InputCfg{
				ID: "name", A11YCfg: A11YCfg{A11YLabel: "name"},
			})},
		})
	})
	if got := autoFocusKeyByLabel(root, "name"); got != "name" {
		t.Fatalf("explicit ID resolved to %q, want %q", got, "name")
	}
}

// An auto-ID container does not open a scope. An explicit ID inside it
// keeps the effective ID it has without the container, so app code
// can still address it by name.
func TestAutoIDContainerIsTransparentForScope(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Column(ContainerCfg{
				Scrollable: true,
				Height:     100,
				Sizing:     FillFixed,
				Content: []View{Input(InputCfg{
					ID: "name", A11YCfg: A11YCfg{A11YLabel: "name"},
				})},
			})},
		})
	})
	if got := autoFocusKeyByLabel(root, "name"); got != "name" {
		t.Fatalf("explicit child of an auto container resolved to %q, "+
			"want %q", got, "name")
	}
}

// Tab reaches auto-ID inputs in tree order.
func TestAutoIDInputsJoinTabOrder(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	root := w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				labelledInput("a"), labelledInput("b"), labelledInput("c"),
			},
		})
	})
	keys := []string{
		autoFocusKeyByLabel(root, "a"),
		autoFocusKeyByLabel(root, "b"),
		autoFocusKeyByLabel(root, "c"),
	}
	w.SetFocus(keys[0])
	for _, want := range keys[1:] {
		w.EventFn(&Event{Type: EventKeyDown, KeyCode: KeyTab})
		if got := w.FocusID(); got != want {
			t.Fatalf("tab: got %q, want %q", got, want)
		}
	}
}

// A same-kind insert above the focused widget moves its key to another
// widget. The detector reports it.
func TestAutoIDSameKindShiftIsReported(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	extra := false
	view := func(_ *Window) View {
		content := []View{}
		if extra {
			content = append(content, labelledInput("extra"))
		}
		content = append(content, labelledInput("a"))
		return Column(ContainerCfg{Sizing: FillFill, Content: content})
	}
	w.TestRender(view)
	w.SetFocus(autoFocusKeyByLabel(&w.layout, "a"))
	if got := w.TestFindings(DebugAutoIDs); len(got) != 0 {
		t.Fatalf("stable frame reported %v", got)
	}
	extra = true
	w.TestRender(view)
	got := w.TestFindings(DebugAutoIDs)
	if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), "shifted") {
		t.Fatalf("same-kind shift not reported, findings %v", got)
	}
}

// Typing changes the value, not the identity: the detector must stay
// quiet while the focused input's text changes.
func TestAutoIDValueChangeIsNotAShift(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	text := "x"
	view := func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Input(InputCfg{
				Text: text, A11YCfg: A11YCfg{A11YLabel: "a"},
			})},
		})
	}
	w.TestRender(view)
	w.SetFocus(autoFocusKeyByLabel(&w.layout, "a"))
	w.TestRender(view)
	text = "xy"
	w.TestRender(view)
	if got := w.TestFindings(DebugAutoIDs); len(got) != 0 {
		t.Fatalf("value change reported as a shift: %v", got)
	}
}

// The ~ prefix is reserved for generated leaves.
func TestAutoIDReservedPrefixIsReported(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{Input(InputCfg{ID: "~mine"})},
		})
	})
	got := w.TestFindings(DebugAutoIDs)
	if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), "reserved") {
		t.Fatalf("reserved prefix not reported, findings %v", got)
	}
}

// After the first frame, a generated leaf costs lookups only.
func TestAutoLeafSteadyStateDoesNotAllocate(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.viewState.idScope = "panel"
	run := func() {
		w.resetAutoIDs()
		for range 8 {
			_ = w.autoLeaf("input")
			_ = w.autoLeaf("slider")
		}
	}
	run()
	if n := testing.AllocsPerRun(50, run); n != 0 {
		t.Fatalf("autoLeaf allocates %.1f times per frame, want 0", n)
	}
}

// Every widget that took a required ID before #881 must build without
// one, and two ID-less copies side by side must not collide. A
// composite that names an inner shape with a relative leaf, expecting
// its own ID to scope it, fails here: an auto ID opens no scope.
func TestAutoIDEveryWidgetTwiceHasNoDuplicates(t *testing.T) {
	widgets := map[string]func(w *Window) View{
		"Button":             func(_ *Window) View { return Button(ButtonCfg{}) },
		"ColorChannelSlider": func(_ *Window) View { return ColorChannelSlider(ColorChannelSliderCfg{}) },
		"ColorFields":        func(_ *Window) View { return ColorFields(ColorFieldsCfg{}) },
		"ColorPicker":        func(_ *Window) View { return ColorPicker(ColorPickerCfg{}) },
		"ColorWheel":         func(_ *Window) View { return ColorWheel(ColorWheelCfg{}) },
		"ColorPlane":         func(_ *Window) View { return ColorPlane(ColorPlaneCfg{}) },
		"Combobox":           func(_ *Window) View { return Combobox(ComboboxCfg{}) },
		"CommandPalette":     func(_ *Window) View { return CommandPalette(CommandPaletteCfg{}) },
		"ContextMenu":        func(w *Window) View { return ContextMenu(w, ContextMenuCfg{}) },
		"DatePickerRoller":   func(_ *Window) View { return DatePickerRoller(DatePickerRollerCfg{}) },
		"DatePicker":         func(_ *Window) View { return DatePicker(DatePickerCfg{}) },
		"Form":               func(_ *Window) View { return Form(FormCfg{}) },
		"InputDate":          func(_ *Window) View { return InputDate(InputDateCfg{}) },
		"NumericInput":       func(_ *Window) View { return NumericInput(NumericInputCfg{}) },
		"Input":              func(_ *Window) View { return Input(InputCfg{}) },
		"ListBox":            func(_ *Window) View { return ListBox(ListBoxCfg{}) },
		"Menubar":            func(w *Window) View { return Menubar(w, MenubarCfg{}) },
		"OverflowPanel":      func(w *Window) View { return OverflowPanel(w, OverflowPanelCfg{}) },
		"ProgressBar":        func(_ *Window) View { return ProgressBar(ProgressBarCfg{}) },
		"RadioButtonGroup":   func(_ *Window) View { return RadioButtonGroupRow(RadioButtonGroupCfg{}) },
		"Radio":              func(_ *Window) View { return Radio(RadioCfg{}) },
		"Select":             func(_ *Window) View { return Select(SelectCfg{}) },
		"SegmentedControl":   func(_ *Window) View { return SegmentedControl(SegmentedControlCfg{}) },
		"Slider":             func(_ *Window) View { return Slider(SliderCfg{}) },
		"Switch":             func(_ *Window) View { return Switch(SwitchCfg{}) },
		"Table":              func(_ *Window) View { return Table(TableCfg{}) },
		"ThinkingOrb":        func(_ *Window) View { return ThinkingOrb(ThinkingOrbCfg{}) },
		"ThinkingOrbLabel":   func(_ *Window) View { return ThinkingOrbLabel(ThinkingOrbLabelCfg{}) },
		"Toggle":             func(_ *Window) View { return Toggle(ToggleCfg{}) },
		"Tree":               func(_ *Window) View { return Tree(TreeCfg{}) },
		"VirtualList":        func(_ *Window) View { return VirtualList(VirtualListCfg{}) },
		"ScrollContainer": func(_ *Window) View {
			return Column(ContainerCfg{Scrollable: true, Height: 50, Sizing: FillFixed})
		},
	}
	assertAutoIDsNoDuplicates(t, widgets)
}

// The same sweep with focus turned off. Before, the ID was required
// only for focus, so these calls took no ID and no auto leaf, and a
// composite that scopes its inner IDs by its own ID gave every copy
// the same bare inner IDs ("input", "calendar", "head").
func TestAutoIDFocusDisabledTwiceHasNoDuplicates(t *testing.T) {
	widgets := map[string]func(w *Window) View{
		"Button":             func(_ *Window) View { return Button(ButtonCfg{FocusDisabled: true}) },
		"ColorChannelSlider": func(_ *Window) View { return ColorChannelSlider(ColorChannelSliderCfg{FocusDisabled: true}) },
		"ColorPicker":        func(_ *Window) View { return ColorPicker(ColorPickerCfg{FocusDisabled: true}) },
		"ColorPlane":         func(_ *Window) View { return ColorPlane(ColorPlaneCfg{FocusDisabled: true}) },
		"ColorWheel":         func(_ *Window) View { return ColorWheel(ColorWheelCfg{FocusDisabled: true}) },
		"Combobox":           func(_ *Window) View { return Combobox(ComboboxCfg{FocusDisabled: true}) },
		"DatePicker":         func(_ *Window) View { return DatePicker(DatePickerCfg{FocusDisabled: true}) },
		"ExpandPanel":        func(_ *Window) View { return ExpandPanel(ExpandPanelCfg{FocusDisabled: true}) },
		"InputDate":          func(_ *Window) View { return InputDate(InputDateCfg{FocusDisabled: true}) },
		"Input":              func(_ *Window) View { return Input(InputCfg{FocusDisabled: true}) },
		"ListBox":            func(_ *Window) View { return ListBox(ListBoxCfg{FocusDisabled: true}) },
		"NumericInput":       func(_ *Window) View { return NumericInput(NumericInputCfg{FocusDisabled: true}) },
		"RadioButtonGroup":   func(_ *Window) View { return RadioButtonGroupRow(RadioButtonGroupCfg{FocusDisabled: true}) },
		"Radio":              func(_ *Window) View { return Radio(RadioCfg{FocusDisabled: true}) },
		"SegmentedControl":   func(_ *Window) View { return SegmentedControl(SegmentedControlCfg{FocusDisabled: true}) },
		"Select":             func(_ *Window) View { return Select(SelectCfg{FocusDisabled: true}) },
		"Slider":             func(_ *Window) View { return Slider(SliderCfg{FocusDisabled: true}) },
		"Switch":             func(_ *Window) View { return Switch(SwitchCfg{FocusDisabled: true}) },
		"Toggle":             func(_ *Window) View { return Toggle(ToggleCfg{FocusDisabled: true}) },
		"Tree":               func(_ *Window) View { return Tree(TreeCfg{FocusDisabled: true}) },
		"VirtualList":        func(_ *Window) View { return VirtualList(VirtualListCfg{FocusDisabled: true}) },
	}
	assertAutoIDsNoDuplicates(t, widgets)
}

// ExpandPanel took no required ID before #881, so the first sweep
// left it out. With focus on, its header is a tab stop.
func TestAutoIDExpandPanelTwiceHasNoDuplicates(t *testing.T) {
	assertAutoIDsNoDuplicates(t, map[string]func(w *Window) View{
		"ExpandPanel": func(_ *Window) View { return ExpandPanel(ExpandPanelCfg{}) },
	})
}

// A radio group builds its option IDs from its own ID. Two ID-less
// groups with options in two panels must not share "opt:0": the inner
// IDs compose from the resolved ID, which carries the panel scope.
func TestAutoIDRadioGroupOptionsInTwoPanelsNoDuplicates(t *testing.T) {
	opts := []RadioOption{
		{Label: "a", Value: "a"}, {Label: "b", Value: "b"},
	}
	build := func(_ *Window) View {
		return RadioButtonGroupRow(RadioButtonGroupCfg{Options: opts})
	}
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{ID: "p1", Content: []View{build(w)}}),
				Column(ContainerCfg{ID: "p2", Content: []View{build(w)}}),
			},
		})
	})
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("ID-less radio groups in two panels collide: %v", dups)
	}
}

// A FocusDisabled segmented control still builds segment buttons with
// IDs scoped by its own. Two ID-less copies must not share "opt:0".
func TestAutoIDSegmentedControlOptionsFocusDisabledNoDuplicates(t *testing.T) {
	opts := []SegmentOption{
		{Label: "a", Value: "a"}, {Label: "b", Value: "b"},
	}
	build := func(_ *Window) View {
		return SegmentedControl(SegmentedControlCfg{
			FocusDisabled: true, Options: opts,
		})
	}
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{build(w), build(w)},
		})
	})
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("two ID-less FocusDisabled segmented controls collide: %v", dups)
	}
}

// An explicit head ID built from the leaf ignores the enclosing scope.
// Two panels with the same ExpandPanel ID must scope their headers.
func TestAutoIDExpandPanelExplicitSameIDInTwoPanelsNoDuplicates(t *testing.T) {
	build := func(_ *Window) View {
		return ExpandPanel(ExpandPanelCfg{ID: "ep"})
	}
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{ID: "p1", Content: []View{build(w)}}),
				Column(ContainerCfg{ID: "p2", Content: []View{build(w)}}),
			},
		})
	})
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("explicit expand panels in two panels collide: %v", dups)
	}
}

// Two ID-less selects, both open, must not share one "dropdown": the
// dropdown shape carries the absolute scroll key, not a relative leaf.
func TestAutoIDSelectOpenTwiceHasNoDuplicates(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	view := func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Select(SelectCfg{Options: []SelectOption{{Label: "a", Value: "a"}}}),
				Select(SelectCfg{Options: []SelectOption{{Label: "b", Value: "b"}}}),
			},
		})
	}
	root := w.TestRender(view)
	keys := []string{}
	for _, s := range autoFocusables(root) {
		keys = append(keys, s.idKey())
	}
	if len(keys) != 2 {
		t.Fatalf("got %d focusable selects, want 2", len(keys))
	}
	ss := StateMap[string, bool](w, nsSelect, capModerate)
	for _, k := range keys {
		ss.Set(k, true)
	}
	w.TestRender(view)
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("two open ID-less selects collide: %v", dups)
	}
}

// Two ID-less comboboxes, both open, must not share one "dropdown".
func TestAutoIDComboboxOpenTwiceHasNoDuplicates(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	view := func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Combobox(ComboboxCfg{Items: []string{"a"}}),
				Combobox(ComboboxCfg{Items: []string{"b"}}),
			},
		})
	}
	root := w.TestRender(view)
	keys := []string{}
	for _, s := range autoFocusables(root) {
		keys = append(keys, s.idKey())
	}
	if len(keys) != 2 {
		t.Fatalf("got %d focusable comboboxes, want 2", len(keys))
	}
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	for _, k := range keys {
		ss.Set(k, true)
	}
	w.TestRender(view)
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("two open ID-less comboboxes collide: %v", dups)
	}
}

// assertAutoIDsNoDuplicates builds each widget twice in one scope,
// then once in each of two panels, and fails on a duplicate ID.
func assertAutoIDsNoDuplicates(t *testing.T, widgets map[string]func(w *Window) View) {
	t.Helper()
	for name, build := range widgets {
		t.Run(name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			w.TestRender(func(w *Window) View {
				return Column(ContainerCfg{
					Sizing:  FillFill,
					Content: []View{build(w), build(w)},
				})
			})
			if dups := w.TestDuplicateIDs(); len(dups) != 0 {
				t.Fatalf("two ID-less %s collide: %v", name, dups)
			}
			// The same leaf in two panels: both get "~<kind>0", so only
			// the panel scope keeps them apart. A composite that builds
			// inner IDs from the unresolved leaf makes absolute IDs that
			// skip the scope and collide here. A fresh window, so state
			// the first render keyed at the top scope is not stale here.
			w = NewTestWindow(t, WindowCfg{})
			w.TestRender(func(w *Window) View {
				return Column(ContainerCfg{
					Sizing: FillFill,
					Content: []View{
						Column(ContainerCfg{ID: "p1", Content: []View{build(w)}}),
						Column(ContainerCfg{ID: "p2", Content: []View{build(w)}}),
					},
				})
			})
			if dups := w.TestDuplicateIDs(); len(dups) != 0 {
				t.Fatalf("ID-less %s in two panels collide: %v", name, dups)
			}
		})
	}
}
