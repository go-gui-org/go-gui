package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// inputLayoutWithText builds the Column → Row → Text shape structure
// inputTextFromLayout / inputSetTextInLayout / inputGlyphLayoutWithText
// navigate, carrying the given text and an optional cached glyph layout.
func inputLayoutWithText(t *testing.T, text string, gl *glyph.Layout) *Layout {
	t.Helper()
	style := DefaultTextStyle
	tc := &shapeTextConfig{
		Text:            text,
		TextStyle:       &style,
		TextMode:        TextModeSingleLine,
		textLayout:      gl,
		textLayoutText:  text,
		textLayoutStyle: style,
		textLayoutMode:  TextModeSingleLine,
		textLayoutValid: gl != nil,
		textLayoutWidth: 0,
	}
	txt := &Shape{TC: tc}
	return &Layout{Children: []Layout{{
		Children: []Layout{{Shape: txt}},
	}}}
}

func TestInputTextFromLayout(t *testing.T) {
	l := inputLayoutWithText(t, "hello", nil)
	if got := inputTextFromLayout(l); got != "hello" {
		t.Fatalf("inputTextFromLayout() = %q, want %q", got, "hello")
	}

	// Empty inner structure yields empty text.
	if got := inputTextFromLayout(&Layout{}); got != "" {
		t.Fatalf("inputTextFromLayout(empty) = %q, want \"\"", got)
	}

	// Placeholder content is not user text.
	lp := inputLayoutWithText(t, "Type here", nil)
	lp.Children[0].Children[0].Shape.TC.textIsPlaceholder = true
	if got := inputTextFromLayout(lp); got != "" {
		t.Fatalf("inputTextFromLayout(placeholder) = %q, want \"\"", got)
	}
}

func TestInputSetTextInLayout(t *testing.T) {
	l := inputLayoutWithText(t, "old", nil)
	l.Children[0].Children[0].Shape.TC.textIsPlaceholder = true

	inputSetTextInLayout(l, "new")
	if got := l.Children[0].Children[0].Shape.TC.Text; got != "new" {
		t.Fatalf("text after write = %q, want %q", got, "new")
	}
	if l.Children[0].Children[0].Shape.TC.textIsPlaceholder {
		t.Fatal("placeholder flag must be cleared by write-back")
	}
	// A follow-up read returns the typed text, not the placeholder.
	if got := inputTextFromLayout(l); got != "new" {
		t.Fatalf("read after write = %q, want %q", got, "new")
	}

	// Degenerate layouts are no-ops.
	inputSetTextInLayout(&Layout{}, "x")
}

func TestInputGlyphLayoutWithText(t *testing.T) {
	w := newTestWindow()
	// No measurer → not available, no panic.
	if _, ok := inputGlyphLayoutWithText("x", inputLayoutWithText(t, "x", nil), w); ok {
		t.Fatal("glyph layout must be unavailable without a measurer")
	}

	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	// Missing inner shapes → unavailable.
	if _, ok := inputGlyphLayoutWithText("x", &Layout{}, w); ok {
		t.Fatal("glyph layout must be unavailable for an empty layout")
	}

	// Cached glyph layout is returned when present.
	gl := &glyph.Layout{Height: 20}
	l := inputLayoutWithText(t, "hello", gl)
	got, ok := inputGlyphLayoutWithText("hello", l, w)
	if !ok || got.Height != 20 {
		t.Fatalf("cached layout = (%v, %v), want height 20, ok", got.Height, ok)
	}
}

func TestTrailingLineStart(t *testing.T) {
	lines := []glyph.Line{
		{StartIndex: 0, Length: 5},
		{StartIndex: 5, Length: 4},
		{StartIndex: 9, Length: 3},
	}
	// At the boundary of line 2 → start of line 1.
	if got := trailingLineStart(lines, 5, -1); got != 0 {
		t.Fatalf("trailingLineStart(5) = %d, want 0", got)
	}
	// At the boundary of line 3 → start of line 2.
	if got := trailingLineStart(lines, 9, -1); got != 5 {
		t.Fatalf("trailingLineStart(9) = %d, want 5", got)
	}
	// Not a boundary → fallback.
	if got := trailingLineStart(lines, 7, 42); got != 42 {
		t.Fatalf("trailingLineStart(7) = %d, want fallback 42", got)
	}
	// Cursor at line 1 start is not a wrap boundary.
	if got := trailingLineStart(lines, 0, -1); got != -1 {
		t.Fatalf("trailingLineStart(0) = %d, want fallback -1", got)
	}
}

