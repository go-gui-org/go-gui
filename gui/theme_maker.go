package gui

import (
	"cmp"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// resolveTextLadder resolves a ThemeCfg's text size ladder to concrete
// values. The body size is the per-theme decision (visual-refresh
// §2.1): the ladder derives from it, so a theme that states
// TextStyleDef.Size alone gets a complete ladder and a theme that seeds
// explicit rungs keeps them. Zero takes the derived value, which is
// what "Zero takes the built-in defaults" on ThemeCfg.SizeText*
// promises.
//
// Shared with AdjustFontSize, which tunes the rungs and so has to know
// what they currently are: the Cfg's own fields are still zero on a
// theme that never stated them, and adding a delta to zero is not a
// tune, it is a collapse.
func resolveTextLadder(cfg ThemeCfg) textSizeLadder {
	body := cfg.TextStyleDef.Size
	if body <= 0 {
		// Zero TextStyleDef.Size is "unset", not "0px text": fall back
		// to the built-in body so a partial theme never derives a
		// negative ladder.
		body = sizeTextMedium
	}
	derived := textSizes(body)
	return textSizeLadder{
		tiny:   cmp.Or(cfg.SizeTextTiny, derived.tiny),
		xSmall: cmp.Or(cfg.SizeTextXSmall, derived.xSmall),
		small:  cmp.Or(cfg.SizeTextSmall, derived.small),
		medium: cmp.Or(cfg.SizeTextMedium, derived.medium),
		large:  cmp.Or(cfg.SizeTextLarge, derived.large),
		xLarge: cmp.Or(cfg.SizeTextXLarge, derived.xLarge),
	}
}

// ThemeMaker builds a full Theme from a ThemeCfg.
func ThemeMaker(cfg ThemeCfg) Theme {
	ts := cfg.TextStyleDef

	ladder := resolveTextLadder(cfg)

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

	// OKLCH offsets are absolute on L, not relative: a relative step
	// collapses to nothing on a dark accent and overshoots on a light
	// one. Clamped to [0,1]; the derivation is pinned against the
	// spec table by TestThemeMakerAccentRamp.
	accentHover := cfg.ColorAccentHover
	if !accentHover.IsSet() {
		accentHover = oklchShift(accent, oklchRampDelta)
	}
	accentPressed := cfg.ColorAccentPressed
	if !accentPressed.IsSet() {
		accentPressed = oklchShift(accent, -oklchRampDelta)
	}
	accentSubtle := cfg.ColorAccentSubtle
	if !accentSubtle.IsSet() {
		accentSubtle = subtleFor(accent, ts.Color, cfg.ColorBackground)
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
		successSubtle = subtleFor(colorSuccess, ts.Color, cfg.ColorBackground)
	}
	warningSubtle := cfg.ColorWarningSubtle
	if !warningSubtle.IsSet() {
		warningSubtle = subtleFor(colorWarning, ts.Color, cfg.ColorBackground)
	}
	errorSubtle := cfg.ColorErrorSubtle
	if !errorSubtle.IsSet() {
		errorSubtle = subtleFor(colorError, ts.Color, cfg.ColorBackground)
	}

	borderFocus := cfg.ColorBorderFocus
	// Unset means "not specified" (Color carries its own set flag),
	// so an explicit fully-transparent focus border stays honorable
	// instead of falling back to the select color.
	if !borderFocus.IsSet() {
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

	// Elevation and focus pointers are per-theme values; isolate the
	// local cfg so the styles and the stored theme.Cfg share copies
	// nothing else can write through.
	isolateThemeShadows(&cfg)

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

		buttonStyle:        buttonBase,
		buttonStylePrimary: buttonPrimary,
		buttonStyleGhost:   buttonGhost,
		buttonStyleDanger:  buttonDanger,
		containerStyle: containerStyle{
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
		inputStyle: InputStyle{
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorActive,
				Focus:       cfg.ColorInterior,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
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
			GapEdge:         cfg.SizeScrollbarGap.Get(scrollbarGapEdge),
			GapEnd:          cfg.SizeScrollbarGapEnd.Get(scrollbarGapEnd),
		},
		radioStyle: RadioStyle{
			Size: cfg.SizeRadio,
			Colors: ColorSet{
				Base:        cfg.ColorPanel,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorActive,
				Focus:       colorSelect,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			ColorSelect:     colorSelect,
			colorUnselect:   cfg.ColorActive,
			Padding:         PadAll(4),
			SizeBorder:      cfg.SizeBorder,
			textStyleNormal: ts,
			textStyleLabel:  ts,
		},
		switchStyle: SwitchStyle{
			sizeWidth:  cfg.SizeSwitchWidth,
			sizeHeight: cfg.SizeSwitchHeight,
			Colors: ColorSet{
				Base:        cfg.ColorPanel,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorInterior,
				Focus:       cfg.ColorInterior,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			ColorSelect:     colorSelect,
			colorUnselect:   cfg.ColorActive,
			Padding:         paddingThree,
			SizeBorder:      cfg.SizeBorder,
			Radius:          radiusLarge * 2,
			textStyleNormal: ts,
			textStyleLabel:  ts,
		},
		toggleStyle: ToggleStyle{
			Colors: ColorSet{
				Base:        cfg.ColorPanel,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorInterior,
				Focus:       cfg.ColorInterior,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			ColorSelect: cfg.ColorInterior,
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
			MinWidth: cfg.SizeFieldMinWidth,
			MaxWidth: 0,
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorActive,
				Focus:       cfg.ColorFocus,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
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
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			ColorSelect:       colorSelect,
			ColorSelectSubtle: accentSubtle,
			Padding:           cfg.Padding,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.Radius,
			textStyleNormal:   ts,
			subheadingStyle:   ts,
		},
		treeStyle: TreeStyle{
			Colors: ColorSet{
				Base:   ColorTransparent,
				Hover:  cfg.ColorHover,
				Focus:  cfg.ColorFocus,
				Border: ColorTransparent,
			},
			Padding:    PaddingNone,
			SizeBorder: cfg.SizeBorder,
			Radius:     cfg.Radius,
			TextStyle:  ts,
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
			Shadow: cfg.ShadowDialog,
			Colors: ColorSet{
				Base:        cfg.ColorPanel,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			Padding:      cfg.PaddingLarge.withSet(),
			SizeBorder:   cfg.SizeBorder,
			Radius:       cfg.RadiusMedium,
			radiusBorder: cfg.RadiusMedium,
			AlignButtons: HAlignCenter,
			MinWidth:     200,
			MaxWidth:     300,
			// titleTextStyle is a heading role, assigned from the B2
			// rung below once the rungs exist. Seeding it here would
			// read cfg.SizeTextLarge raw, bypassing the ladder that
			// gives an unset cfg a complete scale.
			TextStyle: ts,
		},
		toastStyle: ToastStyle{
			Shadow:       cfg.ShadowPopover,
			maxVisible:   5,
			Anchor:       ToastBottomRight,
			Width:        260,
			margin:       16,
			Spacing:      cfg.SpacingMedium,
			accentWidth:  4,
			Padding:      cfg.PaddingMedium.withSet(),
			Radius:       cfg.RadiusMedium,
			SizeBorder:   cfg.SizeBorder,
			Colors:       ColorSet{Base: cfg.ColorPanel, Border: cfg.ColorBorder},
			colorInfo:    colorSelect,
			ColorSuccess: colorSuccess,
			ColorWarning: colorWarning,
			ColorError:   colorError,
			TextStyle:    ts,
			// TitleStyle takes the B3 rung below, for the same reason
			// dialogStyle.titleTextStyle does.
		},
		tooltipStyle: TooltipStyle{
			Shadow:     cfg.ShadowPopover,
			Delay:      500 * time.Millisecond,
			Colors:     ColorSet{Base: cfg.ColorInterior, Border: cfg.ColorBorder},
			Padding:    cfg.PaddingSmall.withSet(),
			SizeBorder: cfg.SizeBorder,
			Radius:     cfg.RadiusSmall,
			TextStyle:  ts,
		},
		badgeStyle: BadgeStyle{
			Colors:       ColorSet{Base: cfg.ColorActive},
			colorInfo:    colorSelect,
			ColorSuccess: colorSuccess,
			ColorWarning: colorWarning,
			ColorError:   colorError,
			Padding:      NewPadding(2, 6, 2, 6),
			dotSize:      8,
		},
		expandPanelStyle: ExpandPanelStyle{
			Colors: ColorSet{
				Base:        cfg.ColorPanel,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorActive,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			Padding:      cfg.PaddingMedium.withSet(),
			SizeBorder:   cfg.SizeBorder,
			Radius:       cfg.RadiusMedium,
			radiusBorder: cfg.RadiusMedium,
		},
		progressBarStyle: ProgressBarStyle{
			Size:     cfg.SizeProgressBar,
			Colors:   ColorSet{Base: cfg.ColorInterior, Border: cfg.ColorBorder},
			colorBar: colorSelect,
			// The readout trails the bar unboxed (visual-refresh §8):
			// it no longer straddles fill and track, so it takes the
			// secondary role outright.
			textBackground: ColorTransparent,
			Padding:        PaddingNone,
			textPadding:    NewPadding(1, 4, 1, 4),
			Radius:         cfg.RadiusSmall,
			TextShow:       true,
			TextStyle:      textSecondary,
		},
		skeletonStyle: SkeletonStyle{
			Colors:         ColorSet{Base: cfg.ColorInterior},
			ColorHighlight: cfg.ColorInterior.Add(RGBA(20, 20, 20, 0)),
			Radius:         cfg.RadiusSmall,
		},
		separatorStyle: SeparatorStyle{
			Colors: ColorSet{Base: colorSeparator},
			Size:   sizeSeparator,
		},
		sliderStyle: SliderStyle{
			Size:      cfg.SizeSlider,
			ThumbSize: cfg.SizeSliderThumb,
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorActive,
				Focus:       colorSelect,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			colorThumb: cfg.ColorPanel,
			colorLeft:  colorSelect,
			Padding:    PaddingNone,
			SizeBorder: cfg.SizeBorder,
			Radius:     cfg.SizeSlider / 2,
		},
		tabControlStyle: TabControlStyle{
			Colors:             ColorSet{Base: cfg.ColorPanel, Border: cfg.ColorBorder},
			ColorHeader:        ColorTransparent,
			colorHeaderBorder:  ColorTransparent,
			colorContent:       cfg.ColorPanel,
			colorContentBorder: cfg.ColorBorder,
			ColorsTab: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Focus:       cfg.ColorFocus,
				Click:       cfg.ColorActive,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			colorTabSelected:  colorSelect,
			colorTabDisabled:  cfg.ColorPanel,
			Padding:           PaddingNone,
			PaddingHeader:     PaddingNone,
			paddingContent:    cfg.PaddingMedium.withSet(),
			paddingTab:        cfg.PaddingSmall.withSet(),
			SizeBorder:        cfg.SizeBorder,
			sizeTabBorder:     cfg.SizeBorder,
			Radius:            cfg.RadiusMedium,
			radiusHeader:      cfg.RadiusSmall,
			radiusContent:     cfg.RadiusMedium,
			radiusTab:         cfg.RadiusSmall,
			spacingHeader:     cfg.SpacingTight,
			TextStyle:         ts,
			textStyleSelected: ts,
			textStyleDisabled: textDisabled,
		},
		breadcrumbStyle: BreadcrumbStyle{
			Separator:  "/",
			Colors:     ColorSet{Base: ColorTransparent, Border: ColorTransparent},
			colorTrail: ColorTransparent,
			ColorsCrumb: ColorSet{
				Base:  ColorTransparent,
				Hover: cfg.ColorHover,
				Click: cfg.ColorActive,
			},
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
			HandleSize:    9,
			dragStep:      0.02,
			dragStepLarge: 0.10,
			ColorsHandle: ColorSet{
				Base:   cfg.ColorInterior,
				Hover:  cfg.ColorHover,
				Click:  cfg.ColorActive,
				Border: cfg.ColorBorder,
			},
			colorGrip: colorSelect,
			ColorsButton: ColorSet{
				Base:  cfg.ColorInterior,
				Hover: cfg.ColorHover,
				Click: cfg.ColorActive,
				Focus: cfg.ColorHover,
			},
			colorButtonIcon: ts.Color,
			SizeBorder:      cfg.SizeBorder,
			Radius:          cfg.RadiusSmall,
			radiusBorder:    cfg.RadiusSmall,
		},
		tableStyle: TableStyle{
			Colors: ColorSet{
				Hover:       cfg.ColorHover,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			ColorSelect:        colorSelect,
			ColorSelectSubtle:  accentSubtle,
			cellPadding:        PaddingTwoFive,
			TextStyle:          ts,
			TextStyleHead:      ts,
			alignHead:          HAlignCenter,
			columnWidthDefault: 50,
			columnWidthMin:     20,
		},
		comboboxStyle: ComboboxStyle{
			Shadow: cfg.ShadowPopover,
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Focus:       cfg.ColorInterior,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
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
			Colors:               ColorSet{Base: cfg.ColorPanel, Border: cfg.ColorBorder},
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
		menubarStyle: MenubarStyle{
			Shadow:          cfg.ShadowPopover,
			widthSubmenuMin: 50,
			widthSubmenuMax: 200,
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Focus:       cfg.ColorFocus,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
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
			Shadow:      cfg.ShadowPopover,
			cellSpacing: cfg.SpacingTight,
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Hover:       cfg.ColorHover,
				Click:       cfg.ColorActive,
				Focus:       cfg.ColorFocus,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			ColorSelect:       colorSelect,
			ColorTextOnSelect: colorTextOnSelect,
			Padding:           cfg.PaddingSmall.withSet(),
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusMedium,
			radiusBorder:      cfg.RadiusMedium,
			TextStyle:         ts,
		},
		colorPickerStyle: ColorPickerStyle{
			Colors: ColorSet{
				Base:        cfg.ColorInterior,
				Border:      cfg.ColorBorder,
				BorderFocus: borderFocus,
			},
			SizeBorder:    cfg.SizeBorder,
			Radius:        cfg.RadiusMedium,
			sVSize:        200,
			sliderHeight:  24,
			indicatorSize: 16,
			TextStyle:     ts,
		},
		dataGridStyle: DataGridStyle{
			ColorBackground:        cfg.ColorInterior,
			ColorsHeader:           ColorSet{Base: cfg.ColorPanel, Hover: cfg.ColorHover},
			ColorFilter:            cfg.ColorInterior,
			ColorQuickFilter:       cfg.ColorPanel,
			ColorsRow:              ColorSet{Hover: cfg.ColorHover, Border: cfg.ColorBorder},
			ColorRowAlt:            ColorTransparent,
			ColorRowSelected:       colorSelect,
			ColorRowSelectedSubtle: accentSubtle,
			ColorsResize:           ColorSet{Base: cfg.ColorBorder, Click: colorSelect},
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

		Sounds: cfg.Sounds,

		ScrollMultiplier: cfg.ScrollMultiplier,
		ScrollDeltaLine:  cfg.ScrollDeltaLine,
		ScrollDeltaPage:  cfg.ScrollDeltaPage,
	}

	// Text rungs, heading roles and the styles that pair with a fill.
	// Own file so theme_maker.go stays under the large-files gate.
	theme.fillTextRungs(cfg, ts, iconFamily)

	// Every theme leaving this constructor carries a fresh identity.
	// Install and cache-invalidation compare ids, never Name: two themes
	// may legitimately share a name and differ in styles.
	theme.id = nextThemeID()

	return theme
}
