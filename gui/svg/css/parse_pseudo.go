package css

import (
	"strconv"
	"strings"

	tdcss "github.com/tdewolff/parse/v2/css"
)

// parsePseudoClass handles a ColonToken at index i and updates c
// with the recognized pseudo-class (:root, :hover, :focus,
// :nth-child(...), or :not(...)). depth bounds nested `:not()`
// recursion. Returns the new token index (the loop's i++ moves past
// it) and ok=false on unsupported pseudo-classes.
func parsePseudoClass(
	toks []tdcss.Token, i int, c *Compound, depth int,
) (int, bool) {
	if i+1 >= len(toks) {
		return i, false
	}
	nx := toks[i+1]
	switch nx.TokenType {
	case tdcss.IdentToken:
		switch strings.ToLower(string(nx.Data)) {
		case "root":
			c.Root = true
			c.Spec[1]++
			return i + 1, true
		case "hover":
			c.HoverPseudo = true
			c.Spec[1]++
			return i + 1, true
		case "focus":
			c.FocusPseudo = true
			c.Spec[1]++
			return i + 1, true
		}
		return i, false
	case tdcss.FunctionToken:
		fname := strings.ToLower(
			strings.TrimSuffix(string(nx.Data), "("))
		end := skipFunctionArgs(toks, i+2)
		if end < 0 {
			return i, false
		}
		switch fname {
		case "nth-child":
			f, ok := parseNthFormula(joinTokens(toks[i+2 : end]))
			if !ok {
				return i, false
			}
			c.NthChild = &f
			c.Spec[1]++
			return end, true
		case "not":
			if c.Not != nil {
				return i, false
			}
			inner, ok := parseCompoundAt(
				trimWS(toks[i+2:end]), depth+1,
			)
			if !ok {
				return i, false
			}
			c.Not = &inner
			// :not(x) adds the specificity of its argument (CSS
			// Selectors L4). Computed with Specificity.Add (rather
			// than the c.Spec[1]++ pattern used by other pseudos)
			// because the inner compound carries its own composite
			// specificity, not a single-tier bump.
			c.Spec = c.Spec.Add(inner.Spec)
			return end, true
		}
		return i, false
	}
	return i, false
}

// parseNthFormula parses :nth-child argument syntax: odd/even, a
// constant, or an+b in the variants documented inline.
func parseNthFormula(s string) (NthFormula, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return NthFormula{}, false
	}
	switch s {
	case "odd":
		return NthFormula{A: 2, B: 1}, true
	case "even":
		return NthFormula{A: 2, B: 0}, true
	}
	// Strip internal whitespace so "2n + 1" reads as "2n+1".
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			continue
		}
		b.WriteByte(s[i])
	}
	s = b.String()
	aPart, bPart, hasN := strings.Cut(s, "n")
	if !hasN {
		v, err := strconv.Atoi(s)
		if err != nil {
			return NthFormula{}, false
		}
		return NthFormula{A: 0, B: v}, true
	}
	var a int
	switch aPart {
	case "", "+":
		a = 1
	case "-":
		a = -1
	default:
		v, err := strconv.Atoi(aPart)
		if err != nil {
			return NthFormula{}, false
		}
		a = v
	}
	bVal := 0
	if bPart != "" {
		v, err := strconv.Atoi(bPart)
		if err != nil {
			return NthFormula{}, false
		}
		bVal = v
	}
	return NthFormula{A: a, B: bVal}, true
}
