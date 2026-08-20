package gui

import (
	"cmp"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// ThemeMaker builds a full Theme from a ThemeCfg.
func ThemeMaker(cfg ThemeCfg) Theme {
	ts := cfg.TextStyleDef
	makeStyle := func(base TextStyle, size float32) TextStyle {
		s := base
		s.Size = size
		return s
	}

	// Text size ladder. The body size is the per-theme decision
	// (visual-refresh §2.1): the ladder derives from it, so a theme
	// that states TextStyleDef.Size alone gets a complete ladder and a
	// theme that seeds explicit rungs keeps them. Zero takes the
	// derived value, which is what "Zero takes the built-in defaults"
	// on ThemeCfg.SizeText* promises.
	body := ts.Size
	if body <= 0 {
		// Zero TextStyleDef.Size is "unset", not "0px text": fall back
		// to the built-in body so a partial theme never derives a
		// negative ladder.
		body = sizeTextMedium
	}
	derived := textSizes(body)
	ladder := textSizeLadder{
		tiny:   cmp.Or(cfg.SizeTextTiny, derived.tiny),
		xSmall: cmp.Or(cfg.SizeTextXSmall, derived.xSmall),
		small:  cmp.Or(cfg.SizeTextSmall, derived.small),
		medium: cmp.Or(cfg.SizeTextMedium, derived.medium),
		large:  cmp.Or(cfg.SizeTextLarge, derived.large),
		xLarge: cmp.Or(cfg.SizeTextXLarge, derived.xLarge),
	}

	// Icon family for every theme-driven icon style. A ThemeCfg built
	// from scratch (not via baseCfg) leaves this empty, so fall back to
	// the bundled font rather than render icons in the default family.
	iconFamily := cmp.Or(cfg.IconFontFamily, IconFontName)

	// Accent ramp (visual-refresh §4.3). One decision with a fallback
	// chain: ColorAccent, else ColorSelect (a platform or taste theme
	// keeps its native select as its accent), else the legacy select
	// value, so a theme that states neither renders as before. Every
	// unset slot derives from the accent; an explicit slot always
	// wins.
	accent := cfg.ColorAccent
	switch {
	case accent.IsSet():
	case cfg.ColorSelect.IsSet():
		accent = cfg.ColorSelect
	default:
		accent = colorSelectDark
	}
	// The selection fill defaults to the accent, so selection and
	// accent stay one decision by default and two when a theme needs
	// them apart.
	colorSelect := cfg.ColorSelect
	if !colorSelect.IsSet() {
		colorSelect = accent
	}

	// HSL offsets are absolute on L, not relative: a relative step
	// collapses to nothing on a dark accent and overshoots on a light
	// one. Clamped to [0,1]; the derivation is pinned against the
	// spec table by TestThemeMakerAccentRamp.
	accentHover := cfg.ColorAccentHover
	if !accentHover.IsSet() {
		hsla := ColorToHSLA(accent)
		hsla.L = f32Clamp(hsla.L+0.12, 0, 1)
		accentHover = hsla.Color()
	}
	accentPressed := cfg.ColorAccentPressed
	if !accentPressed.IsSet() {
		hsla := ColorToHSLA(accent)
		hsla.L = f32Clamp(hsla.L-0.12, 0, 1)
		accentPressed = hsla.Color()
	}
	accentSubtle := cfg.ColorAccentSubtle
	if !accentSubtle.IsSet() {
		accentSubtle = subtleFor(accent, cfg.ColorBackground)
	}
	// Text on the accent: white under the 0.45 luminance threshold,
	// not the midpoint — white on a mid-blue reads better than black
	// at the same luminance.
	textOnAccent := cfg.ColorTextOnAccent
	if !textOnAccent.IsSet() {
		if srgbLuminance(accent) < 0.45 {
			textOnAccent = White
		} else {
			textOnAccent = RGB(0, 0, 0)
		}
	}

	// Semantic colors (visual-refresh §4.4). Filled on the Cfg by the
	// presets; unset falls back to the values the toast/badge styles
	// historically painted, so a theme that never stated them renders
	// as before.
	colorSuccess := cfg.ColorSuccess
	if !colorSuccess.IsSet() {
		colorSuccess = RGBA(46, 160, 67, 255)
	}
	colorWarning := cfg.ColorWarning
	if !colorWarning.IsSet() {
		colorWarning = RGBA(210, 153, 34, 255)
	}
	colorError := cfg.ColorError
	if !colorError.IsSet() {
		colorError = RGBA(218, 54, 51, 255)
	}
	successSubtle := cfg.ColorSuccessSubtle
	if !successSubtle.IsSet() {
		successSubtle = subtleFor(colorSuccess, cfg.ColorBackground)
	}
	warningSubtle := cfg.ColorWarningSubtle
	if !warningSubtle.IsSet() {
		warningSubtle = subtleFor(colorWarning, cfg.ColorBackground)
	}
	errorSubtle := cfg.ColorErrorSubtle
	if !errorSubtle.IsSet() {
		errorSubtle = subtleFor(colorError, cfg.ColorBackground)
	}

	borderFocus := cfg.ColorBorderFocus
	if borderFocus.eq(Color{}) {
		borderFocus = colorSelect
	}

	// Text drawn over the select fill. The fill and the text on it
	// are one decision (issue #373); unset resolves to the
	// accent-paired foreground — white on a dark accent, black on a
	// light one — so the full-accent fills (menus, the selected tab,
	// the slider fill, the progress bar) stay readable on any
	// polarity. This is the same rule as ColorTextOnAccent, which is
	// what the pairing would have been anyway (visual-refresh §4.3;
	// the old body-color default made a light theme draw near-black
	// text on its blue accent).
	colorTextOnSelect := cfg.ColorTextOnSelect
	if !colorTextOnSelect.IsSet() {
		colorTextOnSelect = textOnAccent
	}

	// Separator role: a divider is an edge, and reusing ColorBorder
	// would couple the two forever. Unset falls back to the border
	// color so existing themes keep their look without restating it.
	colorSeparator := cfg.ColorSeparator
	if !colorSeparator.IsSet() {
		colorSeparator = cfg.ColorBorder
	}
	sizeSeparator := cfg.SizeSeparator
	if sizeSeparator == 0 {
		sizeSeparator = 1
	}

	// Scrollbar radius: none if cfg.Radius is none.
	sbRadius := cfg.RadiusSmall
	if cfg.Radius == radiusNone {
		sbRadius = radiusNone
	}

	// Field inset: the padding a text-bearing form control puts
	// around its text. One tier, so an Input and a Select in the same
	// row share a height (issue #335, audit section 7).
	fieldPad := cfg.PaddingField
	if !fieldPad.IsSet() {
		fieldPad = paddingField
	}

	// Named text roles. Every de-emphasized style below draws from
	// these rather than restating an alpha (issue #335).
	textSecondary, textLabel, textDisabled, textPlaceholder :=
		themeTextRoles(cfg, ts, ladder.xSmall)

	buttonBase, buttonPrimary, buttonGhost, buttonDanger :=
		deriveButtonStyles(cfg, accent, accentHover, accentPressed,
			colorError, borderFocus)

	theme := Theme{
		Cfg:                  cfg,
		Name:                 cfg.Name,
		focusRing:            cfg.FocusRing,
		TextStyleSecondary:   textSecondary,
		TextStyleLabel:       textLabel,
		TextStyleDisabled:    textDisabled,
		TextStylePlaceholder: textPlaceholder,
		ColorBackground:      cfg.ColorBackground,
		ColorPanel:           cfg.ColorPanel,
		ColorInterior:        cfg.ColorInterior,
		ColorHover:           cfg.ColorHover,
		ColorFocus:           cfg.ColorFocus,
		ColorActive:          cfg.ColorActive,
		ColorBorder:          cfg.ColorBorder,
		ColorSelect:          colorSelect,
		TitlebarDark:         cfg.TitlebarDark,
		ColorTextOnSelect:    colorTextOnSelect,
		ColorAccent:          accent,
		ColorAccentHover:     accentHover,
		ColorAccentPressed:   accentPressed,
		ColorAccentSubtle:    accentSubtle,
		ColorTextOnAccent:    textOnAccent,
		ColorSuccessSubtle:   successSubtle,
		ColorWarningSubtle:   warningSubtle,
		ColorErrorSubtle:     errorSubtle,

		ButtonStyle:        buttonBase,
		ButtonStylePrimary: buttonPrimary,
		ButtonStyleGhost:   buttonGhost,
		ButtonStyleDanger:  buttonDanger,
		ContainerStyle: containerStyle{
			Color:       ColorTransparent,
			ColorBorder: ColorTransparent,
			Padding:     cfg.Padding,
			Radius:      cfg.Radius,
			Spacing:     cfg.SpacingMedium,
			SizeBorder:  cfg.SizeBorder,
		},
		rectangleStyle: RectangleStyle{
			Color:       ColorTransparent,
			ColorBorder: cfg.ColorBorder,
			Radius:      cfg.Radius,
			SizeBorder:  cfg.SizeBorder,
		},
		TextStyleDef: ts,
		InputStyle: InputStyle{
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorInterior,
			colorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          fieldPad,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.Radius,
			textStyleNormal:  ts,
			PlaceholderStyle: textPlaceholder,
			colorSpellError:  colorError,
		},
		ScrollbarStyle: ScrollbarStyle{
			Size:            cfg.SizeScrollbar,
			minThumbSize:    cfg.SizeScrollbarMin,
			colorThumb:      cfg.ColorActive,
			ColorBackground: ColorTransparent,
			Radius:          sbRadius,
			radiusThumb:     sbRadius,
			GapEdge:         3,
			GapEnd:          2,
		},
		radioStyle: RadioStyle{
			Size:             cfg.SizeRadio,
			Color:            cfg.ColorPanel,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       colorSelect,
			colorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      colorSelect,
			colorUnselect:    cfg.ColorActive,
			Padding:          PadAll(4),
			SizeBorder:       cfg.SizeBorder,
			textStyleNormal:  ts,
		},
		switchStyle: SwitchStyle{
			sizeWidth:        cfg.SizeSwitchWidth,
			sizeHeight:       cfg.SizeSwitchHeight,
			Color:            cfg.ColorPanel,
			colorClick:       cfg.ColorInterior,
			ColorFocus:       cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      colorSelect,
			colorUnselect:    cfg.ColorActive,
			Padding:          paddingThree,
			SizeBorder:       cfg.SizeBorder,
			Radius:           radiusLarge * 2,
			textStyleNormal:  ts,
		},
		toggleStyle: ToggleStyle{
			Color:            cfg.ColorPanel,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			colorClick:       cfg.ColorInterior,
			ColorFocus:       cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorSelect:      cfg.ColorInterior,
			// Symmetric: the check is centred on its ink (see
			// centerGlyphOnInk), so any padding asymmetry here is a
			// pure off-centre error, not a nudge that helps.
			Padding:         PadAll(1),
			Size:            ts.Size + 4,
			SizeBorder:      cfg.SizeBorder,
			Radius:          cfg.Radius,
			textStyleNormal: ts,
			textStyleLabel:  ts,
		},
		selectStyle: SelectStyle{
			Shadow: cfg.ShadowPopover,
			// One theme-stated floor, shared with Input and
			// Combobox; the old 75/200 pair was why a Select and
			// an Input in one row disagreed on width for a reason
			// neither the theme nor the caller stated. No ceiling:
			// a capped Select cannot fill the row a labelled field
			// can now ask for.
			MinWidth:          cfg.SizeFieldMinWidth,
			MaxWidth:          0,
			Color:             cfg.ColorInterior,
			ColorHover:        cfg.ColorHover,
			ColorFocus:        cfg.ColorFocus,
			colorClick:        cfg.ColorActive,
			ColorBorder:       cfg.ColorBorder,
			ColorBorderFocus:  borderFocus,
			ColorSelect:       colorSelect,
			ColorSelectSubtle: accentSubtle,
			Padding:           fieldPad,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusMedium,
			textStyleNormal:   ts,
			subheadingStyle:   ts,
			PlaceholderStyle:  textPlaceholder,
		},
		listBoxStyle: ListBoxStyle{
			Color:             cfg.ColorInterior,
			ColorHover:        cfg.ColorHover,
			ColorBorder:       cfg.ColorBorder,
			ColorBorderFocus:  borderFocus,
			ColorSelect:       colorSelect,
			ColorSelectSubtle: accentSubtle,
			Padding:           cfg.Padding,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.Radius,
			textStyleNormal:   ts,
			subheadingStyle:   ts,
		},
		treeStyle: TreeStyle{
			Color:       ColorTransparent,
			ColorHover:  cfg.ColorHover,
			ColorFocus:  cfg.ColorFocus,
			ColorBorder: ColorTransparent,
			Padding:     PaddingNone,
			SizeBorder:  cfg.SizeBorder,
			Radius:      cfg.Radius,
			TextStyle:   ts,
			textStyleIcon: TextStyle{
				Color:     ts.Color,
				Size:      ladder.small,
				Family:    iconFamily,
				glyphRole: true,
			},
			indent:  25,
			Spacing: 0,
		},
		dialogStyle: DialogStyle{
			Shadow:           cfg.ShadowDialog,
			Color:            cfg.ColorPanel,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          cfg.PaddingLarge.withSet(),
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			radiusBorder:     cfg.RadiusMedium,
			AlignButtons:     HAlignCenter,
			MinWidth:         200,
			MaxWidth:         300,
			titleTextStyle:   makeStyle(ts, cfg.SizeTextLarge),
			TextStyle:        ts,
		},
		toastStyle: ToastStyle{
			Shadow:       cfg.ShadowPopover,
			maxVisible:   5,
			Anchor:       toastBottomRight,
			Width:        260,
			margin:       16,
			Spacing:      cfg.SpacingMedium,
			accentWidth:  4,
			Padding:      cfg.PaddingMedium.withSet(),
			Radius:       cfg.RadiusMedium,
			SizeBorder:   cfg.SizeBorder,
			Color:        cfg.ColorPanel,
			ColorBorder:  cfg.ColorBorder,
			colorInfo:    colorSelect,
			ColorSuccess: colorSuccess,
			ColorWarning: colorWarning,
			ColorError:   colorError,
			TextStyle:    ts,
			TitleStyle:   makeStyle(ts, cfg.SizeTextMedium),
		},
		tooltipStyle: TooltipStyle{
			Shadow:      cfg.ShadowPopover,
			Delay:       500 * time.Millisecond,
			Color:       cfg.ColorInterior,
			ColorBorder: cfg.ColorBorder,
			Padding:     cfg.PaddingSmall.withSet(),
			SizeBorder:  cfg.SizeBorder,
			Radius:      cfg.RadiusSmall,
			TextStyle:   ts,
		},
		badgeStyle: BadgeStyle{
			Color:        cfg.ColorActive,
			colorInfo:    colorSelect,
			ColorSuccess: colorSuccess,
			ColorWarning: colorWarning,
			ColorError:   colorError,
			Padding:      NewPadding(2, 6, 2, 6),
			dotSize:      8,
		},
		expandPanelStyle: ExpandPanelStyle{
			Color:            cfg.ColorPanel,
			ColorHover:       cfg.ColorHover,
			colorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          cfg.PaddingMedium.withSet(),
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			radiusBorder:     cfg.RadiusMedium,
		},
		progressBarStyle: ProgressBarStyle{
			Size:        cfg.SizeProgressBar,
			Color:       cfg.ColorInterior,
			colorBar:    colorSelect,
			ColorBorder: cfg.ColorBorder,
			// The % label keeps the body text on the bar, unboxed:
			// it straddles fill and track, so no single color pairs
			// with both — the label is secondary and stays as
			// before (visual-refresh §4.3).
			textBackground: ColorTransparent,
			Padding:        PaddingNone,
			textPadding:    NewPadding(1, 4, 1, 4),
			Radius:         cfg.RadiusSmall,
			TextShow:       true,
			TextStyle:      ts,
		},
		skeletonStyle: SkeletonStyle{
			Color:          cfg.ColorInterior,
			ColorHighlight: cfg.ColorInterior.Add(RGBA(20, 20, 20, 0)),
			Radius:         cfg.RadiusSmall,
		},
		separatorStyle: SeparatorStyle{
			Color: colorSeparator,
			Size:  sizeSeparator,
		},
		sliderStyle: SliderStyle{
			Size:             cfg.SizeSlider,
			ThumbSize:        cfg.SizeSliderThumb,
			Color:            cfg.ColorInterior,
			colorClick:       cfg.ColorActive,
			colorThumb:       cfg.ColorPanel,
			colorLeft:        colorSelect,
			ColorFocus:       colorSelect,
			ColorHover:       cfg.ColorHover,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          PaddingNone,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.SizeSlider / 2,
		},
		tabControlStyle: TabControlStyle{
			Color:               cfg.ColorPanel,
			ColorBorder:         cfg.ColorBorder,
			ColorHeader:         ColorTransparent,
			colorHeaderBorder:   ColorTransparent,
			colorContent:        cfg.ColorPanel,
			colorContentBorder:  cfg.ColorBorder,
			colorTab:            cfg.ColorInterior,
			colorTabHover:       cfg.ColorHover,
			colorTabFocus:       cfg.ColorFocus,
			colorTabClick:       cfg.ColorActive,
			colorTabSelected:    colorSelect,
			colorTabDisabled:    cfg.ColorPanel,
			colorTabBorder:      cfg.ColorBorder,
			colorTabBorderFocus: borderFocus,
			Padding:             PaddingNone,
			PaddingHeader:       PaddingNone,
			paddingContent:      cfg.PaddingMedium.withSet(),
			paddingTab:          cfg.PaddingSmall.withSet(),
			SizeBorder:          cfg.SizeBorder,
			sizeTabBorder:       cfg.SizeBorder,
			Radius:              cfg.RadiusMedium,
			radiusHeader:        cfg.RadiusSmall,
			radiusContent:       cfg.RadiusMedium,
			radiusTab:           cfg.RadiusSmall,
			spacingHeader:       cfg.SpacingTight,
			TextStyle:           ts,
			textStyleSelected:   ts,
			textStyleDisabled:   textDisabled,
		},
		breadcrumbStyle: BreadcrumbStyle{
			Separator:          "/",
			Color:              ColorTransparent,
			ColorBorder:        ColorTransparent,
			colorTrail:         ColorTransparent,
			colorCrumb:         ColorTransparent,
			colorCrumbHover:    cfg.ColorHover,
			colorCrumbClick:    cfg.ColorActive,
			colorCrumbSelected: ColorTransparent,
			colorCrumbDisabled: ColorTransparent,
			colorContent:       cfg.ColorPanel,
			colorContentBorder: cfg.ColorBorder,
			Padding:            PaddingNone,
			paddingTrail:       cfg.PaddingSmall.withSet(),
			paddingCrumb:       NewPadding(2, 4, 2, 4),
			paddingContent:     cfg.PaddingMedium.withSet(),
			Radius:             cfg.RadiusMedium,
			radiusCrumb:        cfg.RadiusSmall,
			radiusContent:      cfg.RadiusMedium,
			Spacing:            cfg.SpacingSmall,
			spacingTrail:       cfg.SpacingSmall,
			sizeContentBorder:  cfg.SizeBorder,
			TextStyle:          ts,
			textStyleSelected:  ts,
			textStyleDisabled:  textDisabled,
			textStyleSeparator: textSecondary,
		},
		splitterStyle: SplitterStyle{
			HandleSize:        9,
			dragStep:          0.02,
			dragStepLarge:     0.10,
			colorHandle:       cfg.ColorInterior,
			colorHandleHover:  cfg.ColorHover,
			colorHandleActive: cfg.ColorActive,
			colorHandleBorder: cfg.ColorBorder,
			colorGrip:         colorSelect,
			colorButton:       cfg.ColorInterior,
			colorButtonHover:  cfg.ColorHover,
			colorButtonActive: cfg.ColorActive,
			colorButtonIcon:   ts.Color,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusSmall,
			radiusBorder:      cfg.RadiusSmall,
		},
		tableStyle: TableStyle{
			ColorBorder:        cfg.ColorBorder,
			ColorBorderFocus:   borderFocus,
			ColorSelect:        colorSelect,
			ColorSelectSubtle:  accentSubtle,
			ColorHover:         cfg.ColorHover,
			cellPadding:        PaddingTwoFive,
			TextStyle:          ts,
			TextStyleHead:      ts,
			alignHead:          HAlignCenter,
			columnWidthDefault: 50,
			columnWidthMin:     20,
		},
		comboboxStyle: ComboboxStyle{
			Shadow:               cfg.ShadowPopover,
			Color:                cfg.ColorInterior,
			ColorHover:           cfg.ColorHover,
			ColorFocus:           cfg.ColorInterior,
			ColorBorder:          cfg.ColorBorder,
			ColorBorderFocus:     borderFocus,
			ColorHighlight:       colorSelect,
			ColorHighlightSubtle: accentSubtle,
			Padding:              fieldPad,
			SizeBorder:           cfg.SizeBorder,
			Radius:               cfg.Radius,
			// Same floor and same uncapped width as selectStyle
			// above; maxDropdownHeight bounds the popup list, not
			// the control, and is unrelated.
			MinWidth:          cfg.SizeFieldMinWidth,
			MaxWidth:          0,
			maxDropdownHeight: 200,
			TextStyle:         ts,
			PlaceholderStyle:  textPlaceholder,
		},
		commandPaletteStyle: CommandPaletteStyle{
			Shadow:               cfg.ShadowDialog,
			Color:                cfg.ColorPanel,
			ColorBorder:          cfg.ColorBorder,
			ColorHighlight:       colorSelect,
			ColorHighlightSubtle: accentSubtle,
			SizeBorder:           cfg.SizeBorder,
			Radius:               cfg.Radius,
			Width:                500,
			MaxHeight:            400,
			TextStyle:            ts,
			detailStyle:          textSecondary,
			backdropColor:        RGBA(0, 0, 0, 120),
		},
		MenubarStyle: MenubarStyle{
			Shadow:            cfg.ShadowPopover,
			widthSubmenuMin:   50,
			widthSubmenuMax:   200,
			Color:             cfg.ColorInterior,
			ColorHover:        cfg.ColorHover,
			ColorFocus:        cfg.ColorFocus,
			ColorBorder:       cfg.ColorBorder,
			ColorBorderFocus:  borderFocus,
			ColorSelect:       colorSelect,
			ColorTextOnSelect: colorTextOnSelect,
			Padding:           cfg.PaddingSmall.withSet(),
			paddingMenuItem:   PaddingTwoFive,
			paddingSubmenu:    cfg.PaddingSmall.withSet(),
			paddingSubtitle:   NewPadding(0, cfg.PaddingSmall.Right, 0, cfg.PaddingSmall.Left),
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusSmall,
			radiusBorder:      cfg.RadiusMedium,
			radiusSubmenu:     cfg.RadiusSmall,
			radiusMenuItem:    cfg.RadiusSmall,
			Spacing:           cfg.SpacingMedium,
			spacingSubmenu:    cfg.SpacingTight,
			TextStyle:         ts,
			textStyleSubtitle: TextStyle{
				Color: ts.Color,
				Size:  ladder.small,
			},
		},
		datePickerStyle: DatePickerStyle{
			Shadow:            cfg.ShadowPopover,
			cellSpacing:       cfg.SpacingTight,
			Color:             cfg.ColorInterior,
			ColorHover:        cfg.ColorHover,
			ColorFocus:        cfg.ColorFocus,
			colorClick:        cfg.ColorActive,
			ColorBorder:       cfg.ColorBorder,
			ColorBorderFocus:  borderFocus,
			ColorSelect:       colorSelect,
			ColorTextOnSelect: colorTextOnSelect,
			Padding:           cfg.PaddingSmall.withSet(),
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusMedium,
			radiusBorder:      cfg.RadiusMedium,
			TextStyle:         ts,
		},
		colorPickerStyle: ColorPickerStyle{
			Color:            cfg.ColorInterior,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			sVSize:           200,
			sliderHeight:     24,
			indicatorSize:    16,
			TextStyle:        ts,
		},
		dataGridStyle: DataGridStyle{
			ColorBackground:        cfg.ColorInterior,
			ColorHeader:            cfg.ColorPanel,
			ColorHeaderHover:       cfg.ColorHover,
			ColorFilter:            cfg.ColorInterior,
			ColorQuickFilter:       cfg.ColorPanel,
			ColorRowHover:          cfg.ColorHover,
			ColorRowAlt:            ColorTransparent,
			ColorRowSelected:       colorSelect,
			ColorRowSelectedSubtle: accentSubtle,
			ColorBorder:            cfg.ColorBorder,
			ColorResizeHandle:      cfg.ColorBorder,
			ColorResizeActive:      colorSelect,
			PaddingCell:            PaddingTwoFive,
			PaddingHeader:          PaddingTwoFive,
			PaddingFilter:          PaddingNone,
			SizeBorder:             cfg.SizeBorder,
			Radius:                 cfg.RadiusSmall,
			TextStyle:              ts,
			TextStyleHeader: TextStyle{
				Color:    ts.Color,
				Size:     ts.Size,
				Typeface: glyph.TypefaceBold,
			},
			TextStyleFilter: ts,
		},
		inspectorStyle: inspectorStyleFor(textSecondary),

		// Layout constants.
		PaddingSmall:      cfg.PaddingSmall.withSet(),
		PaddingMedium:     cfg.PaddingMedium.withSet(),
		PaddingLarge:      cfg.PaddingLarge.withSet(),
		PaddingField:      fieldPad,
		SizeFieldMinWidth: cfg.SizeFieldMinWidth,
		SizeBorder:        cfg.SizeBorder,

		RadiusSmall:  cfg.RadiusSmall,
		RadiusMedium: cfg.RadiusMedium,
		RadiusLarge:  cfg.RadiusLarge,

		SpacingTight:  cfg.SpacingTight,
		SpacingSmall:  cfg.SpacingSmall,
		SpacingMedium: cfg.SpacingMedium,
		SpacingLarge:  cfg.SpacingLarge,

		SizeTextTiny:   ladder.tiny,
		SizeTextXSmall: ladder.xSmall,
		SizeTextSmall:  ladder.small,
		SizeTextMedium: ladder.medium,
		SizeTextLarge:  ladder.large,
		SizeTextXLarge: ladder.xLarge,

		ScrollMultiplier: cfg.ScrollMultiplier,
		ScrollDeltaLine:  cfg.ScrollDeltaLine,
		ScrollDeltaPage:  cfg.ScrollDeltaPage,
	}

	// Text size shortcuts.
	normal := ts
	bold := ts
	bold.Typeface = glyph.TypefaceBold
	theme.N1 = makeStyle(normal, theme.SizeTextXLarge)
	theme.N2 = makeStyle(normal, theme.SizeTextLarge)
	theme.N3 = ts
	theme.N4 = makeStyle(normal, theme.SizeTextSmall)
	theme.N5 = makeStyle(normal, theme.SizeTextXSmall)
	theme.N6 = makeStyle(normal, theme.SizeTextTiny)
	theme.B1 = makeStyle(bold, theme.SizeTextXLarge)
	theme.B2 = makeStyle(bold, theme.SizeTextLarge)
	theme.B3 = makeStyle(bold, theme.SizeTextMedium)
	theme.B4 = makeStyle(bold, theme.SizeTextSmall)
	theme.B5 = makeStyle(bold, theme.SizeTextXSmall)
	theme.B6 = makeStyle(bold, theme.SizeTextTiny)
	theme.tableStyle.TextStyleHead = theme.B3
	theme.badgeStyle.TextStyle = theme.B5
	theme.badgeStyle.TextStyle.Color = White

	// Heading roles take B rungs (visual-refresh §2.2): a widget
	// rendering a heading, a group-box title, a dialog title, a tab
	// label or a table header names a bold step; body and value text
	// stays N. TableStyle.TextStyleHead above and the DataGrid header
	// are the pre-existing members of the set; these three join it
	// here, where the rungs exist. The group-box title lives in
	// addGroupBoxTitle (gui/view_container.go), reading guiTheme.B3.
	theme.dialogStyle.titleTextStyle = theme.B2
	theme.toastStyle.TitleStyle = theme.B3
	// The selected tab carries the weight, the strip stays quiet.
	// It also fills with the accent, so its label draws in the
	// paired foreground (issue #373, visual-refresh §4.3) — the
	// literal above is overwritten here because the weight rungs
	// only exist after the styles are built.
	theme.tabControlStyle.textStyleSelected =
		textOnFill(theme.B3, true, theme.ColorTextOnSelect)

	// Italic shortcuts.
	italic := ts
	italic.Typeface = glyph.TypefaceItalic
	theme.I1 = makeStyle(italic, theme.SizeTextXLarge)
	theme.I2 = makeStyle(italic, theme.SizeTextLarge)
	theme.I3 = makeStyle(italic, theme.SizeTextMedium)
	theme.I4 = makeStyle(italic, theme.SizeTextSmall)
	theme.I5 = makeStyle(italic, theme.SizeTextXSmall)
	theme.I6 = makeStyle(italic, theme.SizeTextTiny)

	// Bold+italic shortcuts.
	boldItalic := ts
	boldItalic.Typeface = glyph.TypefaceBoldItalic
	theme.BI1 = makeStyle(boldItalic, theme.SizeTextXLarge)
	theme.BI2 = makeStyle(boldItalic, theme.SizeTextLarge)
	theme.BI3 = makeStyle(boldItalic, theme.SizeTextMedium)
	theme.BI4 = makeStyle(boldItalic, theme.SizeTextSmall)
	theme.BI5 = makeStyle(boldItalic, theme.SizeTextXSmall)
	theme.BI6 = makeStyle(boldItalic, theme.SizeTextTiny)

	// Mono shortcuts: +1 at every rung, so M4 (15) does not share N4's
	// (14) baseline. Mono faces typically draw optically smaller than
	// roman ones at the same point size; the offset is the compensation,
	// applied uniformly because it is the same face at every size. Apps
	// reading M-rungs beside N-rungs should expect the step.
	mono := ts
	mono.Family = cfg.MonoFontFamily
	theme.M1 = makeStyle(mono, theme.SizeTextXLarge+1)
	theme.M2 = makeStyle(mono, theme.SizeTextLarge+1)
	theme.M3 = makeStyle(mono, theme.SizeTextMedium+1)
	theme.M4 = makeStyle(mono, theme.SizeTextSmall+1)
	theme.M5 = makeStyle(mono, theme.SizeTextXSmall+1)
	theme.M6 = makeStyle(mono, theme.SizeTextTiny+1)

	// Icon font shortcuts.
	icon := ts
	icon.Family = iconFamily
	// Runs in this family are glyphs, not text: optical centring must
	// centre them on their own ink rather than on a cap band the face
	// does not really have.
	icon.glyphRole = true
	theme.Icon1 = makeStyle(icon, theme.SizeTextXLarge)
	theme.Icon2 = makeStyle(icon, theme.SizeTextLarge)
	theme.Icon3 = makeStyle(icon, theme.SizeTextMedium)
	theme.Icon4 = makeStyle(icon, theme.SizeTextSmall)
	theme.Icon5 = makeStyle(icon, theme.SizeTextXSmall)
	theme.Icon6 = makeStyle(icon, theme.SizeTextTiny)

	// Every theme leaving this constructor carries a fresh identity.
	// Install and cache-invalidation compare ids, never Name: two themes
	// may legitimately share a name and differ in styles.
	theme.id = nextThemeID()

	return theme
}
