package gui

import (
	"log"
	"time"
)

const animIDDragScroll = "input-drag-scroll"

// InputCfg configures a text input field. Supports single-line,
// multiline, password, and masked input modes.
//
// # Text change pipeline
//
// On each keystroke: PreTextChange (optional pre-filter) → text
// updated → OnTextChanged → re-render. On commit (Enter, blur, or
// IME finalize): PostCommitNormalize → OnTextCommit.
type InputCfg struct {
	// Label names this field. Empty renders exactly as before: no
	// wrapper and no extra shape. Set, it stacks above the field in
	// the theme's label role, and fills A11YLabel when that is unset.
	// See gui/field_label.go for the convention and why it is one.
	Label            string
	TextStyle        TextStyle
	PlaceholderStyle TextStyle

	// OnTextChanged fires after every text change. Use for
	// live validation or state sync.
	OnTextChanged func(string, EventCtx)

	// OnTextCommit fires when the user finalizes input: pressing
	// Enter, losing focus (blur), or completing IME composition.
	// The InputCommitReason indicates the trigger.
	OnTextCommit func(string, InputCommitReason, EventCtx)

	OnEnter   func(EventCtx)
	OnKeyDown func(EventCtx)
	OnKeyUp   func(EventCtx)
	OnBlur    func(EventCtx)

	// PreTextChange is called before text changes. Return
	// (adjusted, true) to accept (adjusted may differ from
	// proposed), or ("", false) to reject. Undo/redo bypass this
	// callback by design — if security invariants (max length,
	// forbidden chars) must be enforced unconditionally, use
	// OnTextChanged instead.
	// exportaudit:keep — caller-facing config (issue #372)
	PreTextChange func(current, proposed string) (string, bool)

	// PostCommitNormalize transforms the final text before
	// OnTextCommit fires. Use for trimming whitespace,
	// normalizing case, or formatting.
	// exportaudit:keep — caller-facing config (issue #372)
	PostCommitNormalize func(text string, reason InputCommitReason) string

	ID          string `gui:"required,focus"`
	Text        string
	Placeholder string

	// Mask restricts input to a pattern (e.g. date, phone, SSN).
	// Use with MaskPreset for built-in masks or MaskTokens for
	// custom patterns. See MaskTokenDef for the mask token syntax.
	Mask string

	// Accessibility
	A11YCfg

	// MaskTokens defines custom mask token types for the Mask
	// field. See MaskTokenDef for the format.
	maskTokens []MaskTokenDef

	// Appearance
	Padding    Padding
	Radius     Opt[float32]
	SizeBorder Opt[float32]
	Width      float32
	Height     float32
	MinWidth   float32
	MaxWidth   float32
	MinHeight  float32
	MaxHeight  float32

	// FocusDisabled opts the field out of the focus system. Inputs
	// are focusable by default; set this to exclude the field from
	// Tab order and focus. Note focus also requires a non-empty ID —
	// an ID-less input renders but is inert (never a tab stop).
	FocusDisabled bool

	// ReadOnly blocks text edits while the field stays focusable and
	// selectable: navigation, selection, and copy keep working, and the
	// field is announced to assistive tech as read-only. Typing, paste,
	// cut, undo/redo, delete, and PostCommitNormalize are all skipped.
	// Mirrors HTML's readonly.
	//
	// Distinct from Disabled, which also removes the field from
	// interaction entirely and announces AccessStateDisabled.
	ReadOnly bool

	// Sound overrides the theme's click cue for an Enter commit.
	// SoundNone (the zero value) takes Theme.Sounds.Click.
	//
	// Rejection is separate and always takes Theme.Sounds.Error: a
	// keystroke a mask or a PreTextChange validator refuses, whether
	// typed, pasted, or a delete over a mask literal. Both are
	// suppressed by SoundDisabled below (issue #468).
	// exportaudit:keep — caller-facing config (issue #468)
	Sound SoundCue

	// SoundDisabled suppresses this field's sound regardless of the
	// theme and of Sound above.
	// exportaudit:keep — caller-facing config (issue #468)
	SoundDisabled bool

	// Scrollable opts a multiline input into the scroll system.
	// Scroll state is keyed by Cfg.ID - pass that same id to
	// Window.ScrollVerticalTo. Requires Mode == InputMultiline.
	Scrollable bool

	Color            Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it; the other flat
	// Color* fields win over their Colors slots the same way.
	Colors ColorSet

	Sizing Sizing

	// MaskPreset selects a built-in input mask (date, phone, etc.).
	MaskPreset InputMaskPreset

	// Mode controls input behavior: single-line, multiline, or
	// search with clear button. See InputMode constants.
	Mode inputMode

	// IsPassword masks displayed characters with dots/bullets.
	IsPassword bool

	// opticalDigitCenter opts the field's text into optical centring on
	// the face's figure band (issue #346). Unexported and opt-in because
	// it is only sound where the *caller* guarantees the alphabet is
	// digits and separators — InputDate and NumericInput, by their masks.
	// A general Input keeps metric centring: its text is arbitrary, and a
	// run with a descender is already below the middle.
	//
	// Content-free, so it cannot jitter as the user types; see
	// opticalCenterFieldText.
	opticalDigitCenter bool

	// noMinWidthFloor opts a composed inner Input out of the theme
	// field min-width floor. Set only by widgets that wrap an Input
	// as a text layer inside their own control -- NumericInput and
	// InputDate -- where the outer container already carries the
	// floor and a second one on the fill-sized inner shape would push
	// the whole control wider than the theme asked for.
	noMinWidthFloor bool

	// onMouseScroll attaches a wheel handler to the field's own shape.
	// Set only by widgets that wrap an Input and give the wheel a
	// meaning of their own -- NumericInput's wheel stepping. Private
	// because the general case is wrong: an Input that eats the wheel
	// stops the page under it from scrolling, so this is opt-in per
	// wrapping widget rather than a caller-facing field.
	onMouseScroll func(EventCtx)

	// SpellCheck enables platform spell checking. Mac only.
	SpellCheck bool

	Disabled  bool
	Invisible bool
}

