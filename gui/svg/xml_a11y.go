package svg

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// maxA11yFieldLen caps each accessibility string. Real-world values
// are short labels; any single field above this is treated as
// hostile bloat (and would otherwise pin a copy in the parsed
// cache for the SVG's lifetime).
const maxA11yFieldLen = 1024

// parseRootA11y extracts document-level accessibility metadata from
// root <svg>: first direct <title>/<desc> child element + aria-*
// attributes on root. Nested title/desc inside groups are tooltips,
// not document-level metadata, so they are ignored here.
func parseRootA11y(root *xmlNode) gui.SvgA11y {
	var a gui.SvgA11y
	if root == nil {
		return a
	}
	for i := range root.Children {
		c := &root.Children[i]
		switch c.Name {
		case "title":
			if a.Title == "" {
				a.Title = clampA11yField(c.Text)
			}
		case "desc":
			if a.Desc == "" {
				a.Desc = clampA11yField(c.Text)
			}
		}
	}
	if v, ok := root.AttrMap["aria-label"]; ok {
		a.AriaLabel = clampA11yField(v)
	}
	if v, ok := root.AttrMap["aria-roledescription"]; ok {
		a.AriaRoleDesc = clampA11yField(v)
	}
	if v, ok := root.AttrMap["aria-hidden"]; ok {
		// Bound input before EqualFold to avoid an unnecessary
		// large-string comparison on adversarial values.
		if len(v) > maxA11yFieldLen {
			v = v[:maxA11yFieldLen]
		}
		a.AriaHidden = strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return a
}

func clampA11yField(s string) string {
	if len(s) > maxA11yFieldLen {
		s = s[:maxA11yFieldLen]
	}
	return strings.TrimSpace(s)
}
