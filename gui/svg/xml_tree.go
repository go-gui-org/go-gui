package svg

// xml_tree.go — SVG document decoded once into an xmlNode tree via
// encoding/xml. All downstream parsing walks this tree; no substring
// tag scanning remains in the parsing path.
//
// Helpers that take raw opening-tag strings (findAttr,
// findAttrOrStyle, computeStyle, shape/animation parsers) keep
// their signatures. The walker hands them the reconstructed OpenTag
// text of the current node, so the entire attribute-read path is
// unchanged from the old string-scan backbone.
//
// For elements whose body content is needed as text (<text>,
// <tspan>, <textPath>), the decoder's concatenated character data
// is surfaced on the node. Child elements are navigated via
// Children — no body re-scan required.

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// xmlAttr stores one attribute in the authored name form. xlink: and
// xml: prefixes are preserved so findAttr(openTag, "xlink:href")
// matches. Other namespaces fall back to the local name.
type xmlAttr struct {
	Name  string
	Value string
}

// xmlNode is one decoded SVG element.
type xmlNode struct {
	Name      string            // local tag name
	Attrs     []xmlAttr         // authored order
	AttrMap   map[string]string // fast lookup by authored name
	OpenTag   string            // reconstructed "<name a=b c=d>" or "<name .../>"
	Leading   string            // text content before first child element
	Text      string            // all character data concatenated (trimmed on use)
	Tail      string            // text following this node's close tag, before the next sibling
	Children  []xmlNode
	SelfClose bool
}

// decodeSvgTree parses an SVG document into an xmlNode tree.
// Returns the root element (the outermost tag) or an error if the
// document is malformed or exceeds limits.
func decodeSvgTree(content string) (*xmlNode, error) {
	dec := xml.NewDecoder(strings.NewReader(content))
	dec.Strict = true
	// HTML entities are not enabled by default; SVG assets use named
	// entities rarely, numeric references are always decoded.
	dec.Entity = xml.HTMLEntity

	var root *xmlNode
	stack := make([]*xmlNode, 0, 16)
	// Parallel builder stack avoids O(N²) string concatenation when a
	// hostile <text> body is delivered as many small CharData chunks.
	// pendingTailIdx tracks which child of the current node the next
	// chunk of post-child CharData attaches to; -1 means "no pending
	// tail yet". The builder is flushed into Children[idx].Tail at
	// EndElement or before a new sibling append (which can reallocate
	// the backing array and invalidate index targets).
	type bldrFrame struct {
		text           strings.Builder
		leading        strings.Builder
		tail           strings.Builder
		pendingTailIdx int
	}
	bstack := make([]*bldrFrame, 0, 16)
	elemCount := 0
	depth := 0

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("svg: decode XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			elemCount++
			if elemCount > maxElements {
				return nil, errors.New("svg: element limit exceeded")
			}
			depth++
			if depth > maxGroupDepth+8 {
				// +8 slack vs group depth: the root and defs can
				// wrap genuine group nesting.
				return nil, errors.New("svg: depth limit exceeded")
			}
			n := xmlNode{Name: t.Name.Local}
			n.Attrs = make([]xmlAttr, 0, len(t.Attr))
			n.AttrMap = make(map[string]string, len(t.Attr))
			for _, a := range t.Attr {
				name := attrAuthoredName(a.Name)
				if name == "" {
					continue
				}
				if len(a.Value) > maxAttrLen {
					continue
				}
				n.Attrs = append(n.Attrs, xmlAttr{Name: name, Value: a.Value})
				n.AttrMap[name] = a.Value
			}
			n.OpenTag = buildOpenTag(n.Name, n.Attrs, false)
			if len(stack) == 0 {
				// Attach to root holder. We defer until EndElement to
				// finalize the node.
				stack = append(stack, &n)
			} else {
				parent := stack[len(stack)-1]
				pf := bstack[len(bstack)-1]
				// Flush any pending tail before append: a grow of
				// parent.Children invalidates pointer targets but not
				// the index-based flush since we write before the
				// append below.
				if pf.pendingTailIdx >= 0 && pf.tail.Len() > 0 {
					parent.Children[pf.pendingTailIdx].Tail = pf.tail.String()
				}
				pf.tail.Reset()
				pf.pendingTailIdx = -1
				parent.Children = append(parent.Children, n)
				// Push pointer to the slot we just appended.
				stack = append(stack, &parent.Children[len(parent.Children)-1])
			}
			bstack = append(bstack, &bldrFrame{pendingTailIdx: -1})
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("svg: unbalanced close </%s>",
					t.Name.Local)
			}
			top := stack[len(stack)-1]
			pf := bstack[len(bstack)-1]
			// Flush pending tail before this node finalizes; pendingTailIdx
			// indexes into top.Children whose backing array is stable for
			// the remainder of this scope.
			if pf.pendingTailIdx >= 0 && pf.tail.Len() > 0 {
				top.Children[pf.pendingTailIdx].Tail = pf.tail.String()
			}
			top.Text = pf.text.String()
			top.Leading = pf.leading.String()
			stack = stack[:len(stack)-1]
			bstack = bstack[:len(bstack)-1]
			depth--
			if len(stack) == 0 {
				// Finished the root element.
				root = top
			}
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			pf := bstack[len(bstack)-1]
			s := string(t)
			pf.text.WriteString(s)
			if len(top.Children) == 0 {
				pf.leading.WriteString(s)
			} else {
				// Tail belongs to the most recent child. If a previous
				// pending tail targeted an earlier child (shouldn't
				// happen — appends flush first — but defend anyway),
				// flush it before retargeting.
				curIdx := len(top.Children) - 1
				if pf.pendingTailIdx >= 0 && pf.pendingTailIdx != curIdx &&
					pf.tail.Len() > 0 {
					top.Children[pf.pendingTailIdx].Tail = pf.tail.String()
					pf.tail.Reset()
				}
				pf.pendingTailIdx = curIdx
				pf.tail.WriteString(s)
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
			// Ignore.
		}
	}

	if len(stack) != 0 {
		return nil, fmt.Errorf("svg: unterminated element <%s>",
			stack[len(stack)-1].Name)
	}
	if root == nil {
		return nil, errors.New("svg: no root element")
	}
	// Finalize: emit self-closing OpenTag for childless empty nodes
	// so helpers that inspect the closing "/>" see the right form.
	finalizeSelfClose(root)
	return root, nil
}

