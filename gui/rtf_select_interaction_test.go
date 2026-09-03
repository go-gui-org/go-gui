package gui

// rtf_select_interaction_test.go drives the standalone selectable RTF
// widget (view_rtf_select.go) through the real pipeline: a focusable
// RTF rendered as the whole window (or a panel), with a stub text
// measurer that shapes every block into a deterministic glyph layout
// (10px per byte, per-run Items, real word/cursor log attrs) so clicks
// map to exact runes. The pure-helper guards live in the sibling file
// rtf_select_test.go.
//
// Coordinate spaces matter here and are asserted deliberately:
// OnClick arrives shape-local (callRelative), while MouseLock drag
// callbacks arrive window-absolute — the drag path subtracts the
// block's ShapeX/ShapeY plus the scroll delta itself.

import (
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// --- Deterministic glyph layout stub ---

// rtfSelLogAttrs marks every byte as a cursor position, space runs as
// word boundaries, line breaks as IsLineBreak, and appends the
// end-of-text position — mirroring glyph's shaping path so MoveCursor*,
// GetCursorPos and GetWordAtIndex behave like a real layout.
func rtfSelLogAttrs(text string) (map[int]int, []glyph.LogAttr) {
	byIndex := map[int]int{}
	var attrs []glyph.LogAttr
	add := func(i int, wordStart, wordEnd, cursor, lineBreak bool) {
		byIndex[i] = len(attrs)
		attrs = append(attrs, glyph.LogAttr{
			IsWordStart:      wordStart,
			IsWordEnd:        wordEnd,
			IsCursorPosition: cursor,
			IsLineBreak:      lineBreak,
		})
	}
	isSpace := func(i int) bool { return text[i] == ' ' || text[i] == '\t' }
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\n':
			add(i, false, i > 0 && !isSpace(i-1), true, true)
		default:
			prevSpace := i == 0 || isSpace(i-1)
			space := isSpace(i)
			add(i, !space && prevSpace, space && !prevSpace, true, false)
		}
	}
	if len(text) > 0 && !isSpace(len(text)-1) {
		add(len(text), false, true, true, false)
	} else if len(text) > 0 {
		add(len(text), false, false, true, false)
	}
	return byIndex, attrs
}

// rtfSelItems emits one Item per run, the granularity glyph's shaping
// produces, so link/tooltip hit tests (rtfOnClick, rtfMouseMove) only
// answer for the run under the cursor. The baseline sits at Y=16 with
// Ascent 12 / Descent 4, so the hit rect spans [4,20] — inside the
// char-rect band [0,20) that GetClosestOffset maps.
func rtfSelItems(runs []glyph.StyleRun) []glyph.Item {
	items := make([]glyph.Item, 0, len(runs))
	offset := 0
	for _, r := range runs {
		items = append(items, glyph.Item{
			X: float64(offset) * 10, Y: 16,
			Width:  float64(len(r.Text)) * 10,
			Ascent: 12, Descent: 4,
			StartIndex: offset,
			RunText:    r.Text,
		})
		offset += len(r.Text)
	}
	return items
}

// rtfSelCharLayout shapes text into a deterministic layout: one 10x20
// char rect per byte (newlines excluded), one Line per '\n'-separated
// segment stacked 20px apart, and per-run Items. GetClosestOffset,
// MoveCursor* and GetWordAtIndex all work off these structures, so the
// layout is a complete stand-in for a shaped one.
func rtfSelCharLayout(text string, runs []glyph.StyleRun) glyph.Layout {
	n := len(text)
	charRects := make([]glyph.CharRect, 0, n)
	charRectByIndex := make(map[int]int, n)
	var lines []glyph.Line
	lineIdx := -1
	lineStart := 0
	for i := 0; i <= n; i++ {
		if i == n || text[i] == '\n' {
			lineIdx++
			length := i - lineStart
			lines = append(lines, glyph.Line{
				StartIndex: lineStart,
				Length:     length,
				Rect: glyph.Rect{
					X: 0, Y: float32(lineIdx) * 20,
					Width: float32(length) * 10, Height: 20,
				},
			})
			lineStart = i + 1
			continue
		}
		charRectByIndex[i] = len(charRects)
		charRects = append(charRects, glyph.CharRect{
			Rect: glyph.Rect{
				X:     float32(i-lineStart) * 10,
				Y:     float32(lineIdx+1) * 20,
				Width: 10, Height: 20,
			},
			Index: i,
		})
	}
	var width float32
	for _, ln := range lines {
		if ln.Rect.Width > width {
			width = ln.Rect.Width
		}
	}
	byIndex, attrs := rtfSelLogAttrs(text)
	return glyph.Layout{
		Text:            text,
		CharRects:       charRects,
		CharRectByIndex: charRectByIndex,
		LogAttrByIndex:  byIndex,
		LogAttrs:        attrs,
		Lines:           lines,
		Items:           rtfSelItems(runs),
		Width:           width,
		Height:          float32(len(lines)) * 20,
	}
}

// rtfSelTestMeasurer implements TextMeasurer plus the optional
// LayoutRichText capability by shaping every rich text block into the
// deterministic char layout above. With nil backends the pipeline would
// leave rTFLayout empty and every click would land on rune 0.
type rtfSelTestMeasurer struct{}