func TestTrailingLineEnd(t *testing.T) {
	lines := []glyph.Line{
		{StartIndex: 0, Length: 5},
		{StartIndex: 5, Length: 4},
	}
	// End of the previous visual line = start + length.
	if got := trailingLineEnd(lines, 5, -1); got != 5 {
		t.Fatalf("trailingLineEnd(5) = %d, want 5", got)
	}
	// Not a boundary → fallback.
	if got := trailingLineEnd(lines, 3, 99); got != 99 {
		t.Fatalf("trailingLineEnd(3) = %d, want fallback 99", got)
	}
}

// glyphCursorLayout builds a minimal glyph layout whose valid cursor
// positions are exactly the given byte indices — the same positions a
// shaped layout would expose at grapheme boundaries.
func glyphCursorLayout(text string, cursors []int) *glyph.Layout {
	attrs := make([]glyph.LogAttr, len(cursors))
	byIndex := make(map[int]int, len(cursors))
	for i, c := range cursors {
		attrs[i] = glyph.LogAttr{IsCursorPosition: true}
		byIndex[c] = i
	}
	return &glyph.Layout{
		Text:           text,
		LogAttrs:       attrs,
		LogAttrByIndex: byIndex,
	}
}

func TestInputDeleteGraphemeCombining(t *testing.T) {
	// "a" + combining acute (U+0301) + "b": one grapheme is a+U+0301.
	// Backspace at the cursor (after a+U+0301) must delete the whole
	// cluster, not just the combining mark.
	text := "a\u0301b"
	w := newTestWindow()
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	layout := inputLayoutWithText(t, text,
		glyphCursorLayout(text, []int{0, 3, 4}))
	focusID := "f-grapheme"
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set(focusID, inputState{CursorPos: 2}) // runes: a, U+0301, b

	got, changed := inputDeleteGrapheme(text, focusID, false, layout, w)
	if !changed || got != "b" {
		t.Fatalf("backspace = (%q, %v), want (\"b\", true)", got, changed)
	}
	is := inputStateOrDefault(focusID, w)
	if is.CursorPos != 0 {
		t.Fatalf("cursor = %d, want 0", is.CursorPos)
	}

	// Backspace again deletes the remaining a+U+0301 cluster.
	text2 := "a\u0301"
	imap.Set(focusID, inputState{CursorPos: 2})
	got, changed = inputDeleteGrapheme(text2, focusID, false,
		inputLayoutWithText(t, text2, glyphCursorLayout(text2, []int{0, 3})), w)
	if !changed || got != "" {
		t.Fatalf("backspace cluster = (%q, %v), want (\"\", true)", got, changed)
	}
}

func TestInputDeleteGraphemeEmojiZWJ(t *testing.T) {
	// 👨👩👧 is a single grapheme cluster (family emoji, ZWJ-joined).
	// One backspace must remove the whole cluster, not one rune.
	text := "👨\u200d👩\u200d👧"
	w := newTestWindow()
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	layout := inputLayoutWithText(t, text,
		glyphCursorLayout(text, []int{0, len(text)}))
	focusID := "f-zwj"
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: utf8RuneCount(text)})

	got, changed := inputDeleteGrapheme(text, focusID, false, layout, w)
	if !changed || got != "" {
		t.Fatalf("backspace ZWJ = (%q, %v), want (\"\", true)", got, changed)
	}
}

func TestInputDeleteGraphemeForward(t *testing.T) {
	// Delete at the start of "a\u0301b" removes the a+U+0301 cluster.
	text := "a\u0301b"
	w := newTestWindow()
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	layout := inputLayoutWithText(t, text,
		glyphCursorLayout(text, []int{0, 3, 4}))
	focusID := "f-fwd"
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: 0})

	got, changed := inputDeleteGrapheme(text, focusID, true, layout, w)
	if !changed || got != "b" {
		t.Fatalf("forward delete = (%q, %v), want (\"b\", true)", got, changed)
	}
	if is := inputStateOrDefault(focusID, w); is.CursorPos != 0 {
		t.Fatalf("cursor = %d, want 0", is.CursorPos)
	}
}

