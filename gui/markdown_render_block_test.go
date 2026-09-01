package gui

// Tests for MarkdownCfg.RenderBlock (issue #484).
//
// The hook is a new first branch of markdownBuildContent's render
// switch, and that loop carries three pieces of cross-block state a
// naive branch corrupts: the rune offsets selection keys on, the
// pending-list accumulator, and the tail flush. There is one test per
// rule below the dispatch tests, because each of the three fails
// silently -- a wrong offset only shows up as a bad selection, and a
// dropped list only shows up as missing text.

import (
	"strings"
	"testing"
)

// mdHookLayout renders one source with one hook and returns the
// composed layout. cfg fields other than the hook match
// markdownLayoutForSource, so a hooked and an unhooked render of the
// same source are comparable.
func mdHookLayout(
	t *testing.T, source string, focusable bool,
	hook func(*Window, MarkdownElement) (View, bool),
) Layout {
	t.Helper()

	w := &Window{}
	return generateViewLayout(w.Markdown(MarkdownCfg{
		ID:          "md",
		Source:      source,
		Style:       DefaultMarkdownStyle(),
		Focusable:   focusable,
		RenderBlock: hook,
	}), w)
}

// mdCollectText walks a composed layout and returns every plain and
// rich text string it renders, in tree order.
func mdCollectText(l Layout) []string {
	var out []string
	var walk func(Layout)
	walk = func(n Layout) {
		if n.Shape.TC != nil {
			if n.Shape.TC.Text != "" {
				out = append(out, n.Shape.TC.Text)
			}
			if rt := n.Shape.TC.rTFRuns; rt != nil && len(rt.Runs) > 0 {
				out = append(out, richTextPlain(*rt))
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(l)
	return out
}

// mdBlockStarts returns the rune offset stamped on every selectable
// block, in tree order. This is what a selection resolves against, so
// it is the observable that must not move when a block is hooked.
//
// markdownID is the discriminator: applyMdCtx stamps it only for the
// blocks that were handed a ctx, which is exactly the six selectable
// kinds. A code block or a rule leaves it empty.
func mdBlockStarts(l Layout) []uint32 {
	var out []uint32
	var walk func(Layout)
	walk = func(n Layout) {
		if tc := n.Shape.TC; tc != nil && tc.markdownID != "" {
			out = append(out, tc.markdownBlockStart)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(l)
	return out
}

// mdMarker is a text view a hook returns in place of a block, chosen
// so it cannot collide with any source text.
func mdMarker(tag string) View {
	return Text(TextCfg{Text: "<<" + tag + ">>"})
}

func mdHasText(l Layout, want string) bool {
	for _, s := range mdCollectText(l) {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

// --- Dispatch ---

// TestMarkdownRenderBlockKinds pins the Kind each block arrives with.
// The parsed flags are not mutually exclusive, so a kind derived in a
// different order than the render switch would disagree with the
// branch actually taken.
func TestMarkdownRenderBlockKinds(t *testing.T) {
	const source = "# Head\n\nBody text.\n\n> Quoted.\n\n" +
		"```go\nx := 1\n```\n\n---\n\n- item\n"

	var got []MarkdownBlockKind
	mdHookLayout(t, source, false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			got = append(got, el.Kind)
			return nil, false
		})

	want := []MarkdownBlockKind{
		MarkdownKindHeading,
		MarkdownKindParagraph,
		MarkdownKindBlockquote,
		MarkdownKindCode,
		MarkdownKindHR,
		MarkdownKindList,
	}
	if len(got) != len(want) {
		t.Fatalf("kinds: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d: got kind %d, want %d",
				i, got[i], want[i])
		}
	}
}

// TestMarkdownRenderBlockMermaidIsCode pins the one kind with no
// constant of its own: renderMdMermaid dispatches from inside the code
// branch, so a hook must match on the language, not on a kind.
func TestMarkdownRenderBlockMermaidIsCode(t *testing.T) {
	var kind MarkdownBlockKind
	var lang string
	seen := false
	mdHookLayout(t, "```mermaid\ngraph TD;\n```\n", false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			kind, lang, seen = el.Kind, el.CodeLanguage, true
			return nil, false
		})

	if !seen {
		t.Fatal("hook never fired for a mermaid fence")
	}
	if kind != MarkdownKindCode {
		t.Errorf("mermaid kind: got %d, want MarkdownKindCode", kind)
	}
	if lang != "mermaid" {
		t.Errorf("mermaid language: got %q, want %q", lang, "mermaid")
	}
}

// TestMarkdownRenderBlockElementFields checks the derived fields a
// hook cannot recompute: the resolved document ID, source-order index,
// and the unstyled text.
func TestMarkdownRenderBlockElementFields(t *testing.T) {
	var els []MarkdownElement
	mdHookLayout(t, "# Title\n\nA **bold** word.\n", false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			els = append(els, el)
			return nil, false
		})

	if len(els) != 2 {
		t.Fatalf("got %d elements, want 2", len(els))
	}
	for i, el := range els {
		if el.DocID != "md" {
			t.Errorf("block %d DocID: got %q, want %q",
				i, el.DocID, "md")
		}
		if el.Index != i {
			t.Errorf("block %d Index: got %d", i, el.Index)
		}
	}
	if els[0].HeaderLevel != 1 {
		t.Errorf("heading level: got %d, want 1", els[0].HeaderLevel)
	}
	if got := els[1].PlainText; !strings.Contains(got, "bold") {
		t.Errorf("PlainText: got %q, want it to contain %q",
			got, "bold")
	}
	if strings.Contains(els[1].PlainText, "**") {
		t.Errorf("PlainText kept markup: %q", els[1].PlainText)
	}
}

// TestMarkdownRenderBlockFallback: ok=false must render exactly what
// no hook at all renders.
func TestMarkdownRenderBlockFallback(t *testing.T) {
	const source = "# Head\n\nBody text.\n\n- item\n"

	plain := mdCollectText(markdownLayoutForSource(t, source))
	hooked := mdCollectText(mdHookLayout(t, source, false,
		func(_ *Window, _ MarkdownElement) (View, bool) {
			return mdMarker("never"), false
		}))

	if len(plain) != len(hooked) {
		t.Fatalf("ok=false changed the output: %v vs %v",
			plain, hooked)
	}
	for i := range plain {
		if plain[i] != hooked[i] {
			t.Errorf("text %d: got %q, want %q",
				i, hooked[i], plain[i])
		}
	}
}

// TestMarkdownRenderBlockReplaces: ok=true with a view renders that
// view in the block's place.
func TestMarkdownRenderBlockReplaces(t *testing.T) {
	l := mdHookLayout(t, "# Head\n\nBody text.\n", false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			if el.Kind != MarkdownKindHeading {
				return nil, false
			}
			return mdMarker("head"), true
		})

	if !mdHasText(l, "<<head>>") {
		t.Error("replacement view did not render")
	}
	if mdHasText(l, "Head") {
		t.Error("the replaced heading still rendered")
	}
	if !mdHasText(l, "Body text.") {
		t.Error("the unhooked paragraph stopped rendering")
	}
}

// TestMarkdownRenderBlockDrops: ok=true with a nil view drops the
// block and leaves its neighbors alone.
func TestMarkdownRenderBlockDrops(t *testing.T) {
	l := mdHookLayout(t, "# Head\n\nBody text.\n", false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			return nil, el.Kind == MarkdownKindHeading
		})

	if mdHasText(l, "Head") {
		t.Error("the dropped heading still rendered")
	}
	if !mdHasText(l, "Body text.") {
		t.Error("the unhooked paragraph stopped rendering")
	}
}

// --- Loop state ---

// TestMarkdownRenderBlockOffsetsUnchanged is loop-state rule 1. A
// hooked code block must not advance the document's rune offsets,
// because the default code branch does not: if it did, a selection
// over the blocks after it would land on the wrong runes in the hooked
// document only.
func TestMarkdownRenderBlockOffsetsUnchanged(t *testing.T) {
	const source = "First paragraph.\n\n```go\nx := 1\n```\n\n" +
		"Second paragraph.\n\n- item one\n- item two\n"

	w := &Window{}
	plain := mdBlockStarts(generateViewLayout(w.Markdown(MarkdownCfg{
		ID:        "md",
		Source:    source,
		Style:     DefaultMarkdownStyle(),
		Focusable: true,
	}), w))

	hooked := mdBlockStarts(mdHookLayout(t, source, true,
		func(_ *Window, el MarkdownElement) (View, bool) {
			if el.Kind != MarkdownKindCode {
				return nil, false
			}
			return mdMarker("code"), true
		}))

	if len(plain) == 0 {
		t.Fatal("no selectable blocks recorded offsets")
	}
	if len(plain) != len(hooked) {
		t.Fatalf("selectable block count moved: %v vs %v",
			plain, hooked)
	}
	for i := range plain {
		if plain[i] != hooked[i] {
			t.Errorf("block %d start: got %d, want %d",
				i, hooked[i], plain[i])
		}
	}
}

// TestMarkdownRenderBlockOffsetsAdvanceForSelectable is the other half
// of rule 1: a hooked paragraph must still advance the offsets,
// because the paragraph branch would have. Hooking a selectable block
// without advancing would shift every block after it.
func TestMarkdownRenderBlockOffsetsAdvanceForSelectable(t *testing.T) {
	const source = "First paragraph.\n\nSecond paragraph.\n\n" +
		"Third paragraph.\n"

	w := &Window{}
	plain := mdBlockStarts(generateViewLayout(w.Markdown(MarkdownCfg{
		ID:        "md",
		Source:    source,
		Style:     DefaultMarkdownStyle(),
		Focusable: true,
	}), w))
	if len(plain) != 3 {
		t.Fatalf("got %d selectable blocks, want 3", len(plain))
	}

	hooked := mdBlockStarts(mdHookLayout(t, source, true,
		func(_ *Window, el MarkdownElement) (View, bool) {
			if el.Index != 1 {
				return nil, false
			}
			return mdMarker("para"), true
		}))

	// The hooked block itself is no longer selectable, so it drops out
	// of the list; the block after it must keep the offset it had.
	if len(hooked) != 2 {
		t.Fatalf("got %d selectable blocks, want 2", len(hooked))
	}
	if hooked[0] != plain[0] || hooked[1] != plain[2] {
		t.Errorf("offsets moved: got %v, want %v and %v",
			hooked, plain[0], plain[2])
	}
}

// TestMarkdownRenderBlockListOrder is loop-state rule 2. A hooked list
// item appends straight to the content, so any items already pending
// in the accumulator have to be flushed first or they render after
// their own later sibling.
func TestMarkdownRenderBlockListOrder(t *testing.T) {
	l := mdHookLayout(t, "- alpha\n- beta\n- gamma\n", false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			if el.Kind != MarkdownKindList ||
				!strings.Contains(el.PlainText, "beta") {
				return nil, false
			}
			return mdMarker("beta"), true
		})

	texts := mdCollectText(l)
	pos := func(want string) int {
		for i, s := range texts {
			if strings.Contains(s, want) {
				return i
			}
		}
		return -1
	}
	iAlpha, iBeta, iGamma := pos("alpha"), pos("<<beta>>"), pos("gamma")
	if iAlpha < 0 || iBeta < 0 || iGamma < 0 {
		t.Fatalf("missing text: alpha=%d beta=%d gamma=%d in %v",
			iAlpha, iBeta, iGamma, texts)
	}
	if iAlpha >= iBeta || iBeta >= iGamma {
		t.Errorf("out of source order: alpha=%d beta=%d gamma=%d",
			iAlpha, iBeta, iGamma)
	}
}

