package gui

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-glyph"
)

// The block is checked against a whole-text layout from the same stub
// shaper: with the chunk budget shrunk to a few bytes, every query over
// the chunked block must answer what the single layout answers. The stub
// wraps greedily at spaces and consumes the space it breaks at, breaks a
// word longer than the line by character, and gives a text ending in a
// break an empty last line — the rules go-glyph's layout follows, as
// probed on the pure-Go shaper.

const (
	stubCW = float32(10)
	stubLH = float32(20)
)

type blockStubMeasurer struct{}

func (blockStubMeasurer) TextWidth(text string, _ TextStyle) float32 {
	return float32(len(text)) * stubCW
}
func (blockStubMeasurer) TextHeight(_ string, _ TextStyle) float32 { return stubLH }
func (blockStubMeasurer) FontHeight(_ TextStyle) float32           { return stubLH }
func (blockStubMeasurer) FontAscent(_ TextStyle) float32           { return 16 }
func (blockStubMeasurer) LayoutText(
	text string, _ TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	if text == "" {
		return glyph.Layout{}, errors.New("empty string not allowed")
	}
	return stubLayout(text, wrapWidth), nil
}

type countingBlockMeasurer struct {
	blockStubMeasurer
	layouts int
}

func (m *countingBlockMeasurer) LayoutText(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	m.layouts++
	return m.blockStubMeasurer.LayoutText(text, style, wrapWidth)
}

type probeCountingBlockMeasurer struct {
	blockStubMeasurer
	full, probes int
}

func (m *probeCountingBlockMeasurer) LayoutText(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	m.full++
	return m.blockStubMeasurer.LayoutText(text, style, wrapWidth)
}

func (m *probeCountingBlockMeasurer) LayoutTextNoHitTesting(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	m.probes++
	return m.blockStubMeasurer.LayoutText(text, style, wrapWidth)
}

type stubLine struct{ start, end int }

func stubLayout(text string, wrapW float32) glyph.Layout {
	n := len(text)
	var lines []stubLine
	consumed := map[int]bool{}
	ls, cur := 0, 0
	var w float32
	flush := func(end, next int) {
		lines = append(lines, stubLine{ls, end})
		ls, cur, w = next, next, 0
	}
	for cur < n {
		c := text[cur]
		if c == '\n' {
			flush(cur, cur+1)
			continue
		}
		if c == ' ' {
			if wrapW > 0 && w > 0 && w+stubCW > wrapW {
				consumed[cur] = true
				flush(cur, cur+1)
				continue
			}
			w += stubCW
			cur++
			continue
		}
		j := cur
		for j < n && text[j] != ' ' && text[j] != '\n' {
			j++
		}
		ww := float32(j-cur) * stubCW
		if wrapW > 0 && w > 0 && w+ww > wrapW {
			if cur > ls && text[cur-1] == ' ' {
				consumed[cur-1] = true
				flush(cur-1, cur)
			} else {
				flush(cur, cur)
			}
			continue
		}
		if wrapW > 0 && ww > wrapW {
			fit := max(1, int(wrapW/stubCW))
			flush(cur+fit, cur+fit)
			continue
		}
		w += ww
		cur = j
	}
	flush(cur, cur)

	var l glyph.Layout
	l.Text = text
	l.CharRectByIndex = map[int]int{}
	for li, ln := range lines {
		for i := ln.start; i < ln.end; i++ {
			l.CharRectByIndex[i] = len(l.CharRects)
			l.CharRects = append(l.CharRects, glyph.CharRect{
				Rect: glyph.Rect{
					X: float32(i-ln.start) * stubCW, Y: float32(li) * stubLH,
					Width: stubCW, Height: stubLH,
				},
				Index: i,
			})
		}
		lw := float32(ln.end-ln.start) * stubCW
		l.Lines = append(l.Lines, glyph.Line{
			StartIndex: ln.start, Length: ln.end - ln.start,
			Rect: glyph.Rect{Y: float32(li) * stubLH, Width: lw, Height: stubLH},
		})
		l.Width = f32Max(l.Width, lw)
	}
	l.Height = float32(len(lines)) * stubLH
	l.Text = text // glyph keeps the text it shaped, and so pins it
	l.LogAttrByIndex, l.LogAttrs = stubLogAttrs(text)
	// The consumed space keeps its word attribute but is not a caret
	// stop, so the caret skips from the line end to the next line start.
	for i := range consumed {
		l.LogAttrs[l.LogAttrByIndex[i]].IsCursorPosition = false
	}
	return l
}

