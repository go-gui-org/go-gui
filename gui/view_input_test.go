package gui

import (
	"slices"
	"strings"
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
		Mode: InputMultiline,
		ID:   "f11",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleTextArea {
		t.Fatalf("got role %d, want TextArea", layout.Shape.A11YRole)
	}
}

// ReadOnly is the only trigger for the read-only announcement.
// FocusDisabled alone must NOT announce read-only: with the
// focusable-by-default flip, non-focusable is an explicit opt-out,
// not the historical proxy for an uneditable field.
func TestInputReadOnlyWithoutFocus(t *testing.T) {
	w := newTestWindow()

	ro := generateViewLayout(Input(InputCfg{ID: "input_test_test_input_read_only_without_focus", Text: "readonly", ReadOnly: true}), w)
	if ro.Shape.A11YState != AccessStateReadOnly {
		t.Fatalf("ReadOnly: got state %d, want ReadOnly", ro.Shape.A11YState)
	}

	fd := generateViewLayout(Input(InputCfg{Text: "x", FocusDisabled: true}), w)
	if fd.Shape.A11YState == AccessStateReadOnly {
		t.Fatal("FocusDisabled alone must not announce ReadOnly")
	}
}

// countFocusCandidates walks the layout and returns the number of
// distinct tab stops, pinning against duplicate focus candidates.
func countFocusCandidates(layout *Layout) int {
	var candidates []focusCandidate
	collectFocusCandidates(layout, &candidates, map[string]struct{}{})
	return len(candidates)
}

// An Input with an ID and no FocusDisabled is focusable by default and
// exposes exactly one tab stop (the inner Text is FocusSkip).
func TestInputFocusableByDefaultOneCandidate(t *testing.T) {
	w := newTestWindow()
	layout := generateViewLayout(Input(InputCfg{ID: "f_def", Text: "x"}), w)
	if got := countFocusCandidates(&layout); got != 1 {
		t.Fatalf("got %d focus candidates, want 1", got)
	}
}

// FocusDisabled opts out: zero candidates.
func TestInputFocusDisabledNoCandidates(t *testing.T) {
	w := newTestWindow()
	layout := generateViewLayout(Input(InputCfg{
		ID: "f_off", Text: "x", FocusDisabled: true,
	}), w)
	if got := countFocusCandidates(&layout); got != 0 {
		t.Fatalf("got %d focus candidates, want 0", got)
	}
}

// The old TestInputNoIDInert lived here. Its contract is gone: an
// Input without an ID used to render, accept clicks, and never join
// the tab order, with nothing to say so. The silence was the defect,
// so the factory now refuses the config outright. See
// TestFocusWidgetsRequireID, which covers Input alongside the other
// eight focusable-by-default factories.

func TestInputPlaceholderWhenEmpty(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{
		Placeholder: "Type here",
		ID:          "f12",
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
	setInputState(w, "f14", inputState{
		CursorPos: 7,
		selectBeg: 2,
		selectEnd: 5,
	})
	layout := generateViewLayout(Input(InputCfg{
		Placeholder: "Type here",
		ID:          "f14",
	}), w)
	if len(layout.Children) == 0 {
		t.Fatal("no children")
	}
	inner := &layout.Children[0]
	if inner.Shape.events == nil || inner.Shape.events.OnClick == nil {
		t.Fatal("missing click handler")
	}
	e := &Event{MouseX: inner.Shape.X, MouseY: inner.Shape.Y}
	inner.Shape.events.OnClick(EventCtx{inner, e, w})
	is := getInputState(w, "f14")
	if is.CursorPos != 0 {
		t.Fatalf("cursor=%d, want 0", is.CursorPos)
	}
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Fatalf("selection=%d-%d, want 0-0", is.selectBeg, is.selectEnd)
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
	cfg := InputCfg{ID: "input_test_test_input_defaults"}
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
		ID:          "f13",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.a11Y == nil {
		t.Fatal("A11Y nil")
	}
	if layout.Shape.a11Y.Label != "Search..." {
		t.Fatalf("got %q, want Search...", layout.Shape.a11Y.Label)
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
	setInputState(ctx.w, focusID, inputState{CursorPos: cursorPos})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text: text,
		ID:   focusID,
		OnTextChanged: func(newText string, _ EventCtx) {
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
	setInputState(ctx.w, focusID, inputState{CursorPos: cursorPos})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text: text,
		ID:   focusID,
		Mode: InputMultiline,
		OnTextChanged: func(newText string, _ EventCtx) {
			ctx.lastText = newText
		},
	}), ctx.w)
	return ctx
}

func (c *inputTestCtx) fireChar(charCode uint32) {
	e := &Event{Type: EventChar, CharCode: charCode}
	if c.layout.Shape.events != nil && c.layout.Shape.events.OnChar != nil {
		c.layout.Shape.events.OnChar(EventCtx{&c.layout, e, c.w})
	}
}

func (c *inputTestCtx) fireKeyDown(key KeyCode, mod Modifier) {
	e := &Event{Type: EventKeyDown, KeyCode: key, Modifiers: mod}
	if c.layout.Shape.events != nil && c.layout.Shape.events.OnKeyDown != nil {
		c.layout.Shape.events.OnKeyDown(EventCtx{&c.layout, e, c.w})
	}
}

func (c *inputTestCtx) state() inputState {
	return getInputState(c.w, c.layout.Shape.ID)
}

// TestInputSingleLineTextCentersVertically renders a single-line
// input taller than its font and asserts the inner text shape is
// vertically centered, not pinned to the top. Data grid filter
// cells (no padding, input fills the row) regressed on this: text
// sized FillFill defeats the inner row's VAlignMiddle.
func TestInputSingleLineTextCentersVertically(t *testing.T) {
	w := &Window{}
	w.windowWidth = 200
	w.windowHeight = 40
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}

	layout := generateViewLayout(Input(InputCfg{
		ID:      "f700",
		Text:    "hi",
		Width:   200,
		Height:  40,
		Sizing:  FixedFixed,
		Padding: NoPadding,
	}), w)
	layoutPipeline(&layout, w)

	if len(layout.Children) == 0 || len(layout.Children[0].Children) == 0 {
		t.Fatal("input layout missing inner text child")
	}
	txt := layout.Children[0].Children[0].Shape
	if txt.TC == nil {
		t.Fatal("inner child is not a text shape")
	}
	// FillFit must hug the stub font height; a Fill text stretches
	// to the content box (~38 after borders) and pins glyphs to the
	// top even though its midpoint still matches the input's.
	if abs32(txt.Height-20) > 1 {
		t.Fatalf("text height = %.1f, want ~20 (font height)", txt.Height)
	}
	// Border (default) insets the content box symmetrically, so the
	// text midpoint must still sit at the input midpoint.
	wantMidY := layout.Shape.Y + 20
	gotMidY := txt.Y + txt.Height/2
	if abs32(gotMidY-wantMidY) > 1 {
		t.Fatalf("text midY = %.1f, want ~%.1f (top=%.1f h=%.1f)",
			gotMidY, wantMidY, txt.Y, txt.Height)
	}
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

// Batched-event regression tests: backends drain all pending events
// against the same layout tree before the next frame rebuild. Each
// mutation must be visible to the next event in the batch via the
// layout write-back in fireTextChanged, or earlier keystrokes are
// silently overwritten (dropped chars, backspaces that do not stick).

func TestInputOnCharBatchedCompound(t *testing.T) {
	ctx := newInputTest("ab", "f502", 2)
	ctx.fireChar('c')
	ctx.fireChar('d')
	if ctx.lastText != "abcd" {
		t.Fatalf("got %q, want %q", ctx.lastText, "abcd")
	}
}

