package markdown

import "testing"

func FuzzParseHTML(f *testing.F) {
	f.Add("<h1>Title</h1><p>Text with <b>bold</b> and <i>italic</i>.</p>")
	f.Add("<ul><li>a<ol><li>b</li></ol></li></ul>")
	f.Add("<blockquote><p>q</p><blockquote>qq</blockquote></blockquote>")
	f.Add(`<a href="javascript:alert(1)">x</a><img src="a.png" width="9">`)
	f.Add("<pre>\n  code &amp; more</pre><hr><br>")
	f.Add("<script>alert(1)</script><style>x</style><svg></svg>")
	f.Add("<b><i>misnested</b></i></p></ul>")
	f.Add("<")
	f.Add("")
	f.Fuzz(func(t *testing.T, source string) {
		blocks := ParseHTML(source)
		for _, blk := range blocks {
			if blk.ListIndent < 0 || blk.ListIndent >= maxHTMLNestDepth {
				t.Errorf("ListIndent out of range: %d", blk.ListIndent)
			}
			if blk.IsBlockquote != (blk.BlockquoteDepth > 0) {
				t.Errorf("IsBlockquote=%v disagrees with depth %d",
					blk.IsBlockquote, blk.BlockquoteDepth)
			}
			if blk.HeaderLevel < 0 || blk.HeaderLevel > 6 {
				t.Errorf("HeaderLevel out of range: %d",
					blk.HeaderLevel)
			}
			if !blk.IsHR && !blk.IsImage && len(blk.Runs) == 0 {
				t.Errorf("empty text block: %+v", blk)
			}
			for _, r := range blk.Runs {
				if r.Link != "" && !IsSafeURL(r.Link) {
					t.Errorf("unsafe link kept: %q", r.Link)
				}
			}
			if blk.ImageSrc != "" && !isSafeImagePath(blk.ImageSrc) {
				t.Errorf("unsafe image src kept: %q", blk.ImageSrc)
			}
		}
	})
}
