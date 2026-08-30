package gui

// DialogType identifies the kind of dialog.
type dialogType uint8

// DialogType constants.
const (
	DialogMessage dialogType = iota
	DialogConfirm
	DialogPrompt
	DialogCustom
)

// DialogButton identifies which button of a confirm dialog receives
// initial keyboard focus. The zero value (DialogButtonNo) keeps the safe
// default for destructive actions.
type dialogButton uint8

// DialogButton constants.
const (
	dialogButtonNo dialogButton = iota
	dialogButtonYes
)

const dialogBaseFocusID = "__gui_dialog__"

// DialogCfg configures a modal dialog.
type DialogCfg struct {
	// TitleTextStyle styles the dialog title. Zero takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	TitleTextStyle TextStyle
	TextStyle      TextStyle

	OnOkYes    func(*Window)
	OnCancelNo func(*Window)
	OnReply    func(string, *Window)

	Title string
	Body  string
	Reply string
	ID    string

	CustomContent []View

	Padding    Padding
	SizeBorder Opt[float32]

	MinWidth Opt[float32]
	MaxWidth Opt[float32]

	Radius Opt[float32]

	Width     float32
	Height    float32
	MinHeight float32
	MaxHeight float32

	FocusID      string
	oldFocusID   string
	Color        Color
	ColorBorder  Color
	DialogType   dialogType
	AlignButtons HorizontalAlign // ergonomics-audit:opt-plain — zero (HAlignStart) is the natural default; no distinct unset behavior

	// DefaultButton selects which confirm-dialog button gets initial
	// keyboard focus (so Enter/Space activate it). Defaults to
	// DialogButtonNo, preserving the safe default for destructive
	// actions. Ignored for non-confirm dialog types.
	defaultButton dialogButton

	// unexported
	visible bool
}

// dialogViewGenerator builds the dialog overlay view from cfg.
func dialogViewGenerator(cfg DialogCfg) View {
	applyDialogDefaults(&cfg)
	dn := &DefaultDialogStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)
	minWidth := cfg.MinWidth.Get(dn.MinWidth)
	maxWidth := cfg.MaxWidth.Get(dn.MaxWidth)

	var content []View

	// Title.
	if cfg.Title != "" {
		content = append(content, Text(TextCfg{
			Text:      cfg.Title,
			TextStyle: cfg.TitleTextStyle,
		}))
	}

	// Body (unless custom).
	if cfg.DialogType != DialogCustom && cfg.Body != "" {
		content = append(content, Text(TextCfg{
			Text:      cfg.Body,
			TextStyle: cfg.TextStyle,
			Mode:      TextModeWrap,
		}))
	}

	// Type-specific content.
	switch cfg.DialogType {
	case DialogMessage:
		content = append(content, messageView(cfg))
	case DialogConfirm:
		content = append(content, confirmView(cfg))
	case DialogPrompt:
		content = append(content, promptView(cfg)...)
	case DialogCustom:
		content = append(content, cfg.CustomContent...)
	}

	return Column(ContainerCfg{
		ID:          reservedDialogID,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		BlurRadius:  dn.BlurRadius,
		Shadow:      dn.Shadow,
		Padding:     cfg.Padding,
		Width:       cfg.Width,
		Height:      cfg.Height,
		MinWidth:    minWidth,
		MinHeight:   cfg.MinHeight,
		MaxWidth:    maxWidth,
		MaxHeight:   cfg.MaxHeight,
		Float:       true,
		FloatAnchor: FloatMiddleCenter,
		FloatTieOff: FloatMiddleCenter,
		Spacing:     Some(SpacingMedium),
		OnKeyDown:   dialogKeyDown(cfg),
		A11YRole:    AccessRoleDialog,
		A11YState:   AccessStateModal,
		Content:     content,
	})
}

