package markdown

import "testing"

func FuzzMarkdownParse(f *testing.F) {
	f.Add("# Heading\n\nParagraph with **bold** and *italic*.", false)
	f.Add("```go\nfunc main() {}\n```", false)
	f.Add("| A | B |\n|---|---|\n| 1 | 2 |", true)
	f.Add("*[HTTP]: HyperText Transfer Protocol\n\nHTTP is used.", false)
	f.Add("[^1]: Footnote.\n\nRef[^1] here.", false)
	f.Add("$x^2$", false)
	f.Add("==highlight==", false)
	f.Add("~subscript~", false)
	f.Add("^superscript^", false)
	f.Add("++underline++", false)
	f.Add("- item 1\n  - nested\n    - deep", false)
	f.Add("", false)
	f.Fuzz(func(t *testing.T, source string, hardBreaks bool) {
		blocks := Parse(source, hardBreaks)
		for _, blk := range blocks {
			if blk.ListIndent < 0 {
				t.Errorf("negative ListIndent: %d", blk.ListIndent)
			}
		}
	})
}

func FuzzIsSafeURL(f *testing.F) {
	f.Add("https://example.com")
	f.Add("javascript:alert(1)")
	f.Add("data:text/html,<h1>hi</h1>")
	f.Add("")
	f.Add("#anchor")
	f.Add("relative/path")
	f.Add("%6a%61%76%61%73%63%72%69%70%74:alert(1)")
	f.Fuzz(func(_ *testing.T, url string) {
		_ = IsSafeURL(url)
	})
}

func FuzzHeadingSlug(f *testing.F) {
	f.Add("Hello World")
	f.Add("")
	f.Add("   ---   ")
	f.Add("123 Test Heading!")
	f.Add("\t\n spaces  ")
	f.Fuzz(func(t *testing.T, text string) {
		slug := headingSlug(text)
		for _, r := range slug {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				t.Errorf("invalid char %q in slug %q", string(r), slug)
			}
		}
		if len(slug) > 0 && slug[len(slug)-1] == '-' {
			t.Errorf("trailing dash in slug %q", slug)
		}
	})
}
