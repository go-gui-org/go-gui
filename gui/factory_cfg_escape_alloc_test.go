package gui

import "testing"

// Factories with an auto-ID branch build a ViewFunc closure that sets
// cfg.ID. If that closure captures the cfg parameter itself, Go moves
// the parameter to the heap when the call starts. Every call then pays
// one Cfg-sized allocation, also when ID is set and the closure is
// never built. Cfg structs are larger than the 128 bytes Go captures by
// value, so the capture is always by reference.
//
// The branch must copy cfg into a local of its own and capture that.
// Then the allocation happens only when the branch runs. These tests
// call each factory with an explicit ID, so the branch does not run,
// and pin the allocations that are left. Each budget is the count with
// the escape fixed. A factory over its budget has a closure that
// captures a cfg again, its own or an inner widget's.
func TestFactoryCfgDoesNotEscape(t *testing.T) {
	// ContextMenu and Menubar take a window; the allocations counted are
	// the factory's, not the window's.
	w := NewWindow(WindowCfg{State: new(int)})
	cases := []struct {
		name string
		max  float64
		call func() View
	}{
		{"Column", 1, func() View { return Column(ContainerCfg{ID: "c"}) }},
		{"Row", 1, func() View { return Row(ContainerCfg{ID: "r"}) }},
		{"Button", 1, func() View { return Button(ButtonCfg{ID: "b"}) }},
		{"Input", 12, func() View { return Input(InputCfg{ID: "i"}) }},
		{"Slider", 15, func() View { return Slider(SliderCfg{ID: "s"}) }},
		{"Select", 1, func() View { return Select(SelectCfg{ID: "s"}) }},
		{"Switch", 10, func() View { return Switch(SwitchCfg{ID: "s"}) }},
		{"Toggle", 11, func() View { return Toggle(ToggleCfg{ID: "t"}) }},
		{"Radio", 8, func() View { return Radio(RadioCfg{ID: "r"}) }},
		{"ProgressBar", 7, func() View { return ProgressBar(ProgressBarCfg{ID: "p"}) }},
		{"ListBox", 1, func() View { return ListBox(ListBoxCfg{ID: "l"}) }},
		{"Combobox", 1, func() View { return Combobox(ComboboxCfg{ID: "c"}) }},
		{"Tree", 1, func() View { return Tree(TreeCfg{ID: "t"}) }},
		{"SegmentedControl", 1, func() View {
			return SegmentedControl(SegmentedControlCfg{ID: "s"})
		}},
		{"ExpandPanel", 1, func() View { return ExpandPanel(ExpandPanelCfg{ID: "e"}) }},
		{"NumericInput", 1, func() View { return NumericInput(NumericInputCfg{ID: "n"}) }},
		{"InputDate", 1, func() View { return InputDate(InputDateCfg{ID: "d"}) }},
		{"DatePicker", 1, func() View { return DatePicker(DatePickerCfg{ID: "d"}) }},
		{"Breadcrumb", 4, func() View { return Breadcrumb(BreadcrumbCfg{ID: "b"}) }},
		{"ColorPicker", 1, func() View { return ColorPicker(ColorPickerCfg{ID: "c"}) }},
		{"Form", 1, func() View { return Form(FormCfg{ID: "f"}) }},
		{"ThinkingOrb", 1, func() View { return ThinkingOrb(ThinkingOrbCfg{ID: "o"}) }},
		{"MathSpinner", 1, func() View { return MathSpinner(MathSpinnerCfg{ID: "m"}, nil) }},
		{"VirtualList", 1, func() View { return VirtualList(VirtualListCfg{ID: "v"}) }},
		// Factories that always defer hold cfg in a cfgDeferView: one
		// allocation, not a closure plus the cfg it captures.
		{"RadioButtonGroupColumn", 1, func() View {
			return RadioButtonGroupColumn(RadioButtonGroupCfg{ID: "g"})
		}},
		{"RadioButtonGroupRow", 1, func() View {
			return RadioButtonGroupRow(RadioButtonGroupCfg{ID: "g"})
		}},
		{"Table", 1, func() View { return Table(TableCfg{ID: "t"}) }},
		{"WindowTable", 1, func() View { return w.Table(TableCfg{ID: "t"}) }},
		{"ColorChannelSlider", 1, func() View {
			return ColorChannelSlider(ColorChannelSliderCfg{ID: "c"})
		}},
		{"ColorPlane", 1, func() View { return ColorPlane(ColorPlaneCfg{ID: "c"}) }},
		{"ColorWheel", 1, func() View { return ColorWheel(ColorWheelCfg{ID: "c"}) }},
		{"ColorFields", 1, func() View { return ColorFields(ColorFieldsCfg{ID: "c"}) }},
		// Their key handlers read most of the cfg, so they keep one
		// shared heap copy of it: the view plus that copy. That copy
		// hides the auto-ID branch from these two cases (main counts 2
		// too); they pin the count, not the branch.
		{"ContextMenu", 2, func() View { return ContextMenu(w, ContextMenuCfg{ID: "m"}) }},
		{"Menubar", 2, func() View { return Menubar(w, MenubarCfg{ID: "m"}) }},
		{"DatePickerRoller", 1, func() View {
			return DatePickerRoller(DatePickerRollerCfg{ID: "d"})
		}},
		{"ThinkingOrbLabel", 1, func() View {
			return ThinkingOrbLabel(ThinkingOrbLabelCfg{ID: "o"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := testing.AllocsPerRun(50, func() { _ = tc.call() })
			if got > tc.max {
				t.Fatalf("%s allocs = %v, want <= %v", tc.name, got, tc.max)
			}
		})
	}
}
