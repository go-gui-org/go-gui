package gui

import (
	"testing"
	"unicode/utf8"

	"github.com/go-gui-org/go-glyph"
)

func TestInputGeneratesLayout(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{
		ID:          "email",
		Text:        "hello",
		Placeholder: "Enter email",
		Focusable:   true,
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "email" {
		t.Fatalf("got ID %q, want email", layout.Shape.ID)
	}
	if layout.Shape.A11YRole != AccessRoleTextField {
		t.Fatalf("got role %d, want TextField", layout.Shape.A11YRole)
	}
}

func TestInputMultilineRole(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{
		Mode:      InputMultiline,
		Focusable: true, ID: "f11",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleTextArea {
		t.Fatalf("got role %d, want TextArea", layout.Shape.A11YRole)
	}
}

func TestInputReadOnlyWithoutFocus(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{Text: "readonly"})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateReadOnly {
		t.Fatalf("got state %d, want ReadOnly", layout.Shape.A11YState)
	}
}

func TestInputPlaceholderWhenEmpty(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{
		Placeholder: "Type here",
		Focusable:   true, ID: "f12",
	})
	layout := generateViewLayout(v, w)
	// The inner Row → Text child should use placeholder text.
	if len(layout.Children) == 0 {
		t.Fatal("no children")
	}
	row := layout.Children[0]
	if len(row.Children) == 0 {
		t.Fatal("no row children")
	}
	txt := row.Children[0]
	if txt.Shape.TC == nil {
		t.Fatal("text config is nil")
	}
	if txt.Shape.TC.Text != "Type here" {
		t.Fatalf("got %q, want placeholder", txt.Shape.TC.Text)
	}
}

func TestInputClickPlaceholderResetsCursorToStart(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("f14")
	setInputState(w, "f14", InputState{
		CursorPos: 7,
		SelectBeg: 2,
		SelectEnd: 5,
	})
	layout := generateViewLayout(Input(InputCfg{
		Placeholder: "Type here",
		Focusable:   true, ID: "f14",
	}), w)
	if len(layout.Children) == 0 {
		t.Fatal("no children")
	}
	inner := &layout.Children[0]
	if inner.Shape.events == nil || inner.Shape.events.OnClick == nil {
		t.Fatal("missing click handler")
	}
	e := &Event{MouseX: inner.Shape.X, MouseY: inner.Shape.Y}
	inner.Shape.events.OnClick(inner, e, w)
	is := getInputState(w, "f14")
	if is.CursorPos != 0 {
		t.Fatalf("cursor=%d, want 0", is.CursorPos)
	}
	if is.SelectBeg != 0 || is.SelectEnd != 0 {
		t.Fatalf("selection=%d-%d, want 0-0", is.SelectBeg, is.SelectEnd)
	}
	if !e.IsHandled {
		t.Fatal("click should be handled")
	}
}

func TestInputPasswordMask(t *testing.T) {
	got := passwordMask("abc")
	if got != "•••" {
		t.Fatalf("got %q, want •••", got)
	}
}

func TestInputPasswordMaskEmoji(t *testing.T) {
	got := passwordMask("🔑key")
	want := "••••"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestInputDefaults(t *testing.T) {
	cfg := InputCfg{}
	applyInputDefaults(&cfg)
	if !cfg.Color.IsSet() {
		t.Fatal("Color not defaulted")
	}
	if !cfg.Radius.IsSet() {
		t.Fatal("Radius not defaulted")
	}
	if !cfg.SizeBorder.IsSet() {
		t.Fatal("SizeBorder not defaulted")
	}
	if cfg.TextStyle == (TextStyle{}) {
		t.Fatal("TextStyle not defaulted")
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		t.Fatal("PlaceholderStyle not defaulted")
	}
}

func TestInputA11YLabelFallback(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{
		Placeholder: "Search...",
		Focusable:   true, ID: "f13",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11Y == nil {
		t.Fatal("A11Y nil")
	}
	if layout.Shape.A11Y.Label != "Search..." {
		t.Fatalf("got %q, want Search...", layout.Shape.A11Y.Label)
	}
}

// --- Test helpers for OnChar/OnKeyDown ---

// inputTestSetup creates a window and input layout, sets focus and
// cursor, and returns everything needed to simulate events.
type inputTestCtx struct {
	w        *Window
	layout   Layout
	lastText string
}

func newInputTest(text string, focusID string, cursorPos int) *inputTestCtx {
	ctx := &inputTestCtx{}
	ctx.w = newTestWindow()
	ctx.lastText = text
	ctx.w.SetFocus(focusID)
	setInputState(ctx.w, focusID, InputState{CursorPos: cursorPos})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:      text,
		ID:        focusID,
		Focusable: true,
		OnTextChanged: func(_ *Layout, newText string, _ *Window) {
			ctx.lastText = newText
		},
	}), ctx.w)
	return ctx
}

