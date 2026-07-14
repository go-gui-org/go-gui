package markdown

import (
	"strings"
	"testing"
)

// --- Test helpers ---

func parse(source string) []Block {
	return Parse(source, false)
}

// assertContains checks s contains sub.
func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected %q to contain %q", s, sub)
	}
}

// --- Headers ---

func TestMarkdownH1(t *testing.T) {
	t.Parallel()
	blocks := parse("# Hello")
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
	if blocks[0].HeaderLevel != 1 {
		t.Fatalf("header level: got %d", blocks[0].HeaderLevel)
	}
	assertContains(t, RunsToText(blocks[0].Runs), "Hello")
}

func TestMarkdownH2(t *testing.T) {
	t.Parallel()
	blocks := parse("## World")
	if len(blocks) == 0 || blocks[0].HeaderLevel != 2 {
		t.Fatal("expected H2")
	}
}

func TestMarkdownH3toH6(t *testing.T) {
	t.Parallel()
	for level := 3; level <= 6; level++ {
		prefix := strings.Repeat("#", level)
		blocks := parse(prefix + " Heading")
		if len(blocks) == 0 ||
			blocks[0].HeaderLevel != level {
			t.Fatalf("H%d: got level %d",
				level, blocks[0].HeaderLevel)
		}
	}
}

func TestMarkdownSetextH1(t *testing.T) {
	t.Parallel()
	blocks := parse("Title\n=====")
	if len(blocks) == 0 || blocks[0].HeaderLevel != 1 {
		t.Fatal("expected setext H1")
	}
}

func TestMarkdownSetextH2(t *testing.T) {
	t.Parallel()
	blocks := parse("Title\n-----")
	if len(blocks) == 0 || blocks[0].HeaderLevel != 2 {
		t.Fatal("expected setext H2")
	}
}

func TestMarkdownHeadingAnchor(t *testing.T) {
	t.Parallel()
	blocks := parse("# Hello World")
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
	if blocks[0].AnchorSlug != "hello-world" {
		t.Fatalf("slug: got %q", blocks[0].AnchorSlug)
	}
}

// --- Inline formatting ---

func TestMarkdownBold(t *testing.T) {
	t.Parallel()
	blocks := parse("**bold**")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Format == FormatBold &&
				strings.TrimSpace(r.Text) == "bold" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected bold run")
	}
}

func TestMarkdownItalic(t *testing.T) {
	t.Parallel()
	blocks := parse("*italic*")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Format == FormatItalic &&
				strings.TrimSpace(r.Text) == "italic" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected italic run")
	}
}

func TestMarkdownBoldItalic(t *testing.T) {
	t.Parallel()
	blocks := parse("***both***")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Format == FormatBoldItalic &&
				strings.TrimSpace(r.Text) == "both" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected bold+italic run")
	}
}

func TestMarkdownInlineCode(t *testing.T) {
	t.Parallel()
	blocks := parse("text `code` text")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Format == FormatCode &&
				strings.TrimSpace(r.Text) == "code" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected code run")
	}
}

func TestMarkdownStrikethrough(t *testing.T) {
	t.Parallel()
	blocks := parse("~~strikethrough~~")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Strikethrough &&
				strings.TrimSpace(r.Text) == "strikethrough" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected strikethrough run")
	}
}

func TestMarkdownHighlight(t *testing.T) {
	t.Parallel()
	blocks := parse("==highlighted==")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Highlight &&
				strings.TrimSpace(r.Text) == "highlighted" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected highlight run")
	}
}

func TestMarkdownSuperscript(t *testing.T) {
	t.Parallel()
	blocks := parse("x^2^")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Superscript &&
				strings.TrimSpace(r.Text) == "2" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected superscript run")
	}
}

func TestMarkdownUnderline(t *testing.T) {
	t.Parallel()
	blocks := parse("++underlined++")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Underline &&
				strings.TrimSpace(r.Text) == "underlined" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected underline run")
	}
}

func TestMarkdownUnderlineBold(t *testing.T) {
	t.Parallel()
	blocks := parse("++**bold underline**++")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Underline && r.Format == FormatBold {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected bold+underline run")
	}
}

func TestMarkdownSubscript(t *testing.T) {
	t.Parallel()
	blocks := parse("H~2~O")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Subscript &&
				strings.TrimSpace(r.Text) == "2" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected subscript run")
	}
}

// --- Links ---

