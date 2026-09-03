package gui

// text_drag_edge_test.go covers the drag hit test once the pointer has
// left the text: a drag carried above the text takes its first line
// through the beginning, and one carried below takes its last line
// through the end, instead of stopping at the pointer's column.

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// mdDragText is two lines of 10px chars, laid out by the RTF
// selection suite's measurer: line 1 at y [0,20), line 2 at y [20,40).
const mdDragText = "alpha beta\ngamma delta"

// TestTextDragEdgeX pins the helper both drag paths share: inside the
// band the column is the pointer's, outside it the drag reaches past
// one end of the line.
func TestTextDragEdgeX(t *testing.T) {
	const top, bot = 0, 40
	for _, tc := range []struct {
		name string
		x, y float32
		want float32
	}{
		{name: "inside_top_edge", x: 33, y: 0, want: 33},
		{name: "inside", x: 33, y: 20, want: 33},
		{name: "inside_bottom_edge", x: 33, y: 40, want: 33},
		{name: "above", x: 33, y: -1, want: -textDragReach},
		{name: "far_above", x: 33, y: -900, want: -textDragReach},
		{name: "below", x: 33, y: 41, want: textDragReach},
		{name: "far_below", x: 33, y: 900, want: textDragReach},
	} {
		if got := textDragEdgeX(tc.x, tc.y, top, bot); got != tc.want {
			t.Errorf("%s: textDragEdgeX(%g, %g) = %g, want %g",
				tc.name, tc.x, tc.y, got, tc.want)
		}
	}
}

// TestGlyphTextBand covers both the ordinary band and the one an
// empty layout reports: no lines means an empty band at the origin,
// which is harmless because a layout with no lines has no offset but
// zero to return either way.
func TestGlyphTextBand(t *testing.T) {
	gl := rtfSelCharLayout(mdDragText, nil)
	if top, bot := glyphTextBand(&gl); top != 0 || bot != 40 {
		t.Errorf("band = [%g,%g), want [0,40)", top, bot)
	}
	if top, bot := glyphTextBand(&glyph.Layout{}); top != 0 || bot != 0 {
		t.Errorf("empty band = [%g,%g), want [0,0)", top, bot)
	}
}

// TestRtfSelectDragAboveTakesFirstLineWhole is the reported symptom:
// the drag leaves the top of the paragraph, so the first line must be
// selected through its beginning, not to the column the pointer left
// at. The pointer sits at column 3, which is what the old behaviour
// returned.
func TestRtfSelectDragAboveTakesFirstLineWhole(t *testing.T) {
	h := newRtfSelectHarness(t, RichText{Runs: []RichTextRun{
		{Text: mdDragText, Style: TextStyle{Size: 14}},
	}})
	ly := h.shape(t)

	// Press rune 15 ('a' of "delta"), line 2 at local y=30.
	h.press(ly.Shape.X+45, ly.Shape.Y+30)
	// Drag above the text, still at column 3.
	h.move(ly.Shape.X+35, ly.Shape.Y-40)

	expectRtfSel(t, h, 15, 0)
}

// TestRtfSelectDragBelowTakesLastLineWhole is the same case downward:
// the last line runs to its end, rune 22.
func TestRtfSelectDragBelowTakesLastLineWhole(t *testing.T) {
	h := newRtfSelectHarness(t, RichText{Runs: []RichTextRun{
		{Text: mdDragText, Style: TextStyle{Size: 14}},
	}})
	ly := h.shape(t)

	// Press rune 3 ('h' of "alpha"), line 1 at local y=10.
	h.press(ly.Shape.X+35, ly.Shape.Y+10)
	// Drag below the text, still at column 3.
	h.move(ly.Shape.X+35, ly.Shape.Y+140)

	expectRtfSel(t, h, 3, 22)
}

// TestRtfSelectDragInsideKeepsColumn is the other half: while the
// pointer is still over the text, the column decides the position, so
// the edge rule cannot swallow an ordinary drag.
func TestRtfSelectDragInsideKeepsColumn(t *testing.T) {
	h := newRtfSelectHarness(t, RichText{Runs: []RichTextRun{
		{Text: mdDragText, Style: TextStyle{Size: 14}},
	}})
	ly := h.shape(t)

	h.press(ly.Shape.X+5, ly.Shape.Y+10)
	// Line 2 ('m' of "gamma", byte 13 → rune 13), column 2.
	h.move(ly.Shape.X+25, ly.Shape.Y+30)

	expectRtfSel(t, h, 0, 13)
}

