package gui

import "github.com/go-gui-org/go-glyph"

// fillTextRungs fills the closed 6x6 text-size grid (N/B/I/BI/M/Icon)
// and the styles that can only be resolved once the rungs exist: the
// heading roles, and the two labels that draw over a fill.
//
// Split out of ThemeMaker, which the large-files gate keeps under 800
// lines. Everything here reads sizes already resolved onto theme, so
// the order matters: rungs first, then the roles naming them.
func (theme *Theme) fillTextRungs(
	cfg ThemeCfg, ts TextStyle, iconFamily string,
) {
	makeStyle := func(base TextStyle, size float32) TextStyle {
		s := base
		s.Size = size
		return s
	}

	// Text size shortcuts.
	normal := ts
	bold := ts
	bold.Typeface = glyph.TypefaceBold
	theme.N1 = makeStyle(normal, theme.SizeTextXLarge)
	theme.N2 = makeStyle(normal, theme.SizeTextLarge)
	// Rung 3 off the ladder, not off the body style directly. The two
	// agree on every preset (baseCfg states SizeTextMedium and the body
	// size as the same 14) and on any theme that states one of them,
	// since the ladder derives the medium rung from the body. They part
	// only when a Cfg states both and disagrees — and taking ts there
	// left N3 at the body size while B3, I3, M3 and BI3 took the
	// ladder, so a bold word inside a sentence rendered at a different
	// size from the sentence, and a table header (B3) outgrew the rows
	// it headed.
	theme.N3 = makeStyle(normal, theme.SizeTextMedium)
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
	// The fill and the text on it are one decision (issue #373).
	// A literal White here was polarity-blind: every light preset
	// fills a neutral badge with ColorActive, a near-white, and
	// drew a white label on it at contrast 1.26-1.32. Badge
	// re-pairs per variant and per caller fill.
	theme.badgeStyle.TextStyle.Color = textOnFor(theme.badgeStyle.Colors.Base)

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

}