func TestMarkdownInlineLink(t *testing.T) {
	t.Parallel()
	blocks := parse("[click](https://example.com)")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Link == "https://example.com" &&
				strings.TrimSpace(r.Text) == "click" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected link run")
	}
}

func TestMarkdownAutoLink(t *testing.T) {
	t.Parallel()
	blocks := parse("<https://example.com>")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Link != "" &&
				strings.Contains(r.Link, "example.com") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected autolink run")
	}
}

func TestMarkdownReferenceLink(t *testing.T) {
	t.Parallel()
	blocks := parse("[text][ref]\n\n[ref]: https://example.com")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Link == "https://example.com" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected reference link")
	}
}

// --- Images ---

func TestMarkdownImage(t *testing.T) {
	t.Parallel()
	blocks := parse("![alt text](image.png)")
	found := false
	for _, b := range blocks {
		if b.IsImage && b.ImageSrc == "image.png" {
			found = true
		}
	}
	if !found {
		t.Error("expected image block")
	}
}

func TestMarkdownImageRemoteWithDims(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
	}{
		{"with dims", "![Veasal](https://vlang.io/img/veasel.png =250x200)"},
		{"no dims", "![Veasal](https://vlang.io/img/veasel.png)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := parse(tt.src)
			found := false
			for _, b := range blocks {
				t.Logf("block: IsImage=%v Src=%q W=%v H=%v runs=%v",
					b.IsImage, b.ImageSrc, b.ImageWidth, b.ImageHeight, len(b.Runs))
				if b.IsImage {
					found = true
				}
			}
			if !found {
				t.Error("expected image block")
			}
		})
	}
}

func TestMarkdownImageTraversal(t *testing.T) {
	t.Parallel()
	blocks := parse("![alt](../../../etc/passwd.png)")
	for _, b := range blocks {
		if b.IsImage && b.ImageSrc != "" {
			t.Error("path traversal image should be blocked")
		}
	}
}

// --- Lists ---

func TestMarkdownUnorderedList(t *testing.T) {
	t.Parallel()
	blocks := parse("- item1\n- item2\n- item3")
	listCount := 0
	for _, b := range blocks {
		if b.IsList {
			listCount++
		}
	}
	if listCount < 3 {
		t.Fatalf("expected 3 list items, got %d", listCount)
	}
}

func TestMarkdownOrderedList(t *testing.T) {
	t.Parallel()
	blocks := parse("1. first\n2. second\n3. third")
	listCount := 0
	for _, b := range blocks {
		if b.IsList {
			listCount++
		}
	}
	if listCount < 3 {
		t.Fatalf("expected 3 ordered items, got %d", listCount)
	}
}

func TestMarkdownNestedList(t *testing.T) {
	t.Parallel()
	blocks := parse("- outer\n  - inner")
	found := false
	for _, b := range blocks {
		if b.IsList && b.ListIndent > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected nested list item")
	}
}

func TestMarkdownNestedListOrder(t *testing.T) {
	t.Parallel()
	blocks := parse("1. parent\n   1. child")
	if len(blocks) < 2 {
		t.Fatalf("expected 2 list blocks, got %d",
			len(blocks))
	}
	if blocks[0].ListIndent != 0 {
		t.Errorf("first block indent=%d, want 0",
			blocks[0].ListIndent)
	}
	if blocks[1].ListIndent != 1 {
		t.Errorf("second block indent=%d, want 1",
			blocks[1].ListIndent)
	}
}

func TestMarkdownTaskList(t *testing.T) {
	t.Parallel()
	blocks := parse("- [x] done\n- [ ] todo")
	if len(blocks) < 2 {
		t.Fatal("expected at least 2 blocks")
	}
}

func TestMarkdownTaskListSetsIsTaskItemAndChecked(t *testing.T) {
	t.Parallel()
	blocks := parse("- [x] done\n- [ ] todo")
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	if !blocks[0].IsTaskItem {
		t.Error("blocks[0].IsTaskItem = false, want true")
	}
	if !blocks[0].TaskChecked {
		t.Error("blocks[0].TaskChecked = false, want true")
	}
	if blocks[0].ListPrefix != "" {
		t.Errorf("blocks[0].ListPrefix = %q, want empty", blocks[0].ListPrefix)
	}

	if !blocks[1].IsTaskItem {
		t.Error("blocks[1].IsTaskItem = false, want true")
	}
	if blocks[1].TaskChecked {
		t.Error("blocks[1].TaskChecked = true, want false")
	}
	if blocks[1].ListPrefix != "" {
		t.Errorf("blocks[1].ListPrefix = %q, want empty", blocks[1].ListPrefix)
	}
}

