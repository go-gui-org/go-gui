package markdown

// walker.go converts markdown source to []Block using
// goldmark as the parser backend.

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var getMarkdownParser = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.DefinitionList,
			emoji.Emoji,
		),
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&mathInlineParser{}, 100),
				util.Prioritized(&highlightParser{}, 100),
				util.Prioritized(&underlineParser{}, 100),
				util.Prioritized(&superscriptParser{}, 100),
				util.Prioritized(&subscriptParser{}, 100),
			),
		),
	)
})

// Parse converts markdown source to []Block.
func Parse(source string, hardLineBreaks bool) []Block {
	// Normalize CRLF/CR to LF. The WASM text backend's
	// Intl.Segmenter treats \r\n as a single grapheme cluster,
	// preventing newline recognition in code blocks.
	if strings.ContainsRune(source, '\r') {
		source = strings.ReplaceAll(source, "\r\n", "\n")
		source = strings.ReplaceAll(source, "\r", "\n")
	}
	source, abbrDefs, footnoteDefs := scanSource(source)
	abbrMatcher := buildAbbrMatcher(abbrDefs)
	src := []byte(source)

	md := getMarkdownParser()
	doc := md.Parser().Parse(text.NewReader(src))

	w := &mdWalker{
		source:       src,
		footnoteDefs: footnoteDefs,
		hardBreaks:   hardLineBreaks,
	}
	w.walkDocument(doc)

	for i := range w.blocks {
		w.blocks[i].Runs = mergeAdjacentRuns(
			w.blocks[i].Runs)
		if !w.blocks[i].IsCode {
			applyTypography(w.blocks[i].Runs)
		}
		if len(footnoteDefs) > 0 {
			w.blocks[i].Runs = applyFootnoteRefs(
				w.blocks[i].Runs, footnoteDefs)
		}
		if len(abbrDefs) > 0 {
			w.blocks[i].Runs = replaceAbbreviations(
				w.blocks[i].Runs, abbrMatcher)
		}
	}
	return w.blocks
}

// mdWalker walks a goldmark AST producing Blocks.
type mdWalker struct {
	footnoteDefs map[string]string
	source       []byte
	blocks       []Block
	bqDepth      int
	listDepth    int
	hardBreaks   bool
}

// inlineState tracks formatting context during inline walking.
type inlineState struct {
	link          string
	tooltip       string
	format        Format
	strikethrough bool
	highlight     bool
	underline     bool
	superscript   bool
	subscript     bool
}

type abbrMatcher struct {
	defs       map[string]string
	abbrs      []string
	firstChars [256]bool
}

func (w *mdWalker) walkDocument(doc ast.Node) {
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		w.walkBlock(c)
	}
}

func (w *mdWalker) walkBlock(node ast.Node) {
	switch node.Kind() {
	case ast.KindParagraph:
		w.walkParagraph(node)
	case ast.KindHeading:
		w.walkHeading(node.(*ast.Heading))
	case ast.KindThematicBreak:
		w.blocks = append(w.blocks, Block{IsHR: true})
	case ast.KindFencedCodeBlock:
		w.walkFencedCode(node.(*ast.FencedCodeBlock))
	case ast.KindCodeBlock:
		w.walkCodeBlock(node.(*ast.CodeBlock))
	case ast.KindBlockquote:
		w.walkBlockquote(node.(*ast.Blockquote))
	case ast.KindList:
		w.walkList(node.(*ast.List))
	case ast.KindHTMLBlock:
		// Skip raw HTML blocks.
	default:
		switch node.Kind() {
		case east.KindTable:
			w.walkTable(node.(*east.Table))
		case east.KindDefinitionList:
			w.walkDefList(node)
		default:
			for c := node.FirstChild(); c != nil; c = c.NextSibling() {
				w.walkBlock(c)
			}
		}
	}
}

func (w *mdWalker) walkParagraph(node ast.Node) {
	// Standalone image → image block.
	if node.ChildCount() == 1 &&
		node.FirstChild().Kind() == ast.KindImage {
		w.walkImage(node.FirstChild().(*ast.Image))
		return
	}
	// Standalone display math → math block.
	if node.ChildCount() == 1 &&
		node.FirstChild().Kind() == NodeKindMathDisplay {
		dm := node.FirstChild().(*nodeMathDisplay)
		w.blocks = append(w.blocks, Block{
			IsMath:    true,
			MathLatex: dm.Latex,
		})
		return
	}
	runs := w.collectRuns(node, inlineState{})
	runs = trimTrailingBreaks(runs)
	if len(runs) > 0 {
		w.blocks = append(w.blocks, Block{Runs: runs})
	}
}

func (w *mdWalker) walkHeading(h *ast.Heading) {
	runs := w.collectRuns(h, inlineState{})
	runs = trimTrailingBreaks(runs)
	slug := HeadingSlug(RunsToText(runs))
	w.blocks = append(w.blocks, Block{
		HeaderLevel: h.Level,
		AnchorSlug:  slug,
		Runs:        runs,
	})
}

