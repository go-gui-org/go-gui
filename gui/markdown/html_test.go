package markdown

import (
	"strings"
	"testing"
)

// htmlText joins the text of every run in a block, for assertions
// that care about what the reader sees, not how runs are split.
func htmlText(b Block) string { return runsToText(b.Runs) }

func TestParseHTMLParagraphs(t *testing.T) {
	blocks := ParseHTML("<p>one</p><p>two</p>bare text<div>three</div>")
	want := []string{"one", "two", "bare text", "three"}
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(want), blocks)
	}
	for i, w := range want {
		if got := htmlText(blocks[i]); got != w {
			t.Errorf("block %d = %q, want %q", i, got, w)
		}
		if blocks[i].HeaderLevel != 0 || blocks[i].IsList {
			t.Errorf("block %d is not a plain paragraph: %+v", i, blocks[i])
		}
	}
}

func TestParseHTMLWhitespaceCollapses(t *testing.T) {
	blocks := ParseHTML("<p>\n   hello \t\n  world  </p>")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if got := htmlText(blocks[0]); got != "hello world" {
		t.Errorf("text = %q, want %q", got, "hello world")
	}
}

func TestParseHTMLWhitespaceBetweenInlineTags(t *testing.T) {
	// The space between two inline elements is real text and must
	// survive, but only once.
	blocks := ParseHTML("<p><b>bold</b> <i>italic</i></p>")
	if got := htmlText(blocks[0]); got != "bold italic" {
		t.Errorf("text = %q, want %q", got, "bold italic")
	}
}

func TestParseHTMLLineBreak(t *testing.T) {
	blocks := ParseHTML("<p>line one<br>  line two<br/></p>")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if got := htmlText(blocks[0]); got != "line one\nline two" {
		t.Errorf("text = %q, want %q", got, "line one\nline two")
	}
}

func TestParseHTMLRule(t *testing.T) {
	blocks := ParseHTML("<p>a</p><hr><p>b</p>")
	if len(blocks) != 3 || !blocks[1].IsHR {
		t.Fatalf("want a, hr, b; got %+v", blocks)
	}
}

func TestParseHTMLHeadings(t *testing.T) {
	blocks := ParseHTML("<h1>One</h1><h3>Three Words</h3><h6>Six</h6>")
	levels := []int{1, 3, 6}
	if len(blocks) != len(levels) {
		t.Fatalf("got %d blocks, want %d", len(blocks), len(levels))
	}
	for i, lv := range levels {
		if blocks[i].HeaderLevel != lv {
			t.Errorf("block %d level = %d, want %d", i, blocks[i].HeaderLevel, lv)
		}
	}
	// Headings get the same anchor slugs as Markdown headings, so
	// "#three-words" links work in both formats.
	if blocks[1].AnchorSlug != "three-words" {
		t.Errorf("slug = %q, want %q", blocks[1].AnchorSlug, "three-words")
	}
}

func TestParseHTMLInlineFormats(t *testing.T) {
	tests := []struct {
		src   string
		check func(Run) bool
	}{
		{"<b>x</b>", func(r Run) bool { return r.Format == FormatBold }},
		{"<strong>x</strong>", func(r Run) bool { return r.Format == FormatBold }},
		{"<i>x</i>", func(r Run) bool { return r.Format == FormatItalic }},
		{"<em>x</em>", func(r Run) bool { return r.Format == FormatItalic }},
		{"<b><i>x</i></b>", func(r Run) bool { return r.Format == FormatBoldItalic }},
		{"<i><b>x</b></i>", func(r Run) bool { return r.Format == FormatBoldItalic }},
		{"<code>x</code>", func(r Run) bool { return r.Format == FormatCode }},
		{"<u>x</u>", func(r Run) bool { return r.Underline }},
		{"<s>x</s>", func(r Run) bool { return r.Strikethrough }},
		{"<del>x</del>", func(r Run) bool { return r.Strikethrough }},
		{"<strike>x</strike>", func(r Run) bool { return r.Strikethrough }},
		{"<mark>x</mark>", func(r Run) bool { return r.Highlight }},
		{"<sup>x</sup>", func(r Run) bool { return r.Superscript }},
		{"<sub>x</sub>", func(r Run) bool { return r.Subscript }},
	}
	for _, tt := range tests {
		blocks := ParseHTML("<p>a " + tt.src + " b</p>")
		if len(blocks) != 1 {
			t.Fatalf("%s: got %d blocks", tt.src, len(blocks))
		}
		var found bool
		for _, r := range blocks[0].Runs {
			if r.Text == "x" {
				found = true
				if !tt.check(r) {
					t.Errorf("%s: run %+v lacks the format", tt.src, r)
				}
			} else if r.Format != formatPlain || r.Underline ||
				r.Strikethrough || r.Highlight || r.Superscript || r.Subscript {
				t.Errorf("%s: format leaked onto %q: %+v", tt.src, r.Text, r)
			}
		}
		if !found {
			t.Errorf("%s: no run with text x in %+v", tt.src, blocks[0].Runs)
		}
	}
}