func newInputTestMultiline(text string, focusID string, cursorPos int) *inputTestCtx {
	ctx := &inputTestCtx{}
	ctx.w = newTestWindow()
	ctx.lastText = text
	ctx.w.SetFocus(focusID)
	setInputState(ctx.w, focusID, InputState{CursorPos: cursorPos})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:      text,
		ID:        focusID,
		Focusable: true,
		Mode:      InputMultiline,
		OnTextChanged: func(_ *Layout, newText string, _ *Window) {
			ctx.lastText = newText
		},
	}), ctx.w)
	return ctx
}

func (c *inputTestCtx) fireChar(charCode uint32) {
	e := &Event{Type: EventChar, CharCode: charCode}
	if c.layout.Shape.events != nil && c.layout.Shape.events.OnChar != nil {
		c.layout.Shape.events.OnChar(&c.layout, e, c.w)
	}
}

func (c *inputTestCtx) fireKeyDown(key KeyCode, mod Modifier) {
	e := &Event{Type: EventKeyDown, KeyCode: key, Modifiers: mod}
	if c.layout.Shape.events != nil && c.layout.Shape.events.OnKeyDown != nil {
		c.layout.Shape.events.OnKeyDown(&c.layout, e, c.w)
	}
}

func (c *inputTestCtx) state() InputState {
	return getInputState(c.w, c.layout.Shape.ID)
}

// --- OnChar tests ---

func TestInputOnCharInsert(t *testing.T) {
	ctx := newInputTest("hello", "f500", 5)
	ctx.fireChar('!')
	if ctx.lastText != "hello!" {
		t.Fatalf("got %q, want %q", ctx.lastText, "hello!")
	}
}

func TestInputOnCharInsertAtMiddle(t *testing.T) {
	ctx := newInputTest("ab", "f501", 1)
	ctx.fireChar('X')
	if ctx.lastText != "aXb" {
		t.Fatalf("got %q, want %q", ctx.lastText, "aXb")
	}
}

func TestInputKeyDownBackspace(t *testing.T) {
	ctx := newInputTest("abc", "f550", 3)
	ctx.fireKeyDown(KeyBackspace, 0)
	if ctx.lastText != "ab" {
		t.Fatalf("got %q, want %q", ctx.lastText, "ab")
	}
}

func TestInputKeyDownDelete(t *testing.T) {
	ctx := newInputTest("abc", "f551", 0)
	ctx.fireKeyDown(KeyDelete, 0)
	if ctx.lastText != "bc" {
		t.Fatalf("got %q, want %q", ctx.lastText, "bc")
	}
}

func TestInputKeyDownEnterMultiline(t *testing.T) {
	ctx := newInputTestMultiline("ab", "f505", 2)
	ctx.fireKeyDown(KeyEnter, 0)
	if ctx.lastText != "ab\n" {
		t.Fatalf("got %q, want %q", ctx.lastText, "ab\n")
	}
}

func TestInputKeyDownEnterSingleLine(t *testing.T) {
	committed := false
	w := newTestWindow()
	w.SetFocus("f600")
	setInputState(w, "f600", InputState{CursorPos: 2})
	layout := generateViewLayout(Input(InputCfg{
		Text:      "hi",
		Focusable: true, ID: "f600",
		OnTextCommit: func(_ *Layout, _ string, reason InputCommitReason, _ *Window) {
			committed = true
			if reason != CommitEnter {
				t.Fatalf("got reason %d, want CommitEnter", reason)
			}
		},
	}), w)
	e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
	layout.Shape.events.OnKeyDown(&layout, e, w)
	if !committed {
		t.Fatal("OnTextCommit not called")
	}
}