// a11yReadOnlyState maps a ReadOnly flag to the announced accessibility
// state. Used by the NumericInput/InputDate wrappers.
func a11yReadOnlyState(readOnly bool) AccessState {
	if readOnly {
		return AccessStateReadOnly
	}
	return AccessStateNone
}

// Input creates a text input field view.
func Input(cfg InputCfg) View {
	applyInputDefaults(&cfg)
	requireFocusID("Input", cfg.FocusDisabled, cfg.ID)

	d := &defaultInputStyle
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	radius := cfg.Radius.Get(d.Radius)

	placeholderActive := len(cfg.Text) == 0
	txt := cfg.Text
	if placeholderActive {
		txt = cfg.Placeholder
	}
	txtStyle := cfg.TextStyle
	if placeholderActive {
		txtStyle = cfg.PlaceholderStyle
	}
	mode := TextModeSingleLine
	if cfg.Mode == InputMultiline {
		mode = TextModeWrapKeepSpaces
	}

	colorBorderFocus := cfg.ColorBorderFocus
	colorHover := cfg.ColorHover
	focusID := cfg.ID
	spellChk := cfg.SpellCheck && !cfg.IsPassword
	onBlur := cfg.OnBlur

	hcfg := inputHandlerCfg{
		FocusID:             cfg.ID,
		scrollID:            inputScrollIDFor(&cfg),
		IsPassword:          cfg.IsPassword,
		ReadOnly:            cfg.ReadOnly,
		Mode:                cfg.Mode,
		Mask:                cfg.Mask,
		MaskPreset:          cfg.MaskPreset,
		maskTokens:          cfg.maskTokens,
		OnTextChanged:       cfg.OnTextChanged,
		OnTextCommit:        cfg.OnTextCommit,
		OnEnter:             cfg.OnEnter,
		OnKeyDown:           cfg.OnKeyDown,
		OnKeyUp:             cfg.OnKeyUp,
		preTextChange:       cfg.PreTextChange,
		postCommitNormalize: cfg.PostCommitNormalize,
		cues: resolveSoundCues(
			guiTheme.Sounds.Click, cfg.Sound, cfg.SoundDisabled),
	}
	hcfg.CompiledMask = hcfg.compiledMask()

	txtSizing := Sizing(FillFill)
	innerSizing := Sizing(FillFill)
	if cfg.Mode == InputMultiline && cfg.Scrollable {
		txtSizing = FillFit
		innerSizing = FillFit
	} else if cfg.Mode != InputMultiline {
		// Single-line: the text shape must hug its font height so the
		// inner row's VAlignMiddle can center it. Text renders from
		// the top of its shape, so a Fill-height text pins the glyphs
		// to the top of any input taller than the text (visible in
		// data grid cells, which strip padding and fill the row).
		txtSizing = FillFit
	}

	txtContent := []View{
		Text(TextCfg{
			// No ID: the outer container below is the sole claimant of
			// cfg.ID and the sole focus target. This shape only needs
			// to read that widget's focus and spell-check state, which
			// is what focusOwner expresses — claiming the ID too would
			// put the same identity on two shapes.
			focusOwner:        cfg.ID,
			Sizing:            txtSizing,
			Text:              txt,
			TextStyle:         txtStyle,
			Mode:              mode,
			IsPassword:        cfg.IsPassword,
			placeholderActive: placeholderActive,
			readOnly:          cfg.ReadOnly,
		}),
	}

	a11yRole := AccessRoleTextField
	if cfg.Mode == InputMultiline {
		a11yRole = AccessRoleTextArea
	}
	// ReadOnly is the only signal for the read-only announcement.
	// Inputs are focusable by default, so non-focusable is now an
	// explicit FocusDisabled opt-out — no longer a proxy for
	// read-only.
	a11yState := AccessStateNone
	if cfg.ReadOnly {
		a11yState = AccessStateReadOnly
	}

	vAlign := VAlignMiddle
	if cfg.Mode == InputMultiline {
		vAlign = VAlignTop
	}

	scrollID := inputScrollIDFor(&cfg)
	innerCfg := ContainerCfg{
		Padding: NoPadding,
		// An unset SizeBorder falls back to the theme's container
		// border (1.5), and a container reserves space for its border
		// whether or not the border is painted. This row's is
		// transparent, so it was silently adding 3px to every Input's
		// height -- which is what still made an Input taller than the
		// Select beside it after both took the shared field inset
		// (issue #335, audit section 7).
		SizeBorder: NoBorder,
		Sizing:     innerSizing,
		VAlign:     vAlign,
		OnClick:    inputOnClick(cfg.ID, scrollID, !cfg.FocusDisabled),
		Content:    txtContent,
	}
	// Multiline is excluded on top of the caller's opt-in: it aligns to
	// the top, so there is no centring to correct, and its lines after
	// the first are positioned by the text layout rather than by this
	// row.
	if cfg.opticalDigitCenter && cfg.Mode != InputMultiline {
		innerCfg.AmendLayout = opticalCenterFieldText
	}
	var inner View
	if cfg.Mode == InputMultiline {
		inner = Column(innerCfg)
	} else {
		inner = Row(innerCfg)
	}

	cfg.A11YLabel = a11yLabel(cfg.A11YLabel, cfg.Label)
	field := Column(ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  a11yRole,
		A11YState: a11yState,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Placeholder),
			A11YDescription: cfg.A11YDescription,
		},
		Width:       cfg.Width,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Disabled:    cfg.Disabled,
		Clip:        true,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Invisible:   cfg.Invisible,
		Padding:     cfg.Padding,
		Radius:      Some(radius),
		Sizing:      cfg.Sizing,
		Scrollable:  cfg.Scrollable,
		Spacing:     SomeF(0),
		OnChar:      makeInputOnChar(hcfg),
		OnKeyDown:   makeInputOnKeyDown(hcfg),
		OnKeyUp:     makeInputOnKeyUp(hcfg),
		OnHover: func(ctx EventCtx) {
			ctx.Window.setMouseCursor(CursorIBeam)
			// This handler sits on the container that claims cfg.ID, so
			// its resolved identity is the focus key.
			if !ctx.Window.IsFocus(ctx.Layout.Shape.idKey()) {
				ctx.Layout.Shape.Color = colorHover
			}
		},
		AmendLayout: inputAmendLayout(hcfg, focusID,
			colorBorderFocus, spellChk, onBlur, cfg.onMouseScroll),
		Content: []View{inner},
	})
	return labelledField(cfg.Label, cfg.TextStyle, HAlignLeft, cfg.Sizing, field)
}