// TestMarkdownRenderBlockLastListItem is loop-state rule 3. The tail
// flush used to live inside the list case, so a hooked final item
// skipped it and the earlier items were silently dropped.
func TestMarkdownRenderBlockLastListItem(t *testing.T) {
	l := mdHookLayout(t, "- alpha\n- beta\n", false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			if el.Kind != MarkdownKindList ||
				!strings.Contains(el.PlainText, "beta") {
				return nil, false
			}
			return mdMarker("beta"), true
		})

	if !mdHasText(l, "alpha") {
		t.Error("the earlier list item was dropped")
	}
	if !mdHasText(l, "<<beta>>") {
		t.Error("the hooked final item did not render")
	}
}

// TestMarkdownRenderBlockTailListUnhooked guards the same flush from
// the other side: an unhooked document ending on a list item must
// still render it after the flush moved out of the list case.
func TestMarkdownRenderBlockTailListUnhooked(t *testing.T) {
	l := markdownLayoutForSource(t, "Intro.\n\n- alpha\n- beta\n")
	if !mdHasText(l, "alpha") || !mdHasText(l, "beta") {
		t.Errorf("trailing list not rendered: %v", mdCollectText(l))
	}
}

// --- Hook-built IDs ---

// TestMarkdownRenderBlockIDsScoped proves the debug gate sees inside
// hook output. A hook owns its IDs: composed from el.DocID with
// ScopeIDN they are unique per block, and the sweep stays quiet.
func TestMarkdownRenderBlockIDsScoped(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				win.Markdown(MarkdownCfg{
					ID:     "doc",
					Source: "One.\n\nTwo.\n\nThree.\n",
					RenderBlock: func(
						_ *Window, el MarkdownElement,
					) (View, bool) {
						return ProgressBar(ProgressBarCfg{
							ID: ScopeIDN(el.DocID, "hooked",
								el.Index),
							Percent: 50,
						}), true
					},
				}),
			},
		})
	})

	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no findings, got %q", got)
	}
}

