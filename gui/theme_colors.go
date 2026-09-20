package gui

import "errors"

// ColorOverrides specifies which colors to update across all widget
// styles. A zero Color means "keep existing" (Color carries its own
// set flag), so an explicit fully-transparent override stays
// honorable. See WithColors for how derived slots follow.
//
// exportaudit:keep — public recolor surface (WithColors); consumers
// land with the next release.
type ColorOverrides struct {
	ColorBackground  Color
	ColorPanel       Color
	ColorInterior    Color
	ColorHover       Color
	ColorFocus       Color
	ColorActive      Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSeparator   Color
	ColorSelect      Color
	// ColorTextOnSelect is the text drawn over ColorSelect fills.
	// Unset keeps the theme's current value.
	ColorTextOnSelect Color

	// Accent ramp. ColorAccent is the single accent decision; the
	// hover, pressed, subtle and text-on slots derive from it exactly
	// as in ThemeMaker unless explicitly overridden here.
	ColorAccent        Color
	ColorAccentHover   Color
	ColorAccentPressed Color
	ColorAccentSubtle  Color
	ColorTextOnAccent  Color

	// Semantic colors and their subtle washes, painted by the toast,
	// badge and danger-button styles.
	ColorSuccess       Color
	ColorWarning       Color
	ColorError         Color
	ColorSuccessSubtle Color
	ColorWarningSubtle Color
	ColorErrorSubtle   Color
}

func colorOr(override, fallback Color) Color {
	if override.IsSet() {
		return override
	}
	return fallback
}

// track resolves a derived slot: the explicit override wins; otherwise
// the slot tracks its parent while the two still agree, and stays put
// once forked. oldWant is the parent's old derivation, newWant the new
// one — so a slot the theme stated explicitly is never stomped by an
// unrelated override. The comparison includes the set flag, so an
// explicit transparent stays distinct from an unset zero value.
func track(cur, override, oldWant, newWant Color) Color {
	if override.IsSet() {
		return override
	}
	if cur == oldWant {
		return newWant
	}
	return cur
}

// accentShift moves c by delta on the lightness axis, the same
// absolute-L step ThemeMaker builds the accent and danger ramps with.
func accentShift(c Color, delta float32) Color {
	hsla := ColorToHSLA(c)
	hsla.L = f32Clamp(hsla.L+delta, 0, 1)
	return hsla.Color()
}

// textOnFor returns the foreground paired with an accent fill: white
// under the 0.45 luminance threshold, black above it — the same rule
// ThemeMaker uses.
func textOnFor(c Color) Color {
	if srgbLuminance(c) < 0.45 {
		return White
	}
	return RGB(0, 0, 0)
}

