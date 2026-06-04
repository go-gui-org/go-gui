// Package markdown parses and renders Markdown text.
package markdown

// extensions.go provides custom goldmark inline parsers for
// math ($...$, $$...$$), highlight (==text==), underline
// (++text++), superscript (^text^), and subscript (~text~).

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// --- AST node types ---

// NodeKindMathInline is an inline math span ($...$).
var NodeKindMathInline = ast.NewNodeKind("MathInline")

type nodeMathInline struct {
	Latex string
	ast.BaseInline
}

func (*nodeMathInline) Kind() ast.NodeKind { return NodeKindMathInline }
func (n *nodeMathInline) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// NodeKindMathDisplay is a display math span ($$...$$).
var NodeKindMathDisplay = ast.NewNodeKind("MathDisplay")

type nodeMathDisplay struct {
	Latex string
	ast.BaseInline
}

func (*nodeMathDisplay) Kind() ast.NodeKind { return NodeKindMathDisplay }
func (n *nodeMathDisplay) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// NodeKindHighlight is a highlighted span (==text==).
var NodeKindHighlight = ast.NewNodeKind("Highlight")

type nodeHighlight struct {
	ast.BaseInline
}

func (*nodeHighlight) Kind() ast.NodeKind { return NodeKindHighlight }
func (n *nodeHighlight) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// NodeKindUnderline is an underlined span (++text++).
var NodeKindUnderline = ast.NewNodeKind("Underline")

type nodeUnderline struct {
	ast.BaseInline
}

func (*nodeUnderline) Kind() ast.NodeKind { return NodeKindUnderline }
func (n *nodeUnderline) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// NodeKindSuperscript is a superscript span (^text^).
var NodeKindSuperscript = ast.NewNodeKind("Superscript")

type nodeSuperscript struct {
	ast.BaseInline
}

func (*nodeSuperscript) Kind() ast.NodeKind { return NodeKindSuperscript }
func (n *nodeSuperscript) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// NodeKindSubscript is a subscript span (~text~).
var NodeKindSubscript = ast.NewNodeKind("Subscript")

type nodeSubscript struct {
	ast.BaseInline
}

func (*nodeSubscript) Kind() ast.NodeKind { return NodeKindSubscript }
func (n *nodeSubscript) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// --- Inline parsers ---

// mathInlineParser parses $...$  and $$...$$.
type mathInlineParser struct{}

