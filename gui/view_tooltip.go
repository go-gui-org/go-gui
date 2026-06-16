package gui

import "time"

// tooltipState tracks active tooltip bounds and ID.
type tooltipState struct {
	hoverStart   time.Time // when hover began
	id           string
	popupID      string   // cached popup ID string
	hoverID      string   // trigger currently hovered
	text         string   // RTF tooltip content
	blockKey     uint64   // FNV hash of owning RichText
	bounds       drawClip // absolute run bounds (mouse check)
	floatOffsetX float32  // run-relative X for float popup
	floatOffsetY float32  // run-relative Y for float popup
}

// clearText resets RTF-sourced tooltip state. No-op when
// text is empty so WithTooltip-managed state is preserved.
func (ts *tooltipState) clearText() {
	if ts.text == "" {
		return
	}
	ts.hoverID = ""
	ts.hoverStart = time.Time{}
	ts.id = ""
	ts.popupID = ""
	ts.text = ""
	ts.bounds = drawClip{}
	ts.floatOffsetX = 0
	ts.floatOffsetY = 0
	ts.blockKey = 0
}

// TooltipCfg configures a tooltip popup.
type TooltipCfg struct {
	TextStyle   TextStyle
	ID          string
	Content     []View
	Delay       time.Duration
	FloatZIndex int
	Padding     Opt[Padding]
	Radius      Opt[float32]
	SizeBorder  Opt[float32]
	OffsetX     Opt[float32]
	OffsetY     Opt[float32]
	Color       Color
	ColorBorder Color
	Anchor      Opt[FloatAttach]
	TieOff      Opt[FloatAttach]
}

// Tooltip creates a floating tooltip view.
func Tooltip(cfg TooltipCfg) View {
	applyTooltipDefaults(&cfg)
	d := &DefaultTooltipStyle
	return Column(ContainerCfg{
		ID:            cfg.ID,
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   cfg.Anchor.Get(FloatBottomCenter),
		FloatTieOff:   cfg.TieOff.Get(FloatTopCenter),
		FloatOffsetX:  cfg.OffsetX.Get(-3),
		FloatOffsetY:  cfg.OffsetY.Get(-3),
		FloatZIndex:   cfg.FloatZIndex,
		Color:         cfg.Color,
		ColorBorder:   cfg.ColorBorder,
		SizeBorder:    SomeF(cfg.SizeBorder.Get(d.SizeBorder)),
		Radius:        SomeF(cfg.Radius.Get(d.Radius)),
		Padding:       cfg.Padding,
		MaxWidth:      300,
		Content:       cfg.Content,
	})
}

// AnimationTooltip returns an Animate that checks mouse position
// after a delay and activates the tooltip if the mouse is still
// inside the trigger bounds.
func AnimationTooltip(cfg TooltipCfg) *Animate {
	delay := cfg.Delay
	if delay == 0 {
		delay = DefaultTooltipStyle.Delay
	}
	id := cfg.ID
	return &Animate{
		AnimID: "___tooltip___",
		Delay:  delay,
		Callback: func(_ *Animate, w *Window) {
			ts := &w.viewState.tooltip
			b := ts.bounds
			mx := w.viewState.mousePosX
			my := w.viewState.mousePosY
			if ts.hoverID == id &&
				mx >= b.X && my >= b.Y &&
				mx < b.X+b.Width && my < b.Y+b.Height {
				ts.id = id
				ts.popupID = id + "_popup"
			}
		},
	}
}

func applyTooltipDefaults(cfg *TooltipCfg) {
	d := &DefaultTooltipStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.Delay == 0 {
		cfg.Delay = d.Delay
	}
}

// WithTooltipCfg configures a tooltip wrapper.
type WithTooltipCfg struct {
	ID      string
	Text    string
	Content []View
	Delay   time.Duration
	Anchor  Opt[FloatAttach]
	TieOff  Opt[FloatAttach]
}

// WithTooltip wraps content and shows a tooltip on hover after
// a delay. Tooltip state is managed via AmendLayout.
func WithTooltip(w *Window, cfg WithTooltipCfg) View {
	tipID := cfg.ID
	if tipID == "" {
		tipID = cfg.Text
	}

	delay := cfg.Delay
	if delay == 0 {
		delay = DefaultTooltipStyle.Delay
	}

	anchor := cfg.Anchor.Get(FloatBottomCenter)
	tieOff := cfg.TieOff.Get(FloatTopCenter)

	content := make([]View, 0, len(cfg.Content)+1)
	content = append(content, cfg.Content...)

	ts := &w.viewState.tooltip
	if ts.id == tipID {
		content = append(content, Tooltip(TooltipCfg{
			ID:     ts.popupID,
			Anchor: Some(anchor),
			TieOff: Some(tieOff),
			Content: []View{
				Text(TextCfg{
					Text:      cfg.Text,
					TextStyle: DefaultTooltipStyle.TextStyle,
					Mode:      TextModeWrap,
				}),
			},
		}))
	}

	return Column(ContainerCfg{
		A11YRole:        AccessRoleGroup,
		A11YDescription: cfg.Text,
		SizeBorder:      NoBorder,
		Content:         content,
		AmendLayout:     withTooltipAmend(tipID, delay),
	})
}

// withTooltipAmend returns the AmendLayout callback for a
// WithTooltip wrapper.
func withTooltipAmend(
	tipID string, delay time.Duration,
) func(*Layout, *Window) {
	popupID := tipID + "_popup"
	return func(l *Layout, w *Window) {
		ts := &w.viewState.tooltip
		mx := w.viewState.mousePosX
		my := w.viewState.mousePosY
		inside := mx >= l.Shape.X && my >= l.Shape.Y &&
			mx < l.Shape.X+l.Shape.Width &&
			my < l.Shape.Y+l.Shape.Height

		switch {
		case inside && ts.hoverID == "":
			ts.hoverID = tipID
			ts.hoverStart = time.Now()
			w.AnimationAdd(&Animate{
				AnimID: "___tooltip___",
				Delay:  delay,
				Callback: func(_ *Animate, w *Window) {
					if w.viewState.tooltip.hoverID == tipID {
						w.viewState.tooltip.id = tipID
						w.viewState.tooltip.popupID = popupID
					}
				},
			})

		case inside && ts.hoverID == tipID &&
			time.Since(ts.hoverStart) >= delay:
			ts.id = tipID
			ts.popupID = popupID

		case !inside && ts.hoverID == tipID:
			ts.hoverID = ""
			ts.hoverStart = time.Time{}
			ts.id = ""
			ts.popupID = ""
		}
	}
}