// WithColors returns a new Theme with the specified colors updated
// across all widget styles.
//
// An explicitly set override always wins; an unset one keeps the
// theme's current value. Derived slots follow their parent while the
// two still agree and stay put once forked: overriding select alone
// moves the accent while accent and select match (the state every
// preset ships in), and overriding both sets them apart. The same
// rule carries border-focus with select, the separator with border,
// text-on-select with text-on-accent, and every subtle wash with its
// color. Touched fields sync into Cfg, so a later rebuild
// (WithPadding, WithBorders, AdjustFontSize) keeps the overrides.
// Like every with*Style helper, the result carries a fresh id.
//
// exportaudit:keep — public recolor surface; consumers land with the
// next release.
func (t Theme) WithColors(o ColorOverrides) Theme {
	bg := colorOr(o.ColorBackground, t.ColorBackground)
	panel := colorOr(o.ColorPanel, t.ColorPanel)
	interior := colorOr(o.ColorInterior, t.ColorInterior)
	hover := colorOr(o.ColorHover, t.ColorHover)
	focus := colorOr(o.ColorFocus, t.ColorFocus)
	active := colorOr(o.ColorActive, t.ColorActive)
	border := colorOr(o.ColorBorder, t.ColorBorder)

	oldSel := t.ColorSelect
	oldAccent := t.ColorAccent
	oldBg := t.ColorBackground
	oldBorder := t.ColorBorder
	oldTextOnAccent := t.ColorTextOnAccent
	oldTextOnSelect := t.ColorTextOnSelect
	oldBorderFocus := t.ButtonStyle.ColorBorderFocus
	oldSeparator := t.separatorStyle.Color

	// Select and accent are one decision while they agree: a lone
	// override moves both, stating both sets them apart.
	sel := colorOr(o.ColorSelect, oldSel)
	accent := colorOr(o.ColorAccent, oldAccent)
	if !o.ColorAccent.IsSet() && o.ColorSelect.IsSet() && oldAccent.eq(oldSel) {
		accent = sel
	}
	if !o.ColorSelect.IsSet() && o.ColorAccent.IsSet() && oldAccent.eq(oldSel) {
		sel = accent
	}

	// Accent ramp, derived exactly as in ThemeMaker.
	accentHover := track(t.ColorAccentHover, o.ColorAccentHover,
		accentShift(oldAccent, 0.12), accentShift(accent, 0.12))
	accentPressed := track(t.ColorAccentPressed, o.ColorAccentPressed,
		accentShift(oldAccent, -0.12), accentShift(accent, -0.12))
	accentSubtle := track(t.ColorAccentSubtle, o.ColorAccentSubtle,
		subtleFor(oldAccent, oldBg), subtleFor(accent, bg))
	textOnAccent := track(oldTextOnAccent, o.ColorTextOnAccent,
		textOnFor(oldAccent), textOnFor(accent))

	// Slots that default to a sibling: follow while they agree.
	selText := track(t.ColorTextOnSelect, o.ColorTextOnSelect,
		oldTextOnAccent, textOnAccent)
	borderFocus := track(oldBorderFocus, o.ColorBorderFocus, oldSel, sel)
	separator := track(oldSeparator, o.ColorSeparator, oldBorder, border)

	// Semantic colors live on the toast and badge styles; the danger
	// button derives its ramp from the error color.
	oldSuccess := t.toastStyle.ColorSuccess
	oldWarning := t.toastStyle.ColorWarning
	oldError := t.toastStyle.ColorError
	colorSuccess := colorOr(o.ColorSuccess, oldSuccess)
	colorWarning := colorOr(o.ColorWarning, oldWarning)
	colorError := colorOr(o.ColorError, oldError)
	successSubtle := track(t.ColorSuccessSubtle, o.ColorSuccessSubtle,
		subtleFor(oldSuccess, oldBg), subtleFor(colorSuccess, bg))
	warningSubtle := track(t.ColorWarningSubtle, o.ColorWarningSubtle,
		subtleFor(oldWarning, oldBg), subtleFor(colorWarning, bg))
	errorSubtle := track(t.ColorErrorSubtle, o.ColorErrorSubtle,
		subtleFor(oldError, oldBg), subtleFor(colorError, bg))

	// Named text roles are polarity-matched, not fixed (textRolesFor):
	// the same role means a different alpha over a dark ground than
	// over a light one. A background override can therefore flip the
	// whole ladder, and before this every quiet style kept the old
	// polarity's alpha — unreadable across a dark-to-light recolor.
	//
	// Derived twice and carried with track's rule: a role the theme
	// forked by hand (or stated through ThemeCfg.ColorText*) keeps its
	// value, one still sitting on its derivation follows.
	oldCfg := t.Cfg
	newCfg := oldCfg
	newCfg.ColorBackground = bg
	oldSecondary, oldLabel, oldDisabled, oldPlaceholder :=
		themeTextRoles(oldCfg, t.TextStyleDef, t.SizeTextXSmall)
	newSecondary, newLabel, newDisabled, newPlaceholder :=
		themeTextRoles(newCfg, t.TextStyleDef, t.SizeTextXSmall)
	trackStyle := func(cur, oldWant, newWant TextStyle) TextStyle {
		if cur == oldWant {
			return newWant
		}
		return cur
	}

	t.TextStyleSecondary = trackStyle(
		t.TextStyleSecondary, oldSecondary, newSecondary)
	t.TextStyleLabel = trackStyle(t.TextStyleLabel, oldLabel, newLabel)
	t.TextStyleDisabled = trackStyle(
		t.TextStyleDisabled, oldDisabled, newDisabled)
	t.TextStylePlaceholder = trackStyle(
		t.TextStylePlaceholder, oldPlaceholder, newPlaceholder)

	// Every style ThemeMaker builds from a role follows it here, or
	// the role moves and the widget drawing it does not.
	t.InputStyle.PlaceholderStyle = trackStyle(
		t.InputStyle.PlaceholderStyle, oldPlaceholder, newPlaceholder)
	t.selectStyle.PlaceholderStyle = trackStyle(
		t.selectStyle.PlaceholderStyle, oldPlaceholder, newPlaceholder)
	t.comboboxStyle.PlaceholderStyle = trackStyle(
		t.comboboxStyle.PlaceholderStyle, oldPlaceholder, newPlaceholder)
	t.progressBarStyle.TextStyle = trackStyle(
		t.progressBarStyle.TextStyle, oldSecondary, newSecondary)
	t.commandPaletteStyle.detailStyle = trackStyle(
		t.commandPaletteStyle.detailStyle, oldSecondary, newSecondary)
	t.breadcrumbStyle.textStyleSeparator = trackStyle(
		t.breadcrumbStyle.textStyleSeparator, oldSecondary, newSecondary)
	t.breadcrumbStyle.textStyleDisabled = trackStyle(
		t.breadcrumbStyle.textStyleDisabled, oldDisabled, newDisabled)
	t.tabControlStyle.textStyleDisabled = trackStyle(
		t.tabControlStyle.textStyleDisabled, oldDisabled, newDisabled)
	// The inspector's panel and wireframe colors stay deliberately
	// theme-independent (inspectorStyleFor); only its help text is
	// ordinary supporting text, so only that follows.
	t.inspectorStyle.colorTextHelp = track(
		t.inspectorStyle.colorTextHelp, Color{},
		oldSecondary.Color, newSecondary.Color)

	t.ColorBackground = bg
	t.ColorPanel = panel
	t.ColorInterior = interior
	t.ColorHover = hover
	t.ColorFocus = focus
	t.ColorActive = active
	t.ColorBorder = border
	t.ColorSelect = sel
	t.ColorTextOnSelect = selText
	t.ColorAccent = accent
	t.ColorAccentHover = accentHover
	t.ColorAccentPressed = accentPressed
	t.ColorAccentSubtle = accentSubtle
	t.ColorTextOnAccent = textOnAccent
	t.ColorSuccessSubtle = successSubtle
	t.ColorWarningSubtle = warningSubtle
	t.ColorErrorSubtle = errorSubtle

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
	t.InputStyle.colorSpellError = colorError

	t.radioStyle.Color = panel
	t.radioStyle.ColorHover = hover
	t.radioStyle.ColorFocus = sel
	t.radioStyle.colorClick = active
	t.radioStyle.ColorBorder = border
	t.radioStyle.ColorBorderFocus = borderFocus
	t.radioStyle.ColorSelect = sel
	t.radioStyle.colorUnselect = active

	t.switchStyle.Color = panel
	t.switchStyle.ColorHover = hover
	t.switchStyle.colorClick = interior
	t.switchStyle.ColorFocus = interior
	t.switchStyle.ColorBorder = border
	t.switchStyle.ColorBorderFocus = borderFocus
	t.switchStyle.ColorSelect = sel
	t.switchStyle.colorUnselect = active

	t.toggleStyle.Color = panel
	t.toggleStyle.ColorHover = hover
	t.toggleStyle.colorClick = interior
	t.toggleStyle.ColorFocus = interior
	t.toggleStyle.ColorSelect = interior
	t.toggleStyle.ColorBorder = border
	t.toggleStyle.ColorBorderFocus = borderFocus

	t.selectStyle.Color = interior
	t.selectStyle.ColorHover = hover
	t.selectStyle.ColorFocus = focus
	t.selectStyle.colorClick = active
	t.selectStyle.ColorBorder = border
	t.selectStyle.ColorBorderFocus = borderFocus
	t.selectStyle.ColorSelect = sel
	t.selectStyle.ColorSelectSubtle = accentSubtle

	t.listBoxStyle.Color = interior
	t.listBoxStyle.ColorHover = hover
	t.listBoxStyle.ColorBorder = border
	t.listBoxStyle.ColorBorderFocus = borderFocus
	t.listBoxStyle.ColorSelect = sel
	t.listBoxStyle.ColorSelectSubtle = accentSubtle

	t.treeStyle.ColorHover = hover
	t.treeStyle.ColorFocus = focus
	// No ColorBorder: ThemeMaker gives the tree a transparent border
	// whatever the theme's border color is, and painting one here
	// made a recolor add an outline the theme never asked for.

	t.ScrollbarStyle.colorThumb = active
	t.rectangleStyle.ColorBorder = border

	t.ButtonStylePrimary.Color = accent
	t.ButtonStylePrimary.ColorHover = accentHover
	t.ButtonStylePrimary.colorClick = accentPressed
	t.ButtonStylePrimary.ColorFocus = accent
	t.ButtonStylePrimary.ColorBorder = accent
	t.ButtonStylePrimary.ColorBorderFocus = borderFocus

	// Ghost drops fill and border, but keeps the base button's
	// pressed color and focus ring (deriveButtonStyles).
	t.ButtonStyleGhost.ColorHover = hover
	t.ButtonStyleGhost.colorClick = focus
	t.ButtonStyleGhost.ColorBorderFocus = borderFocus

	errorHover := accentShift(colorError, 0.12)
	errorPressed := accentShift(colorError, -0.12)
	t.ButtonStyleDanger.Color = colorError
	t.ButtonStyleDanger.ColorHover = errorHover
	t.ButtonStyleDanger.colorClick = errorPressed
	t.ButtonStyleDanger.ColorFocus = colorError
	t.ButtonStyleDanger.ColorBorder = colorError
	t.ButtonStyleDanger.ColorBorderFocus = borderFocus

	t.dialogStyle.Color = panel
	t.dialogStyle.ColorBorder = border
	t.dialogStyle.ColorBorderFocus = borderFocus

	t.toastStyle.Color = panel
	t.toastStyle.ColorBorder = border
	t.toastStyle.colorInfo = sel
	t.toastStyle.ColorSuccess = colorSuccess
	t.toastStyle.ColorWarning = colorWarning
	t.toastStyle.ColorError = colorError

	t.tooltipStyle.Color = interior
	t.tooltipStyle.ColorBorder = border

	// The neutral badge fill moves with ColorActive, and its label is
	// the paired foreground (issue #373), so the label follows.
	t.badgeStyle.TextStyle.Color = track(
		t.badgeStyle.TextStyle.Color, Color{},
		textOnFor(t.badgeStyle.Color), textOnFor(active))
	t.badgeStyle.Color = active
	t.badgeStyle.colorInfo = sel
	t.badgeStyle.ColorSuccess = colorSuccess
	t.badgeStyle.ColorWarning = colorWarning
	t.badgeStyle.ColorError = colorError

	t.expandPanelStyle.Color = panel
	t.expandPanelStyle.ColorHover = hover
	t.expandPanelStyle.colorClick = active
	t.expandPanelStyle.ColorBorder = border
	t.expandPanelStyle.ColorBorderFocus = borderFocus

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
	// The selected tab fills with the select color, so its label is
	// the paired foreground (issue #373). Without this a light accent
	// kept a white label on it.
	t.tabControlStyle.textStyleSelected = trackStyle(
		t.tabControlStyle.textStyleSelected,
		textOnFill(t.B3, true, oldTextOnSelect),
		textOnFill(t.B3, true, selText))

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
	t.tableStyle.ColorBorderFocus = borderFocus
	t.tableStyle.ColorSelect = sel
	t.tableStyle.ColorSelectSubtle = accentSubtle
	t.tableStyle.ColorHover = hover

	t.comboboxStyle.Color = interior
	t.comboboxStyle.ColorHover = hover
	t.comboboxStyle.ColorFocus = interior
	t.comboboxStyle.ColorBorder = border
	t.comboboxStyle.ColorBorderFocus = borderFocus
	t.comboboxStyle.ColorHighlight = sel
	t.comboboxStyle.ColorHighlightSubtle = accentSubtle

	t.commandPaletteStyle.Color = panel
	t.commandPaletteStyle.ColorBorder = border
	t.commandPaletteStyle.ColorHighlight = sel
	t.commandPaletteStyle.ColorHighlightSubtle = accentSubtle

	t.MenubarStyle.Color = interior
	t.MenubarStyle.ColorHover = hover
	t.MenubarStyle.ColorFocus = focus
	t.MenubarStyle.ColorBorder = border
	t.MenubarStyle.ColorBorderFocus = borderFocus
	t.MenubarStyle.ColorSelect = sel
	t.MenubarStyle.ColorTextOnSelect = selText

	t.datePickerStyle.Color = interior
	t.datePickerStyle.ColorHover = hover
	t.datePickerStyle.ColorFocus = focus
	t.datePickerStyle.colorClick = active
	t.datePickerStyle.ColorBorder = border
	t.datePickerStyle.ColorBorderFocus = borderFocus
	t.datePickerStyle.ColorSelect = sel
	t.datePickerStyle.ColorTextOnSelect = selText

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
	t.dataGridStyle.ColorRowSelectedSubtle = accentSubtle
	t.dataGridStyle.ColorBorder = border
	t.dataGridStyle.ColorResizeHandle = border
	t.dataGridStyle.ColorResizeActive = sel

	t.separatorStyle.Color = separator

	t.skeletonStyle.Color = interior
	// The shimmer is the fill lifted a fixed amount, not the hover
	// color — same derivation ThemeMaker uses.
	t.skeletonStyle.ColorHighlight = interior.Add(RGBA(20, 20, 20, 0))

	// Keep Cfg in sync so a later rebuild from the configuration
	// (WithPadding, WithBorders, AdjustFontSize) does not drop the
	// overrides. Only set fields sync; an unset override leaves both
	// the styles and the configuration alone. A stripped theme tunes
	// its restore point the same way, so strip, recolor, restore
	// keeps the recolor.
	// applyTo writes every set override into a configuration. Both the
	// live Cfg and the restore point take the same pass, so a stripped
	// theme's restore, restore-after-recolor and recolor-after-strip
	// all land on the same colors.
	applyTo := func(cfg *ThemeCfg) {
		syncColor := func(dst *Color, src Color) {
			if src.IsSet() {
				*dst = src
			}
		}
		syncColor(&cfg.ColorBackground, o.ColorBackground)
		syncColor(&cfg.ColorPanel, o.ColorPanel)
		syncColor(&cfg.ColorInterior, o.ColorInterior)
		syncColor(&cfg.ColorHover, o.ColorHover)
		syncColor(&cfg.ColorFocus, o.ColorFocus)
		syncColor(&cfg.ColorActive, o.ColorActive)
		syncColor(&cfg.ColorBorder, o.ColorBorder)
		syncColor(&cfg.ColorBorderFocus, o.ColorBorderFocus)
		syncColor(&cfg.ColorSeparator, o.ColorSeparator)
		syncColor(&cfg.ColorSelect, o.ColorSelect)
		syncColor(&cfg.ColorTextOnSelect, o.ColorTextOnSelect)
		syncColor(&cfg.ColorAccent, o.ColorAccent)
		syncColor(&cfg.ColorAccentHover, o.ColorAccentHover)
		syncColor(&cfg.ColorAccentPressed, o.ColorAccentPressed)
		syncColor(&cfg.ColorAccentSubtle, o.ColorAccentSubtle)
		syncColor(&cfg.ColorTextOnAccent, o.ColorTextOnAccent)
		syncColor(&cfg.ColorSuccess, o.ColorSuccess)
		syncColor(&cfg.ColorWarning, o.ColorWarning)
		syncColor(&cfg.ColorError, o.ColorError)
		syncColor(&cfg.ColorSuccessSubtle, o.ColorSuccessSubtle)
		syncColor(&cfg.ColorWarningSubtle, o.ColorWarningSubtle)
		syncColor(&cfg.ColorErrorSubtle, o.ColorErrorSubtle)
	}
	applyTo(&t.Cfg)
	if t.restoreCfg != nil {
		dup := *t.restoreCfg
		applyTo(&dup)
		t.restoreCfg = &dup
	}

	t.id = nextThemeID()
	return t
}

