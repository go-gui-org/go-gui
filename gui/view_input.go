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
	TextStyle        TextStyle
	PlaceholderStyle TextStyle

	// OnTextChanged fires after every text change. Use for
	// live validation or state sync.
	OnTextChanged func(*Layout, string, *Window)

	// OnTextCommit fires when the user finalizes input: pressing
	// Enter, losing focus (blur), or completing IME composition.
	// The InputCommitReason indicates the trigger.
	OnTextCommit func(*Layout, string, InputCommitReason, *Window)

	OnEnter   func(*Layout, *Event, *Window)
	OnKeyDown func(*Layout, *Event, *Window)
	OnKeyUp   func(*Layout, *Event, *Window)
	OnBlur    func(*Layout, *Window)

	// PreTextChange is called before text changes. Return
	// (adjusted, true) to accept (adjusted may differ from
	// proposed), or ("", false) to reject. Undo/redo bypass this
	// callback by design — if security invariants (max length,
	// forbidden chars) must be enforced unconditionally, use
	// OnTextChanged instead.
	PreTextChange func(current, proposed string) (string, bool)

	// PostCommitNormalize transforms the final text before
	// OnTextCommit fires. Use for trimming whitespace,
	// normalizing case, or formatting.
	PostCommitNormalize func(text string, reason InputCommitReason) string

	ID          string
	Text        string
	Placeholder string

	// Mask restricts input to a pattern (e.g. date, phone, SSN).
	// Use with MaskPreset for built-in masks or MaskTokens for
	// custom patterns. See MaskTokenDef for the mask token syntax.
	Mask string

	// Accessibility
	A11YLabel       string
	A11YDescription string

	// MaskTokens defines custom mask token types for the Mask
	// field. See MaskTokenDef for the format.
	MaskTokens []MaskTokenDef

	// Appearance
	Padding    Opt[Padding]
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

	// Scrollable opts a multiline input into the scroll system.
	// Scroll state is keyed by Cfg.ID - pass that same id to
	// Window.ScrollVerticalTo. Requires Mode == InputMultiline.
	Scrollable bool

	Color            Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color

	Sizing Sizing

	// MaskPreset selects a built-in input mask (date, phone, etc.).
	MaskPreset InputMaskPreset

	// Mode controls input behavior: single-line, multiline, or
	// search with clear button. See InputMode constants.
	Mode InputMode

	// IsPassword masks displayed characters with dots/bullets.
	IsPassword bool

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

	d := &DefaultInputStyle
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
		ScrollID:            inputScrollIDFor(&cfg),
		IsPassword:          cfg.IsPassword,
		ReadOnly:            cfg.ReadOnly,
		Mode:                cfg.Mode,
		Mask:                cfg.Mask,
		MaskPreset:          cfg.MaskPreset,
		MaskTokens:          cfg.MaskTokens,
		OnTextChanged:       cfg.OnTextChanged,
		OnTextCommit:        cfg.OnTextCommit,
		OnEnter:             cfg.OnEnter,
		OnKeyDown:           cfg.OnKeyDown,
		OnKeyUp:             cfg.OnKeyUp,
		PreTextChange:       cfg.PreTextChange,
		PostCommitNormalize: cfg.PostCommitNormalize,
	}
	hcfg.CompiledMask = hcfg.compiledMask()

	txtSizing := Sizing(FillFill)
	innerSizing := Sizing(FillFill)
	if cfg.Mode == InputMultiline && cfg.Scrollable {
		txtSizing = FillFit
		innerSizing = FillFit
	}

	txtContent := []View{
		Text(TextCfg{
			ID:                cfg.ID,
			Focusable:         !cfg.FocusDisabled,
			FocusSkip:         true,
			Sizing:            txtSizing,
			Text:              txt,
			TextStyle:         txtStyle,
			Mode:              mode,
			IsPassword:        cfg.IsPassword,
			PlaceholderActive: placeholderActive,
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
		Sizing:  innerSizing,
		VAlign:  vAlign,
		OnClick: inputOnClick(scrollID),
		Content: txtContent,
	}
	var inner View
	if cfg.Mode == InputMultiline {
		inner = Column(innerCfg)
	} else {
		inner = Row(innerCfg)
	}

	return Column(ContainerCfg{
		ID:              cfg.ID,
		Focusable:       !cfg.FocusDisabled,
		A11YRole:        a11yRole,
		A11YState:       a11yState,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Placeholder),
		A11YDescription: cfg.A11YDescription,
		Width:           cfg.Width,
		Height:          cfg.Height,
		MinWidth:        cfg.MinWidth,
		MaxWidth:        cfg.MaxWidth,
		MinHeight:       cfg.MinHeight,
		MaxHeight:       cfg.MaxHeight,
		Disabled:        cfg.Disabled,
		Clip:            true,
		Color:           cfg.Color,
		ColorBorder:     cfg.ColorBorder,
		SizeBorder:      Some(sizeBorder),
		Invisible:       cfg.Invisible,
		Padding:         cfg.Padding,
		Radius:          Some(radius),
		Sizing:          cfg.Sizing,
		Scrollable:      cfg.Scrollable,
		Spacing:         SomeF(0),
		OnChar:          makeInputOnChar(hcfg),
		OnKeyDown:       makeInputOnKeyDown(hcfg),
		OnKeyUp:         makeInputOnKeyUp(hcfg),
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			w.setMouseCursor(CursorIBeam)
			if !w.IsFocus(focusID) {
				layout.Shape.Color = colorHover
			}
		},
		AmendLayout: inputAmendLayout(hcfg, focusID,
			colorBorderFocus, spellChk, onBlur),
		Content: []View{inner},
	})
}

