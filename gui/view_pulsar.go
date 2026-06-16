package gui

const pulsarAnimationID = "___pulsar_animation___"

// PulsarCfg configures a blinking text indicator.
type PulsarCfg struct {
	TextStyle TextStyle
	ID        string
	Text1     string
	Text2     string
	Size      Opt[float32]
	Width     float32
	Color     Color
}

// Pulsar creates a blinking text view that toggles between
// Text1 and Text2 based on the window's input cursor state.
func Pulsar(cfg PulsarCfg, w *Window) View {
	if cfg.Text1 == "" {
		cfg.Text1 = "..."
	}
	if cfg.Text2 == "" {
		cfg.Text2 = ".."
	}

	ts := cfg.TextStyle
	if ts == (TextStyle{}) {
		ts = guiTheme.TextStyleDef
	}
	if cfg.Color.IsSet() {
		ts.Color = cfg.Color
	}
	if cfg.Size.IsSet() {
		ts.Size = cfg.Size.Get(0)
	}

	// Pulsar toggles text in the view function, so it needs a
	// layout refresh — not the render-only refresh that
	// BlinkCursorAnimation provides. Use a dedicated Animate
	// that piggybacks on the same inputCursorOn state but
	// requests AnimationRefreshLayout.
	if !w.HasAnimation(blinkCursorAnimationID) {
		w.AnimationAdd(NewBlinkCursorAnimation())
	}
	// Note: animation delay is sampled once on first render via
	// touchViewBoundAnimation. Use a different widget ID to apply
	// new parameters.
	if !w.touchViewBoundAnimation(pulsarAnimationID) {
		w.animationAddViewBound(&Animate{
			AnimID:  pulsarAnimationID,
			Delay:   blinkCursorAnimationDelay,
			Repeat:  true,
			Refresh: AnimationRefreshLayout,
			Callback: func(a *Animate, _ *Window) {
				// No-op: the blink cursor animation already
				// toggles inputCursorOn. This animation exists
				// solely to promote the refresh to layout.
			},
		})
	}

	txt := cfg.Text2
	if w.inputCursorOn() {
		txt = cfg.Text1
	}

	width := cfg.Width
	if width <= 0 {
		width = w.TextWidth(cfg.Text1, ts)
	}

	return Column(ContainerCfg{
		ID:         cfg.ID,
		MinWidth:   width,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Sizing:     FitFit,
		Content: []View{
			Text(TextCfg{
				Text:      txt,
				TextStyle: ts,
			}),
		},
	})
}