func (rtfSelTestMeasurer) TextWidth(text string, style TextStyle) float32 {
	return float32(len(text)) * style.Size * 0.5
}

func (rtfSelTestMeasurer) TextHeight(_ string, style TextStyle) float32 {
	return style.Size * 1.2
}

func (rtfSelTestMeasurer) FontAscent(style TextStyle) float32 {
	return style.Size * 0.8
}

func (rtfSelTestMeasurer) FontHeight(style TextStyle) float32 {
	return style.Size * 1.2
}

func (rtfSelTestMeasurer) LayoutText(
	_ string, style TextStyle, _ float32,
) (glyph.Layout, error) {
	return glyph.Layout{Height: style.Size * 1.2}, nil
}

// LayoutRichText concatenates the run texts — the same flat text the
// shape carries in rTFFlatText — and shapes it.
func (rtfSelTestMeasurer) LayoutRichText(
	rt glyph.RichText, _ glyph.TextConfig,
) (glyph.Layout, error) {
	var b strings.Builder
	for _, r := range rt.Runs {
		b.WriteString(r.Text)
	}
	return rtfSelCharLayout(b.String(), rt.Runs), nil
}

// --- Harness ---

// rtfHelloWorld is the workhorse text: 11 runes, no links, so clicks
// never trip the link path.
func rtfHelloWorld() RichText {
	return RichText{Runs: []RichTextRun{
		{Text: "hello world", Style: TextStyle{Size: 14}},
	}}
}

// rtfTwoRuns splits the line into a plain run and a linked one, so a
// test can click link and non-link text in the same block.
func rtfTwoRuns() RichText {
	return RichText{Runs: []RichTextRun{
		{Text: "go to ", Style: TextStyle{Size: 14}},
		{Text: "page", Link: "https://safe.example.com",
			Style: TextStyle{Size: 14}},
	}}
}

type rtfSelectHarness struct {
	w      *Window
	id     string // effective RTF ID: "rtf" flat, "panel:rtf" nested
	rt     RichText
	mode   textMode
	nested bool
}

func newRtfSelectHarness(t *testing.T, rt RichText) *rtfSelectHarness {
	t.Helper()
	h := &rtfSelectHarness{
		w:  NewTestWindow(WindowCfg{Width: 800, Height: 800}),
		id: "rtf",
		rt: rt,
	}
	h.w.textMeasurer = rtfSelTestMeasurer{}
	h.render()
	return h
}

func newRtfSelectHarnessNested(t *testing.T, rt RichText) *rtfSelectHarness {
	t.Helper()
	h := &rtfSelectHarness{
		w:      NewTestWindow(WindowCfg{Width: 800, Height: 800}),
		id:     "panel:rtf",
		rt:     rt,
		nested: true,
	}
	h.w.textMeasurer = rtfSelTestMeasurer{}
	h.render()
	return h
}

func (h *rtfSelectHarness) render() {
	h.w.TestRender(func(win *Window) View {
		rtf := RTF(RTFCfg{
			ID: "rtf", Focusable: true, Mode: h.mode, RichText: h.rt,
		})
		if h.nested {
			return Column(ContainerCfg{
				ID: "panel", Sizing: FillFill,
				Content: []View{rtf},
			})
		}
		return rtf
	})
}

func (h *rtfSelectHarness) shape(t *testing.T) *Layout {
	t.Helper()
	ly, ok := h.w.layout.FindByID(h.id)
	if !ok {
		t.Fatalf("no RTF shape with effective ID %q", h.id)
	}
	return ly
}

func (h *rtfSelectHarness) selState() inputState {
	return StateReadOr(h.w, nsInput, h.id, inputState{})
}

func (h *rtfSelectHarness) press(x, y float32) Event {
	return h.pressButton(x, y, MouseLeft)
}

func (h *rtfSelectHarness) pressButton(x, y float32, btn MouseButton) Event {
	e := Event{
		Type: EventMouseDown, MouseButton: btn,
		MouseX: x, MouseY: y,
	}
	h.w.EventFn(&e)
	h.w.settle()
	return e
}

func (h *rtfSelectHarness) move(x, y float32) Event {
	e := Event{
		Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x, MouseY: y,
	}
	h.w.EventFn(&e)
	h.w.settle()
	return e
}

func (h *rtfSelectHarness) release(x, y float32) Event {
	e := Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	h.w.EventFn(&e)
	h.w.settle()
	return e
}

// key dispatches one key down/up pair to the RTF via TestKey, which
// re-asserts focus on the same id. Same-id re-asserts no longer clear
// selections (issue #277), so chained calls build on the selection the
// preceding click or key established. The original helper dispatched
// raw EventFn pairs to dodge the old unconditional clear that
// setFocusLocked ran on every re-assert; that workaround is obsolete.
func (h *rtfSelectHarness) key(t *testing.T, k KeyCode, mods Modifier) {
	t.Helper()
	if err := h.w.TestKey(h.id, k, mods); err != nil {
		t.Fatalf("TestKey(%s): %v", keyName(k), err)
	}
}

// rtfRunePoint returns the window coordinate of the middle of localRune
// on the first line of ly: char rects start at the shape origin, so
// byte*10+5 is the rune's center and y=10 the line's middle.
func rtfRunePoint(ly *Layout, localRune int) (float32, float32) {
	byteIdx := runeToByteIndex(ly.Shape.TC.rTFFlatText, localRune)
	return ly.Shape.X + float32(byteIdx)*10 + 5, ly.Shape.Y + 10
}

