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

	Focusable bool
	IDScroll  uint32

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
		IDScroll:            cfg.IDScroll,
		IsPassword:          cfg.IsPassword,
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
	if cfg.Mode == InputMultiline && cfg.IDScroll > 0 {
		txtSizing = FillFit
		innerSizing = FillFit
	}

	txtContent := []View{
		Text(TextCfg{
			ID:                cfg.ID,
			Focusable:         cfg.Focusable,
			FocusSkip:         true,
			Sizing:            txtSizing,
			Text:              txt,
			TextStyle:         txtStyle,
			Mode:              mode,
			IsPassword:        cfg.IsPassword,
			PlaceholderActive: placeholderActive,
		}),
	}

	a11yRole := AccessRoleTextField
	if cfg.Mode == InputMultiline {
		a11yRole = AccessRoleTextArea
	}
	a11yState := AccessStateNone
	if !cfg.Focusable {
		a11yState = AccessStateReadOnly
	}

	vAlign := VAlignMiddle
	if cfg.Mode == InputMultiline {
		vAlign = VAlignTop
	}

	idScroll := cfg.IDScroll
	innerCfg := ContainerCfg{
		Padding: NoPadding,
		Sizing:  innerSizing,
		VAlign:  vAlign,
		OnClick: inputOnClick(idScroll),
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
		Focusable:       cfg.Focusable,
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
		IDScroll:        cfg.IDScroll,
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
	IDScroll            uint32
	IsPassword          bool
	Mode                InputMode
	MaskPreset          InputMaskPreset
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

func inputOnClick(idScroll uint32) func(*Layout, *Event, *Window) {
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
		if idScroll > 0 && layout.Parent != nil {
			inputScrollCursorIntoView(
				idScroll, text, layout.Parent, w,
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
			idScroll:    idScroll,
		}
		if doubleClick {
			ds.runes = runes
		}
		if idScroll > 0 && layout.Parent != nil {
			sy := w.scrollY()
			ds.scrollY0, _ = sy.Get(idScroll) // ok ignored: zero offset is correct initial scroll
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
			if hcfg.PostCommitNormalize != nil {
				normalized := hcfg.PostCommitNormalize(
					text, CommitBlur)
				if normalized != text {
					text = normalized
					if hcfg.OnTextChanged != nil {
						hcfg.OnTextChanged(
							layout, text, w)
					}
				}
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
func inputTextChange(hcfg inputHandlerCfg, layout *Layout, text, ins string, id string, w *Window) (string, bool) {
	mask := hcfg.CompiledMask
	if mask != nil {
		is := inputStateOrDefault(id, w)
		res := InputMaskInsert(text, is.CursorPos, is.SelectBeg, is.SelectEnd, ins, mask)
		if res.Changed {
			undo := inputPushUndo(is, text)
			text = res.Text
			StateMap[string, InputState](w, nsInput, capMany).Set(id, InputState{
				CursorPos: res.CursorPos, Undo: undo,
			})
			return text, true
		}
	} else if hcfg.PreTextChange != nil {
		proposed := inputProposedText(text, ins, id, w)
		if adjusted, ok := hcfg.PreTextChange(text, proposed); ok {
			if adjusted == proposed {
				text = inputInsert(text, ins, id, w)
			} else {
				inputSetTextAndCursorAtEnd(
					text, adjusted, id, w)
				text = adjusted
			}
			return text, true
		}
	} else {
		text = inputInsert(text, ins, id, w)
		return text, true
	}
	return text, false
}

func makeInputOnChar(hcfg inputHandlerCfg) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		if hcfg.FocusID == "" || !w.IsFocus(hcfg.FocusID) {
			return
		}
		ch := e.CharCode
		id := hcfg.FocusID

		// Control characters are handled by OnKeyDown.
		if ch < CharSpace {
			e.IsHandled = true
			return
		}

		text := inputTextFromLayout(layout)
		ins := e.IMEText
		if len(ins) == 0 {
			ins = string(rune(ch))
		}
		text, changed := inputTextChange(hcfg, layout, text, ins, id, w)

		if changed {
			resetBlinkCursorVisible(w)
			if hcfg.OnTextChanged != nil {
				hcfg.OnTextChanged(layout, text, w)
			}
			inputScrollCursorIntoView(
				hcfg.IDScroll, text, layout, w,
			)
		}
		e.IsHandled = true
	}
}

func makeInputOnKeyDown(hcfg inputHandlerCfg) func(*Layout, *Event, *Window) {
	mask := hcfg.CompiledMask
	return func(layout *Layout, e *Event, w *Window) {
		if hcfg.FocusID == "" || !w.IsFocus(hcfg.FocusID) {
			return
		}
		id := hcfg.FocusID
		imap := StateMap[string, InputState](w, nsInput, capMany)
		// ok ignored: zero CursorOffset/CursorTrailing seed initial state;
		// both are immediately overwritten below.
		is, _ := imap.Get(id)
		savedOffset := is.CursorOffset
		savedTrailing := is.CursorTrailing
		is.CursorOffset = -1
		is.CursorTrailing = false
		text := inputTextFromLayout(layout)
		runeLen := utf8RuneCount(text)
		pos := is.CursorPos
		pos = min(pos, runeLen)
		isShift := e.Modifiers.Has(ModShift)
		isWordMod := e.Modifiers.HasAny(ModCtrl, ModAlt, ModSuper)
		handled := true
		textChanged := false

		// Use glyph layout for cursor navigation when available.
		gl, glOK := inputGlyphLayoutWithText(text, layout, w)

		switch e.KeyCode {
		case KeyLeft:
			inputKeyLeft(imap, id, is, text, pos,
				isShift, isWordMod, gl, glOK)
		case KeyRight:
			inputKeyRight(imap, id, is, text, pos, runeLen,
				isShift, isWordMod, gl, glOK)
		case KeyHome:
			inputKeyHome(imap, id, is, text, pos,
				isShift, savedTrailing, gl, glOK)
		case KeyEnd:
			inputKeyEnd(imap, id, is, text, pos,
				isShift, savedTrailing, gl, glOK)
		case KeyUp:
			handled = inputKeyVertical(imap, id, is, text, pos,
				isShift, savedOffset, true, hcfg.Mode, gl, glOK)
		case KeyDown:
			handled = inputKeyVertical(imap, id, is, text, pos,
				isShift, savedOffset, false, hcfg.Mode, gl, glOK)
		case KeyEnter:
			text, textChanged = inputKeyEnter(
				hcfg, layout, text, id, e, w)
		case KeyEscape:
			inputKeyEscape(imap, id, is)
			handled = false
		case KeyA:
			if e.Modifiers.HasAny(ModCtrl, ModSuper) {
				inputSelectAll(text, id, w)
			} else {
				handled = false
			}
		case KeyC:
			handled = inputKeyCopy(
				text, id, hcfg.IsPassword, e, w)
		case KeyV:
			if e.Modifiers.HasAny(ModCtrl, ModSuper) {
				text, textChanged = inputKeyPaste(
					text, w.GetClipboard(), id,
					mask, hcfg, w)
			} else {
				handled = false
			}
		case KeyX:
			text, textChanged, handled = inputKeyCut(
				text, id, hcfg.IsPassword, e, w)
		case KeyZ:
			text, textChanged, handled = inputKeyUndoRedo(
				text, id, e, w)
		case KeyBackspace:
			text, textChanged = inputKeyBackspaceOrDelete(
				text, id, false, mask, layout, w)
		case KeyDelete:
			text, textChanged = inputKeyBackspaceOrDelete(
				text, id, true, mask, layout, w)
		default:
			handled = false
		}

		if handled {
			resetBlinkCursorVisible(w)
			if textChanged && hcfg.OnTextChanged != nil {
				hcfg.OnTextChanged(layout, text, w)
			}
			inputScrollCursorIntoView(
				hcfg.IDScroll, text, layout, w,
			)
			e.IsHandled = true
		} else if hcfg.OnKeyDown != nil {
			hcfg.OnKeyDown(layout, e, w)
		}
	}
}

func makeInputOnKeyUp(hcfg inputHandlerCfg) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		if hcfg.FocusID == "" || !w.IsFocus(hcfg.FocusID) {
			return
		}
		if hcfg.OnKeyUp != nil {
			hcfg.OnKeyUp(layout, e, w)
		}
	}
}

func inputKeyEnter(
	hcfg inputHandlerCfg, layout *Layout, text string,
	id string, e *Event, w *Window,
) (string, bool) {
	if hcfg.Mode == InputMultiline {
		return inputInsert(text, "\n", id, w), true
	}
	inputCommitEnter(hcfg, layout, text, e, w)
	return text, false
}

func inputKeyEscape(
	imap *BoundedMap[string, InputState], id string, is InputState,
) {
	is.SelectBeg = 0
	is.SelectEnd = 0
	imap.Set(id, is)
}

func inputKeyCopy(
	text string, id string, isPassword bool, e *Event, w *Window,
) bool {
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) {
		return false
	}
	if copied, ok := inputCopy(text, id, isPassword, w); ok {
		w.SetClipboard(copied)
	}
	return true
}

func inputKeyCut(
	text string, id string, isPassword bool, e *Event, w *Window,
) (string, bool, bool) {
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) {
		return text, false, false
	}
	newText, copied, ok := inputCut(text, id, isPassword, w)
	if ok {
		w.SetClipboard(copied)
		return newText, true, true
	}
	return text, false, true
}

func inputKeyUndoRedo(
	text string, id string, e *Event, w *Window,
) (string, bool, bool) {
	if !e.Modifiers.HasAny(ModCtrl, ModSuper) {
		return text, false, false
	}
	if e.Modifiers.Has(ModShift) {
		if nt := inputRedo(text, id, w); nt != text {
			return nt, true, true
		}
	} else {
		if nt := inputUndo(text, id, w); nt != text {
			return nt, true, true
		}
	}
	return text, false, true
}

func inputKeyBackspaceOrDelete(
	text string, id string, forward bool,
	mask *CompiledInputMask, layout *Layout, w *Window,
) (string, bool) {
	if newText, ok := inputHandleDelete(
		text, id, forward, mask, layout, w,
	); ok {
		return newText, true
	}
	return text, false
}