func (p *mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (p *mathInlineParser) Parse(
	_ ast.Node, block text.Reader, _ parser.Context,
) ast.Node {
	line, seg := block.PeekLine()
	if len(line) == 0 || line[0] != '$' {
		return nil
	}

	// $$...$$ display math (inline span node).
	if len(line) > 1 && line[1] == '$' {
		end := findDoubleDollar(line, 2)
		if end < 0 {
			return nil
		}
		latex := string(line[2:end])
		node := &nodeMathDisplay{Latex: latex}
		block.Advance(end + 2)
		_ = seg // suppress unused
		return node
	}

	// $...$ inline math.
	// Not math: digit before $, space after opening $,
	// digit after closing $.
	pos := seg.Start
	src := block.Source()
	if pos > 0 && src[pos-1] >= '0' && src[pos-1] <= '9' {
		return nil
	}
	if len(line) < 2 || line[1] == ' ' {
		return nil
	}
	end := 2
	for end < len(line) {
		if line[end] == '$' {
			if line[end-1] == ' ' {
				end++
				continue
			}
			// Digit after closing $ → not math.
			if end+1 < len(line) &&
				line[end+1] >= '0' && line[end+1] <= '9' {
				end++
				continue
			}
			latex := string(line[1:end])
			node := &nodeMathInline{Latex: latex}
			block.Advance(end + 1)
			return node
		}
		end++
	}
	return nil
}

func findDoubleDollar(line []byte, start int) int {
	for i := start; i < len(line)-1; i++ {
		if line[i] == '$' && line[i+1] == '$' {
			return i
		}
	}
	return -1
}

// highlightDelimiterProcessor handles == delimiter matching.
type highlightDelimiterProcessor struct{}

func (p *highlightDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '='
}

func (p *highlightDelimiterProcessor) CanOpenCloser(
	opener, closer *parser.Delimiter,
) bool {
	return opener.Char == closer.Char
}

func (p *highlightDelimiterProcessor) OnMatch(
	_ int,
) ast.Node {
	return &nodeHighlight{}
}

var defaultHighlightDelimiterProcessor = &highlightDelimiterProcessor{}

// highlightParser parses ==text== using delimiter matching,
// allowing nested inline formatting.
type highlightParser struct{}

func (p *highlightParser) Trigger() []byte { return []byte{'='} }

func (p *highlightParser) Parse(
	_ ast.Node, block text.Reader, pc parser.Context,
) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(
		line, before, 2, defaultHighlightDelimiterProcessor)
	if node == nil || node.OriginalLength > 2 || before == '=' {
		return nil
	}
	node.Segment = segment.WithStop(
		segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (p *highlightParser) CloseBlock(
	_ ast.Node, _ parser.Context,
) {
}

// underlineDelimiterProcessor handles ++ delimiter matching.
type underlineDelimiterProcessor struct{}

func (p *underlineDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '+'
}

func (p *underlineDelimiterProcessor) CanOpenCloser(
	opener, closer *parser.Delimiter,
) bool {
	return opener.Char == closer.Char
}

func (p *underlineDelimiterProcessor) OnMatch(
	_ int,
) ast.Node {
	return &nodeUnderline{}
}

var defaultUnderlineDelimiterProcessor = &underlineDelimiterProcessor{}

// underlineParser parses ++text++ using delimiter matching,
// allowing nested inline formatting.
type underlineParser struct{}

func (p *underlineParser) Trigger() []byte { return []byte{'+'} }

func (p *underlineParser) Parse(
	_ ast.Node, block text.Reader, pc parser.Context,
) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(
		line, before, 2, defaultUnderlineDelimiterProcessor)
	if node == nil || node.OriginalLength > 2 || before == '+' {
		return nil
	}
	node.Segment = segment.WithStop(
		segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (p *underlineParser) CloseBlock(
	_ ast.Node, _ parser.Context,
) {
}

// superscriptDelimiterProcessor handles ^ delimiter matching.
type superscriptDelimiterProcessor struct{}

func (p *superscriptDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '^'
}

func (p *superscriptDelimiterProcessor) CanOpenCloser(
	opener, closer *parser.Delimiter,
) bool {
	return opener.Char == closer.Char
}

func (p *superscriptDelimiterProcessor) OnMatch(
	_ int,
) ast.Node {
	return &nodeSuperscript{}
}

var defaultSuperscriptDelimiterProcessor = &superscriptDelimiterProcessor{}

// superscriptParser parses ^text^ using delimiter matching,
// allowing nested inline formatting.
type superscriptParser struct{}

func (p *superscriptParser) Trigger() []byte { return []byte{'^'} }

func (p *superscriptParser) Parse(
	_ ast.Node, block text.Reader, pc parser.Context,
) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(
		line, before, 1, defaultSuperscriptDelimiterProcessor)
	if node == nil || node.OriginalLength > 1 || before == '^' {
		return nil
	}
	node.Segment = segment.WithStop(
		segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (p *superscriptParser) CloseBlock(
	_ ast.Node, _ parser.Context,
) {
}

// subscriptDelimiterProcessor handles ~ delimiter matching.
type subscriptDelimiterProcessor struct{}

func (p *subscriptDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '~'
}

func (p *subscriptDelimiterProcessor) CanOpenCloser(
	opener, closer *parser.Delimiter,
) bool {
	return opener.Char == closer.Char
}

func (p *subscriptDelimiterProcessor) OnMatch(
	_ int,
) ast.Node {
	return &nodeSubscript{}
}

var defaultSubscriptDelimiterProcessor = &subscriptDelimiterProcessor{}

// subscriptParser parses ~text~ using delimiter matching.
// Rejects ~~ (length > 1) to avoid conflict with
// strikethrough.
type subscriptParser struct{}

func (p *subscriptParser) Trigger() []byte { return []byte{'~'} }

func (p *subscriptParser) Parse(
	_ ast.Node, block text.Reader, pc parser.Context,
) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(
		line, before, 1, defaultSubscriptDelimiterProcessor)
	if node == nil || node.OriginalLength > 1 || before == '~' {
		return nil
	}
	node.Segment = segment.WithStop(
		segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (p *subscriptParser) CloseBlock(
	_ ast.Node, _ parser.Context,
) {
}