// stubLogAttrs marks every byte and the end as a caret stop, a word
// start at a non-separator after a separator, and a word end at a
// separator after a non-separator. Space, tab and newline separate, so
// the attributes of a byte depend only on its neighbour and a chunk
// cut at a separator recomputes the same ones.
func stubLogAttrs(text string) (map[int]int, []glyph.LogAttr) {
	byIndex := map[int]int{}
	var attrs []glyph.LogAttr
	sep := func(i int) bool {
		return i < 0 || i >= len(text) ||
			text[i] == ' ' || text[i] == '\t' || text[i] == '\n'
	}
	for i := 0; i <= len(text); i++ {
		byIndex[i] = len(attrs)
		attrs = append(attrs, glyph.LogAttr{
			IsCursorPosition: true,
			IsWordStart:      !sep(i) && sep(i-1),
			IsWordEnd:        sep(i) && !sep(i-1),
			IsLineBreak:      i < len(text) && text[i] == '\n',
		})
	}
	return byIndex, attrs
}

// shapedBlockLayout returns the one shaped layout of a rendered text
// shape's block, for tests that inspect what the sizing pass shaped.
func shapedBlockLayout(t *testing.T, tc *ShapeTextConfig) *glyph.Layout {
	t.Helper()
	if tc == nil || tc.block == nil {
		t.Fatal("text shape has no block")
	}
	if n := len(tc.block.chunks); n != 1 {
		t.Fatalf("block has %d chunks, want 1", n)
	}
	l := tc.block.chunks[0].layout
	if l == nil {
		t.Fatal("block chunk is not shaped")
	}
	return l
}

func newBlockTestWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
	t.Cleanup(w.WindowCleanup)
	w.textMeasurer = blockStubMeasurer{}
	return w
}

// withChunkBytes shrinks the chunk budget for one test.
func withChunkBytes(t *testing.T, n int) {
	t.Helper()
	old := textChunkBytes
	textChunkBytes = n
	t.Cleanup(func() { textChunkBytes = old })
}

func blockEnsureAll(b *textBlock, w *Window) {
	for i := 0; i < len(b.chunks); i++ {
		b.ensure(i, w)
	}
	b.settle()
}

func blockRanges(b *textBlock) []string {
	out := make([]string, 0, len(b.chunks))
	for _, c := range b.chunks {
		s := fmt.Sprintf("[%d,%d)", c.start, c.end)
		if c.piece {
			s += "p"
		}
		if c.open {
			s += "?"
		}
		out = append(out, s)
	}
	return out
}

var blockTestStyle = TextStyle{Size: 16}

func TestTextBlockChunksParagraphs(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	text := "aaaa bbbb\n\ncccc\ndddd eeee ffff gggg hhhh"
	b := newTextBlock(text, 100, blockTestStyle, TextModeWrap, w)
	// Built eagerly: the long paragraph is cut into pieces at the line
	// breaks a whole-paragraph layout has, leaving out the consumed space.
	want := []string{"[0,9)", "[10,10)", "[11,15)", "[16,25)", "[26,35)p", "[36,40)p"}
	if got := blockRanges(b); !slices.Equal(got, want) {
		t.Fatalf("chunks = %v, want %v", got, want)
	}
	// Six rows of 20: "aaaa bbbb", "", "cccc", "dddd eeee", "ffff gggg", "hhhh".
	if b.height != 6*stubLH {
		t.Errorf("height = %v, want %v", b.height, 6*stubLH)
	}
	if b.boxHeight() != 6*stubLH {
		t.Errorf("boxHeight = %v, want %v", b.boxHeight(), 6*stubLH)
	}
}

func TestTextBlockPacksShortParagraphs(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	b := newTextBlock("ab\ncd\nef\nghij\nk", 0, blockTestStyle, TextModeMultiline, w)
	// "ab\ncd\nef" is 8 bytes; adding "\nghij" would make 13, so "ghij"
	// starts the next chunk, which "\nk" then joins.
	want := []string{"[0,8)", "[9,15)"}
	if got := blockRanges(b); !slices.Equal(got, want) {
		t.Fatalf("chunks = %v, want %v", got, want)
	}
}