func expectRtfSel(t *testing.T, h *rtfSelectHarness, beg, end uint32) {
	t.Helper()
	is := h.selState()
	if is.selectBeg != beg || is.selectEnd != end {
		t.Errorf("selection = [%d,%d), want [%d,%d)",
			is.selectBeg, is.selectEnd, beg, end)
	}
}

func expectRtfCursor(t *testing.T, h *rtfSelectHarness, pos int) {
	t.Helper()
	if is := h.selState(); is.CursorPos != pos {
		t.Errorf("cursor = %d, want %d", is.CursorPos, pos)
	}
}

// rtfURLCapturePlatform stubs the native platform to record OpenURI
// calls; everything else stays on the noop defaults.
type rtfURLCapturePlatform struct {
	noopNativePlatform
	opened string
}

func (p *rtfURLCapturePlatform) OpenURI(uri string) error {
	p.opened = uri
	return nil
}

// --- Click and word selection ---

// TestRtfSelectClickPlacesCursorAtRune asserts a single click collapses
// the selection to the clicked rune, focuses the RTF and consumes the
// event.
func TestRtfSelectClickPlacesCursorAtRune(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x, y := rtfRunePoint(h.shape(t), 6)
	e := h.press(x, y)
	h.release(x, y)

	expectRtfSel(t, h, 6, 6)
	if !e.IsHandled {
		t.Error("click on the RTF was not consumed")
	}
	if got := h.w.FocusID(); got != "rtf" {
		t.Errorf("focus = %q, want %q", got, "rtf")
	}
}

// TestRtfSelectNestedClickSelectsAndFocuses asserts selection state and
// focus both key on the effective ID under an ID-bearing ancestor.
func TestRtfSelectNestedClickSelectsAndFocuses(t *testing.T) {
	h := newRtfSelectHarnessNested(t, rtfHelloWorld())
	if dups := h.w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs in window: %q", dups)
	}

	x, y := rtfRunePoint(h.shape(t), 2)
	h.press(x, y)
	h.release(x, y)

	expectRtfSel(t, h, 2, 2)
	if got := h.w.FocusID(); got != "panel:rtf" {
		t.Errorf("focus = %q, want %q (effective ID)", got, "panel:rtf")
	}
}

// TestRtfSelectDoubleClickSelectsWord asserts two rapid clicks expand
// the selection to the word under the cursor.
func TestRtfSelectDoubleClickSelectsWord(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	// Local rune 9 ('r') sits inside "world" [6,11).
	x, y := rtfRunePoint(h.shape(t), 9)
	h.press(x, y)
	h.release(x, y)
	if is := h.selState(); is.selectBeg != is.selectEnd {
		t.Fatalf("first click selected [%d,%d), want a cursor",
			is.selectBeg, is.selectEnd)
	}

	h.press(x, y)
	h.release(x, y)

	expectRtfSel(t, h, 6, 11)
	expectRtfCursor(t, h, 11)
	if is := h.selState(); is.LastClickTime == 0 {
		t.Error("LastClickTime not recorded")
	}
}

// TestRtfSelectDoubleClickStaleThresholdIsolated splits the double
// click across more than doubleClickThresholdMs so the second click is
// a fresh cursor placement. Simulated by backdating LastClickTime — the
// handler reads wall-clock time, so a real sleep would be flaky.
func TestRtfSelectDoubleClickStaleThresholdIsolated(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	StateMap[string, inputState](h.w, nsInput, capMany).Set("rtf",
		inputState{
			CursorPos: 2, selectBeg: 2, selectEnd: 2,
			LastClickTime: time.Now().UnixMilli() -
				doubleClickThresholdMs - 1,
		})

	x, y := rtfRunePoint(h.shape(t), 10)
	h.press(x, y)
	h.release(x, y)

	expectRtfSel(t, h, 10, 10)
}

// --- Drag selection ---

// TestRtfSelectDragSelects presses at rune 2 and drags to rune 8: the
// MouseLock drag path receives window-absolute coordinates, so the
// runes asserted here prove the handler subtracts the shape origin.
func TestRtfSelectDragSelects(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x0, y0 := rtfRunePoint(h.shape(t), 2)
	h.press(x0, y0)
	if !h.w.mouseIsLocked() {
		t.Fatal("press on the RTF did not lock the mouse")
	}

	x1, y1 := rtfRunePoint(h.shape(t), 8)
	h.move(x1, y1)
	expectRtfSel(t, h, 2, 8)

	h.release(x1, y1)
	if h.w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	expectRtfSel(t, h, 2, 8)
}

// TestRtfSelectDragBackwardsInvertsSelection drags from a later rune to
// an earlier one: the raw state keeps the anchor at the press rune and
// SelEnd at the cursor, so SelBeg > SelEnd.
func TestRtfSelectDragBackwardsInvertsSelection(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x0, y0 := rtfRunePoint(h.shape(t), 8)
	h.press(x0, y0)
	x1, y1 := rtfRunePoint(h.shape(t), 2)
	h.move(x1, y1)
	h.release(x1, y1)

	expectRtfSel(t, h, 8, 2)
}

