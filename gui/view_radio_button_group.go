package gui

import "strconv"

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

	OnSelect func(string, *Window)
	Value    string
	Title    string

	A11YLabel       string
	A11YDescription string
	// Items is a convenience field for simple string lists. Each
	// string becomes a RadioOption with Label==Value. When set,
	// Items takes precedence over Options.
	Items      []string
	Options    []RadioOption
	ID         string
	Padding    Opt[Padding]
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
}

// DefaultRadioGroupStyle holds defaults for RadioButtonGroupCfg Opt fields.
var DefaultRadioGroupStyle = RadioGroupStyle{
	SizeBorder: 1.5,
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
	if len(cfg.Items) > 0 {
		n := min(len(cfg.Items), maxDataConvLen)
		cfg.Options = make([]RadioOption, n)
		for i := range n {
			cfg.Options[i] = RadioOption{
				Label: cfg.Items[i], Value: cfg.Items[i]}
		}
	}
	sizeBorder := cfg.SizeBorder.Get(DefaultRadioGroupStyle.SizeBorder)
	return axis(ContainerCfg{
		A11YRole:        AccessRoleRadioGroup,
		A11YLabel:       cfg.A11YLabel,
		A11YDescription: cfg.A11YDescription,
		ColorBorder:     cfg.ColorBorder,
		SizeBorder:      Some(sizeBorder),
		Title:           cfg.Title,
		TitleBG:         cfg.TitleBG,
		Spacing:         cfg.Spacing,
		Padding:         cfg.Padding,
		MinWidth:        cfg.MinWidth,
		MinHeight:       cfg.MinHeight,
		Sizing:          cfg.Sizing,
		Disabled:        cfg.Disabled,
		Content:         buildRadioOptions(cfg),
	})
}

func buildRadioOptions(cfg RadioButtonGroupCfg) []View {
	content := make([]View, 0, len(cfg.Options))
	onSelect := cfg.OnSelect
	for i, opt := range cfg.Options {
		optValue := opt.Value
		content = append(content, Radio(RadioCfg{
			ID:            cfg.ID + "/" + strconv.Itoa(i),
			Label:         opt.Label,
			FocusDisabled: cfg.FocusDisabled,
			Selected:      cfg.Value == opt.Value,
			Disabled:      cfg.Disabled,
			TextStyle:     cfg.TextStyle,
			OnClick: func(_ *Layout, _ *Event, w *Window) {
				if onSelect != nil {
					onSelect(optValue, w)
				}
			},
		}))
	}
	return content
}

func applyRadioGroupDefaults(cfg *RadioButtonGroupCfg) {
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = guiTheme.ColorBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(guiTheme.PaddingLarge)
	}
	if !cfg.Spacing.IsSet() {
		cfg.Spacing = Some(SpacingSmall)
	}
}