// TestMarkdownRenderBlockIDCollisionReported is the same shape with
// the Index dropped, so every hooked block claims one identity. The
// gate must report it rather than let the blocks silently share a
// state slot.
func TestMarkdownRenderBlockIDCollisionReported(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				win.Markdown(MarkdownCfg{
					ID:     "doc",
					Source: "One.\n\nTwo.\n\nThree.\n",
					RenderBlock: func(
						_ *Window, el MarkdownElement,
					) (View, bool) {
						return ProgressBar(ProgressBarCfg{
							ID:      ScopeID(el.DocID, "hooked"),
							Percent: 50,
						}), true
					},
				}),
			},
		})
	})

	got := w.TestDuplicateIDs()
	if len(got) == 0 {
		t.Fatal("a hook claiming one ID for every block went unreported")
	}
	if !strings.Contains(got[0], "doc:hooked") {
		t.Errorf("finding does not name the collided ID: %q", got[0])
	}
}

// TestMarkdownRenderBlockRemainingKinds covers the five kinds the
// main dispatch test does not: table, image, math, definition term
// and definition value. Missing any one is a silent wrong-Kind bug
// for hooks that branch on el.Kind.
func TestMarkdownRenderBlockRemainingKinds(t *testing.T) {
	const source = "| H1 | H2 |\n|---|---|\n| a | b |\n\n" +
		"![alt](image.png)\n\n$$\nE = mc^2\n$$\n\n" +
		"Term\n: Definition value.\n"

	var got []MarkdownBlockKind
	var tableDataSeen bool
	mdHookLayout(t, source, false,
		func(_ *Window, el MarkdownElement) (View, bool) {
			got = append(got, el.Kind)
			if el.Kind == MarkdownKindTable {
				if el.TableData == nil {
					t.Error("table element has nil TableData")
				} else {
					tableDataSeen = true
				}
			} else if el.TableData != nil {
				t.Errorf("non-table kind %d has non-nil TableData",
					el.Kind)
			}
			return nil, false
		})

	want := []MarkdownBlockKind{
		MarkdownKindTable,
		MarkdownKindImage,
		MarkdownKindMath,
		MarkdownKindDefTerm,
		MarkdownKindDefValue,
	}
	if len(got) != len(want) {
		t.Fatalf("kinds: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d: got kind %d, want %d",
				i, got[i], want[i])
		}
	}
	if !tableDataSeen {
		t.Error("table block TableData was never non-nil")
	}
}

// TestMarkdownRenderBlockEmptySource verifies a document with no
// blocks still builds: the hook fires zero times and the doc copy
// button alone keeps the layout valid.
func TestMarkdownRenderBlockEmptySource(t *testing.T) {
	calls := 0
	l := mdHookLayout(t, "", false,
		func(_ *Window, _ MarkdownElement) (View, bool) {
			calls++
			return nil, false
		})
	if calls != 0 {
		t.Fatalf("empty source: hook called %d times, want 0", calls)
	}
	// Layout must still have the document container (copy button).
	if l.Shape == nil {
		t.Fatal("empty source produced no layout")
	}
}
