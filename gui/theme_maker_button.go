package gui

// deriveButtonStyles builds the button variant styles (visual-refresh
// §6). Each is the base button style with its fill swapped for a ramp
// color; geometry (padding, border, radius) stays the base, so variants
// of one theme align in a row. Ghost drops fill and border outright.
//
// The fills are derived, never literal: primary takes the accent ramp,
// danger derives hover/pressed from the error color with the same
// absolute-L ±0.12 steps the accent ramp uses (theme_maker's accent
// derivation). The label color is not part of a style: filled variants
// pair with ColorTextOnAccent at the call site.
//
// Own file so theme_maker.go stays under the large-files gate.
func deriveButtonStyles(
	cfg ThemeCfg, accent, accentHover, accentPressed, colorError, borderFocus Color,
) (buttonBase, buttonPrimary, buttonGhost, buttonDanger buttonStyle) {
	buttonBase = buttonStyle{
		Colors: ColorSet{
			Base:        cfg.ColorInterior,
			Hover:       cfg.ColorHover,
			Click:       cfg.ColorFocus,
			Focus:       cfg.ColorActive,
			Border:      cfg.ColorBorder,
			BorderFocus: borderFocus,
		},
		Padding:    paddingButton,
		SizeBorder: cfg.SizeBorder,
		Radius:     cfg.Radius,
	}
	errorHover := accentShift(colorError, oklchRampDelta)
	errorPressed := accentShift(colorError, -oklchRampDelta)
	buttonPrimary = buttonBase
	buttonPrimary.Colors.Base = accent
	buttonPrimary.Colors.Hover = accentHover
	buttonPrimary.Colors.Click = accentPressed
	buttonPrimary.Colors.Focus = accent
	buttonPrimary.Colors.Border = accent
	buttonGhost = buttonBase
	buttonGhost.Colors.Base = ColorTransparent
	buttonGhost.Colors.Hover = cfg.ColorHover
	buttonGhost.Colors.Focus = ColorTransparent
	buttonGhost.Colors.Border = ColorTransparent
	buttonDanger = buttonBase
	buttonDanger.Colors.Base = colorError
	buttonDanger.Colors.Hover = errorHover
	buttonDanger.Colors.Click = errorPressed
	buttonDanger.Colors.Focus = colorError
	buttonDanger.Colors.Border = colorError
	return buttonBase, buttonPrimary, buttonGhost, buttonDanger
}
