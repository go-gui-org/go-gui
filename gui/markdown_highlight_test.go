package gui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/highlight"
)

func TestMarkdownChromaHighlighter(t *testing.T) {
	style := DefaultMarkdownStyle()
	style.CodeHighlighter = highlight.Default()

	src := "```go\npackage main\n\nfunc main() {}\n```\n"
	blocks := markdownToBlocks(src, style)

	var code *MarkdownBlock
	for i := range blocks {
		if blocks[i].IsCode {
			code = &blocks[i]
			break
		}
	}
	if code == nil {
		t.Fatal("no code block parsed")
	}
	if code.CodeLanguage != "go" {
		t.Fatalf("lang = %q, want go", code.CodeLanguage)
	}

	var keywordColored, funcColored bool
	for _, r := range code.Content.Runs {
		if r.Text == "package" || r.Text == "func" {
			if r.Style.Color == style.CodeKeywordColor {
				keywordColored = true
			}
		}
		if r.Text == "main" &&
			r.Style.Color == style.CodeFunctionColor {
			funcColored = true
		}
	}
	if !keywordColored {
		t.Error("expected keyword run colored with CodeKeywordColor")
	}
	if !funcColored {
		t.Error("expected func name colored with CodeFunctionColor")
	}
}

type stubHighlighter struct {
	toks []highlight.Token
}

func (s stubHighlighter) Tokenize(_, _ string) []highlight.Token {
	return s.toks
}

func TestMarkdownHighlighter_EmptyResultKeepsParserRuns(t *testing.T) {
	style := DefaultMarkdownStyle()
	style.CodeHighlighter = stubHighlighter{toks: nil}
	src := "```go\npackage main\n```\n"
	blocks := markdownToBlocks(src, style)
	var code *MarkdownBlock
	for i := range blocks {
		if blocks[i].IsCode {
			code = &blocks[i]
			break
		}
	}
	if code == nil {
		t.Fatal("no code block")
	}
	if len(code.Content.Runs) == 0 {
		t.Error("empty highlighter result should fall back to parser runs")
	}
}

func TestMarkdownHighlighter_UnknownLang(t *testing.T) {
	style := DefaultMarkdownStyle()
	style.CodeHighlighter = highlight.Default()
	src := "```fortran\nprogram hello\nend\n```\n"
	blocks := markdownToBlocks(src, style)
	var code *MarkdownBlock
	for i := range blocks {
		if blocks[i].IsCode {
			code = &blocks[i]
			break
		}
	}
	if code == nil {
		t.Fatal("no code block")
	}
	// Uncurated lang: chroma returns a single plain token; the full
	// source text must be preserved in the rendered runs.
	var b strings.Builder
	for _, r := range code.Content.Runs {
		b.WriteString(r.Text)
	}
	if !strings.Contains(b.String(), "program hello") {
		t.Errorf("source not preserved: %q", b.String())
	}
}

func TestMarkdownHighlighterNilUnchanged(t *testing.T) {
	style := DefaultMarkdownStyle()
	src := "```go\npackage main\n```\n"
	blocks := markdownToBlocks(src, style)
	var code *MarkdownBlock
	for i := range blocks {
		if blocks[i].IsCode {
			code = &blocks[i]
			break
		}
	}
	if code == nil {
		t.Fatal("no code block")
	}
	// With nil highlighter, runs come from the parser's primitive
	// tokenizer. Just assert non-empty.
	if len(code.Content.Runs) == 0 {
		t.Error("expected runs from parser fallback")
	}
}