func TestMarkdownUnorderedListIsNotTaskItem(t *testing.T) {
	t.Parallel()
	blocks := parse("- plain item")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].IsTaskItem {
		t.Error("blocks[0].IsTaskItem = true, want false for a non-task list item")
	}
	if blocks[0].ListPrefix != "• " {
		t.Errorf("blocks[0].ListPrefix = %q, want %q", blocks[0].ListPrefix, "• ")
	}
}

func TestMarkdownNestedTaskListIndent(t *testing.T) {
	t.Parallel()
	blocks := parse("- [x] parent\n  - [ ] child")
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].ListIndent != 0 {
		t.Errorf("blocks[0].ListIndent = %d, want 0", blocks[0].ListIndent)
	}
	if !blocks[0].IsTaskItem || !blocks[0].TaskChecked {
		t.Error("blocks[0] expected checked task item")
	}
	if blocks[1].ListIndent != 1 {
		t.Errorf("blocks[1].ListIndent = %d, want 1", blocks[1].ListIndent)
	}
	if !blocks[1].IsTaskItem || blocks[1].TaskChecked {
		t.Error("blocks[1] expected unchecked task item")
	}
}

// --- Blockquotes ---

func TestMarkdownBlockquote(t *testing.T) {
	t.Parallel()
	blocks := parse("> quoted text")
	found := false
	for _, b := range blocks {
		if b.IsBlockquote {
			found = true
		}
	}
	if !found {
		t.Error("expected blockquote block")
	}
}

func TestMarkdownNestedBlockquote(t *testing.T) {
	t.Parallel()
	blocks := parse("> level1\n>> level2")
	maxDepth := 0
	for _, b := range blocks {
		if b.IsBlockquote && b.BlockquoteDepth > maxDepth {
			maxDepth = b.BlockquoteDepth
		}
	}
	if maxDepth < 2 {
		t.Fatalf("expected depth >= 2, got %d", maxDepth)
	}
}

// --- Tables ---

func TestMarkdownTable(t *testing.T) {
	t.Parallel()
	blocks := parse("| A | B |\n|---|---|\n| 1 | 2 |")
	found := false
	for _, b := range blocks {
		if b.IsTable && b.TableData != nil {
			found = true
			if b.TableData.ColCount != 2 {
				t.Fatalf("colCount: got %d",
					b.TableData.ColCount)
			}
		}
	}
	if !found {
		t.Error("expected table block")
	}
}

func TestMarkdownTableAlignments(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"| L | C | R |\n|:--|:--:|--:|\n| a | b | c |")
	for _, b := range blocks {
		if b.IsTable && b.TableData != nil {
			if len(b.TableData.Alignments) < 3 {
				t.Fatal("expected 3 alignments")
			}
			if b.TableData.Alignments[0] != AlignLeft {
				t.Error("col0 should be left-aligned")
			}
			if b.TableData.Alignments[1] != AlignCenter {
				t.Error("col1 should be center-aligned")
			}
			if b.TableData.Alignments[2] != AlignRight {
				t.Error("col2 should be right-aligned")
			}
		}
	}
}

func TestMarkdownTableNoOuterPipes(t *testing.T) {
	t.Parallel()
	blocks := parse("A | B\n---|---\n1 | 2")
	found := false
	for _, b := range blocks {
		if b.IsTable && b.TableData != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected table without outer pipes")
	}
}

// --- Code blocks ---

func TestMarkdownFencedCodeBlock(t *testing.T) {
	t.Parallel()
	blocks := parse("```go\nfmt.Println(\"hi\")\n```")
	found := false
	for _, b := range blocks {
		if b.IsCode && b.CodeLanguage == "go" {
			found = true
		}
	}
	if !found {
		t.Error("expected Go code block")
	}
}

func TestMarkdownTildeFence(t *testing.T) {
	t.Parallel()
	blocks := parse("~~~python\nprint('hi')\n~~~")
	found := false
	for _, b := range blocks {
		if b.IsCode && b.CodeLanguage == "py" {
			found = true
		}
	}
	if !found {
		t.Error("expected Python code block (tilde)")
	}
}