// messageView returns an OK button row.
func messageView(cfg DialogCfg) View {
	onOkYes := cfg.OnOkYes
	return Row(ContainerCfg{
		Sizing:     FillFit,
		HAlign:     cfg.AlignButtons,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content: []View{
			Button(ButtonCfg{
				ID:      cfg.FocusID,
				Content: []View{Text(TextCfg{Text: "OK"})},
				OnClick: func(ctx EventCtx) {
					ctx.Window.DialogDismiss()
					if onOkYes != nil {
						onOkYes(ctx.Window)
					}
				},
			}),
		},
	})
}

// confirmView returns Yes/No button row.
func confirmView(cfg DialogCfg) View {
	onOkYes := cfg.OnOkYes
	onCancelNo := cfg.OnCancelNo
	return Row(ContainerCfg{
		Sizing:     FillFit,
		HAlign:     cfg.AlignButtons,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Spacing:    Some(SpacingMedium),
		Content: []View{
			Button(ButtonCfg{
				ID:      ScopeIDN(cfg.FocusID, "", 1),
				Content: []View{Text(TextCfg{Text: "Yes"})},
				OnClick: func(ctx EventCtx) {
					ctx.Window.DialogDismiss()
					if onOkYes != nil {
						onOkYes(ctx.Window)
					}
				},
			}),
			Button(ButtonCfg{
				ID:      cfg.FocusID,
				Content: []View{Text(TextCfg{Text: "No"})},
				OnClick: func(ctx EventCtx) {
					ctx.Window.DialogDismiss()
					if onCancelNo != nil {
						onCancelNo(ctx.Window)
					}
				},
			}),
		},
	})
}

// promptView returns input + OK/Cancel button row.
func promptView(cfg DialogCfg) []View {
	onReply := cfg.OnReply
	onCancelNo := cfg.OnCancelNo

	var views []View

	views = append(views, Input(InputCfg{
		ID:     cfg.FocusID,
		Text:   cfg.Reply,
		Sizing: FillFit,
		OnTextChanged: func(text string, ctx EventCtx) {
			ctx.Window.dialogCfg.Reply = text
		},
	}))

	views = append(views, Row(ContainerCfg{
		Sizing:     FillFit,
		HAlign:     cfg.AlignButtons,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Spacing:    Some(SpacingMedium),
		Content: []View{
			Button(ButtonCfg{
				ID:       ScopeIDN(cfg.FocusID, "", 1),
				Disabled: len(cfg.Reply) == 0,
				Content:  []View{Text(TextCfg{Text: "OK"})},
				OnClick: func(ctx EventCtx) {
					reply := ctx.Window.dialogCfg.Reply
					ctx.Window.DialogDismiss()
					if onReply != nil {
						onReply(reply, ctx.Window)
					}
				},
			}),
			Button(ButtonCfg{
				ID:      ScopeIDN(cfg.FocusID, "", 2),
				Content: []View{Text(TextCfg{Text: "Cancel"})},
				OnClick: func(ctx EventCtx) {
					ctx.Window.DialogDismiss()
					if onCancelNo != nil {
						onCancelNo(ctx.Window)
					}
				},
			}),
		},
	}))

	return views
}

// dialogKeyDown handles Escape to dismiss dialog.
func dialogKeyDown(cfg DialogCfg) func(EventCtx) {
	onCancelNo := cfg.OnCancelNo
	return func(ctx EventCtx) {
		if ctx.Event.KeyCode == KeyEscape {
			ctx.Window.DialogDismiss()
			if onCancelNo != nil {
				onCancelNo(ctx.Window)
			}
			ctx.Consume()
			return
		}
		if ctx.Event.KeyCode == KeyC &&
			ctx.Event.Modifiers.HasAny(ModCtrl, ModSuper) &&
			cfg.Body != "" {
			ctx.Window.SetClipboard(cfg.Body)
			ctx.Consume()
		}
	}
}

