package gui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// --- Test helpers ---

func markdownLayoutForSource(t *testing.T, source string) Layout {
	t.Helper()

	w := &Window{}
	return generateViewLayout(w.Markdown(MarkdownCfg{
		Source: source,
		Style:  DefaultMarkdownStyle(),
	}), w)
}

// --- Styled output ---

func TestSanitizeLatex(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"E = mc^2", "E = mc^2"},
		{"\\input{secrets}", "{secrets}"},
		{"\\def\\x{y}", "\\x{y}"},
		{"\\write18{cmd}", "{cmd}"},
		{"\\sum_{n=1}^{\\infty}\r\n\\frac{1}{n^2}",
			"\\sum_{n=1}^{\\infty} \\frac{1}{n^2}"},
	}
	for _, tc := range tests {
		got := sanitizeLatex(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeLatex(%q): got %q, want %q",
				tc.input, got, tc.want)
		}
	}
}

func TestMarkdownToRichText(t *testing.T) {
	style := DefaultMarkdownStyle()
	rt := MarkdownToRichText("**bold** and *italic*", style)
	if len(rt.Runs) == 0 {
		t.Fatal("expected runs")
	}
	text := richTextPlain(rt)
	if !strings.Contains(text, "bold") {
		t.Error("expected bold text")
	}
	if !strings.Contains(text, "italic") {
		t.Error("expected italic text")
	}
}

func TestMarkdownToBlocksCode(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks("```\ncode\n```", style)
	found := false
	for _, b := range blocks {
		if b.IsCode {
			found = true
		}
	}
	if !found {
		t.Error("expected code block in styled output")
	}
}

func TestMarkdownToBlocksTable(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks(
		"| A | B |\n|---|---|\n| 1 | 2 |", style)
	found := false
	for _, b := range blocks {
		if b.IsTable && b.TableData != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected table block in styled output")
	}
}

func TestMarkdownStyledLinkColor(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks(
		"[link](https://example.com)", style)
	for _, b := range blocks {
		for _, r := range b.Content.Runs {
			if r.Link == "https://example.com" {
				if r.Style.Color != style.LinkColor {
					t.Error("link should use LinkColor")
				}
				if !r.Style.Underline {
					t.Error("link should be underlined")
				}
			}
		}
	}
}

func TestMarkdownStyledHighlightBG(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks("==marked==", style)
	for _, b := range blocks {
		for _, r := range b.Content.Runs {
			if strings.TrimSpace(r.Text) == "marked" {
				if r.Style.BgColor != style.HighlightBG {
					t.Error("highlight should use HighlightBG")
				}
			}
		}
	}
}

func TestMarkdownStyledStrikethrough(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks("~~deleted~~", style)
	for _, b := range blocks {
		for _, r := range b.Content.Runs {
			if strings.TrimSpace(r.Text) == "deleted" {
				if !r.Style.Strikethrough {
					t.Error("strikethrough not set")
				}
			}
		}
	}
}

func TestMarkdownBuildTableData(t *testing.T) {
	style := DefaultMarkdownStyle()
	blocks := markdownToBlocks(
		"| H1 | H2 |\n|---|---|\n| a | b |", style)
	for _, b := range blocks {
		if b.IsTable && b.TableData != nil {
			rows := buildMarkdownTableData(
				*b.TableData, style)
			if len(rows) < 2 {
				t.Fatal("expected header + data row")
			}
			// Header row cells.
			if len(rows[0].Cells) != 2 {
				t.Fatalf("header cells: got %d",
					len(rows[0].Cells))
			}
			if !rows[0].Cells[0].HeadCell {
				t.Error("first header cell not marked as head")
			}
		}
	}
}

func TestMarkdownDefaultsToWrap(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.Markdown(MarkdownCfg{
		Source: "A paragraph that should wrap by default.",
		Style:  DefaultMarkdownStyle(),
	}), w)

	if layout.Shape == nil || layout.Shape.Sizing != FillFit {
		t.Fatalf("markdown sizing = %v, want FillFit", layout.Shape.Sizing)
	}

	mode, ok := firstMarkdownTextMode(layout)
	if !ok {
		t.Fatal("expected markdown text node")
	}
	if mode != TextModeWrap {
		t.Fatalf("markdown text mode = %v, want TextModeWrap", mode)
	}
}

func TestMarkdownCanExplicitlyUseSingleLine(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.Markdown(MarkdownCfg{
		Source: "single line",
		Style:  DefaultMarkdownStyle(),
		Mode:   Some(TextModeSingleLine),
	}), w)

	if layout.Shape == nil || layout.Shape.Sizing != FitFit {
		t.Fatalf("markdown sizing = %v, want FitFit", layout.Shape.Sizing)
	}

	mode, ok := firstMarkdownTextMode(layout)
	if !ok {
		t.Fatal("expected markdown text node")
	}
	if mode != TextModeSingleLine {
		t.Fatalf("markdown text mode = %v, want TextModeSingleLine", mode)
	}
}