// attrAuthoredName reconstructs the authored attribute name from
// encoding/xml's namespace-resolved xml.Name. xlink and xml prefixes
// are the only namespaces SVG attributes routinely use; other
// namespaces fall through to the local name.
func attrAuthoredName(n xml.Name) string {
	switch n.Space {
	case "":
		return n.Local
	case "http://www.w3.org/1999/xlink":
		return "xlink:" + n.Local
	case "http://www.w3.org/XML/1998/namespace":
		return "xml:" + n.Local
	case "http://www.w3.org/2000/svg":
		return n.Local
	default:
		// xmlns declarations arrive as Space="xmlns", Local=prefix —
		// drop them; they don't participate in presentation attrs.
		if n.Space == "xmlns" {
			return ""
		}
		return n.Local
	}
}

// buildOpenTag reconstructs a raw opening-tag string so helpers like
// findAttr can substring-scan a single element. selfClose controls
// the closing slash.
func buildOpenTag(name string, attrs []xmlAttr, selfClose bool) string {
	var b strings.Builder
	b.Grow(16 + len(attrs)*24)
	b.WriteByte('<')
	b.WriteString(name)
	for _, a := range attrs {
		b.WriteByte(' ')
		b.WriteString(a.Name)
		b.WriteString(`="`)
		// Attributes come back from encoding/xml already decoded
		// (entities expanded). Re-escape every character a downstream
		// substring scanner (findAttr, findStyleProperty) could treat
		// as a value boundary or markup token — both quote styles, &,
		// and angle brackets — so a hostile attr value cannot smuggle
		// a fake attribute past the cascade.
		writeAttrEscaped(&b, a.Value)
		b.WriteByte('"')
	}
	if selfClose {
		b.WriteString("/>")
	} else {
		b.WriteByte('>')
	}
	return b.String()
}

func writeAttrEscaped(b *strings.Builder, v string) {
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '"':
			b.WriteString("&quot;")
		case '\'':
			// Escape single quote so a value cannot smuggle a fake
			// attribute that findAttr's quote-aware scan would honor
			// (e.g. note=" x='99' " injecting an x override).
			b.WriteString("&#39;")
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(v[i])
		}
	}
}

// finalizeSelfClose walks the tree and marks nodes with no children
// and no text content as self-closing, rewriting OpenTag to the />
// form. Matches the old scanner's isSelfClosing detection.
func finalizeSelfClose(n *xmlNode) {
	if len(n.Children) == 0 && strings.TrimSpace(n.Text) == "" {
		n.SelfClose = true
		n.OpenTag = buildOpenTag(n.Name, n.Attrs, true)
	}
	for i := range n.Children {
		finalizeSelfClose(&n.Children[i])
	}
}

// findChild returns the first child element whose local name matches
// any of the given names.
func (n *xmlNode) findChild(names ...string) *xmlNode {
	for i := range n.Children {
		c := &n.Children[i]
		if slices.Contains(names, c.Name) {
			return c
		}
	}
	return nil
}
