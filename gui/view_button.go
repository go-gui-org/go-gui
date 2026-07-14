package gui

// ButtonCfg configures a clickable button. Without an OnClick handler,
// it renders as a styled label with no mouse interaction.
//
// Focus-driven keyboard interaction: when Focusable is true, pressing Space
// or Enter while focused triggers OnClick.
type ButtonCfg struct {
	Shadow   *BoxShadow
	Gradient *GradientDef

	// OnClick fires when the button is clicked or activated via
	// keyboard (Space/Enter). Required for interactive buttons;
	// omit for bubble-text labels.
	OnClick func(*Layout, *Event, *Window)

	// OnHover fires when the mouse enters or leaves the button.
	// The event's HoverEntered field indicates direction.
	OnHover func(*Layout, *Event, *Window)

	// AmendLayout runs after sizing. Use to reposition child
	// overlays or adjust layout post-arrange.
	AmendLayout func(*Layout, *Window)

	ID              string
	A11YLabel       string
	A11YDescription string
	Content         []View
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	Radius          Opt[float32]

	// BlurRadius controls the shadow blur. 0 = no shadow.
	BlurRadius float32

	FloatOffsetX float32
	FloatOffsetY float32
	Focusable    bool
	Width        float32
	Height       float32
	MinWidth     float32
	MaxWidth     float32
	MinHeight    float32
	MaxHeight    float32

	A11YState AccessState
	Color     Color

	// ColorHover is the background color on mouse hover.
	ColorHover Color

	// ColorFocus is the background color when keyboard-focused.
	ColorFocus Color

	// ColorClick is the background color during mouse press.
	ColorClick Color

	ColorBorder      Color
	ColorBorderFocus Color
	HAlign           Opt[HorizontalAlign]
	VAlign           Opt[VerticalAlign]

	Sizing      Sizing
	Float       bool
	FloatAnchor FloatAttach
	FloatTieOff FloatAttach
	Disabled    bool
	Invisible   bool

	// Accessibility
	A11YRole AccessRole
}

// buttonView wraps a containerView with per-button hover/focus
// colors, replacing per-frame closure allocations with pooled
// shapeButtonColors and package-level handler functions.
type buttonView struct {
	cv               *containerView
	userOnHover      func(*Layout, *Event, *Window)
	userAmendLayout  func(*Layout, *Window)
	colorHover       Color
	colorClick       Color
	colorFocus       Color
	colorBorderFocus Color
}

func (bv *buttonView) Content() []View { return bv.cv.Content() }

func (bv *buttonView) GenerateLayout(w *Window) Layout {
	layout := bv.cv.GenerateLayout(w)
	if layout.Shape.events != nil {
		bc := shapeButtonColors{
			ColorHover:       bv.colorHover,
			ColorClick:       bv.colorClick,
			ColorFocus:       bv.colorFocus,
			ColorBorderFocus: bv.colorBorderFocus,
			OnHover:          bv.userOnHover,
			OnAmend:          bv.userAmendLayout,
		}
		if w != nil {
			layout.Shape.bc = w.scratch.buttonColors.alloc(bc)
		} else {
			layout.Shape.bc = &bc
		}
		layout.Shape.events.AmendLayout = buttonAmendLayout
		layout.Shape.events.OnHover = buttonOnHover
	}
	return layout
}

func buttonAmendLayout(layout *Layout, w *Window) {
	if layout.Shape.Disabled ||
		!layout.Shape.hasEvents() ||
		layout.Shape.events.OnClick == nil {
		return
	}
	if w.IsFocus(layout.Shape.ID) {
		layout.Shape.Color = layout.Shape.bc.ColorFocus
		layout.Shape.ColorBorder = layout.Shape.bc.ColorBorderFocus
	}
	if layout.Shape.bc.OnAmend != nil {
		layout.Shape.bc.OnAmend(layout, w)
	}
}

func buttonOnHover(layout *Layout, e *Event, w *Window) {
	if layout.Shape.Disabled ||
		!layout.Shape.hasEvents() ||
		layout.Shape.events.OnClick == nil {
		return
	}
	w.setMouseCursor(CursorPointingHand)
	if !w.IsFocus(layout.Shape.ID) {
		layout.Shape.Color = layout.Shape.bc.ColorHover
	}
	if e.MouseButton == MouseLeft {
		layout.Shape.Color = layout.Shape.bc.ColorClick
	}
	if layout.Shape.bc.OnHover != nil {
		layout.Shape.bc.OnHover(layout, e, w)
	}
}