func applyInputDefaults(cfg *InputCfg) {
	d := &defaultInputStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	cfg.Colors.applyTo(&cfg.Color, &cfg.ColorHover, nil, nil,
		&cfg.ColorBorder, &cfg.ColorBorderFocus)
	if !cfg.Padding.IsSet() {
		// Was a hardcoded inset, which made InputStyle.Padding dead:
		// a theme author editing it saw Container and ListBox move
		// while Input stayed put (issue #335, audit section 7).
		cfg.Padding = d.Padding
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = defaultInputStyle.PlaceholderStyle
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(d.Radius)
	}
	if !cfg.noMinWidthFloor {
		cfg.MinWidth = fieldMinWidth(cfg.MinWidth, cfg.Width)
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = Some(d.SizeBorder)
	}
}

// inputHandlerCfg captures the fields shared by OnChar and
// OnKeyDown handler factories.
type inputHandlerCfg struct {
	CompiledMask        *CompiledInputMask
	OnTextChanged       func(string, EventCtx)
	OnTextCommit        func(string, InputCommitReason, EventCtx)
	OnEnter             func(EventCtx)
	OnKeyDown           func(EventCtx)
	OnKeyUp             func(EventCtx)
	preTextChange       func(current, proposed string) (string, bool)
	postCommitNormalize func(text string, reason InputCommitReason) string
	// cues are resolved at generation time and travel by value:
	// act sounds an Enter commit, reject sounds a refused keystroke.
	// Neither path reaches event dispatch, so neither can read a cue
	// off the shape (issue #468).
	cues       soundCues
	Mask       string
	maskTokens []MaskTokenDef
	FocusID    string
	scrollID   string
	IsPassword bool
	ReadOnly   bool
	Mode       inputMode
	MaskPreset InputMaskPreset
}

