package gui

// markdown_select_interaction_test.go drives the cross-block selection
// cluster (markdown_select.go) through the real pipeline: a focusable
// markdown document rendered as the whole window, with a stub text
// measurer that shapes every RTF block into a deterministic glyph
// layout so clicks map to exact runes. The sibling file
// (markdown_select_test.go) covers the pure helpers; anything that
// needs a real layout tree, event dispatch or the mouse lock lives
// here.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// mdSelSource mixes the three RTF-producing block kinds: a heading, a
// paragraph and a two-item list. Rune offsets accumulate across them:
// heading "Heading" (7), paragraph "First paragraph text" (20), then
// "item one" (8) and "item two" (8), for 43 runes total.
const mdSelSource = "# Heading\n\nFirst paragraph text\n\n- item one\n- item two"

// mdWordBoundaryLogAttrs marks word starts (first non-space byte after
// a space) and word ends (the first space after a word, plus the end
// of text), mirroring the convention glyph's shaping path uses so
// GetWordAtIndex behaves like a real layout.
func mdWordBoundaryLogAttrs(text string) (map[int]int, []glyph.LogAttr) {
	byIndex := map[int]int{}
	var attrs []glyph.LogAttr
	add := func(i int, start, end bool) {
		byIndex[i] = len(attrs)
		attrs = append(attrs, glyph.LogAttr{
			IsWordStart: start,
			IsWordEnd:   end,
		})
	}
	isSpace := func(i int) bool {
		return text[i] == ' ' || text[i] == '\t'
	}
	for i := 0; i < len(text); i++ {
		prevSpace := i == 0 || isSpace(i-1)
		if !isSpace(i) && prevSpace {
			add(i, true, false)
		}
		if isSpace(i) && !prevSpace {
			add(i, false, true)
		}
	}
	if len(text) > 0 && !isSpace(len(text)-1) {
		add(len(text), false, true)
	}
	return byIndex, attrs
}

// mdCharLayoutForText builds a single-line glyph layout with one 10x20
// character rect per BYTE (ASCII sources in these tests), word-boundary
// log attributes, and a single line spanning the text. GetClosestOffset
// and GetWordAtIndex — the two queries the selection code relies on —
// work off CharRects/Lines and LogAttrs respectively, so this is a
// complete stand-in for a real shaped layout.
func mdCharLayoutForText(text string) glyph.Layout {
	n := len(text)
	charRects := make([]glyph.CharRect, 0, n)
	charRectByIndex := make(map[int]int, n)
	for i := range n {
		charRectByIndex[i] = i
		charRects = append(charRects, glyph.CharRect{
			Rect:  glyph.Rect{X: float32(i) * 10, Y: 0, Width: 10, Height: 20},
			Index: i,
		})
	}
	byIndex, attrs := mdWordBoundaryLogAttrs(text)
	return glyph.Layout{
		Text:            text,
		CharRects:       charRects,
		CharRectByIndex: charRectByIndex,
		LogAttrByIndex:  byIndex,
		LogAttrs:        attrs,
		Lines: []glyph.Line{{
			StartIndex: 0,
			Length:     n,
			Rect:       glyph.Rect{X: 0, Y: 0, Width: float32(n) * 10, Height: 20},
		}},
		Width:  float32(n) * 10,
		Height: 20,
	}
}

// mdSelectTestMeasurer implements TextMeasurer (plus the optional
// LayoutRichText capability) by shaping every rich text block into a
// deterministic single-line char layout. With nil backends the layout
// pipeline would leave rTFLayout empty, GetClosestOffset would always
// return 0, and every click would land on the block's first rune —
// useless for selection tests.
type mdSelectTestMeasurer struct{}

func (mdSelectTestMeasurer) TextWidth(text string, style TextStyle) float32 {
	return float32(len(text)) * style.Size * 0.5
}

func (mdSelectTestMeasurer) TextHeight(_ string, style TextStyle) float32 {
	return style.Size * 1.2
}

func (mdSelectTestMeasurer) FontAscent(style TextStyle) float32 {
	return style.Size * 0.8
}

func (mdSelectTestMeasurer) FontHeight(style TextStyle) float32 {
	return style.Size * 1.2
}

func (mdSelectTestMeasurer) LayoutText(
	_ string, style TextStyle, _ float32,
) (glyph.Layout, error) {
	return glyph.Layout{Height: style.Size * 1.2}, nil
}

// LayoutRichText concatenates the run texts — the same flat text the
// shape carries in rTFFlatText — and shapes it.
func (mdSelectTestMeasurer) LayoutRichText(
	rt glyph.RichText, _ glyph.TextConfig,
) (glyph.Layout, error) {
	var b strings.Builder
	for _, r := range rt.Runs {
		b.WriteString(r.Text)
	}
	return mdCharLayoutForText(b.String()), nil
}