func applyInputDefaults(cfg *InputCfg) {
	d := &DefaultInputStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(paddingTwoFour)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = DefaultInputStyle.PlaceholderStyle
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(d.Radius)
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = Some(d.SizeBorder)
	}
}

// inputHandlerCfg captures the fields shared by OnChar and
// OnKeyDown handler factories.
type inputHandlerCfg struct {
	CompiledMask        *CompiledInputMask
	OnTextChanged       func(*Layout, string, *Window)
	OnTextCommit        func(*Layout, string, InputCommitReason, *Window)
	OnEnter             func(*Layout, *Event, *Window)
	OnKeyDown           func(*Layout, *Event, *Window)
	OnKeyUp             func(*Layout, *Event, *Window)
	PreTextChange       func(current, proposed string) (string, bool)
	PostCommitNormalize func(text string, reason InputCommitReason) string
	Mask                string
	MaskTokens          []MaskTokenDef
	FocusID             string
	ScrollID            string
	IsPassword          bool
	ReadOnly            bool
	Mode                InputMode
	MaskPreset          InputMaskPreset
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
	if h.ReadOnly || h.OnTextChanged == nil {
		return
	}
	h.OnTextChanged(layout, text, w)
}

// normalizeOnCommit applies PostCommitNormalize, which transforms the
// text and is therefore an edit: read-only fields skip it and commit
// the text unchanged.
func (h *inputHandlerCfg) normalizeOnCommit(
	text string, reason InputCommitReason,
) string {
	if h.ReadOnly || h.PostCommitNormalize == nil {
		return text
	}
	return h.PostCommitNormalize(text, reason)
}