// fireTextChanged notifies the caller that the text changed. Every
// mutation path in this file ends here, which makes it the one place
// ReadOnly has to be enforced: a read-only field by definition never
// changes text, so the notification is dropped rather than relying on
// each entry point to remember to check. Gating only the entry points
// missed the commit paths, where PostCommitNormalize reaches this
// without going through any text mutator.
func (h *inputHandlerCfg) fireTextChanged(
	layout *Layout, text string, w *Window,
) {
	// Echo the mutation into the layout tree before notifying the
	// app. Events are drained in batches against the same tree, so
	// a later event in the batch must see this text, not the text
	// the last frame rendered (see inputSetTextInLayout).
	inputSetTextInLayout(layout, text)
	if h.ReadOnly || h.OnTextChanged == nil {
		return
	}
	h.OnTextChanged(text, EventCtx{layout, nil, w})
}

// fireTextChangedDeferred is fireTextChanged for the one caller that
// runs under the frame lock: the blur block in inputAmendLayout.
//
// The layout echo happens now — it must, the arrange pass is still
// reading this tree — but the app callback is deferred, because
// OnTextChanged is app code and calling SetFocus from it under w.mu
// hangs the main thread (issue #394). The deferred ctx carries no
// *Layout: this tree is rebuilt from pooled arenas before the callback
// runs, so the pointer would dangle.
func (h *inputHandlerCfg) fireTextChangedDeferred(
	layout *Layout, text string, w *Window,
) {
	inputSetTextInLayout(layout, text)
	if h.ReadOnly || h.OnTextChanged == nil {
		return
	}
	changed := h.OnTextChanged
	w.deferCallback(func(w *Window) {
		changed(text, EventCtx{nil, nil, w})
	})
}

// normalizeOnCommit applies PostCommitNormalize, which transforms the
// text and is therefore an edit: read-only fields skip it and commit
// the text unchanged.
func (h *inputHandlerCfg) normalizeOnCommit(
	text string, reason InputCommitReason,
) string {
	if h.ReadOnly || h.postCommitNormalize == nil {
		return text
	}
	return h.postCommitNormalize(text, reason)
}

// compiledMask returns a non-nil *CompiledInputMask if the
// handler config specifies a mask pattern.
func (h *inputHandlerCfg) compiledMask() *CompiledInputMask {
	pattern := h.Mask
	if pattern == "" && h.MaskPreset != maskNone {
		pattern = inputMaskFromPreset(h.MaskPreset)
	}
	if pattern == "" {
		return nil
	}
	c, err := compileInputMask(pattern, h.maskTokens)
	if err != nil {
		log.Printf("input: mask compile failed: %v", err)
		return nil
	}
	return &c
}

