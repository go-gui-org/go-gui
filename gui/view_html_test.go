package gui

import (
	"slices"
	"testing"
)

// TestHTMLRendersSubset checks that Window.HTML draws the HTML subset
// through the Markdown renderer: text reaches the layout with the tags
// gone, and Markdown syntax in the text stays literal (#1158).
func TestHTMLRendersSubset(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.HTML(MarkdownCfg{
		ID: "promo",
		Source: `<h2>Sale</h2><p>Save *50%* on <b>shoes</b></p>` +
			`<ul><li>one</li></ul><script>alert(1)</script>`,
		Style: DefaultMarkdownStyle(),
	}), w)
	got := mdCollectText(layout)
	for _, want := range []string{"Sale", "Save *50%* on shoes", "one"} {
		if !slices.Contains(got, want) {
			t.Errorf("text %q not rendered; got %q", want, got)
		}
	}
	for _, s := range got {
		if s == "alert(1)" || s == "Save 50% on shoes" {
			t.Errorf("unexpected text %q", s)
		}
	}
}

// TestHTMLRemoteImageBlockedByDefault: the #1105 image policy applies
// to HTML too, since it runs after the parse.
func TestHTMLRemoteImageBlockedByDefault(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.HTML(MarkdownCfg{
		Source: `<img src="https://example.test/` + t.Name() + `.png" alt="banner">`,
		Style:  DefaultMarkdownStyle(),
	}), w)
	if mdLayoutHasImage(layout) {
		t.Error("blocked remote image produced an image shape")
	}
	if !slices.Contains(mdCollectText(layout), "banner") {
		t.Errorf("alt text not shown; got %q", mdCollectText(layout))
	}
}

// TestHTMLAndMarkdownDoNotShareCache is the regression for the cache
// key: one source string read as Markdown and then as HTML must parse
// twice. Before the key carried the format, the second view served
// the first view's blocks.
func TestHTMLAndMarkdownDoNotShareCache(t *testing.T) {
	w := &Window{}
	src := "<b>x</b> **y**"
	md := mdCollectText(generateViewLayout(w.Markdown(MarkdownCfg{
		ID: "md", Source: src, Style: DefaultMarkdownStyle(),
	}), w))
	html := mdCollectText(generateViewLayout(w.HTML(MarkdownCfg{
		ID: "html", Source: src, Style: DefaultMarkdownStyle(),
	}), w))
	// Markdown drops raw HTML and bolds **y**; HTML bolds x and keeps
	// the asterisks.
	if !slices.Contains(html, "x **y**") {
		t.Errorf("HTML view text = %q, want %q", html, "x **y**")
	}
	if slices.Equal(md, html) {
		t.Errorf("Markdown and HTML views rendered the same text %q", md)
	}
}

func TestHTMLInvisible(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.HTML(MarkdownCfg{
		Source: "<p>x</p>", Invisible: true,
	}), w)
	if len(mdCollectText(layout)) != 0 {
		t.Error("invisible HTML view rendered text")
	}
}