// mdSelectHarness renders one focusable markdown document and exposes
// block lookup and press/move/release helpers. The document is written
// with the leaf ID "md" and, when nested, sits under Column{ID:"panel"}
// — the arrangement that resolves its effective ID to "panel:md".
// Rendering the document as the whole window (or the whole panel
// content) keeps block coordinates equal to window coordinates; the
// #274 regression tests push it sideways on purpose.
type mdSelectHarness struct {
	w      *Window
	id     string // effective markdown ID: "md" flat, "panel:md" nested
	source string
	nested bool
}

func newMdSelectHarness(t *testing.T) *mdSelectHarness {
	t.Helper()
	h := &mdSelectHarness{
		w:      NewTestWindow(WindowCfg{Width: 800, Height: 800}),
		id:     "md",
		source: mdSelSource,
	}
	h.w.textMeasurer = mdSelectTestMeasurer{}
	h.render()
	return h
}

func newMdSelectHarnessNested(t *testing.T) *mdSelectHarness {
	t.Helper()
	h := &mdSelectHarness{
		w:      NewTestWindow(WindowCfg{Width: 800, Height: 800}),
		id:     "panel:md",
		source: mdSelSource,
		nested: true,
	}
	h.w.textMeasurer = mdSelectTestMeasurer{}
	h.render()
	return h
}

// render installs the view generator. The generator reads h.source
// every frame, so changing the source and calling TestRender(nil) is
// how a live document is swapped between frames — TestRender(nil)
// re-runs the installed generator without clearing the registry (a
// fresh generator would go through UpdateView, which clears per-widget
// state by design and would mask the signature reset under test).
func (h *mdSelectHarness) render() {
	h.w.TestRender(func(win *Window) View {
		md := win.Markdown(MarkdownCfg{
			ID:        "md",
			Source:    h.source,
			Style:     DefaultMarkdownStyle(),
			Focusable: true,
		})
		if h.nested {
			return Column(ContainerCfg{
				ID:     "panel",
				Sizing: FillFill,
				Content: []View{
					md,
				},
			})
		}
		return md
	})
}

// blocks returns the nsMdBlocks list markdownContainerAmendLayout wrote
// on the last frame.
func (h *mdSelectHarness) blocks(t *testing.T) []mdBlockInfo {
	t.Helper()
	bm := StateMap[string, []mdBlockInfo](h.w, nsMdBlocks, capMany)
	blocks := bm.GetOr(h.id, nil)
	if len(blocks) == 0 {
		t.Fatal("no markdown blocks recorded by amend")
	}
	return blocks
}

// blockByText resolves a block by its flat text, so tests can address
// runes without hardcoding parse-dependent offsets.
func (h *mdSelectHarness) blockByText(t *testing.T, text string) mdBlockInfo {
	t.Helper()
	for _, b := range h.blocks(t) {
		if b.FlatText == text {
			return b
		}
	}
	t.Fatalf("no block with flat text %q", text)
	return mdBlockInfo{}
}

// runePoint returns the window coordinate of the middle of localRune
// in block b. The char rects start at the shape origin (X=0 in this
// harness), so shapeX + byte*10 + 5 is the rune's center.
func runePoint(b mdBlockInfo, localRune int) (float32, float32) {
	byteIdx := runeToByteIndex(b.FlatText, localRune)
	return b.ShapeX + float32(byteIdx)*10 + 5, b.ShapeY + 10
}

func (h *mdSelectHarness) press(x, y float32) Event {
	e := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	h.w.EventFn(&e)
	h.w.settle()
	return e
}

func (h *mdSelectHarness) move(x, y float32) Event {
	e := Event{
		Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x, MouseY: y,
	}
	h.w.EventFn(&e)
	h.w.settle()
	return e
}

func (h *mdSelectHarness) release(x, y float32) Event {
	e := Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	h.w.EventFn(&e)
	h.w.settle()
	return e
}

func (h *mdSelectHarness) selState() mdSelState {
	return StateReadOr(h.w, nsMdSel, h.id, mdSelState{})
}

// --- AmendLayout block collection ---

// TestMarkdownAmendPopulatesBlockList asserts the amend pass records
// one mdBlockInfo per RTF block in Y order, with rune offsets that
// accumulate across blocks (the virtual flat-text coordinates selection
// keys on).
func TestMarkdownAmendPopulatesBlockList(t *testing.T) {
	h := newMdSelectHarness(t)
	blocks := h.blocks(t)

	wantTexts := []string{
		"Heading", "First paragraph text", "item one", "item two",
	}
	if len(blocks) != len(wantTexts) {
		t.Fatalf("block count = %d, want %d",
			len(blocks), len(wantTexts))
	}
	for i, b := range blocks {
		if b.FlatText != wantTexts[i] {
			t.Errorf("block[%d] text = %q, want %q",
				i, b.FlatText, wantTexts[i])
		}
		if b.RuneLen == 0 {
			t.Errorf("block[%d] (%q) has zero runes", i, b.FlatText)
		}
		if b.H <= 0 {
			t.Errorf("block[%d] (%q) has non-positive height %g",
				i, b.FlatText, b.H)
		}
		if i > 0 && b.ShapeY <= blocks[i-1].ShapeY {
			t.Errorf("blocks not ordered by Y: block[%d] y=%g, "+
				"block[%d] y=%g", i-1, blocks[i-1].ShapeY, i, b.ShapeY)
		}
	}

	if blocks[0].StartRune != 0 {
		t.Errorf("first block StartRune = %d, want 0",
			blocks[0].StartRune)
	}
	for i := 1; i < len(blocks); i++ {
		wantStart := blocks[i-1].StartRune + blocks[i-1].RuneLen
		if blocks[i].StartRune != wantStart {
			t.Errorf("block[%d] StartRune = %d, want %d (previous "+
				"block end)", i, blocks[i].StartRune, wantStart)
		}
	}
}

