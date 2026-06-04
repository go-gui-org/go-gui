package gui

// TextCfg configures a text view. Use for labels or larger
// blocks of multiline text. Giving it an IDFocus allows mark
// and copy operations.
type TextCfg struct {
	TextStyle TextStyle
	ID        string
	Text      string

	// Accessibility
	A11YLabel       string
	A11YDescription string
	Opacity         Opt[float32]
	IDFocus         uint32
	TabSize         uint32
	MinWidth        float32
	Sizing          Sizing

	Mode              TextMode
	Invisible         bool
	Clip              bool
	FocusSkip         bool
	Disabled          bool
	IsPassword        bool
	PlaceholderActive bool
	Hero              bool
}

// textView implements View for text rendering.
type textView struct {
	cfg TextCfg
	tc  ShapeTextConfig
}

// textEventHandlers is a shared handler set for focused text
// widgets, avoiding per-frame heap allocations.
var textEventHandlers = &EventHandlers{
	OnClick:     textOnClick,
	OnKeyDown:   textOnKeyDown,
	AmendLayout: textAmendLayout,
}

func (tv *textView) Content() []View { return nil }

func (tv *textView) GenerateLayout(w *Window) Layout {
	c := &tv.cfg
	ts := &c.TextStyle

	tv.tc = ShapeTextConfig{
		Text:              c.Text,
		TextStyle:         ts,
		TextIsPassword:    c.IsPassword,
		TextIsPlaceholder: c.PlaceholderActive,
		TextMode:          c.Mode,
		TextTabSize:       c.TabSize,
	}

	layout := Layout{
		Shape: w.allocShape(Shape{
			ShapeType: ShapeText,
			ID:        c.ID,
			IDFocus:   c.IDFocus,
			A11YRole:  AccessRoleStaticText,
			A11Y: makeA11YInfo(
				a11yLabel(c.A11YLabel, c.Text), c.A11YDescription,
			),
			Clip:      c.Clip,
			FocusSkip: c.FocusSkip,
			Disabled:  c.Disabled,
			MinWidth:  c.MinWidth,
			Sizing:    c.Sizing,
			Hero:      c.Hero,
			Opacity:   c.Opacity.Get(1.0),
			TC:        &tv.tc,
		}),
	}

	layout.Shape.Width = w.TextWidth(c.Text, *ts)
	if w.textMeasurer != nil {
		layout.Shape.Height = w.textMeasurer.FontHeight(*ts)
	} else {
		layout.Shape.Height = ts.Size * 1.4
	}
	if c.Mode == TextModeSingleLine ||
		layout.Shape.Sizing.Width == SizingFixed {
		layout.Shape.MinWidth = f32Max(
			layout.Shape.Width, layout.Shape.MinWidth,
		)
		layout.Shape.Width = layout.Shape.MinWidth
	}
	if c.Mode == TextModeSingleLine ||
		layout.Shape.Sizing.Height == SizingFixed {
		layout.Shape.MinHeight = f32Max(
			layout.Shape.Height, layout.Shape.MinHeight,
		)
		layout.Shape.Height = layout.Shape.MinHeight
	}
	ApplyFixedSizingConstraints(layout.Shape)

	if c.IDFocus > 0 {
		layout.Shape.Events = textEventHandlers
	}

	return layout
}

// Text creates a text view for displaying text content.
func Text(cfg TextCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	sizing := cfg.Sizing
	if sizing == (Sizing{}) {
		if cfg.Mode == TextModeWrap ||
			cfg.Mode == TextModeWrapKeepSpaces {
			sizing = FillFit
		} else {
			sizing = FitFit
		}
	}
	if cfg.TabSize == 0 {
		cfg.TabSize = 4
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.TextStyle.Size == 0 {
		cfg.TextStyle.Size = SizeTextMedium
	}
	cfg.Sizing = sizing
	return &textView{cfg: cfg}
}
