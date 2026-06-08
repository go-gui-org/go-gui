package gui

import (
	"strings"
	"testing"
)

// TestMarkdownTableIDsUnique verifies that two tables inside a
// single markdown document generate distinct Table widget IDs so
// their registry state (column widths) cannot collide.
func TestMarkdownTableIDsUnique(t *testing.T) {
	source := strings.Join([]string{
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
		"",
		"| X | Y |",
		"|---|---|",
		"| 3 | 4 |",
		"",
	}, "\n")

	w := &Window{}
	layout := generateViewLayout(w.Markdown(MarkdownCfg{
		ID:     "md-doc",
		Source: source,
		Style:  DefaultMarkdownStyle(),
	}), w)

	ids := collectTableShapeIDs(&layout)
	if len(ids) < 2 {
		t.Fatalf("expected at least 2 tables, found %d", len(ids))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" {
			t.Error("table shape ID must be non-empty")
		}
		if !strings.HasPrefix(id, "md-doc.table.") {
			t.Errorf("id %q missing prefix md-doc.table.", id)
		}
		if seen[id] {
			t.Errorf("duplicate table id %q", id)
		}
		seen[id] = true
	}
}

// collectTableShapeIDs walks the layout tree and returns Shape.ID
// of every node whose ID starts with the markdown table prefix.
func collectTableShapeIDs(l *Layout) []string {
	var out []string
	var walk func(*Layout)
	walk = func(n *Layout) {
		if strings.HasPrefix(n.Shape.ID, "md-doc.table.") {
			out = append(out, n.Shape.ID)
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(l)
	return out
}