// TestMarkdownWalkBlocksSkipsNonSelectableBlocks renders a document
// whose non-RTF blocks (table, code, HR, image) outnumber the RTF ones
// and asserts only the paragraph is collected — and that it starts at
// rune 0, because non-RTF content consumes no rune offsets.
func TestMarkdownWalkBlocksSkipsNonSelectableBlocks(t *testing.T) {
	// The parser drops data-URL destinations (ImageSrc becomes ""),
	// which trips Image's path validation and logs; a real temp PNG
	// keeps the render quiet.
	f, err := os.CreateTemp(t.TempDir(), "md_skip_*.png")
	if err != nil {
		t.Fatal(err)
	}
	imgPath := f.Name()
	_ = f.Close()

	source := strings.Join([]string{
		"First paragraph",
		"",
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
		"",
		"```go",
		"code",
		"```",
		"",
		"---",
		"",
		"![alt](" + imgPath + ")",
		"",
	}, "\n")

	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return win.Markdown(MarkdownCfg{
			ID:        "md",
			Source:    source,
			Style:     DefaultMarkdownStyle(),
			Focusable: true,
		})
	})

	bm := StateMap[string, []mdBlockInfo](w, nsMdBlocks, capMany)
	blocks := bm.GetOr("md", nil)
	if len(blocks) != 1 {
		t.Fatalf("block count = %d, want 1 (table/code/HR/image "+
			"are not selectable)", len(blocks))
	}
	if blocks[0].FlatText != "First paragraph" {
		t.Errorf("collected text = %q, want %q",
			blocks[0].FlatText, "First paragraph")
	}
	if blocks[0].StartRune != 0 {
		t.Errorf("StartRune = %d, want 0: non-selectable blocks "+
			"must consume no rune offsets", blocks[0].StartRune)
	}
}

// --- Click and word selection ---

// TestMarkdownClickPlacesCursorAtRune asserts a single click collapses
// the selection to the clicked rune, focuses the markdown container and
// consumes the event.
func TestMarkdownClickPlacesCursorAtRune(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	abs := p.StartRune + 2 // 'r'
	x, y := runePoint(p, 2)
	e := h.press(x, y)
	h.release(x, y)

	st := h.selState()
	if st.SelBeg != abs || st.SelEnd != abs {
		t.Errorf("selection = [%d,%d), want [%d,%d]",
			st.SelBeg, st.SelEnd, abs, abs)
	}
	if !e.IsHandled {
		t.Error("click on a block was not consumed")
	}
	if got := h.w.FocusID(); got != "md" {
		t.Errorf("focus = %q, want %q (container must take focus)", got, "md")
	}
}

// --- Nested document (#273) ---

// TestMarkdownNestedAmendPopulatesBlockList asserts a markdown nested
// under an ID-bearing container collects its blocks under the
// *effective* ID: before the fix the container keyed on the resolved
// "panel:md" while the blocks were stamped with the raw "md", so the
// list came back empty and selection was dead.
func TestMarkdownNestedAmendPopulatesBlockList(t *testing.T) {
	h := newMdSelectHarnessNested(t)
	blocks := h.blocks(t)

	wantTexts := []string{
		"Heading", "First paragraph text", "item one", "item two",
	}
	if len(blocks) != len(wantTexts) {
		t.Fatalf("block count = %d, want %d",
			len(blocks), len(wantTexts))
	}
	if blocks[0].StartRune != 0 {
		t.Errorf("first block StartRune = %d, want 0",
			blocks[0].StartRune)
	}
	for i, b := range blocks {
		if b.FlatText != wantTexts[i] {
			t.Errorf("block[%d] text = %q, want %q",
				i, b.FlatText, wantTexts[i])
		}
		if i > 0 && b.StartRune !=
			blocks[i-1].StartRune+blocks[i-1].RuneLen {
			t.Errorf("block[%d] StartRune = %d, want previous end",
				i, b.StartRune)
		}
	}
	if _, ok := StateMapRead[string, []mdBlockInfo](
		h.w, nsMdBlocks).Get("md"); ok {
		t.Error("block list also stored under the raw leaf ID")
	}
}