func (w *mdWalker) walkFencedCode(fcb *ast.FencedCodeBlock) {
	langHint := normalizeLanguageHint(
		string(fcb.Language(w.source)))
	codeText := w.collectLines(fcb)

	if langHint == "math" {
		w.blocks = append(w.blocks, Block{
			IsMath:    true,
			MathLatex: codeText,
		})
		return
	}
	w.emitCodeBlock(codeText, langHint)
}

func (w *mdWalker) walkCodeBlock(cb *ast.CodeBlock) {
	w.emitCodeBlock(w.collectLines(cb), "")
}

func (w *mdWalker) emitCodeBlock(code, langHint string) {
	lang := LangFromHint(langHint)
	tokens := tokenizeCode(code, lang,
		maxCodeBlockHighlightBytes)
	var runs []Run
	if len(tokens) == 0 {
		runs = []Run{{Text: code, Format: FormatCode}}
	} else {
		runs = make([]Run, len(tokens))
		for i, tok := range tokens {
			runs[i] = Run{
				Text:      code[tok.Start:tok.End],
				Format:    FormatCode,
				CodeToken: tok.Kind,
			}
		}
	}
	w.blocks = append(w.blocks, Block{
		IsCode:       true,
		CodeLanguage: langHint,
		Runs:         runs,
	})
}

// collectLines extracts text from a block node's Lines().
func (w *mdWalker) collectLines(node ast.Node) string {
	type hasLines interface {
		Lines() *text.Segments
	}
	hl, ok := node.(hasLines)
	if !ok {
		return ""
	}
	segs := hl.Lines()
	var sb strings.Builder
	for i := range segs.Len() {
		seg := segs.At(i)
		sb.Write(seg.Value(w.source))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (w *mdWalker) walkBlockquote(bq *ast.Blockquote) {
	w.bqDepth++
	depth := w.bqDepth
	var runs []Run
	var nested []*ast.Blockquote
	for c := bq.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == ast.KindParagraph {
			cr := w.collectRuns(c, inlineState{})
			if len(runs) > 0 && len(cr) > 0 {
				runs = append(runs, Run{Text: "\n"})
			}
			runs = append(runs, cr...)
		} else if c.Kind() == ast.KindBlockquote {
			nested = append(nested, c.(*ast.Blockquote))
		} else {
			w.walkBlock(c)
		}
	}
	runs = trimTrailingBreaks(runs)
	if len(runs) > 0 {
		w.blocks = append(w.blocks, Block{
			IsBlockquote:    true,
			BlockquoteDepth: depth,
			Runs:            runs,
		})
	}
	// Recurse after emitting parent; bqDepth still at
	// current level so nested depths are correct.
	for _, nbq := range nested {
		w.walkBlockquote(nbq)
	}
	w.bqDepth--
}

func (w *mdWalker) walkList(list *ast.List) {
	startNum := list.Start
	for c := list.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() != ast.KindListItem {
			continue
		}
		li := c.(*ast.ListItem)

		// Determine prefix.
		var prefix string
		if list.IsOrdered() {
			prefix = fmt.Sprintf("%d. ", startNum)
			startNum++
		} else {
			prefix = "• "
		}

		// Check for task checkbox.
		if li.HasChildren() {
			p := li.FirstChild()
			if p.HasChildren() {
				fc := p.FirstChild()
				if fc.Kind() == east.KindTaskCheckBox {
					cb := fc.(*east.TaskCheckBox)
					if cb.IsChecked {
						prefix = "☑ "
					} else {
						prefix = "☐ "
					}
				}
			}
		}

		// Collect runs from inline children; defer nested
		// lists so the parent block is emitted first.
		var runs []Run
		var nested []*ast.List
		for ic := li.FirstChild(); ic != nil; ic = ic.NextSibling() {
			switch ic.Kind() {
			case ast.KindParagraph, ast.KindTextBlock:
				runs = append(runs,
					w.collectRuns(ic, inlineState{})...)
			case ast.KindList:
				nested = append(nested,
					ic.(*ast.List))
			default:
				w.walkBlock(ic)
			}
		}
		runs = trimTrailingBreaks(runs)
		w.blocks = append(w.blocks, Block{
			IsList:     true,
			ListPrefix: prefix,
			ListIndent: w.listDepth,
			Runs:       runs,
		})
		for _, nl := range nested {
			w.listDepth++
			w.walkList(nl)
			w.listDepth--
		}
	}
}

func (w *mdWalker) walkImage(img *ast.Image) {
	alt := w.collectText(img)
	src, width, height := parseImageSrc(
		string(img.Destination))
	if !isSafeImagePath(src) {
		src = ""
	}
	w.blocks = append(w.blocks, Block{
		IsImage:     true,
		ImageSrc:    src,
		ImageAlt:    alt,
		ImageWidth:  width,
		ImageHeight: height,
	})
}

