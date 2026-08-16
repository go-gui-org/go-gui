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

	// Icon family for every theme-driven icon style. A ThemeCfg built
	// from scratch (not via baseCfg) leaves this empty, so fall back to
	// the bundled font rather than render icons in the default family.
	iconFamily := cmp.Or(cfg.iconFontFamily, IconFontName)

	borderFocus := cfg.ColorBorderFocus
	if borderFocus.eq(Color{}) {
		borderFocus = cfg.ColorSelect
	}

	// Separator role: a divider is an edge, and reusing ColorBorder
	// would couple the two forever. Unset falls back to the border
	// color so existing themes keep their look without restating it.
	colorSeparator := cfg.ColorSeparator
	if !colorSeparator.IsSet() {
		colorSeparator = cfg.ColorBorder
	}
	sizeSeparator := cfg.sizeSeparator
	if sizeSeparator == 0 {
		sizeSeparator = 1
	}

	// Scrollbar radius: none if cfg.Radius is none.
	sbRadius := cfg.RadiusSmall
	if cfg.Radius == radiusNone {
		sbRadius = radiusNone
	}

	// Named text roles. Every de-emphasized style below draws from
	// these rather than restating an alpha (issue #335).
	textSecondary, textLabel, textDisabled, textPlaceholder :=
		themeTextRoles(cfg, ts, cfg.sizeTextXSmall)

	theme := Theme{
		Cfg:                  cfg,
		Name:                 cfg.Name,
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
		ColorSelect:          cfg.ColorSelect,
		TitlebarDark:         cfg.TitlebarDark,

		ButtonStyle: buttonStyle{
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorActive,
			colorClick:       cfg.ColorFocus,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          paddingButton,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.Radius,
		},
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
			Padding:          cfg.Padding,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.Radius,
			textStyleNormal:  ts,
			PlaceholderStyle: textPlaceholder,
			colorSpellError:  cfg.ColorError,
		},
		ScrollbarStyle: ScrollbarStyle{
			Size:            cfg.sizeScrollbar,
			minThumbSize:    cfg.sizeScrollbarMin,
			colorThumb:      cfg.ColorActive,
			ColorBackground: ColorTransparent,
			Radius:          sbRadius,
			radiusThumb:     sbRadius,
			GapEdge:         3,
			GapEnd:          2,
		},
		radioStyle: RadioStyle{
			Size:             cfg.sizeRadio,
			Color:            cfg.ColorPanel,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorSelect,
			colorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			colorUnselect:    cfg.ColorActive,
			Padding:          PadAll(4),
			SizeBorder:       cfg.SizeBorder,
			textStyleNormal:  ts,
		},
		switchStyle: SwitchStyle{
			sizeWidth:        cfg.sizeSwitchWidth,
			sizeHeight:       cfg.sizeSwitchHeight,
			Color:            cfg.ColorPanel,
			colorClick:       cfg.ColorInterior,
			ColorFocus:       cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
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
			MinWidth:         75,
			MaxWidth:         200,
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorFocus,
			colorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			Padding:          PaddingSmall,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			textStyleNormal:  ts,
			subheadingStyle:  ts,
			PlaceholderStyle: textPlaceholder,
		},
		listBoxStyle: ListBoxStyle{
			Color:           cfg.ColorInterior,
			ColorHover:      cfg.ColorHover,
			ColorBorder:     cfg.ColorBorder,
			ColorSelect:     cfg.ColorSelect,
			Padding:         cfg.Padding,
			SizeBorder:      cfg.SizeBorder,
			Radius:          cfg.Radius,
			textStyleNormal: ts,
			subheadingStyle: ts,
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
				Color:  ts.Color,
				Size:   cfg.sizeTextSmall,
				Family: iconFamily,
			},
			indent:  25,
			Spacing: 0,
		},
		dialogStyle: DialogStyle{
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
			titleTextStyle:   makeStyle(ts, cfg.sizeTextLarge),
			TextStyle:        ts,
		},
		toastStyle: ToastStyle{
			maxVisible:   5,
			Anchor:       toastBottomRight,
			Width:        260,
			margin:       16,
			Spacing:      8,
			accentWidth:  4,
			Padding:      cfg.PaddingMedium.withSet(),
			Radius:       cfg.RadiusMedium,
			SizeBorder:   cfg.SizeBorder,
			Color:        cfg.ColorPanel,
			ColorBorder:  cfg.ColorBorder,
			colorInfo:    cfg.ColorSelect,
			ColorSuccess: RGBA(46, 160, 67, 255),
			ColorWarning: RGBA(210, 153, 34, 255),
			ColorError:   cfg.ColorError,
			TextStyle:    ts,
			TitleStyle:   makeStyle(ts, cfg.sizeTextMedium),
		},
		tooltipStyle: TooltipStyle{
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
			colorInfo:    cfg.ColorSelect,
			ColorSuccess: RGBA(46, 160, 67, 255),
			ColorWarning: RGBA(210, 153, 34, 255),
			ColorError:   cfg.ColorError,
			Padding:      NewPadding(2, 6, 2, 6),
			dotSize:      8,
		},
		expandPanelStyle: ExpandPanelStyle{
			Color:        cfg.ColorPanel,
			ColorHover:   cfg.ColorHover,
			colorClick:   cfg.ColorActive,
			ColorBorder:  cfg.ColorBorder,
			Padding:      cfg.PaddingMedium.withSet(),
			SizeBorder:   cfg.SizeBorder,
			Radius:       cfg.RadiusMedium,
			radiusBorder: cfg.RadiusMedium,
		},
		progressBarStyle: ProgressBarStyle{
			Size:           cfg.sizeProgressBar,
			Color:          cfg.ColorInterior,
			colorBar:       cfg.ColorSelect,
			ColorBorder:    cfg.ColorBorder,
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
			Size:             cfg.sizeSlider,
			ThumbSize:        cfg.sizeSliderThumb,
			Color:            cfg.ColorInterior,
			colorClick:       cfg.ColorActive,
			colorThumb:       cfg.ColorPanel,
			colorLeft:        cfg.ColorSelect,
			ColorFocus:       cfg.ColorSelect,
			ColorHover:       cfg.ColorHover,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          PaddingNone,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.sizeSlider / 2,
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
			colorTabSelected:    cfg.ColorSelect,
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
			spacingHeader:       2,
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
			colorGrip:         cfg.ColorSelect,
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
			ColorSelect:        cfg.ColorSelect,
			ColorHover:         cfg.ColorHover,
			cellPadding:        PaddingTwoFive,
			TextStyle:          ts,
			TextStyleHead:      ts,
			alignHead:          HAlignCenter,
			columnWidthDefault: 50,
			columnWidthMin:     20,
		},
		comboboxStyle: ComboboxStyle{
			Color:             cfg.ColorInterior,
			ColorHover:        cfg.ColorHover,
			ColorFocus:        cfg.ColorInterior,
			ColorBorder:       cfg.ColorBorder,
			ColorBorderFocus:  borderFocus,
			ColorHighlight:    cfg.ColorSelect,
			Padding:           cfg.PaddingSmall.withSet(),
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.Radius,
			MinWidth:          75,
			MaxWidth:          200,
			maxDropdownHeight: 200,
			TextStyle:         ts,
			PlaceholderStyle:  textPlaceholder,
		},
		commandPaletteStyle: CommandPaletteStyle{
			Color:          cfg.ColorPanel,
			ColorBorder:    cfg.ColorBorder,
			ColorHighlight: cfg.ColorSelect,
			SizeBorder:     cfg.SizeBorder,
			Radius:         cfg.Radius,
			Width:          500,
			MaxHeight:      400,
			TextStyle:      ts,
			detailStyle:    textSecondary,
			backdropColor:  RGBA(0, 0, 0, 120),
		},
		MenubarStyle: MenubarStyle{
			widthSubmenuMin:  50,
			widthSubmenuMax:  200,
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorFocus,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			Padding:          cfg.PaddingSmall.withSet(),
			paddingMenuItem:  PaddingTwoFive,
			paddingSubmenu:   cfg.PaddingSmall.withSet(),
			paddingSubtitle:  NewPadding(0, cfg.PaddingSmall.Right, 0, cfg.PaddingSmall.Left),
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusSmall,
			radiusBorder:     cfg.RadiusMedium,
			radiusSubmenu:    cfg.RadiusSmall,
			radiusMenuItem:   cfg.RadiusSmall,
			Spacing:          cfg.SpacingMedium,
			spacingSubmenu:   1,
			TextStyle:        ts,
			textStyleSubtitle: TextStyle{
				Color: ts.Color,
				Size:  cfg.sizeTextSmall,
			},
		},
		datePickerStyle: DatePickerStyle{
			cellSpacing:      2,
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorFocus,
			colorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			Padding:          cfg.PaddingSmall.withSet(),
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			radiusBorder:     cfg.RadiusMedium,
			TextStyle:        ts,
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
			ColorBackground:   cfg.ColorInterior,
			ColorHeader:       cfg.ColorPanel,
			ColorHeaderHover:  cfg.ColorHover,
			ColorFilter:       cfg.ColorInterior,
			ColorQuickFilter:  cfg.ColorPanel,
			ColorRowHover:     cfg.ColorHover,
			ColorRowAlt:       ColorTransparent,
			ColorRowSelected:  cfg.ColorSelect,
			ColorBorder:       cfg.ColorBorder,
			ColorResizeHandle: cfg.ColorBorder,
			ColorResizeActive: cfg.ColorSelect,
			PaddingCell:       PaddingTwoFive,
			PaddingHeader:     PaddingTwoFive,
			PaddingFilter:     PaddingNone,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusSmall,
			TextStyle:         ts,
			TextStyleHeader: TextStyle{
				Color:    ts.Color,
				Size:     ts.Size,
				Typeface: glyph.TypefaceBold,
			},
			TextStyleFilter: ts,
		},
		inspectorStyle: inspectorStyleFor(textSecondary),

		// Layout constants.
		PaddingSmall:  cfg.PaddingSmall.withSet(),
		PaddingMedium: cfg.PaddingMedium.withSet(),
		PaddingLarge:  cfg.PaddingLarge.withSet(),
		SizeBorder:    cfg.SizeBorder,

		RadiusSmall:  cfg.RadiusSmall,
		RadiusMedium: cfg.RadiusMedium,
		RadiusLarge:  cfg.RadiusLarge,

		SpacingSmall:  cfg.SpacingSmall,
		SpacingMedium: cfg.SpacingMedium,
		SpacingLarge:  cfg.SpacingLarge,

		SizeTextTiny:   cfg.SizeTextTiny,
		sizeTextXSmall: cfg.sizeTextXSmall,
		sizeTextSmall:  cfg.sizeTextSmall,
		sizeTextMedium: cfg.sizeTextMedium,
		sizeTextLarge:  cfg.sizeTextLarge,
		sizeTextXLarge: cfg.sizeTextXLarge,

		scrollMultiplier: cfg.scrollMultiplier,
		scrollDeltaLine:  cfg.scrollDeltaLine,
		scrollDeltaPage:  cfg.scrollDeltaPage,
	}

	// Text size shortcuts.
	normal := ts
	bold := ts
	bold.Typeface = glyph.TypefaceBold
	theme.N1 = makeStyle(normal, theme.sizeTextXLarge)
	theme.N2 = makeStyle(normal, theme.sizeTextLarge)
	theme.N3 = ts
	theme.N4 = makeStyle(normal, theme.sizeTextSmall)
	theme.N5 = makeStyle(normal, theme.sizeTextXSmall)
	theme.N6 = makeStyle(normal, theme.SizeTextTiny)
	theme.B1 = makeStyle(bold, theme.sizeTextXLarge)
	theme.B2 = makeStyle(bold, theme.sizeTextLarge)
	theme.B3 = makeStyle(bold, theme.sizeTextMedium)
	theme.B4 = makeStyle(bold, theme.sizeTextSmall)
	theme.B5 = makeStyle(bold, theme.sizeTextXSmall)
	theme.B6 = makeStyle(bold, theme.SizeTextTiny)
	theme.tableStyle.TextStyleHead = theme.B3
	theme.badgeStyle.TextStyle = theme.B5
	theme.badgeStyle.TextStyle.Color = White

	// Italic shortcuts.
	italic := ts
	italic.Typeface = glyph.TypefaceItalic
	theme.i1 = makeStyle(italic, theme.sizeTextXLarge)
	theme.i2 = makeStyle(italic, theme.sizeTextLarge)
	theme.I3 = makeStyle(italic, theme.sizeTextMedium)
	theme.i4 = makeStyle(italic, theme.sizeTextSmall)
	theme.i5 = makeStyle(italic, theme.sizeTextXSmall)
	theme.i6 = makeStyle(italic, theme.SizeTextTiny)

	// Bold+italic shortcuts.
	boldItalic := ts
	boldItalic.Typeface = glyph.TypefaceBoldItalic
	theme.bI1 = makeStyle(boldItalic, theme.sizeTextXLarge)
	theme.bI2 = makeStyle(boldItalic, theme.sizeTextLarge)
	theme.BI3 = makeStyle(boldItalic, theme.sizeTextMedium)
	theme.bI4 = makeStyle(boldItalic, theme.sizeTextSmall)
	theme.bI5 = makeStyle(boldItalic, theme.sizeTextXSmall)
	theme.bI6 = makeStyle(boldItalic, theme.SizeTextTiny)

	// Mono shortcuts (+1 size offset).
	mono := ts
	mono.Family = cfg.monoFontFamily
	theme.M1 = makeStyle(mono, theme.sizeTextXLarge+1)
	theme.M2 = makeStyle(mono, theme.sizeTextLarge+1)
	theme.M3 = makeStyle(mono, theme.sizeTextMedium+1)
	theme.M4 = makeStyle(mono, theme.sizeTextSmall+1)
	theme.M5 = makeStyle(mono, theme.sizeTextXSmall+1)
	theme.M6 = makeStyle(mono, theme.SizeTextTiny+1)

	// Icon font shortcuts.
	icon := ts
	icon.Family = iconFamily
	theme.Icon1 = makeStyle(icon, theme.sizeTextXLarge)
	theme.Icon2 = makeStyle(icon, theme.sizeTextLarge)
	theme.Icon3 = makeStyle(icon, theme.sizeTextMedium)
	theme.Icon4 = makeStyle(icon, theme.sizeTextSmall)
	theme.icon5 = makeStyle(icon, theme.sizeTextXSmall)
	theme.icon6 = makeStyle(icon, theme.SizeTextTiny)

	// Every theme leaving this constructor carries a fresh identity.
	// Install and cache-invalidation compare ids, never Name: two themes
	// may legitimately share a name and differ in styles.
	theme.id = nextThemeID()

	return theme
}
