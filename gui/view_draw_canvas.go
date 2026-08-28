package gui

// DrawCanvasCfg configures a draw canvas view.
type DrawCanvasCfg struct {
	OnDraw        func(*DrawContext)
	OnClick       func(EventCtx)
	OnHover       func(EventCtx)
	OnMouseMove   func(EventCtx)
	OnMouseUp     func(EventCtx)
	OnMouseLeave  func(EventCtx)
	OnGesture     func(EventCtx)
	OnMouseScroll func(EventCtx)
	OnFileDrop    func(EventCtx)
	OnKeyDown     func(EventCtx)
	ID            string
	A11YCfg
	Version   uint64
	Padding   Padding
	Width     float32
	Height    float32
	MinWidth  float32
	MaxWidth  float32
	MinHeight float32
	MaxHeight float32
	Radius    float32 // ergonomics-audit:opt-plain — a canvas is a primitive: radius 0 (sharp) is a real choice, theme default not applicable
	Focusable bool
	Color     Color
	Sizing    Sizing
	Clip      bool

	// AlwaysRedraw re-runs OnDraw on every render pass, ignoring
	// Version.
	//
	// It is what makes an animated canvas work under
	// AnimationRefreshRenderOnly. That refresh kind rebuilds the
	// renderers from the layout already in hand and never re-runs the
	// view function — which is the point, since a canvas animating off
	// its own state has no reason to rebuild the widgets around it —
	// but Version is stamped onto the shape during view generation, so
	// a version bump made between frames would never reach the cache
	// and the canvas would freeze on its first frame.
	//
	// Costs nothing where it is wanted: a canvas that bumps Version
	// every frame never gets a cache hit anyway. The tessellation
	// buffers are still pooled across redraws either way.
	//
	// exportaudit:keep — the app-side half of
	// AnimationRefreshRenderOnly; an animated canvas is unusable with
	// that refresh kind without it
	AlwaysRedraw bool
}

// drawCanvasView implements View for user-drawn canvas content.
type drawCanvasView struct {
	cfg DrawCanvasCfg
}

// DrawCanvas creates a canvas with user-drawn geometry.
func DrawCanvas(cfg DrawCanvasCfg) View {
	cfg.Sizing = cfg.Sizing.Or(FixedFixed)
	if !cfg.Color.IsSet() {
		cfg.Color = ColorTransparent
	}
	return &drawCanvasView{cfg: cfg}
}

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
			shapeType:    shapeDrawCanvas,
			ID:           c.ID,
			Version:      c.Version,
			A11YRole:     a11yRole,
			a11Y:         c.a11yInfo(""),
			Width:        c.Width,
			Height:       c.Height,
			MinWidth:     c.MinWidth,
			MaxWidth:     c.MaxWidth,
			MinHeight:    c.MinHeight,
			MaxHeight:    c.MaxHeight,
			Sizing:       c.Sizing,
			Padding:      c.Padding.Or(PaddingNone),
			Clip:         c.Clip,
			alwaysRedraw: c.AlwaysRedraw,
			Color:        c.Color,
			Radius:       c.Radius,
			Focusable:    c.Focusable,
			events:       events,
		}),
	}
	applyFixedSizingConstraints(layout.Shape)
	return layout
}
