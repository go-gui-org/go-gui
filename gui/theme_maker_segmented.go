package gui

// segmentedInset is the gap between a segmented control's track edge
// and its pill (issue #600). Apple's control uses about 2pt. It is
// geometry of the widget, not a ladder rung: the ladder sizes gaps
// between things, this sizes a part of one control.
const segmentedInset = 2

// segmentedControlStyleFor builds the segmented control style (issue
// #600). Own file so theme_maker.go stays under the large-files gate.
//
// The track takes the field fill (ColorInterior) and the pill the
// accent (ColorSelect), the same fill as a selected tab (#741). A
// neutral "raised" pill was tried first and failed in the goldens: the
// light ladder has ColorPanel == ColorInterior == white, so the pill
// vanished into the track, and the dark ladder put it below the track.
// The accent reads on every preset. A segment at rest is transparent
// so the track shows through; hover and press tint it like a ghost
// button.
func segmentedControlStyleFor(
	cfg ThemeCfg, ts, textDisabled TextStyle, fieldPad Padding,
	colorSelect, borderFocus Color,
) segmentedControlStyle {
	// The track adds its inset above and below the segment, so the
	// segment gives the same amount back. The control then lands at
	// the height of an Input with the same field inset, and the two
	// share a row (Theme.PaddingField, issue #335).
	segPad := NewPadding(
		max(0, fieldPad.Top-segmentedInset), fieldPad.Right,
		max(0, fieldPad.Bottom-segmentedInset), fieldPad.Left)
	return segmentedControlStyle{
		textStyle: ts,
		// textStyleSelected is filled by fillTextRungs: the label on the
		// pill pairs with ColorTextOnSelect, which is set after this.
		textStyleSelected: ts,
		textStyleDisabled: textDisabled,
		// textStyleIcon is filled by fillTextRungs: the icon roles do
		// not exist yet when this runs.
		padding:        PadAll(segmentedInset),
		paddingSegment: segPad,
		sizeBorder:     cfg.SizeBorder,
		sizeDivider:    max(1, cfg.SizeBorder),
		radius:         cfg.Radius,
		// Concentric corners: the pill sits segmentedInset inside the
		// track, so its radius is the track's less that inset.
		radiusSegment: max(0, cfg.Radius-segmentedInset),
		colors: ColorSet{
			Base:        cfg.ColorInterior,
			Border:      cfg.ColorBorder,
			BorderFocus: borderFocus,
		},
		colorsSegment: ColorSet{
			Base:        ColorTransparent,
			Hover:       cfg.ColorHover,
			Click:       cfg.ColorActive,
			Focus:       ColorTransparent,
			Border:      ColorTransparent,
			BorderFocus: ColorTransparent,
			Selected:    colorSelect,
			Disabled:    ColorTransparent,
		},
		colorDivider: cfg.ColorBorder,
	}
}
