package gui

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// Large text starts with estimated chunk heights. These tests make the
// estimate wrong on purpose: the stub shaper wraps at spaces, so a line
// of the 300 px views below holds one 15-letter word, where the estimate
// fills every line to the width; with the eager budget at zero every
// chunk starts estimated, about 60 % of its height.

// withEagerBytes lowers the eager-measure budget for one test.
func withEagerBytes(t *testing.T, n int) {
	t.Helper()
	old := textBlockEagerBytes
	textBlockEagerBytes = n
	t.Cleanup(func() { textBlockEagerBytes = old })
}

func anchorTestText(paras int) string {
	word := strings.Repeat("w", 15)
	var b strings.Builder
	for i := range paras {
		fmt.Fprintf(&b, "p%04d", i)
		for range 8 {
			b.WriteString(" " + word)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// chunkScreenYs maps each drawn chunk's text to the y it was drawn at.
func chunkScreenYs(w *Window) map[string]float32 {
	out := map[string]float32{}
	for _, r := range w.Renderers() {
		if r.Kind == RenderLayout && r.LayoutPtr != nil {
			out[r.Text] = r.Y
		}
	}
	return out
}

// anchorLongText is anchorTestText with paragraphs longer than a 256-byte
// chunk budget, so each starts as an open chunk that measuring cuts into
// pieces. Every word is distinct, so every piece is too.
func anchorLongText(paras, words int) string {
	var b strings.Builder
	for i := range paras {
		fmt.Fprintf(&b, "p%04d", i)
		for k := range words {
			fmt.Fprintf(&b, " q%05dx%08d", i, k)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestTextAnchorWheelUpKeepsTextInPlace(t *testing.T) {
	withChunkBytes(t, 256)
	withEagerBytes(t, 0)
	t.Run("short paragraphs", func(t *testing.T) {
		wheelUpKeepsTextInPlace(t, anchorTestText(200))
	})
	// Measuring an open chunk inserts its pieces before the anchor.
	t.Run("paragraphs past the chunk budget", func(t *testing.T) {
		wheelUpKeepsTextInPlace(t, anchorLongText(40, 40))
	})
}

func wheelUpKeepsTextInPlace(t *testing.T, text string) {
	w := newBlockTestWindow(t)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "sc", Scrollable: true,
			Sizing: FixedFixed, Width: 300, Height: 200, SizeBorder: NoBorder,
			Content: []View{
				Text(TextCfg{ID: "big", Text: text, Mode: TextModeWrap}),
				Text(TextCfg{ID: "below", Text: "below"}),
			}})
	})
	w.ScrollVerticalToPct("sc", 1)
	w.TestRender(nil)

	const step = 2 // wheel units; 20 px each
	corrected := 0
	for range 2000 {
		before := chunkScreenYs(w)
		_, off0, _ := w.TestScrollOffset("sc")
		if err := w.TestScroll("sc", 0, step); err != nil {
			t.Fatal(err)
		}
		_, off1, _ := w.TestScrollOffset("sc")
		if off1 >= 0 {
			break // reached the top: the clamp shortens the step
		}
		if off1-off0 != step*20 {
			corrected++
		}
		after := chunkScreenYs(w)
		kept := 0
		for txt, y0 := range before {
			y1, ok := after[txt]
			if !ok {
				continue
			}
			kept++
			if y1-y0 != step*20 {
				t.Fatalf("text in view moved %v px for a %v px wheel step (offset %v -> %v)",
					y1-y0, step*20, off0, off1)
			}
		}
		// A step much shorter than the view keeps some of it on screen;
		// none left means the text jumped further than the view.
		if kept == 0 && len(before) > 0 {
			t.Fatalf("nothing drawn before a %v px wheel step is drawn after it (offset %v -> %v)",
				step*20, off0, off1)
		}
		ly := w.TestRender(nil)
		big, ok1 := ly.FindByID(ScopeID("sc", "big"))
		below, ok2 := ly.FindByID(ScopeID("sc", "below"))
		if !ok1 || !ok2 {
			t.Fatal("text shapes not found")
		}
		if below.Shape.Y < big.Shape.Y+big.Shape.Height {
			t.Fatalf("the view below the text overlaps it: %v < %v",
				below.Shape.Y, big.Shape.Y+big.Shape.Height)
		}
	}
	// The estimates were wrong, so the offset had to absorb measured
	// height; a run with no correction tested nothing.
	if corrected == 0 {
		t.Fatal("no scroll correction happened; the estimates were right")
	}
	t.Logf("%d corrections", corrected)
}

// The text grows when its visible chunks are measured; the boxes that fit
// it, the view below it and the scroll range follow in the same frame, so
// that frame matches the next one, laid out from the measured heights.
func TestTextAnchorGrowthReachesLaterSiblings(t *testing.T) {
	withChunkBytes(t, 256)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	text := anchorTestText(200)
	view := func(*Window) View {
		return Column(ContainerCfg{ID: "sc", Scrollable: true,
			Sizing: FixedFixed, Width: 300, Height: 200, SizeBorder: NoBorder,
			Content: []View{
				Column(ContainerCfg{Sizing: FillFit, SizeBorder: NoBorder, Content: []View{
					Text(TextCfg{ID: "big", Text: text, Mode: TextModeWrap}),
				}}),
				Text(TextCfg{ID: "below", Text: "below"}),
			}})
	}
	type frame struct{ bigY, bigH, belowY, content float32 }
	grab := func(ly *Layout) frame {
		t.Helper()
		sc, ok0 := ly.FindByID("sc")
		big, ok1 := ly.FindByID(ScopeID("sc", "big"))
		below, ok2 := ly.FindByID(ScopeID("sc", "below"))
		if !ok0 || !ok1 || !ok2 {
			t.Fatal("shapes not found")
		}
		if got, want := contentHeight(sc), computeContentHeight(sc); got != want {
			t.Errorf("cached content height %v, children add up to %v", got, want)
		}
		return frame{big.Shape.Y, big.Shape.Height, below.Shape.Y, contentHeight(sc)}
	}
	top := grab(w.TestRender(view))
	w.ScrollVerticalToPct("sc", 1)
	// One pass of the pipeline, without the second pass TestRender may
	// add, so what is checked is the frame the anchor pass produced.
	w.markLayoutRefresh(refreshTest)
	w.Update()
	anchored := grab(&w.layout)
	if anchored.bigH == top.bigH {
		t.Fatal("measuring the chunks at the end grew nothing; the estimates were right")
	}
	next := grab(w.TestRender(nil))
	if anchored != next {
		t.Errorf("anchored frame %+v, next frame %+v", anchored, next)
	}
}

// A selection delete in a large field leaves the caret where the
// selection began, drawn at the same place, and does not scroll: the text
// above it did not change. A paragraph above the caret re-estimated by the
// edit would move it by the estimate's error.
func TestTextAnchorDeleteKeepsCaretInPlace(t *testing.T) {
	withChunkBytes(t, 2048)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	text := anchorTestText(300)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, SizeBorder: NoBorder,
			Content: []View{
				Input(InputCfg{ID: "f", Text: text, Mode: InputMultiline,
					Scrollable: true, Sizing: FixedFixed, Width: 300, Height: 200,
					OnTextChanged: func(s string, _ EventCtx) { text = s }}),
			}})
	})
	caret := func(trial int) RenderCmd {
		t.Helper()
		if !w.caretCmd.ok {
			t.Fatalf("trial %d: no caret drawn", trial)
		}
		return w.renderers[w.caretCmd.idx]
	}
	rng := rand.New(rand.NewSource(7))
	for trial := range 40 {
		_ = w.TestScroll("f", 0, float32(rng.Intn(400)-200))
		if err := w.TestClick("f"); err != nil {
			t.Fatal(err)
		}
		start := caret(trial)
		_, offStart, _ := w.TestScrollOffset("f")
		for range 1 + rng.Intn(3) {
			_ = w.TestKey("f", KeyDown, ModShift)
		}
		_, offSel, _ := w.TestScrollOffset("f")
		key := KeyDelete
		if rng.Intn(2) == 0 {
			key = KeyBackspace
		}
		_ = w.TestKey("f", key, ModNone)
		_, offDel, _ := w.TestScrollOffset("f")
		got := caret(trial)
		wantY := start.Y + (offSel - offStart)
		if offDel != offSel || got.X != start.X || got.Y != wantY {
			t.Fatalf("trial %d: caret drawn at (%v, %v), selection began at (%v, %v); scroll %v -> %v",
				trial, got.X, got.Y, start.X, wantY, offSel, offDel)
		}
	}
}