func TestInputOnCharUndo(t *testing.T) {
	ctx := newInputTest("hello", "f506", 5)
	ctx.fireChar('!')
	if ctx.lastText != "hello!" {
		t.Fatalf("insert: got %q", ctx.lastText)
	}
	// Rebuild layout with new text for undo.
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:      ctx.lastText,
		Focusable: true, ID: "f506",
		OnTextChanged: func(_ *Layout, newText string, _ *Window) {
			ctx.lastText = newText
		},
	}), ctx.w)
	ctx.fireKeyDown(KeyZ, ModCtrl)
	if ctx.lastText != "hello" {
		t.Fatalf("undo: got %q, want %q", ctx.lastText, "hello")
	}
}

func TestInputOnCharRedo(t *testing.T) {
	ctx := newInputTest("hello", "f507", 5)
	ctx.fireChar('!')
	// Rebuild with new text.
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:      ctx.lastText,
		Focusable: true, ID: "f507",
		OnTextChanged: func(_ *Layout, newText string, _ *Window) {
			ctx.lastText = newText
		},
	}), ctx.w)
	ctx.fireKeyDown(KeyZ, ModCtrl) // undo
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:      ctx.lastText,
		Focusable: true, ID: "f507",
		OnTextChanged: func(_ *Layout, newText string, _ *Window) {
			ctx.lastText = newText
		},
	}), ctx.w)
	ctx.fireKeyDown(KeyZ, ModCtrl|ModShift) // redo
	if ctx.lastText != "hello!" {
		t.Fatalf("redo: got %q, want %q", ctx.lastText, "hello!")
	}
}

func TestInputSelectAll(t *testing.T) {
	ctx := newInputTest("abc", "f508", 1)
	ctx.fireKeyDown(KeyA, ModCtrl)
	is := ctx.state()
	if is.SelectBeg != 0 || is.SelectEnd != 3 {
		t.Fatalf("select all: got %d-%d, want 0-3",
			is.SelectBeg, is.SelectEnd)
	}
}

func TestInputCopyPaste(t *testing.T) {
	var clipboard string
	ctx := newInputTest("hello", "f509", 0)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	ctx.w.SetClipboardGetFn(func() string { return clipboard })
	// Select all, copy.
	setInputState(ctx.w, "f509", InputState{
		CursorPos: 5, SelectBeg: 0, SelectEnd: 5,
	})
	ctx.fireKeyDown(KeyC, ModCtrl)
	if clipboard != "hello" {
		t.Fatalf("copy: clipboard=%q, want hello", clipboard)
	}
	// Move cursor to end, paste.
	setInputState(ctx.w, "f509", InputState{CursorPos: 5})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:      "hello",
		Focusable: true, ID: "f509",
		OnTextChanged: func(_ *Layout, newText string, _ *Window) {
			ctx.lastText = newText
		},
	}), ctx.w)
	ctx.fireKeyDown(KeyV, ModCtrl)
	if ctx.lastText != "hellohello" {
		t.Fatalf("paste: got %q, want %q", ctx.lastText, "hellohello")
	}
}

func TestInputCut(t *testing.T) {
	var clipboard string
	ctx := newInputTest("abcd", "f510", 2)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	setInputState(ctx.w, "f510", InputState{
		CursorPos: 2, SelectBeg: 1, SelectEnd: 3,
	})
	ctx.fireKeyDown(KeyX, ModCtrl)
	if clipboard != "bc" {
		t.Fatalf("cut clipboard=%q, want bc", clipboard)
	}
	if ctx.lastText != "ad" {
		t.Fatalf("cut text=%q, want ad", ctx.lastText)
	}
}

// --- OnKeyDown tests ---

func TestInputOnKeyDownLeft(t *testing.T) {
	ctx := newInputTest("abc", "f600", 2)
	ctx.fireKeyDown(KeyLeft, ModNone)
	is := ctx.state()
	if is.CursorPos != 1 {
		t.Fatalf("cursor=%d, want 1", is.CursorPos)
	}
}

