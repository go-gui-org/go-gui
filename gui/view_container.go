package gui

// ContainerCfg configures container views ([Column], [Row], [Canvas],
// [Circle], [Wrap]). Containers layout children vertically,
// horizontally, or freely with sizing, alignment, scrolling,
// floating, borders, and event handling.
//
// # Title and group-box style
//
// When Title is set, the container renders a group-box label in the
// top border — like an HTML fieldset/legend. TitleBG must match the
// parent background color to erase the border behind the title text.
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

	// ClickButton filters OnClick by mouse button (0 = any).
	// Set to MouseLeft for left-click-only widgets; avoids the
	// per-frame closure allocation from leftClickOnly.
	ClickButton MouseButton

	// ClickOnSpace fires OnClick on spacebar via the char dispatch
	// path. Avoids the per-frame closure allocation from
	// spacebarToClick.
	ClickOnSpace bool

	// ClickOnEnter fires OnClick on Enter key via the key-down
	// dispatch path. Avoids the per-frame closure allocation from
	// enterToClick.
	ClickOnEnter bool

	// OnScroll fires when the container receives scroll events.
	// Requires Scrollable and a scrollable Overflow/ScrollMode.
	OnScroll func(*Layout, *Window)

	// AmendLayout runs after sizing to reposition overlays
	// (color picker circles, splitter handles) or manage hover
	// indicators. Coordinates are absolute.
	AmendLayout func(*Layout, *Window)

	OnHover     func(*Layout, *Event, *Window)
	OnGesture   func(*Layout, *Event, *Window)
	OnFileDrop  func(*Layout, *Event, *Window)
	OnIMECommit func(*Layout, string, *Window)

	// ScrollbarCfgX/Y override scrollbar appearance for this
	// container. nil uses theme defaults. Only active when
	// Scrollable.
	ScrollbarCfgX *ScrollbarCfg
	ScrollbarCfgY *ScrollbarCfg

	// Identity
	ID string

	// Title renders a group-box label in the top border. See the
	// type doc for TitleBG requirements.
	Title           string
	A11YLabel       string
	A11YDescription string

	// Content holds the child views displayed inside this container.
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
	Focusable bool

	// Scrollable opts the container into the scroll system. Scroll
	// state is keyed by Cfg.ID — pass that same id to
	// Window.ScrollVerticalTo and friends. Requires a non-empty ID.
	Scrollable   bool
	FloatOffsetX float32
	FloatOffsetY float32

	// Position
	X float32
	Y float32

	A11YState AccessState

	// TitleBG is the background color behind the group-box title
	// text. Must match the parent container's background to erase
	// the border line behind the title.
	TitleBG Color

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
	shapeType            shapeType // zero = shapeRectangle
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
// ContainerCfg is stored by value; the Shape is built per
// GenerateLayout call using pooled allocs (allocShape,
// allocEventHandlers, allocEffects). This eliminates the
// factory-phase &Shape{} heap alloc at the cost of rebuilding
// ~70 fields each frame. Cached views (combobox/command-palette
// dropdowns) re-pay the build cost without heap allocs.
type containerView struct {
	cfg     ContainerCfg
	content []View

	// Button-specific fields — set only by Button (step 4 fold-in).
	isButton         bool
	userOnHover      func(*Layout, *Event, *Window)
	userAmendLayout  func(*Layout, *Window)
	colorHover       Color
	colorClick       Color
	colorFocus       Color
	colorBorderFocus Color
}

func (cv *containerView) Content() []View { return cv.content }

