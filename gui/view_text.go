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
	Mode textMode

	Invisible  bool
	Clip       bool
	FocusSkip  bool
	Disabled   bool
	IsPassword bool

	// PlaceholderActive enables placeholder styling (dimmed).
	// Set by input widgets; not typically set directly.
	placeholderActive bool

	// Hero marks this text element for hero transition
	// animations between views.
	Hero bool

	// readOnly is set by input widgets (view_input.go) to suppress
	// IME preedit on a read-only field that stays Focusable.
	// Unexported: not a meaningful knob for standalone Text callers.
	readOnly bool

	// focusOwner is set by input widgets (view_input.go) to the ID of
	// the container that owns the focus and per-widget state this text
	// renders from. See Shape.focusOwner. Unexported: a standalone
	// Text owns its identity through ID.
	focusOwner string
}

// textView implements View for text rendering.
type textView struct {
	cfg TextCfg
	tc  shapeTextConfig
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

	tv.tc = shapeTextConfig{
		Text:              c.Text,
		TextStyle:         ts,
		textIsPassword:    c.IsPassword,
		textIsPlaceholder: c.placeholderActive,
		TextMode:          c.Mode,
		TextTabSize:       c.TabSize,
		textReadOnly:      c.readOnly,
	}

	layout := Layout{
		Shape: w.allocShape(Shape{
			shapeType:  shapeText,
			ID:         c.ID,
			focusOwner: c.focusOwner,
			Focusable:  c.Focusable,
			A11YRole:   AccessRoleStaticText,
			a11Y: makeA11YInfo(
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
		layout.Shape.Height = fallbackLineHeight(*ts)
	}
	if c.Mode == TextModeSingleLine ||
		layout.Shape.Sizing.Width == sizingFixed {
		layout.Shape.MinWidth = f32Max(
			layout.Shape.Width, layout.Shape.MinWidth,
		)
		layout.Shape.Width = layout.Shape.MinWidth
	}
	if c.Mode == TextModeSingleLine ||
		layout.Shape.Sizing.Height == sizingFixed {
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
	if !sizing.IsSet() {
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
		cfg.TextStyle.Size = sizeTextMedium
	}
	cfg.Sizing = sizing
	return &textView{cfg: cfg}
}