// Typing at the end of the last paragraph of a full chunk grows that
// paragraph out of the chunk: the edit packs the paragraphs above it into
// a chunk of their own. Were that chunk estimated, the caret would move by
// the estimate's error. Typing only moves the caret right, or down a line.
// Paragraphs follow the chunk, so the end-of-text clamp cannot hide a
// jump.
func TestTextAnchorTypingKeepsLineInPlace(t *testing.T) {
	withChunkBytes(t, 2048)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	at := len(anchorTestText(15)) - 1 // end of the chunk's last paragraph
	if at > textChunkBytes || at+80 <= textChunkBytes {
		t.Fatalf("a chunk of %d bytes must spill when typed into", at)
	}
	text := anchorTestText(45)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, SizeBorder: NoBorder,
			Content: []View{
				Input(InputCfg{ID: "f", Text: text, Mode: InputMultiline,
					Scrollable: true, Sizing: FixedFixed, Width: 300, Height: 200,
					OnTextChanged: func(s string, _ EventCtx) { text = s }}),
			}})
	})
	field, ok := w.TestRender(nil).FindByID("f")
	if !ok {
		t.Fatal("field not found")
	}
	top, bot := field.Shape.Y, field.Shape.Y+field.Shape.Height
	w.SetFocus("f")
	StateMap[string, inputState](w, nsInput, capMany).Set(
		"f", inputState{CursorPos: at})
	_ = w.TestKey("f", KeyEnd, ModNone) // scrolls the caret into view
	if !w.caretCmd.ok {
		t.Fatal("no caret drawn")
	}
	prev := w.renderers[w.caretCmd.idx]
	if prev.Y < top || prev.Y+prev.H > bot {
		t.Fatalf("caret drawn at y %v before typing, view %v..%v", prev.Y, top, bot)
	}
	for i := range 80 {
		if err := w.TestType("f", string(rune('a'+i%26))); err != nil {
			t.Fatal(err)
		}
		if !w.caretCmd.ok {
			t.Fatalf("char %d: no caret drawn", i)
		}
		c := w.renderers[w.caretCmd.idx]
		if c.Y < prev.Y || c.Y < top || c.Y+c.H > bot {
			t.Fatalf("char %d: caret drawn at y %v, before at %v, view %v..%v",
				i, c.Y, prev.Y, top, bot)
		}
		prev = c
	}
	ly := w.TestRender(nil)
	f, _ := ly.FindByID("f")
	chunks := 0
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if l.Shape.TC != nil && l.Shape.TC.block != nil {
			chunks = max(chunks, len(l.Shape.TC.block.chunks))
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(f)
	if chunks < 2 {
		t.Fatal("the typed paragraph never left its chunk; the test tested nothing")
	}
}

