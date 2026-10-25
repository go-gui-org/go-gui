package markdown

import (
	stdhtml "html"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	tdparse "github.com/tdewolff/parse/v2"
	htmllex "github.com/tdewolff/parse/v2/html"
)

// html.go turns a small, safe subset of HTML into the same []Block
// that Parse makes from Markdown (issue #1158). It is not a browser:
// there is no CSS, no script and no layout engine. Each tag maps to a
// block kind or a run flag that the Markdown renderer already draws.
// The text goes straight into runs, so characters that are syntax in
// Markdown (*, _, #, |) stay literal.

// Nesting caps. A hostile document can open thousands of lists,
// quotes or inline tags. Past the cap, an open tag is ignored, so
// the indent, the quote depth and the inline stack stay bounded.
const (
	maxHTMLNestDepth   = 16
	maxHTMLInlineDepth = 64
	// maxHTMLImageDim is the largest width or height attribute kept.
	// A larger value is ignored and the image takes its own size.
	maxHTMLImageDim = 8192
	// maxHTMLListStart bounds <ol start>, so the marker stays short.
	maxHTMLListStart = 1_000_000
)

// htmlSkipTags are elements whose content is dropped with the tag.
// Their text is code, metadata or a form value, never prose to show.
var htmlSkipTags = map[string]bool{
	"script": true, "style": true, "template": true, "head": true,
	"title": true, "textarea": true, "iframe": true, "noscript": true,
	"object": true, "select": true, "xmp": true, "plaintext": true,
}

// htmlHeadTags may appear inside <head>. HTML lets a document leave
// out </head>; any other start tag (body, p, h1, ...) ends the head,
// so a missing close does not drop the whole document.
var htmlHeadTags = map[string]bool{
	"head": true, "meta": true, "link": true, "base": true,
	"title": true, "style": true, "script": true, "noscript": true,
	"template": true,
}

// htmlBreakTags end the current paragraph and start a new one. They
// have no style of their own. The list includes the structure tags
// (sections, tables, definition lists), so their text keeps its
// paragraph breaks instead of running together.
var htmlBreakTags = map[string]bool{
	"p": true, "div": true, "section": true, "article": true,
	"header": true, "footer": true, "main": true, "nav": true,
	"aside": true, "figure": true, "figcaption": true, "address": true,
	"center": true, "table": true, "caption": true, "tr": true,
	"td": true, "th": true, "dl": true, "dt": true, "dd": true,
	"details": true, "summary": true, "fieldset": true, "legend": true,
	"form": true, "body": true, "html": true,
}

// htmlInlineFrame is one open inline tag and the state to restore
// when it closes.
type htmlInlineFrame struct {
	tag  string
	prev inlineState
}

// htmlList is one open <ul> or <ol>. The item state lives here, not
// on the parser, so text after a nested list returns to the parent
// item.
type htmlList struct {
	// prefix is the marker the current item has not yet used. After
	// the first block of an item it becomes spaces of the same width,
	// so a later block of the same item lines up under its text.
	prefix  string
	next    int
	indent  int
	ordered bool
	inItem  bool
	// implied marks the list a stray <li> opens. It closes with that
	// item, so it does not indent a later list or absorb a stray </ul>.
	implied bool
}

// htmlAttrs holds the attributes the subset reads. Every other
// attribute (style, class, on*) is ignored.
type htmlAttrs struct {
	href, src, alt, width, height, start string
}

type htmlParser struct {
	blocks  []Block
	runs    []Run
	inline  []htmlInlineFrame
	lists   []htmlList
	skipTag string
	tag     string
	attrs   htmlAttrs
	state   inlineState
	// listOverflow and bqOverflow count opens past the nesting cap,
	// so their closes do not pop a level that is still open.
	listOverflow int
	bqOverflow   int
	bqDepth      int
	heading      int
	pre          int
	skipDepth    int
	// atSpace is true when the text so far ends in collapsed space or
	// is at the start of a block: the next leading space is dropped.
	atSpace bool
	// preLead drops the one newline right after <pre>, as browsers do.
	preLead bool
	// headChild is the open head child that holds content (title,
	// style, script, noscript, template) while the head is skipped.
	// Its text is head content and does not end the head.
	headChild string
}