// mdDragBlocks lays two paragraphs out one under the other, 10px
// apart, the way markdownBlockAmendSel records them: block 0 at
// y [0,40), block 1 at y [50,90).
func mdDragBlocks() []mdBlockInfo {
	gl := rtfSelCharLayout(mdDragText, nil)
	runes := uint32(utf8RuneCount(mdDragText))
	return []mdBlockInfo{
		{FlatText: mdDragText, Layout: gl, H: 40, StartRune: 0,
			RuneLen: runes, ShapeX: 0, ShapeY: 0},
		{FlatText: mdDragText, Layout: gl, H: 40, StartRune: runes,
			RuneLen: runes, ShapeX: 0, ShapeY: 50},
	}
}

// TestMdHitAbsRuneOutsideBlockTakesLineWhole covers the markdown drag
// path, where a paragraph is one block among several: above the first
// block the hit is the start of its first line, below the last block
// the end of its last line, and in the gap between two blocks the end
// of the block above — the block the hit test picks there.
func TestMdHitAbsRuneOutsideBlockTakesLineWhole(t *testing.T) {
	blocks := mdDragBlocks()
	runes := uint32(utf8RuneCount(mdDragText))
	// Column 3 in every case: the old behaviour returned it.
	const x = 35

	for _, tc := range []struct {
		name string
		y    float32
		want uint32
	}{
		{name: "above_first_block", y: -30, want: 0},
		{name: "in_the_gap", y: 45, want: runes},
		{name: "below_last_block", y: 200, want: runes * 2},
		{name: "inside_first_block", y: 10, want: 3},
	} {
		got := mdHitAbsRune(x, tc.y, blocks,
			glyph.Layout{}, "", 0)
		if got != tc.want {
			t.Errorf("%s: mdHitAbsRune(y=%g) = %d, want %d",
				tc.name, tc.y, got, tc.want)
		}
	}
}

// TestInputDragStateDragEdge covers the Input path, which reads its
// band from the drag state's own layout copy: a drag above the field's
// text takes the first line through its beginning, one below takes the
// last line through its end.
func TestInputDragStateDragEdge(t *testing.T) {
	// Two lines of two chars, 10px wide each: line 1 y [0,9),
	// line 2 y [9,20).
	text := "abcd"
	gl := glyph.Layout{
		Lines: []glyph.Line{
			{StartIndex: 0, Length: 2,
				Rect: glyph.Rect{X: 0, Y: 0, Width: 20, Height: 9}},
			{StartIndex: 2, Length: 2,
				Rect: glyph.Rect{X: 0, Y: 9, Width: 20, Height: 11}},
		},
		CharRectByIndex: map[int]int{0: 0, 1: 1, 2: 2, 3: 3},
		CharRects: []glyph.CharRect{
			{Rect: glyph.Rect{X: 0, Y: 0, Width: 10, Height: 9}, Index: 0},
			{Rect: glyph.Rect{X: 10, Y: 0, Width: 10, Height: 9}, Index: 1},
			{Rect: glyph.Rect{X: 0, Y: 9, Width: 10, Height: 11}, Index: 2},
			{Rect: glyph.Rect{X: 10, Y: 9, Width: 10, Height: 11}, Index: 3},
		},
		// A real shaped layout carries a cursor position past the last
		// character; GetClosestOffset returns a line's end only where
		// one exists, so the fixture must have it too.
		LogAttrs: []glyph.LogAttr{
			{IsCursorPosition: true}, {IsCursorPosition: true},
			{IsCursorPosition: true}, {IsCursorPosition: true},
			{IsCursorPosition: true},
		},
		LogAttrByIndex: map[int]int{0: 0, 1: 1, 2: 2, 3: 3, 4: 4},
	}
	w := newTestWindow()
	d := &inputDragState{displayText: text, gl: gl}

	// Above the text at column 1: the whole first line, so rune 0.
	if got := d.computeRunePos(15, -5, w); got != 0 {
		t.Errorf("above = %d, want 0", got)
	}
	// Below the text at column 0: the whole last line, so rune 4.
	if got := d.computeRunePos(5, 30, w); got != 4 {
		t.Errorf("below = %d, want 4", got)
	}
	// Still over the text: the column decides, unchanged.
	if got := d.computeRunePos(15, 4, w); got != 1 {
		t.Errorf("inside = %d, want 1", got)
	}
}

// TestMdHitAbsRuneFallbackTakesLineWhole covers mdHitAbsRune's
// no-blocks path, which hits the widget's own layout directly. The
// edge rule has to reach it too, or a markdown widget whose blocks
// were never recorded keeps the old mid-line behaviour.
func TestMdHitAbsRuneFallbackTakesLineWhole(t *testing.T) {
	gl := rtfSelCharLayout(mdDragText, nil)
	runes := uint32(utf8RuneCount(mdDragText))

	if got := mdHitAbsRune(35, -40, nil, gl, mdDragText, 0); got != 0 {
		t.Errorf("above = %d, want 0", got)
	}
	if got := mdHitAbsRune(35, 140, nil, gl, mdDragText, 0); got != runes {
		t.Errorf("below = %d, want %d", got, runes)
	}
	if got := mdHitAbsRune(25, 30, nil, gl, mdDragText, 0); got != 13 {
		t.Errorf("inside = %d, want 13", got)
	}
}