func TestParseHTMLMarkdownCharsStayLiteral(t *testing.T) {
	// The bug the issue reports for HTML → Markdown text: these
	// characters must not change meaning.
	src := "<p>*not bold* _not italic_ # not heading | not table ==x==</p>"
	blocks := ParseHTML(src)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks", len(blocks))
	}
	want := "*not bold* _not italic_ # not heading | not table ==x=="
	if got := htmlText(blocks[0]); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
	for _, r := range blocks[0].Runs {
		if r.Format != formatPlain || r.Highlight {
			t.Errorf("markdown syntax formatted a run: %+v", r)
		}
	}
	// No typography pass either: "--" and "..." are literal in HTML.
	if got := htmlText(ParseHTML("a -- b ...")[0]); got != "a -- b ..." {
		t.Errorf("typography applied: %q", got)
	}
}

func TestParseHTMLEntities(t *testing.T) {
	blocks := ParseHTML("<p>a &amp; b &lt;tag&gt; &quot;q&quot; &#169; &copy;</p>")
	want := `a & b <tag> "q" © ©`
	if got := htmlText(blocks[0]); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestParseHTMLNonBreakingSpaceKept(t *testing.T) {
	// &nbsp; is not collapsible whitespace.
	blocks := ParseHTML("<p>a&nbsp;&nbsp;b</p>")
	if got := htmlText(blocks[0]); got != "a  b" {
		t.Errorf("text = %q", got)
	}
}

func TestParseHTMLPreKeepsWhitespace(t *testing.T) {
	blocks := ParseHTML("<pre>\n  x := 1\n\ty  &lt;- 2</pre>")
	if len(blocks) != 1 || !blocks[0].IsCode {
		t.Fatalf("want one code block, got %+v", blocks)
	}
	// The newline right after <pre> is dropped, as browsers do.
	if got := htmlText(blocks[0]); got != "  x := 1\n\ty  <- 2" {
		t.Errorf("text = %q", got)
	}
	for _, r := range blocks[0].Runs {
		if r.Format != FormatCode {
			t.Errorf("code run not FormatCode: %+v", r)
		}
	}
}

func TestParseHTMLBlockquote(t *testing.T) {
	blocks := ParseHTML("<blockquote>outer<blockquote>inner</blockquote></blockquote>after")
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	if !blocks[0].IsBlockquote || blocks[0].BlockquoteDepth != 1 {
		t.Errorf("outer = %+v", blocks[0])
	}
	if !blocks[1].IsBlockquote || blocks[1].BlockquoteDepth != 2 {
		t.Errorf("inner = %+v", blocks[1])
	}
	if blocks[2].IsBlockquote || blocks[2].BlockquoteDepth != 0 {
		t.Errorf("after = %+v", blocks[2])
	}
}

func TestParseHTMLLists(t *testing.T) {
	src := `<ul>
	  <li>one</li>
	  <li>two
	    <ol start="3"><li>three</li><li>four</li></ol>
	  </li>
	</ul>`
	blocks := ParseHTML(src)
	type want struct {
		text   string
		prefix string
		indent int
	}
	wants := []want{
		{"one", "• ", 0},
		{"two", "• ", 0},
		{"three", "3. ", 1},
		{"four", "4. ", 1},
	}
	if len(blocks) != len(wants) {
		t.Fatalf("got %d blocks, want %d: %+v", len(blocks), len(wants), blocks)
	}
	for i, w := range wants {
		b := blocks[i]
		if !b.IsList || htmlText(b) != w.text ||
			b.ListPrefix != w.prefix || b.ListIndent != w.indent {
			t.Errorf("block %d = {%q %q %d list=%v}, want %+v",
				i, htmlText(b), b.ListPrefix, b.ListIndent, b.IsList, w)
		}
	}
}

func TestParseHTMLOrderedListDefaultsToOne(t *testing.T) {
	blocks := ParseHTML("<ol><li>a<li>b</ol>")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks", len(blocks))
	}
	if blocks[0].ListPrefix != "1. " || blocks[1].ListPrefix != "2. " {
		t.Errorf("prefixes = %q, %q", blocks[0].ListPrefix, blocks[1].ListPrefix)
	}
}