// TestRtfSelectDoubleClickDragExtendsWord asserts double-click-then-
// drag extends the word selection: a third press-and-hold within the
// double-click window re-arms the word anchor, and dragging right ends
// at the word under the cursor.
func TestRtfSelectDoubleClickDragExtendsWord(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	// Two clicks select "hello" [0,5); a third press-and-hold starts
	// the word-extending drag (each release unlocks the mouse, so the
	// drag only runs while the third press is held).
	x, y := rtfRunePoint(h.shape(t), 1)
	h.press(x, y)
	h.release(x, y)
	h.press(x, y)
	h.release(x, y)
	expectRtfSel(t, h, 0, 5)

	h.press(x, y)
	if !h.w.mouseIsLocked() {
		t.Fatal("third press did not lock the mouse")
	}
	x1, y1 := rtfRunePoint(h.shape(t), 8)
	h.move(x1, y1)
	expectRtfSel(t, h, 0, 11)
	h.release(x1, y1)
	expectRtfSel(t, h, 0, 11)
}

// TestRtfSelectDoubleClickDragBackwardsInverts drags from a later word
// back to an earlier one: the anchor word stays at the double-click
// site and the selection inverts around it.
func TestRtfSelectDoubleClickDragBackwardsInverts(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	// Two clicks select "world" [6,11); the third press-and-hold drags
	// back into "hello", inverting the selection.
	x, y := rtfRunePoint(h.shape(t), 8)
	h.press(x, y)
	h.release(x, y)
	h.press(x, y)
	h.release(x, y)
	expectRtfSel(t, h, 6, 11)

	h.press(x, y)
	x1, y1 := rtfRunePoint(h.shape(t), 2)
	h.move(x1, y1)
	expectRtfSel(t, h, 11, 0)
	h.release(x1, y1)
	expectRtfSel(t, h, 11, 0)
}

// TestRtfSelectRightClickLeavesSelectionUntouched asserts the right
// button (reserved for the link context menu) never moves the
// selection.
func TestRtfSelectRightClickLeavesSelectionUntouched(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x, y := rtfRunePoint(h.shape(t), 2)
	h.press(x, y)
	h.release(x, y)

	xr, yr := rtfRunePoint(h.shape(t), 9)
	h.pressButton(xr, yr, MouseRight)
	h.release(xr, yr)

	expectRtfSel(t, h, 2, 2)
}

// TestRtfSelectDragCancelZeroesSelection pins issue #281: capture
// loss mid-drag (MouseCancel — the platform revoked the lock, no
// release will come) must zero the partial selection instead of
// leaving it stuck. The Cancel hook is scoped to the dragged RTF's
// own nsInput key.
func TestRtfSelectDragCancelZeroesSelection(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x0, y0 := rtfRunePoint(h.shape(t), 2)
	h.press(x0, y0)
	x1, y1 := rtfRunePoint(h.shape(t), 8)
	h.move(x1, y1)
	expectRtfSel(t, h, 2, 8)

	h.w.MouseCancel()
	h.w.settle()
	if h.w.mouseIsLocked() {
		t.Error("mouse still locked after MouseCancel")
	}
	expectRtfSel(t, h, 0, 0)
}

// TestRtfSelectDragCancelScopedToOwnState asserts the Cancel hook
// zeroes only the dragged RTF's selection: a second widget's nsInput
// entry written after the drag started survives the cancel. (A
// selection present before the press would already have been cleared
// by the RTF taking focus — a real focus change clears selections
// window-wide by design, so the unrelated entry must be planted after
// focus is established.)
func TestRtfSelectDragCancelScopedToOwnState(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x0, y0 := rtfRunePoint(h.shape(t), 2)
	h.press(x0, y0)
	x1, y1 := rtfRunePoint(h.shape(t), 8)
	h.move(x1, y1)
	expectRtfSel(t, h, 2, 8)

	setInputState(h.w, "other", inputState{CursorPos: 3, selectBeg: 1, selectEnd: 3})
	h.w.MouseCancel()
	h.w.settle()
	expectRtfSel(t, h, 0, 0)
	if is := getInputState(h.w, "other"); is.selectBeg != 1 || is.selectEnd != 3 {
		t.Errorf("unrelated widget selection = [%d,%d), want [1,3)",
			is.selectBeg, is.selectEnd)
	}
}

// TestRtfSelectReleaseOutsideCommitsSelection pins the asymmetry that
// makes cancel-zeros correct: a normal MouseUp — even at a point far
// outside the widget — COMMITS the drag selection. The mouse lock
// delivers the release regardless of position; only capture loss goes
// through the Cancel hook.
func TestRtfSelectReleaseOutsideCommitsSelection(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x0, y0 := rtfRunePoint(h.shape(t), 2)
	h.press(x0, y0)
	x1, y1 := rtfRunePoint(h.shape(t), 8)
	h.move(x1, y1)
	expectRtfSel(t, h, 2, 8)

	h.release(700, 700) // outside the widget: commits
	if h.w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	expectRtfSel(t, h, 2, 8)
}

// TestRtfSelectDragCancelCollapsedSelectionHarmless asserts the
// guard: a cancel when no selection is in progress (the press only
// collapsed to a caret) leaves the state clean — no corruption, no
// stuck lock.
func TestRtfSelectDragCancelCollapsedSelectionHarmless(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())

	x, y := rtfRunePoint(h.shape(t), 2)
	h.press(x, y)
	expectRtfSel(t, h, 2, 2)

	h.w.MouseCancel()
	h.w.settle()
	if h.w.mouseIsLocked() {
		t.Error("mouse still locked after MouseCancel")
	}
	expectRtfSel(t, h, 0, 0)
	expectRtfCursor(t, h, 2)
}