func TestTextBlockEmptyAndNewlines(t *testing.T) {
	w := newBlockTestWindow(t)
	cases := []struct {
		text  string
		lines int
		want  []string
	}{
		{"", 1, []string{"[0,0)"}},
		{"\n", 2, []string{"[0,0)", "[1,1)"}},
		{"\n\n", 3, []string{"[0,0)", "[1,1)", "[2,2)"}},
		{"a\n", 2, []string{"[0,1)", "[2,2)"}},
		{"a\n\nb", 3, []string{"[0,1)", "[2,2)", "[3,4)"}},
	}
	for _, tc := range cases {
		b := newTextBlock(tc.text, 100, blockTestStyle, TextModeWrap, w)
		if got := blockRanges(b); !slices.Equal(got, tc.want) {
			t.Errorf("%q: chunks = %v, want %v", tc.text, got, tc.want)
		}
		if b.height != float32(tc.lines)*stubLH {
			t.Errorf("%q: height = %v, want %d lines", tc.text, b.height, tc.lines)
		}
		// The caret at the end sits on the last line: an empty one after
		// a trailing newline, else after the last character.
		wantX := float32(0)
		if n := len(tc.text); n > 0 && tc.text[n-1] != '\n' {
			wantX = stubCW
		}
		cp, ok := b.cursorPos(len(tc.text))
		if !ok || cp.Y != float32(tc.lines-1)*stubLH || cp.X != wantX {
			t.Errorf("%q: caret at end = %+v %v, want x=%v y=%v",
				tc.text, cp, ok, wantX, float32(tc.lines-1)*stubLH)
		}
	}
}

// blockOracleTexts are compared against a whole-text layout at a
// 12-byte chunk budget, so each one spans several chunks.
var blockOracleTexts = []string{
	"the quick brown fox jumps over the lazy dog",
	"aaaa bbbb\n\ncccc\ndddd eeee ffff gggg hhhh",
	"one\ntwo\nthree\n",
	"abcdefghijklmnopqrstuvwxyz0123456789",
	"short\n\n\nlines here and a somewhat longer paragraph that wraps a few times over\n",
	"  leading and trailing spaces  \nx y z",
	"word " + strings.Repeat("ab ", 30) + "end",
}

func TestTextBlockMatchesWholeLayout(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	for _, text := range blockOracleTexts {
		for _, wrapW := range []float32{0, 100} {
			mode := TextModeMultiline
			if wrapW > 0 {
				mode = TextModeWrap
			}
			t.Run(fmt.Sprintf("%q/w%v", truncatePreview(text, 12), wrapW), func(t *testing.T) {
				whole := stubLayout(text, wrapW)
				b := newTextBlock(text, wrapW, blockTestStyle, mode, w)
				if len(b.chunks) < 2 {
					t.Fatalf("expected several chunks, got %v", blockRanges(b))
				}
				compareBlockToWhole(t, b, &whole, text, w)
			})
		}
	}
}

