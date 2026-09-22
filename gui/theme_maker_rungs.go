package gui

import "github.com/go-gui-org/go-glyph"

// fillTextRungs fills the semantic text roles (issue #734) and the
// styles that can only be resolved once the roles exist: the heading
// roles, and the two labels that draw over a fill.
//
// Split out of ThemeMaker, which the large-files gate keeps under 800
// lines. Everything here reads sizes already resolved onto theme, so
// the order matters: roles first, then the styles naming them.
func (theme *Theme) fillTextRungs(
	cfg ThemeCfg, ts TextStyle, iconFamily string,
) {
	makeStyle := func(base TextStyle, size float32) TextStyle {
		s := base
		s.Size = size
		return s
	}

	normal := ts
	bold := ts
	bold.Typeface = glyph.TypefaceBold

	// Display and TitleSmall come off the ladder, not off a body
	// style directly. The two agree on every preset (baseCfg states
	// SizeTextMedium and the body size as the same 14) and on any
	// theme that states one of them, since the ladder derives the
	// medium rung from the body. They part only when a Cfg states
	// both and disagrees — and taking ts there left Body at the body
	// size while TitleSmall took the ladder, so a bold word inside a
	// sentence rendered at a different size from the sentence, and a
	// table header outgrew the rows it headed.
	theme.TextStyleDisplay = makeStyle(bold, theme.SizeTextXLarge)
	theme.TextStyleTitle = makeStyle(bold, theme.SizeTextLarge)
	theme.TextStyleTitleSmall = makeStyle(bold, theme.SizeTextMedium)
	theme.TextStyleBodyLarge = makeStyle(normal, theme.SizeTextLarge)
	theme.TextStyleBody = makeStyle(normal, theme.SizeTextMedium)
	theme.TextStyleBodySmall = makeStyle(normal, theme.SizeTextSmall)
	theme.TextStyleCaption = makeStyle(normal, theme.SizeTextXSmall)
	theme.TextStyleCaptionSmall = makeStyle(normal, theme.SizeTextTiny)
	theme.tableStyle.TextStyleHead = theme.TextStyleTitleSmall
	theme.badgeStyle.TextStyle = theme.TextStyleCaption.Bold()
	// The fill and the text on it are one decision (issue #373).
	// A literal White here was polarity-blind: every light preset
	// fills a neutral badge with ColorActive, a near-white, and
	// drew a white label on it at contrast 1.26-1.32. Badge
	// re-pairs per variant and per caller fill.
	theme.badgeStyle.TextStyle.Color = textOnFor(theme.badgeStyle.Colors.Base)

	// Heading roles take the title roles (visual-refresh §2.2): a widget
	// rendering a heading, a group-box title, a dialog title, a tab
	// label or a table header names a title role; body and value text
	// stays Body. TableStyle.TextStyleHead above and the DataGrid header
	// are the pre-existing members of the set; these three join it
	// here, where the roles exist. The group-box title lives in
	// addGroupBoxTitle (gui/view_container.go), reading the TitleSmall role.
	theme.dialogStyle.titleTextStyle = theme.TextStyleTitle
	theme.toastStyle.TitleStyle = theme.TextStyleTitleSmall
	// The selected tab carries the weight, the strip stays quiet.
	// It also fills with the accent, so its label draws in the
	// paired foreground (issue #373, visual-refresh §4.3) — the
	// literal above is overwritten here because the title roles
	// only exist after the styles are built.
	theme.tabControlStyle.textStyleSelected =
		textOnFill(theme.TextStyleTitleSmall, true, theme.ColorTextOnSelect)

	// Code roles: +1 at every rung, so CodeSmall (13) does not share
	// BodySmall's (12) baseline. Mono faces typically draw optically
	// smaller than regular ones at the same point size; the offset is
	// the compensation, applied uniformly because it is the same face
	// at every size. Theme.Mono applies the same step to a donor.
	mono := ts
	mono.Family = cfg.MonoFontFamily
	theme.TextStyleCode = makeStyle(mono, theme.SizeTextMedium+1)
	theme.TextStyleCodeSmall = makeStyle(mono, theme.SizeTextXSmall+1)
	theme.TextStyleCodeTiny = makeStyle(mono, theme.SizeTextTiny+1)

	// Icon roles.
	icon := ts
	icon.Family = iconFamily
	// Runs in this family are glyphs, not text: optical centring must
	// centre them on their own ink rather than on a cap band the face
	// does not really have.
	icon.glyphRole = true
	theme.TextStyleIconXLarge = makeStyle(icon, theme.SizeTextXLarge)
	theme.TextStyleIconLarge = makeStyle(icon, theme.SizeTextLarge)
	theme.TextStyleIconMedium = makeStyle(icon, theme.SizeTextMedium)
	theme.TextStyleIconSmall = makeStyle(icon, theme.SizeTextSmall)
	theme.TextStyleIconXSmall = makeStyle(icon, theme.SizeTextXSmall)
	theme.TextStyleIconTiny = makeStyle(icon, theme.SizeTextTiny)
}