// --- Keyboard navigation ---

// TestRtfSelectKeyNavArrows walks the single-line key map: plain
// arrows, Ctrl/Alt word movement, Home/End, and selection collapse on
// an unmodified arrow.
func TestRtfSelectKeyNavArrows(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 4)
	h.press(x, y)
	h.release(x, y)
	expectRtfCursor(t, h, 4)

	h.key(t, KeyRight, ModNone)
	expectRtfCursor(t, h, 5)
	h.key(t, KeyRight, ModNone)
	expectRtfCursor(t, h, 6)
	h.key(t, KeyLeft, ModNone)
	expectRtfCursor(t, h, 5)

	// Word movement: from ' ' (5) Ctrl+Right lands on 'w' (6);
	// Ctrl+Left from there lands on 'h' (0).
	h.key(t, KeyRight, ModCtrl)
	expectRtfCursor(t, h, 6)
	h.key(t, KeyLeft, ModCtrl)
	expectRtfCursor(t, h, 0)

	// Alt is a word modifier too, matching the Input/Text handlers.
	h.key(t, KeyRight, ModAlt)
	expectRtfCursor(t, h, 6)

	h.key(t, KeyHome, ModNone)
	expectRtfCursor(t, h, 0)
	h.key(t, KeyEnd, ModNone)
	expectRtfCursor(t, h, 11)
	h.key(t, KeyHome, ModNone)
	expectRtfCursor(t, h, 0)

	// An unmodified arrow collapses the selection to its end.
	h.key(t, KeyRight, ModShift)
	expectRtfSel(t, h, 0, 1)
	h.key(t, KeyRight, ModNone)
	expectRtfSel(t, h, 0, 0)
	expectRtfCursor(t, h, 1)
}

// TestRtfSelectKeyShiftExtendsSelection walks the Shift+arrow
// extension rules, including the inversion when the cursor moves back
// past the anchor.
func TestRtfSelectKeyShiftExtendsSelection(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 4)
	h.press(x, y)
	h.release(x, y)

	h.key(t, KeyRight, ModShift)
	expectRtfSel(t, h, 4, 5)
	h.key(t, KeyRight, ModShift)
	expectRtfSel(t, h, 4, 6)
	h.key(t, KeyLeft, ModShift)
	expectRtfSel(t, h, 4, 5)
	h.key(t, KeyLeft, ModShift)
	expectRtfSel(t, h, 4, 4)
	h.key(t, KeyLeft, ModShift)
	expectRtfSel(t, h, 4, 3) // inverted: anchor stays at 4
}

// TestRtfSelectEscapeClearsSelection asserts Escape resets the
// selection without moving the cursor, and travels on (not consumed).
func TestRtfSelectEscapeClearsSelection(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 4)
	h.press(x, y)
	h.release(x, y)

	if err := h.w.TestKey("rtf", KeyRight, ModShift); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	expectRtfSel(t, h, 4, 5)

	ly := h.shape(t)
	e := &Event{Type: EventKeyDown, KeyCode: KeyEscape}
	rtfSelectOnKeyDown(EventCtx{Layout: ly, Event: e, Window: h.w})
	if e.IsHandled {
		t.Error("Escape was consumed")
	}
	expectRtfSel(t, h, 0, 0)
	expectRtfCursor(t, h, 5)
}

// TestRtfSelectCtrlASelectsAllAndCtrlCCopies asserts select-all and
// copy through the real key dispatch, with the clipboard captured.
func TestRtfSelectCtrlASelectsAllAndCtrlCCopies(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 3)
	h.press(x, y)
	h.release(x, y)

	h.key(t, KeyA, ModCtrl)
	expectRtfSel(t, h, 0, 11)

	var got string
	h.w.SetClipboardFn(func(s string) { got = s })
	h.key(t, KeyC, ModCtrl)
	if got != "hello world" {
		t.Errorf("clipboard = %q, want %q", got, "hello world")
	}
}

// TestRtfSelectPlainCNotConsumed asserts bare C does nothing: no
// consume, no clipboard write.
func TestRtfSelectPlainCNotConsumed(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 2)
	h.press(x, y)
	h.release(x, y)

	var got string
	h.w.SetClipboardFn(func(s string) { got = s })
	ly := h.shape(t)
	e := &Event{Type: EventKeyDown, KeyCode: KeyC}
	rtfSelectOnKeyDown(EventCtx{Layout: ly, Event: e, Window: h.w})
	if e.IsHandled {
		t.Error("bare C consumed the key event")
	}
	if got != "" {
		t.Errorf("clipboard = %q, want empty for unmodified C", got)
	}
}

// TestRtfSelectKeyDownUnfocusedIgnored asserts keyboard dispatch to an
// unfocused RTF changes nothing: keydown targets the focused shape, so
// the handler is not even invoked.
func TestRtfSelectKeyDownUnfocusedIgnored(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	h.w.ClearFocus()

	e := Event{Type: EventKeyDown, KeyCode: KeyA, Modifiers: ModCtrl}
	h.w.EventFn(&e)
	h.w.settle()

	if is := h.selState(); is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("unfocused Ctrl+A set selection to [%d,%d)",
			is.selectBeg, is.selectEnd)
	}
}