// A text above the eager budget can hold a long paragraph as one open
// chunk. Every query must keep resolving after shaping cuts that chunk:
// the requested offset or point usually belongs to the newly inserted
// remainder, not to the first piece just shaped.
func TestTextBlockLazyLongParagraphQueries(t *testing.T) {
	withChunkBytes(t, 12)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	text := strings.Repeat("ab ", 100) + "end"

	for _, wrapW := range []float32{0, 100} {
		mode := TextModeMultiline
		if wrapW > 0 {
			mode = TextModeWrap
		}
		whole := stubLayout(text, wrapW)
		fresh := func(t *testing.T) *textBlock {
			t.Helper()
			b := newTextBlock(text, wrapW, blockTestStyle, mode, w)
			if len(b.chunks) != 1 || !b.chunks[0].open || b.chunks[0].measured {
				t.Fatalf("initial chunks = %v, want one unmeasured open chunk",
					blockRanges(b))
			}
			return b
		}

		t.Run(fmt.Sprintf("wrap=%v/cursor", wrapW), func(t *testing.T) {
			got, ok := fresh(t).cursorPos(len(text))
			want, wantOK := whole.GetCursorPos(len(text))
			if got != want || ok != wantOK {
				t.Fatalf("cursor at end = %+v/%v, want %+v/%v",
					got, ok, want, wantOK)
			}
		})

		t.Run(fmt.Sprintf("wrap=%v/offset-queries", wrapW), func(t *testing.T) {
			idx := len(text) - 1
			checks := []struct {
				name string
				got  func(*textBlock) int
				want int
			}{
				{"word-left", func(b *textBlock) int { return b.moveWordLeft(len(text)) }, whole.MoveCursorWordLeft(len(text))},
				{"word-right", func(b *textBlock) int { return b.moveWordRight(idx) }, whole.MoveCursorWordRight(idx)},
				{"line-start", func(b *textBlock) int { return b.moveLineStart(idx) }, whole.MoveCursorLineStart(idx)},
				{"line-end", func(b *textBlock) int { return b.moveLineEnd(idx) }, whole.MoveCursorLineEnd(idx)},
				{"up", func(b *textBlock) int { return b.moveUp(len(text), -1) }, whole.MoveCursorUp(len(text), -1)},
			}
			for _, check := range checks {
				if got := check.got(fresh(t)); got != check.want {
					t.Errorf("%s = %d, want %d", check.name, got, check.want)
				}
			}

			gs, ge := fresh(t).wordAt(idx)
			ws, we := whole.GetWordAtIndex(idx)
			if gs != ws || ge != we {
				t.Errorf("wordAt(%d) = [%d,%d), want [%d,%d)",
					idx, gs, ge, ws, we)
			}

			wantDelete := glyph.DeleteBackward(text, whole, idx)
			gotText, gotPos, ok := fresh(t).deleteGrapheme(idx, false)
			if !ok || gotText != wantDelete.NewText || gotPos != wantDelete.CursorPos {
				t.Errorf("delete backward = %q/%d/%v, want %q/%d/true",
					gotText, gotPos, ok, wantDelete.NewText, wantDelete.CursorPos)
			}
		})

		t.Run(fmt.Sprintf("wrap=%v/point", wrapW), func(t *testing.T) {
			last := whole.Lines[len(whole.Lines)-1].Rect
			x := last.X + last.Width - stubCW/4
			y := last.Y + last.Height/2
			if got, want := fresh(t).closestOffset(x, y), whole.GetClosestOffset(x, y); got != want {
				t.Errorf("closestOffset(%v,%v) = %d, want %d", x, y, got, want)
			}
		})

		if len(whole.Lines) > 1 {
			t.Run(fmt.Sprintf("wrap=%v/line-before", wrapW), func(t *testing.T) {
				idx := whole.Lines[len(whole.Lines)-1].StartIndex
				gotStart, gotEnd, gotRect, ok := fresh(t).lineBefore(idx)
				prev := whole.Lines[len(whole.Lines)-2]
				wantStart := prev.StartIndex
				wantEnd := prev.StartIndex + prev.Length
				if !ok || gotStart != wantStart || gotEnd != wantEnd || gotRect != prev.Rect {
					t.Errorf("lineBefore(%d) = %d,%d,%+v/%v, want %d,%d,%+v/true",
						idx, gotStart, gotEnd, gotRect, ok,
						wantStart, wantEnd, prev.Rect)
				}
			})
		}
	}
}

// A far offset in one unwrapped paragraph needs only its destination
// layout. The intervening pieces use estimated widths until they become
// visible; shaping all of them made End linear in full-layout allocations.
func TestTextBlockLazyNoWrapFarCursorShapesOnlyDestination(t *testing.T) {
	withChunkBytes(t, 12)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	m := &countingBlockMeasurer{}
	w.textMeasurer = m
	text := strings.Repeat("ab ", 100) + "end"
	b := newTextBlock(text, 0, blockTestStyle, TextModeMultiline, w)
	// One tiny layout probes line height while estimating the initial open
	// chunk. The far cursor should add exactly its destination layout.
	if m.layouts != 1 {
		t.Fatalf("setup shaped %d layouts, want 1 metric probe", m.layouts)
	}
	got, ok := b.cursorPos(len(text))
	whole := stubLayout(text, 0)
	want, wantOK := whole.GetCursorPos(len(text))
	if got != want || ok != wantOK {
		t.Fatalf("cursor at end = %+v/%v, want %+v/%v",
			got, ok, want, wantOK)
	}
	if m.layouts != 2 {
		t.Fatalf("far cursor shaped %d layouts, want metric probe + destination",
			m.layouts)
	}
	if len(b.chunks) < 2 {
		t.Fatalf("far cursor did not split the open paragraph: %v", blockRanges(b))
	}
}