func (w *mdWalker) walkTable(tbl *east.Table) {
	var aligns []Align
	for _, a := range tbl.Alignments {
		aligns = append(aligns, convertAlign(a))
	}
	var headers [][]Run
	var rows [][][]Run
	for c := tbl.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case east.KindTableHeader:
			for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if cell.Kind() == east.KindTableCell {
					headers = append(headers,
						w.collectRuns(cell, inlineState{}))
				}
			}
		case east.KindTableRow:
			var row [][]Run
			for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if cell.Kind() == east.KindTableCell {
					row = append(row,
						w.collectRuns(cell, inlineState{}))
				}
			}
			rows = append(rows, row)
		}
	}
	colCount := len(headers)
	if colCount == 0 && len(rows) > 0 {
		colCount = len(rows[0])
	}
	w.blocks = append(w.blocks, Block{
		IsTable: true,
		TableData: &Table{
			Headers:    headers,
			Alignments: aligns,
			Rows:       rows,
			ColCount:   colCount,
		},
	})
}

func convertAlign(a east.Alignment) Align {
	switch a {
	case east.AlignLeft:
		return AlignLeft
	case east.AlignRight:
		return AlignRight
	case east.AlignCenter:
		return AlignCenter
	default:
		return AlignStart
	}
}

func (w *mdWalker) walkDefList(node ast.Node) {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case east.KindDefinitionTerm:
			runs := w.collectRuns(c,
				inlineState{format: FormatBold})
			runs = trimTrailingBreaks(runs)
			w.blocks = append(w.blocks, Block{
				IsDefTerm: true,
				Runs:      runs,
			})
		case east.KindDefinitionDescription:
			var runs []Run
			for dc := c.FirstChild(); dc != nil; dc = dc.NextSibling() {
				if dc.Kind() == ast.KindTextBlock ||
					dc.Kind() == ast.KindParagraph {
					runs = append(runs,
						w.collectRuns(dc, inlineState{})...)
				}
			}
			runs = trimTrailingBreaks(runs)
			w.blocks = append(w.blocks, Block{
				IsDefValue: true,
				Runs:       runs,
			})
		}
	}
}

// --- Pre-processing ---

// reImageDims matches image links with dimension syntax:
// ](url =WxH) → ](url#dim=WxH)
// Goldmark splits on the space, so encode dims as a fragment.
var reImageDims = regexp.MustCompile(
	`(\]\([^\s)]+) (=\d+x\d+\))`)

func scanSource(source string) (string, map[string]string, map[string]string) {
	source = reImageDims.ReplaceAllString(source, "${1}#dim${2}")
	lines := strings.Split(source, "\n")
	abbrDefs := map[string]string{}
	footnoteDefs := map[string]string{}
	result := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if isAbbrDef(trimmed) {
			if len(abbrDefs) < maxAbbreviationDefs {
				idx := strings.Index(trimmed, "]:")
				if idx >= 3 {
					abbr := trimmed[2:idx]
					expansion := strings.TrimSpace(trimmed[idx+2:])
					if len(abbr) > 0 && len(expansion) > 0 {
						abbrDefs[abbr] = expansion
					}
				}
			}
			result = append(result, "")
			i++
			continue
		}

		if isFootnoteDef(trimmed) {
			var id, content string
			idx := strings.Index(trimmed, "]:")
			if idx >= 3 {
				id = trimmed[2:idx]
				content = strings.TrimSpace(trimmed[idx+2:])
			}
			result = append(result, "")
			i++
			contCount := 0
			var contentSb487 strings.Builder
			for i < len(lines) {
				next := lines[i]
				if len(next) == 0 {
					if i+1 < len(lines) &&
						len(lines[i+1]) > 0 &&
						(lines[i+1][0] == ' ' ||
							lines[i+1][0] == '\t') {
						if contCount < maxFootnoteContinuationLines {
							contentSb487.WriteString("\n\n")
						}
						result = append(result, "")
						i++
						continue
					}
					break
				}
				if next[0] != ' ' && next[0] != '\t' {
					break
				}
				if contCount < maxFootnoteContinuationLines {
					contentSb487.WriteString(" " + strings.TrimSpace(next))
					contCount++
				}
				result = append(result, "")
				i++
			}
			content += contentSb487.String()
			if len(footnoteDefs) < maxFootnoteDefs &&
				len(id) > 0 && len(content) > 0 {
				footnoteDefs[id] = content
			}
			continue
		}

		// Multi-line $$...$$ → ```math fences.
		if trimmed == "$$" {
			result = append(result, "```math")
			i++
			const maxMathLines = 200
			mathLines := 0
			for i < len(lines) && mathLines < maxMathLines {
				if strings.TrimSpace(lines[i]) == "$$" {
					result = append(result, "```")
					i++
					break
				}
				result = append(result, lines[i])
				i++
				mathLines++
			}
			continue
		}

		result = append(result, line)
		i++
	}
	return strings.Join(result, "\n"), abbrDefs, footnoteDefs
}

func isAbbrDef(line string) bool {
	return strings.HasPrefix(line, "*[") &&
		strings.Contains(line, "]:")
}

func isFootnoteDef(line string) bool {
	if !strings.HasPrefix(line, "[^") {
		return false
	}
	return strings.Index(line, "]:") >= 3
}