func TestInputOnCharBatchedFromPlaceholder(t *testing.T) {
	ctx := newInputTest("", "f503", 0)
	ctx.fireChar('a')
	ctx.fireChar('b')
	if ctx.lastText != "ab" {
		t.Fatalf("got %q, want %q", ctx.lastText, "ab")
	}
}

func TestInputKeyDownBatchedBackspace(t *testing.T) {
	ctx := newInputTest("abcd", "f504", 4)
	ctx.fireKeyDown(KeyBackspace, 0)
	ctx.fireKeyDown(KeyBackspace, 0)
	if ctx.lastText != "ab" {
		t.Fatalf("got %q, want %q", ctx.lastText, "ab")
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
	setInputState(w, "f600", inputState{CursorPos: 2})
	layout := generateViewLayout(Input(InputCfg{
		Text: "hi",
		ID:   "f600",
		OnTextCommit: func(_ string, reason InputCommitReason, ctx EventCtx) {
			committed = true
			if reason != commitEnter {
				t.Fatalf("got reason %d, want CommitEnter", reason)
			}
		},
	}), w)
	e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
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
		Text: ctx.lastText,
		ID:   "f506",
		OnTextChanged: func(newText string, _ EventCtx) {
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
		Text: ctx.lastText,
		ID:   "f507",
		OnTextChanged: func(newText string, _ EventCtx) {
			ctx.lastText = newText
		},
	}), ctx.w)
	ctx.fireKeyDown(KeyZ, ModCtrl) // undo
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text: ctx.lastText,
		ID:   "f507",
		OnTextChanged: func(newText string, _ EventCtx) {
			ctx.lastText = newText
		},
	}), ctx.w)
	ctx.fireKeyDown(KeyZ, ModCtrl|ModShift) // redo
	if ctx.lastText != "hello!" {
		t.Fatalf("redo: got %q, want %q", ctx.lastText, "hello!")
	}
}

// Typing one character at a time through the real OnChar pipeline
// must coalesce into a single undo step: one Ctrl+Z restores "", and
// one redo restores the whole run (issue #328).
func TestInputUndoCoalescesTypingRun(t *testing.T) {
	ctx := newInputTest("", "f508", 0)
	for _, ch := range "hello" {
		ctx.fireChar(uint32(ch))
		ctx.layout = generateViewLayout(Input(InputCfg{
			Text: ctx.lastText,
			ID:   "f508",
			OnTextChanged: func(newText string, _ EventCtx) {
				ctx.lastText = newText
			},
		}), ctx.w)
	}
	if ctx.lastText != "hello" {
		t.Fatalf("typing: got %q, want %q", ctx.lastText, "hello")
	}
	ctx.fireKeyDown(KeyZ, ModCtrl)
	if ctx.lastText != "" {
		t.Fatalf("undo: got %q, want empty", ctx.lastText)
	}
	ctx.fireKeyDown(KeyZ, ModCtrl|ModShift)
	if ctx.lastText != "hello" {
		t.Fatalf("redo: got %q, want %q", ctx.lastText, "hello")
	}
}

func TestInputSelectAll(t *testing.T) {
	ctx := newInputTest("abc", "f508", 1)
	ctx.fireKeyDown(KeyA, ModCtrl)
	is := ctx.state()
	if is.selectBeg != 0 || is.selectEnd != 3 {
		t.Fatalf("select all: got %d-%d, want 0-3",
			is.selectBeg, is.selectEnd)
	}
}