// A far caret in a wrapped paragraph must discover every preceding soft-wrap
// seam, but those probes already provide exact prefix geometry. Only the
// destination needs a retained, hit-testable layout; reshaping every prefix
// was the dominant cost of a megabyte one-paragraph paste.
func TestTextBlockLazyWrapFarCursorShapesOnlyDestinationFully(t *testing.T) {
	withChunkBytes(t, 12)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	m := &probeCountingBlockMeasurer{}
	w.textMeasurer = m
	text := strings.Repeat("ab ", 100) + "end"
	b := newTextBlock(text, 100, blockTestStyle, TextModeWrap, w)
	if m.full != 1 || m.probes != 0 {
		t.Fatalf("setup layouts = %d full, %d probes; want 1, 0",
			m.full, m.probes)
	}
	got, ok := b.cursorPos(len(text))
	whole := stubLayout(text, 100)
	want, wantOK := whole.GetCursorPos(len(text))
	if got != want || ok != wantOK {
		t.Fatalf("cursor at end = %+v/%v, want %+v/%v",
			got, ok, want, wantOK)
	}
	if m.full != 2 {
		t.Fatalf("far cursor shaped %d full layouts, want metric + destination",
			m.full)
	}
	if m.probes < 2 {
		t.Fatalf("far cursor used %d seam probes, want multiple", m.probes)
	}
	if b.live != 1 {
		t.Fatalf("far cursor retained %d layouts, want destination only", b.live)
	}
}

func compareBlockToWhole(t *testing.T, b *textBlock, whole *glyph.Layout, text string, w *Window) {
	t.Helper()
	if b.height != whole.Height {
		t.Errorf("height = %v, want %v", b.height, whole.Height)
	}
	if b.width != whole.Width {
		t.Errorf("width = %v, want %v", b.width, whole.Width)
	}
	for idx := 0; idx <= len(text); idx++ {
		wcp, wok := whole.GetCursorPos(idx)
		bcp, bok := b.cursorPos(idx)
		// At a character-level wrap, where no byte separates the lines,
		// glyph.Layout.GetCursorPos answers with the earlier line's y
		// and the later line's x. The block puts the caret on the later
		// line, where the character is, so that one offset is not
		// compared.
		if wok && !hardWrapAt(whole, idx) && (!bok || wcp != bcp) {
			t.Errorf("cursorPos(%d) = %+v %v, want %+v %v", idx, bcp, bok, wcp, wok)
		}
		if got, want := b.moveLeft(idx), whole.MoveCursorLeft(idx); got != want {
			t.Errorf("moveLeft(%d) = %d, want %d", idx, got, want)
		}
		if got, want := b.moveRight(idx), whole.MoveCursorRight(idx); got != want {
			t.Errorf("moveRight(%d) = %d, want %d", idx, got, want)
		}
		if got, want := b.moveWordLeft(idx), whole.MoveCursorWordLeft(idx); got != want {
			t.Errorf("moveWordLeft(%d) = %d, want %d", idx, got, want)
		}
		if got, want := b.moveWordRight(idx), whole.MoveCursorWordRight(idx); got != want {
			t.Errorf("moveWordRight(%d) = %d, want %d", idx, got, want)
		}
		if got, want := b.moveLineStart(idx), whole.MoveCursorLineStart(idx); got != want {
			t.Errorf("moveLineStart(%d) = %d, want %d", idx, got, want)
		}
		if got, want := b.moveLineEnd(idx), whole.MoveCursorLineEnd(idx); got != want {
			t.Errorf("moveLineEnd(%d) = %d, want %d", idx, got, want)
		}
		if wok {
			if got, want := b.moveUp(idx, wcp.X), whole.MoveCursorUp(idx, wcp.X); got != want {
				t.Errorf("moveUp(%d) = %d, want %d", idx, got, want)
			}
			if got, want := b.moveDown(idx, wcp.X), whole.MoveCursorDown(idx, wcp.X); got != want {
				t.Errorf("moveDown(%d) = %d, want %d", idx, got, want)
			}
		}
		gs, ge := b.wordAt(idx)
		ws, we := whole.GetWordAtIndex(idx)
		if gs != ws || ge != we {
			t.Errorf("wordAt(%d) = [%d,%d), want [%d,%d)", idx, gs, ge, ws, we)
		}
	}
	for y := float32(-10); y <= whole.Height+10; y += 10 {
		for x := float32(-5); x <= whole.Width+15; x += 5 {
			if got, want := b.closestOffset(x, y), whole.GetClosestOffset(x, y); got != want {
				t.Errorf("closestOffset(%v,%v) = %d, want %d", x, y, got, want)
			}
		}
	}
	ranges := [][2]int{{0, len(text)}, {1, len(text) - 1}, {3, 7}, {len(text) / 2, len(text)}}
	for _, r := range ranges {
		if r[0] >= r[1] {
			continue
		}
		got := b.selectionRects(r[0], r[1], -1e9, 1e9)
		want := whole.GetSelectionRects(r[0], r[1])
		sortRects := func(rs []glyph.Rect) {
			slices.SortFunc(rs, func(a, c glyph.Rect) int {
				if a.Y != c.Y {
					if a.Y < c.Y {
						return -1
					}
					return 1
				}
				if a.X < c.X {
					return -1
				}
				if a.X > c.X {
					return 1
				}
				return 0
			})
		}
		sortRects(got)
		sortRects(want)
		if !slices.Equal(got, want) {
			t.Errorf("selectionRects(%d,%d) = %v, want %v", r[0], r[1], got, want)
		}
	}
}