// --- Text widget ---
//
// The Text widget's existing drag tests (view_text_select_test.go)
// build their window with no text measurer, so the click path takes
// its char-width fallback: a single line, no glyph geometry, and so no
// band for the edge rule to leave. These three drive the same widget
// through EventFn with a measurer installed, which is what puts real
// lines under the drag.

// textDragMeasurer shapes plain text with the deterministic
// 10x20-per-byte layout the RTF suite uses, so a column and a line are
// exact pixel counts. Only LayoutText differs from the RTF measurer it
// embeds: that one returns a bare height, which inputGlyphLayout
// reports as ok=false, dropping the widget back to the fallback above.
type textDragMeasurer struct{ rtfSelTestMeasurer }

func (textDragMeasurer) LayoutText(
	text string, _ TextStyle, _ float32,
) (glyph.Layout, error) {
	return rtfSelCharLayout(text, nil), nil
}

// newTextDragWindow renders one focusable Text filling the window,
// measured by textDragMeasurer. It is newTextSelWindow with the
// measurer installed before the render, which the layout needs: a
// single-line Text is one line box high whatever its text says, so a
// press on the second line would fall outside the shape and never
// reach the widget. Wrap mode sizes the shape from the glyph layout
// instead.
func newTextDragWindow(t *testing.T, text string) (*Window, *Layout) {
	t.Helper()
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = textDragMeasurer{}
	w.TestRender(func(_ *Window) View {
		return Text(TextCfg{
			ID: "txtdrag", Text: text, Focusable: true,
			Mode: TextModeWrap, TextStyle: TextStyle{Size: 14},
		})
	})
	ly, ok := w.layout.FindByID("txtdrag")
	if !ok {
		t.Fatal("no text with effective ID txtdrag")
	}
	return w, ly
}

// textDragMouse dispatches one mouse event and settles the frame.
func textDragMouse(
	w *Window, typ EventType, btn MouseButton, x, y float32,
) {
	e := Event{Type: typ, MouseButton: btn, MouseX: x, MouseY: y}
	w.EventFn(&e)
	w.settle()
}

// expectTextDragSel asserts the selection the dragged widget holds.
// A backward drag is stored anchor-first, so beg > end is expected
// there rather than normalised.
func expectTextDragSel(t *testing.T, w *Window, beg, end uint32) {
	t.Helper()
	is := getInputState(w, "txtdrag")
	if is.selectBeg != beg || is.selectEnd != end {
		t.Errorf("selection = [%d,%d), want [%d,%d)",
			is.selectBeg, is.selectEnd, beg, end)
	}
}

// TestTextDragInsideKeepsColumn is the control: while the pointer is
// over the text the column decides where the selection ends, so the
// edge rule cannot swallow an ordinary drag. It also pins the fixture
// — press and move both map to the rune the geometry says.
func TestTextDragInsideKeepsColumn(t *testing.T) {
	w, ly := newTextDragWindow(t, mdDragText)

	textDragMouse(w, EventMouseDown, MouseLeft,
		ly.Shape.X+5, ly.Shape.Y+10)
	// Line 2 ('m' of "gamma", byte 13 → rune 13), column 2.
	textDragMouse(w, EventMouseMove, MouseInvalid,
		ly.Shape.X+25, ly.Shape.Y+30)

	expectTextDragSel(t, w, 0, 13)
}

// TestTextDragAboveTakesFirstLineWhole is the reported symptom on the
// Text path: the drag leaves the top of the text, so the first line
// is selected through its beginning, not to the column the pointer
// left at (column 3, which is what the old behaviour returned).
func TestTextDragAboveTakesFirstLineWhole(t *testing.T) {
	w, ly := newTextDragWindow(t, mdDragText)

	// Press rune 15 ('a' of "delta") on line 2.
	textDragMouse(w, EventMouseDown, MouseLeft,
		ly.Shape.X+45, ly.Shape.Y+30)
	textDragMouse(w, EventMouseMove, MouseInvalid,
		ly.Shape.X+35, ly.Shape.Y-40)

	expectTextDragSel(t, w, 15, 0)
}

// TestTextDragBelowTakesLastLineWhole is the same case downward: the
// last line runs to its end, rune 22.
func TestTextDragBelowTakesLastLineWhole(t *testing.T) {
	w, ly := newTextDragWindow(t, mdDragText)

	// Press rune 3 ('h' of "alpha") on line 1.
	textDragMouse(w, EventMouseDown, MouseLeft,
		ly.Shape.X+35, ly.Shape.Y+10)
	textDragMouse(w, EventMouseMove, MouseInvalid,
		ly.Shape.X+35, ly.Shape.Y+140)

	expectTextDragSel(t, w, 3, 22)
}
