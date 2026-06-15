package css

import (
	"strings"

	tdcss "github.com/tdewolff/parse/v2/css"
)

// parseSelectorList splits a selector token stream by ',' and parses
// each group into a ComplexSelector. Groups containing unrecognized
// pseudo-classes or syntactically malformed compounds are dropped.
// `:is(a, b)` groups expand into one selector per argument before
// parsing.
func parseSelectorList(toks []tdcss.Token) []ComplexSelector {
	var out []ComplexSelector
	for _, g := range splitByComma(toks) {
		for _, expanded := range expandIs(g, 0) {
			cs, ok := parseComplexSelector(expanded)
			if !ok {
				continue
			}
			out = append(out, cs)
		}
	}
	return out
}

// maxIsExpansion caps fan-out from nested :is() to bound CPU on
// adversarial selectors. Real-world authored CSS uses single-level
// :is() with a handful of args — the cap is well above any practical
// document.
const maxIsExpansion = 256

// maxIsDepth caps recursion depth on adversarial nested :is() so a
// pathological stylesheet cannot exhaust the goroutine stack.
const maxIsDepth = 16

// expandIs rewrites `prefix :is(a, b, ...) suffix` into
// `[prefix a suffix, prefix b suffix, ...]`, recursing so nested
// :is() also expand. Selectors with no `:is(` are returned as a
// single-group slice. Specificity for :is() args nests via standard
// compound parsing (we lose the spec rule "outer specificity = max of
// inner" but the matched compound carries its own specificity, which
// is good enough for our targets).
func expandIs(toks []tdcss.Token, depth int) [][]tdcss.Token {
	if depth >= maxIsDepth {
		return [][]tdcss.Token{toks}
	}
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].TokenType != tdcss.ColonToken {
			continue
		}
		nx := toks[i+1]
		if nx.TokenType != tdcss.FunctionToken {
			continue
		}
		fname := strings.ToLower(strings.TrimSuffix(string(nx.Data), "("))
		if fname != "is" {
			continue
		}
		end := skipFunctionArgs(toks, i+2)
		if end < 0 {
			break
		}
		args := splitByComma(toks[i+2 : end])
		prefix := toks[:i]
		suffix := toks[end+1:]
		var out [][]tdcss.Token
		for _, a := range args {
			a = trimWS(a)
			if len(a) == 0 {
				continue
			}
			merged := make([]tdcss.Token, 0, len(prefix)+len(a)+len(suffix))
			merged = append(merged, prefix...)
			merged = append(merged, a...)
			merged = append(merged, suffix...)
			for _, e := range expandIs(merged, depth+1) {
				if len(out) >= maxIsExpansion {
					return out
				}
				out = append(out, e)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return [][]tdcss.Token{toks}
}

func splitByComma(toks []tdcss.Token) [][]tdcss.Token {
	var out [][]tdcss.Token
	start := 0
	depth := 0
	for i, t := range toks {
		switch t.TokenType {
		case tdcss.FunctionToken, tdcss.LeftParenthesisToken:
			depth++
		case tdcss.RightParenthesisToken:
			if depth > 0 {
				depth--
			}
		case tdcss.CommaToken:
			if depth == 0 {
				out = append(out, trimWS(toks[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, trimWS(toks[start:]))
	return out
}

func trimWS(toks []tdcss.Token) []tdcss.Token {
	for len(toks) > 0 && toks[0].TokenType == tdcss.WhitespaceToken {
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].TokenType == tdcss.WhitespaceToken {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// parseComplexSelector walks a selector group, splitting it into
// compound chunks separated by descendant, child, adjacent (`+`), or
// general-sibling (`~`) combinators.
func parseComplexSelector(toks []tdcss.Token) (ComplexSelector, bool) {
	if len(toks) == 0 {
		return ComplexSelector{}, false
	}
	var parts []SelectorPart
	nextComb := CombStart
	i := 0
	for i < len(toks) {
		// Collect tokens for this compound until we hit whitespace,
		// or one of the explicit combinator delim tokens.
		start := i
		for i < len(toks) {
			t := toks[i]
			if t.TokenType == tdcss.WhitespaceToken {
				break
			}
			if _, ok := combinatorFromDelim(t); ok {
				break
			}
			// Skip the matched argument span of a function token
			// (nth-child / :not() / :is() arg list).
			if t.TokenType == tdcss.FunctionToken {
				j := skipFunctionArgs(toks, i+1)
				if j < 0 {
					return ComplexSelector{}, false
				}
				i = j + 1
				continue
			}
			// Skip the matched [...] span so internal whitespace in
			// `[name = "value"]` does not split the compound.
			if t.TokenType == tdcss.LeftBracketToken {
				j := skipBrackets(toks, i+1)
				if j < 0 {
					return ComplexSelector{}, false
				}
				i = j + 1
				continue
			}
			i++
		}
		chunk := toks[start:i]
		if len(chunk) == 0 {
			return ComplexSelector{}, false
		}
		c, ok := parseCompound(chunk)
		if !ok {
			return ComplexSelector{}, false
		}
		parts = append(parts, SelectorPart{
			Combinator: nextComb,
			Compound:   c,
		})
		// Skip whitespace, then accept an optional explicit combinator
		// delim. Whitespace alone is the descendant combinator.
		sawWS := false
		for i < len(toks) && toks[i].TokenType == tdcss.WhitespaceToken {
			sawWS = true
			i++
		}
		if i >= len(toks) {
			break
		}
		if comb, ok := combinatorFromDelim(toks[i]); ok {
			nextComb = comb
			i++
			for i < len(toks) && toks[i].TokenType == tdcss.WhitespaceToken {
				i++
			}
		} else if sawWS {
			nextComb = CombDescendant
		} else {
			// No whitespace, no combinator, yet more tokens — malformed.
			return ComplexSelector{}, false
		}
	}
	if len(parts) == 0 {
		return ComplexSelector{}, false
	}
	var spec Specificity
	for _, p := range parts {
		spec = spec.Add(p.Compound.Spec)
	}
	return ComplexSelector{Parts: parts, Spec: spec}, true
}

// combinatorFromDelim recognizes the single-char combinator delim
// tokens. Returns the combinator and true on match.
func combinatorFromDelim(t tdcss.Token) (Combinator, bool) {
	if t.TokenType != tdcss.DelimToken || len(t.Data) != 1 {
		return 0, false
	}
	switch t.Data[0] {
	case '>':
		return CombChild, true
	case '+':
		return CombAdjacent, true
	case '~':
		return CombGeneralSibling, true
	}
	return 0, false
}

// skipBrackets returns the index of the RightBracketToken matching a
// LeftBracketToken's opening. start points one past the
// LeftBracketToken.
func skipBrackets(toks []tdcss.Token, start int) int {
	depth := 1
	for j := start; j < len(toks); j++ {
		switch toks[j].TokenType {
		case tdcss.LeftBracketToken:
			depth++
		case tdcss.RightBracketToken:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// skipFunctionArgs returns the index of the RightParenthesisToken
// matching a function token's opening paren. start points one past
// the FunctionToken.
func skipFunctionArgs(toks []tdcss.Token, start int) int {
	depth := 1
	for j := start; j < len(toks); j++ {
		switch toks[j].TokenType {
		case tdcss.FunctionToken, tdcss.LeftParenthesisToken:
			depth++
		case tdcss.RightParenthesisToken:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// maxNotDepth caps recursion depth on adversarial nested `:not(...)`
// so a pathological stylesheet cannot exhaust the goroutine stack.
// Mirrors maxIsDepth's role for `:is()` expansion.
const maxNotDepth = 8

// parseCompound parses one compound selector. Rejects (returns
// ok=false) any chunk that contains unsupported constructs. Top-level
// callers use parseCompound; the `:not(inner)` handler recurses via
// parseCompoundAt with an incremented depth so nested negation cannot
// blow the stack.
func parseCompound(toks []tdcss.Token) (Compound, bool) {
	return parseCompoundAt(toks, 0)
}

func parseCompoundAt(toks []tdcss.Token, depth int) (Compound, bool) {
	if len(toks) == 0 {
		return Compound{}, false
	}
	if depth > maxNotDepth {
		return Compound{}, false
	}
	var c Compound
	tagSeen := false
	for i := 0; i < len(toks); i++ {
		adv, ok := parseCompoundToken(toks, i, &c, &tagSeen, depth)
		if !ok {
			return Compound{}, false
		}
		i = adv
	}
	if !compoundIsNonEmpty(&c, tagSeen) {
		return Compound{}, false
	}
	return c, true
}

// parseCompoundToken handles one token in the compound stream. It
// returns the advanced index (the loop's i++ moves past it) and
// ok=false on rejection.
func parseCompoundToken(
	toks []tdcss.Token, i int, c *Compound, tagSeen *bool, depth int,
) (int, bool) {
	t := toks[i]
	switch t.TokenType {
	case tdcss.IdentToken:
		if *tagSeen || !compoundEmpty(c) {
			return i, false
		}
		c.Tag = string(t.Data)
		c.Spec[2]++
		*tagSeen = true
		return i, true
	case tdcss.HashToken:
		return parseCompoundHash(t, i, c)
	case tdcss.DelimToken:
		return parseCompoundDelim(toks, i, c, tagSeen)
	case tdcss.LeftBracketToken:
		return parseCompoundAttr(toks, i, c)
	case tdcss.ColonToken:
		return parsePseudoClass(toks, i, c, depth)
	}
	return i, false
}

func parseCompoundHash(t tdcss.Token, i int, c *Compound) (int, bool) {
	data := t.Data
	if len(data) > 0 && data[0] == '#' {
		data = data[1:]
	}
	if len(data) == 0 || c.ID != "" {
		return i, false
	}
	c.ID = string(data)
	c.Spec[0]++
	return i, true
}

func parseCompoundDelim(
	toks []tdcss.Token, i int, c *Compound, tagSeen *bool,
) (int, bool) {
	t := toks[i]
	if len(t.Data) != 1 {
		return i, false
	}
	switch t.Data[0] {
	case '.':
		if i+1 >= len(toks) ||
			toks[i+1].TokenType != tdcss.IdentToken {
			return i, false
		}
		c.Classes = append(c.Classes, string(toks[i+1].Data))
		c.Spec[1]++
		return i + 1, true
	case '*':
		if *tagSeen {
			return i, false
		}
		c.Tag = "*"
		*tagSeen = true
		return i, true
	}
	return i, false
}

func parseCompoundAttr(
	toks []tdcss.Token, i int, c *Compound,
) (int, bool) {
	end := skipBrackets(toks, i+1)
	if end < 0 {
		return i, false
	}
	a, ok := parseAttrSel(toks[i+1 : end])
	if !ok {
		return i, false
	}
	c.Attrs = append(c.Attrs, a)
	c.Spec[1]++
	return end, true
}

// compoundEmpty reports whether c carries no constraint other than a
// possibly-pending tag selector. Used by IdentToken handling: an
// element-name selector must come first in the compound.
func compoundEmpty(c *Compound) bool {
	return c.ID == "" && len(c.Classes) == 0 && len(c.Attrs) == 0 &&
		c.NthChild == nil && !c.Root && !c.HoverPseudo &&
		!c.FocusPseudo && c.Not == nil
}

// compoundIsNonEmpty reports whether c carries at least one selector
// constraint. A compound chunk that produced no constraints is
// rejected by parseCompound.
func compoundIsNonEmpty(c *Compound, tagSeen bool) bool {
	return tagSeen || !compoundEmpty(c)
}

// parseAttrSel parses the inner tokens of a `[...]` attribute selector
// (the tokens between but not including the brackets). Supported
// shapes: `name`, `name=value`, `name~=value`, `name|=value`,
// `name^=value`, `name$=value`, `name*=value`. Value may be IdentToken,
// NumberToken, or StringToken (quoted). Empty value is rejected for
// operators that require a non-empty needle. Case-sensitive matching
// (no `i` / `s` flag).
func parseAttrSel(toks []tdcss.Token) (AttrSel, bool) {
	toks = trimWS(toks)
	if len(toks) == 0 || toks[0].TokenType != tdcss.IdentToken {
		return AttrSel{}, false
	}
	name := strings.ToLower(string(toks[0].Data))
	rest := trimWS(toks[1:])
	if len(rest) == 0 {
		return AttrSel{Name: name, Op: AttrOpExists}, true
	}
	op, opLen, ok := parseAttrOp(rest)
	if !ok {
		return AttrSel{}, false
	}
	rest = trimWS(rest[opLen:])
	if len(rest) != 1 {
		return AttrSel{}, false
	}
	val, ok := attrValueText(rest[0])
	if !ok {
		return AttrSel{}, false
	}
	return AttrSel{Name: name, Op: op, Value: val}, true
}

// parseAttrOp recognizes the operator tokens that follow the attribute
// name in `[name op value]`. Returns the op, the number of tokens
// consumed, and ok.
func parseAttrOp(toks []tdcss.Token) (AttrOp, int, bool) {
	if len(toks) == 0 {
		return 0, 0, false
	}
	t := toks[0]
	switch t.TokenType {
	case tdcss.IncludeMatchToken:
		return AttrOpInclude, 1, true
	case tdcss.DashMatchToken:
		return AttrOpDashMatch, 1, true
	case tdcss.PrefixMatchToken:
		return AttrOpPrefix, 1, true
	case tdcss.SuffixMatchToken:
		return AttrOpSuffix, 1, true
	case tdcss.SubstringMatchToken:
		return AttrOpSubstring, 1, true
	case tdcss.DelimToken:
		if len(t.Data) == 1 && t.Data[0] == '=' {
			return AttrOpEqual, 1, true
		}
	}
	return 0, 0, false
}

// attrValueText extracts the literal text of an attribute selector
// value token, stripping matched quotes from string tokens.
func attrValueText(t tdcss.Token) (string, bool) {
	switch t.TokenType {
	case tdcss.IdentToken, tdcss.NumberToken, tdcss.DimensionToken:
		return string(t.Data), true
	case tdcss.StringToken:
		d := t.Data
		if len(d) >= 2 {
			q := d[0]
			if (q == '"' || q == '\'') && d[len(d)-1] == q {
				return string(d[1 : len(d)-1]), true
			}
		}
		return string(d), true
	}
	return "", false
}