// hardWrapAt reports whether idx starts a line that begins exactly where
// the previous line ends, with no consumed byte between.
func hardWrapAt(l *glyph.Layout, idx int) bool {
	for k := 1; k < len(l.Lines); k++ {
		prev := l.Lines[k-1]
		if l.Lines[k].StartIndex == idx && prev.StartIndex+prev.Length == idx {
			return true
		}
	}
	return false
}

// checkBlockInvariants asserts the chunk list covers the text: in
// order, starting at 0 and ending at the text end, each seam either a
// separator byte the chunks leave out or a piece boundary.
func checkBlockInvariants(t *testing.T, b *textBlock) {
	t.Helper()
	text := b.text
	if len(b.chunks) == 0 {
		t.Fatal("no chunks")
	}
	if b.chunks[0].start != 0 {
		t.Fatalf("first chunk starts at %d", b.chunks[0].start)
	}
	if last := b.chunks[len(b.chunks)-1]; last.end != len(text) {
		t.Fatalf("last chunk ends at %d, text is %d bytes", last.end, len(text))
	}
	for i, c := range b.chunks {
		if c.start > c.end || c.end > len(text) {
			t.Fatalf("chunk %d has range [%d,%d)", i, c.start, c.end)
		}
		if i == 0 {
			continue
		}
		p := b.chunks[i-1]
		switch {
		case c.start == p.end+1 && text[p.end] == '\n':
			if c.piece {
				t.Fatalf("chunk %d after a newline is marked piece", i)
			}
		case c.start == p.end+1 && b.wraps() && isBlankByte(text[p.end]):
			if !c.piece {
				t.Fatalf("chunk %d after a consumed space is not a piece", i)
			}
		case c.start == p.end:
			if !c.piece {
				t.Fatalf("chunk %d abuts chunk %d but is not a piece", i, i-1)
			}
		default:
			t.Fatalf("chunk %d [%d,%d) does not follow chunk %d [%d,%d)",
				i, c.start, c.end, i-1, p.start, p.end)
		}
	}
}

// TestTextBlockPatchMatchesRebuild drives random edits through update
// and checks the patched block against a block built from the result.
func TestTextBlockPatchMatchesRebuild(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	rng := rand.New(rand.NewSource(1))
	alphabet := []string{"a", "b", " ", "\n", "xy", "\n\n", "word "}
	for _, wrapW := range []float32{0, 100} {
		mode := TextModeMultiline
		if wrapW > 0 {
			mode = TextModeWrap
		}
		text := "the quick brown fox\njumps over\n\nthe lazy dog and keeps running"
		b := newTextBlock(text, wrapW, blockTestStyle, mode, w)
		for step := range 300 {
			pos := rng.Intn(len(text) + 1)
			var next string
			switch rng.Intn(3) {
			case 0: // insert
				next = text[:pos] + alphabet[rng.Intn(len(alphabet))] + text[pos:]
			case 1: // delete a few bytes
				end := min(len(text), pos+1+rng.Intn(4))
				next = text[:pos] + text[end:]
			default: // replace a span
				end := min(len(text), pos+rng.Intn(6))
				next = text[:pos] + alphabet[rng.Intn(len(alphabet))] + text[end:]
			}
			b.update(next, wrapW, blockTestStyle, mode, w)
			text = next
			if b.text != text {
				t.Fatalf("step %d: block text drifted", step)
			}
			blockEnsureAll(b, w)
			// A patch re-chunks only the paragraphs the edit touched, so
			// the packing may differ from a fresh build; the invariants
			// and the geometry may not.
			checkBlockInvariants(t, b)
			fresh := newTextBlock(text, wrapW, blockTestStyle, mode, w)
			blockEnsureAll(fresh, w)
			if b.height != fresh.height || b.width != fresh.width {
				t.Fatalf("step %d (%q): extent %v x %v, fresh %v x %v",
					step, text, b.width, b.height, fresh.width, fresh.height)
			}
			whole := stubLayout(text, wrapW)
			if text != "" {
				compareBlockToWhole(t, b, &whole, text, w)
			}
			if t.Failed() {
				t.Fatalf("after step %d, text %q", step, text)
			}
		}
	}
}

