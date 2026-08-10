package gui

// DrawCanvasCfg configures a draw canvas view.
type DrawCanvasCfg struct {
	OnDraw          func(*DrawContext)
	OnClick         func(EventCtx)
	OnHover         func(EventCtx)
	OnMouseMove     func(EventCtx)
	OnMouseUp       func(EventCtx)
	OnMouseLeave    func(EventCtx)
	OnGesture       func(EventCtx)
	OnMouseScroll   func(EventCtx)
	OnFileDrop      func(EventCtx)
	OnKeyDown       func(EventCtx)
	ID              string
	A11YLabel       string
	A11YDescription string
	Version         uint64
	Padding         Opt[Padding]
	Width           float32
	Height          float32
	MinWidth        float32
	MaxWidth        float32
	MinHeight       float32
	MaxHeight       float32
	Radius          float32 // ergonomics-audit:opt-plain — a canvas is a primitive: radius 0 (sharp) is a real choice, theme default not applicable
	Focusable       bool
	Color           Color
	Sizing          Sizing
	Clip            bool
}

// drawCanvasView implements View for user-drawn canvas content.
type drawCanvasView struct {
	cfg DrawCanvasCfg
}

// DrawCanvas creates a canvas with user-drawn geometry.
func DrawCanvas(cfg DrawCanvasCfg) View {
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FixedFixed
	}
	if !cfg.Color.IsSet() {
		cfg.Color = ColorTransparent
	}
	return &drawCanvasView{cfg: cfg}
}

func (dv *drawCanvasView) Content() []View { return nil }

func (dv *drawCanvasView) GenerateLayout(w *Window) Layout {
	c := &dv.cfg

	var events *eventHandlers
	if c.OnClick != nil || c.OnHover != nil || c.OnMouseMove != nil ||
		c.OnMouseUp != nil || c.OnMouseLeave != nil || c.OnGesture != nil ||
		c.OnMouseScroll != nil || c.OnFileDrop != nil ||
		c.OnKeyDown != nil || c.OnDraw != nil {
		events = w.allocEventHandlers(eventHandlers{
			OnClick:       c.OnClick,
			clickButton:   MouseLeft,
			OnHover:       c.OnHover,
			OnMouseMove:   c.OnMouseMove,
			OnMouseUp:     c.OnMouseUp,
			OnMouseLeave:  c.OnMouseLeave,
			OnGesture:     c.OnGesture,
			OnMouseScroll: c.OnMouseScroll,
			OnFileDrop:    c.OnFileDrop,
			OnKeyDown:     c.OnKeyDown,
			OnDraw:        c.OnDraw,
		})
	}

	// Focusable canvas advertises as a button to assistive tech
	// so screen readers announce it as interactive.
	a11yRole := AccessRoleImage
	if c.Focusable {
		a11yRole = AccessRoleButton
	}

	layout := Layout{
		Shape: w.allocShape(Shape{
			shapeType: shapeDrawCanvas,
			ID:        c.ID,
			Version:   c.Version,
			A11YRole:  a11yRole,
			a11Y:      makeA11YInfo(c.A11YLabel, c.A11YDescription),
			Width:     c.Width,
			Height:    c.Height,
			MinWidth:  c.MinWidth,
			MaxWidth:  c.MaxWidth,
			MinHeight: c.MinHeight,
			MaxHeight: c.MaxHeight,
			Sizing:    c.Sizing,
			Padding:   c.Padding.Get(Padding{}),
			Clip:      c.Clip,
			Color:     c.Color,
			Radius:    c.Radius,
			Focusable: c.Focusable,
			events:    events,
		}),
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}