func TestParseHTMLLinks(t *testing.T) {
	blocks := ParseHTML(`<p>see <a href="https://example.com/?a=1&amp;b=2">the <b>site</b></a> now</p>`)
	var linked []Run
	for _, r := range blocks[0].Runs {
		if r.Link != "" {
			linked = append(linked, r)
		}
	}
	if len(linked) != 2 {
		t.Fatalf("linked runs = %+v", linked)
	}
	for _, r := range linked {
		if r.Link != "https://example.com/?a=1&b=2" {
			t.Errorf("link = %q (entities in attributes must decode)", r.Link)
		}
	}
	if linked[1].Format != FormatBold {
		t.Errorf("bold inside link lost: %+v", linked[1])
	}
	if got := htmlText(blocks[0]); got != "see the site now" {
		t.Errorf("text = %q", got)
	}
}

func TestParseHTMLUnsafeLinkDropped(t *testing.T) {
	for _, href := range []string{
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		"java&#x09;script:alert(1)",
		"data:text/html,<b>x</b>",
		"vbscript:x",
	} {
		blocks := ParseHTML(`<a href="` + href + `">click</a>`)
		if len(blocks) != 1 {
			t.Fatalf("%s: got %d blocks", href, len(blocks))
		}
		for _, r := range blocks[0].Runs {
			if r.Link != "" {
				t.Errorf("%s: unsafe link kept: %q", href, r.Link)
			}
		}
		if htmlText(blocks[0]) != "click" {
			t.Errorf("%s: link text lost", href)
		}
	}
}

func TestParseHTMLImage(t *testing.T) {
	blocks := ParseHTML(`<p>before<img src="https://x.test/a.png" alt="An &quot;A&quot;" width="120" height="40">after</p>`)
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	img := blocks[1]
	if !img.IsImage || img.ImageSrc != "https://x.test/a.png" ||
		img.ImageAlt != `An "A"` || img.ImageWidth != 120 || img.ImageHeight != 40 {
		t.Errorf("image = %+v", img)
	}
	if htmlText(blocks[0]) != "before" || htmlText(blocks[2]) != "after" {
		t.Errorf("split text = %q / %q", htmlText(blocks[0]), htmlText(blocks[2]))
	}
}

func TestParseHTMLImageUnsafeSrcDropped(t *testing.T) {
	for _, src := range []string{
		"javascript:alert(1)",
		"../../etc/passwd.png",
		"/abs/path.png",
		"file.exe",
	} {
		blocks := ParseHTML(`<img src="` + src + `" alt="alt">`)
		if len(blocks) != 1 || !blocks[0].IsImage {
			t.Fatalf("%s: got %+v", src, blocks)
		}
		if blocks[0].ImageSrc != "" {
			t.Errorf("%s: unsafe src kept", src)
		}
		if blocks[0].ImageAlt != "alt" {
			t.Errorf("%s: alt lost", src)
		}
	}
}

func TestParseHTMLImageBadDimensionsIgnored(t *testing.T) {
	blocks := ParseHTML(`<img src="a.png" width="-5" height="NaN">`)
	if blocks[0].ImageWidth != 0 || blocks[0].ImageHeight != 0 {
		t.Errorf("bad dimensions kept: %+v", blocks[0])
	}
	blocks = ParseHTML(`<img src="a.png" width="1e9">`)
	if blocks[0].ImageWidth != 0 {
		t.Errorf("huge width kept: %v", blocks[0].ImageWidth)
	}
}