func TestTextBlockEvictsLiveLayouts(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	text := strings.Repeat("abc def\n", 200)
	b := newTextBlock(text, 100, blockTestStyle, TextModeWrap, w)
	// Built eagerly, so every chunk is measured; the shaped set is bounded.
	for i := range b.chunks {
		if !b.chunks[i].measured {
			t.Fatalf("chunk %d not measured", i)
		}
	}
	if b.live > textBlockLiveLayouts {
		t.Fatalf("live = %d after build, want <= %d", b.live, textBlockLiveLayouts)
	}
	for idx := 0; idx <= len(text); idx += 7 {
		b.cursorPos(idx)
	}
	live := 0
	for i := range b.chunks {
		if b.chunks[i].layout != nil {
			live++
		}
	}
	if live != b.live || live > textBlockLiveLayouts {
		t.Fatalf("live = %d (counted %d), want <= %d", b.live, live, textBlockLiveLayouts)
	}
}

func TestTextBlockLargeTextEstimatesThenMeasures(t *testing.T) {
	w := newBlockTestWindow(t)
	text := strings.Repeat(strings.Repeat("x", 39)+"\n", 3000) // 120,000 bytes
	b := newTextBlock(text, 600, blockTestStyle, TextModeWrap, w)
	if len(text) <= textBlockEagerBytes {
		t.Fatal("text should be above the eager budget")
	}
	measured := 0
	for i := range b.chunks {
		if b.chunks[i].measured && b.chunks[i].start < b.chunks[i].end {
			measured++
		}
	}
	if measured != 0 {
		t.Fatalf("%d chunks measured before any query", measured)
	}
	// 3000 lines of 20 px, estimated at one line per 39-byte paragraph,
	// plus the empty line after the trailing newline.
	if b.height != 3001*stubLH {
		t.Errorf("estimated height = %v, want %v", b.height, 3001*stubLH)
	}
	cp, ok := b.cursorPos(len(text))
	if !ok || cp.Y != 3000*stubLH {
		t.Errorf("caret at end = %+v %v", cp, ok)
	}
	if b.chunks[len(b.chunks)-1].measured == false {
		t.Error("the queried chunk was not measured")
	}
}

func TestTextBlockWidthChangeRebuilds(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	text := "dddd eeee ffff gggg hhhh"
	b := newTextBlock(text, 100, blockTestStyle, TextModeWrap, w)
	before := blockRanges(b)
	b.update(text, 200, blockTestStyle, TextModeWrap, w)
	after := blockRanges(b)
	if slices.Equal(before, after) {
		t.Fatalf("chunks unchanged by a width change: %v", after)
	}
	whole := stubLayout(text, 200)
	compareBlockToWhole(t, b, &whole, text, w)
}

func TestTextBlockDeleteGrapheme(t *testing.T) {
	withChunkBytes(t, 12)
	w := newBlockTestWindow(t)
	text := "aaaa bbbb cccc dddd"
	b := newTextBlock(text, 100, blockTestStyle, TextModeWrap, w)
	nt, pos, ok := b.deleteGrapheme(7, false)
	if !ok || nt != "aaaa bbb cccc dddd" || pos != 6 {
		t.Errorf("backspace at 7 = %q %d %v", nt, pos, ok)
	}
	nt, pos, ok = b.deleteGrapheme(7, true)
	if !ok || nt != "aaaa bbbb cccc dddd"[:7]+"b cccc dddd" || pos != 7 {
		t.Errorf("delete at 7 = %q %d %v", nt, pos, ok)
	}
	// Chunk edges decline, so the caller's string path takes over.
	first := b.chunks[0]
	if _, _, ok = b.deleteGrapheme(first.end, true); ok {
		t.Error("forward delete at a chunk end should decline")
	}
	if _, _, ok = b.deleteGrapheme(0, false); ok {
		t.Error("backspace at 0 should decline")
	}
}