// TestMarkdownNestedClickSelectsAndFocuses asserts the click state and
// focus both key on the effective ID under an ID-bearing ancestor.
func TestMarkdownNestedClickSelectsAndFocuses(t *testing.T) {
	h := newMdSelectHarnessNested(t)
	p := h.blockByText(t, "First paragraph text")

	abs := p.StartRune + 2
	x, y := runePoint(p, 2)
	h.press(x, y)
	h.release(x, y)

	st := h.selState()
	if st.SelBeg != abs || st.SelEnd != abs {
		t.Errorf("selection = [%d,%d), want [%d,%d]",
			st.SelBeg, st.SelEnd, abs, abs)
	}
	if got := h.w.FocusID(); got != "panel:md" {
		t.Errorf("focus = %q, want %q (effective ID)",
			got, "panel:md")
	}
}

func TestMarkdownNestedCtrlASelectsAll(t *testing.T) {
	h := newMdSelectHarnessNested(t)

	if err := h.w.TestKey("panel:md", KeyA, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	var total uint32
	for _, b := range h.blocks(t) {
		total += b.RuneLen
	}
	st := h.selState()
	if st.SelBeg != 0 || st.SelEnd != total {
		t.Errorf("Ctrl+A selection = [%d,%d), want [0,%d)",
			st.SelBeg, st.SelEnd, total)
	}
}

// TestMarkdownNestedCtrlCCopies asserts the copy path works through the
// effective ID: select a word by double-click, then Ctrl+C.
func TestMarkdownNestedCtrlCCopies(t *testing.T) {
	h := newMdSelectHarnessNested(t)
	p := h.blockByText(t, "First paragraph text")

	x, y := runePoint(p, 9) // inside "paragraph"
	h.press(x, y)
	h.release(x, y)
	h.press(x, y)
	h.release(x, y)

	var got string
	h.w.SetClipboardFn(func(s string) { got = s })
	if err := h.w.TestKey("panel:md", KeyC, ModCtrl); err != nil {
		t.Fatalf("TestKey(Ctrl+C): %v", err)
	}
	if got != "paragraph" {
		t.Errorf("clipboard = %q, want %q", got, "paragraph")
	}
}

// TestMarkdownNestedDragAcrossBlocks drags from the paragraph into the
// first list item: the drag reads the block list by effective ID, so
// cross-block extension must work nested.
func TestMarkdownNestedDragAcrossBlocks(t *testing.T) {
	h := newMdSelectHarnessNested(t)
	p := h.blockByText(t, "First paragraph text")
	li := h.blockByText(t, "item one")

	x0, y0 := runePoint(p, 6)
	h.press(x0, y0)
	x1, y1 := runePoint(li, 4)
	h.move(x1, y1)
	h.release(x1, y1)

	st := h.selState()
	if st.SelBeg != p.StartRune+6 || st.SelEnd != li.StartRune+4 {
		t.Errorf("drag selection = [%d,%d), want [%d,%d)",
			st.SelBeg, st.SelEnd,
			p.StartRune+6, li.StartRune+4)
	}
}

// TestMarkdownSameLeafUnderTwoPanelsIsolates renders the same leaf ID
// "md" under two ID-bearing panels — the collision #273 was about: two
// documents with one leaf ID must keep separate selections, focus and
// block lists.
func TestMarkdownSameLeafUnderTwoPanelsIsolates(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{ID: "a", Sizing: FillFit, Content: []View{
					win.Markdown(MarkdownCfg{
						ID: "md", Source: mdSelSource,
						Style: DefaultMarkdownStyle(), Focusable: true,
					}),
				}}),
				Column(ContainerCfg{ID: "b", Sizing: FillFit, Content: []View{
					win.Markdown(MarkdownCfg{
						ID: "md", Source: mdSelSource,
						Style: DefaultMarkdownStyle(), Focusable: true,
					}),
				}}),
			},
		})
	})

	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs in window: %q", dups)
	}

	bm := StateMap[string, []mdBlockInfo](w, nsMdBlocks, capMany)
	aBlocks := bm.GetOr("a:md", nil)
	if len(aBlocks) != 4 {
		t.Fatalf("panel a blocks = %d, want 4", len(aBlocks))
	}
	bBlocks := bm.GetOr("b:md", nil)
	if len(bBlocks) != 4 {
		t.Fatalf("panel b blocks = %d, want 4", len(bBlocks))
	}

	// Click rune 2 of panel a's paragraph; panel b must stay untouched.
	x, y := runePoint(aBlocks[1], 2)
	e := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&e)
	w.settle()
	w.EventFn(&Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	})
	w.settle()

	stA := StateReadOr(w, nsMdSel, "a:md", mdSelState{})
	want := aBlocks[1].StartRune + 2
	if stA.SelBeg != want || stA.SelEnd != want {
		t.Errorf("panel a selection = [%d,%d), want [%d,%d]",
			stA.SelBeg, stA.SelEnd, want, want)
	}
	if got := w.FocusID(); got != "a:md" {
		t.Errorf("focus = %q, want %q", got, "a:md")
	}
	if stB := StateReadOr(w, nsMdSel, "b:md", mdSelState{}); stB.SelEnd != 0 {
		t.Errorf("panel b selection = [%d,%d), want untouched",
			stB.SelBeg, stB.SelEnd)
	}
}

// --- Offset blocks (#274) ---

