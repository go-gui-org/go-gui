package gui

// ButtonCfg configures a clickable button. Without an OnClick handler,
// it renders as a styled label with no mouse interaction.
//
// Buttons are focusable by default — pressing Space or Enter while
// focused triggers OnClick. Set FocusDisabled to opt out.
type ButtonCfg struct {
	Shadow   *BoxShadow
	Gradient *GradientDef

	// OnClick fires when the button is clicked or activated via
	// keyboard (Space/Enter). Required for interactive buttons;
	// omit for bubble-text labels.
	OnClick func(EventCtx)

	// OnHover fires when the mouse enters or leaves the button.
	// The event's HoverEntered field indicates direction.
	OnHover func(EventCtx)

	// AmendLayout runs after sizing. Use to reposition child
	// overlays or adjust layout post-arrange.
	AmendLayout func(EventCtx)

	ID string `gui:"required,focus"`
	A11YCfg
	Content    []View
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]

	// BlurRadius controls the shadow blur. 0 = no shadow.
	BlurRadius float32

	FloatOffsetX float32
	FloatOffsetY float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Width         float32
	Height        float32
	MinWidth      float32
	MaxWidth      float32
	MinHeight     float32
	MaxHeight     float32

	A11YState AccessState
	Color     Color

	// Colors sets the per-state colors. Flat(c) covers the common
	// "keep one appearance" case in a single line.
	//
	// Color above is the shorthand for Colors.Base and wins over it
	// when both are set.
	Colors ColorSet

	HAlign Opt[HorizontalAlign]
	VAlign Opt[verticalAlign]

	Sizing      Sizing
	Float       bool
	FloatAnchor floatAttach
	FloatTieOff floatAttach
	Disabled    bool
	Invisible   bool
	// opticalDigitLabel centres this button's label on the face's
	// figure band rather than its cap band. Digits measure shorter than
	// caps, so a digit-only label centred on the cap band lands as far
	// low as an uncorrected one sits high — the overshoot measured on
	// NumericInput (issue #346).
	//
	// Unexported and opt-in, like InputCfg.opticalDigitCenter: the
	// alphabet is a guarantee only the widget building the label can
	// make. A date picker's day cell can; an app's button holding a
	// count it also relabels cannot.
	opticalDigitLabel bool

	// Accessibility
	A11YRole AccessRole
}

func buttonAmendLayout(ctx EventCtx) {
	// Unconditional: the correction is a property of the label, not of
	// the button's state, and the early return below covers only the
	// hover/focus colouring and the caller's own hook.
	//
	// The cap band, not the run: a button is a control whose text is a
	// label, and a strip of tabs or a toolbar row is a list at a
	// regular pitch, where measuring each run would drop a
	// descender-free label and leave its descending neighbour behind.
	// A label holding digits by construction says so and takes the
	// figure band; an icon child opts itself back onto its own ink
	// through the style's glyph role (issue #346).
	band := opticalBandCap
	if ctx.Layout.Shape.bc != nil && ctx.Layout.Shape.bc.opticalDigits {
		band = opticalBandDigit
	}
	opticalCenterChildren(ctx, band)
	if ctx.Layout.Shape.Disabled ||
		!ctx.Layout.Shape.hasEvents() ||
		ctx.Layout.Shape.events.OnClick == nil {
		return
	}
	if ctx.Window.IsFocus(ctx.Layout.Shape.idKey()) {
		ctx.Layout.Shape.Color = ctx.Layout.Shape.bc.ColorFocus
		ctx.Layout.Shape.ColorBorder = ctx.Layout.Shape.bc.ColorBorderFocus
		applyFocusRingShadow(ctx.Layout.Shape, ctx.Window,
			ctx.Layout.Shape.bc.focusRing)
	}
	if ctx.Layout.Shape.bc.OnAmend != nil {
		ctx.Layout.Shape.bc.OnAmend(EventCtx{ctx.Layout, nil, ctx.Window})
	}
}

func buttonOnHover(ctx EventCtx) {
	if ctx.Layout.Shape.Disabled ||
		!ctx.Layout.Shape.hasEvents() ||
		ctx.Layout.Shape.events.OnClick == nil {
		return
	}
	ctx.Window.setMouseCursor(CursorPointingHand)
	if !ctx.Window.IsFocus(ctx.Layout.Shape.idKey()) {
		ctx.Layout.Shape.Color = ctx.Layout.Shape.bc.ColorHover
	}
	if ctx.Event.MouseButton == MouseLeft {
		ctx.Layout.Shape.Color = ctx.Layout.Shape.bc.colorClick
	}
	if ctx.Layout.Shape.bc.OnHover != nil {
		ctx.Layout.Shape.bc.OnHover(EventCtx{ctx.Layout, ctx.Event, ctx.Window})
	}
}