// Button creates a clickable button. Delegates to Row with
// package-level amend_layout for focus coloring and on_hover
// for cursor/color state changes. Colors are stored in a pooled
// shapeButtonColors to avoid per-frame closure allocations.
func Button(cfg ButtonCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}

	applyButtonDefaults(&cfg)

	d := &DefaultButtonStyle
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	radius := cfg.Radius.Get(d.Radius)
	hAlign := cfg.HAlign.Get(HAlignCenter)
	vAlign := cfg.VAlign.Get(VAlignMiddle)

	onClick := cfg.OnClick

	a11yRole := cfg.A11YRole
	if a11yRole == AccessRoleNone {
		a11yRole = AccessRoleButton
	}

	cv := Row(ContainerCfg{
		ID:              cfg.ID,
		Focusable:       cfg.Focusable,
		A11YRole:        a11yRole,
		A11YState:       cfg.A11YState,
		A11YLabel:       cfg.A11YLabel,
		A11YDescription: cfg.A11YDescription,
		Color:           cfg.Color,
		ColorBorder:     cfg.ColorBorder,
		SizeBorder:      Some(sizeBorder),
		BlurRadius:      cfg.BlurRadius,
		Shadow:          cfg.Shadow,
		Gradient:        cfg.Gradient,
		Padding:         cfg.Padding,
		Radius:          Some(radius),
		Width:           cfg.Width,
		Height:          cfg.Height,
		MinWidth:        cfg.MinWidth,
		MaxWidth:        cfg.MaxWidth,
		MinHeight:       cfg.MinHeight,
		MaxHeight:       cfg.MaxHeight,
		Sizing:          cfg.Sizing,
		Disabled:        cfg.Disabled,
		HAlign:          hAlign,
		VAlign:          vAlign,
		Float:           cfg.Float,
		FloatAnchor:     cfg.FloatAnchor,
		FloatTieOff:     cfg.FloatTieOff,
		FloatOffsetX:    cfg.FloatOffsetX,
		FloatOffsetY:    cfg.FloatOffsetY,
		OnClick:         onClick,
		ClickOnSpace:    true,
		ClickOnEnter:    true,
		Content:         cfg.Content,
	}).(*containerView)

	return &buttonView{
		cv:               cv,
		colorHover:       cfg.ColorHover,
		colorClick:       cfg.ColorClick,
		colorFocus:       cfg.ColorFocus,
		colorBorderFocus: cfg.ColorBorderFocus,
		userOnHover:      cfg.OnHover,
		userAmendLayout:  cfg.AmendLayout,
	}
}

// CommandButton creates a button wired to a registered
// command. Auto-fills label from Command.Label when Content
// is nil. Auto-disables via CanExecute. Wires OnClick to
// Command.Execute.
func CommandButton(w *Window, cmdID string, cfg ButtonCfg) View {
	cmd, ok := w.CommandByID(cmdID)
	if !ok {
		return Text(TextCfg{
			Text:      "unknown command: " + cmdID,
			TextStyle: TextStyle{Color: Red},
		})
	}

	// Auto-fill content from command label.
	if cfg.Content == nil && cmd.Label != "" {
		label := cmd.Label
		hint := cmd.Shortcut.String()
		if hint != "" {
			label += "  " + hint
		}
		cfg.Content = []View{
			Text(TextCfg{Text: label}),
		}
	}

	// Wire OnClick to command execute.
	if cfg.OnClick == nil {
		cmdExec := cmd.Execute
		cID := cmdID
		cfg.OnClick = func(_ *Layout, e *Event, w *Window) {
			if w.CommandCanExecute(cID) && cmdExec != nil {
				cmdExec(e, w)
			}
		}
	}

	// Auto-disable via CanExecute.
	if cmd.CanExecute != nil && !cmd.CanExecute(w) {
		cfg.Disabled = true
	}

	return Button(cfg)
}

func applyButtonDefaults(cfg *ButtonCfg) {
	d := &DefaultButtonStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorClick.IsSet() {
		cfg.ColorClick = d.ColorClick
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
}