func TestParseHTMLDroppedContent(t *testing.T) {
	src := `<head><title>T</title></head>
	<style>p { color: red }</style>
	<script>alert("x")</script>
	<template><p>tpl</p></template>
	<noscript>ns</noscript>
	<textarea>ta</textarea>
	<iframe src="x">fr</iframe>
	<svg><text>svg</text></svg>
	<!-- comment -->
	<p>kept</p>`
	blocks := ParseHTML(src)
	if len(blocks) != 1 || htmlText(blocks[0]) != "kept" {
		t.Errorf("got %+v", blocks)
	}
}

func TestParseHTMLUnknownTagsKeepText(t *testing.T) {
	blocks := ParseHTML(`<p><span class="x" style="color:red">hi</span> <font>there</font></p>`)
	if len(blocks) != 1 || htmlText(blocks[0]) != "hi there" {
		t.Errorf("got %+v", blocks)
	}
}

func TestParseHTMLUnclosedAndStrayTags(t *testing.T) {
	blocks := ParseHTML("</b></ul></blockquote></h2><p>one<p>two <b>bold")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	if htmlText(blocks[0]) != "one" || htmlText(blocks[1]) != "two bold" {
		t.Errorf("text = %q, %q", htmlText(blocks[0]), htmlText(blocks[1]))
	}
	last := blocks[1].Runs[len(blocks[1].Runs)-1]
	if last.Format != FormatBold {
		t.Errorf("unclosed <b> lost: %+v", last)
	}
}

func TestParseHTMLMisnestedInlineTags(t *testing.T) {
	// <b><i>x</b>y</i>: closing <b> restores the state before it,
	// so y is plain, and the late </i> is a no-op.
	blocks := ParseHTML("<b><i>x</b>y</i>z")
	for _, r := range blocks[0].Runs {
		switch r.Text {
		case "x":
			if r.Format != FormatBoldItalic {
				t.Errorf("x = %+v", r)
			}
		case "yz":
			if r.Format != formatPlain {
				t.Errorf("yz = %+v", r)
			}
		}
	}
	if htmlText(blocks[0]) != "xyz" {
		t.Errorf("text = %q", htmlText(blocks[0]))
	}
}

func TestParseHTMLDepthCapped(t *testing.T) {
	// Deep nesting must not grow ListIndent or BlockquoteDepth past
	// the cap, and must not crash.
	src := strings.Repeat("<ul><li>x", 500) + strings.Repeat("<blockquote>", 500) + "q"
	blocks := ParseHTML(src)
	for _, b := range blocks {
		if b.ListIndent >= maxHTMLNestDepth {
			t.Fatalf("ListIndent %d past cap", b.ListIndent)
		}
		if b.BlockquoteDepth > maxHTMLNestDepth {
			t.Fatalf("BlockquoteDepth %d past cap", b.BlockquoteDepth)
		}
	}
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
}

func TestParseHTMLInlineDepthCapped(t *testing.T) {
	// Opens past maxHTMLInlineDepth are ignored. Their closes find no
	// frame, so the text after the last close is plain again.
	src := strings.Repeat("<b>", 500) + "x" + strings.Repeat("</b>", 500) + "y"
	blocks := ParseHTML(src)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1: %+v", len(blocks), blocks)
	}
	runs := blocks[0].Runs
	if len(runs) != 2 || runs[0].Text != "x" || runs[1].Text != "y" {
		t.Fatalf("runs = %+v, want x then y", runs)
	}
	if runs[0].Format != FormatBold || runs[1].Format != formatPlain {
		t.Errorf("formats = %v, %v; want bold, plain", runs[0].Format, runs[1].Format)
	}
}

func TestParseHTMLCarriageReturnsNormalize(t *testing.T) {
	// CRLF and a bare CR both become one newline, as in Parse.
	blocks := ParseHTML("<pre>a\r\nb\rc</pre>")
	if len(blocks) != 1 || !blocks[0].IsCode {
		t.Fatalf("want one code block, got %+v", blocks)
	}
	if got := htmlText(blocks[0]); got != "a\nb\nc" {
		t.Errorf("code = %q, want %q", got, "a\nb\nc")
	}
}

func TestParseHTMLSourceCapped(t *testing.T) {
	src := "<p>" + strings.Repeat("é", maxSourceBytes) + "</p>"
	blocks := ParseHTML(src)
	total := 0
	for _, b := range blocks {
		total += len(htmlText(b))
	}
	if total > maxSourceBytes {
		t.Errorf("parsed %d bytes, cap %d", total, maxSourceBytes)
	}
}