// TestMarkdownClickAtOffsetSelectsExactRune pushes the markdown to the
// right of a fixed filler and asserts a click lands on the exact rune.
// OnClick is dispatched through callRelative, which hands handlers
// shape-local coordinates, so a click at the center of local rune 2
// must select rune 2 regardless of where the block sits in the window.
func TestMarkdownClickAtOffsetSelectsExactRune(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Row(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Rectangle(RectangleCfg{
					Sizing: FixedFill,
					Width:  120,
				}),
				win.Markdown(MarkdownCfg{
					ID: "md", Source: mdSelSource,
					Style: DefaultMarkdownStyle(), Focusable: true,
				}),
			},
		})
	})

	bm := StateMap[string, []mdBlockInfo](w, nsMdBlocks, capMany)
	blocks := bm.GetOr("md", nil)
	if len(blocks) == 0 {
		t.Fatal("no blocks recorded")
	}
	p := blocks[1]
	if p.ShapeX < 100 {
		t.Fatalf("paragraph ShapeX = %g, want it offset from origin",
			p.ShapeX)
	}

	x, y := runePoint(p, 2)
	e := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&e)
	w.settle()
	w.EventFn(&Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	})
	w.settle()

	st := StateReadOr(w, nsMdSel, "md", mdSelState{})
	want := p.StartRune + 2
	if st.SelBeg != want || st.SelEnd != want {
		t.Errorf("selection = [%d,%d), want [%d,%d] — the click "+
			"must map to the rune under the cursor",
			st.SelBeg, st.SelEnd, want, want)
	}
}

// TestRtfSelectClickAtOffsetSelectsExactRune is the same check for
// standalone selectable RTF (rtfSelectOnClick), which shares the
// callRelative coordinate contract.
func TestRtfSelectClickAtOffsetSelectsExactRune(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Row(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Rectangle(RectangleCfg{
					Sizing: FixedFill,
					Width:  120,
				}),
				RTF(RTFCfg{
					ID:        "rtf",
					Focusable: true,
					RichText: RichText{Runs: []RichTextRun{
						{Text: "hello world",
							Style: TextStyle{Size: 14}},
					}},
				}),
			},
		})
	})

	ly, ok := w.layout.FindByID("rtf")
	if !ok {
		t.Fatal("no RTF shape")
	}
	if ly.Shape.X < 100 {
		t.Fatalf("RTF ShapeX = %g, want it offset from origin",
			ly.Shape.X)
	}

	// Click the middle of local rune 6 ('w').
	gl := ly.Shape.TC.rTFLayout
	bi := runeToByteIndex(ly.Shape.TC.rTFFlatText, 6)
	x := ly.Shape.X + float32(bi)*10 + 5
	y := ly.Shape.Y + 10
	e := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&e)
	w.settle()
	w.EventFn(&Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	})
	w.settle()

	is := StateReadOr(w, nsInput, "rtf", inputState{})
	if is.selectBeg != 6 || is.selectEnd != 6 {
		t.Errorf("selection = [%d,%d), want cursor at 6",
			is.selectBeg, is.selectEnd)
	}
	_ = gl
}

// --- Source change (#275) ---

// TestMarkdownSourceChangeClearsSelection asserts the selection is
// reset when the document's content changes between frames: the stored
// rune offsets point into the old source and would highlight or copy
// the wrong runes of the new one.
func TestMarkdownSourceChangeClearsSelection(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	x, y := runePoint(p, 2)
	h.press(x, y)
	h.release(x, y)
	want := p.StartRune + 2
	if st := h.selState(); st.SelBeg != want || st.SelEnd != want {
		t.Fatalf("selection before change = [%d,%d), want [%d,%d]",
			st.SelBeg, st.SelEnd, want, want)
	}

	h.source = "# Different\n\nchanged text"
	h.w.TestRender(nil)

	st := h.selState()
	if st.SelBeg != 0 || st.SelEnd != 0 {
		t.Errorf("selection after source change = [%d,%d), want "+
			"[0,0] (stale offsets must not survive)",
			st.SelBeg, st.SelEnd)
	}

	// Ctrl+C on the cleared selection must not touch the clipboard.
	var copied bool
	h.w.SetClipboardFn(func(string) { copied = true })
	if err := h.w.TestKey("md", KeyC, ModCtrl); err != nil {
		t.Fatalf("TestKey(Ctrl+C): %v", err)
	}
	if copied {
		t.Error("Ctrl+C copied despite a cleared selection")
	}
}

// TestMarkdownSameSourceKeepsSelection is the positive control for the
// signature reset: an idle frame with identical content must keep the
// selection (a reset would be a visible flicker on every frame).
func TestMarkdownSameSourceKeepsSelection(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	x, y := runePoint(p, 2)
	h.press(x, y)
	h.release(x, y)
	want := p.StartRune + 2

	h.w.TestRender(nil)

	st := h.selState()
	if st.SelBeg != want || st.SelEnd != want {
		t.Errorf("selection after idle re-render = [%d,%d), "+
			"want [%d,%d]", st.SelBeg, st.SelEnd, want, want)
	}
}

