package markdown

// walker_inline.go — inline-level AST walking (text, emphasis,
// links, code spans, extensions).

import (
	"fmt"
	"strings"

	emast "github.com/yuin/goldmark-emoji/ast"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
)

// collectRuns walks inline children producing flat Run slices.
func (w *mdWalker) collectRuns(
	node ast.Node, state inlineState,
) []Run {
	runs := make([]Run, 0, node.ChildCount())
	return w.collectRunsInto(runs, node, state)
}

func (w *mdWalker) collectRunsInto(
	dst []Run, node ast.Node, state inlineState,
) []Run {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		dst = w.walkInline(dst, c, state)
	}
	return dst
}

func (w *mdWalker) walkInline(
	dst []Run, node ast.Node, state inlineState,
) []Run {
	switch node.Kind() {
	case ast.KindText:
		return w.walkText(dst, node.(*ast.Text), state)
	case ast.KindString:
		s := node.(*ast.String)
		v := string(s.Value)
		if len(v) == 0 {
			return dst
		}
		return append(dst, w.makeRun(v, state))
	case ast.KindEmphasis:
		return w.walkEmphasis(dst, node.(*ast.Emphasis), state)
	case ast.KindCodeSpan:
		t := w.collectText(node)
		return append(dst, Run{
			Text: t, Format: FormatCode, Link: state.link,
		})
	case ast.KindLink:
		return w.walkLink(dst, node.(*ast.Link), state)
	case ast.KindAutoLink:
		return w.walkAutoLink(dst, node.(*ast.AutoLink), state)
	case ast.KindImage:
		alt := w.collectText(node)
		return append(dst, Run{Text: alt, Format: state.format})
	case ast.KindRawHTML:
		return dst
	case emast.KindEmoji:
		e := node.(*emast.Emoji)
		if len(e.Value.Unicode) > 0 {
			return append(dst, w.makeRun(
				string(e.Value.Unicode), state))
		}
		return append(dst, w.makeRun(
			":"+string(e.ShortName)+":", state))
	default:
		return w.walkInlineExt(dst, node, state)
	}
}

func (w *mdWalker) walkText(
	dst []Run, t *ast.Text, state inlineState,
) []Run {
	seg := t.Segment
	v := string(seg.Value(w.source))
	if len(v) > 0 {
		dst = append(dst, w.makeRun(v, state))
	}
	if t.HardLineBreak() ||
		(w.hardBreaks && t.SoftLineBreak()) {
		dst = append(dst, Run{Text: "\n"})
	} else if t.SoftLineBreak() {
		dst = append(dst, Run{Text: " ", Format: state.format})
	}
	return dst
}

func (w *mdWalker) walkEmphasis(
	dst []Run, em *ast.Emphasis, state inlineState,
) []Run {
	ns := state
	if em.Level == 1 {
		ns.format = mergeFormat(ns.format, FormatItalic)
	} else {
		ns.format = mergeFormat(ns.format, FormatBold)
	}
	return w.collectRunsInto(dst, em, ns)
}

func (w *mdWalker) walkLink(
	dst []Run, link *ast.Link, state inlineState,
) []Run {
	url := string(link.Destination)
	if !IsSafeURL(url) {
		url = ""
	}
	ns := state
	ns.link = url
	return w.collectRunsInto(dst, link, ns)
}

func (w *mdWalker) walkAutoLink(
	dst []Run, al *ast.AutoLink, state inlineState,
) []Run {
	url := string(al.URL(w.source))
	label := string(al.Label(w.source))
	if !IsSafeURL(url) {
		return append(dst, Run{
			Text: label, Format: state.format,
		})
	}
	return append(dst, Run{
		Text: label, Format: state.format, Link: url,
	})
}

func (w *mdWalker) walkInlineExt(
	dst []Run, node ast.Node, state inlineState,
) []Run {
	switch kind := node.Kind(); kind {
	case east.KindStrikethrough:
		ns := state
		ns.strikethrough = true
		return w.collectRunsInto(dst, node, ns)
	case east.KindTaskCheckBox:
		return dst
	case nodeKindMathInline:
		mi := node.(*nodeMathInline)
		// A math cache key, not a widget ID.
		id := fmt.Sprintf("math_%x", MathHash(mi.Latex)) // ergoaudit:not-an-id
		return append(dst, Run{
			MathID: id, MathLatex: mi.Latex, Format: state.format,
		})
	case nodeKindMathDisplay:
		md := node.(*nodeMathDisplay)
		// A math cache key, not a widget ID.
		id := fmt.Sprintf("math_%x", MathHash(md.Latex)) // ergoaudit:not-an-id
		return append(dst, Run{
			MathID: id, MathLatex: md.Latex, Format: state.format,
		})
	case nodeKindHighlight:
		ns := state
		ns.highlight = true
		return w.collectRunsInto(dst, node, ns)
	case nodeKindUnderline:
		ns := state
		ns.underline = true
		return w.collectRunsInto(dst, node, ns)
	case nodeKindSuperscript:
		ns := state
		ns.superscript = true
		return w.collectRunsInto(dst, node, ns)
	case nodeKindSubscript:
		ns := state
		ns.subscript = true
		return w.collectRunsInto(dst, node, ns)
	default:
		return w.collectRunsInto(dst, node, state)
	}
}

func (w *mdWalker) makeRun(
	text string, s inlineState,
) Run {
	return Run{
		Text:          text,
		Format:        s.format,
		Strikethrough: s.strikethrough,
		Highlight:     s.highlight,
		Underline:     s.underline,
		Superscript:   s.superscript,
		Subscript:     s.subscript,
		Link:          s.link,
		Tooltip:       s.tooltip,
	}
}

// collectText extracts plain text from a node tree.
func (w *mdWalker) collectText(node ast.Node) string {
	if node == nil {
		return ""
	}
	var sb strings.Builder
	var local [16]ast.Node
	stack := local[:0]
	for c := node.LastChild(); c != nil; c = c.PreviousSibling() {
		stack = append(stack, c)
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch n.Kind() {
		case ast.KindText:
			t := n.(*ast.Text)
			seg := t.Segment
			sb.Write(seg.Value(w.source))
		case ast.KindString:
			sb.Write(n.(*ast.String).Value)
		}
		for c := n.LastChild(); c != nil; c = c.PreviousSibling() {
			stack = append(stack, c)
		}
	}
	return sb.String()
}

// mergeFormat combines formatting levels.
func mergeFormat(parent, child Format) Format {
	if parent == FormatCode || child == FormatCode {
		return FormatCode
	}
	switch parent {
	case formatPlain:
		return child
	case FormatBold:
		if child == FormatItalic ||
			child == FormatBoldItalic {
			return FormatBoldItalic
		}
		return parent
	case FormatItalic:
		if child == FormatBold ||
			child == FormatBoldItalic {
			return FormatBoldItalic
		}
		return parent
	case FormatBoldItalic:
		return FormatBoldItalic
	}
	return parent
}
