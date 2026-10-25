package gui

import "github.com/go-gui-org/go-gui/gui/markdown"

// view_markdown_factory.go holds the two factories that build a
// markdownView: Window.Markdown and Window.HTML (#1158). The views
// differ only in the parse step and the cache key, both below.

// Markdown creates a markdown view. Method on *Window to
// access viewState for caching.
func (w *Window) Markdown(cfg MarkdownCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	return &markdownView{cfg: cfg}
}

// HTML creates a view that renders a small, safe subset of HTML: the
// tags listed on markdown.ParseHTML. It is not a browser. There is no
// CSS, no script and no layout engine; each tag maps to a block or a
// text style that Markdown already draws, so cfg takes the same
// MarkdownCfg (Style, image policy, selection, link handling).
// Unknown tags are dropped and their text kept; script and style are
// dropped with their content. Use it to preview HTML that something
// else renders, such as a snippet an admin writes for a mobile app
// (#1158). Source is HTML, not Markdown: "*" and "#" stay literal.
func (w *Window) HTML(cfg MarkdownCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	return &markdownView{cfg: cfg, html: true}
}

// cacheKey names the styled-block cache entry for cfg. The html flag
// is part of the key: one string read as HTML and as Markdown gives
// different blocks, so the two must not share an entry.
func (mv *markdownView) cacheKey(cfg *MarkdownCfg) markdownCacheKey {
	key := newMarkdownCacheKey(cfg.Source, &cfg.Style)
	key.html = mv.html
	return key
}

// parse turns cfg.Source into styled blocks with the parser that
// matches the view: markdown.ParseHTML for Window.HTML, the Markdown
// parser otherwise. Styling is the same for both.
func (mv *markdownView) parse(cfg *MarkdownCfg) []markdownBlock {
	if mv.html {
		return styleMdBlocks(markdown.ParseHTML(cfg.Source), cfg.Style)
	}
	return markdownToBlocks(cfg.Source, cfg.Style)
}