func TestInputOnKeyDownRight(t *testing.T) {
	ctx := newInputTest("abc", "f601", 1)
	ctx.fireKeyDown(KeyRight, ModNone)
	is := ctx.state()
	if is.CursorPos != 2 {
		t.Fatalf("cursor=%d, want 2", is.CursorPos)
	}
}

func TestInputOnKeyDownShiftLeft(t *testing.T) {
	ctx := newInputTest("abc", "f602", 2)
	ctx.fireKeyDown(KeyLeft, ModShift)
	is := ctx.state()
	if is.CursorPos != 1 {
		t.Fatalf("cursor=%d, want 1", is.CursorPos)
	}
	if is.SelectBeg != 2 || is.SelectEnd != 1 {
		t.Fatalf("sel=%d-%d, want 2-1", is.SelectBeg, is.SelectEnd)
	}
}

func TestInputOnKeyDownHome(t *testing.T) {
	ctx := newInputTest("hello", "f603", 3)
	ctx.fireKeyDown(KeyHome, ModNone)
	is := ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("cursor=%d, want 0", is.CursorPos)
	}
}

func TestInputOnKeyDownEnd(t *testing.T) {
	ctx := newInputTest("hello", "f604", 1)
	ctx.fireKeyDown(KeyEnd, ModNone)
	is := ctx.state()
	if is.CursorPos != 5 {
		t.Fatalf("cursor=%d, want 5", is.CursorPos)
	}
}

func TestInputOnKeyDownEscape(t *testing.T) {
	ctx := newInputTest("abc", "f605", 2)
	setInputState(ctx.w, "f605", InputState{
		CursorPos: 2, SelectBeg: 0, SelectEnd: 3,
	})
	ctx.fireKeyDown(KeyEscape, ModNone)
	is := ctx.state()
	if is.SelectBeg != 0 || is.SelectEnd != 0 {
		t.Fatalf("sel=%d-%d, want 0-0", is.SelectBeg, is.SelectEnd)
	}
}

func TestInputOnKeyDownWordLeft(t *testing.T) {
	ctx := newInputTest("hello world", "f606", 11)
	ctx.fireKeyDown(KeyLeft, ModCtrl)
	is := ctx.state()
	if is.CursorPos != 6 {
		t.Fatalf("cursor=%d, want 6", is.CursorPos)
	}
}

func TestInputOnKeyDownWordRight(t *testing.T) {
	ctx := newInputTest("hello world", "f607", 0)
	ctx.fireKeyDown(KeyRight, ModCtrl)
	is := ctx.state()
	if is.CursorPos != 6 {
		t.Fatalf("cursor=%d, want 6", is.CursorPos)
	}
}

func TestInputOnKeyDownUpDown(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef\nghi", "f608", 5)
	// Cursor at "ef" (line 1, col 1). Move up.
	ctx.fireKeyDown(KeyUp, ModNone)
	is := ctx.state()
	if is.CursorPos != 1 {
		t.Fatalf("up: cursor=%d, want 1", is.CursorPos)
	}
	// Move down twice to get to last line.
	ctx.fireKeyDown(KeyDown, ModNone)
	ctx.fireKeyDown(KeyDown, ModNone)
	is = ctx.state()
	if is.CursorPos != 9 {
		t.Fatalf("down: cursor=%d, want 9", is.CursorPos)
	}
}

func TestInputOnKeyDownLeftCollapsesSelection(t *testing.T) {
	ctx := newInputTest("abcdef", "f609", 3)
	setInputState(ctx.w, "f609", InputState{
		CursorPos: 3, SelectBeg: 1, SelectEnd: 4,
	})
	ctx.fireKeyDown(KeyLeft, ModNone)
	is := ctx.state()
	// Should collapse to selection start (beg=1).
	if is.CursorPos != 1 {
		t.Fatalf("cursor=%d, want 1", is.CursorPos)
	}
	if is.SelectBeg != 0 || is.SelectEnd != 0 {
		t.Fatalf("sel not cleared: %d-%d", is.SelectBeg, is.SelectEnd)
	}
}

// --- Cursor movement helpers ---