func TestMarkdownListWrappersHaveNoBorder(t *testing.T) {
	layout := markdownLayoutForSource(t, "- item")
	if len(layout.Children) == 0 {
		t.Fatal("len(layout.Children) = 0, want list wrapper")
	}

	list := layout.Children[0]
	if got, want := list.Shape.SizeBorder, float32(0); !f32AreClose(got, want) {
		t.Fatalf("layout.Children[0].Shape.SizeBorder = %v, want %v", got, want)
	}
	if len(list.Children) == 0 {
		t.Fatal("len(layout.Children[0].Children) = 0, want list row")
	}

	row := list.Children[0]
	if got, want := row.Shape.SizeBorder, float32(0); !f32AreClose(got, want) {
		t.Fatalf("layout.Children[0].Children[0].Shape.SizeBorder = %v, want %v", got, want)
	}
	if len(row.Children) != 2 {
		t.Fatalf("len(layout.Children[0].Children[0].Children) = %d, want 2", len(row.Children))
	}

	for i, child := range row.Children {
		if got, want := child.Shape.SizeBorder, float32(0); !f32AreClose(got, want) {
			t.Fatalf("layout.Children[0].Children[0].Children[%d].Shape.SizeBorder = %v, want %v", i, got, want)
		}
	}
}

func TestMarkdownBlockquoteWrappersHaveNoBorder(t *testing.T) {
	layout := markdownLayoutForSource(t, "> quote")
	if len(layout.Children) == 0 {
		t.Fatal("len(layout.Children) = 0, want blockquote row")
	}

	row := layout.Children[0]
	if got, want := row.Shape.SizeBorder, float32(0); !f32AreClose(got, want) {
		t.Fatalf("layout.Children[0].Shape.SizeBorder = %v, want %v", got, want)
	}
	if len(row.Children) != 2 {
		t.Fatalf("len(layout.Children[0].Children) = %d, want 2", len(row.Children))
	}

	quote := row.Children[1]
	if got, want := quote.Shape.SizeBorder, float32(0); !f32AreClose(got, want) {
		t.Fatalf("layout.Children[0].Children[1].Shape.SizeBorder = %v, want %v", got, want)
	}
}

func TestMarkdownCodeBlocksHaveNoBorder(t *testing.T) {
	layout := markdownLayoutForSource(t, "```\ncode\n```")
	if len(layout.Children) == 0 {
		t.Fatal("len(layout.Children) = 0, want code block")
	}

	code := layout.Children[0]
	if got, want := code.Shape.SizeBorder, float32(0); !f32AreClose(got, want) {
		t.Fatalf("layout.Children[0].Shape.SizeBorder = %v, want %v", got, want)
	}
}

func TestRenderMdMathErrorsWrap(t *testing.T) {
	prev := markdownExternalAPIsEnabled
	SetMarkdownExternalAPIsEnabled(true)
	t.Cleanup(func() {
		SetMarkdownExternalAPIsEnabled(prev)
	})

	style := DefaultMarkdownStyle()
	block := MarkdownBlock{
		IsMath:    true,
		MathLatex: `\badcommand{this error should wrap}`,
	}
	hash := diagramCacheHash(
		fmt.Sprintf("display_%d",
			markdown.MathHash(block.MathLatex)),
	)
	errText := strings.Repeat("codecogs api error ", 8)

	w := &Window{}
	w.viewState.diagramCache = NewBoundedDiagramCache(50)
	w.viewState.diagramCache.Set(hash, DiagramCacheEntry{
		State: DiagramError,
		Error: errText,
	})

	layout := generateViewLayout(renderMdMath(block, MarkdownCfg{
		Style: style,
	}, w), w)

	mode, ok := findTextModeByText(layout, errText)
	if !ok {
		t.Fatal("expected markdown math error text")
	}
	if mode != TextModeWrap {
		t.Fatalf("math error text mode = %v, want TextModeWrap", mode)
	}
}

func firstMarkdownTextMode(layout Layout) (TextMode, bool) {
	if layout.Shape != nil && layout.Shape.TC != nil {
		return layout.Shape.TC.TextMode, true
	}
	for _, child := range layout.Children {
		if mode, ok := firstMarkdownTextMode(child); ok {
			return mode, true
		}
	}
	return 0, false
}

func findTextModeByText(layout Layout, text string) (TextMode, bool) {
	if layout.Shape != nil && layout.Shape.TC != nil &&
		layout.Shape.TC.Text == text {
		return layout.Shape.TC.TextMode, true
	}
	for _, child := range layout.Children {
		if mode, ok := findTextModeByText(child, text); ok {
			return mode, true
		}
	}
	return 0, false
}