// TestRtfSelectEmptyTextCtrlAInert asserts select-all on an empty RTF
// is harmless: the selection stays [0,0].
func TestRtfSelectEmptyTextCtrlAInert(t *testing.T) {
	h := newRtfSelectHarness(t, RichText{})
	if err := h.w.TestKey("rtf", KeyA, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	expectRtfSel(t, h, 0, 0)
}

// TestRtfSelectWrapModeKeyUpDownMovesBetweenLines asserts vertical
// navigation on a wrapped RTF: KeyUp/KeyDown move to the neighbouring
// line at the cursor's preferred x (only TextModeWrap answers them).
func TestRtfSelectWrapModeKeyUpDownMovesBetweenLines(t *testing.T) {
	h := newRtfSelectHarness(t, RichText{Runs: []RichTextRun{
		{Text: "alpha beta\ngamma delta", Style: TextStyle{Size: 14}},
	}})
	h.mode = TextModeWrap
	h.w.TestRender(nil)

	// Click 'a' of "gamma delta" (byte 12, line 2 starts at byte 11).
	ly := h.shape(t)
	h.press(ly.Shape.X+15, ly.Shape.Y+30)
	h.release(ly.Shape.X+15, ly.Shape.Y+30)
	expectRtfCursor(t, h, 12)

	// Up lands on line 1 at x=10: 'a' of "alpha" (byte 0).
	if err := h.w.TestKey("rtf", KeyUp, ModNone); err != nil {
		t.Fatalf("TestKey(KeyUp): %v", err)
	}
	expectRtfCursor(t, h, 0)

	// Down returns to line 2 at x=0: 'g' of "gamma" (byte 11).
	if err := h.w.TestKey("rtf", KeyDown, ModNone); err != nil {
		t.Fatalf("TestKey(KeyDown): %v", err)
	}
	expectRtfCursor(t, h, 11)
}

// --- AmendLayout stamping ---

// TestRtfSelectAmendStampsSelectionOnFrame asserts the amend pass
// copies the selection into the shape's TextSelBeg/End on the next
// frame, which is what the renderer highlights.
func TestRtfSelectAmendStampsSelectionOnFrame(t *testing.T) {
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 2)
	h.press(x, y)
	h.release(x, y)

	h.w.TestRender(nil)
	ly := h.shape(t)
	if ly.Shape.TC.textSelBeg != 2 || ly.Shape.TC.textSelEnd != 2 {
		t.Errorf("highlight = [%d,%d), want [2,2]",
			ly.Shape.TC.textSelBeg, ly.Shape.TC.textSelEnd)
	}
}

// --- Drag with a scroll container ---