// AdjustFontSize returns a new Theme with all font sizes adjusted by
// delta. The body size must land inside [minSize, maxSize] or the
// theme comes back unchanged with an error; the derived rungs are
// clamped individually at a floor of sizeTextFloor, which is a
// legibility limit and not the caller's range. A stripped theme keeps
// its restore point, tuned by the same delta, so a later
// WithPadding(true) restores with the tune kept.
func (t Theme) AdjustFontSize(delta, minSize, maxSize float32) (Theme, error) {
	// The guard is >= 1, so say so: the old text read "> 0" and a
	// caller passing 0.5 got an error the message denied.
	if minSize < 1 {
		return t, errors.New("minSize must be >= 1")
	}
	// An inverted range accepted nothing, and said "out of range"
	// about the size rather than about the range.
	if maxSize < minSize {
		return t, errors.New("maxSize must be >= minSize")
	}
	cfg := t.Cfg
	newSize := cfg.TextStyleDef.Size + delta
	if newSize < minSize || newSize > maxSize {
		return t, errors.New("new font size out of range")
	}
	cfg.TextStyleDef.Size = newSize
	cfg.SizeTextTiny = max(cfg.SizeTextTiny+delta, sizeTextFloor)
	cfg.SizeTextXSmall = max(cfg.SizeTextXSmall+delta, sizeTextFloor)
	cfg.SizeTextSmall = max(cfg.SizeTextSmall+delta, sizeTextFloor)
	cfg.SizeTextMedium = max(cfg.SizeTextMedium+delta, sizeTextFloor)
	cfg.SizeTextLarge = max(cfg.SizeTextLarge+delta, sizeTextFloor)
	cfg.SizeTextXLarge = max(cfg.SizeTextXLarge+delta, sizeTextFloor)
	out := ThemeMaker(cfg)
	if t.restoreCfg != nil {
		dup := *t.restoreCfg
		dup.TextStyleDef.Size += delta
		dup.SizeTextTiny = max(dup.SizeTextTiny+delta, sizeTextFloor)
		dup.SizeTextXSmall = max(dup.SizeTextXSmall+delta, sizeTextFloor)
		dup.SizeTextSmall = max(dup.SizeTextSmall+delta, sizeTextFloor)
		dup.SizeTextMedium = max(dup.SizeTextMedium+delta, sizeTextFloor)
		dup.SizeTextLarge = max(dup.SizeTextLarge+delta, sizeTextFloor)
		dup.SizeTextXLarge = max(dup.SizeTextXLarge+delta, sizeTextFloor)
		out.restoreCfg = &dup
	}
	return out, nil
}
