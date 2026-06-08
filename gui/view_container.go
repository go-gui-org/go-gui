package gui

// ContainerCfg configures container views (column, row, canvas,
// circle, wrap). Containers layout children vertically,
// horizontally, or freely with sizing, alignment, scrolling,
// floating, borders, and event handling.
type ContainerCfg struct {
	ColorFilter    *ColorFilter
	Shadow         *BoxShadow
	Gradient       *GradientDef
	BorderGradient *GradientDef
	Shader         *Shader

	A11Y *AccessInfo

	// Event handlers
	OnClick     func(*Layout, *Event, *Window)
	OnAnyClick  func(*Layout, *Event, *Window)
	OnChar      func(*Layout, *Event, *Window)
	OnKeyDown   func(*Layout, *Event, *Window)
	OnKeyUp     func(*Layout, *Event, *Window)
	OnMouseMove func(*Layout, *Event, *Window)
	OnMouseUp   func(*Layout, *Event, *Window)
	OnScroll    func(*Layout, *Window)
	AmendLayout func(*Layout, *Window)
	OnHover     func(*Layout, *Event, *Window)
	OnGesture   func(*Layout, *Event, *Window)
	OnFileDrop  func(*Layout, *Event, *Window)
	OnIMECommit func(*Layout, string, *Window)

	// Scrollbar config overrides. nil = use defaults when
	// IDScroll > 0. Set Overflow to ScrollbarHidden to suppress.
	ScrollbarCfgX *ScrollbarCfg
	ScrollbarCfgY *ScrollbarCfg

	// Identity
	ID string

	// Appearance
	Title           string
	A11YLabel       string
	A11YDescription string

	// Content
	Content []View

	FloatZIndex int

	Padding Opt[Padding]

	// Layout
	Spacing    Opt[float32]
	SizeBorder Opt[float32]
	Radius     Opt[float32]
	Opacity    Opt[float32]
	Width      float32
	Height     float32
	MinWidth   float32
	MaxWidth   float32
	MinHeight  float32
	MaxHeight  float32

	BlurRadius float32

	// Behavior
	IDFocus      uint32
	IDScroll     uint32
	FloatOffsetX float32
	FloatOffsetY float32

	// Position
	X float32
	Y float32

	A11YState   AccessState
	TitleBG     Color
	Color       Color
	ColorBorder Color

	// Sizing
	Sizing   Sizing
	HAlign   HorizontalAlign
	VAlign   VerticalAlign
	TextDir  TextDirection
	Wrap     bool
	Overflow bool

	ScrollMode   ScrollMode
	Clip         bool
	ClipContents bool
	FocusSkip    bool
	Disabled     bool
	Invisible    bool
	OverDraw     bool
	Hero         bool

	// Floating
	Float         bool
	FloatAutoFlip bool
	FloatAnchor   FloatAttach
	FloatTieOff   FloatAttach

	// Accessibility
	A11YRole AccessRole

	// Internal — set by factory functions.
	axis                 Axis
	scrollbarOrientation ScrollbarOrientation
}

func applyContainerDefaults(cfg *ContainerCfg) (spacing, sizeBorder, radius float32, padding Padding) {
	d := &DefaultContainerStyle
	return cfg.Spacing.Get(d.Spacing),
		cfg.SizeBorder.Get(d.SizeBorder),
		cfg.Radius.Get(d.Radius),
		cfg.Padding.Get(d.Padding)
}

// containerView implements View for container-based layouts.
// Shape is pre-built at factory time so the full ContainerCfg
// (~400 B) never escapes to heap with the view.
//
// IMPORTANT: GenerateLayout shallow-copies the shape on every
// call. The layout pipeline mutates Shape fields in place
// (X += offset, Width += childWidth, etc.), so returning the
// same pointer would let mutations accumulate when a view is
// cached and reused across frames (combobox/command-palette
// dropdown caches). The copy keeps the template pristine while
// still avoiding the per-frame ContainerCfg→Shape build cost.
type containerView struct {
	shape       *Shape
	title       string
	content     []View
	titleBG     Color
	colorBorder Color
	disabled    bool
}

func (cv *containerView) Content() []View { return cv.content }

func (cv *containerView) GenerateLayout(w *Window) Layout {
	layout := Layout{Shape: w.allocShape(*cv.shape)}
	addGroupBoxTitle(cv.title, cv.titleBG, cv.colorBorder,
		cv.disabled, w, &layout)
	return layout
}

