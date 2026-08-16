package gui

import "errors"

// ColorOverrides specifies which semantic colors to update across
// all widget styles. Nil pointers mean "keep existing".
// exportaudit:keep — reachable from an exported signature
type ColorOverrides struct {
	ColorBackground  *Color
	ColorPanel       *Color
	ColorInterior    *Color
	ColorHover       *Color
	ColorFocus       *Color
	ColorActive      *Color
	ColorBorder      *Color
	ColorBorderFocus *Color
	ColorSelect      *Color
}

func colorOr(override *Color, fallback Color) Color {
	if override != nil {
		return *override
	}
	return fallback
}

// WithColors returns a new Theme with the specified colors updated
// across all widget styles.
func (t Theme) withColors(o ColorOverrides) Theme {
	bg := colorOr(o.ColorBackground, t.ColorBackground)
	panel := colorOr(o.ColorPanel, t.ColorPanel)
	interior := colorOr(o.ColorInterior, t.ColorInterior)
	hover := colorOr(o.ColorHover, t.ColorHover)
	focus := colorOr(o.ColorFocus, t.ColorFocus)
	active := colorOr(o.ColorActive, t.ColorActive)
	border := colorOr(o.ColorBorder, t.ColorBorder)
	borderFocus := colorOr(o.ColorBorderFocus, t.ColorSelect)
	sel := colorOr(o.ColorSelect, t.ColorSelect)

	t.ColorBackground = bg
	t.ColorPanel = panel
	t.ColorInterior = interior
	t.ColorHover = hover
	t.ColorFocus = focus
	t.ColorActive = active
	t.ColorBorder = border
	t.ColorSelect = sel

	t.ButtonStyle.Color = interior
	t.ButtonStyle.ColorHover = hover
	t.ButtonStyle.ColorFocus = active
	t.ButtonStyle.colorClick = focus
	t.ButtonStyle.ColorBorder = border
	t.ButtonStyle.ColorBorderFocus = borderFocus

	t.InputStyle.Color = interior
	t.InputStyle.ColorHover = hover
	t.InputStyle.ColorFocus = interior
	t.InputStyle.colorClick = active
	t.InputStyle.ColorBorder = border
	t.InputStyle.ColorBorderFocus = borderFocus

	t.radioStyle.Color = panel
	t.radioStyle.ColorHover = hover
	t.radioStyle.ColorFocus = sel
	t.radioStyle.ColorBorder = border
	t.radioStyle.ColorBorderFocus = borderFocus
	t.radioStyle.ColorSelect = sel
	t.radioStyle.colorUnselect = active

	t.switchStyle.Color = panel
	t.switchStyle.ColorHover = hover
	t.switchStyle.ColorBorder = border
	t.switchStyle.ColorBorderFocus = borderFocus
	t.switchStyle.ColorSelect = sel
	t.switchStyle.colorUnselect = active

	t.toggleStyle.Color = panel
	t.toggleStyle.ColorHover = hover
	t.toggleStyle.ColorBorder = border
	t.toggleStyle.ColorBorderFocus = borderFocus

	t.selectStyle.Color = interior
	t.selectStyle.ColorHover = hover
	t.selectStyle.ColorFocus = focus
	t.selectStyle.colorClick = active
	t.selectStyle.ColorBorder = border
	t.selectStyle.ColorBorderFocus = borderFocus
	t.selectStyle.ColorSelect = sel

	t.listBoxStyle.Color = interior
	t.listBoxStyle.ColorHover = hover
	t.listBoxStyle.ColorBorder = border
	t.listBoxStyle.ColorBorderFocus = borderFocus
	t.listBoxStyle.ColorSelect = sel

	t.treeStyle.ColorHover = hover
	t.treeStyle.ColorFocus = focus
	if o.ColorBorder != nil {
		t.treeStyle.ColorBorder = border
	}

	t.ScrollbarStyle.colorThumb = active
	t.rectangleStyle.ColorBorder = border

	t.dialogStyle.Color = panel
	t.dialogStyle.ColorBorder = border
	t.dialogStyle.ColorBorderFocus = borderFocus

	t.toastStyle.Color = panel
	t.toastStyle.ColorBorder = border
	t.toastStyle.colorInfo = sel

	t.tooltipStyle.Color = interior
	t.tooltipStyle.ColorBorder = border

	t.badgeStyle.Color = active
	t.badgeStyle.colorInfo = sel

	t.expandPanelStyle.Color = panel
	t.expandPanelStyle.ColorHover = hover
	t.expandPanelStyle.colorClick = active
	t.expandPanelStyle.ColorBorder = border

	t.progressBarStyle.Color = interior
	t.progressBarStyle.colorBar = sel
	t.progressBarStyle.ColorBorder = border
	t.progressBarStyle.textBackground = ColorTransparent

	t.sliderStyle.Color = interior
	t.sliderStyle.colorClick = active
	t.sliderStyle.colorThumb = panel
	t.sliderStyle.colorLeft = sel
	t.sliderStyle.ColorFocus = sel
	t.sliderStyle.ColorHover = hover
	t.sliderStyle.ColorBorder = border
	t.sliderStyle.ColorBorderFocus = borderFocus

	t.tabControlStyle.Color = panel
	t.tabControlStyle.ColorBorder = border
	t.tabControlStyle.colorContent = panel
	t.tabControlStyle.colorContentBorder = border
	t.tabControlStyle.colorTab = interior
	t.tabControlStyle.colorTabHover = hover
	t.tabControlStyle.colorTabFocus = focus
	t.tabControlStyle.colorTabClick = active
	t.tabControlStyle.colorTabSelected = sel
	t.tabControlStyle.colorTabDisabled = panel
	t.tabControlStyle.colorTabBorder = border
	t.tabControlStyle.colorTabBorderFocus = borderFocus

	t.breadcrumbStyle.colorCrumbHover = hover
	t.breadcrumbStyle.colorCrumbClick = active
	t.breadcrumbStyle.colorContent = panel
	t.breadcrumbStyle.colorContentBorder = border

	t.splitterStyle.colorHandle = interior
	t.splitterStyle.colorHandleHover = hover
	t.splitterStyle.colorHandleActive = active
	t.splitterStyle.colorHandleBorder = border
	t.splitterStyle.colorGrip = sel
	t.splitterStyle.colorButton = interior
	t.splitterStyle.colorButtonHover = hover
	t.splitterStyle.colorButtonActive = active

	t.tableStyle.ColorBorder = border
	t.tableStyle.ColorSelect = sel
	t.tableStyle.ColorHover = hover

	t.comboboxStyle.Color = interior
	t.comboboxStyle.ColorHover = hover
	t.comboboxStyle.ColorFocus = interior
	t.comboboxStyle.ColorBorder = border
	t.comboboxStyle.ColorBorderFocus = borderFocus
	t.comboboxStyle.ColorHighlight = sel

	t.commandPaletteStyle.Color = panel
	t.commandPaletteStyle.ColorBorder = border
	t.commandPaletteStyle.ColorHighlight = sel

	t.MenubarStyle.Color = interior
	t.MenubarStyle.ColorHover = hover
	t.MenubarStyle.ColorFocus = focus
	t.MenubarStyle.ColorBorder = border
	t.MenubarStyle.ColorBorderFocus = borderFocus
	t.MenubarStyle.ColorSelect = sel

	t.datePickerStyle.Color = interior
	t.datePickerStyle.ColorHover = hover
	t.datePickerStyle.ColorFocus = focus
	t.datePickerStyle.colorClick = active
	t.datePickerStyle.ColorBorder = border
	t.datePickerStyle.ColorBorderFocus = borderFocus
	t.datePickerStyle.ColorSelect = sel

	t.colorPickerStyle.Color = interior
	t.colorPickerStyle.ColorBorder = border
	t.colorPickerStyle.ColorBorderFocus = borderFocus

	t.dataGridStyle.ColorBackground = interior
	t.dataGridStyle.ColorHeader = panel
	t.dataGridStyle.ColorHeaderHover = hover
	t.dataGridStyle.ColorFilter = interior
	t.dataGridStyle.ColorQuickFilter = panel
	t.dataGridStyle.ColorRowHover = hover
	t.dataGridStyle.ColorRowSelected = sel
	t.dataGridStyle.ColorBorder = border
	t.dataGridStyle.ColorResizeHandle = border
	t.dataGridStyle.ColorResizeActive = sel

	t.skeletonStyle.Color = interior
	t.skeletonStyle.ColorHighlight = hover

	t.inspectorStyle.ColorPanel = panel

	return t
}

// AdjustFontSize returns a new Theme with all font sizes adjusted
// by delta, clamped to [minSize, maxSize].
func (t Theme) AdjustFontSize(delta, minSize, maxSize float32) (Theme, error) {
	if minSize < 1 {
		return t, errors.New("minSize must be > 0")
	}
	cfg := t.Cfg
	newSize := cfg.TextStyleDef.Size + delta
	if newSize < minSize || newSize > maxSize {
		return t, errors.New("new font size out of range")
	}
	cfg.TextStyleDef.Size = newSize
	cfg.SizeTextTiny = max(cfg.SizeTextTiny+delta, 6)
	cfg.sizeTextXSmall = max(cfg.sizeTextXSmall+delta, 6)
	cfg.sizeTextSmall = max(cfg.sizeTextSmall+delta, 6)
	cfg.sizeTextMedium = max(cfg.sizeTextMedium+delta, 6)
	cfg.sizeTextLarge = max(cfg.sizeTextLarge+delta, 6)
	cfg.sizeTextXLarge = max(cfg.sizeTextXLarge+delta, 6)
	return ThemeMaker(cfg), nil
}