func TestInputCopyPaste(t *testing.T) {
	var clipboard string
	ctx := newInputTest("hello", "f509", 0)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	ctx.w.SetClipboardGetFn(func() string { return clipboard })
	// Select all, copy.
	setInputState(ctx.w, "f509", inputState{
		CursorPos: 5, selectBeg: 0, selectEnd: 5,
	})
	ctx.fireKeyDown(KeyC, ModCtrl)
	if clipboard != "hello" {
		t.Fatalf("copy: clipboard=%q, want hello", clipboard)
	}
	// Move cursor to end, paste.
	setInputState(ctx.w, "f509", inputState{CursorPos: 5})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text: "hello",
		ID:   "f509",
		OnTextChanged: func(newText string, _ EventCtx) {
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
	setInputState(ctx.w, "f510", inputState{
		CursorPos: 2, selectBeg: 1, selectEnd: 3,
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
	if is.selectBeg != 2 || is.selectEnd != 1 {
		t.Fatalf("sel=%d-%d, want 2-1", is.selectBeg, is.selectEnd)
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
	setInputState(ctx.w, "f605", inputState{
		CursorPos: 2, selectBeg: 0, selectEnd: 3,
	})
	ctx.fireKeyDown(KeyEscape, ModNone)
	is := ctx.state()
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Fatalf("sel=%d-%d, want 0-0", is.selectBeg, is.selectEnd)
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
	setInputState(ctx.w, "f609", inputState{
		CursorPos: 3, selectBeg: 1, selectEnd: 4,
	})
	ctx.fireKeyDown(KeyLeft, ModNone)
	is := ctx.state()
	// Should collapse to selection start (beg=1).
	if is.CursorPos != 1 {
		t.Fatalf("cursor=%d, want 1", is.CursorPos)
	}
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Fatalf("sel not cleared: %d-%d", is.selectBeg, is.selectEnd)
	}
}

// --- Grapheme-cluster caret motion (issue #330) ---

// seedInputGlyphLayout stamps a fake glyph layout onto the input's
// inner text shape so the key handler's navigation takes the shaped
// path headlessly. cursors are the byte indices valid as cursor
// positions — the cluster boundaries a real measurer would produce.
func seedInputGlyphLayout(t *testing.T, ctx *inputTestCtx, text string, cursors []int) {
	t.Helper()
	ctx.w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	if len(ctx.layout.Children) == 0 || len(ctx.layout.Children[0].Children) == 0 {
		t.Fatal("input layout missing inner text shape")
	}
	shape := ctx.layout.Children[0].Children[0].Shape
	if shape.TC == nil {
		t.Fatal("inner child is not a text shape")
	}
	style := textStyleOrDefault(shape)
	shape.TC.textLayout = glyphCursorLayout(text, cursors)
	shape.TC.textLayoutWidth = plainTextLayoutWidthArg(shape, shape.TC, style)
	shape.TC.textLayoutText = text
	shape.TC.textLayoutStyle = style
	shape.TC.textLayoutMode = shape.TC.TextMode
	shape.TC.textLayoutValid = true
}

func TestInputArrowLeftGraphemeGlyphPath(t *testing.T) {
	// 👍🏽 = base + skin-tone modifier, one cluster: Left from the end
	// lands before the cluster, never between base and modifier.
	text := "\U0001F44D\U0001F3FD"
	ctx := newInputTest(text, "f620", utf8RuneCount(text))
	seedInputGlyphLayout(t, ctx, text, []int{0, len(text)})
	ctx.fireKeyDown(KeyLeft, ModNone)
	if is := ctx.state(); is.CursorPos != 0 {
		t.Fatalf("cursor=%d, want 0", is.CursorPos)
	}
}

func TestInputArrowRightGraphemeGlyphPath(t *testing.T) {
	// 🇺🇸 = two regional indicators, one cluster: Right from before it
	// steps past the whole cluster in one key.
	text := "\U0001F1FA\U0001F1F8"
	ctx := newInputTest(text, "f621", 0)
	seedInputGlyphLayout(t, ctx, text, []int{0, len(text)})
	ctx.fireKeyDown(KeyRight, ModNone)
	if is := ctx.state(); is.CursorPos != utf8RuneCount(text) {
		t.Fatalf("cursor=%d, want %d", is.CursorPos, utf8RuneCount(text))
	}
}

func TestInputShiftLeftGraphemeGlyphPath(t *testing.T) {
	// Shift+Left from the end of a ZWJ family emoji extends the
	// selection by the whole cluster, not one rune.
	text := "a\U0001F468\u200D\U0001F469\u200D\U0001F467"
	ctx := newInputTest(text, "f622", utf8RuneCount(text))
	seedInputGlyphLayout(t, ctx, text, []int{0, 1, len(text)})
	ctx.fireKeyDown(KeyLeft, ModShift)
	is := ctx.state()
	if is.CursorPos != 1 {
		t.Fatalf("cursor=%d, want 1", is.CursorPos)
	}
	if is.selectBeg != uint32(utf8RuneCount(text)) || is.selectEnd != 1 {
		t.Fatalf("sel=%d-%d, want %d-1", is.selectBeg, is.selectEnd,
			utf8RuneCount(text))
	}
}

func TestInputArrowLeftGraphemeFallback(t *testing.T) {
	// No glyph layout: Left snaps to the previous UAX #29 boundary.
	text := "\U0001F44D\U0001F3FD"
	ctx := newInputTest(text, "f623", utf8RuneCount(text))
	ctx.fireKeyDown(KeyLeft, ModNone)
	if is := ctx.state(); is.CursorPos != 0 {
		t.Fatalf("cursor=%d, want 0", is.CursorPos)
	}
}

func TestInputArrowRightGraphemeFallback(t *testing.T) {
	// No glyph layout: Right steps past the whole cluster in one key.
	text := "\U0001F1FA\U0001F1F8"
	ctx := newInputTest(text, "f624", 0)
	ctx.fireKeyDown(KeyRight, ModNone)
	if is := ctx.state(); is.CursorPos != utf8RuneCount(text) {
		t.Fatalf("cursor=%d, want %d", is.CursorPos, utf8RuneCount(text))
	}
}

func TestInputBackspaceGraphemeFallback(t *testing.T) {
	// One Backspace on a ZWJ family emoji removes the whole cluster,
	// leaving the leading "a".
	text := "a\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466"
	ctx := newInputTest(text, "f625", utf8RuneCount(text))
	ctx.fireKeyDown(KeyBackspace, ModNone)
	if ctx.lastText != "a" {
		t.Fatalf("text=%q, want %q", ctx.lastText, "a")
	}
	if is := ctx.state(); is.CursorPos != 1 {
		t.Fatalf("cursor=%d, want 1", is.CursorPos)
	}
}

func TestInputForwardDeleteGraphemeFallback(t *testing.T) {
	// Forward Delete at the start of a skin-tone emoji removes the
	// whole cluster — symmetric with Backspace.
	text := "\U0001F44D\U0001F3FD"
	ctx := newInputTest(text, "f626", 0)
	ctx.fireKeyDown(KeyDelete, ModNone)
	if ctx.lastText != "" {
		t.Fatalf("text=%q, want empty", ctx.lastText)
	}
}

func TestInputArrowDownGraphemeFallback(t *testing.T) {
	// Down from the end of "ab" onto a line whose ZWJ emoji starts at
	// the target column: rune-column math lands at rune 5 (inside the
	// cluster); the fallback snaps to the nearest boundary, 4.
	text := "ab\na\U0001F468\u200D\U0001F469\u200D\U0001F467"
	ctx := newInputTestMultiline(text, "f627", 2)
	ctx.fireKeyDown(KeyDown, ModNone)
	if is := ctx.state(); is.CursorPos != 4 {
		t.Fatalf("cursor=%d, want 4", is.CursorPos)
	}
}

// --- Cursor movement helpers ---

func TestMoveCursorWordLeft(t *testing.T) {
	const text = "hello world test"
	assertEqual(t, moveCursorWordLeft(text, 16), 12)
	assertEqual(t, moveCursorWordLeft(text, 12), 6)
	assertEqual(t, moveCursorWordLeft(text, 6), 0)
	assertEqual(t, moveCursorWordLeft(text, 0), 0)
}

func TestMoveCursorWordRight(t *testing.T) {
	const text = "hello world test"
	assertEqual(t, moveCursorWordRight(text, 0), 6)
	assertEqual(t, moveCursorWordRight(text, 6), 12)
	// Past the last word start there is nowhere to go but the end.
	assertEqual(t, moveCursorWordRight(text, 12), 16)
	// At the end, and beyond it, the caret stays put.
	assertEqual(t, moveCursorWordRight(text, 16), 16)
	assertEqual(t, moveCursorWordRight(text, 100), 16)
}

// Word motion segments by rune class, so a punctuation run is a word of
// its own and a script change ends a word (issue #329). These are the
// same rules go-glyph applies when a layout is available.
func TestMoveCursorWordClassRuns(t *testing.T) {
	// Walking left from the end of a dotted identifier must stop at every
	// component and every separator, not jump the whole token.
	const dotted = "foo.bar.baz"
	var left []int
	for pos := len(dotted); pos > 0; {
		pos = moveCursorWordLeft(dotted, pos)
		left = append(left, pos)
	}
	if want := []int{8, 7, 4, 3, 0}; !slices.Equal(left, want) {
		t.Fatalf("word-left walk over %q = %v, want %v", dotted, left, want)
	}

	// Underscore is a word rune, so snake_case stays one word.
	assertEqual(t, moveCursorWordLeft("snake_case_name", 15), 0)

	// Japanese has no spaces; script transitions are the boundaries.
	// 日本語 | の | テキスト at rune indices 0, 3, 4.
	const jp = "日本語のテキスト"
	assertEqual(t, moveCursorWordRight(jp, 0), 3)
	assertEqual(t, moveCursorWordRight(jp, 3), 4)
	assertEqual(t, moveCursorWordLeft(jp, 8), 4)

	// Right motion skips a punctuation run to land on the next word
	// start; the old whitespace-only separator rules skipped through the
	// whole following token instead ("foo.bar" from the dot landed past
	// "bar", at the end of the string).
	assertEqual(t, moveCursorWordRight("foo.bar", 3), 4)
	assertEqual(t, moveCursorWordRight("a+b", 1), 2)
}

func TestWordBoundsAt(t *testing.T) {
	const text = "hello world test"
	// Middle of first word.
	b, e := wordBoundsAt(text, 2)
	if b != 0 || e != 5 {
		t.Fatalf("got %d-%d, want 0-5", b, e)
	}
	// On space between words.
	b, e = wordBoundsAt(text, 5)
	if b != 5 || e != 6 {
		t.Fatalf("got %d-%d, want 5-6", b, e)
	}
	// Last word.
	b, e = wordBoundsAt(text, 14)
	if b != 12 || e != 16 {
		t.Fatalf("got %d-%d, want 12-16", b, e)
	}
	// Empty string.
	b, e = wordBoundsAt("", 0)
	if b != 0 || e != 0 {
		t.Fatalf("empty: got %d-%d, want 0-0", b, e)
	}
	// Caret past the end still selects the final word.
	b, e = wordBoundsAt(text, 16)
	if b != 12 || e != 16 {
		t.Fatalf("past-end: got %d-%d, want 12-16", b, e)
	}
	// Whitespace-only text has no words: the range is empty at pos,
	// matching the layout path (GetWordAtIndex reports the same).
	b, e = wordBoundsAt("  ", 1)
	if b != 1 || e != 1 {
		t.Fatalf("whitespace-only: got %d-%d, want 1-1", b, e)
	}
}

// Double-click selection follows the same class runs as word motion.
func TestWordBoundsAtClassRuns(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		pos      int
		beg, end int
	}{
		{"dotted component", "foo.bar.baz", 5, 4, 7},
		{"separator alone", "foo.bar.baz", 3, 3, 4},
		{"lone punctuation", "a+b", 1, 1, 2},
		{"snake_case is one word", "snake_case_name", 3, 0, 15},
		{"whitespace run selects the run", "hello   world", 6, 5, 8},
		// The combining acute stays attached to its base "e".
		{"combining mark joins base", "café", 3, 0, 5},
		{"han run", "日本語のテキスト", 1, 0, 3},
		{"katakana run", "日本語のテキスト", 5, 4, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beg, end := wordBoundsAt(tt.text, tt.pos)
			if beg != tt.beg || end != tt.end {
				t.Fatalf("wordBoundsAt(%q, %d) = %d-%d, want %d-%d",
					tt.text, tt.pos, beg, end, tt.beg, tt.end)
			}
		})
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
	setInputState(w, "f700", inputState{CursorPos: 2})
	style := DefaultTextStyle
	shape := &Shape{
		Focusable: true, ID: "f700",
		shapeType: shapeText,
		Width:     200,
		Height:    20,
		TC: &shapeTextConfig{
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
		TC: &shapeTextConfig{
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
		TC: &shapeTextConfig{
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

// Input's inner text shape carries neither ID nor Focusable — it names
// the owning container through focusOwner (see Shape.focusKey). The
// caret, and the cursor position it is drawn at, must resolve through
// that reference.
func TestInputCursorRenderedViaFocusOwner(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	w.SetFocus("f703")
	setInputState(w, "f703", inputState{CursorPos: 2})
	style := DefaultTextStyle
	shape := &Shape{
		focusOwner: "f703",
		shapeType:  shapeText,
		Width:      200,
		Height:     20,
		TC: &shapeTextConfig{
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
		t.Fatal("cursor RenderRect not emitted for focusOwner shape")
	}
}

// The owner reference is what decides, not the mere presence of a
// text shape: a shape whose owner is not the focused widget renders
// nothing.
func TestInputCursorNotRenderedWhenFocusOwnerUnfocused(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	w.SetFocus("other")
	style := DefaultTextStyle
	shape := &Shape{
		focusOwner: "f704",
		shapeType:  shapeText,
		TC: &shapeTextConfig{
			Text:      "hello",
			TextStyle: &style,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	if len(w.renderers) != 0 {
		t.Fatal("cursor should not render for an unfocused owner")
	}
}

// Ordinary text renders no caret however the window is focused: it is
// neither Focusable nor owned, so it has no focus state to draw.
func TestInputCursorNotRenderedForPlainText(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	w.SetFocus("f705")
	setInputState(w, "f705", inputState{CursorPos: 2})
	style := DefaultTextStyle
	shape := &Shape{
		ID:        "f705",
		shapeType: shapeText,
		TC: &shapeTextConfig{
			Text:      "hello",
			TextStyle: &style,
		},
	}
	w.renderers = w.renderers[:0]
	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)
	if len(w.renderers) != 0 {
		t.Fatal("cursor should not render for non-focusable text")
	}
}

func TestInputCursorUsesColumnZeroForPlaceholder(t *testing.T) {
	w := newTestWindow()
	w.viewState.inputCursorOn.Store(true)
	w.SetFocus("f702")
	setInputState(w, "f702", inputState{CursorPos: 8})
	style := DefaultTextStyle
	shape := &Shape{
		Focusable: true, ID: "f702",
		shapeType: shapeText,
		Width:     200,
		Height:    20,
		TC: &shapeTextConfig{
			Text:              "Add your task",
			TextStyle:         &style,
			textIsPlaceholder: true,
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
		TC: &shapeTextConfig{
			Text:       "hello",
			TextStyle:  &style,
			textSelBeg: 1,
			textSelEnd: 4,
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
		TC: &shapeTextConfig{
			Text:       "hello",
			TextStyle:  &style,
			textSelBeg: 0,
			textSelEnd: 0,
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
		TC: &shapeTextConfig{
			Text:       text,
			TextStyle:  &style,
			TextMode:   TextModeWrapKeepSpaces,
			textSelBeg: 2,
			textSelEnd: uint32(utf8.RuneCountInString("abc\nde")),
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
	if is.selectBeg != 5 || is.selectEnd != 4 {
		t.Fatalf("Shift+Home 1: sel=%d-%d, want 5-4",
			is.selectBeg, is.selectEnd)
	}
	ctx.fireKeyDown(KeyHome, ModShift)
	is = ctx.state()
	if is.CursorPos != 0 {
		t.Fatalf("Shift+Home 2: cursor=%d, want 0", is.CursorPos)
	}
	if is.selectBeg != 5 || is.selectEnd != 0 {
		t.Fatalf("Shift+Home 2: sel=%d-%d, want 5-0",
			is.selectBeg, is.selectEnd)
	}
}

func TestInputOnKeyDownShiftEndCycleSelection(t *testing.T) {
	ctx := newInputTestMultiline("abc\ndef\nghi", "f705", 5)
	ctx.fireKeyDown(KeyEnd, ModShift)
	is := ctx.state()
	if is.CursorPos != 7 {
		t.Fatalf("Shift+End 1: cursor=%d, want 7", is.CursorPos)
	}
	if is.selectBeg != 5 || is.selectEnd != 7 {
		t.Fatalf("Shift+End 1: sel=%d-%d, want 5-7",
			is.selectBeg, is.selectEnd)
	}
	ctx.fireKeyDown(KeyEnd, ModShift)
	is = ctx.state()
	if is.CursorPos != 11 {
		t.Fatalf("Shift+End 2: cursor=%d, want 11", is.CursorPos)
	}
	if is.selectBeg != 5 || is.selectEnd != 11 {
		t.Fatalf("Shift+End 2: sel=%d-%d, want 5-11",
			is.selectBeg, is.selectEnd)
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
		OnKeyUp: func(ctx EventCtx) {
			called = true
		},
	}

	handler := makeInputOnKeyUp(hcfg)
	layout := &Layout{}
	w := &Window{}
	e := &Event{KeyCode: KeyEnter}

	// Test when not focused - should not call handler
	handler(EventCtx{layout, e, w})
	if called {
		t.Error("Handler should not be called when not focused")
	}

	// Test when focused - should call handler
	w.SetFocus("f123")
	called = false
	handler(EventCtx{layout, e, w})
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
		OnKeyUp: func(ctx EventCtx) {
			called = true
		},
	}

	handler := makeInputOnKeyUp(hcfg)
	layout := &Layout{}
	w := &Window{}
	w.ClearFocus() // Even with focus set to 0
	e := &Event{KeyCode: KeyEnter}

	handler(EventCtx{layout, e, w})
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

	handler(EventCtx{layout, e, w})
}

// --- ReadOnly tests ---

// newInputTestReadOnly mirrors newInputTest but marks the field
// read-only. The field stays focusable (the default): the point of
// ReadOnly is a field that keeps focus and selection while refusing
// edits.
func newInputTestReadOnly(
	text string, focusID string, cursorPos int, mode inputMode,
) *inputTestCtx {
	ctx := &inputTestCtx{}
	ctx.w = newTestWindow()
	ctx.lastText = text
	ctx.w.SetFocus(focusID)
	setInputState(ctx.w, focusID, inputState{CursorPos: cursorPos})
	ctx.layout = generateViewLayout(Input(InputCfg{
		Text:     text,
		ID:       focusID,
		ReadOnly: true,
		Mode:     mode,
		OnTextChanged: func(newText string, _ EventCtx) {
			ctx.lastText = newText
		},
	}), ctx.w)
	return ctx
}

// The state that was previously inexpressible: focusable AND announced
// read-only. Before ReadOnly, AccessStateReadOnly required
// Focusable: false, which also dropped the field from the tab order.
func TestInputReadOnlyIsFocusableAndAnnouncedReadOnly(t *testing.T) {
	w := newTestWindow()
	v := Input(InputCfg{
		ID: "ro_a11y", Text: "x", ReadOnly: true,
	})
	layout := generateViewLayout(v, w)

	if layout.Shape.A11YState != AccessStateReadOnly {
		t.Errorf("A11YState = %d, want AccessStateReadOnly",
			layout.Shape.A11YState)
	}
	// Focusable && ID != "" is what isFocusedTarget requires.
	if !layout.Shape.Focusable || layout.Shape.ID == "" {
		t.Error("read-only field must stay keyboard-reachable")
	}
}

func TestInputReadOnlyBlocksTyping(t *testing.T) {
	ctx := newInputTestReadOnly("hello", "ro1", 5, inputSingleLine)
	ctx.fireChar('!')
	if ctx.lastText != "hello" {
		t.Fatalf("typing mutated read-only field: got %q", ctx.lastText)
	}
}

func TestInputReadOnlyBlocksIMEText(t *testing.T) {
	ctx := newInputTestReadOnly("hello", "ro_ime", 5, inputSingleLine)
	e := &Event{Type: EventChar, CharCode: 'a', IMEText: "日本"}
	ctx.layout.Shape.events.OnChar(EventCtx{&ctx.layout, e, ctx.w})
	if ctx.lastText != "hello" {
		t.Fatalf("IME text mutated read-only field: got %q", ctx.lastText)
	}
}

// TestInputCharIMETextInsertsWholeCommit pins the contract every
// backend now follows for IME commits: one EventChar carries the whole
// committed string in IMEText, with CharCode holding only the first
// rune. The handler must insert IMEText, not string(CharCode) — Android
// relied on a per-rune event loop before this was enforced.
func TestInputCharIMETextInsertsWholeCommit(t *testing.T) {
	ctx := newInputTest("", "ime_commit", 0)
	e := &Event{Type: EventChar, CharCode: 'か', IMEText: "かんじ"}
	ctx.layout.Shape.events.OnChar(EventCtx{&ctx.layout, e, ctx.w})
	if ctx.lastText != "かんじ" {
		t.Fatalf("IME commit inserted %q, want %q",
			ctx.lastText, "かんじ")
	}
	if !e.IsHandled {
		t.Fatal("IME commit event not marked handled")
	}
}

// TestInputCharEmptyIMETextFallsBackToCharCode covers the plain typing
// path: backends that emit a bare key char leave IMEText empty, so the
// handler must fall back to CharCode.
func TestInputCharEmptyIMETextFallsBackToCharCode(t *testing.T) {
	ctx := newInputTest("ab", "ime_fallback", 2)
	e := &Event{Type: EventChar, CharCode: 'c'}
	ctx.layout.Shape.events.OnChar(EventCtx{&ctx.layout, e, ctx.w})
	if ctx.lastText != "abc" {
		t.Fatalf("plain char insert produced %q, want %q",
			ctx.lastText, "abc")
	}
}

func TestInputReadOnlyBlocksDeleteKeys(t *testing.T) {
	for _, key := range []struct {
		name string
		code KeyCode
	}{{"backspace", KeyBackspace}, {"delete", KeyDelete}} {
		ctx := newInputTestReadOnly("hello", "ro_"+key.name, 3,
			inputSingleLine)
		ctx.fireKeyDown(key.code, 0)
		if ctx.lastText != "hello" {
			t.Errorf("%s mutated read-only field: got %q",
				key.name, ctx.lastText)
		}
	}
}

func TestInputReadOnlyBlocksPaste(t *testing.T) {
	ctx := newInputTestReadOnly("hello", "ro2", 5, inputSingleLine)
	ctx.w.SetClipboardGetFn(func() string { return "XX" })
	ctx.fireKeyDown(KeyV, ModCtrl)
	if ctx.lastText != "hello" {
		t.Fatalf("paste mutated read-only field: got %q", ctx.lastText)
	}
}

func TestInputReadOnlyBlocksCut(t *testing.T) {
	var clipboard string
	ctx := newInputTestReadOnly("abcd", "ro3", 2, inputSingleLine)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	setInputState(ctx.w, "ro3", inputState{
		CursorPos: 2, selectBeg: 1, selectEnd: 3,
	})
	ctx.fireKeyDown(KeyX, ModCtrl)
	if ctx.lastText != "abcd" {
		t.Errorf("cut mutated read-only field: got %q", ctx.lastText)
	}
	if clipboard != "" {
		t.Errorf("cut wrote clipboard %q on read-only field", clipboard)
	}
}

// Undo needs real history to be a meaningful test: seed it by editing
// the field while it is still editable, then flip it to read-only and
// confirm Ctrl+Z cannot roll the text back.
func TestInputReadOnlyBlocksUndo(t *testing.T) {
	ctx := newInputTest("hello", "ro4", 5)
	ctx.fireChar('!')
	if ctx.lastText != "hello!" {
		t.Fatalf("setup: insert failed, got %q", ctx.lastText)
	}

	ctx.layout = generateViewLayout(Input(InputCfg{
		Text: ctx.lastText, ID: "ro4", ReadOnly: true,
		OnTextChanged: func(newText string, _ EventCtx) {
			ctx.lastText = newText
		},
	}), ctx.w)

	ctx.fireKeyDown(KeyZ, ModCtrl)
	if ctx.lastText != "hello!" {
		t.Fatalf("undo mutated read-only field: got %q, want %q",
			ctx.lastText, "hello!")
	}
}

func TestInputReadOnlyBlocksMultilineEnter(t *testing.T) {
	ctx := newInputTestReadOnly("ab", "ro5", 1, InputMultiline)
	ctx.fireKeyDown(KeyEnter, 0)
	if ctx.lastText != "ab" {
		t.Fatalf("Enter inserted newline in read-only multiline: got %q",
			ctx.lastText)
	}
}

// Single-line Enter commits without changing text, so it must still
// reach OnEnter on a read-only field (HTML readonly submits too).
func TestInputReadOnlySingleLineEnterStillCommits(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("ro6")
	enterFired := false
	layout := generateViewLayout(Input(InputCfg{
		Text: "hello", ID: "ro6", ReadOnly: true,
		OnEnter: func(ctx EventCtx) {
			enterFired = true
		},
	}), w)
	e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
	if !enterFired {
		t.Error("single-line Enter should still commit when read-only")
	}
}

// Navigation, selection, and copy are the whole reason ReadOnly is not
// just Focusable: false — they must keep working.
func TestInputReadOnlyAllowsNavigation(t *testing.T) {
	ctx := newInputTestReadOnly("hello", "ro7", 0, inputSingleLine)
	ctx.fireKeyDown(KeyRight, 0)
	if got := ctx.state().CursorPos; got != 1 {
		t.Fatalf("CursorPos = %d, want 1: arrows must work read-only", got)
	}
}

func TestInputReadOnlyAllowsSelectAll(t *testing.T) {
	ctx := newInputTestReadOnly("abc", "ro8", 1, inputSingleLine)
	ctx.fireKeyDown(KeyA, ModCtrl)
	is := ctx.state()
	if is.selectBeg != 0 || is.selectEnd != 3 {
		t.Fatalf("select all: got %d-%d, want 0-3",
			is.selectBeg, is.selectEnd)
	}
}

func TestInputReadOnlyAllowsCopy(t *testing.T) {
	var clipboard string
	ctx := newInputTestReadOnly("hello", "ro9", 5, inputSingleLine)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	setInputState(ctx.w, "ro9", inputState{
		CursorPos: 5, selectBeg: 0, selectEnd: 5,
	})
	ctx.fireKeyDown(KeyC, ModCtrl)
	if clipboard != "hello" {
		t.Fatalf("copy: clipboard=%q, want hello", clipboard)
	}
}

// Default (ReadOnly: false) must be unaffected.
func TestInputNotReadOnlyStillEdits(t *testing.T) {
	ctx := newInputTest("hello", "ro10", 5)
	ctx.fireChar('!')
	if ctx.lastText != "hello!" {
		t.Fatalf("got %q, want %q", ctx.lastText, "hello!")
	}
}

// --- ReadOnly commit-path tests ---
//
// These cover the paths that the char/key gates cannot see:
// PostCommitNormalize transforms text and notifies via OnTextChanged
// without going through any text mutator, so gating the two
// dispatchers left it reachable. Both cases below failed before
// normalizeOnCommit/fireTextChanged existed.

func TestInputReadOnlyEnterDoesNotNormalize(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("ro_norm_enter")
	changed := ""
	committed := ""
	layout := generateViewLayout(Input(InputCfg{
		Text: "  hi  ", ID: "ro_norm_enter", ReadOnly: true,
		postCommitNormalize: func(_ string, _ InputCommitReason) string {
			return "NORMALIZED"
		},
		OnTextChanged: func(nt string, ctx EventCtx) {
			changed = nt
		},
		OnTextCommit: func(ct string, _ InputCommitReason, ctx EventCtx) {
			committed = ct
		},
	}), w)

	e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})

	if changed != "" {
		t.Errorf("OnTextChanged fired with %q on a read-only field", changed)
	}
	// Commit still fires — with the original text, not the normalized one.
	if committed != "  hi  " {
		t.Errorf("OnTextCommit got %q, want the unnormalized %q",
			committed, "  hi  ")
	}
}

func TestInputReadOnlyBlurDoesNotNormalize(t *testing.T) {
	w := newTestWindow()
	changed := ""
	committed := ""
	cfg := InputCfg{
		Text: "  hi  ", ID: "ro_norm_blur", ReadOnly: true,
		postCommitNormalize: func(_ string, _ InputCommitReason) string {
			return "NORMALIZED"
		},
		OnTextChanged: func(nt string, ctx EventCtx) {
			changed = nt
		},
		OnTextCommit: func(ct string, _ InputCommitReason, ctx EventCtx) {
			committed = ct
		},
	}

	// Frame 1: focused. AmendLayout records focus in nsInputFocus.
	w.SetFocus("ro_norm_blur")
	l1 := generateViewLayout(Input(cfg), w)
	l1.Shape.events.AmendLayout(EventCtx{&l1, nil, w})

	// Frame 2: focus moved away -> blur commit path runs.
	w.SetFocus("elsewhere")
	l2 := generateViewLayout(Input(cfg), w)
	l2.Shape.events.AmendLayout(EventCtx{&l2, nil, w})

	if changed != "" {
		t.Errorf("OnTextChanged fired with %q on blur of a read-only field",
			changed)
	}
	if committed != "  hi  " {
		t.Errorf("OnTextCommit got %q, want the unnormalized %q",
			committed, "  hi  ")
	}
}

// An editable field must still normalize on both paths — the guard
// must not have disabled the feature outright.
func TestInputEditableEnterStillNormalizes(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("rw_norm_enter")
	changed := ""
	layout := generateViewLayout(Input(InputCfg{
		Text: "  hi  ", ID: "rw_norm_enter", postCommitNormalize: func(_ string, _ InputCommitReason) string {
			return "NORMALIZED"
		},
		OnTextChanged: func(nt string, ctx EventCtx) {
			changed = nt
		},
	}), w)

	e := &Event{Type: EventKeyDown, KeyCode: KeyEnter}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})

	if changed != "NORMALIZED" {
		t.Errorf("editable field: OnTextChanged got %q, want NORMALIZED",
			changed)
	}
}

// renderInnerInputText renders the inner Text shape of an Input and
// returns the concatenation of every emitted command's Text field.
// The rendered string includes the IME preedit when the field is
// composing, so it is the observable signal for #85.
func renderInnerInputText(t *testing.T, w *Window, v View) string {
	t.Helper()
	layout := generateViewLayout(v, w)
	// Input tree is Column -> Row/Column -> Text.
	if len(layout.Children) == 0 || len(layout.Children[0].Children) == 0 {
		t.Fatal("input layout missing inner text child")
	}
	txt := layout.Children[0].Children[0].Shape
	if txt.TC == nil {
		t.Fatal("inner text shape has no TC")
	}
	// Give the shape a real box so it overlaps the clip rect
	// (rectsOverlap is strict-<, so a zero-size shape never draws).
	txt.X, txt.Y, txt.Width, txt.Height = 0, 0, 100, 20
	w.renderers = w.renderers[:0]
	renderText(txt, drawClip{Width: 800, Height: 600}, w)
	var sb strings.Builder
	for i := range w.renderers {
		sb.WriteString(w.renderers[i].Text)
	}
	return sb.String()
}

// TestInputReadOnlyDoesNotRenderIMEPreedit covers #85: a composition
// started on a read-only field must not render its preedit, because
// makeInputOnChar swallows the commit and the preedit would sit there
// until focus change. The editable control case proves the probe
// actually observes preedit rendering (guards against a vacuous pass:
// remove the !tc.TextReadOnly gate in render_text.go and the read-only
// assertion below fails).
func TestInputReadOnlyDoesNotRenderIMEPreedit(t *testing.T) {
	const id = "f-ro-ime"
	const preedit = "ㅎ" // Hangul jamo, mid-composition

	// Control: an editable field with the same IME state DOES insert
	// the preedit into the rendered text.
	we := newTestWindow()
	we.SetFocus(id)
	we.ime.composing = true
	we.ime.compText = preedit
	editable := renderInnerInputText(t, we, Input(InputCfg{
		ID: id, Text: "abc",
	}))
	if !strings.Contains(editable, preedit) {
		t.Fatalf("editable field: preedit not rendered; got %q", editable)
	}

	// Read-only field with identical IME state must not render preedit.
	wr := newTestWindow()
	wr.SetFocus(id)
	wr.ime.composing = true
	wr.ime.compText = preedit
	readonly := renderInnerInputText(t, wr, Input(InputCfg{
		ID: id, ReadOnly: true, Text: "abc",
	}))
	if strings.Contains(readonly, preedit) {
		t.Fatalf("read-only field rendered IME preedit; got %q", readonly)
	}
	// Sanity: the underlying text is still rendered — only the preedit
	// is suppressed, not the field content.
	if !strings.Contains(readonly, "abc") {
		t.Fatalf("read-only field dropped its text; got %q", readonly)
	}
}

// --- SetFocus / selection contract (issue #277) ---

// TestSetFocusReassertSameIDPreservesSelection pins the #277 fix: a
// same-widget SetFocus re-assert must leave in-flight text selections
// alone. Before the fix the unconditional clearInputSelections() in
// setFocusLocked wiped every nsInput selection window-wide on every
// re-assert, even when nothing changed. Real focus *changes* still
// clear them (tested below).
func TestSetFocusReassertSameIDPreservesSelection(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("f900")
	setInputState(w, "f900", inputState{
		CursorPos: 5, selectBeg: 0, selectEnd: 5,
	})
	w.SetFocus("f900")
	if is := getInputState(w, "f900"); is.selectBeg != 0 || is.selectEnd != 5 {
		t.Fatalf("same-id re-assert dropped selection: [%d,%d), want [0,5)",
			is.selectBeg, is.selectEnd)
	}
}

// The #156 pattern: a View function that re-asserts focus on every
// frame (documented as legitimate in window_focus.go). TestRender(nil)
// re-runs the installed generator without clearing the registry
// (UpdateView would), so the second frame observes the state the first
// frame's re-assert must not have wiped.
func TestSetFocusReassertFromViewPreservesSelection(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	view := func(win *Window) View {
		w.SetFocus("f901")
		return Input(InputCfg{ID: "f901", Text: "hello"})
	}
	w.TestRender(view)
	setInputState(w, "f901", inputState{
		CursorPos: 5, selectBeg: 0, selectEnd: 5,
	})
	w.TestRender(nil) // next frame: the View re-asserts focus on f901
	if is := getInputState(w, "f901"); is.selectBeg != 0 || is.selectEnd != 5 {
		t.Fatalf("frame re-assert dropped selection: [%d,%d), want [0,5)",
			is.selectBeg, is.selectEnd)
	}
}

// Real focus changes keep today's contract: every nsInput selection in
// the window is cleared — the widget losing focus, the one gaining it,
// and unrelated fields — not just the newly focused one.
func TestSetFocusRealChangeClearsSelectionsWindowWide(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{ID: "f911", Text: "aaaa"}),
			Input(InputCfg{ID: "f912", Text: "bbbb"}),
			Input(InputCfg{ID: "f913", Text: "cccc"}),
		}})
	})
	w.SetFocus("f911")
	for _, id := range []string{"f911", "f912", "f913"} {
		setInputState(w, id, inputState{
			CursorPos: 4, selectBeg: 0, selectEnd: 4,
		})
	}
	w.SetFocus("f912") // real change: f911 -> f912
	for _, id := range []string{"f911", "f912", "f913"} {
		if is := getInputState(w, id); is.selectBeg != 0 || is.selectEnd != 0 {
			t.Errorf("%s selection survived real focus change: [%d,%d), want [0,0)",
				id, is.selectBeg, is.selectEnd)
		}
	}
}

// Returning to a widget (empty focus -> widget) is a real transition:
// selections that predate focus arrival must not survive it.
func TestSetFocusFromEmptyClearsPreExistingSelections(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("f920")
	setInputState(w, "f920", inputState{
		CursorPos: 4, selectBeg: 0, selectEnd: 4,
	})
	w.SetFocus("") // real change: f920 -> none
	// Seed again while unfocused: a stale selection must not survive
	// the empty -> widget transition below.
	setInputState(w, "f920", inputState{
		CursorPos: 4, selectBeg: 0, selectEnd: 4,
	})
	w.SetFocus("f920")
	if is := getInputState(w, "f920"); is.selectBeg != 0 || is.selectEnd != 0 {
		t.Fatalf("empty->widget transition kept selection: [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
}

// inputSelTestMeasurer shapes plain text into the deterministic
// 10x20-per-byte char-rect layout rtfSelCharLayout builds, so
// inputOnClick's GetClosestOffset maps clicks to runes headlessly.
// The metric methods are rtfSelTestMeasurer's; only the plain-text
// shaper differs.
type inputSelTestMeasurer struct {
	rtfSelTestMeasurer
}

func (inputSelTestMeasurer) LayoutText(
	text string, _ TextStyle, _ float32,
) (glyph.Layout, error) {
	return rtfSelCharLayout(text, nil), nil
}

// rtfSelCharLayout lays one char rect per byte at a fixed 10x20
// pitch. The click coordinates below are derived from it, so they
// stay consistent with the measurer that produced the layout.
const (
	inputSelCharW = 10
	inputSelCharH = 20
)

// Condition-1 pin for #277: clicking an already-focused input must
// collapse its selection to a caret at the click position. The input's
// own click path (inputOnClick) sets selectBeg/selectEnd explicitly
// from the click, so the fix — which leaves same-id re-asserts alone
// — cannot regress it.
func TestInputClickOnFocusedInputCollapsesSelectionToCaret(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.textMeasurer = inputSelTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Input(InputCfg{ID: "f930", Text: "hello world"})
	})
	w.SetFocus("f930")
	setInputState(w, "f930", inputState{
		CursorPos: 5, selectBeg: 0, selectEnd: 5,
	})

	ly, ok := w.layout.FindByID("f930")
	if !ok {
		t.Fatal("no input with effective ID f930")
	}
	if len(ly.Children) == 0 || len(ly.Children[0].Children) == 0 {
		t.Fatal("input layout missing inner text child")
	}
	txt := ly.Children[0].Children[0].Shape
	if txt.TC == nil {
		t.Fatal("inner child is not a text shape")
	}

	// Click mid-rune-8 ('o' of "world"): char rects are inputSelCharW
	// wide and start at the text shape origin, so X + 8*W + W/2 is
	// the rune's center; Y + H/2 is the middle of the glyph line band.
	clickX := txt.X + 8*inputSelCharW + inputSelCharW/2
	clickY := txt.Y + inputSelCharH/2
	down := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: clickX, MouseY: clickY,
	}
	w.EventFn(&down)
	w.settle()
	w.EventFn(&Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: clickX, MouseY: clickY,
	})
	w.settle()

	is := getInputState(w, "f930")
	if is.selectBeg != 8 || is.selectEnd != 8 {
		t.Fatalf("click on focused input left selection [%d,%d), want caret at 8",
			is.selectBeg, is.selectEnd)
	}
	if is.CursorPos != 8 {
		t.Fatalf("cursor = %d, want 8", is.CursorPos)
	}
}

// TestInputDoubleClickSelectsWord drives two real clicks through the
// event pipeline: the second, within doubleClickThresholdMs of the first,
// selects the class run under the caret. Issue #329 made the runs
// go-glyph's — a punctuation run is a word of its own and a script
// change ends a word — so the dotted-identifier cases are the fix.
func TestInputDoubleClickSelectsWord(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		runeAt int
		beg    int
		end    int
	}{
		{"plain word", "hello world", 2, 0, 5},
		{"dotted component", "foo.bar.baz", 5, 4, 7},
		{"separator is its own word", "foo.bar.baz", 3, 3, 4},
		{"whitespace run selects itself", "hello  world", 5, 5, 7},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w, ly := newInputSelWindow(t, InputCfg{
				ID: "fdc", Text: tt.text,
			})
			w.SetFocus("fdc")
			txt := inputTextShape(t, ly)
			x, y := inputRunePoint(txt, tt.runeAt)
			click := func() {
				w.EventFn(&Event{Type: EventMouseDown,
					MouseButton: MouseLeft, MouseX: x, MouseY: y})
				w.settle()
				w.EventFn(&Event{Type: EventMouseUp,
					MouseButton: MouseLeft, MouseX: x, MouseY: y})
				w.settle()
			}
			click()
			click()
			is := getInputState(w, "fdc")
			if is.selectBeg != uint32(tt.beg) ||
				is.selectEnd != uint32(tt.end) {
				t.Fatalf("double-click at rune %d selected [%d,%d), "+
					"want [%d,%d)", tt.runeAt, is.selectBeg,
					is.selectEnd, tt.beg, tt.end)
			}
			if is.CursorPos != tt.end {
				t.Fatalf("cursor = %d, want %d", is.CursorPos, tt.end)
			}
		})
	}
}

