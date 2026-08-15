package gui

// disclosureArrowScale shrinks the disclosure triangle relative to the
// widget's body text. U+25BC/U+25B2 are full-width geometric glyphs that
// fill nearly the whole em box, so drawn at the label's size the wedge
// reads as an oversized graphic rather than an affordance. Roughly
// three-fifths of the body size matches the optical weight of the arrow
// native controls draw.
const disclosureArrowScale float32 = 0.6

// disclosureArrowNudge corrects the triangle's optical center, as a
// fraction of the arrow's own font size. Centering aligns line boxes,
// not ink: the box runs from the ascent down to the descent, while the
// triangle's ink stops at the baseline, so the wedge sits about a half
// descent above the true middle. The padding goes on the top only; the
// middle alignment then re-centers the taller box, which moves the ink
// down by half this fraction.
const disclosureArrowNudge float32 = 0.2

// disclosureArrow builds the open/closed triangle shared by ExpandPanel,
// Combobox and Select. style is the widget's body text style; only the
// size is altered, so the arrow keeps the label's color, family and
// typeface.
//
// The shrunken glyph's line box is shorter than the label's, so the
// container holding both must set VAlign: VAlignMiddle — with the
// default top alignment the arrow rides against the top of the row.
func disclosureArrow(open bool, style TextStyle) View {
	text := "▼"
	if open {
		text = "▲"
	}
	// Resolve the default here rather than leaving it to Text(): Text
	// substitutes the full medium size when Size is zero, which would
	// land after the scale had already been applied to a zero. Mirror
	// Text()'s substitution exactly: a fully zero style gets the whole
	// DefaultTextStyle (family and color too), a style with fields but
	// zero Size only gets the size.
	if style == (TextStyle{}) {
		style = DefaultTextStyle
	}
	if style.Size == 0 {
		style.Size = sizeTextMedium
	}
	style.Size *= disclosureArrowScale
	// The wrapper exists only to carry the optical-center padding; Text
	// has no vertical offset of its own.
	return Row(ContainerCfg{
		Padding: NewPadding(style.Size*disclosureArrowNudge, 0, 0, 0),
		Content: []View{
			Text(TextCfg{
				Text:      text,
				TextStyle: style,
			}),
		},
	})
}