// addGroupBoxTitle injects floating eraser + text children to render
// a title label in the container's top border (HTML fieldset style).
func addGroupBoxTitle(title string, titleBG, colorBorder Color,
	disabled bool, w *Window, layout *Layout) {
	if len(title) == 0 {
		return
	}
	ts := DefaultTextStyle

	var textWidth, fontHeight float32
	const pad float32 = 5
	if w.textMeasurer != nil {
		textWidth = w.textMeasurer.TextWidth(title, ts)
		fontHeight = w.textMeasurer.FontHeight(ts)
	} else {
		// Fallback for tests without a text measurer.
		textWidth = float32(len(title)) * 8
		fontHeight = 16
	}
	// Center the title vertically on the top border line.
	offset := fontHeight / 2

	eraserColor := titleBG
	if !eraserColor.IsSet() {
		eraserColor = ColorTransparent
	}
	if disabled {
		eraserColor = dimAlpha(eraserColor)
	}

	// Eraser hides the border behind the title text.
	eraserShape := Shape{
		shapeType: shapeRectangle,
		Width:     textWidth + pad + pad - 1,
		Height:    fontHeight,
		X:         20,
		Y:         -offset,
		Color:     eraserColor,
		Opacity:   1.0,
		Float:     true,
	}
	layout.Children = append(layout.Children, Layout{
		Shape: w.allocShape(eraserShape),
	})

	textColor := colorBorder
	if disabled {
		textColor = dimAlpha(textColor)
	}
	ts.Color = textColor
	textShape := Shape{
		shapeType: shapeText,
		Width:     textWidth,
		Height:    fontHeight,
		X:         20 + pad,
		Y:         -offset,
		Color:     textColor,
		Opacity:   1.0,
		Float:     true,
		TC: &ShapeTextConfig{
			Text:      title,
			TextStyle: &ts,
		},
	}
	layout.Children = append(layout.Children, Layout{
		Shape: w.allocShape(textShape),
	})
}

func makeContainerEffects(c *ContainerCfg) *shapeEffects {
	if c.Shadow == nil && c.Gradient == nil &&
		c.BorderGradient == nil && c.Shader == nil &&
		c.ColorFilter == nil && c.BlurRadius == 0 {
		return nil
	}
	return &shapeEffects{
		Shadow:         c.Shadow,
		Gradient:       c.Gradient,
		BorderGradient: c.BorderGradient,
		Shader:         c.Shader,
		ColorFilter:    c.ColorFilter,
		BlurRadius:     c.BlurRadius,
	}
}

func makeContainerEvents(c *ContainerCfg) *eventHandlers {
	if c.OnClick == nil && c.OnChar == nil &&
		c.OnKeyDown == nil && c.OnKeyUp == nil &&
		c.OnMouseMove == nil && c.OnMouseUp == nil &&
		c.OnHover == nil && c.OnGesture == nil &&
		c.OnFileDrop == nil && c.OnIMECommit == nil &&
		c.OnScroll == nil && c.AmendLayout == nil {
		return nil
	}
	return &eventHandlers{
		OnClick:     c.OnClick,
		OnChar:      c.OnChar,
		OnKeyDown:   c.OnKeyDown,
		OnKeyUp:     c.OnKeyUp,
		OnMouseMove: c.OnMouseMove,
		OnMouseUp:   c.OnMouseUp,
		OnHover:     c.OnHover,
		OnGesture:   c.OnGesture,
		OnFileDrop:  c.OnFileDrop,
		OnIMECommit: c.OnIMECommit,
		OnScroll:    c.OnScroll,
		AmendLayout: c.AmendLayout,
	}
}

func makeContainerA11Y(c *ContainerCfg) *AccessInfo {
	if c.A11Y != nil {
		return c.A11Y
	}
	return makeA11YInfo(c.A11YLabel, c.A11YDescription)
}

func deriveContainerA11YRole(c *ContainerCfg) AccessRole {
	if c.A11YRole != AccessRoleNone {
		return c.A11YRole
	}
	if c.IDScroll > 0 {
		return AccessRoleScrollArea
	}
	return AccessRoleNone
}