// Double-click then press-drag extends the selection by whole words: the
// press within doubleClickThresholdMs of the click builds the drag state
// with wordSelect set, so updateSelection rounds the pointer position to
// word bounds. Dragging from "hello" into "world" must end at [0, 11).
func TestInputDoubleClickDragExtendsByWord(t *testing.T) {
	w, ly := newInputSelWindow(t, InputCfg{ID: "fdc", Text: "hello world"})
	w.SetFocus("fdc")
	txt := inputTextShape(t, ly)
	x0, y0 := inputRunePoint(txt, 2)
	x1, y1 := inputRunePoint(txt, 10)
	click := func() {
		w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: x0, MouseY: y0})
		w.settle()
		w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
			MouseX: x0, MouseY: y0})
		w.settle()
	}
	click()
	click()
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()
	is := getInputState(w, "fdc")
	if is.selectBeg != 0 || is.selectEnd != 11 {
		t.Fatalf("word-drag selected [%d,%d), want [0,11)",
			is.selectBeg, is.selectEnd)
	}
	w.MouseCancel()
	w.settle()
}

// --- Issue #281: capture-loss cancel zeroes the partial drag selection ---

// newInputSelWindow renders a single-line input with the deterministic
// measurer and returns the window plus the layout for ID.
func newInputSelWindow(t *testing.T, cfg InputCfg) (*Window, *Layout) {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	w.textMeasurer = inputSelTestMeasurer{}
	w.TestRender(func(win *Window) View { return Input(cfg) })
	ly, ok := w.layout.FindByID(cfg.ID)
	if !ok {
		t.Fatalf("no input with effective ID %s", cfg.ID)
	}
	return w, ly
}

