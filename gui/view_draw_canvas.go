package gui

// DrawCanvasCfg configures a draw canvas view.
type DrawCanvasCfg struct {
	OnDraw          func(*DrawContext)
	OnClick         func(*Layout, *Event, *Window)
	OnHover         func(*Layout, *Event, *Window)
	OnMouseMove     func(*Layout, *Event, *Window)
	OnMouseUp       func(*Layout, *Event, *Window)
	OnMouseLeave    func(*Layout, *Event, *Window)
	OnGesture       func(*Layout, *Event, *Window)
	OnMouseScroll   func(*Layout, *Event, *Window)
	OnFileDrop      func(*Layout, *Event, *Window)
	OnKeyDown       func(*Layout, *Event, *Window)
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
	Radius          float32
	IDFocus         uint32
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

func (dv *drawCanvasView) GenerateLayout(_ *Window) Layout {
	c := &dv.cfg

	var events *eventHandlers
	if c.OnClick != nil || c.OnHover != nil || c.OnMouseMove != nil ||
		c.OnMouseUp != nil || c.OnMouseLeave != nil || c.OnGesture != nil ||
		c.OnMouseScroll != nil || c.OnFileDrop != nil ||
		c.OnKeyDown != nil || c.OnDraw != nil {
		events = &eventHandlers{
			OnClick:       c.OnClick,
			ClickButton:   MouseLeft,
			OnHover:       c.OnHover,
			OnMouseMove:   c.OnMouseMove,
			OnMouseUp:     c.OnMouseUp,
			OnMouseLeave:  c.OnMouseLeave,
			OnGesture:     c.OnGesture,
			OnMouseScroll: c.OnMouseScroll,
			OnFileDrop:    c.OnFileDrop,
			OnKeyDown:     c.OnKeyDown,
			OnDraw:        c.OnDraw,
		}
	}

	// Focusable canvas advertises as a button to assistive tech
	// so screen readers announce it as interactive.
	a11yRole := AccessRoleImage
	if c.IDFocus > 0 {
		a11yRole = AccessRoleButton
	}

	layout := Layout{
		Shape: &Shape{
			shapeType: shapeDrawCanvas,
			ID:        c.ID,
			Version:   c.Version,
			A11YRole:  a11yRole,
			A11Y:      makeA11YInfo(c.A11YLabel, c.A11YDescription),
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
			IDFocus:   c.IDFocus,
			events:    events,
		},
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}
