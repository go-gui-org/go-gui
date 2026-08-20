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
		Color:            cfg.ColorInterior,
		ColorHover:       cfg.ColorHover,
		ColorFocus:       cfg.ColorActive,
		colorClick:       cfg.ColorFocus,
		ColorBorder:      cfg.ColorBorder,
		ColorBorderFocus: borderFocus,
		Padding:          paddingButton,
		SizeBorder:       cfg.SizeBorder,
		Radius:           cfg.Radius,
	}
	errorHover := ColorToHSLA(colorError)
	errorHover.L = f32Clamp(errorHover.L+0.12, 0, 1)
	errorPressed := ColorToHSLA(colorError)
	errorPressed.L = f32Clamp(errorPressed.L-0.12, 0, 1)
	buttonPrimary = buttonBase
	buttonPrimary.Color = accent
	buttonPrimary.ColorHover = accentHover
	buttonPrimary.colorClick = accentPressed
	buttonPrimary.ColorFocus = accent
	buttonPrimary.ColorBorder = accent
	buttonGhost = buttonBase
	buttonGhost.Color = ColorTransparent
	buttonGhost.ColorHover = cfg.ColorHover
	buttonGhost.ColorFocus = ColorTransparent
	buttonGhost.ColorBorder = ColorTransparent
	buttonDanger = buttonBase
	buttonDanger.Color = colorError
	buttonDanger.ColorHover = errorHover.Color()
	buttonDanger.colorClick = errorPressed.Color()
	buttonDanger.ColorFocus = colorError
	buttonDanger.ColorBorder = colorError
	return buttonBase, buttonPrimary, buttonGhost, buttonDanger
}