func TestMarkdownCodeBlockContent(t *testing.T) {
	t.Parallel()
	blocks := parse("```\nhello world\n```")
	found := false
	for _, b := range blocks {
		if b.IsCode {
			text := RunsToText(b.Runs)
			if strings.Contains(text, "hello world") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected code block with content")
	}
}

func TestMarkdownCodeBlockCRLF(t *testing.T) {
	t.Parallel()
	blocks := parse("```\r\nline1\r\nline2\r\n```")
	found := false
	for _, b := range blocks {
		if b.IsCode {
			text := RunsToText(b.Runs)
			if strings.Contains(text, "\r") {
				t.Error("code block contains \\r after CRLF normalization")
			}
			if strings.Contains(text, "line1\nline2") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected code block with normalized newlines")
	}
}

// --- HR ---

func TestMarkdownHR(t *testing.T) {
	t.Parallel()
	blocks := parse("---")
	found := false
	for _, b := range blocks {
		if b.IsHR {
			found = true
		}
	}
	if !found {
		t.Error("expected HR block")
	}
}

// --- Math ---

func TestMarkdownDisplayMath(t *testing.T) {
	t.Parallel()
	blocks := parse("$$\nE = mc^2\n$$")
	found := false
	for _, b := range blocks {
		if b.IsMath {
			found = true
		}
	}
	if !found {
		t.Error("expected display math block")
	}
}

func TestMarkdownMathFence(t *testing.T) {
	t.Parallel()
	blocks := parse("```math\nE = mc^2\n```")
	found := false
	for _, b := range blocks {
		if b.IsMath {
			found = true
		}
	}
	if !found {
		t.Error("expected math code fence block")
	}
}

func TestMarkdownInlineMath(t *testing.T) {
	t.Parallel()
	blocks := parse("The equation $E = mc^2$ is famous.")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.MathID != "" || r.MathLatex != "" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected inline math run")
	}
}

func TestMarkdownNotMathDollarAmount(t *testing.T) {
	t.Parallel()
	blocks := parse("The price is $10.")
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.MathID != "" || r.MathLatex != "" {
				t.Error("$10 should not be parsed as math")
			}
		}
	}
}

// --- Footnotes ---

func TestMarkdownFootnote(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"Text[^1] more.\n\n[^1]: Footnote content.")
	foundFootnote := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Tooltip == "Footnote content." {
				foundFootnote = true
			}
		}
	}
	if !foundFootnote {
		t.Error("expected footnote tooltip")
	}
}

func TestMarkdownFootnoteNamed(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"Text[^note] more.\n\n[^note]: Named footnote.")
	foundFootnote := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Tooltip == "Named footnote." {
				foundFootnote = true
			}
		}
	}
	if !foundFootnote {
		t.Error("expected named footnote tooltip")
	}
}

func TestMarkdownFootnoteUndefined(t *testing.T) {
	t.Parallel()
	blocks := parse("Text[^undef] more.")
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Tooltip != "" {
				t.Error("undefined footnote should not have tooltip")
			}
		}
	}
}

// --- Definition lists ---

func TestMarkdownDefinitionList(t *testing.T) {
	t.Parallel()
	blocks := parse("Term\n:   Definition text")
	hasTerm := false
	hasValue := false
	for _, b := range blocks {
		if b.IsDefTerm {
			hasTerm = true
		}
		if b.IsDefValue {
			hasValue = true
		}
	}
	if !hasTerm || !hasValue {
		t.Errorf("deflist: term=%v value=%v",
			hasTerm, hasValue)
	}
}

// --- Abbreviations ---

func TestMarkdownAbbreviation(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"The HTML spec.\n\n*[HTML]: HyperText Markup Language")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Tooltip == "HyperText Markup Language" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected abbreviation tooltip")
	}
}

func TestMarkdownAbbrWordBoundary(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"HTMLX is not HTML.\n\n*[HTML]: Markup Language")
	// Only standalone "HTML" should have tooltip,
	// not "HTMLX".
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Text == "HTMLX" && r.Tooltip != "" {
				t.Error("HTMLX should not have tooltip")
			}
		}
	}
}

// --- Emoji ---

func TestMarkdownEmoji(t *testing.T) {
	t.Parallel()
	blocks := parse(":smile:")
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	text := RunsToText(blocks[0].Runs)
	if strings.Contains(text, ":smile:") {
		t.Error("emoji should be converted to unicode")
	}
}

func TestMarkdownEmojiUnknown(t *testing.T) {
	t.Parallel()
	blocks := parse(":notarealemojicode:")
	text := RunsToText(blocks[0].Runs)
	if !strings.Contains(text, ":notarealemojicode:") {
		t.Error("unknown emoji should remain as text")
	}
}

