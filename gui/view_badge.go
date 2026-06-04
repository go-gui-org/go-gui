package gui

import "strconv"

// BadgeVariant selects the badge color preset.
type BadgeVariant uint8

// BadgeVariant values.
const (
	BadgeDefault BadgeVariant = iota
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
	A11YLabel       string
	A11YDescription string
	Max             int // 0 = no cap; shows "max+" when exceeded
	Padding         Opt[Padding]
	DotSize         Opt[float32]

	Color   Color
	Variant BadgeVariant
	Dot     bool
}

// Badge creates a badge view. Dot mode renders a fixed-size
// circle; labeled mode renders text inside a rounded row.
func Badge(cfg BadgeCfg) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.BadgeStyle.Color
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(guiTheme.BadgeStyle.Padding)
	}
	style := guiTheme.BadgeStyle
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = style.TextStyle
	}
	pad := cfg.Padding.Get(style.Padding)
	radius := (cfg.TextStyle.Size + pad.Top + pad.Bottom) / 2
	dotSize := cfg.DotSize.Get(style.DotSize)
	bg := cfg.Color
	switch cfg.Variant {
	case BadgeInfo:
		bg = style.ColorInfo
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
			A11YRole:        AccessRoleStaticText,
			A11YLabel:       a11yLabel(cfg.A11YLabel, "status"),
			A11YDescription: cfg.A11YDescription,
			Color:           bg,
			Radius:          Some(sz / 2),
			Width:           sz,
			Height:          sz,
			Sizing:          FixedFixed,
			Padding:         NoPadding,
			SizeBorder:      NoBorder,
		})
	}

	label := badgeLabel(cfg.Label, cfg.Max)
	return Row(ContainerCfg{
		A11YRole:        AccessRoleStaticText,
		A11YLabel:       a11yLabel(cfg.A11YLabel, label),
		A11YDescription: cfg.A11YDescription,
		Color:           bg,
		Radius:          Some(radius),
		Sizing:          FitFit,
		Padding:         Some(pad),
		SizeBorder:      NoBorder,
		HAlign:          HAlignCenter,
		VAlign:          VAlignMiddle,
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