// compiledMask returns a non-nil *CompiledInputMask if the
// handler config specifies a mask pattern.
func (h *inputHandlerCfg) compiledMask() *CompiledInputMask {
	pattern := h.Mask
	if pattern == "" && h.MaskPreset != MaskNone {
		pattern = InputMaskFromPreset(h.MaskPreset)
	}
	if pattern == "" {
		return nil
	}
	c, err := CompileInputMask(pattern, h.MaskTokens)
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

func inputOnClick(scrollID string) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		if len(layout.Children) < 1 {
			return
		}
		ly := layout.Children[0]
		if ly.Shape.Focusable && ly.Shape.ID != "" {
			w.SetFocus(ly.Shape.ID)
		}
		if ly.Shape.TC == nil {
			return
		}
		if ly.Shape.TC.TextIsPlaceholder {
			imap := StateMap[string, InputState](
				w, nsInput, capMany,
			)
			is, _ := imap.Get(ly.Shape.ID) // ok ignored: zero value seeds initial state
			is.CursorPos = 0
			is.SelectBeg = 0
			is.SelectEnd = 0
			is.CursorOffset = -1
			imap.Set(ly.Shape.ID, is)
			resetBlinkCursorVisible(w)
			e.IsHandled = true
			return
		}
		text := ly.Shape.TC.Text
		style := textStyleOrDefault(ly.Shape)
		gl, ok := inputGlyphLayout(
			text, ly.Shape, style, w,
		)
		if !ok {
			return
		}
		relX := e.MouseX - (ly.Shape.X - layout.Shape.X)
		relY := e.MouseY - (ly.Shape.Y - layout.Shape.Y)
		byteIdx := gl.GetClosestOffset(relX, relY)
		displayText := text
		if ly.Shape.TC.TextIsPassword {
			displayText = passwordMask(text)
		}
		runePos := byteToRuneIndex(displayText, byteIdx)
		imap := StateMap[string, InputState](
			w, nsInput, capMany,
		)
		// ok ignored: zero LastClickTime safely gates double-click
		// (the > 0 check on line below prevents false match).
		is, _ := imap.Get(ly.Shape.ID)

		// Double-click selects word.
		now := time.Now().UnixMilli()
		doubleClick := is.LastClickTime > 0 &&
			now-is.LastClickTime <= 400
		is.LastClickTime = now

		var runes []rune
		if doubleClick {
			runes = []rune(displayText)
			beg, end := wordBoundsAt(runes, runePos)
			is.CursorPos = end
			is.SelectBeg = uint32(beg)
			is.SelectEnd = uint32(end)
		} else {
			is.CursorPos = runePos
			is.SelectBeg = uint32(runePos)
			is.SelectEnd = uint32(runePos)
		}
		is.CursorOffset = -1
		imap.Set(ly.Shape.ID, is)
		resetBlinkCursorVisible(w)
		if scrollID != "" && layout.Parent != nil {
			inputScrollCursorIntoView(
				scrollID, text, layout.Parent, w,
			)
		}
		e.IsHandled = true

		// Drag-to-select via MouseLock.
		ds := &inputDragState{
			anchorPos:   is.SelectBeg,
			anchorEnd:   is.SelectEnd,
			gl:          gl,
			displayText: displayText,
			txtOffX:     ly.Shape.X - layout.Shape.X,
			txtOffY:     ly.Shape.Y - layout.Shape.Y,
			focusID:     ly.Shape.ID,
			scrollID:    scrollID,
		}
		if doubleClick {
			ds.runes = runes
		}
		if scrollID != "" && layout.Parent != nil {
			sy := w.scrollY()
			ds.scrollY0, _ = sy.Get(scrollID) // ok ignored: zero offset is correct initial scroll
			p := layout.Parent.Shape
			ds.viewTop = p.Y + p.Padding.Top
			viewH := p.Height - p.paddingHeight()
			ds.viewBot = ds.viewTop + viewH
			ds.maxScrollNeg = f32Min(0,
				viewH-layout.Shape.Height)
		}
		startInputDrag(ds, w)
	}
}

func inputAmendLayout(
	hcfg inputHandlerCfg, focusID string,
	colorBorderFocus Color, spellChk bool,
	onBlur func(*Layout, *Window),
) func(*Layout, *Window) {
	return func(layout *Layout, w *Window) {
		if !layout.Shape.Focusable || layout.Shape.ID == "" {
			return
		}
		focused := !layout.Shape.Disabled &&
			w.IsFocus(layout.Shape.ID)
		if focused {
			layout.Shape.ColorBorder = colorBorderFocus
		}

		// Blur detection: fire commit on focus loss.
		focusMap := StateMap[string, bool](
			w, nsInputFocus, capMany)
		wasFocused, _ := focusMap.Get(layout.Shape.ID) // ok ignored: false means "wasn't focused"
		focusMap.Set(layout.Shape.ID, focused)
		if wasFocused && !focused {
			text := inputTextFromLayout(layout)
			if normalized := hcfg.normalizeOnCommit(
				text, CommitBlur,
			); normalized != text {
				text = normalized
				hcfg.fireTextChanged(layout, text, w)
			}
			if hcfg.OnTextCommit != nil {
				hcfg.OnTextCommit(
					layout, text, CommitBlur, w)
			}
			if spellChk {
				spellCheckClear(
					layout.Shape.ID, w)
			}
			if onBlur != nil {
				onBlur(layout, w)
			}
		}

		// Propagate selection to inner text shape.
		if len(layout.Children) > 0 {
			inner := &layout.Children[0]
			if len(inner.Children) > 0 {
				txt := &inner.Children[0]
				if txt.Shape.TC != nil {
					is := StateReadOr(w, nsInput,
						layout.Shape.ID,
						InputState{})
					txt.Shape.TC.TextSelBeg = is.SelectBeg
					txt.Shape.TC.TextSelEnd = is.SelectEnd
				}
			}
		}

		// Spell check: trigger when enabled, clear when
		// disabled.
		if spellChk && focused {
			text := inputTextFromLayout(layout)
			spellCheckTrigger(
				layout.Shape.ID, text, w)
		} else if !spellChk {
			spellCheckClear(layout.Shape.ID, w)
		}
	}
}

// inputTextChange handles text modification logic for input widgets
