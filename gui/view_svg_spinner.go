//go:generate go run ./internal/gen/spinnerkinds/

package gui

// SvgSpinnerCfg configures a built-in SVG spinner.
//
// Animation coverage: SMIL <animate>/<animateTransform>/
// <animateMotion> with rotate, translate, scale, opacity / fill-
// opacity / stroke-opacity, attribute keyframes, spline easing,
// syncbase begins; CSS @keyframes for the same transform and
// opacity properties (animation-delay, iteration count, fill-
// mode, direction, timing-function). Out of scope: CSS keyframes
// targeting stroke-dasharray, stroke-dashoffset, stroke-width,
// or rect width; 3D rotateX / rotateY; calc() in transform-
// origin; :is() / :where() selectors. Affected assets render
// as the static initial frame.
type SvgSpinnerCfg struct {
	OnClick         func(*Layout, *Event, *Window)
	ID              string
	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	Width           float32
	Height          float32
	MinWidth        float32
	MaxWidth        float32
	MinHeight       float32
	MaxHeight       float32
	Kind            SvgSpinnerKind
	Color           Color
	Sizing          Sizing
}

// SvgSpinner renders a built-in animated SVG spinner identified
// by cfg.Kind. cfg.Color recolors monochrome assets via the
// SVG fill="currentColor" convention. When unset, defaults to
// the current theme's text color so spinners are visible on
// the standard background.
func SvgSpinner(cfg SvgSpinnerCfg) View {
	if uint16(cfg.Kind) >= svgSpinnerCount {
		cfg.Kind = 0
	}
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.TextStyleDef.Color
	}
	width := cfg.Width
	height := cfg.Height
	if width <= 0 && height <= 0 {
		width = 48
		height = 48
	}
	return Svg(SvgCfg{
		ID:              cfg.ID,
		SvgData:         svgSpinnerData[cfg.Kind],
		Width:           width,
		Height:          height,
		Color:           cfg.Color,
		Sizing:          cfg.Sizing,
		Padding:         cfg.Padding,
		OnClick:         cfg.OnClick,
		A11YLabel:       cfg.A11YLabel,
		A11YDescription: cfg.A11YDescription,
	})
}

// SvgSpinnerName returns the asset basename for a given kind
// (e.g. "90-ring"). Useful for debugging and gallery labels.
func SvgSpinnerName(k SvgSpinnerKind) string {
	if uint16(k) >= svgSpinnerCount {
		return ""
	}
	return svgSpinnerName[k]
}

// SvgSpinnerCount returns the number of built-in spinner kinds.
func SvgSpinnerCount() int { return svgSpinnerCount }