// TestMarkdownDoubleClickSelectsWord asserts two rapid clicks expand
// the selection to the word under the cursor.
func TestMarkdownDoubleClickSelectsWord(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	// Local rune 9 ('a') sits inside "paragraph" [6,15).
	x, y := runePoint(p, 9)
	h.press(x, y)
	h.release(x, y)
	if st := h.selState(); st.SelBeg != st.SelEnd {
		t.Fatalf("first click selected [%d,%d), want a cursor",
			st.SelBeg, st.SelEnd)
	}

	h.press(x, y)
	h.release(x, y)

	wantBeg, wantEnd := p.StartRune+6, p.StartRune+15
	st := h.selState()
	if st.SelBeg != wantBeg || st.SelEnd != wantEnd {
		t.Errorf("word selection = [%d,%d), want [%d,%d)",
			st.SelBeg, st.SelEnd, wantBeg, wantEnd)
	}
	if st.LastClickTime == 0 {
		t.Error("LastClickTime not recorded")
	}
}

// TestMarkdownClickIsolatedByDoubleClickThreshold splits the double
// click across more than doubleClickThresholdMs so the second click
// must be treated as a fresh cursor placement. Simulated by stubbing
// LastClickTime in the past — the handler reads wall-clock time, so a
// real sleep would be both slow and flaky.
func TestMarkdownClickIsolatedByDoubleClickThreshold(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	imap := StateMap[string, mdSelState](h.w, nsMdSel, capMany)
	imap.Set("md", mdSelState{
		SelBeg: p.StartRune + 2, SelEnd: p.StartRune + 2,
		LastClickTime: time.Now().UnixMilli() -
			doubleClickThresholdMs - 1,
	})

	x, y := runePoint(p, 10)
	h.press(x, y)
	h.release(x, y)

	st := h.selState()
	if st.SelBeg != st.SelEnd || st.SelBeg != p.StartRune+10 {
		t.Errorf("stale double-click window selected [%d,%d), "+
			"want cursor at %d", st.SelBeg, st.SelEnd, p.StartRune+10)
	}
}

// --- Drag selection ---

// TestMarkdownDragSelectsAcrossBlocks presses in the paragraph and
// drags into the first list item, asserting the selection spans the
// block boundary and the mouse lock is released on mouse-up.
func TestMarkdownDragSelectsAcrossBlocks(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")
	li := h.blockByText(t, "item one")

	anchor := p.StartRune + 6 // 'p' of "paragraph"
	target := li.StartRune + 4

	x0, y0 := runePoint(p, 6)
	h.press(x0, y0)
	if !h.w.mouseIsLocked() {
		t.Fatal("press on a block did not lock the mouse")
	}

	x1, y1 := runePoint(li, 4)
	h.move(x1, y1)
	st := h.selState()
	if st.SelBeg != anchor || st.SelEnd != target {
		t.Fatalf("mid-drag selection = [%d,%d), want [%d,%d)",
			st.SelBeg, st.SelEnd, anchor, target)
	}

	h.release(x1, y1)
	if h.w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	st = h.selState()
	if st.SelBeg != anchor || st.SelEnd != target {
		t.Errorf("selection after release = [%d,%d), want [%d,%d)",
			st.SelBeg, st.SelEnd, anchor, target)
	}
}

// TestMarkdownDragBackwardsInvertsSelection drags from a later rune to
// an earlier one: the raw state keeps SelBeg at the press anchor and
// SelEnd at the cursor, so SelBeg > SelEnd — the amend pass sorts
// before stamping per-block highlights.
func TestMarkdownDragBackwardsInvertsSelection(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	anchor := p.StartRune + 10
	target := p.StartRune + 2

	x0, y0 := runePoint(p, 10)
	h.press(x0, y0)
	x1, y1 := runePoint(p, 2)
	h.move(x1, y1)
	h.release(x1, y1)

	st := h.selState()
	if st.SelBeg != anchor || st.SelEnd != target {
		t.Errorf("inverted selection = [%d,%d), want [%d,%d) "+
			"(anchor held at press, cursor at release)",
			st.SelBeg, st.SelEnd, anchor, target)
	}

	// The per-block highlight uses the normalized order: the block
	// shape covers [2,10) locally.
	ly := h.findBlockShape(t, p.StartRune)
	if ly.Shape.TC.textSelBeg != 2 || ly.Shape.TC.textSelEnd != 10 {
		t.Errorf("block highlight = [%d,%d), want [2,10)",
			ly.Shape.TC.textSelBeg, ly.Shape.TC.textSelEnd)
	}
}

// TestMarkdownRightClickLeavesSelectionUntouched asserts the right
// button (reserved for the link context menu) never moves the
// selection.
func TestMarkdownRightClickLeavesSelectionUntouched(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	x, y := runePoint(p, 2)
	h.press(x, y)
	h.release(x, y)
	want := p.StartRune + 2

	xr, yr := runePoint(p, 12)
	e := Event{
		Type: EventMouseDown, MouseButton: MouseRight,
		MouseX: xr, MouseY: yr,
	}
	h.w.EventFn(&e)
	h.w.settle()
	h.w.EventFn(&Event{
		Type: EventMouseUp, MouseButton: MouseRight,
		MouseX: xr, MouseY: yr,
	})
	h.w.settle()

	st := h.selState()
	if st.SelBeg != want || st.SelEnd != want {
		t.Errorf("right click moved selection to [%d,%d), "+
			"want it untouched at [%d,%d]",
			st.SelBeg, st.SelEnd, want, want)
	}
}

