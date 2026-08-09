package gui

// WithButtonStyle returns a Theme with the given style.
func (t Theme) withButtonStyle(s buttonStyle) Theme {
	t.ButtonStyle = s
	return t
}

// WithContainerStyle returns a Theme with the given style.
func (t Theme) withContainerStyle(s containerStyle) Theme {
	t.ContainerStyle = s
	return t
}

// WithRectangleStyle returns a Theme with the given style.
func (t Theme) withRectangleStyle(s RectangleStyle) Theme {
	t.rectangleStyle = s
	return t
}

// WithTextStyle returns a Theme with the given style.
func (t Theme) withTextStyle(s TextStyle) Theme {
	t.TextStyleDef = s
	return t
}

// WithInputStyle returns a Theme with the given style.
func (t Theme) withInputStyle(s InputStyle) Theme {
	t.InputStyle = s
	return t
}

// WithScrollbarStyle returns a Theme with the given style.
func (t Theme) withScrollbarStyle(s ScrollbarStyle) Theme {
	t.ScrollbarStyle = s
	return t
}

// WithRadioStyle returns a Theme with the given style.
func (t Theme) withRadioStyle(s RadioStyle) Theme {
	t.radioStyle = s
	return t
}

// WithSwitchStyle returns a Theme with the given style.
func (t Theme) withSwitchStyle(s SwitchStyle) Theme {
	t.switchStyle = s
	return t
}

// WithToggleStyle returns a Theme with the given style.
func (t Theme) withToggleStyle(s ToggleStyle) Theme {
	t.toggleStyle = s
	return t
}

// WithSelectStyle returns a Theme with the given style.
func (t Theme) withSelectStyle(s SelectStyle) Theme {
	t.selectStyle = s
	return t
}

// WithListBoxStyle returns a Theme with the given style.
func (t Theme) withListBoxStyle(s ListBoxStyle) Theme {
	t.listBoxStyle = s
	return t
}

// WithTreeStyle returns a Theme with the given style.
func (t Theme) withTreeStyle(s TreeStyle) Theme {
	t.treeStyle = s
	return t
}

// WithDialogStyle returns a Theme with the given style.
func (t Theme) withDialogStyle(s DialogStyle) Theme {
	t.dialogStyle = s
	return t
}

// WithToastStyle returns a Theme with the given style.
func (t Theme) withToastStyle(s ToastStyle) Theme {
	t.toastStyle = s
	return t
}

// WithTooltipStyle returns a Theme with the given style.
func (t Theme) withTooltipStyle(s TooltipStyle) Theme {
	t.tooltipStyle = s
	return t
}

// WithBadgeStyle returns a Theme with the given style.
func (t Theme) withBadgeStyle(s BadgeStyle) Theme {
	t.badgeStyle = s
	return t
}

// WithExpandPanelStyle returns a Theme with the given style.
func (t Theme) withExpandPanelStyle(s ExpandPanelStyle) Theme {
	t.expandPanelStyle = s
	return t
}

// WithProgressBarStyle returns a Theme with the given style.
func (t Theme) withProgressBarStyle(s ProgressBarStyle) Theme {
	t.progressBarStyle = s
	return t
}

// WithSliderStyle returns a Theme with the given style.
func (t Theme) withSliderStyle(s SliderStyle) Theme {
	t.sliderStyle = s
	return t
}

// WithTabControlStyle returns a Theme with the given style.
func (t Theme) withTabControlStyle(s TabControlStyle) Theme {
	t.tabControlStyle = s
	return t
}

// WithBreadcrumbStyle returns a Theme with the given style.
func (t Theme) withBreadcrumbStyle(s BreadcrumbStyle) Theme {
	t.breadcrumbStyle = s
	return t
}

// WithSplitterStyle returns a Theme with the given style.
func (t Theme) withSplitterStyle(s SplitterStyle) Theme {
	t.splitterStyle = s
	return t
}

// WithTableStyle returns a Theme with the given style.
func (t Theme) withTableStyle(s TableStyle) Theme {
	t.tableStyle = s
	return t
}

// WithComboboxStyle returns a Theme with the given style.
func (t Theme) withComboboxStyle(s ComboboxStyle) Theme {
	t.comboboxStyle = s
	return t
}

// WithCommandPaletteStyle returns a Theme with the given style.
func (t Theme) withCommandPaletteStyle(s CommandPaletteStyle) Theme {
	t.commandPaletteStyle = s
	return t
}

// WithMenubarStyle returns a Theme with the given style.
func (t Theme) withMenubarStyle(s MenubarStyle) Theme {
	t.MenubarStyle = s
	return t
}

// WithDatePickerStyle returns a Theme with the given style.
func (t Theme) withDatePickerStyle(s DatePickerStyle) Theme {
	t.datePickerStyle = s
	return t
}

// WithColorPickerStyle returns a Theme with the given style.
func (t Theme) withColorPickerStyle(s ColorPickerStyle) Theme {
	t.colorPickerStyle = s
	return t
}

// WithSkeletonStyle returns a Theme with the given style.
func (t Theme) withSkeletonStyle(s SkeletonStyle) Theme {
	t.skeletonStyle = s
	return t
}

// WithDataGridStyle returns a Theme with the given style.
func (t Theme) withDataGridStyle(s DataGridStyle) Theme {
	t.dataGridStyle = s
	return t
}

// WithInspectorStyle returns a Theme with the given style.
// exportaudit:keep — public theme API used by sibling repos
func (t Theme) WithInspectorStyle(s InspectorStyle) Theme {
	t.inspectorStyle = s
	return t
}
