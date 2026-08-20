package gui

// ButtonVariant picks a button's hierarchy position (visual-refresh
// §6). The zero value is today's button, so existing call sites are
// unaffected in structure.
//
// exportaudit:keep — phase-6 variant surface; consumers land with the
// next release (docs/specs/visual-refresh.md).
type ButtonVariant uint8

const (
	// ButtonSecondary is the zero value: the theme's plain button.
	// exportaudit:keep — phase-6 variant surface (see above).
	ButtonSecondary ButtonVariant = iota
	// ButtonPrimary fills with the accent and pairs the label with
	// ColorTextOnAccent. One accent-filled primary per surface is the
	// convention (docs/style-guide.md).
	// exportaudit:keep — phase-6 variant surface (see above).
	ButtonPrimary
	// ButtonGhost has no fill and no border; hover fills only.
	// exportaudit:keep — phase-6 variant surface (see above).
	ButtonGhost
	// ButtonDanger fills with the error color and pairs the label
	// with ColorTextOnAccent.
	// exportaudit:keep — phase-6 variant surface (see above).
	ButtonDanger
)

// ButtonCfg configures a clickable button. Without an OnClick handler,
// it renders as a styled label with no mouse interaction.
//
// Buttons are focusable by default — pressing Space or Enter while
// focused triggers OnClick. Set FocusDisabled to opt out.
type ButtonCfg struct {
	Shadow   *BoxShadow
	Gradient *GradientDef

	// Label, when set, builds the button's text here: Button wraps it
	// in a Text carrying the variant's label color (ColorTextOnAccent
	// on the filled variants). This is the path a variant button
	// should use — see TextButton. When empty, Content is used as-is.
	Label string

	// Variant picks the button's hierarchy position. Zero value
	// ButtonSecondary is today's button. Colors set on this cfg keep
	// precedence over the variant's fills, state by state.
	Variant ButtonVariant

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

	// Filled variants recolor the labels that took the default color
	// (visual-refresh §6). The color is captured at generation — this
	// amend is a plain func with no closure — and the style's
	// defaulted-color marker says which shapes may be recolored, so an
	// explicitly colored label stays the caller's choice. Mutates the
	// per-frame shape text config, never the caller's TextCfg.
	if bc := ctx.Layout.Shape.bc; bc != nil && bc.labelColor.IsSet() {
		stampButtonLabelColor(ctx.Layout, bc.labelColor)
	}

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

// stampButtonLabelColor recolors a button subtree's label shapes to c.
// Only shapes whose style took the DefaultTextStyle fallback — the
// defaulted-color marker — are touched; a label the caller colored
// itself is a deliberate choice and is left alone. Runs after arrange,
// when every text shape carries its resolved per-frame style.
func stampButtonLabelColor(l *Layout, c Color) {
	for i := range l.Children {
		child := &l.Children[i]
		if child.Shape == nil {
			continue
		}
		if ts := child.Shape.TC; ts != nil &&
			child.Shape.shapeType == shapeText &&
			ts.TextStyle != nil &&
			ts.TextStyle.defaultedColor {
			ts.TextStyle.Color = c
		}
		stampButtonLabelColor(child, c)
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

// TextButtonVariant is TextButton with a variant (visual-refresh §6).
// It takes the Label path, so the button builds the text and applies
// the variant's label color itself — the one way a filled variant's
// label can be recolored (see ButtonCfg.Label).
func TextButtonVariant(id, label string, v ButtonVariant, onClick func(EventCtx)) View {
	return Button(ButtonCfg{
		ID:      id,
		Variant: v,
		Label:   label,
		OnClick: onClick,
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

	// Variant fills (visual-refresh §6). Secondary is the zero value
	// and keeps the mirror; the rest read the installed theme at
	// generation time, like every other theme-derived style. The
	// ThemeMaker derives the variant geometry from the base button
	// style, so padding, border and radius still resolve from one
	// source.
	d := &defaultButtonStyle
	labelColor := Color{}
	switch cfg.Variant {
	case ButtonPrimary:
		d = &guiTheme.ButtonStylePrimary
		labelColor = guiTheme.ColorTextOnAccent
	case ButtonGhost:
		d = &guiTheme.ButtonStyleGhost
	case ButtonDanger:
		d = &guiTheme.ButtonStyleDanger
		labelColor = guiTheme.ColorTextOnAccent
	}

	applyButtonDefaults(&cfg, d)
	requireFocusID("Button", cfg.FocusDisabled, cfg.ID)

	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	radius := cfg.Radius.Get(d.Radius)
	hAlign := cfg.HAlign.Get(HAlignCenter)
	vAlign := cfg.VAlign.Get(VAlignMiddle)

	onClick := cfg.OnClick

	a11yRole := cfg.A11YRole
	if a11yRole == AccessRoleNone {
		a11yRole = AccessRoleButton
	}

	// The Label path builds the button's text here, so the variant's
	// label color can be applied directly (the caller's own Text
	// children cannot be recolored — see TextStyle.defaultedColor).
	content := cfg.Content
	if cfg.Label != "" {
		labelStyle := DefaultTextStyle
		if labelColor.IsSet() {
			labelStyle.Color = labelColor
		}
		content = append(
			[]View{Text(TextCfg{Text: cfg.Label, TextStyle: labelStyle})},
			content...,
		)
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
		Content:     content,
	}).(*containerView)

	cv.isButton = true
	cv.colorHover = cfg.Colors.Hover
	cv.colorClick = cfg.Colors.Click
	cv.colorFocus = cfg.Colors.Focus
	cv.colorBorderFocus = cfg.Colors.BorderFocus
	cv.labelColor = labelColor
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

// applyButtonDefaults resolves the cfg's colors against the variant
// style: the caller's own Colors keep precedence per state, and every
// state the caller left unset takes the variant's value (visual-refresh
// §6). d is the style the variant resolved to — the mirror for the
// zero-value secondary, a Theme style otherwise.
func applyButtonDefaults(cfg *ButtonCfg, d *buttonStyle) {
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
}
