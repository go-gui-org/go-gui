package gui

// RadioOption defines a radio button for a RadioButtonGroupCfg.
type RadioOption struct {
	Label string
	Value string
}

// NewRadioOption creates a RadioOption.
func NewRadioOption(label, value string) RadioOption {
	return RadioOption{Label: label, Value: value}
}

// RadioButtonGroupCfg configures a radio button group.
type RadioButtonGroupCfg struct {
	TextStyle TextStyle

	OnSelect func(string, EventCtx)
	Value    string
	Title    string

	A11YCfg
	// Items is a convenience field for simple string lists. Each
	// string becomes a RadioOption with Label==Value. When set,
	// Items takes precedence over Options.
	Items      []string
	Options    []RadioOption
	ID         string `gui:"required,focus"`
	Padding    Padding
	Spacing    Opt[float32]
	SizeBorder Opt[float32]
	MinWidth   float32
	MinHeight  float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	ColorBorder   Color
	TitleBG       Color
	Sizing        Sizing
	Disabled      bool

	// Sound overrides the theme's selection cue for this instance.
	// SoundNone (the zero value) takes the theme's cue for that role,
	// which is itself silent unless the app opted in (issue #446).
	// exportaudit:keep — caller-facing config (issue #467)
	Sound SoundCue

	// SoundDisabled suppresses every option's sound regardless of the theme
	// and of Sound above.
	// exportaudit:keep — caller-facing config (issue #467)
	SoundDisabled bool
}

// RadioButtonGroupColumn creates a vertically stacked radio
// button group.
func RadioButtonGroupColumn(cfg RadioButtonGroupCfg) View {
	return radioGroup(cfg, Column)
}

// RadioButtonGroupRow creates a horizontally stacked radio
// button group.
func RadioButtonGroupRow(cfg RadioButtonGroupCfg) View {
	return radioGroup(cfg, Row)
}

func radioGroup(cfg RadioButtonGroupCfg, axis func(ContainerCfg) View) View {
	applyRadioGroupDefaults(&cfg)
	requireFocusID("RadioButtonGroup", cfg.FocusDisabled, cfg.ID)
	if len(cfg.Items) > 0 {
		n := min(len(cfg.Items), maxDataConvLen)
		cfg.Options = make([]RadioOption, n)
		for i := range n {
			cfg.Options[i] = RadioOption{
				Label: cfg.Items[i], Value: cfg.Items[i]}
		}
	}
	// The group's border is the group box's, so pass the Opt through
	// unresolved and let Container fall back to the themed container
	// style. Resolving it here against a private literal was how this
	// widget stayed at a 1.5px border under every theme (issue #300).
	return axis(ContainerCfg{
		A11YRole:    AccessRoleRadioGroup,
		A11YCfg:     cfg.A11YCfg,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Title:       cfg.Title,
		TitleBG:     cfg.TitleBG,
		Spacing:     cfg.Spacing,
		Padding:     cfg.Padding,
		MinWidth:    cfg.MinWidth,
		MinHeight:   cfg.MinHeight,
		Sizing:      cfg.Sizing,
		Disabled:    cfg.Disabled,
		Content:     buildRadioOptions(cfg),
	})
}

func buildRadioOptions(cfg RadioButtonGroupCfg) []View {
	content := make([]View, 0, len(cfg.Options))
	onSelect := cfg.OnSelect
	for i, opt := range cfg.Options {
		optValue := opt.Value
		content = append(content, Radio(RadioCfg{
			ID:            ScopeIDN(cfg.ID, "opt", i),
			Label:         opt.Label,
			FocusDisabled: cfg.FocusDisabled,
			Selected:      cfg.Value == opt.Value,
			Disabled:      cfg.Disabled,
			TextStyle:     cfg.TextStyle,
			// Forwarded, not resolved here: Radio owns the
			// precedence, so the group only has to hand its own
			// choice down to every option (issue #467).
			Sound:         cfg.Sound,
			SoundDisabled: cfg.SoundDisabled,
			OnClick: func(ctx EventCtx) {
				if onSelect != nil {
					onSelect(optValue, ctx)
				}
			},
		}))
	}
	return content
}

func applyRadioGroupDefaults(cfg *RadioButtonGroupCfg) {
	// No ColorBorder fallback here: a titled group is a group box, and
	// the container resolves the group-box ink for an unset border
	// (gui/view_container.go). Resolving to guiTheme.ColorBorder here
	// would pin the hairline wash, which reads as nothing on a
	// transparent ground.
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.PaddingLarge
	}
	if !cfg.Spacing.IsSet() {
		// Sibling controls in a stack take the Medium tier (audit §4,
		// issue #344), not the Small one — Small is for members of one
		// visual group, and the options in a radio group are separate
		// siblings stacked under the group box.
		cfg.Spacing = Some(SpacingMedium)
	}
}