// TestRtfSelectScrollDragEdgeScrolls asserts the scroll-aware drag
// path: dragging outside the scroll viewport arms the edge-scroll
// animation, its callback scrolls and re-maps the cursor through the
// scroll delta, and moving back inside disarms it. The mouse lock
// receives window-absolute coordinates, so the rune assertions prove
// the ShapeX/ShapeY/scrollDelta translation stays correct while the
// container scrolls under the cursor.
func TestRtfSelectScrollDragEdgeScrolls(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = rtfSelTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			ID: "view", Scrollable: true, Sizing: FillFill,
			Content: []View{
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 100}),
				RTF(RTFCfg{ID: "rtf", Focusable: true,
					RichText: RichText{Runs: []RichTextRun{
						{Text: "hello world\nalpha beta",
							Style: TextStyle{Size: 14}},
					}}}),
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 850}),
			},
		})
	})

	// Scroll a little (TestScroll applies the theme's scroll
	// multiplier): the RTF must stay visible inside the viewport.
	if err := w.TestScroll("view", 0, -0.5); err != nil {
		t.Fatalf("TestScroll: %v", err)
	}
	_, preDragY, _ := w.TestScrollOffset("view")
	if preDragY >= 0 {
		t.Fatalf("scroll offset = %g, want negative", preDragY)
	}
	ly, ok := w.layout.FindByID("view:rtf")
	if !ok {
		t.Fatal("no RTF shape under the scroll container")
	}
	if ly.Shape.Y <= 0 || ly.Shape.Y+ly.Shape.Height >= 800 {
		t.Fatalf("RTF at y=%g h=%g, want it visible after scroll",
			ly.Shape.Y, ly.Shape.Height)
	}

	// Press rune 2 (line 1, y=10 local), then drag below the viewport.
	press := func(x, y float32) {
		w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
			MouseX: x, MouseY: y})
		w.settle()
	}
	move := func(x, y float32) {
		w.EventFn(&Event{Type: EventMouseMove,
			MouseButton: MouseInvalid, MouseX: x, MouseY: y})
		w.settle()
	}
	release := func(x, y float32) {
		w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
			MouseX: x, MouseY: y})
		w.settle()
	}
	x0, y0 := ly.Shape.X+25, ly.Shape.Y+10
	press(x0, y0)
	if !w.mouseIsLocked() {
		t.Fatal("press did not lock the mouse")
	}
	is := StateReadOr(w, nsInput, "view:rtf", inputState{})
	if is.selectBeg != 2 {
		t.Fatalf("press selected rune %d, want 2", is.selectBeg)
	}

	// Drag below the viewport (y=810): arms edge scroll. The mouse is
	// far below both text lines, so the drag takes the last line whole
	// (textDragEdgeX) and the column under the cursor stops mattering.
	const dragX = 86.5
	move(dragX, 810)
	if !w.HasAnimation(animIDTextDragScroll) {
		t.Fatal("drag outside the viewport did not arm edge scroll")
	}

	// Tick the edge-scroll animation once: the offset moves one 0.3x
	// step toward the edge and the selection extends to the rune under
	// the cursor, re-mapped through the new scroll delta.
	w.animMu.Lock()
	a, ok := w.animations[animIDTextDragScroll].(*Animate)
	w.animMu.Unlock()
	if !ok {
		t.Fatal("edge-scroll animation is not an Animate")
	}
	a.Callback(a, w)
	_, y, _ := w.TestScrollOffset("view")
	if y >= preDragY {
		t.Errorf("scroll = %g, want it past the pre-drag offset %g",
			y, preDragY)
	}
	is = StateReadOr(w, nsInput, "view:rtf", inputState{})
	if is.selectBeg != 2 || is.selectEnd != 22 {
		t.Errorf("selection after tick = [%d,%d), want [2,22) — "+
			"a drag below the text takes the last line whole",
			is.selectBeg, is.selectEnd)
	}

	// Drag back onto the press point, which the scroll moved up the
	// window: the selection must collapse to rune 2 again. This is
	// what proves the ShapeX/ShapeY/scrollDelta translation survives
	// the container scrolling under the cursor — the edge-clamped
	// assertions above no longer read the column.
	//
	// The animation moved the offset without a re-arrange, so the drag
	// path — not the layout — carries the delta: the press point is
	// now that much further up the window.
	move(x0, y0+(y-preDragY))
	is = StateReadOr(w, nsInput, "view:rtf", inputState{})
	if is.selectBeg != 2 || is.selectEnd != 2 {
		t.Errorf("selection back at the press point = [%d,%d), "+
			"want [2,2)", is.selectBeg, is.selectEnd)
	}

	// Dragging back inside the viewport disarms the animation.
	move(dragX, 700)
	if w.HasAnimation(animIDTextDragScroll) {
		t.Error("edge-scroll animation still armed inside the viewport")
	}

	release(dragX, 700)
	if w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	if w.HasAnimation(animIDTextDragScroll) {
		t.Error("edge-scroll animation survived the release")
	}
	is = StateReadOr(w, nsInput, "view:rtf", inputState{})
	if is.selectBeg != 2 || is.selectEnd != 22 {
		t.Errorf("selection after release = [%d,%d), want [2,22)",
			is.selectBeg, is.selectEnd)
	}
}

// --- Link context menu ---

// TestRtfSelectLinkMenuOpensOnRightClick asserts the end-to-end open:
// right-click on the linked run sets the menu state, takes focus, and
// the popup appears as a child of the owning block on the next frame.
func TestRtfSelectLinkMenuOpensOnRightClick(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	x, y := rtfRunePoint(h.shape(t), 8) // inside "page"
	e := h.pressButton(x, y, MouseRight)
	h.release(x, y)

	if !e.IsHandled {
		t.Error("right-click on the link was not consumed")
	}
	st := StateReadOr(h.w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{})
	if !st.Open || st.Link != "https://safe.example.com" {
		t.Fatalf("menu state = %+v, want open with the link", st)
	}
	if got := h.w.FocusID(); got != rtfLinkMenuFocusID {
		t.Errorf("focus = %q, want %q", got, rtfLinkMenuFocusID)
	}
	if _, ok := h.w.layout.FindByID(rtfLinkMenuFocusID); !ok {
		t.Error("link menu popup not in the layout tree")
	}
}

// TestRtfSelectLinkMenuEnterActivatesOpenLink asserts Enter on the
// auto-selected "Open Link" item opens the URI through the native
// platform and dismisses the menu.
func TestRtfSelectLinkMenuEnterActivatesOpenLink(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	platform := &rtfURLCapturePlatform{}
	h.w.SetNativePlatform(platform)

	x, y := rtfRunePoint(h.shape(t), 8)
	h.pressButton(x, y, MouseRight)
	h.release(x, y)

	if err := h.w.TestKey(rtfLinkMenuFocusID, KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey(Enter): %v", err)
	}
	if platform.opened != "https://safe.example.com" {
		t.Errorf("OpenURI = %q, want the link", platform.opened)
	}
	st := StateReadOr(h.w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{})
	if st.Open {
		t.Error("menu still open after activating Open Link")
	}
	if got := h.w.FocusID(); got != "" {
		t.Errorf("focus = %q, want cleared", got)
	}
	if _, ok := h.w.layout.FindByID(rtfLinkMenuFocusID); ok {
		t.Error("menu popup still in the layout tree after activation")
	}
}

