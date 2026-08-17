package gui

import "strconv"

// BadgeVariant selects the badge color preset.
type badgeVariant uint8

// BadgeVariant values.
const (
	badgeDefault badgeVariant = iota
	BadgeInfo
	BadgeSuccess
	BadgeWarning
	BadgeError
)

// BadgeCfg configures a Badge view.
type BadgeCfg struct {
	TextStyle TextStyle
	Label     string

	// Accessibility
	A11YCfg
	Max     int // 0 = no cap; shows "max+" when exceeded
	Padding Padding
	dotSize Opt[float32]

	Color   Color
	Variant badgeVariant
	Dot     bool
}

// Badge creates a badge view. Dot mode renders a fixed-size
// circle; labeled mode renders text inside a rounded row.
func Badge(cfg BadgeCfg) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.badgeStyle.Color
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.badgeStyle.Padding
	}
	style := guiTheme.badgeStyle
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = style.TextStyle
	}
	pad := cfg.Padding.Or(style.Padding)
	radius := (cfg.TextStyle.Size + pad.Top + pad.Bottom) / 2
	dotSize := cfg.dotSize.Get(style.dotSize)
	bg := cfg.Color
	switch cfg.Variant {
	case BadgeInfo:
		bg = style.colorInfo
	case BadgeSuccess:
		bg = style.ColorSuccess
	case BadgeWarning:
		bg = style.ColorWarning
	case BadgeError:
		bg = style.ColorError
	}

	if cfg.Dot {
		sz := dotSize
		return Row(ContainerCfg{
			A11YRole: AccessRoleStaticText,
			A11YCfg: A11YCfg{
				A11YLabel:       a11yLabel(cfg.A11YLabel, "status"),
				A11YDescription: cfg.A11YDescription,
			},
			Color:      bg,
			Radius:     Some(sz / 2),
			Width:      sz,
			Height:     sz,
			Sizing:     FixedFixed,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
		})
	}

	label := badgeLabel(cfg.Label, cfg.Max)
	return Row(ContainerCfg{
		A11YRole: AccessRoleStaticText,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, label),
			A11YDescription: cfg.A11YDescription,
		},
		Color:      bg,
		Radius:     Some(radius),
		Sizing:     FitFit,
		Padding:    pad,
		SizeBorder: NoBorder,
		HAlign:     HAlignCenter,
		VAlign:     VAlignMiddle,
		// A badge carries a count or a short tag it renders itself, so
		// the reserved descent below a cap-only label is what makes an
		// uncorrected badge read high in its pill (issue #346). The
		// label is measured, so "128" centres exactly and a tag with a
		// descender keeps metric centring instead of sinking.
		AmendLayout: opticalCenterText,
		Content: []View{
			Text(TextCfg{
				Text:      label,
				TextStyle: cfg.TextStyle,
			}),
		},
	})
}

func badgeLabel(label string, maxVal int) string {
	if maxVal <= 0 || len(label) == 0 {
		return label
	}
	n := 0
	for _, c := range label {
		if c < '0' || c > '9' {
			return label
		}
		n = n*10 + int(c-'0')
		if n > maxVal {
			return strconv.Itoa(maxVal) + "+"
		}
	}
	return label
}