// ParseHTML converts a small HTML subset to []Block. Supported tags:
// p, div, br, hr, h1-h6, b/strong, i/em, u/ins, s/del/strike, mark,
// sup, sub, code/kbd/samp/tt, pre, blockquote, ul, ol (start), li,
// a (href) and img (src, alt, width, height). Script, style and
// similar elements are dropped with their content. Any other tag is
// dropped and its text kept. Links and image sources pass the same
// safety checks as Markdown. Sources past maxSourceBytes are
// truncated, so a hostile document costs a bounded parse.
func ParseHTML(source string) []Block {
	if len(source) > maxSourceBytes {
		cut := maxSourceBytes
		for cut > 0 && !utf8.RuneStart(source[cut]) {
			cut--
		}
		source = source[:cut]
	}
	if strings.ContainsRune(source, '\r') {
		source = strings.ReplaceAll(source, "\r\n", "\n")
		source = strings.ReplaceAll(source, "\r", "\n")
	}
	p := &htmlParser{atSpace: true}
	lex := htmllex.NewLexer(tdparse.NewInputString(source))
	for {
		tt, _ := lex.Next()
		if tt == htmllex.ErrorToken {
			break
		}
		p.token(tt, lex)
	}
	p.flush()
	for i := range p.blocks {
		if !p.blocks[i].IsCode {
			p.blocks[i].Runs = mergeAdjacentRuns(p.blocks[i].Runs)
		}
	}
	return p.blocks
}

func (p *htmlParser) token(tt htmllex.TokenType, lex *htmllex.Lexer) {
	switch tt {
	case htmllex.StartTagToken:
		p.tag = string(lex.Text())
		p.attrs = htmlAttrs{}
		// A body tag ends an unclosed head here, before its
		// attributes arrive, so attr keeps them.
		if p.skipTag == "head" && p.headChild == "" && !htmlHeadTags[p.tag] {
			p.leaveHead()
		}
	case htmllex.AttributeToken:
		p.attr(string(lex.AttrKey()), lex.AttrVal())
	case htmllex.StartTagCloseToken, htmllex.StartTagVoidToken:
		// A self-closing slash is ignored, as browsers do: <b/> still
		// opens bold. Void tags (br, hr, img) need no close anyway.
		p.open(p.tag)
		p.tag = ""
	case htmllex.EndTagToken:
		name := strings.ToLower(string(lex.Text()))
		if i := strings.IndexAny(name, " \t\n\f/"); i >= 0 {
			name = name[:i]
		}
		p.close(name)
	case htmllex.TextToken:
		// Skipped content (a script body, a head child's text) is
		// dropped before the copy and unescape.
		openHead := p.skipTag == "head" && p.headChild == ""
		if p.skipTag != "" && !openHead {
			return
		}
		text := stdhtml.UnescapeString(string(lex.Text()))
		// As in browsers, text that is not all space ends an unclosed
		// head, unless it is the content of a head child like <title>.
		if openHead && strings.Trim(text, " \t\n\f\r") != "" {
			p.leaveHead()
		}
		if p.skipTag == "" {
			p.text(text)
		}
	default:
		// Comments, doctypes and inline SVG/MathML are dropped.
	}
}

func (p *htmlParser) attr(key string, raw []byte) {
	if p.skipTag != "" {
		return
	}
	var dst *string
	switch key {
	case "href":
		dst = &p.attrs.href
	case "src":
		dst = &p.attrs.src
	case "alt":
		dst = &p.attrs.alt
	case "width":
		dst = &p.attrs.width
	case "height":
		dst = &p.attrs.height
	case "start":
		dst = &p.attrs.start
	default:
		return
	}
	val := string(raw)
	if n := len(val); n >= 2 &&
		(val[0] == '"' || val[0] == '\'') && val[n-1] == val[0] {
		val = val[1 : n-1]
	} else if n >= 1 && (val[0] == '"' || val[0] == '\'') {
		// Unterminated quote at the end of input.
		val = val[1:]
	}
	*dst = stdhtml.UnescapeString(val)
}

