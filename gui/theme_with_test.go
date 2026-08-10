package gui

import "testing"

func TestWithStyleMethodsSetsField(t *testing.T) {
	var base Theme

	// Verify a representative method properly sets its field.
	got := base.withButtonStyle(buttonStyle{Color: RGBA(42, 0, 0, 0)})
	if got.ButtonStyle.Color.R != 42 {
		t.Error("WithButtonStyle did not set ButtonStyle.Color")
	}
	// Verify non-target fields are unchanged.
	if got.Name != base.Name {
		t.Error("WithButtonStyle modified Name")
	}
}

func TestWithStyleMethodsCoverage(t *testing.T) {
	// Exercise every With* method for coverage.
	// Each method is a 2-statement setter; calling with any
	// value covers the full method body.
	var th Theme
	th = th.withButtonStyle(buttonStyle{})
	th = th.withContainerStyle(containerStyle{})
	th = th.withRectangleStyle(RectangleStyle{})
	th = th.withTextStyle(TextStyle{})
	th = th.withInputStyle(InputStyle{})
	th = th.withScrollbarStyle(ScrollbarStyle{})
	th = th.withRadioStyle(RadioStyle{})
	th = th.withSwitchStyle(SwitchStyle{})
	th = th.withToggleStyle(ToggleStyle{})
	th = th.withSelectStyle(SelectStyle{})
	th = th.withListBoxStyle(ListBoxStyle{})
	th = th.withTreeStyle(TreeStyle{})
	th = th.withDialogStyle(DialogStyle{})
	th = th.withToastStyle(ToastStyle{})
	th = th.withTooltipStyle(TooltipStyle{})
	th = th.withBadgeStyle(BadgeStyle{})
	th = th.withExpandPanelStyle(ExpandPanelStyle{})
	th = th.withProgressBarStyle(ProgressBarStyle{})
	th = th.withSliderStyle(SliderStyle{})
	th = th.withTabControlStyle(TabControlStyle{})
	th = th.withBreadcrumbStyle(BreadcrumbStyle{})
	th = th.withSplitterStyle(SplitterStyle{})
	th = th.withTableStyle(TableStyle{})
	th = th.withComboboxStyle(ComboboxStyle{})
	th = th.withCommandPaletteStyle(CommandPaletteStyle{})
	th = th.withMenubarStyle(MenubarStyle{})
	th = th.withDatePickerStyle(DatePickerStyle{})
	th = th.withColorPickerStyle(ColorPickerStyle{})
	th = th.withSkeletonStyle(SkeletonStyle{})
	th = th.withDataGridStyle(DataGridStyle{})
	_ = th
}

func TestWithTextStyleUsesTextStyleDefField(t *testing.T) {
	var base Theme
	got := base.withTextStyle(TextStyle{Color: RGBA(0, 99, 0, 0)})
	if got.TextStyleDef.Color.G != 99 {
		t.Error("WithTextStyle should set TextStyleDef field")
	}
}
