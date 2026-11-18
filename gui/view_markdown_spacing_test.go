package gui

// view_markdown_spacing_test.go pins the gaps inside list items,
// blockquotes and fenced code blocks: the marker-to-text gap, the
// quote bar-to-text inset and tab indentation in code.

import (
	"strings"
	"testing"
)

// mdSpacingLayout lays out v with a fixed-width stub measurer, so the
// asserted gaps come from the containers, not from glyph metrics.
func mdSpacingLayout(t *testing.T, v View) Layout {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{State: new(int)})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	layout := generateViewLayout(v, w)
	layoutPipeline(&layout, w)
	return layout
}

// A list item's text starts SpacingSmall after the marker ink. The row
// used to take the container default (SpacingMedium, 14px) and the
// marker kept its trailing space, so the gap was 14px plus a space.
func TestMdListItemMarkerGap(t *testing.T) {
	cfg := MarkdownCfg{Style: DefaultMarkdownStyle()}
	for _, prefix := range []string{"• ", "1. ", "10. "} {
		layout := mdSpacingLayout(t, mdRenderListItem(
			markdownBlock{ListPrefix: prefix}, cfg,
			TextModeSingleLine, nil))
		marker, text := layout.Children[0].Shape, layout.Children[1].Shape
		markerText := layout.Children[0].Children[0].Shape
		if got := markerText.TC.Text; got != strings.TrimRight(prefix, " ") {
			t.Errorf("%q: marker text = %q, want no trailing space", prefix, got)
		}
		gap := text.X - (marker.X + marker.Width)
		if gap != gapSmall {
			t.Errorf("%q: marker-to-text gap = %v, want %v", prefix, gap, gapSmall)
		}
	}
}

// A task item's text starts SpacingSmall after the box. The marker
// column used to be the box plus 6px, then the 14px row default.
func TestMdTaskItemBoxGap(t *testing.T) {
	cfg := MarkdownCfg{Style: DefaultMarkdownStyle()}
	layout := mdSpacingLayout(t, mdRenderListItem(
		markdownBlock{IsTaskItem: true}, cfg, TextModeSingleLine, nil))
	box, text := layout.Children[0].Shape, layout.Children[1].Shape
	if box.Width != mdTaskBoxEdge(cfg) {
		t.Errorf("task marker column = %v, want the box edge %v",
			box.Width, mdTaskBoxEdge(cfg))
	}
	if gap := text.X - (box.X + box.Width); gap != gapSmall {
		t.Errorf("box-to-text gap = %v, want %v", gap, gapSmall)
	}
}

// The quote tint starts at the bar, and the text is inset inside the
// tint. The row used to put a transparent 14px gap between bar and
// tint, and the text sat flush on the tint's left edge.
func TestMdQuoteBarInset(t *testing.T) {
	cfg := MarkdownCfg{Style: DefaultMarkdownStyle()}
	layout := mdSpacingLayout(t, mdWrapQuote(
		markdownBlock{BlockquoteDepth: 1},
		Text(TextCfg{Text: "quoted"}), cfg))
	bar, tint := layout.Children[0].Shape, layout.Children[1].Shape
	text := layout.Children[1].Children[0].Shape
	if tint.X != bar.X+bar.Width {
		t.Errorf("tint starts at %v, want flush with bar end %v",
			tint.X, bar.X+bar.Width)
	}
	if inset := text.X - tint.X; inset != mdQuoteInset {
		t.Errorf("text inset in tint = %v, want %v", inset, mdQuoteInset)
	}
}

// Tabs in a fenced block expand to the next tab stop: glyph has no tab
// stops, so a raw tab drew one space wide and flattened indentation.
// The copy button still copies the source with its tabs.
func TestMdCodeBlockExpandsTabs(t *testing.T) {
	src := "```go\nfunc f() {\n\tif x {\n\t\ty()\n\t}\n}\n```\n"
	for _, hl := range []bool{false, true} {
		style := DefaultMarkdownStyle()
		if !hl {
			style.CodeHighlighter = nil
		}
		blocks := markdownToBlocks(src, style)
		if len(blocks) != 1 || !blocks[0].IsCode {
			t.Fatalf("want one code block, got %d", len(blocks))
		}
		got := richTextPlain(blocks[0].Content)
		want := "func f() {\n    if x {\n        y()\n    }\n}"
		if strings.TrimRight(got, "\n") != want {
			t.Errorf("hl=%v: display text =\n%q\nwant\n%q", hl, got, want)
		}
		if cp := mdCodeCopyText(blocks[0]); !strings.Contains(cp, "\t\ty()") {
			t.Errorf("hl=%v: copy text lost its tabs: %q", hl, cp)
		}
	}
}