func BenchmarkTextBlockKeystroke(b *testing.B) {
	w := NewTestWindow(b, WindowCfg{Width: 400, Height: 400})
	defer w.WindowCleanup()
	w.textMeasurer = blockStubMeasurer{}
	text := strings.Repeat("the quick brown fox jumps over the lazy dog\n", 2300) // ~100 KB
	blk := newTextBlock(text, 600, blockTestStyle, TextModeWrap, w)
	mid := len(text) / 2
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var next string
		if i%2 == 0 {
			next = text[:mid] + "x" + text[mid:]
		} else {
			next = text
		}
		blk.update(next, 600, blockTestStyle, TextModeWrap, w)
		blk.cursorPos(mid)
	}
}

// A chunk's layout keeps the text it was shaped from (glyph.Layout.Text).
// Shaped from a slice of the block's text, it would keep that whole text
// alive: after edits, every chunk an edit did not touch would pin the
// generation it was shaped in, up to one full copy per live layout.
func TestTextBlockLayoutsDoNotPinTheText(t *testing.T) {
	withChunkBytes(t, 64)
	w := newBlockTestWindow(t)
	text := strings.Repeat("abcd efgh ijkl\n", 40)
	b := newTextBlock(text, 600, blockTestStyle, TextModeWrap, w)
	edited := text[:30] + "X" + text[30:]
	b.update(edited, 600, blockTestStyle, TextModeWrap, w)
	blockEnsureAll(b, w)
	shaped := 0
	for _, txt := range []string{text, edited} {
		lo := uintptr(unsafe.Pointer(unsafe.StringData(txt)))
		hi := lo + uintptr(len(txt))
		for i := range b.chunks {
			l := b.chunks[i].layout
			if l == nil || l.Text == "" {
				continue
			}
			shaped++
			p := uintptr(unsafe.Pointer(unsafe.StringData(l.Text)))
			if p >= lo && p < hi {
				t.Errorf("chunk %d's layout points into a text the block held", i)
			}
		}
	}
	if shaped == 0 {
		t.Fatal("no chunk shaped")
	}
}

// A rebuild reuses the chunk slice; the slots past its new length must
// not keep the layouts of the chunks they held.
func TestTextBlockRebuildClearsDroppedChunks(t *testing.T) {
	withChunkBytes(t, 64)
	w := newBlockTestWindow(t)
	text := strings.Repeat("abcd efgh ijkl\n", 40)
	b := newTextBlock(text, 600, blockTestStyle, TextModeWrap, w)
	blockEnsureAll(b, w)
	n := len(b.chunks)
	b.update("short", 300, blockTestStyle, TextModeWrap, w) // width: rebuild
	if len(b.chunks) >= n {
		t.Fatalf("rebuild kept %d of %d chunks", len(b.chunks), n)
	}
	for i, c := range b.chunks[len(b.chunks):cap(b.chunks)] {
		if c.layout != nil {
			t.Errorf("dropped slot %d still holds a layout", len(b.chunks)+i)
		}
	}
}

// A block holds its whole text and up to textBlockLiveLayouts layouts.
// A text that leaves the tree drops its block within two view passes,
// rather than staying until capTextBlocks other identities push it out;
// a text that stays keeps its block.
func TestTextBlockCacheDropsTextsThatLeft(t *testing.T) {
	w := newBlockTestWindow(t)
	big := strings.Repeat("abcd efgh ijkl\n", 400)
	show := true
	view := func(*Window) View {
		content := []View{Text(TextCfg{ID: "small", Text: "x y", Mode: TextModeWrap})}
		if show {
			content = append(content,
				Text(TextCfg{ID: "big", Text: big, Mode: TextModeWrap}))
		}
		return Column(ContainerCfg{Sizing: FillFill, Content: content})
	}
	holding := func() (bigBlocks int, small *textBlock) {
		m := StateMapRead[string, *textBlock](w, nsTextBlock)
		if m == nil {
			return 0, nil
		}
		m.Range(func(_ string, b *textBlock) bool {
			switch b.text {
			case big:
				bigBlocks++
			case "x y":
				small = b
			}
			return true
		})
		return bigBlocks, small
	}
	w.TestRender(view)
	n, small := holding()
	if n != 1 || small == nil {
		t.Fatalf("cache holds %d blocks of the big text and small %v", n, small != nil)
	}
	show = false
	for range 3 {
		w.TestRender(nil)
	}
	n, again := holding()
	if n != 0 {
		t.Errorf("a text that left the tree still holds %d cached blocks", n)
	}
	if again != small {
		t.Error("a text that stayed lost its block")
	}
}