// An edit shapes the chunks it re-packs up to the edit. A caret query
// shapes only the caret's own chunk, and an edit the app makes runs none,
// so an estimate left above the edit would be measured by the frame that
// draws it and move the text below by its error. Inserted text after the
// edit point stays estimated, so a large paste costs what it shows.
func TestTextBlockPatchShapesItsRegion(t *testing.T) {
	withChunkBytes(t, 2048)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	at := len(anchorTestText(15)) - 1 // end of the first chunk's last paragraph
	text := anchorTestText(45)
	b := newTextBlock(text, 244, blockTestStyle, TextModeWrap, w)
	if len(b.chunks) < 3 {
		t.Fatalf("want several chunks, got %v", blockRanges(b))
	}
	// The grown paragraph no longer fits its chunk, so the edit packs the
	// paragraphs above it into a chunk of their own.
	edited := text[:at] + strings.Repeat(" xx", 30) + text[at:]
	b.update(edited, 244, blockTestStyle, TextModeWrap, w)
	if b.chunkAt(at) == 0 {
		t.Fatalf("the edit did not re-pack the chunk: %v", blockRanges(b))
	}
	for i := 0; i <= b.chunkAt(at); i++ {
		if c := &b.chunks[i]; c.needsLayout() {
			t.Errorf("chunk [%d,%d) of the edited region is still estimated", c.start, c.end)
		}
	}
	whole := newTextBlock(edited, 244, blockTestStyle, TextModeWrap, w)
	blockEnsureAll(whole, w)
	b.settle()
	if got, want := b.chunks[b.chunkAt(at)].y, whole.chunks[whole.chunkAt(at)].y; got != want {
		t.Errorf("edited paragraph at y %v, measured from scratch at %v", got, want)
	}

	paste := anchorTestText(800) // about 100 KB, 50 chunks
	b.update(edited[:at]+paste+edited[at:], 244, blockTestStyle, TextModeWrap, w)
	shaped := 0
	for i := b.chunkAt(at) + 1; i < len(b.chunks) && b.chunks[i].start < at+len(paste); i++ {
		if !b.chunks[i].needsLayout() {
			shaped++
		}
	}
	if shaped > 1 {
		t.Errorf("a paste of %d bytes shaped %d of its chunks", len(paste), shaped)
	}
}