// Expansion is column-aware: a tab after text moves to the next stop,
// not a fixed four spaces, and the column resets at each newline and
// carries across run boundaries.
func TestMdExpandTabsColumns(t *testing.T) {
	runs := []RichTextRun{
		{Text: "ab\tc"}, {Text: "\td\n"}, {Text: "\te"},
	}
	out, _ := mdExpandCodeTabs(runs)
	var sb strings.Builder
	for _, r := range out {
		sb.WriteString(r.Text)
	}
	want := "ab  c   d\n    e"
	if sb.String() != want {
		t.Errorf("expanded = %q, want %q", sb.String(), want)
	}
	// The column counts runes, not bytes, and a tab at a stop moves a
	// full stop.
	for in, exp := range map[string]string{
		"é\tx":    "é   x",
		"abcd\tx": "abcd    x",
		"\t":      "    ",
	} {
		got, _ := mdExpandCodeTabs([]RichTextRun{{Text: in}})
		if got[0].Text != exp {
			t.Errorf("%q expanded = %q, want %q", in, got[0].Text, exp)
		}
	}
	if got, changed := mdExpandCodeTabs(nil); changed || got != nil {
		t.Errorf("nil runs = %v, changed %v", got, changed)
	}
	// No tab: the runs come back as they were, without a copy.
	plain := []RichTextRun{{Text: "x\ny"}}
	if got, changed := mdExpandCodeTabs(plain); changed || &got[0] != &plain[0] {
		t.Error("tab-free runs were copied")
	}
}

// A render hook sees the source with its tabs: only the drawn Content
// is expanded. This pins the hook's PlainText for a go fence and a
// mermaid fence. A hooked block never reaches renderMdMermaid, so
// TestMdMermaidHashesTabbedSource pins the diagram source.
func TestMdCodeHookPlainTextKeepsTabs(t *testing.T) {
	for _, lang := range []string{"go", "mermaid"} {
		src := "```" + lang + "\na\n\tb\n```\n"
		var got string
		w := NewTestWindow(t, WindowCfg{State: new(int)})
		_ = generateViewLayout(w.Markdown(MarkdownCfg{
			ID:     "md",
			Source: src,
			RenderBlock: func(_ *Window, el MarkdownElement) (View, bool) {
				if el.Kind == MarkdownKindCode {
					got = el.PlainText
				}
				return nil, false
			},
		}), w)
		if !strings.Contains(got, "\tb") {
			t.Errorf("%s: hook PlainText = %q, want the tab kept", lang, got)
		}
	}
}

// The marker floor counts the drawn (trimmed) marker. With the stub's
// glyph width equal to PrefixCharWidth, the floor and the ink agree,
// so the text starts SpacingSmall after the ink. Counting the trailing
// space kept its slot and pushed the text one PrefixCharWidth further.
func TestMdListItemFloorMatchesTrimmedMarker(t *testing.T) {
	cfg := MarkdownCfg{Style: DefaultMarkdownStyle()}
	w := NewTestWindow(t, WindowCfg{State: new(int)})
	w.textMeasurer = &stubTextMeasurer{
		charWidth: cfg.Style.PrefixCharWidth, fontHeight: 16}
	for _, prefix := range []string{"• ", "1. ", "10. "} {
		layout := generateViewLayout(mdRenderListItem(
			markdownBlock{ListPrefix: prefix}, cfg,
			TextModeSingleLine, nil), w)
		layoutPipeline(&layout, w)
		ink := layout.Children[0].Children[0].Shape
		text := layout.Children[1].Shape
		if gap := text.X - (ink.X + ink.Width); gap != gapSmall {
			t.Errorf("%q: ink-to-text gap = %v, want %v", prefix, gap, gapSmall)
		}
	}
}

// A first-level item is indented one NestIndent, as browsers and
// GitHub indent a top-level list; each nesting level adds one more.
func TestMdListItemIndent(t *testing.T) {
	cfg := MarkdownCfg{Style: DefaultMarkdownStyle()}
	for level := range 3 {
		layout := mdSpacingLayout(t, mdRenderListItem(
			markdownBlock{ListPrefix: "• ", ListIndent: level}, cfg,
			TextModeSingleLine, nil))
		marker := layout.Children[0].Shape
		want := float32(level+1) * cfg.Style.NestIndent
		if got := marker.X - layout.Shape.X; got != want {
			t.Errorf("level %d: marker at %v, want %v", level, got, want)
		}
	}
}

// renderMdMermaid hashes and fetches the source with its tabs, not the
// expanded display text: a cache entry seeded under the tabbed
// source's hash must be the one the render finds. Hashing Content
// would miss it and start a fetch of the wrong source instead.
func TestMdMermaidHashesTabbedSource(t *testing.T) {
	prev := markdownExternalAPIsEnabled
	SetMarkdownExternalAPIsEnabled(true)
	t.Cleanup(func() { SetMarkdownExternalAPIsEnabled(prev) })

	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks("```mermaid\ngraph TD\n\tA-->B\n```\n", style)
	if len(blocks) != 1 || blocks[0].CodeLanguage != "mermaid" {
		t.Fatalf("want one mermaid block, got %+v", blocks)
	}
	src := "graph TD\n\tA-->B"
	if got := strings.TrimRight(mdCodeCopyText(blocks[0]), "\n"); got != src {
		t.Fatalf("block source = %q, want %q", got, src)
	}

	w := NewTestWindow(t, WindowCfg{State: new(int)})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 16}
	ensureDiagramCache(w).Set(diagramCacheHash(mdCodeCopyText(blocks[0])),
		diagramCacheEntry{State: diagramLoading})
	layout := generateViewLayout(renderMdMermaid(blocks[0],
		MarkdownCfg{Style: style}, w), w)
	if layout.Shape.TC == nil || layout.Shape.TC.Text != "Loading diagram..." {
		t.Error("render missed the cache entry for the tabbed source")
	}
}