func TestMoveCursorWordLeft(t *testing.T) {
	runes := []rune("hello world test")
	assertEqual(t, moveCursorWordLeft(runes, 16), 12)
	assertEqual(t, moveCursorWordLeft(runes, 12), 6)
	assertEqual(t, moveCursorWordLeft(runes, 6), 0)
	assertEqual(t, moveCursorWordLeft(runes, 0), 0)
}

func TestMoveCursorWordRight(t *testing.T) {
	runes := []rune("hello world test")
	assertEqual(t, moveCursorWordRight(runes, 0), 6)
	assertEqual(t, moveCursorWordRight(runes, 6), 12)
	assertEqual(t, moveCursorWordRight(runes, 12), 16)
}

func TestWordBoundsAt(t *testing.T) {
	runes := []rune("hello world test")
	// Middle of first word.
	b, e := wordBoundsAt(runes, 2)
	if b != 0 || e != 5 {
		t.Fatalf("got %d-%d, want 0-5", b, e)
	}
	// On space between words.
	b, e = wordBoundsAt(runes, 5)
	if b != 5 || e != 6 {
		t.Fatalf("got %d-%d, want 5-6", b, e)
	}
	// Last word.
	b, e = wordBoundsAt(runes, 14)
	if b != 12 || e != 16 {
		t.Fatalf("got %d-%d, want 12-16", b, e)
	}
	// Empty string.
	b, e = wordBoundsAt(nil, 0)
	if b != 0 || e != 0 {
		t.Fatalf("empty: got %d-%d, want 0-0", b, e)
	}
}

func TestMoveCursorUpDown(t *testing.T) {
	runes := []rune("abc\ndef\nghi")
	// From middle of line 1 → line 0.
	assertEqual(t, moveCursorUp(runes, 5), 1)
	// From line 0 → stays at 0.
	assertEqual(t, moveCursorUp(runes, 1), 0)
	// From line 0 → line 1.
	assertEqual(t, moveCursorDown(runes, 1), 5)
	// From line 1 → line 2.
	assertEqual(t, moveCursorDown(runes, 5), 9)
	// From last line → end.
	assertEqual(t, moveCursorDown(runes, 9), 11)
}

func TestMoveCursorLineStartEnd(t *testing.T) {
	runes := []rune("abc\ndef")
	assertEqual(t, moveCursorLineStart(runes, 5), 4)
	assertEqual(t, moveCursorLineEnd(runes, 0), 3)
	assertEqual(t, moveCursorLineEnd(runes, 4), 7)
}

// --- Cursor render tests ---