// Walking the caret up from the middle of a large field, where the
// scrollbar dropped it into text never shown, scrolls a row at a time.
// A chunk measured only when the caret enters it would push everything
// below it down by its estimate's error.
func TestTextAnchorCaretWalkScrollsByRows(t *testing.T) {
	withChunkBytes(t, 2048)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	text := anchorTestText(300)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, SizeBorder: NoBorder,
			Content: []View{
				Input(InputCfg{ID: "f", Text: text, Mode: InputMultiline,
					Scrollable: true, Sizing: FixedFixed, Width: 300, Height: 200,
					OnTextChanged: func(s string, _ EventCtx) { text = s }}),
			}})
	})
	field, ok := w.TestRender(nil).FindByID("f")
	if !ok {
		t.Fatal("field not found")
	}
	top, bot := field.Shape.Y, field.Shape.Y+field.Shape.Height
	w.ScrollVerticalToPct("f", 0.5)
	w.TestRender(nil)
	if err := w.TestClick("f"); err != nil {
		t.Fatal(err)
	}
	keys := 0
	walk := func(key KeyCode, n int) {
		t.Helper()
		for range n {
			before := chunkScreenYs(w)
			_ = w.TestKey("f", key, ModNone)
			keys++
			if !w.caretCmd.ok {
				t.Fatalf("key %d: no caret drawn", keys)
			}
			c := w.renderers[w.caretCmd.idx]
			if c.Y < top || c.Y+c.H > bot {
				t.Fatalf("key %d: caret drawn at y %v, view %v..%v", keys, c.Y, top, bot)
			}
			after := chunkScreenYs(w)
			for txt, y0 := range before {
				if y1, ok := after[txt]; ok {
					if d := y1 - y0; d > 1.5*stubLH || d < -1.5*stubLH {
						t.Fatalf("key %d: the text in view moved %v px", keys, d)
					}
				}
			}
		}
	}
	walk(KeyUp, 300)
	walk(KeyLeft, 300)
}

// The anchor's scroll view is the nearest vertical scroller inside the
// text's own layer, reached through fit-height columns only.
func TestTextScrollViewWalk(t *testing.T) {
	type box struct {
		float, scroll bool
		mode          scrollMode
		id            string
		sizing        sizingType
		axis          Axis
	}
	// build makes a chain from the outermost box to a fit-height text.
	build := func(boxes ...box) *Layout {
		root := &Layout{}
		cur := root
		for i, b := range boxes {
			cur.Shape = &Shape{Float: b.float, Scrollable: b.scroll, ScrollMode: b.mode,
				ID: b.id, Axis: b.axis}
			cur.Shape.Sizing.Height = b.sizing
			if i == len(boxes)-1 {
				break
			}
			cur.Children = []Layout{{Parent: cur}}
			cur = &cur.Children[0]
		}
		return cur
	}
	col := box{axis: axisTopToBottom}
	view := box{scroll: true, id: "sc", sizing: sizingFixed, axis: axisTopToBottom}
	text := box{}
	tests := []struct {
		name    string
		chain   []box
		found   bool
		follows bool
	}{
		{"direct child", []box{view, text}, true, true},
		{"through a fit column", []box{view, col, text}, true, true},
		{"through a fixed column", []box{view, {axis: axisTopToBottom, sizing: sizingFixed}, text}, true, false},
		{"through a row", []box{view, {axis: axisLeftToRight}, text}, true, false},
		{"float between", []box{view, {float: true, axis: axisTopToBottom}, text}, false, false},
		{"float text", []box{view, {float: true}}, false, false},
		{"horizontal-only scroller is a box", []box{view, {scroll: true, id: "h", mode: ScrollHorizontalOnly, axis: axisTopToBottom}, text}, true, true},
		{"scroller without an id", []box{{scroll: true, axis: axisTopToBottom}, text}, false, false},
	}
	for _, tt := range tests {
		ly := build(tt.chain...)
		sc, follows := textScrollView(ly)
		if (sc != nil) != tt.found || follows != tt.follows {
			t.Errorf("%s: found %v follows %v, want %v %v", tt.name, sc != nil, follows, tt.found, tt.follows)
		}
		if sc != nil && sc.Shape.ID != "sc" {
			t.Errorf("%s: found %q, want sc", tt.name, sc.Shape.ID)
		}
	}
}