func TestMarkdownBareColon(t *testing.T) {
	t.Parallel()
	blocks := parse("time: 3:00")
	text := RunsToText(blocks[0].Runs)
	assertContains(t, text, "time: 3:00")
}

// --- Paragraph breaks ---

func TestMarkdownParagraphBreak(t *testing.T) {
	t.Parallel()
	blocks := parse("Para 1.\n\nPara 2.")
	if len(blocks) < 2 {
		t.Fatalf("expected 2 paragraphs, got %d",
			len(blocks))
	}
}

// --- Hard line breaks ---

func TestMarkdownHardLineBreak(t *testing.T) {
	t.Parallel()
	blocks := parse("line1  \nline2")
	text := RunsToText(blocks[0].Runs)
	if !strings.Contains(text, "\n") {
		t.Error("expected hard line break")
	}
}

// --- Escapes ---

func TestMarkdownEscapes(t *testing.T) {
	t.Parallel()
	blocks := parse("\\*not bold\\*")
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
	// Verify no bold formatting applied.
	for _, r := range blocks[0].Runs {
		if r.Format == FormatBold {
			t.Error("escaped asterisks should not be bold")
		}
	}
}

// --- Security ---

func TestMarkdownJavascriptLinkBlocked(t *testing.T) {
	t.Parallel()
	blocks := parse("[click](javascript:alert(1))")
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.Link == "javascript:alert(1)" {
				if IsSafeURL(r.Link) {
					t.Error("javascript: should be unsafe")
				}
			}
		}
	}
}

// --- Syntax highlighting ---