// inputTextShape walks the input layout to its inner text shape, whose
// origin anchors the deterministic char rects.
func inputTextShape(t *testing.T, ly *Layout) *Shape {
	t.Helper()
	if len(ly.Children) == 0 || len(ly.Children[0].Children) == 0 {
		t.Fatal("input layout missing inner text child")
	}
	txt := ly.Children[0].Children[0].Shape
	if txt.TC == nil {
		t.Fatal("inner child is not a text shape")
	}
	return txt
}

// inputRunePoint returns the window-absolute center of localRune —
// valid for both the click path (shape-relative after callRelative)
// and the MouseLock drag path (window-absolute; the drag subtracts
// the text shape's offset from the input box itself).
func inputRunePoint(txt *Shape, localRune int) (float32, float32) {
	return txt.X + float32(localRune)*inputSelCharW + inputSelCharW/2,
		txt.Y + inputSelCharH/2
}

// TestInputDragCancelZeroesSelection pins issue #281: capture loss
// mid-drag (MouseCancel — no release will ever arrive) must zero the
// partial selection, not leave a stuck highlight.
func TestInputDragCancelZeroesSelection(t *testing.T) {
	w, ly := newInputSelWindow(t, InputCfg{ID: "f281", Text: "hello world"})
	txt := inputTextShape(t, ly)

	x0, y0 := inputRunePoint(txt, 2)
	x1, y1 := inputRunePoint(txt, 8)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	if !w.mouseIsLocked() {
		t.Fatal("press on the input did not lock the mouse")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()
	is := getInputState(w, "f281")
	if is.selectBeg != 2 || is.selectEnd != 8 {
		t.Fatalf("drag selected [%d,%d), want [2,8)",
			is.selectBeg, is.selectEnd)
	}

	w.MouseCancel()
	w.settle()
	if w.mouseIsLocked() {
		t.Error("mouse still locked after MouseCancel")
	}
	is = getInputState(w, "f281")
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("selection after cancel = [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
}

// TestInputDragCancelScopedToOwnState asserts the Input Cancel hook
// zeroes only the dragged input's nsInput entry: another widget's
// state written after the drag started survives the cancel. (A state
// written before the press would be wiped by the focus change.)
func TestInputDragCancelScopedToOwnState(t *testing.T) {
	w, ly := newInputSelWindow(t, InputCfg{ID: "f281", Text: "hello world"})
	txt := inputTextShape(t, ly)

	x0, y0 := inputRunePoint(txt, 2)
	x1, y1 := inputRunePoint(txt, 8)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()

	setInputState(w, "other", inputState{CursorPos: 3, selectBeg: 1, selectEnd: 3})
	w.MouseCancel()
	w.settle()
	if is := getInputState(w, "f281"); is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("dragged input selection = [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
	if is := getInputState(w, "other"); is.selectBeg != 1 || is.selectEnd != 3 {
		t.Errorf("unrelated widget selection = [%d,%d), want [1,3)",
			is.selectBeg, is.selectEnd)
	}
}

// TestInputReleaseOutsideCommitsSelection pins the asymmetry: a
// normal MouseUp — even at a point far outside the fixed-size input —
// COMMITS the drag selection. The lock delivers the release anywhere;
// only capture loss goes through the Cancel hook.
func TestInputReleaseOutsideCommitsSelection(t *testing.T) {
	w, ly := newInputSelWindow(t, InputCfg{
		ID: "f281", Text: "hello world",
		Sizing: FixedFixed, Width: 200, Height: 30,
	})
	txt := inputTextShape(t, ly)

	x0, y0 := inputRunePoint(txt, 2)
	x1, y1 := inputRunePoint(txt, 8)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()
	if is := getInputState(w, "f281"); is.selectBeg != 2 || is.selectEnd != 8 {
		t.Fatalf("drag selected [%d,%d), want [2,8)",
			is.selectBeg, is.selectEnd)
	}

	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: 700, MouseY: 700})
	w.settle()
	if w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	is := getInputState(w, "f281")
	if is.selectBeg != 2 || is.selectEnd != 8 {
		t.Errorf("selection after outside release = [%d,%d), want [2,8)",
			is.selectBeg, is.selectEnd)
	}
}

// TestInputDragCancelCollapsedSelectionHarmless asserts the guard:
// a cancel when no selection is in progress (the press only collapsed
// to a caret) leaves the state clean — no corruption, no stuck lock.
func TestInputDragCancelCollapsedSelectionHarmless(t *testing.T) {
	w, ly := newInputSelWindow(t, InputCfg{ID: "f281", Text: "hello world"})
	txt := inputTextShape(t, ly)

	x, y := inputRunePoint(txt, 2)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y})
	w.settle()
	if !w.mouseIsLocked() {
		t.Fatal("press on the input did not lock the mouse")
	}
	if is := getInputState(w, "f281"); is.selectBeg != 2 || is.selectEnd != 2 {
		t.Fatalf("press left selection [%d,%d), want caret at 2",
			is.selectBeg, is.selectEnd)
	}

	w.MouseCancel()
	w.settle()
	if w.mouseIsLocked() {
		t.Error("mouse still locked after MouseCancel")
	}
	is := getInputState(w, "f281")
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("selection after cancel = [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
	if is.CursorPos != 2 {
		t.Errorf("cursor = %d, want 2", is.CursorPos)
	}
}