// A height limit stops the text's growth from reaching the scroll view.
func TestTextAnchorRoom(t *testing.T) {
	tests := []struct {
		name             string
		h, minH, maxH, d float32
		want             bool
	}{
		{"no limits", 100, 0, 0, 50, true},
		{"above a derived minimum", 100, 28, 0, 50, true},
		{"shrinks to the minimum", 100, 28, 0, -72, true},
		{"would fall below the minimum", 100, 28, 0, -80, false},
		{"held at its minimum", 28, 28, 0, 50, false},
		{"under its maximum", 100, 0, 200, 50, true},
		{"would pass its maximum", 100, 0, 120, 50, false},
	}
	for _, tt := range tests {
		s := &Shape{Height: tt.h, MinHeight: tt.minH, MaxHeight: tt.maxH}
		if got := textAnchorRoom(s, tt.d); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

// tallStubMeasurer shapes every chunk twice as tall as the block's
// estimate, which it takes from shaping "x", so a text that does not wrap
// is estimated wrong too.
type tallStubMeasurer struct{ blockStubMeasurer }

func (tallStubMeasurer) LayoutText(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	l, err := blockStubMeasurer{}.LayoutText(text, style, wrapWidth)
	if text != "x" {
		l.Height *= 2
	}
	return l, err
}

// A single-line text keeps a one-line box whatever its chunks measure
// (layoutPlainText), so the anchor pass leaves it alone: it neither grows
// the box by the measured paragraphs, which would move the views below it
// for a frame, nor shapes rows the box never shows.
func TestTextAnchorLeavesSingleLineText(t *testing.T) {
	withChunkBytes(t, 256)
	withEagerBytes(t, 0)
	w := newBlockTestWindow(t)
	w.textMeasurer = tallStubMeasurer{}
	text := anchorTestText(100) // paragraphs, as a paste would leave them
	spacer := func() View {
		return Column(ContainerCfg{Sizing: FixedFixed, Width: 10, Height: 1000,
			SizeBorder: NoBorder})
	}
	view := func(*Window) View {
		return Column(ContainerCfg{ID: "sc", Scrollable: true,
			Sizing: FixedFixed, Width: 300, Height: 200, SizeBorder: NoBorder,
			Content: []View{
				spacer(),
				Text(TextCfg{ID: "one", Text: text}),
				Text(TextCfg{ID: "below", Text: "below"}),
				spacer(),
			}})
	}
	type frame struct {
		oneH, belowY float32
		measured     int
	}
	grab := func(ly *Layout) frame {
		t.Helper()
		one, ok1 := ly.FindByID(ScopeID("sc", "one"))
		below, ok2 := ly.FindByID(ScopeID("sc", "below"))
		if !ok1 || !ok2 {
			t.Fatal("shapes not found")
		}
		blk := one.Shape.TC.block
		if blk == nil || len(blk.chunks) < 2 {
			t.Fatal("the single-line text is not laid out in chunks")
		}
		measured := 0
		for i := range blk.chunks {
			if blk.chunks[i].measured {
				measured++
			}
		}
		return frame{one.Shape.Height, below.Shape.Y - one.Shape.Y, measured}
	}
	first := grab(w.TestRender(view))
	// The view a little below the text: its rows there were never drawn.
	w.ScrollVerticalToPct("sc", 0.7)
	w.markLayoutRefresh(refreshTest)
	w.Update()
	if got := grab(&w.layout); got != first {
		t.Errorf("anchored frame %+v, the text's one-line frame %+v", got, first)
	}
}
