package gui

// SeparatorCfg configures a divider rule.
//
// A separator is a thin colored box: horizontal by default, filling the
// parent's width at the theme thickness. Vertical separators fill the
// parent's height at the theme thickness. The theme's divider color and
// thickness apply unless overridden.
type SeparatorCfg struct {
	A11YCfg
	ID       string
	Vertical bool    // default horizontal
	Color    Color   // zero takes the theme's divider color
	Size     float32 // ergonomics-audit:opt-plain — zero means "theme default thickness", not a 0px rule a caller means
	Inset    Padding // transparent margin around the rule
	Length   float32 // rule extent along its long axis; zero fills the parent
}

// Separator draws a divider rule. The rule never takes a border or
// radius from the theme — a separator is an edge, so a border around
// it is never wanted.
func Separator(cfg SeparatorCfg) View {
	// Theme defaults resolve at generation time via the mirror
	// applyTheme fills (see the theme phase rule).
	style := defaultSeparatorStyle
	size := cfg.Size
	if size == 0 {
		size = style.Size
	}
	color := cfg.Color
	if !color.IsSet() {
		color = style.Color
	}

	// The rule itself is a borderless, radius-free box. Padding must be
	// explicit: a bare container would otherwise inherit the theme's
	// border (doubling the height) and radius.
	rule := ContainerCfg{
		ID:         cfg.ID,
		A11YCfg:    cfg.A11YCfg,
		Color:      color,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Radius:     NoRadius,
		Sizing:     FixedFixed,
	}
	if cfg.Vertical {
		rule.Width = size
		if cfg.Length > 0 {
			rule.Height = cfg.Length
		} else {
			rule.Sizing = FixedFill
		}
	} else {
		rule.Height = size
		if cfg.Length > 0 {
			rule.Width = cfg.Length
		} else {
			rule.Sizing = FillFixed
		}
	}

	ruleView := container(rule)
	if !cfg.Inset.IsSet() {
		return ruleView
	}
	// The inset wrapper is transparent; the rule fills it, so the
	// padding reads as margin around the rule. Along the parent's
	// cross axis the wrapper still fills (Inset only ever insets the
	// rule itself). The wrapper is borderless: a theme border here
	// would eat into the rule's fill width.
	if cfg.Vertical {
		return Row(ContainerCfg{
			Padding:    cfg.Inset,
			Sizing:     FixedFill,
			SizeBorder: NoBorder,
			Content:    []View{ruleView},
		})
	}
	return Column(ContainerCfg{
		Padding:    cfg.Inset,
		Sizing:     FillFit,
		SizeBorder: NoBorder,
		Content:    []View{ruleView},
	})
}
