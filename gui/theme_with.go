package gui

// WithButtonStyle returns a Theme with the given style.
func (t Theme) withButtonStyle(s buttonStyle) Theme {
	t.ButtonStyle = s
	t.id = nextThemeID()
	return t
}

// WithContainerStyle returns a Theme with the given style.
func (t Theme) withContainerStyle(s containerStyle) Theme {
	t.ContainerStyle = s
	t.id = nextThemeID()
	return t
}

// WithRectangleStyle returns a Theme with the given style.
func (t Theme) withRectangleStyle(s RectangleStyle) Theme {
	t.rectangleStyle = s
	t.id = nextThemeID()
	return t
}

// WithTextStyle returns a Theme with the given style.
func (t Theme) withTextStyle(s TextStyle) Theme {
	t.TextStyleDef = s
	t.id = nextThemeID()
	return t
}

// WithInputStyle returns a Theme with the given style.
func (t Theme) withInputStyle(s InputStyle) Theme {
	t.InputStyle = s
	t.id = nextThemeID()
	return t
}

// WithScrollbarStyle returns a Theme with the given style.
func (t Theme) withScrollbarStyle(s ScrollbarStyle) Theme {
	t.ScrollbarStyle = s
	t.id = nextThemeID()
	return t
}

// WithRadioStyle returns a Theme with the given style.
func (t Theme) withRadioStyle(s RadioStyle) Theme {
	t.radioStyle = s
	t.id = nextThemeID()
	return t
}

// WithSwitchStyle returns a Theme with the given style.
func (t Theme) withSwitchStyle(s SwitchStyle) Theme {
	t.switchStyle = s
	t.id = nextThemeID()
	return t
}

// WithToggleStyle returns a Theme with the given style.
func (t Theme) withToggleStyle(s ToggleStyle) Theme {
	t.toggleStyle = s
	t.id = nextThemeID()
	return t
}

// WithSelectStyle returns a Theme with the given style.
func (t Theme) withSelectStyle(s SelectStyle) Theme {
	t.selectStyle = s
	t.id = nextThemeID()
	return t
}

// WithListBoxStyle returns a Theme with the given style.
func (t Theme) withListBoxStyle(s ListBoxStyle) Theme {
	t.listBoxStyle = s
	t.id = nextThemeID()
	return t
}

// WithTreeStyle returns a Theme with the given style.
func (t Theme) withTreeStyle(s TreeStyle) Theme {
	t.treeStyle = s
	t.id = nextThemeID()
	return t
}

// WithDialogStyle returns a Theme with the given style.
func (t Theme) withDialogStyle(s DialogStyle) Theme {
	t.dialogStyle = s
	t.id = nextThemeID()
	return t
}

// WithToastStyle returns a Theme with the given style.
func (t Theme) withToastStyle(s ToastStyle) Theme {
	t.toastStyle = s
	t.id = nextThemeID()
	return t
}

// WithTooltipStyle returns a Theme with the given style.
func (t Theme) withTooltipStyle(s TooltipStyle) Theme {
	t.tooltipStyle = s
	t.id = nextThemeID()
	return t
}

// WithBadgeStyle returns a Theme with the given style.
func (t Theme) withBadgeStyle(s BadgeStyle) Theme {
	t.badgeStyle = s
	t.id = nextThemeID()
	return t
}

// WithExpandPanelStyle returns a Theme with the given style.
func (t Theme) withExpandPanelStyle(s ExpandPanelStyle) Theme {
	t.expandPanelStyle = s
	t.id = nextThemeID()
	return t
}

// WithProgressBarStyle returns a Theme with the given style.
func (t Theme) withProgressBarStyle(s ProgressBarStyle) Theme {
	t.progressBarStyle = s
	t.id = nextThemeID()
	return t
}

// WithSliderStyle returns a Theme with the given style.
func (t Theme) withSliderStyle(s SliderStyle) Theme {
	t.sliderStyle = s
	t.id = nextThemeID()
	return t
}

// WithTabControlStyle returns a Theme with the given style.
func (t Theme) withTabControlStyle(s TabControlStyle) Theme {
	t.tabControlStyle = s
	t.id = nextThemeID()
	return t
}

// WithBreadcrumbStyle returns a Theme with the given style.
func (t Theme) withBreadcrumbStyle(s BreadcrumbStyle) Theme {
	t.breadcrumbStyle = s
	t.id = nextThemeID()
	return t
}

// WithSplitterStyle returns a Theme with the given style.
func (t Theme) withSplitterStyle(s SplitterStyle) Theme {
	t.splitterStyle = s
	t.id = nextThemeID()
	return t
}

// WithTableStyle returns a Theme with the given style.
func (t Theme) withTableStyle(s TableStyle) Theme {
	t.tableStyle = s
	t.id = nextThemeID()
	return t
}

// WithComboboxStyle returns a Theme with the given style.
func (t Theme) withComboboxStyle(s ComboboxStyle) Theme {
	t.comboboxStyle = s
	t.id = nextThemeID()
	return t
}

// WithCommandPaletteStyle returns a Theme with the given style.
func (t Theme) withCommandPaletteStyle(s CommandPaletteStyle) Theme {
	t.commandPaletteStyle = s
	t.id = nextThemeID()
	return t
}

// WithMenubarStyle returns a Theme with the given style.
func (t Theme) withMenubarStyle(s MenubarStyle) Theme {
	t.MenubarStyle = s
	t.id = nextThemeID()
	return t
}

// WithDatePickerStyle returns a Theme with the given style.
func (t Theme) withDatePickerStyle(s DatePickerStyle) Theme {
	t.datePickerStyle = s
	t.id = nextThemeID()
	return t
}

// WithColorPickerStyle returns a Theme with the given style.
func (t Theme) withColorPickerStyle(s ColorPickerStyle) Theme {
	t.colorPickerStyle = s
	t.id = nextThemeID()
	return t
}

// WithSkeletonStyle returns a Theme with the given style.
func (t Theme) withSkeletonStyle(s SkeletonStyle) Theme {
	t.skeletonStyle = s
	t.id = nextThemeID()
	return t
}

// WithDataGridStyle returns a Theme with the given style.
func (t Theme) withDataGridStyle(s DataGridStyle) Theme {
	t.dataGridStyle = s
	t.id = nextThemeID()
	return t
}

// withInspectorStyle returns a Theme with the given style.
func (t Theme) withInspectorStyle(s InspectorStyle) Theme {
	t.inspectorStyle = s
	t.id = nextThemeID()
	return t
}
