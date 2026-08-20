package gui

// Field minimum widths.
//
// A Fit-sized empty field is a stub: without a floor a text-bearing
// control is the width of whatever happens to be typed into it, so an
// empty Input and a filled one in the same form are different widths.
//
// Before this, the floor was per widget style — ThemeMaker spelled
// MinWidth 75 on selectStyle and again on comboboxStyle, and Input had
// no floor at all — which is why a Select and an Input in one row
// disagreed on width for a reason neither the theme nor the caller
// stated. The theme now states it once (Theme.SizeFieldMinWidth) and
// this file holds the one rule for reading it, so the opt-out cannot
// drift between the controls that share it.

// fieldMinWidth returns the MinWidth a text-bearing form control takes,
// applying the theme floor only where the caller stated no width of its
// own.
//
// Both a stated MinWidth and a stated Width opt out: each already
// answers the question the floor exists for, and forcing the floor over
// a stated Width would blow apart a deliberately narrow field (the
// ColorFields channel inputs). Zero on the theme means no floor rather
// than "derive the default" — see Theme.SizeFieldMinWidth.
//
// Reads guiTheme, not w.Theme(): every caller runs inside generation,
// where the bare read is what respects a Themed subtree.
func fieldMinWidth(minWidth, width float32) float32 {
	if minWidth != 0 || width != 0 {
		return minWidth
	}
	return guiTheme.SizeFieldMinWidth
}