func TestParseHTMLEmpty(t *testing.T) {
	for _, src := range []string{"", "   \n ", "<p></p>", "<p> </p><div></div>"} {
		if blocks := ParseHTML(src); len(blocks) != 0 {
			t.Errorf("%q: got %+v", src, blocks)
		}
	}
}

func TestParseHTMLHeadingInListItemKeepsBullet(t *testing.T) {
	blocks := ParseHTML("<ul><li><h3>Title</h3></li></ul>")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	b := blocks[0]
	if !b.IsList || b.ListPrefix != "• " || b.HeaderLevel != 0 {
		t.Errorf("block = %+v, want a bullet item", b)
	}
	if b.Runs[0].Format != FormatBold {
		t.Errorf("heading text in item not bold: %+v", b.Runs[0])
	}
}

func TestParseHTMLPreNewlineAfterTagKept(t *testing.T) {
	// Only the newline right after <pre> is dropped. One after a tag
	// inside the pre is content.
	blocks := ParseHTML("<pre><b>x</b>\ny</pre>")
	if len(blocks) != 1 || htmlText(blocks[0]) != "x\ny" {
		t.Errorf("got %+v", blocks)
	}
}

func TestParseHTMLUpperCaseMarkup(t *testing.T) {
	blocks := ParseHTML(`<SCRIPT>x</SCRIPT><B>y</B><A HREF="https://e.test">z</A>`)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	if htmlText(blocks[0]) != "yz" {
		t.Errorf("text = %q, want %q", htmlText(blocks[0]), "yz")
	}
	for _, r := range blocks[0].Runs {
		switch r.Text {
		case "y":
			if r.Format != FormatBold {
				t.Errorf("y not bold: %+v", r)
			}
		case "z":
			if r.Link != "https://e.test" {
				t.Errorf("z link = %q", r.Link)
			}
		}
	}
}

func TestParseHTMLHeadWithoutClose(t *testing.T) {
	// HTML lets </head> be left out; the body must still render.
	blocks := ParseHTML("<html><head><meta charset=utf-8><title>T</title><body><p>x</p>")
	if len(blocks) != 1 || htmlText(blocks[0]) != "x" {
		t.Errorf("got %+v", blocks)
	}
}

func TestParseHTMLHeadEndsBeforeBodyAttrs(t *testing.T) {
	// The first body tag ends the head before its attributes are
	// read, so the link keeps its href.
	blocks := ParseHTML(`<head><title>T</title><a href="https://e.test">z</a>`)
	if len(blocks) != 1 || len(blocks[0].Runs) != 1 ||
		blocks[0].Runs[0].Link != "https://e.test" {
		t.Errorf("got %+v", blocks)
	}
}

func TestParseHTMLHeadEndsAtBodyText(t *testing.T) {
	// Body text ends an unclosed head; the title text does not.
	blocks := ParseHTML("<head><title>T</title> \n Hello <b>x</b>")
	if len(blocks) != 1 || htmlText(blocks[0]) != "Hello x" {
		t.Errorf("got %+v", blocks)
	}
}

func TestParseHTMLStrayItemListCloses(t *testing.T) {
	// The list a stray <li> implies ends with the item, so the next
	// list starts at indent 0.
	blocks := ParseHTML("<li>a</li><ul><li>b</li></ul>")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	for _, b := range blocks {
		if !b.IsList || b.ListIndent != 0 {
			t.Errorf("block %q: list=%v indent=%d, want indent 0",
				htmlText(b), b.IsList, b.ListIndent)
		}
	}
	// A stray </ul> after it must not pop a real list.
	blocks = ParseHTML("<ul><li>x<li>stray</li></ul></ul><p>after</p>")
	if n := len(blocks); n != 3 || blocks[2].IsList {
		t.Errorf("got %+v", blocks)
	}
	// Two stray items in a row share one implied list.
	blocks = ParseHTML("<li>a<li>b")
	if len(blocks) != 2 || blocks[1].ListPrefix != "• " || blocks[1].ListIndent != 0 {
		t.Errorf("got %+v", blocks)
	}
}