// TextButton is the thin form of Button for the common case: one
// label on a clickable button. The padding default (8, 16, 8, 16) is
// the most common explicit padding across the examples; a caller that
// wants the theme default or a custom inset uses Button directly.
func TextButton(id, label string, onClick func(EventCtx)) View {
	return Button(ButtonCfg{
		ID:      id,
		OnClick: onClick,
		Padding: NewPadding(8, 16, 8, 16),
		Content: []View{
			Text(TextCfg{Text: label}),
		},
	})
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
	requireFocusID("Button", cfg.FocusDisabled, cfg.ID)

	d := &defaultButtonStyle
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
		ID:           cfg.ID,
		Focusable:    !cfg.FocusDisabled,
		A11YRole:     a11yRole,
		A11YState:    cfg.A11YState,
		A11YCfg:      cfg.A11YCfg,
		Color:        cfg.Colors.Base,
		ColorBorder:  cfg.Colors.Border,
		SizeBorder:   Some(sizeBorder),
		BlurRadius:   cfg.BlurRadius,
		Shadow:       cfg.Shadow,
		Gradient:     cfg.Gradient,
		Padding:      cfg.Padding,
		Radius:       Some(radius),
		Width:        cfg.Width,
		Height:       cfg.Height,
		MinWidth:     cfg.MinWidth,
		MaxWidth:     cfg.MaxWidth,
		MinHeight:    cfg.MinHeight,
		MaxHeight:    cfg.MaxHeight,
		Sizing:       cfg.Sizing,
		Disabled:     cfg.Disabled,
		HAlign:       hAlign,
		VAlign:       vAlign,
		Float:        cfg.Float,
		FloatAnchor:  cfg.FloatAnchor,
		FloatTieOff:  cfg.FloatTieOff,
		FloatOffsetX: cfg.FloatOffsetX,
		FloatOffsetY: cfg.FloatOffsetY,
		OnClick:      onClick,
		ClickOnSpace: true,
		ClickOnEnter: true,
		// A button's label takes the optical correction (issue #346);
		// tabs, command buttons and every other widget built on Button
		// inherit it. The hook here is not the one that runs —
		// buttonAmendLayout replaces it and picks the band — it is what
		// guarantees the shape gets an events record at all, which a
		// bubble-text Button otherwise has no reason to allocate.
		//
		// Not routed through cv.userAmendLayout, because
		// that slot is reached from buttonAmendLayout, which returns
		// early for a disabled or click-less button — a disabled label
		// would then sit a pixel above the enabled one beside it. This
		// also guarantees the shape gets an events record, which a
		// bubble-text Button otherwise has no reason to allocate.
		AmendLayout: opticalCenterText,
		Content:     cfg.Content,
	}).(*containerView)

	cv.isButton = true
	cv.colorHover = cfg.Colors.Hover
	cv.colorClick = cfg.Colors.Click
	cv.colorFocus = cfg.Colors.Focus
	cv.colorBorderFocus = cfg.Colors.BorderFocus
	cv.userOnHover = cfg.OnHover
	cv.userAmendLayout = cfg.AmendLayout
	cv.opticalDigits = cfg.opticalDigitLabel

	return cv
}

// commandButtonIDScope namespaces auto-filled CommandButton IDs.
// Menu items are keyed by raw command ID (see MenuItemCfg.CommandID),
// and menu item shapes carry that ID; without the scope a menubar and
// a CommandButton for the same command would produce two shapes with
// the same ID in one window, making focus ambiguous.
const commandButtonIDScope = "cmdbtn"

// CommandButton creates a button wired to a registered
// command. Construction is deferred to layout time via ViewFunc
// so the caller does not need to pass *Window. Auto-fills label
// from Command.Label when Content is nil. Auto-fills ID from
// cmdID. Auto-disables via CanExecute. Wires OnClick to
// Command.Execute.
//
// The auto-filled ID is commandButtonIDScope scoped to cmdID. Set cfg.ID
// explicitly when placing two buttons for the same command in one
// window, otherwise both get the same focus ID.
func CommandButton(cmdID string, cfg ButtonCfg) View {
	return viewFunc(func(w *Window) View {
		cmd, ok := w.CommandByID(cmdID)
		if !ok {
			return Text(TextCfg{
				Text:      "unknown command: " + cmdID,
				TextStyle: TextStyle{Color: Red},
			})
		}

		// Focus traversal is keyed by ID (see isFocusedTarget), so
		// Focusable: true is a silent no-op without one.
		if cfg.ID == "" {
			cfg.ID = ScopeID(commandButtonIDScope, cmdID)
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
			cfg.OnClick = func(ctx EventCtx) {
				if ctx.Window.commandCanExecute(cID) && cmdExec != nil {
					cmdExec(ctx.Event, ctx.Window)
				}
			}
		}

		// Auto-disable via CanExecute.
		if cmd.CanExecute != nil && !cmd.CanExecute(w) {
			cfg.Disabled = true
		}

		return Button(cfg)
	})
}

func applyButtonDefaults(cfg *ButtonCfg) {
	d := &defaultButtonStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
}