// --- Keyboard: select all and copy ---

func TestMarkdownCtrlASelectsAll(t *testing.T) {
	h := newMdSelectHarness(t)

	if err := h.w.TestKey("md", KeyA, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}

	var total uint32
	for _, b := range h.blocks(t) {
		total += b.RuneLen
	}
	st := h.selState()
	if st.SelBeg != 0 || st.SelEnd != total {
		t.Errorf("Ctrl+A selection = [%d,%d), want [0,%d)",
			st.SelBeg, st.SelEnd, total)
	}
}

// TestMarkdownCtrlCWithoutModifierIgnored asserts plain C does nothing
// (no consume, no clipboard write) — the shortcut requires Ctrl/Super.
func TestMarkdownCtrlCWithoutModifierIgnored(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")
	x, y := runePoint(p, 2)
	h.press(x, y)
	h.release(x, y)

	var got string
	h.w.SetClipboardFn(func(s string) { got = s })

	ly, ok := h.w.layout.FindByID("md")
	if !ok {
		t.Fatal("no container shape with ID md")
	}
	e := Event{Type: EventKeyDown, KeyCode: KeyC}
	markdownContainerOnKeyDown(EventCtx{Layout: ly, Event: &e, Window: h.w})
	if e.IsHandled {
		t.Error("bare C consumed the key event")
	}
	if got != "" {
		t.Errorf("clipboard = %q, want empty for unmodified C", got)
	}
}

// TestMarkdownCtrlCCopiesCrossBlockSelection drags from inside the
// paragraph into the first list item and copies: the clipboard must
// hold the extracted flat text with blocks joined by a newline. The
// anchor sits at the start of "paragraph", so the drag spans the rest
// of the paragraph ("paragraph text") and the first four runes of
// "item one".
func TestMarkdownCtrlCCopiesCrossBlockSelection(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")
	li := h.blockByText(t, "item one")

	x0, y0 := runePoint(p, 6)
	h.press(x0, y0)
	x1, y1 := runePoint(li, 4)
	h.move(x1, y1)
	h.release(x1, y1)

	var got string
	h.w.SetClipboardFn(func(s string) { got = s })

	if err := h.w.TestKey("md", KeyC, ModCtrl); err != nil {
		t.Fatalf("TestKey(Ctrl+C): %v", err)
	}
	if got != "paragraph text\nitem" {
		t.Errorf("clipboard = %q, want %q",
			got, "paragraph text\nitem")
	}
}

// TestMarkdownCtrlCWithInvertedSelectionNormalizes drags from local
// rune 10 back to local rune 2 of the paragraph, so the raw state is
// inverted; the copy path must sort before extracting, yielding the
// text between the two runes: "rst para".
func TestMarkdownCtrlCWithInvertedSelectionNormalizes(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	x0, y0 := runePoint(p, 10)
	h.press(x0, y0)
	x1, y1 := runePoint(p, 2)
	h.move(x1, y1)
	h.release(x1, y1)

	var got string
	h.w.SetClipboardFn(func(s string) { got = s })

	if err := h.w.TestKey("md", KeyC, ModCtrl); err != nil {
		t.Fatalf("TestKey(Ctrl+C): %v", err)
	}
	if got != "rst para" {
		t.Errorf("clipboard = %q, want %q", got, "rst para")
	}
}

// --- Persistence and edge cases ---

// TestMarkdownSelectionSurvivesIdleFrame asserts the selection state
// (keyed by markdown ID in the StateMap) and the per-block highlight
// stamps both persist across a frame with no input.
func TestMarkdownSelectionSurvivesIdleFrame(t *testing.T) {
	h := newMdSelectHarness(t)
	p := h.blockByText(t, "First paragraph text")

	x, y := runePoint(p, 2)
	h.press(x, y)
	h.release(x, y)
	want := p.StartRune + 2

	h.w.TestRender(nil)

	st := h.selState()
	if st.SelBeg != want || st.SelEnd != want {
		t.Errorf("state after idle frame = [%d,%d), want [%d,%d]",
			st.SelBeg, st.SelEnd, want, want)
	}
	ly := h.findBlockShape(t, p.StartRune)
	if ly.Shape.TC.textSelBeg != 2 || ly.Shape.TC.textSelEnd != 2 {
		t.Errorf("block highlight after idle frame = [%d,%d), "+
			"want [2,2]", ly.Shape.TC.textSelBeg, ly.Shape.TC.textSelEnd)
	}
}