// inputScrollIDFor returns the scroll key for a multiline input, or
// "" when the input does not opt into scrolling. Multiline-only:
// single-line inputs never scroll vertically.
func inputScrollIDFor(cfg *InputCfg) string {
	if cfg.Mode == InputMultiline && cfg.Scrollable {
		return cfg.ID
	}
	return ""
}

// inputOnClick handles a click on the inner row that wraps the text
// shape. focusID is the owning container's ID — the key for focus and
// for the nsInput state — and canFocus is false for a FocusDisabled
// input, which still tracks a cursor but never takes focus. Both are
// passed in rather than read off the text shape, which carries no ID
// of its own (see Shape.focusOwner).
func inputOnClick(leafID, leafScrollID string, canFocus bool) func(EventCtx) {
	return func(ctx EventCtx) {
		if len(ctx.Layout.Children) < 1 {
			// No inner text shape: not ours
			return
		}
		// Input builds its tree eagerly, with no Window in hand, so the
		// captured IDs are leaves. Resolve them here: the owning
		// container is this shape's parent, and by dispatch time every
		// shape carries its effective identity.
		focusID := ctx.EffID(leafID)
		scrollID := ctx.EffID(leafScrollID)
		ly := ctx.Layout.Children[0]
		if canFocus && focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		if ly.Shape.TC == nil {
			// No text config: not ours
			return
		}
		if ly.Shape.TC.textIsPlaceholder {
			imap := StateMap[string, inputState](
				ctx.Window, nsInput, capMany,
			)
			// Default InputState{}: zero value seeds initial state.
			is := imap.GetOr(focusID, inputState{})
			is.CursorPos = 0
			is.selectBeg = 0
			is.selectEnd = 0
			is.cursorOffset = -1
			imap.Set(focusID, is)
			resetBlinkCursorVisible(ctx.Window)
			ctx.Consume()
			return
		}
		text := ly.Shape.TC.Text
		style := textStyleOrDefault(ly.Shape)
		gl, ok := inputGlyphLayout(
			text, ly.Shape, style, ctx.Window,
		)
		if !ok {
			// No glyph layout: cannot place a cursor
			return
		}
		relX := ctx.Event.MouseX - (ly.Shape.X - ctx.Layout.Shape.X)
		relY := ctx.Event.MouseY - (ly.Shape.Y - ctx.Layout.Shape.Y)
		byteIdx := gl.GetClosestOffset(relX, relY)
		displayText := text
		if ly.Shape.TC.textIsPassword {
			displayText = passwordMask(text)
		}
		runePos := byteToRuneIndex(displayText, byteIdx)
		imap := StateMap[string, inputState](
			ctx.Window, nsInput, capMany,
		)
		// Default InputState{}: zero LastClickTime safely gates
		// double-click (the > 0 check below prevents false match).
		is := imap.GetOr(focusID, inputState{})

		// Double-click selects word.
		now := time.Now().UnixMilli()
		doubleClick := is.LastClickTime > 0 &&
			now-is.LastClickTime <= 400
		is.LastClickTime = now

		if doubleClick {
			beg, end := wordBoundsAt(displayText, runePos)
			is.CursorPos = end
			is.selectBeg = uint32(beg)
			is.selectEnd = uint32(end)
		} else {
			is.CursorPos = runePos
			is.selectBeg = uint32(runePos)
			is.selectEnd = uint32(runePos)
		}
		is.cursorOffset = -1
		imap.Set(focusID, is)
		resetBlinkCursorVisible(ctx.Window)
		if scrollID != "" && ctx.Layout.Parent != nil {
			inputScrollCursorIntoView(
				scrollID, text, ctx.Layout.Parent, ctx.Window,
			)
		}
		ctx.Consume()

		// Drag-to-select via MouseLock. The lock delivers mouse moves in
		// window-absolute coordinates (event_handlers.go), so the drag
		// offset must be the text shape's absolute position — subtracting
		// the box-relative offset (txt.X - box.X) would leave the drag
		// shifted by the input's window X. The click path derives the
		// same text-relative frame from the box-relative event.
		ds := &inputDragState{
			anchorPos:   is.selectBeg,
			anchorEnd:   is.selectEnd,
			gl:          gl,
			displayText: displayText,
			wordSelect:  doubleClick,
			txtOffX:     ly.Shape.X,
			txtOffY:     ly.Shape.Y,
			focusID:     focusID,
			scrollID:    scrollID,
		}
		if scrollID != "" && ctx.Layout.Parent != nil {
			sy := ctx.Window.scrollY()
			// Default 0: unscrolled input before first scroll event.
			ds.scrollY0 = sy.GetOr(scrollID, 0)
			p := ctx.Layout.Parent.Shape
			ds.viewTop = p.Y + p.Padding.Top
			viewH := p.Height - p.paddingHeight()
			ds.viewBot = ds.viewTop + viewH
			ds.maxScrollNeg = f32Min(0,
				viewH-ctx.Layout.Shape.Height)
		}
		startInputDrag(ds, ctx.Window)
	}
}