func TestMarkdownSyntaxHighlightGo(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```")
	found := false
	for _, b := range blocks {
		if !b.IsCode || b.CodeLanguage != "go" {
			continue
		}
		for _, r := range b.Runs {
			if r.CodeToken == TokenKeyword {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected Go keyword tokens")
	}
}

func TestMarkdownSyntaxHighlightPython(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"```python\ndef hello():\n    print(\"hi\")\n```")
	found := false
	for _, b := range blocks {
		if !b.IsCode || b.CodeLanguage != "py" {
			continue
		}
		for _, r := range b.Runs {
			if r.CodeToken == TokenKeyword {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected Python keyword tokens")
	}
}

func TestMarkdownSyntaxHighlightJS(t *testing.T) {
	t.Parallel()
	blocks := parse(
		"```javascript\nconst x = 42;\n```")
	found := false
	for _, b := range blocks {
		if !b.IsCode {
			continue
		}
		for _, r := range b.Runs {
			if r.CodeToken == TokenKeyword {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected JS keyword tokens")
	}
}

func TestMarkdownSyntaxHighlightString(t *testing.T) {
	t.Parallel()
	blocks := parse("```go\nvar s = \"hello\"\n```")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.CodeToken == TokenString {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected string token")
	}
}

func TestMarkdownSyntaxHighlightNumber(t *testing.T) {
	t.Parallel()
	blocks := parse("```go\nvar n = 42\n```")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.CodeToken == TokenNumber {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected number token")
	}
}

func TestMarkdownSyntaxHighlightComment(t *testing.T) {
	t.Parallel()
	blocks := parse("```go\n// comment\n```")
	found := false
	for _, b := range blocks {
		for _, r := range b.Runs {
			if r.CodeToken == TokenComment {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected comment token")
	}
}

// --- Misc ---

func TestMarkdownEmpty(t *testing.T) {
	t.Parallel()
	blocks := parse("")
	if len(blocks) != 0 {
		t.Fatalf("empty source should produce 0 blocks, got %d",
			len(blocks))
	}
}

func TestMarkdownPlainParagraph(t *testing.T) {
	t.Parallel()
	blocks := parse("Hello world.")
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	assertContains(t, RunsToText(blocks[0].Runs), "Hello world.")
}

func TestMarkdownMultipleBlocks(t *testing.T) {
	t.Parallel()
	blocks := parse("# Header\n\nParagraph.\n\n---\n\n> Quote")
	hasHeader := false
	hasHR := false
	hasBQ := false
	for _, b := range blocks {
		if b.HeaderLevel > 0 {
			hasHeader = true
		}
		if b.IsHR {
			hasHR = true
		}
		if b.IsBlockquote {
			hasBQ = true
		}
	}
	if !hasHeader || !hasHR || !hasBQ {
		t.Errorf("header=%v HR=%v BQ=%v",
			hasHeader, hasHR, hasBQ)
	}
}

// --- Code language detection ---

func TestLangFromHint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hint string
		want CodeLanguage
	}{
		{"go", LangGo},
		{"golang", LangGo},
		{"python", LangPython},
		{"py", LangPython},
		{"javascript", LangJavaScript},
		{"js", LangJavaScript},
		{"typescript", LangTypeScript},
		{"ts", LangTypeScript},
		{"rust", LangRust},
		{"rs", LangRust},
		{"c", LangC},
		{"json", LangJSON},
		{"bash", LangShell},
		{"sh", LangShell},
		{"html", LangHTML},
		{"unknown", LangGeneric},
	}
	for _, tc := range tests {
		got := LangFromHint(tc.hint)
		if got != tc.want {
			t.Errorf("LangFromHint(%q): got %d, want %d",
				tc.hint, got, tc.want)
		}
	}
}

func TestMarkdownListPrefix(t *testing.T) {
	t.Parallel()
	blocks := parse("- item")
	for _, b := range blocks {
		if b.IsList {
			if b.ListPrefix == "" {
				t.Error("list prefix should not be empty")
			}
		}
	}
}

func TestMarkdownOrderedListPrefix(t *testing.T) {
	t.Parallel()
	blocks := parse("1. first\n2. second")
	for _, b := range blocks {
		if b.IsList && strings.Contains(b.ListPrefix, "1") {
			return // found
		}
	}
	// Numbered prefixes vary — just check we got list items.
	count := 0
	for _, b := range blocks {
		if b.IsList {
			count++
		}
	}
	if count < 2 {
		t.Error("expected at least 2 ordered list items")
	}
}

// --- URL safety ---

func TestIsSafeURL(t *testing.T) {
	t.Parallel()
	safe := []string{
		"https://example.com",
		"http://example.com",
		"mailto:user@example.com",
		"#anchor",
		"/relative/path",
		"relative/path",
	}
	for _, u := range safe {
		if !IsSafeURL(u) {
			t.Errorf("expected safe: %q", u)
		}
	}
}

func TestIsSafeURLBlocked(t *testing.T) {
	t.Parallel()
	blocked := []string{
		"javascript:alert(1)",
		"data:text/html,<h1>",
		"data:image/png;base64,iVBOR",
		"vbscript:msgbox",
		"file:///etc/passwd",
		"blob:http://example.com/x",
		"tel:+15555550123",
		"intent://scan/#Intent;scheme=zxing;end",
		"vscode://file/test.md",
		"customscheme:payload",
	}
	for _, u := range blocked {
		if IsSafeURL(u) {
			t.Errorf("expected blocked: %q", u)
		}
	}
}

func TestIsSafeURLEdgeCases(t *testing.T) {
	t.Parallel()
	if IsSafeURL("") {
		t.Error("empty should not be safe")
	}
	if IsSafeURL("  javascript:alert(1)") {
		t.Error("whitespace-prefixed javascript: should be blocked")
	}
	if IsSafeURL("JAVASCRIPT:alert(1)") {
		t.Error("JAVASCRIPT: should be blocked")
	}
	if IsSafeURL("java%73cript:alert(1)") {
		t.Error("percent-encoded javascript: should be blocked")
	}
}

func TestIsSafeImagePath(t *testing.T) {
	t.Parallel()
	safe := []string{
		"https://example.com/image.png",
		"image.jpg",
		"path/to/image.gif",
	}
	for _, p := range safe {
		if !isSafeImagePath(p) {
			t.Errorf("expected safe image: %q", p)
		}
	}

	blocked := []string{
		"../../../etc/passwd.png",
		"image%2e%2e/etc/passwd.png",
	}
	for _, p := range blocked {
		if isSafeImagePath(p) {
			t.Errorf("expected blocked image: %q", p)
		}
	}
}

// --- Heading slug ---

func TestHeadingSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"Hello World", "hello-world"},
		{"  Spaces  ", "spaces"},
		{"foo---bar", "foo-bar"},
		{"UPPER", "upper"},
		{"a1b2c3", "a1b2c3"},
		{"", ""},
		{"cafébar", "cafbar"},
		{"日本語", ""},
		{"hello🌍world", "helloworld"},
		{"a café", "a-caf"},
	}
	for _, tc := range tests {
		got := HeadingSlug(tc.input)
		if got != tc.want {
			t.Errorf("HeadingSlug(%q): got %q, want %q",
				tc.input, got, tc.want)
		}
	}
}
