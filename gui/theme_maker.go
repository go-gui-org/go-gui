package gui

import (
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

	borderFocus := cfg.ColorBorderFocus
	if borderFocus.Eq(Color{}) {
		borderFocus = cfg.ColorSelect
	}

	// Scrollbar radius: none if cfg.Radius is none.
	sbRadius := cfg.RadiusSmall
	if cfg.Radius == RadiusNone {
		sbRadius = RadiusNone
	}

	placeholderColor := RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 100)

	theme := Theme{
		Cfg:             cfg,
		Name:            cfg.Name,
		ColorBackground: cfg.ColorBackground,
		ColorPanel:      cfg.ColorPanel,
		ColorInterior:   cfg.ColorInterior,
		ColorHover:      cfg.ColorHover,
		ColorFocus:      cfg.ColorFocus,
		ColorActive:     cfg.ColorActive,
		ColorBorder:     cfg.ColorBorder,
		ColorSelect:     cfg.ColorSelect,
		TitlebarDark:    cfg.TitlebarDark,

		ButtonStyle: ButtonStyle{
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorActive,
			ColorClick:       cfg.ColorFocus,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          PaddingButton,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.Radius,
		},
		ContainerStyle: ContainerStyle{
			Color:       ColorTransparent,
			ColorBorder: ColorTransparent,
			Padding:     cfg.Padding,
			Radius:      cfg.Radius,
			Spacing:     cfg.SpacingMedium,
			SizeBorder:  cfg.SizeBorder,
		},
		RectangleStyle: RectangleStyle{
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
			ColorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          cfg.Padding,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.Radius,
			TextStyleNormal:  ts,
			PlaceholderStyle: TextStyle{
				Color: placeholderColor,
				Size:  ts.Size,
			},
			ColorSpellError: cfg.ColorError,
		},
		ScrollbarStyle: ScrollbarStyle{
			Size:            cfg.SizeScrollbar,
			MinThumbSize:    cfg.SizeScrollbarMin,
			ColorThumb:      cfg.ColorActive,
			ColorBackground: ColorTransparent,
			Radius:          sbRadius,
			RadiusThumb:     sbRadius,
			GapEdge:         3,
			GapEnd:          2,
		},
		RadioStyle: RadioStyle{
			Size:             cfg.SizeRadio,
			Color:            cfg.ColorPanel,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorSelect,
			ColorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			ColorUnselect:    cfg.ColorActive,
			Padding:          PadAll(4),
			SizeBorder:       cfg.SizeBorder,
			TextStyleNormal:  ts,
		},
		SwitchStyle: SwitchStyle{
			SizeWidth:        cfg.SizeSwitchWidth,
			SizeHeight:       cfg.SizeSwitchHeight,
			Color:            cfg.ColorPanel,
			ColorClick:       cfg.ColorInterior,
			ColorFocus:       cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			ColorUnselect:    cfg.ColorActive,
			Padding:          paddingThree,
			SizeBorder:       cfg.SizeBorder,
			Radius:           RadiusLarge * 2,
			TextStyleNormal:  ts,
		},
		ToggleStyle: ToggleStyle{
			Color:            cfg.ColorPanel,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorClick:       cfg.ColorInterior,
			ColorFocus:       cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorSelect:      cfg.ColorInterior,
			Padding:          NewPadding(1, 1, 1, 2),
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.Radius,
			TextStyleNormal:  ts,
			TextStyleLabel:   ts,
		},
		SelectStyle: SelectStyle{
			MinWidth:         75,
			MaxWidth:         200,
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorFocus,
			ColorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			Padding:          PaddingSmall,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			TextStyleNormal:  ts,
			SubheadingStyle:  ts,
			PlaceholderStyle: TextStyle{
				Color: placeholderColor,
				Size:  ts.Size,
			},
		},
		ListBoxStyle: ListBoxStyle{
			Color:           cfg.ColorInterior,
			ColorHover:      cfg.ColorHover,
			ColorBorder:     cfg.ColorBorder,
			ColorSelect:     cfg.ColorSelect,
			Padding:         cfg.Padding,
			SizeBorder:      cfg.SizeBorder,
			Radius:          cfg.Radius,
			TextStyleNormal: ts,
			SubheadingStyle: ts,
		},
		TreeStyle: TreeStyle{
			Color:       ColorTransparent,
			ColorHover:  cfg.ColorHover,
			ColorFocus:  cfg.ColorFocus,
			ColorBorder: ColorTransparent,
			Padding:     PaddingNone,
			SizeBorder:  cfg.SizeBorder,
			Radius:      cfg.Radius,
			TextStyle:   ts,
			TextStyleIcon: TextStyle{
				Color:  ts.Color,
				Size:   cfg.SizeTextSmall,
				Family: IconFontName,
			},
			Indent:  25,
			Spacing: 0,
		},
		DialogStyle: DialogStyle{
			Color:            cfg.ColorPanel,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          cfg.PaddingLarge,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			RadiusBorder:     cfg.RadiusMedium,
			AlignButtons:     HAlignCenter,
			MinWidth:         200,
			MaxWidth:         300,
			TitleTextStyle:   makeStyle(ts, cfg.SizeTextLarge),
			TextStyle:        ts,
		},
		ToastStyle: ToastStyle{
			MaxVisible:   5,
			Anchor:       ToastBottomRight,
			Width:        260,
			Margin:       16,
			Spacing:      8,
			AccentWidth:  4,
			Padding:      cfg.PaddingMedium,
			Radius:       cfg.RadiusMedium,
			SizeBorder:   cfg.SizeBorder,
			Color:        cfg.ColorPanel,
			ColorBorder:  cfg.ColorBorder,
			ColorInfo:    cfg.ColorSelect,
			ColorSuccess: RGBA(46, 160, 67, 255),
			ColorWarning: RGBA(210, 153, 34, 255),
			ColorError:   cfg.ColorError,
			TextStyle:    ts,
			TitleStyle:   makeStyle(ts, cfg.SizeTextMedium),
		},
		TooltipStyle: TooltipStyle{
			Delay:       500 * time.Millisecond,
			Color:       cfg.ColorInterior,
			ColorBorder: cfg.ColorBorder,
			Padding:     cfg.PaddingSmall,
			SizeBorder:  cfg.SizeBorder,
			Radius:      cfg.RadiusSmall,
			TextStyle:   ts,
		},
		BadgeStyle: BadgeStyle{
			Color:        cfg.ColorActive,
			ColorInfo:    cfg.ColorSelect,
			ColorSuccess: RGBA(46, 160, 67, 255),
			ColorWarning: RGBA(210, 153, 34, 255),
			ColorError:   cfg.ColorError,
			Padding:      NewPadding(2, 6, 2, 6),
			DotSize:      8,
		},
		ExpandPanelStyle: ExpandPanelStyle{
			Color:        cfg.ColorPanel,
			ColorHover:   cfg.ColorHover,
			ColorClick:   cfg.ColorActive,
			ColorBorder:  cfg.ColorBorder,
			Padding:      cfg.PaddingMedium,
			SizeBorder:   cfg.SizeBorder,
			Radius:       cfg.RadiusMedium,
			RadiusBorder: cfg.RadiusMedium,
		},
		ProgressBarStyle: ProgressBarStyle{
			Size:           cfg.SizeProgressBar,
			Color:          cfg.ColorInterior,
			ColorBar:       cfg.ColorSelect,
			ColorBorder:    cfg.ColorBorder,
			TextBackground: ColorTransparent,
			Padding:        PaddingNone,
			TextPadding:    NewPadding(1, 4, 1, 4),
			Radius:         cfg.RadiusSmall,
			TextShow:       true,
			TextStyle:      ts,
		},
		SkeletonStyle: SkeletonStyle{
			Color:          cfg.ColorInterior,
			ColorHighlight: cfg.ColorInterior.Add(RGBA(20, 20, 20, 0)),
			Radius:         cfg.RadiusSmall,
		},
		SliderStyle: SliderStyle{
			Size:             cfg.SizeSlider,
			ThumbSize:        cfg.SizeSliderThumb,
			Color:            cfg.ColorInterior,
			ColorClick:       cfg.ColorActive,
			ColorThumb:       cfg.ColorPanel,
			ColorLeft:        cfg.ColorSelect,
			ColorFocus:       cfg.ColorSelect,
			ColorHover:       cfg.ColorHover,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			Padding:          PaddingNone,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.SizeSlider / 2,
		},
		TabControlStyle: TabControlStyle{
			Color:               cfg.ColorPanel,
			ColorBorder:         cfg.ColorBorder,
			ColorHeader:         ColorTransparent,
			ColorHeaderBorder:   ColorTransparent,
			ColorContent:        cfg.ColorPanel,
			ColorContentBorder:  cfg.ColorBorder,
			ColorTab:            cfg.ColorInterior,
			ColorTabHover:       cfg.ColorHover,
			ColorTabFocus:       cfg.ColorFocus,
			ColorTabClick:       cfg.ColorActive,
			ColorTabSelected:    cfg.ColorSelect,
			ColorTabDisabled:    cfg.ColorPanel,
			ColorTabBorder:      cfg.ColorBorder,
			ColorTabBorderFocus: borderFocus,
			Padding:             PaddingNone,
			PaddingHeader:       PaddingNone,
			PaddingContent:      cfg.PaddingMedium,
			PaddingTab:          cfg.PaddingSmall,
			SizeBorder:          cfg.SizeBorder,
			SizeTabBorder:       cfg.SizeBorder,
			Radius:              cfg.RadiusMedium,
			RadiusHeader:        cfg.RadiusSmall,
			RadiusContent:       cfg.RadiusMedium,
			RadiusTab:           cfg.RadiusSmall,
			SpacingHeader:       2,
			TextStyle:           ts,
			TextStyleSelected:   ts,
			TextStyleDisabled: TextStyle{
				Color: RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 130),
				Size:  ts.Size,
			},
		},
		BreadcrumbStyle: BreadcrumbStyle{
			Separator:          "/",
			Color:              ColorTransparent,
			ColorBorder:        ColorTransparent,
			ColorTrail:         ColorTransparent,
			ColorCrumb:         ColorTransparent,
			ColorCrumbHover:    cfg.ColorHover,
			ColorCrumbClick:    cfg.ColorActive,
			ColorCrumbSelected: ColorTransparent,
			ColorCrumbDisabled: ColorTransparent,
			ColorContent:       cfg.ColorPanel,
			ColorContentBorder: cfg.ColorBorder,
			Padding:            PaddingNone,
			PaddingTrail:       cfg.PaddingSmall,
			PaddingCrumb:       NewPadding(2, 4, 2, 4),
			PaddingContent:     cfg.PaddingMedium,
			Radius:             cfg.RadiusMedium,
			RadiusCrumb:        cfg.RadiusSmall,
			RadiusContent:      cfg.RadiusMedium,
			Spacing:            cfg.SpacingSmall,
			SpacingTrail:       cfg.SpacingSmall,
			SizeContentBorder:  cfg.SizeBorder,
			TextStyle:          ts,
			TextStyleSelected:  ts,
			TextStyleDisabled: TextStyle{
				Color: RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 130),
				Size:  ts.Size,
			},
			TextStyleSeparator: TextStyle{
				Color: RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 160),
				Size:  ts.Size,
			},
		},
		SplitterStyle: SplitterStyle{
			HandleSize:        9,
			DragStep:          0.02,
			DragStepLarge:     0.10,
			ColorHandle:       cfg.ColorInterior,
			ColorHandleHover:  cfg.ColorHover,
			ColorHandleActive: cfg.ColorActive,
			ColorHandleBorder: cfg.ColorBorder,
			ColorGrip:         cfg.ColorSelect,
			ColorButton:       cfg.ColorInterior,
			ColorButtonHover:  cfg.ColorHover,
			ColorButtonActive: cfg.ColorActive,
			ColorButtonIcon:   ts.Color,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.RadiusSmall,
			RadiusBorder:      cfg.RadiusSmall,
		},
		TableStyle: TableStyle{
			ColorBorder:        cfg.ColorBorder,
			ColorSelect:        cfg.ColorSelect,
			ColorHover:         cfg.ColorHover,
			CellPadding:        PaddingTwoFive,
			TextStyle:          ts,
			TextStyleHead:      ts,
			AlignHead:          HAlignCenter,
			ColumnWidthDefault: 50,
			ColumnWidthMin:     20,
		},
		ComboboxStyle: ComboboxStyle{
			Color:             cfg.ColorInterior,
			ColorHover:        cfg.ColorHover,
			ColorFocus:        cfg.ColorInterior,
			ColorBorder:       cfg.ColorBorder,
			ColorBorderFocus:  borderFocus,
			ColorHighlight:    cfg.ColorSelect,
			Padding:           cfg.PaddingSmall,
			SizeBorder:        cfg.SizeBorder,
			Radius:            cfg.Radius,
			MinWidth:          75,
			MaxWidth:          200,
			MaxDropdownHeight: 200,
			TextStyle:         ts,
			PlaceholderStyle: TextStyle{
				Color: placeholderColor,
				Size:  ts.Size,
			},
		},
		CommandPaletteStyle: CommandPaletteStyle{
			Color:          cfg.ColorPanel,
			ColorBorder:    cfg.ColorBorder,
			ColorHighlight: cfg.ColorSelect,
			SizeBorder:     cfg.SizeBorder,
			Radius:         cfg.Radius,
			Width:          500,
			MaxHeight:      400,
			TextStyle:      ts,
			DetailStyle: TextStyle{
				Color: RGBA(ts.Color.R, ts.Color.G, ts.Color.B, 140),
				Size:  ts.Size,
			},
			BackdropColor: RGBA(0, 0, 0, 120),
		},
		MenubarStyle: MenubarStyle{
			WidthSubmenuMin:  50,
			WidthSubmenuMax:  200,
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorFocus,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			Padding:          cfg.PaddingSmall,
			PaddingMenuItem:  PaddingTwoFive,
			PaddingSubmenu:   cfg.PaddingSmall,
			PaddingSubtitle:  NewPadding(0, cfg.PaddingSmall.Right, 0, cfg.PaddingSmall.Left),
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusSmall,
			RadiusBorder:     cfg.RadiusMedium,
			RadiusSubmenu:    cfg.RadiusSmall,
			RadiusMenuItem:   cfg.RadiusSmall,
			Spacing:          cfg.SpacingMedium,
			SpacingSubmenu:   1,
			TextStyle:        ts,
			TextStyleSubtitle: TextStyle{
				Color: ts.Color,
				Size:  cfg.SizeTextSmall,
			},
		},
		DatePickerStyle: DatePickerStyle{
			CellSpacing:      2,
			Color:            cfg.ColorInterior,
			ColorHover:       cfg.ColorHover,
			ColorFocus:       cfg.ColorFocus,
			ColorClick:       cfg.ColorActive,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			ColorSelect:      cfg.ColorSelect,
			Padding:          cfg.PaddingSmall,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			RadiusBorder:     cfg.RadiusMedium,
			TextStyle:        ts,
		},
		ColorPickerStyle: ColorPickerStyle{
			Color:            cfg.ColorInterior,
			ColorBorder:      cfg.ColorBorder,
			ColorBorderFocus: borderFocus,
			SizeBorder:       cfg.SizeBorder,
			Radius:           cfg.RadiusMedium,
			SVSize:           200,
			SliderHeight:     24,
			IndicatorSize:    16,
			TextStyle:        ts,
		},
		DataGridStyle: DataGridStyle{
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
		InspectorStyle: DefaultInspectorStyle,

		// Layout constants.
		PaddingSmall:  cfg.PaddingSmall,
		PaddingMedium: cfg.PaddingMedium,
		PaddingLarge:  cfg.PaddingLarge,
		SizeBorder:    cfg.SizeBorder,

		RadiusSmall:  cfg.RadiusSmall,
		RadiusMedium: cfg.RadiusMedium,
		RadiusLarge:  cfg.RadiusLarge,

		SpacingSmall:  cfg.SpacingSmall,
		SpacingMedium: cfg.SpacingMedium,
		SpacingLarge:  cfg.SpacingLarge,

		SizeTextTiny:   cfg.SizeTextTiny,
		SizeTextXSmall: cfg.SizeTextXSmall,
		SizeTextSmall:  cfg.SizeTextSmall,
		SizeTextMedium: cfg.SizeTextMedium,
		SizeTextLarge:  cfg.SizeTextLarge,
		SizeTextXLarge: cfg.SizeTextXLarge,

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
	theme.TableStyle.TextStyleHead = theme.B3
	theme.BadgeStyle.TextStyle = theme.B5
	theme.BadgeStyle.TextStyle.Color = White

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

	// Mono shortcuts (+1 size offset).
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
	icon.Family = IconFontName
	theme.Icon1 = makeStyle(icon, theme.SizeTextXLarge)
	theme.Icon2 = makeStyle(icon, theme.SizeTextLarge)
	theme.Icon3 = makeStyle(icon, theme.SizeTextMedium)
	theme.Icon4 = makeStyle(icon, theme.SizeTextSmall)
	theme.Icon5 = makeStyle(icon, theme.SizeTextXSmall)
	theme.Icon6 = makeStyle(icon, theme.SizeTextTiny)

	return theme
}