// buildContainerShape constructs a Shape from a ContainerCfg.
// Used by widgets that build containerView directly.
func buildContainerShape(cfg *ContainerCfg) *Shape {
	spacing, sizeBorder, radius, padding := applyContainerDefaults(cfg)
	shape := &Shape{
		shapeType:            shapeRectangle,
		ID:                   cfg.ID,
		IDFocus:              cfg.IDFocus,
		Axis:                 cfg.axis,
		ScrollbarOrientation: cfg.scrollbarOrientation,
		X:                    cfg.X,
		Y:                    cfg.Y,
		Width:                cfg.Width,
		MinWidth:             cfg.MinWidth,
		MaxWidth:             cfg.MaxWidth,
		Height:               cfg.Height,
		MinHeight:            cfg.MinHeight,
		MaxHeight:            cfg.MaxHeight,
		Clip:                 cfg.Clip,
		ClipContents:         cfg.ClipContents,
		FocusSkip:            cfg.FocusSkip,
		Spacing:              spacing,
		Sizing:               cfg.Sizing,
		Padding:              padding,
		HAlign:               cfg.HAlign,
		VAlign:               cfg.VAlign,
		TextDir:              cfg.TextDir,
		Radius:               radius,
		Color:                cfg.Color,
		fx:                   makeContainerEffects(cfg),
		SizeBorder:           sizeBorder,
		ColorBorder:          cfg.ColorBorder,
		Disabled:             cfg.Disabled,
		Float:                cfg.Float,
		FloatAutoFlip:        cfg.FloatAutoFlip,
		FloatAnchor:          cfg.FloatAnchor,
		FloatTieOff:          cfg.FloatTieOff,
		FloatOffsetX:         cfg.FloatOffsetX,
		FloatOffsetY:         cfg.FloatOffsetY,
		FloatZIndex:          cfg.FloatZIndex,
		IDScroll:             cfg.IDScroll,
		OverDraw:             cfg.OverDraw,
		ScrollMode:           cfg.ScrollMode,
		events:               makeContainerEvents(cfg),
		Hero:                 cfg.Hero,
		Wrap:                 cfg.Wrap,
		Overflow:             cfg.Overflow,
		Opacity:              cfg.Opacity.Get(1.0),
		A11YRole:             deriveContainerA11YRole(cfg),
		A11YState:            cfg.A11YState,
		A11Y:                 makeContainerA11Y(cfg),
	}
	applyFixedSizingConstraints(shape)
	return shape
}

// container is the fundamental layout builder. Factory
// functions (Column, Row, etc.) set axis then delegate here.
func container(cfg ContainerCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	// Resolve click handler.
	if cfg.OnAnyClick != nil {
		cfg.OnClick = cfg.OnAnyClick
	} else {
		cfg.OnClick = leftClickOnly(cfg.OnClick)
	}

	content := cfg.Content
	if cfg.IDScroll > 0 {
		content = make([]View, 0, len(cfg.Content)+2)
		content = append(content, cfg.Content...)
		content = appendScrollbar(content, cfg.ScrollbarCfgX,
			ScrollbarHorizontal, cfg.IDScroll)
		content = appendScrollbar(content, cfg.ScrollbarCfgY,
			ScrollbarVertical, cfg.IDScroll)
	}

	return &containerView{
		shape:       buildContainerShape(&cfg),
		content:     content,
		title:       cfg.Title,
		titleBG:     cfg.TitleBG,
		colorBorder: cfg.ColorBorder,
		disabled:    cfg.Disabled,
	}
}

// Column arranges content top to bottom.
func Column(cfg ContainerCfg) View {
	cfg.axis = AxisTopToBottom
	return container(cfg)
}

// Row arranges content left to right.
func Row(cfg ContainerCfg) View {
	cfg.axis = AxisLeftToRight
	return container(cfg)
}

// Wrap arranges content left to right, flowing to the next
// line when container width is exceeded.
func Wrap(cfg ContainerCfg) View {
	cfg.axis = AxisLeftToRight
	cfg.Wrap = true
	return container(cfg)
}

// Canvas does not arrange or layout its content.
func Canvas(cfg ContainerCfg) View {
	return container(cfg)
}

// Circle creates a circular container.
func Circle(cfg ContainerCfg) View {
	cfg.axis = AxisTopToBottom
	cv := container(cfg).(*containerView)
	cv.shape.shapeType = shapeCircle
	return cv
}

func appendScrollbar(content []View, override *ScrollbarCfg, orientation ScrollbarOrientation, idScroll uint32) []View {
	if override != nil {
		if override.Overflow == ScrollbarHidden {
			return content
		}
		merged := *override
		merged.Orientation = orientation
		merged.IDScroll = idScroll
		return append(content, Scrollbar(merged))
	}
	return append(content, Scrollbar(ScrollbarCfg{
		Orientation: orientation,
		IDScroll:    idScroll,
	}))
}

func invisibleContainerView() *containerView {
	cfg := ContainerCfg{
		Disabled: true,
		OverDraw: true,
		Padding:  NoPadding,
	}
	return &containerView{
		shape: buildContainerShape(&cfg),
	}
}