//nolint:gocyclo // one flat switch over the supported tags
func (p *htmlParser) open(tag string) {
	// Only a newline right after <pre> is dropped; any tag in
	// between ends that window.
	p.preLead = false
	if p.skipTag == "head" && p.headChild == "" {
		switch tag {
		case "title", "style", "script", "noscript", "template":
			p.headChild = tag
		}
	}
	if p.skipTag != "" {
		if tag == p.skipTag {
			p.skipDepth++
		}
		return
	}
	if tag != "li" {
		p.closeImpliedList()
	}
	if htmlSkipTags[tag] {
		p.skipTag = tag
		p.skipDepth = 1
		return
	}
	switch tag {
	case "br":
		p.trimTrailingSpace()
		p.runs = append(p.runs, Run{Text: "\n"})
		p.atSpace = true
	case "hr":
		p.flush()
		p.emit(Block{IsHR: true})
	case "img":
		p.image()
	case "h1", "h2", "h3", "h4", "h5", "h6":
		p.flush()
		p.heading = int(tag[1] - '0')
	case "pre":
		p.flush()
		p.pre++
		// Set after the reset at the top of open.
		p.preLead = true
	case "blockquote":
		p.flush()
		if p.bqDepth >= maxHTMLNestDepth {
			p.bqOverflow++
			return
		}
		p.bqDepth++
	case "ul", "ol":
		p.flush()
		if len(p.lists) >= maxHTMLNestDepth {
			p.listOverflow++
			return
		}
		next := 1
		if tag == "ol" && p.attrs.start != "" {
			if n, err := strconv.Atoi(strings.TrimSpace(p.attrs.start)); err == nil &&
				n >= 0 && n <= maxHTMLListStart {
				next = n
			}
		}
		p.lists = append(p.lists, htmlList{
			ordered: tag == "ol",
			next:    next,
			indent:  len(p.lists),
		})
	case "li":
		p.flush()
		if len(p.lists) == 0 {
			// A stray <li> gets an implied bullet list.
			p.lists = append(p.lists, htmlList{next: 1, implied: true})
		}
		l := &p.lists[len(p.lists)-1]
		if l.ordered {
			l.prefix = strconv.Itoa(l.next) + ". "
			if l.next < maxHTMLListStart {
				l.next++
			}
		} else {
			l.prefix = "• "
		}
		l.inItem = true
	case "b", "strong":
		p.pushInline(tag, func(s *inlineState) {
			s.format = mergeFormat(s.format, FormatBold)
		})
	case "i", "em":
		p.pushInline(tag, func(s *inlineState) {
			s.format = mergeFormat(s.format, FormatItalic)
		})
	case "code", "kbd", "samp", "tt":
		p.pushInline(tag, func(s *inlineState) { s.format = FormatCode })
	case "u", "ins":
		p.pushInline(tag, func(s *inlineState) { s.underline = true })
	case "s", "del", "strike":
		p.pushInline(tag, func(s *inlineState) { s.strikethrough = true })
	case "mark":
		p.pushInline(tag, func(s *inlineState) { s.highlight = true })
	case "sup":
		p.pushInline(tag, func(s *inlineState) { s.superscript = true })
	case "sub":
		p.pushInline(tag, func(s *inlineState) { s.subscript = true })
	case "a":
		// An unsafe or missing href still opens the frame, so the
		// matching </a> restores the state before it.
		href := strings.TrimSpace(p.attrs.href)
		if !IsSafeURL(href) {
			href = ""
		}
		p.pushInline(tag, func(s *inlineState) { s.link = href })
	default:
		if htmlBreakTags[tag] {
			p.flush()
		}
		// Unknown tags are dropped; their text is kept.
	}
}

//nolint:gocyclo // mirrors the switch in open
func (p *htmlParser) close(tag string) {
	p.preLead = false
	if p.skipTag == "head" && tag == p.headChild {
		p.headChild = ""
	}
	if p.skipTag != "" {
		if tag == p.skipTag {
			p.skipDepth--
			if p.skipDepth == 0 {
				p.skipTag = ""
			}
		}
		return
	}
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		p.flush()
		p.heading = 0
	case "pre":
		if p.pre > 0 {
			p.flush()
			p.pre--
		}
	case "blockquote":
		p.flush()
		switch {
		case p.bqOverflow > 0:
			p.bqOverflow--
		case p.bqDepth > 0:
			p.bqDepth--
		}
	case "ul", "ol":
		p.flush()
		switch {
		case p.listOverflow > 0:
			p.listOverflow--
		case len(p.lists) > 0:
			p.lists = p.lists[:len(p.lists)-1]
		}
	case "li":
		p.flush()
		if n := len(p.lists); n > 0 {
			p.lists[n-1].inItem = false
		}
		p.closeImpliedList()
	default:
		if htmlBreakTags[tag] {
			p.flush()
			return
		}
		p.popInline(tag)
	}
}

// leaveHead ends the skip of an unclosed <head>. HTML lets </head> be
// left out: the first body tag or body text ends the head.
func (p *htmlParser) leaveHead() {
	p.skipTag = ""
	p.skipDepth = 0
	p.headChild = ""
}

// closeImpliedList pops the list a stray <li> opened, once its item
// has ended. Inside the item (inItem), the list stays: a nested
// <ul> there is a real child list.
func (p *htmlParser) closeImpliedList() {
	if n := len(p.lists); n > 0 && p.lists[n-1].implied &&
		!p.lists[n-1].inItem {
		p.flush()
		p.lists = p.lists[:n-1]
	}
}

// pushInline opens an inline frame. Past maxHTMLInlineDepth the tag
// is ignored: it changes nothing, and its close finds no frame.
func (p *htmlParser) pushInline(tag string, apply func(*inlineState)) {
	if len(p.inline) >= maxHTMLInlineDepth {
		return
	}
	p.inline = append(p.inline, htmlInlineFrame{tag: tag, prev: p.state})
	apply(&p.state)
}

