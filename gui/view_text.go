package gui

// TextCfg configures a text view. Use for labels, headings, or
// multiline text blocks. Set Focusable to enable text selection
// and clipboard copy.
type TextCfg struct {
	TextStyle TextStyle
	ID        string
	Text      string

	A11YLabel       string
	A11YDescription string
	Opacity         Opt[float32]
	Focusable       bool

	// TabSize sets the tab stop width in spaces (default 4).
	TabSize uint32

	MinWidth float32
	Sizing   Sizing

	// Mode controls text wrapping and overflow behavior. See
	// TextMode constants.
	Mode TextMode

	Invisible  bool
	Clip       bool
	FocusSkip  bool
	Disabled   bool
	IsPassword bool

	// PlaceholderActive enables placeholder styling (dimmed).
	// Set by input widgets; not typically set directly.
	PlaceholderActive bool

	// Hero marks this text element for hero transition
	// animations between views.
	Hero bool
}

// textView implements View for text rendering.
type textView struct {
	cfg TextCfg
	tc  ShapeTextConfig
}

// textEventHandlers is a shared handler set for focused text
// widgets, avoiding per-frame heap allocations.
var textEventHandlers = &eventHandlers{
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
			shapeType: shapeText,
			ID:        c.ID,
			Focusable: c.Focusable,
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
	applyFixedSizingConstraints(layout.Shape)

	if c.Focusable {
		layout.Shape.events = textEventHandlers
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