func TestInputCursorRenderedWhenFocused(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	w.SetFocus("f700")
	setInputState(w, "f700", InputState{CursorPos: 2})
	style := DefaultTextStyle
	shape := &Shape{
		Focusable: true, ID: "f700",
		shapeType: shapeText,
		Width:     200,
		Height:    20,
		TC: &ShapeTextConfig{
			Text:      "hello",
			TextStyle: &style,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill && r.W < 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("cursor RenderRect not emitted")
	}
}

func TestInputCursorNotRenderedWhenUnfocused(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	style := DefaultTextStyle
	shape := &Shape{
		Focusable: true, ID: "f700",
		shapeType: shapeText,
		TC: &ShapeTextConfig{
			Text:      "hello",
			TextStyle: &style,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	if len(w.renderers) != 0 {
		t.Fatal("cursor should not render when unfocused")
	}
}

func TestInputCursorNotRenderedWhenBlinkOff(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("f701")
	w.viewState.inputCursorOn.Store(false)
	style := DefaultTextStyle
	shape := &Shape{
		Focusable: true, ID: "f701",
		shapeType: shapeText,
		TC: &ShapeTextConfig{
			Text:      "hello",
			TextStyle: &style,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	if len(w.renderers) != 0 {
		t.Fatal("cursor should not render when blink off")
	}
}

func TestInputCursorUsesColumnZeroForPlaceholder(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	w.SetFocus("f702")
	setInputState(w, "f702", InputState{CursorPos: 8})
	style := DefaultTextStyle
	shape := &Shape{
		Focusable: true, ID: "f702",
		shapeType: shapeText,
		Width:     200,
		Height:    20,
		TC: &ShapeTextConfig{
			Text:              "Add your task",
			TextStyle:         &style,
			TextIsPlaceholder: true,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputCursor(shape, "Add your task", 0, 0, glyph.Layout{}, false, w)
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill && r.W < 2 {
			if r.X != 0 {
				t.Fatalf("cursor x=%v, want 0", r.X)
			}
			return
		}
	}
	t.Fatal("cursor RenderRect not emitted")
}

// --- Selection render tests ---

func TestInputSelectionRendered(t *testing.T) {
	w := newTestWindow()
	style := DefaultTextStyle
	shape := &Shape{
		shapeType: shapeText,
		Width:     200,
		Height:    20,
		TC: &ShapeTextConfig{
			Text:       "hello",
			TextStyle:  &style,
			TextSelBeg: 1,
			TextSelEnd: 4,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputSelection(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill && r.Color.A > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("selection RenderRect not emitted")
	}
}

func TestInputSelectionNotRenderedWhenNoSelection(t *testing.T) {
	w := newTestWindow()
	style := DefaultTextStyle
	shape := &Shape{
		shapeType: shapeText,
		TC: &ShapeTextConfig{
			Text:       "hello",
			TextStyle:  &style,
			TextSelBeg: 0,
			TextSelEnd: 0,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputSelection(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	if len(w.renderers) != 0 {
		t.Fatal("selection should not render with no selection")
	}
}

func TestInputSelectionMultiline(t *testing.T) {
	w := newTestWindow()
	style := DefaultTextStyle
	text := "abc\ndef\nghi"
	shape := &Shape{
		shapeType: shapeText,
		Width:     200,
		Height:    60,
		TC: &ShapeTextConfig{
			Text:       text,
			TextStyle:  &style,
			TextMode:   TextModeWrapKeepSpaces,
			TextSelBeg: 2,
			TextSelEnd: uint32(utf8.RuneCountInString("abc\nde")),
		},
	}
	w.renderers = w.renderers[:0]
	renderInputSelection(shape, text, 0, 0, glyph.Layout{}, false, w)
	count := 0
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill {
			count++
		}
	}
	// Fallback (no textMeasurer) emits a single approximation rect.
	// With a real glyph backend, GetSelectionRects returns per-line rects.
	if count < 1 {
		t.Fatalf("expected >=1 selection rects, got %d", count)
	}
}

func TestInputOnKeyDownHomeCycleToDocument(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef\nghi", "f700", 5)
	ctx.fireKeyDown(KeyHome, ModNone)
	is := ctx.state()
	if is.CursorPos != 4 {
		t.Fatalf("Home 1: cursor=%d, want 4", is.CursorPos)
	}
	ctx.fireKeyDown(KeyHome, ModNone)
	is = ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("Home 2: cursor=%d, want 0", is.CursorPos)
	}
}

func TestInputOnKeyDownEndCycleToDocument(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef\nghi", "f701", 5)
	ctx.fireKeyDown(KeyEnd, ModNone)
	is := ctx.state()
	if is.CursorPos != 7 {
		t.Fatalf("End 1: cursor=%d, want 7", is.CursorPos)
	}
	ctx.fireKeyDown(KeyEnd, ModNone)
	is = ctx.state()
	if is.CursorPos != 11 {
		t.Fatalf("End 2: cursor=%d, want 11", is.CursorPos)
	}
}

func TestInputOnKeyDownHomeAtDocStart(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef", "f702", 0)
	ctx.fireKeyDown(KeyHome, ModNone)
	is := ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("cursor=%d, want 0", is.CursorPos)
	}
}

func TestInputOnKeyDownEndAtDocEnd(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef", "f703", 7)
	ctx.fireKeyDown(KeyEnd, ModNone)
	is := ctx.state()
	if is.CursorPos != 7 {
		t.Fatalf("cursor=%d, want 7", is.CursorPos)
	}
}

func TestInputOnKeyDownShiftHomeCycleSelection(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef\nghi", "f704", 5)
	ctx.fireKeyDown(KeyHome, ModShift)
	is := ctx.state()
	if is.CursorPos != 4 {
		t.Fatalf("Shift+Home 1: cursor=%d, want 4", is.CursorPos)
	}
	if is.SelectBeg != 5 || is.SelectEnd != 4 {
		t.Fatalf("Shift+Home 1: sel=%d-%d, want 5-4",
			is.SelectBeg, is.SelectEnd)
	}
	ctx.fireKeyDown(KeyHome, ModShift)
	is = ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("Shift+Home 2: cursor=%d, want 0", is.CursorPos)
	}
	if is.SelectBeg != 5 || is.SelectEnd != 0 {
		t.Fatalf("Shift+Home 2: sel=%d-%d, want 5-0",
			is.SelectBeg, is.SelectEnd)
	}
}

func TestInputOnKeyDownShiftEndCycleSelection(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef\nghi", "f705", 5)
	ctx.fireKeyDown(KeyEnd, ModShift)
	is := ctx.state()
	if is.CursorPos != 7 {
		t.Fatalf("Shift+End 1: cursor=%d, want 7", is.CursorPos)
	}
	if is.SelectBeg != 5 || is.SelectEnd != 7 {
		t.Fatalf("Shift+End 1: sel=%d-%d, want 5-7",
			is.SelectBeg, is.SelectEnd)
	}
	ctx.fireKeyDown(KeyEnd, ModShift)
	is = ctx.state()
	if is.CursorPos != 11 {
		t.Fatalf("Shift+End 2: cursor=%d, want 11", is.CursorPos)
	}
	if is.SelectBeg != 5 || is.SelectEnd != 11 {
		t.Fatalf("Shift+End 2: sel=%d-%d, want 5-11",
			is.SelectBeg, is.SelectEnd)
	}
}

func TestInputOnKeyDownHomeSingleLine(t *testing.T) {
	ctx := newInputTest("hello", "f706", 3)
	ctx.fireKeyDown(KeyHome, ModNone)
	is := ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("Home 1: cursor=%d, want 0", is.CursorPos)
	}
	ctx.fireKeyDown(KeyHome, ModNone)
	is = ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("Home 2: cursor=%d, want 0", is.CursorPos)
	}
}

func TestMakeInputOnKeyUp_FocusCheck(t *testing.T) {
	t.Parallel()
	// Test that handler is only called when focused
	called := false
	hcfg := inputHandlerCfg{
		FocusID: "f123",
		OnKeyUp: func(_ *Layout, e *Event, _ *Window) {
			called = true
		},
	}

	handler := makeInputOnKeyUp(hcfg)
	layout := &Layout{}
	w := &Window{}
	e := &Event{KeyCode: KeyEnter}

	// Test when not focused - should not call handler
	handler(layout, e, w)
	if called {
		t.Error("Handler should not be called when not focused")
	}

	// Test when focused - should call handler
	w.SetFocus("f123")
	called = false
	handler(layout, e, w)
	if !called {
		t.Error("Handler should be called when focused")
	}
}

func TestMakeInputOnKeyUp_ZeroIDFocusNoCall(t *testing.T) {
	t.Parallel()
	// Test that IDFocus=0 prevents handler from being called
	called := false
	hcfg := inputHandlerCfg{
		FocusID: "", // Empty focus id should prevent calls
		OnKeyUp: func(_ *Layout, e *Event, _ *Window) {
			called = true
		},
	}

	handler := makeInputOnKeyUp(hcfg)
	layout := &Layout{}
	w := &Window{}
	w.ClearFocus() // Even with focus set to 0
	e := &Event{KeyCode: KeyEnter}

	handler(layout, e, w)
	if called {
		t.Error("Handler should not be called when IDFocus is 0")
	}
}

func TestMakeInputOnKeyUp_NilHandler(t *testing.T) {
	t.Parallel()
	// Test that nil OnKeyUp doesn't panic
	hcfg := inputHandlerCfg{
		FocusID: "f123",
		OnKeyUp: nil, // Nil handler
	}

	handler := makeInputOnKeyUp(hcfg)
	layout := &Layout{}
	w := &Window{}
	w.SetFocus("f123")
	e := &Event{KeyCode: KeyEnter}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("makeInputOnKeyUp panicked with nil handler: %v", r)
		}
	}()

	handler(layout, e, w)
}