func TestInputDeleteGraphemeFallbackCluster(t *testing.T) {
	// Without a text measurer, deletion falls back to UAX #29
	// boundaries computed by gui — a whole grapheme per keystroke,
	// same granularity as the shaped path.
	w := newTestWindow() // no textMeasurer
	focusID := "f-fallback"

	// ASCII: one rune per cluster, behaves exactly as before.
	layout := inputLayoutWithText(t, "ab", nil)
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: 1})
	got, changed := inputDeleteGrapheme("ab", focusID, false, layout, w)
	if !changed || got != "b" {
		t.Fatalf("fallback backspace = (%q, %v), want (\"b\", true)", got, changed)
	}

	// ZWJ family emoji: one Backspace removes all seven runes.
	text := "a\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466"
	layout = inputLayoutWithText(t, text, nil)
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: utf8RuneCount(text)})
	got, changed = inputDeleteGrapheme(text, focusID, false, layout, w)
	if !changed || got != "a" {
		t.Fatalf("fallback cluster backspace = (%q, %v), want (\"a\", true)", got, changed)
	}
	if is := inputStateOrDefault(focusID, w); is.CursorPos != 1 {
		t.Fatalf("cursor = %d, want 1", is.CursorPos)
	}

	// Forward delete from after "a" removes the whole cluster too.
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: 1})
	got, changed = inputDeleteGrapheme(text, focusID, true, layout, w)
	if !changed || got != "a" {
		t.Fatalf("fallback cluster forward = (%q, %v), want (\"a\", true)", got, changed)
	}
}

func TestInputDeleteGraphemeSelection(t *testing.T) {
	// An active selection deletes the selected range regardless of
	// grapheme clusters, exactly like inputDelete.
	text := "a\u0301b"
	w := newTestWindow()
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	layout := inputLayoutWithText(t, text,
		glyphCursorLayout(text, []int{0, 3, 4}))
	focusID := "f-sel"
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: 3, selectBeg: 1, selectEnd: 3})

	got, changed := inputDeleteGrapheme(text, focusID, false, layout, w)
	if !changed || got != "a" {
		t.Fatalf("selection delete = (%q, %v), want (\"a\", true)", got, changed)
	}
}

func TestInputDragStateComputeRunePos(t *testing.T) {
	// Two visual lines; the drag state must map the pointer to the
	// correct rune, honouring the recorded scroll delta.
	text := "abcd"
	gl := &glyph.Layout{
		Lines: []glyph.Line{
			{StartIndex: 0, Length: 2, Rect: glyph.Rect{X: 0, Y: 0, Width: 20, Height: 9}},
			{StartIndex: 2, Length: 2, Rect: glyph.Rect{X: 0, Y: 9, Width: 20, Height: 11}},
		},
		CharRectByIndex: map[int]int{0: 0, 1: 1, 2: 2, 3: 3},
		CharRects: []glyph.CharRect{
			{Rect: glyph.Rect{X: 0, Y: 0, Width: 10, Height: 9}, Index: 0},
			{Rect: glyph.Rect{X: 10, Y: 0, Width: 10, Height: 9}, Index: 1},
			{Rect: glyph.Rect{X: 0, Y: 9, Width: 10, Height: 11}, Index: 2},
			{Rect: glyph.Rect{X: 10, Y: 9, Width: 10, Height: 11}, Index: 3},
		},
	}
	w := newTestWindow()

	// No scroll: y=0 lands on the first line.
	d0 := &inputDragState{displayText: text, gl: *gl}
	if got := d0.computeRunePos(5, 5, w); got != 0 {
		t.Fatalf("computeRunePos(5,5) = %d, want 0", got)
	}

	// Scroll -10: the view has moved down one line, so y=0 maps to
	// the start of the second line.
	w.scrollY().Set("inp", -10)
	d1 := &inputDragState{
		displayText: text, gl: *gl,
		scrollID: "inp", scrollY0: 0,
	}
	if got := d1.computeRunePos(5, 0, w); got != 2 {
		t.Fatalf("computeRunePos(5,0) scrolled = %d, want 2", got)
	}
}