// inputTextShapeY renders one Input and reports the arranged Y of its
// text shape, which is what the optical correction moves.
func inputTextShapeY(t *testing.T, cfg InputCfg) float32 {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	cfg.ID = "f"
	w.TestRender(func(*Window) View { return Input(cfg) })
	field, ok := w.layout.FindByID("f")
	if !ok {
		t.Fatal("no input in the rendered window")
	}
	// The text sits under the inner row that carries the hook, so walk
	// to the first text descendant rather than assuming a depth.
	var find func(l *Layout) *Shape
	find = func(l *Layout) *Shape {
		for i := range l.Children {
			c := &l.Children[i]
			if c.Shape != nil && c.Shape.shapeType == shapeText {
				return c.Shape
			}
			if got := find(c); got != nil {
				return got
			}
		}
		return nil
	}
	txt := find(field)
	if txt == nil {
		t.Fatal("no text shape under the input")
	}
	return txt.Y
}

// The correction must be content-free where it is applied at all: an
// offset that followed the string would move the baseline as the user
// types, which is the failure that ruled out correcting Input in general
// (issue #346).
func TestInputOpticalCenterDoesNotFollowContent(t *testing.T) {
	base := InputCfg{Text: "0", opticalDigitCenter: true}
	first := inputTextShapeY(t, base)

	for _, txt := range []string{"1", "0000", "08/16/2026", ""} {
		cfg := base
		cfg.Text = txt
		if got := inputTextShapeY(t, cfg); got != first {
			t.Errorf("text %q moved the baseline: got %v, want %v",
				txt, got, first)
		}
	}
}

// The opt-in is the whole safety argument: a plain Input keeps metric
// centring, and only a caller that can name the alphabet turns the
// correction on.
func TestInputOpticalCenterIsOptIn(t *testing.T) {
	plain := inputTextShapeY(t, InputCfg{Text: "128"})
	opted := inputTextShapeY(t, InputCfg{Text: "128", opticalDigitCenter: true})
	if opted <= plain {
		t.Errorf("opted-in text y = %v, want below plain %v", opted, plain)
	}
}
