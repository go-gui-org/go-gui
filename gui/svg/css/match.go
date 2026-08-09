package css

import (
	"slices"
	"strings"
)

// Matches reports whether a compound selector matches el.
func (c compound) Matches(el ElementInfo) bool {
	if c.Tag != "" && c.Tag != "*" && c.Tag != el.Tag {
		return false
	}
	if c.ID != "" && c.ID != el.ID {
		return false
	}
	for _, cls := range c.Classes {
		if !slices.Contains(el.Classes, cls) {
			return false
		}
	}
	for _, a := range c.Attrs {
		if !matchAttr(a, el.Attrs) {
			return false
		}
	}
	if c.Root && !el.IsRoot {
		return false
	}
	if c.nthChild != nil && !c.nthChild.Matches(el.Index) {
		return false
	}
	if c.hoverPseudo && !el.State.Hover {
		return false
	}
	if c.focusPseudo && !el.State.Focus {
		return false
	}
	if c.not != nil && c.not.Matches(el) {
		return false
	}
	return true
}

// matchAttr reports whether the (op, value) pair holds for el's named
// attribute. Missing attributes never match (even AttrOpExists is
// false when the attr is absent). Empty selector value is rejected
// for operators that require a non-empty needle.
func matchAttr(a attrSel, attrs map[string]string) bool {
	v, ok := attrs[a.Name]
	if !ok {
		return false
	}
	switch a.Op {
	case attrOpExists:
		return true
	case attrOpEqual:
		return v == a.Value
	case attrOpInclude:
		if a.Value == "" || strings.ContainsAny(a.Value, " \t\n\r\f") {
			return false
		}
		return slices.Contains(strings.Fields(v), a.Value)
	case attrOpDashMatch:
		return v == a.Value || strings.HasPrefix(v, a.Value+"-")
	case attrOpPrefix:
		return a.Value != "" && strings.HasPrefix(v, a.Value)
	case attrOpSuffix:
		return a.Value != "" && strings.HasSuffix(v, a.Value)
	case attrOpSubstring:
		return a.Value != "" && strings.Contains(v, a.Value)
	}
	return false
}

// Matches reports whether the complex selector matches el under the
// given ancestor stack and preceding sibling list. Ancestors are
// ordered root-first; ancestors[len-1] is the immediate parent.
// Siblings are ordered first-to-last with siblings[len-1] = el's
// immediate previous sibling. Both slices may be nil.
func (cs complexSelector) Matches(
	el ElementInfo, ancestors, siblings []ElementInfo,
) bool {
	parts := cs.parts
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	if !last.compound.Matches(el) {
		return false
	}
	ai := len(ancestors) - 1
	si := len(siblings) - 1
	for i := range slices.Backward(parts[:len(parts)-1]) {
		comb := parts[i+1].combinator
		switch comb {
		case combChild:
			if ai < 0 || !parts[i].compound.Matches(ancestors[ai]) {
				return false
			}
			ai--
		case combDescendant:
			matched := false
			for j := ai; j >= 0; j-- {
				if parts[i].compound.Matches(ancestors[j]) {
					ai = j - 1
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case combAdjacent:
			if si < 0 || !parts[i].compound.Matches(siblings[si]) {
				return false
			}
			si--
		case combGeneralSibling:
			matched := false
			for j := si; j >= 0; j-- {
				if parts[i].compound.Matches(siblings[j]) {
					si = j - 1
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// MatchedDecl pairs a declaration with the cascade context (origin,
// specificity, source order, !important) needed to rank it against
// other declarations during the cascade walk.
type MatchedDecl struct {
	Decl
	Origin origin
	spec   Specificity
	Source int
}

// Match returns every author-rule declaration whose rule has at
// least one selector matching el under ancestors and preceding
// siblings. The result is in source order; callers run SortCascade
// to apply spec ordering.
func Match(
	rules []Rule, el ElementInfo, ancestors, siblings []ElementInfo,
) []MatchedDecl {
	if len(rules) == 0 {
		return nil
	}
	var out []MatchedDecl
	for ri := range rules {
		r := &rules[ri]
		var spec Specificity
		matched := false
		for _, sel := range r.selectors {
			if !sel.Matches(el, ancestors, siblings) {
				continue
			}
			if !matched || spec.less(sel.spec) {
				spec = sel.spec
				matched = true
			}
		}
		if !matched {
			continue
		}
		for _, d := range r.Decls {
			out = append(out, MatchedDecl{
				Decl:   d,
				Origin: originRule,
				spec:   spec,
				Source: r.Source,
			})
		}
	}
	return out
}

// SortCascade orders matched declarations by the SVG-CSS cascade.
// The comparator gives a single rank per decl that bakes Origin and
// !important into a uint8 layer (low layer applies first, high layer
// last so it wins under "last write wins"):
//
//	0  pres-attr normal
//	1  rule normal
//	2  inline normal
//	3  pres-attr important (rare; SVG pres-attrs cannot carry
//	   !important per spec, but slot is reserved for symmetry)
//	4  rule important
//	5  inline important
//
// Within a layer: specificity ascending, then source order. Stable
// so callers can iterate and apply "last write wins" per property.
func SortCascade(decls []MatchedDecl) {
	for i := 1; i < len(decls); i++ {
		j := i
		for j > 0 && cascadeLess(decls[j], decls[j-1]) {
			decls[j], decls[j-1] = decls[j-1], decls[j]
			j--
		}
	}
}

func cascadeLayer(d MatchedDecl) uint8 {
	base := uint8(d.Origin)
	if d.Important {
		return base + numOrigins
	}
	return base
}

func cascadeLess(a, b MatchedDecl) bool {
	la, lb := cascadeLayer(a), cascadeLayer(b)
	if la != lb {
		return la < lb
	}
	if a.spec != b.spec {
		return a.spec.less(b.spec)
	}
	return a.Source < b.Source
}