// TestMarkdownSelectionStateIsPerWidget renders two markdown widgets
// and asserts each keeps its own selection — the nsMdSel state is
// keyed by widget ID, so a click in one must not disturb the other.
func TestMarkdownSelectionStateIsPerWidget(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = mdSelectTestMeasurer{}
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				win.Markdown(MarkdownCfg{
					ID: "md-a", Source: "alpha", Style: DefaultMarkdownStyle(),
					Focusable: true,
				}),
				win.Markdown(MarkdownCfg{
					ID: "md-b", Source: "beta", Style: DefaultMarkdownStyle(),
					Focusable: true,
				}),
			},
		})
	})

	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs in window: %q", dups)
	}

	// Click rune 2 of md-a; assert md-b's state is untouched.
	bm := StateMap[string, []mdBlockInfo](w, nsMdBlocks, capMany)
	aBlocks := bm.GetOr("md-a", nil)
	if len(aBlocks) != 1 {
		t.Fatalf("md-a blocks = %d, want 1", len(aBlocks))
	}
	x, y := runePoint(aBlocks[0], 2)
	e := Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&e)
	w.settle()
	w.EventFn(&Event{
		Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y,
	})
	w.settle()

	stA := StateReadOr(w, nsMdSel, "md-a", mdSelState{})
	if stA.SelBeg != 2 || stA.SelEnd != 2 {
		t.Errorf("md-a selection = [%d,%d), want [2,2]",
			stA.SelBeg, stA.SelEnd)
	}
	if stB := StateReadOr(w, nsMdSel, "md-b", mdSelState{}); stB.SelEnd != 0 {
		t.Errorf("md-b selection = [%d,%d), want untouched",
			stB.SelBeg, stB.SelEnd)
	}
}

// TestMarkdownKeydownWhenNotFocusedIgnored asserts Ctrl+A dispatched
// while the markdown does not hold focus changes nothing. The event
// never reaches the handler: keydown dispatch targets the focused
// shape (isFocusedTarget), so an unfocused container's OnKeyDown is
// not invoked at all — this pins the end-to-end behavior, not the
// handler's own guard.
func TestMarkdownKeydownWhenNotFocusedIgnored(t *testing.T) {
	h := newMdSelectHarness(t)
	h.w.ClearFocus()

	e := Event{Type: EventKeyDown, KeyCode: KeyA, Modifiers: ModCtrl}
	h.w.EventFn(&e)
	h.w.settle()

	st := h.selState()
	if st.SelBeg != 0 || st.SelEnd != 0 {
		t.Errorf("unfocused Ctrl+A set selection to [%d,%d)",
			st.SelBeg, st.SelEnd)
	}
}

// TestMarkdownKeydownWithoutBlocksInert covers the handler's empty
// block-list guard: a focusable markdown containing only non-RTF
// blocks has nothing to select, so Ctrl+A must not consume.
func TestMarkdownKeydownWithoutBlocksInert(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 800})
	w.TestRender(func(win *Window) View {
		return win.Markdown(MarkdownCfg{
			ID:        "md",
			Source:    "```go\ncode\n```",
			Style:     DefaultMarkdownStyle(),
			Focusable: true,
		})
	})

	if err := w.TestKey("md", KeyA, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	st := StateReadOr(w, nsMdSel, "md", mdSelState{})
	if st.SelEnd != 0 {
		t.Errorf("Ctrl+A selected [%d,%d) with no RTF blocks",
			st.SelBeg, st.SelEnd)
	}

	// Direct call: the guard returns without consuming.
	ly, ok := w.layout.FindByID("md")
	if !ok {
		t.Fatal("no container shape with ID md")
	}
	e := Event{Type: EventKeyDown, KeyCode: KeyA, Modifiers: ModCtrl}
	markdownContainerOnKeyDown(EventCtx{Layout: ly, Event: &e, Window: w})
	if e.IsHandled {
		t.Error("Ctrl+A with no blocks consumed the key event")
	}
}

// TestMarkdownAmendLayoutWithEmptyIDSkips asserts the amend hook bails
// on an ID-less container instead of writing a block list.
func TestMarkdownAmendLayoutWithEmptyIDSkips(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	e := &Event{}
	markdownContainerAmendLayout(EventCtx{
		Layout: &Layout{Shape: &Shape{}},
		Event:  e,
		Window: w,
	})
	if bm := StateMapRead[string, []mdBlockInfo](w, nsMdBlocks); bm != nil {
		if _, ok := bm.Get(""); ok {
			t.Error("amend wrote a block list for an ID-less container")
		}
	}
}

// findBlockShape walks the composed tree for the RTF shape whose
// markdown block starts at startRune — the shape the amend pass stamps
// the per-block highlight on.
func (h *mdSelectHarness) findBlockShape(t *testing.T, startRune uint32) *Layout {
	t.Helper()
	var found *Layout
	var walk func(*Layout)
	walk = func(l *Layout) {
		if found != nil {
			return
		}
		if l.Shape != nil && l.Shape.TC != nil &&
			l.Shape.TC.markdownID == h.id &&
			l.Shape.TC.markdownBlockStart == startRune {
			found = l
			return
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(&h.w.layout)
	if found == nil {
		t.Fatalf("no RTF shape with block start %d", startRune)
	}
	return found
}