func inputAmendLayout(
	hcfg inputHandlerCfg, focusID string,
	colorBorderFocus Color, spellChk bool,
	onBlur func(EventCtx), onMouseScroll func(EventCtx),
) func(EventCtx) {
	// Captured at generation; see focusRingAmend.
	ring := guiTheme.focusRing
	return func(ctx EventCtx) {
		// Attached ahead of the focus gate below: the wheel does not
		// need focus, and a FocusDisabled numeric field should still
		// step under the pointer if its wrapper asked for it.
		if onMouseScroll != nil {
			if ctx.Layout.Shape.events == nil {
				ctx.Layout.Shape.events = &eventHandlers{}
			}
			ctx.Layout.Shape.events.OnMouseScroll = onMouseScroll
		}
		if !ctx.Layout.Shape.Focusable || ctx.Layout.Shape.ID == "" {
			return
		}
		// This callback runs on the container that claims cfg.ID, so its
		// own resolved identity is the key for focus, blur tracking,
		// selection and spell-check state.
		key := ctx.Layout.Shape.idKey()
		focused := !ctx.Layout.Shape.Disabled &&
			ctx.Window.IsFocus(key)
		if focused {
			ctx.Layout.Shape.ColorBorder = colorBorderFocus
			applyFocusRingShadow(ctx.Layout.Shape, ctx.Window, ring)
		}

		// Blur detection: fire commit on focus loss.
		focusMap := StateMap[string, bool](
			ctx.Window, nsInputFocus, capMany)
		// Default false: absent entry means not previously focused.
		wasFocused := focusMap.GetOr(key, false)
		focusMap.Set(key, focused)
		if wasFocused && !focused {
			text := inputTextFromLayout(ctx.Layout)
			if normalized := hcfg.normalizeOnCommit(
				text, InputCommitBlur,
			); normalized != text {
				text = normalized
				hcfg.fireTextChangedDeferred(ctx.Layout, text, ctx.Window)
			}
			// This hook runs from layoutArrange, which holds w.mu, so
			// app callbacks are raised rather than called: OnTextCommit
			// on a blur reasonably calls SetFocus, and doing that under
			// the frame lock froze the app (issue #394). gui-internal
			// work below stays inline — it touches only window state.
			// The deferred ctx has no *Layout; see deferCallback.
			commit, blur := hcfg.OnTextCommit, onBlur
			if commit != nil || blur != nil {
				committed := text
				ctx.Window.deferCallback(func(w *Window) {
					c := EventCtx{nil, nil, w}
					if commit != nil {
						commit(committed, InputCommitBlur, c)
					}
					if blur != nil {
						blur(c)
					}
				})
			}
			if spellChk {
				spellCheckClear(key, ctx.Window)
			}
		}

		// Propagate selection to inner text shape.
		if len(ctx.Layout.Children) > 0 {
			inner := &ctx.Layout.Children[0]
			if len(inner.Children) > 0 {
				txt := &inner.Children[0]
				if txt.Shape.TC != nil {
					is := StateReadOr(ctx.Window, nsInput,
						key, inputState{})
					txt.Shape.TC.textSelBeg = is.selectBeg
					txt.Shape.TC.textSelEnd = is.selectEnd
				}
			}
		}

		// Spell check: trigger when enabled, clear when
		// disabled.
		if spellChk && focused {
			text := inputTextFromLayout(ctx.Layout)
			spellCheckTrigger(key, text, ctx.Window)
		} else if !spellChk {
			spellCheckClear(key, ctx.Window)
		}
	}
}

// inputTextChange handles text modification logic for input widgets