func (cv *containerView) GenerateLayout(w *Window) Layout {
	layout := Layout{
		Shape: w.allocShape(buildContainerShape(&cv.cfg, w)),
	}
	if cv.isButton && layout.Shape.events != nil {
		bc := shapeButtonColors{
			ColorHover:       cv.colorHover,
			ColorClick:       cv.colorClick,
			ColorFocus:       cv.colorFocus,
			ColorBorderFocus: cv.colorBorderFocus,
			OnHover:          cv.userOnHover,
			OnAmend:          cv.userAmendLayout,
		}
		if w != nil {
			layout.Shape.bc = w.scratch.buttonColors.alloc(bc)
		} else {
			layout.Shape.bc = &bc
		}
		layout.Shape.events.AmendLayout = buttonAmendLayout
		layout.Shape.events.OnHover = buttonOnHover
	}
	addGroupBoxTitle(cv.cfg.Title, cv.cfg.TitleBG, cv.cfg.ColorBorder,
		cv.cfg.Disabled, w, &layout)
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

func makeContainerEffects(c *ContainerCfg) (shapeEffects, bool) {
	if c.Shadow == nil && c.Gradient == nil &&
		c.BorderGradient == nil && c.Shader == nil &&
		c.ColorFilter == nil && c.BlurRadius == 0 {
		return shapeEffects{}, false
	}
	return shapeEffects{
		Shadow:         c.Shadow,
		Gradient:       c.Gradient,
		BorderGradient: c.BorderGradient,
		Shader:         c.Shader,
		ColorFilter:    c.ColorFilter,
		BlurRadius:     c.BlurRadius,
	}, true
}

func makeContainerEvents(c *ContainerCfg) (eventHandlers, bool) {
	if c.OnClick == nil && c.OnChar == nil &&
		c.OnKeyDown == nil && c.OnKeyUp == nil &&
		c.OnMouseMove == nil && c.OnMouseUp == nil &&
		c.OnHover == nil && c.OnGesture == nil &&
		c.OnFileDrop == nil && c.OnIMECommit == nil &&
		c.OnScroll == nil && c.AmendLayout == nil {
		return eventHandlers{}, false
	}
	return eventHandlers{
		OnClick:      c.OnClick,
		OnChar:       c.OnChar,
		OnKeyDown:    c.OnKeyDown,
		OnKeyUp:      c.OnKeyUp,
		OnMouseMove:  c.OnMouseMove,
		OnMouseUp:    c.OnMouseUp,
		OnHover:      c.OnHover,
		OnGesture:    c.OnGesture,
		OnFileDrop:   c.OnFileDrop,
		OnIMECommit:  c.OnIMECommit,
		OnScroll:     c.OnScroll,
		AmendLayout:  c.AmendLayout,
		ClickButton:  c.ClickButton,
		ClickOnSpace: c.ClickOnSpace,
		ClickOnEnter: c.ClickOnEnter,
	}, true
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
	if c.Scrollable {
		return AccessRoleScrollArea
	}
	return AccessRoleNone
}

// buildContainerShape constructs a Shape from a ContainerCfg.
// Uses pooled allocs for effects and events via w.
func buildContainerShape(cfg *ContainerCfg, w *Window) Shape {
	RequireScrollID("container", cfg.Scrollable, cfg.ID)
	spacing, sizeBorder, radius, padding := applyContainerDefaults(cfg)
	shapeType := cfg.shapeType
	if shapeType == shapeNone {
		shapeType = shapeRectangle
	}
	shape := Shape{
		shapeType:            shapeType,
		ID:                   cfg.ID,
		Focusable:            cfg.Focusable,
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
		Scrollable:           cfg.Scrollable,
		OverDraw:             cfg.OverDraw,
		ScrollMode:           cfg.ScrollMode,
		Hero:                 cfg.Hero,
		Wrap:                 cfg.Wrap,
		Overflow:             cfg.Overflow,
		Opacity:              cfg.Opacity.Get(1.0),
		A11YRole:             deriveContainerA11YRole(cfg),
		A11YState:            cfg.A11YState,
		A11Y:                 makeContainerA11Y(cfg),
	}
	if fx, ok := makeContainerEffects(cfg); ok {
		shape.fx = w.allocEffects(fx)
	}
	if ev, ok := makeContainerEvents(cfg); ok {
		shape.events = w.allocEventHandlers(ev)
	}
	applyFixedSizingConstraints(&shape)
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
		cfg.ClickButton = MouseLeft
	}

	content := cfg.Content
	if cfg.Scrollable {
		content = make([]View, 0, len(cfg.Content)+2)
		content = append(content, cfg.Content...)
		content = appendScrollbar(content, cfg.ScrollbarCfgX,
			ScrollbarHorizontal, cfg.ID)
		content = appendScrollbar(content, cfg.ScrollbarCfgY,
			ScrollbarVertical, cfg.ID)
	}

	return &containerView{
		cfg:     cfg,
		content: content,
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
	cfg.shapeType = shapeCircle
	return container(cfg)
}

func appendScrollbar(content []View, override *ScrollbarCfg, orientation ScrollbarOrientation, id string) []View {
	if override != nil {
		if override.Overflow == ScrollbarHidden {
			return content
		}
		merged := *override
		merged.Orientation = orientation
		merged.ScrollID = id
		return append(content, Scrollbar(merged))
	}
	return append(content, Scrollbar(ScrollbarCfg{
		Orientation: orientation,
		ScrollID:    id,
	}))
}

// invisibleContainerView provides a singleton immutable
// invisible placeholder. Since containerView is now
// cfg-by-value (no mutable template shape), a single instance
// is safe across the lifetime of the package.
var invisibleContainerViewSingleton = &containerView{
	cfg: ContainerCfg{
		Disabled: true,
		OverDraw: true,
		Padding:  NoPadding,
	},
}

func invisibleContainerView() *containerView {
	return invisibleContainerViewSingleton
}