// popInline closes the most recent frame for tag and every frame
// opened after it. Restoring that frame's saved state also ends the
// inner tags, which is how <b><i>x</b>y renders y plain. A close
// with no open frame is ignored.
func (p *htmlParser) popInline(tag string) {
	for i, f := range slices.Backward(p.inline) {
		if f.tag == tag {
			p.state = f.prev
			p.inline = p.inline[:i]
			return
		}
	}
}

// text appends character data. Outside <pre>, each run of ASCII
// whitespace becomes one space, and a space at the start of a block
// or after another space is dropped. A non-breaking space is not
// whitespace here, so &nbsp; keeps its width.
func (p *htmlParser) text(s string) {
	if p.pre > 0 {
		if p.preLead {
			s = strings.TrimPrefix(s, "\n")
			p.preLead = false
		}
		if s != "" {
			p.runs = append(p.runs, Run{Text: s, Format: FormatCode})
		}
		return
	}
	var sb strings.Builder
	sb.Grow(len(s))
	space := p.atSpace
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\f' {
			if !space {
				sb.WriteByte(' ')
				space = true
			}
			continue
		}
		sb.WriteByte(c)
		space = false
	}
	if sb.Len() == 0 {
		return
	}
	p.atSpace = space
	st := p.state
	p.runs = append(p.runs, Run{
		Text:          sb.String(),
		Link:          st.link,
		Format:        st.format,
		Strikethrough: st.strikethrough,
		Underline:     st.underline,
		Highlight:     st.highlight,
		Superscript:   st.superscript,
		Subscript:     st.subscript,
	})
}

// trimTrailingSpace drops collapsed spaces at the end of the pending
// runs, so a block or a line does not end in a space.
func (p *htmlParser) trimTrailingSpace() {
	for len(p.runs) > 0 {
		last := &p.runs[len(p.runs)-1]
		if last.Format == FormatCode && p.pre > 0 {
			return
		}
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text != "" {
			return
		}
		p.runs = p.runs[:len(p.runs)-1]
	}
}

// image ends the current paragraph and emits an image block. The
// Markdown renderer draws images as blocks, so text on both sides of
// an inline <img> becomes two paragraphs.
func (p *htmlParser) image() {
	p.flush()
	src := strings.TrimSpace(p.attrs.src)
	if !isSafeImagePath(src) {
		src = ""
	}
	p.emit(Block{
		IsImage:     true,
		ImageSrc:    src,
		ImageAlt:    p.attrs.alt,
		ImageWidth:  htmlImageDim(p.attrs.width),
		ImageHeight: htmlImageDim(p.attrs.height),
	})
}

// htmlImageDim parses a width or height attribute. A missing,
// non-numeric, non-positive or oversized value returns 0, which
// means "use the image's own size".
func htmlImageDim(s string) float32 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "px")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil || math.IsNaN(v) || v <= 0 || v > maxHTMLImageDim {
		return 0
	}
	return float32(v)
}

// flush turns the pending runs into one block. The kind comes from
// the open context: heading, then <pre>, then list item, then plain
// paragraph. Empty text makes no block.
func (p *htmlParser) flush() {
	if p.pre > 0 {
		code := runsToText(p.runs)
		p.runs = p.runs[:0]
		p.atSpace = true
		if strings.TrimSpace(code) != "" {
			p.emit(newCodeBlock(code, ""))
		}
		return
	}
	p.trimTrailingSpace()
	runs := trimTrailingBreaks(p.runs)
	p.runs = nil
	p.atSpace = true
	if len(runs) == 0 {
		return
	}
	b := Block{Runs: runs}
	switch {
	case len(p.lists) > 0 && p.lists[len(p.lists)-1].inItem:
		// The list item wins over a heading inside it: the renderer
		// picks the heading kind before the list kind, so a block
		// with both draws no bullet. The heading text shows as bold
		// item text instead.
		if p.heading > 0 {
			for i := range runs {
				runs[i].Format = mergeFormat(runs[i].Format, FormatBold)
			}
		}
		l := &p.lists[len(p.lists)-1]
		b.IsList = true
		b.ListIndent = l.indent
		b.ListPrefix = l.prefix
		// Later blocks of this item align under its text.
		l.prefix = strings.Repeat(" ", utf8.RuneCountInString(l.prefix))
	case p.heading > 0:
		b.HeaderLevel = p.heading
		b.AnchorSlug = headingSlug(runsToText(runs))
	}
	p.emit(b)
}

// emit appends a block, stamped with the open quote depth.
func (p *htmlParser) emit(b Block) {
	if p.bqDepth > 0 {
		b.IsBlockquote = true
		b.BlockquoteDepth = p.bqDepth
	}
	p.blocks = append(p.blocks, b)
}
