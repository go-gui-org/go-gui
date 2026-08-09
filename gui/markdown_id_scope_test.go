package gui

import (
	"strings"
	"testing"
)

// mdTwoDocSource is a document whose heading and code block both
// produce IDs derived from content, not from the widget: the heading
// slug comes from the heading text and the copy button's key comes
// from the block index. Rendered twice in one window, both used to
// collide.
var mdTwoDocSource = strings.Join([]string{
	"# Shared Heading",
	"",
	"```go",
	"fmt.Println(\"hi\")",
	"```",
	"",
}, "\n")

// TestMarkdownIDsScopedToDocument renders the same source in two
// Markdown widgets and asserts the window has no duplicate IDs. Before
// heading and code-block IDs were scoped by cfg.ID, this produced one
// duplicate per heading and one per code block.
func TestMarkdownIDsScopedToDocument(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 800, Height: 600})
	w.TestRender(func(win *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				win.Markdown(MarkdownCfg{
					ID:     "doc-a",
					Source: mdTwoDocSource,
					Style:  DefaultMarkdownStyle(),
				}),
				win.Markdown(MarkdownCfg{
					ID:     "doc-b",
					Source: mdTwoDocSource,
					Style:  DefaultMarkdownStyle(),
				}),
			},
		})
	})

	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Errorf("%d duplicate IDs in window: %q", len(dups), dups)
	}
}

// TestMarkdownHeadingIDIsScoped pins the composed spelling, because it
// is what an application passes to ScrollAnchor to jump to a heading.
func TestMarkdownHeadingIDIsScoped(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.Markdown(MarkdownCfg{
		ID:     "doc-a",
		Source: "# Shared Heading\n",
		Style:  DefaultMarkdownStyle(),
	}), w)

	want := "doc-a:h:shared-heading"
	if _, ok := layout.FindByID(want); !ok {
		t.Errorf("no shape with heading ID %q; got:\n%s",
			want, strings.Join(collectShapeIDs(&layout), "\n"))
	}
}

// collectShapeIDs returns every non-empty Shape.ID in the tree, for
// failure messages.
func collectShapeIDs(l *Layout) []string {
	var out []string
	var walk func(*Layout)
	walk = func(n *Layout) {
		if n.Shape != nil && n.Shape.ID != "" {
			out = append(out, n.Shape.ID)
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(l)
	return out
}