func TestInputDragStateUpdateSelection(t *testing.T) {
	w := newTestWindow()
	focusID := "f-drag"

	// Plain drag: anchor at rune 1, pointer at rune 3.
	d := &inputDragState{focusID: focusID, anchorPos: 1}
	d.updateSelection(3, w)
	is := inputStateOrDefault(focusID, w)
	if is.CursorPos != 3 || is.selectBeg != 1 || is.selectEnd != 3 {
		t.Fatalf("plain drag state = %+v, want cursor 3, sel 1..3", is)
	}

	// Double-click word drag: anchor inside "hello", pointer moves
	// into the word → selection expands to the whole word forward.
	dw := &inputDragState{
		focusID:     focusID,
		displayText: "hello world",
		wordSelect:  true,
		anchorPos:   0, anchorEnd: 5,
	}
	dw.updateSelection(3, w)
	is = inputStateOrDefault(focusID, w)
	if is.CursorPos != 5 || is.selectBeg != 0 || is.selectEnd != 5 {
		t.Fatalf("word drag forward state = %+v, want cursor 5, sel 0..5", is)
	}

	// Word drag backwards: pointer before the anchor → selection
	// runs from the word start back to the anchor end.
	dw2 := &inputDragState{
		focusID:     focusID,
		displayText: "hello world",
		wordSelect:  true,
		anchorPos:   6, anchorEnd: 11,
	}
	dw2.updateSelection(1, w)
	is = inputStateOrDefault(focusID, w)
	if is.CursorPos != 0 || is.selectBeg != 11 || is.selectEnd != 0 {
		t.Fatalf("word drag backward state = %+v, want cursor 0, sel 11..0", is)
	}
}

func TestInputDragStateScrollCallback(t *testing.T) {
	text := "abcd"
	gl := &glyph.Layout{
		Lines: []glyph.Line{
			{StartIndex: 0, Length: 4, Rect: glyph.Rect{X: 0, Y: 0, Width: 40, Height: 10}},
		},
		CharRectByIndex: map[int]int{0: 0, 1: 1, 2: 2, 3: 3},
		CharRects: []glyph.CharRect{
			{Rect: glyph.Rect{X: 0, Y: 0, Width: 10, Height: 10}, Index: 0},
			{Rect: glyph.Rect{X: 10, Y: 0, Width: 10, Height: 10}, Index: 1},
			{Rect: glyph.Rect{X: 20, Y: 0, Width: 10, Height: 10}, Index: 2},
			{Rect: glyph.Rect{X: 30, Y: 0, Width: 10, Height: 10}, Index: 3},
		},
	}
	w := newTestWindow()
	focusID := "f-scroll"
	StateMap[string, inputState](w, nsInput, capMany).Set(
		focusID, inputState{CursorPos: 0})

	// Pointer above the viewport → scrolls up (toward less negative)
	// and re-selects from the recomputed rune position.
	d := &inputDragState{
		displayText:  text,
		gl:           *gl,
		focusID:      focusID,
		lastMouseX:   15,
		lastMouseY:   5,
		viewTop:      20,
		viewBot:      80,
		scrollID:     "inp",
		scrollY0:     0,
		maxScrollNeg: -100,
	}
	w.scrollY().Set("inp", -10)
	d.scrollCallback(nil, w)
	if got := w.scrollY().GetOr("inp", 0); got >= 0 {
		t.Fatalf("scrollY after above-viewport callback = %v, want < 0", got)
	}
	is := inputStateOrDefault(focusID, w)
	if is.CursorPos == 0 && is.selectEnd == 0 {
		t.Fatal("scroll callback must update the selection from the recomputed position")
	}

	// Pointer inside the viewport → stops the drag-scroll animation.
	w.AnimationAdd(&Animate{AnimID: animIDDragScroll, Delay: 1})
	d.viewTop, d.viewBot = 0, 100
	d.lastMouseY = 50
	d.scrollCallback(nil, w)
	if w.HasAnimation(animIDDragScroll) {
		t.Fatal("drag-scroll animation must be removed while the pointer is inside")
	}

	// Pointer below the viewport but already at the scroll floor:
	// the clamped target equals the current scroll, so nothing moves.
	d2 := &inputDragState{
		displayText:  text,
		gl:           *gl,
		focusID:      focusID,
		lastMouseX:   15,
		lastMouseY:   200,
		viewTop:      20,
		viewBot:      80,
		scrollID:     "inp",
		scrollY0:     -5.5,
		maxScrollNeg: -5.5,
	}
	before := w.scrollY().GetOr("inp", 0)
	d2.scrollCallback(nil, w)
	if got := w.scrollY().GetOr("inp", 0); got != before {
		t.Fatalf("scrollY changed without room to scroll: %v → %v", before, got)
	}
}