func applyDialogDefaults(cfg *DialogCfg) {
	d := &DefaultDialogStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
	if cfg.TitleTextStyle == (TextStyle{}) {
		cfg.TitleTextStyle = d.titleTextStyle
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.FocusID == "" {
		cfg.FocusID = dialogBaseFocusID
	}
	// The dialog's controls hang off a container carrying
	// reservedDialogID, so a plain-leaf focus ID resolves under it —
	// while SetFocus and retainDialogFocus, which run outside layout
	// generation, would keep passing the bare leaf and focus nothing.
	// Scoping it here makes it absolute, so the shape's identity is
	// exactly this string, and it keeps the ScopeIDN-derived siblings
	// (":1", ":2") in the same namespace instead of a separate one.
	// Idempotent, and it leaves a caller's already-composed ID alone.
	cfg.FocusID = resolveLeaf(reservedDialogID, cfg.FocusID)
	// HAlignStart is 0 (zero value), so explicit HAlignStart cannot be
	// distinguished from unset. Use HAlignLeft for left alignment.
	if cfg.AlignButtons == HAlignStart {
		cfg.AlignButtons = d.AlignButtons
	}
}

// dialogFocusID returns the focus ID of the button that should receive
// initial keyboard focus. For confirm dialogs with DefaultButton set to
// DialogButtonYes, this is the "Yes" button (scoped ":1"); otherwise
// the base focus ID (the "No"/"OK"/input element).
func dialogFocusID(cfg DialogCfg) string {
	if cfg.DialogType == DialogConfirm && cfg.defaultButton == dialogButtonYes {
		return ScopeIDN(cfg.FocusID, "", 1)
	}
	return cfg.FocusID
}

// Dialog shows a modal dialog.
func (w *Window) Dialog(cfg DialogCfg) {
	applyDialogDefaults(&cfg)
	cfg.visible = true
	cfg.oldFocusID = w.viewState.focusID
	w.dialogCfg = cfg
	// The dialog overlay is built during a full layout pass, so the flag has
	// to be set explicitly: a caller outside the event path (a native menu
	// action, a QueueCommand from a worker) leaves the window otherwise idle,
	// and the render-only frames a blinking cursor produces reuse the existing
	// layout tree — the dialog would stay invisible until an unrelated event
	// forced a rebuild. wakeMain pairs with the flag for the same reason:
	// without it a sleeping backend never learns the flag was set.
	w.markLayoutRefresh()
	w.wakeMain()
	w.SetFocus(dialogFocusID(cfg))
}

// DialogDismiss closes the current dialog.
func (w *Window) DialogDismiss() {
	oldFocus := w.dialogCfg.oldFocusID
	w.dialogCfg = DialogCfg{}
	// Same reasoning as Dialog: without a rebuild the overlay stays on screen
	// after a programmatic dismiss, and without the wake a sleeping backend
	// never learns the flag was set.
	w.markLayoutRefresh()
	w.wakeMain()
	w.SetFocus(oldFocus)
}

// DialogIsVisible returns true if a dialog is showing — either the
// in-app modal overlay or a native (OS) modal dialog.
func (w *Window) DialogIsVisible() bool {
	return w.dialogCfg.visible || w.nativeDialogVisible
}

// retainDialogFocus keeps keyboard focus inside the modal dialog. A
// focus-claiming widget (one that re-asserts SetFocus on every view
// rebuild, e.g. a terminal that wants keystrokes without a prior click)
// can steal idFocus back from the dialog overlay. Events still route to
// the dialog layer, but with no focused element there Tab/Esc/Enter stop
// working while mouse clicks still land by coordinate. When the current
// focus is not a focusable element within the freshly generated dialog
// subtree, reassert the dialog's focus id so apps need not guard their
// own SetFocus with DialogIsVisible.
//
// dialog is the dialog's layout for this frame. Called from layoutArrange
// under w.mu.
func (w *Window) retainDialogFocus(dialog *Layout) {
	// A malformed/empty dialog layer has no focusable target; leave focus
	// untouched rather than blindly reasserting (which could steal it).
	if dialog == nil || dialog.Shape == nil {
		return
	}
	// empty focus means nothing is focused: FindLayoutByFocusID would match
	// the dialog root (it is not focusable), so treat it as escaped.
	if id := w.viewState.focusID; id != "" {
		if _, ok := findLayoutByFocusID(dialog, id); ok {
			return
		}
	}
	w.setFocusLocked(dialogFocusID(w.dialogCfg))
}