// TestRtfSelectLinkMenuDownEnterCopiesLink asserts arrow navigation to
// "Copy Link" and activation copies the URL to the clipboard.
func TestRtfSelectLinkMenuDownEnterCopiesLink(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	var got string
	h.w.SetClipboardFn(func(s string) { got = s })

	x, y := rtfRunePoint(h.shape(t), 8)
	h.pressButton(x, y, MouseRight)
	h.release(x, y)

	if err := h.w.TestKey(rtfLinkMenuFocusID, KeyDown, ModNone); err != nil {
		t.Fatalf("TestKey(KeyDown): %v", err)
	}
	if err := h.w.TestKey(rtfLinkMenuFocusID, KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey(Enter): %v", err)
	}
	if got != "https://safe.example.com" {
		t.Errorf("clipboard = %q, want the link", got)
	}
	st := StateReadOr(h.w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{})
	if st.Open {
		t.Error("menu still open after activating Copy Link")
	}
}

// TestRtfSelectLinkMenuEscapeCloses asserts Escape dismisses the menu:
// focus drops immediately and the amend pass clears the state, so the
// popup leaves the tree on the next frame.
func TestRtfSelectLinkMenuEscapeCloses(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	x, y := rtfRunePoint(h.shape(t), 8)
	h.pressButton(x, y, MouseRight)
	h.release(x, y)

	if err := h.w.TestKey(rtfLinkMenuFocusID, KeyEscape, ModNone); err != nil {
		t.Fatalf("TestKey(Escape): %v", err)
	}
	st := StateReadOr(h.w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{})
	if st.Open {
		t.Error("menu state still open after Escape")
	}
	h.w.TestRender(nil)
	if _, ok := h.w.layout.FindByID(rtfLinkMenuFocusID); ok {
		t.Error("menu popup still in the layout tree after Escape")
	}
}

// TestRtfSelectLinkMenuDismissedByClickElsewhere asserts a left click
// anywhere else dismisses the open menu: the popup state is cleared
// before dispatch, and the click proceeds to place the selection.
func TestRtfSelectLinkMenuDismissedByClickElsewhere(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	x, y := rtfRunePoint(h.shape(t), 8)
	h.pressButton(x, y, MouseRight)
	h.release(x, y)
	if _, ok := h.w.layout.FindByID(rtfLinkMenuFocusID); !ok {
		t.Fatal("menu did not open")
	}

	// Left-click the non-link run: menu must close, selection moves.
	xp, yp := rtfRunePoint(h.shape(t), 3)
	h.press(xp, yp)
	h.release(xp, yp)

	st := StateReadOr(h.w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{})
	if st.Open {
		t.Error("menu still open after clicking elsewhere")
	}
	if got := h.w.FocusID(); got != "rtf" {
		t.Errorf("focus = %q, want the RTF", got)
	}
	expectRtfSel(t, h, 3, 3)
}

// TestRtfSelectLinkRightClickOnPlainRunNoMenu asserts right-click on a
// run without a link never opens the menu — and is not consumed.
func TestRtfSelectLinkRightClickOnPlainRunNoMenu(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	x, y := rtfRunePoint(h.shape(t), 3) // "go to " has no link
	e := h.pressButton(x, y, MouseRight)
	h.release(x, y)

	if e.IsHandled {
		t.Error("right-click on a plain run was consumed")
	}
	st := StateReadOr(h.w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{})
	if st.Open {
		t.Error("menu opened for a plain run")
	}
}

// TestRtfSelectClickOnLinkNavigatesAndSelects asserts a left click on
// a link inside a selectable RTF both navigates (OpenURI) and places
// the selection — rtfOnClick runs first, selection update follows.
func TestRtfSelectClickOnLinkNavigatesAndSelects(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	platform := &rtfURLCapturePlatform{}
	h.w.SetNativePlatform(platform)

	x, y := rtfRunePoint(h.shape(t), 8)
	e := h.press(x, y)
	h.release(x, y)

	if platform.opened != "https://safe.example.com" {
		t.Errorf("OpenURI = %q, want the link", platform.opened)
	}
	if !e.IsHandled {
		t.Error("link click was not consumed")
	}
	expectRtfSel(t, h, 8, 8)
}

// TestRtfOnClickAnchorLinkScrollsToView asserts the in-document anchor
// branch: a '#' link scrolls the owning scroll container to the target
// instead of opening a URI. scrollToView addresses the target by its
// effective ID, so the link names the full scoped path under the
// scroll container ("view:bottom"), the same way a heading slug would
// carry the markdown container's prefix.
func TestRtfOnClickAnchorLinkScrollsToView(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = rtfSelTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			ID: "view", Scrollable: true, Sizing: FillFill,
			Content: []View{
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 100}),
				RTF(RTFCfg{ID: "rtf", RichText: RichText{Runs: []RichTextRun{
					{Text: "jump", Link: "#view:bottom",
						Style: TextStyle{Size: 14}},
				}}}),
				Rectangle(RectangleCfg{Sizing: FixedFill,
					Width: 50, Height: 900}),
				Rectangle(RectangleCfg{ID: "bottom",
					Sizing: FixedFill, Width: 50, Height: 100}),
			},
		})
	})

	ly, ok := w.layout.FindByID("view:rtf")
	if !ok {
		t.Fatal("no RTF shape under the scroll container")
	}
	x, y := ly.Shape.X+15, ly.Shape.Y+10
	e := Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y}
	w.EventFn(&e)
	w.settle()
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y})
	w.settle()

	if !e.IsHandled {
		t.Error("anchor link click was not consumed")
	}
	_, sy, _ := w.TestScrollOffset("view")
	if sy == 0 {
		t.Error("anchor link did not scroll the container")
	}
}
