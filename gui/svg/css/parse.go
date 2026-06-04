package css

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2"
	tdcss "github.com/tdewolff/parse/v2/css"
)

// Hard caps on stylesheet shape to bound memory and CPU on hostile
// input. Real authored stylesheets are tiny compared to these limits;
// the caller (svg pkg) also caps total source bytes upstream.
const (
	maxRules         = 4096
	maxDeclsPerRule  = 256
	maxSelectorsRule = 256
	maxKeyframesDefs = 256
	maxStopsPerKF    = 256
	maxDeclsPerStop  = 256
)

// ParseStylesheet parses a CSS stylesheet and returns the list of
// rules. Rules whose selector list ends up empty (every group was
// rejected) are dropped, as are rules with no declarations. Use
// ParseFull when @keyframes definitions are also needed.
func ParseStylesheet(src string, opts ParseOptions) []Rule {
	return ParseFull(src, opts).Rules
}

// parseCtx threads the running state of ParseFull through the
// per-grammar-event helpers. Splitting the loop body keeps each helper
// linear and brings ParseFull's cyclomatic complexity under the
// project cap.
type parseCtx struct {
	out            Stylesheet
	curKF          KeyframesDef
	pendingOffsets []float32
	pendingDecls   []Decl
	current        Rule
	nextOrder      int
	// Media context: skipMedia drops every nested ruleset until the
	// matching EndAtRule. mediaDepth lets nested @-rules (e.g.
	// @keyframes inside @media) resume normal processing once the
	// outer block ends.
	mediaDepth int
	inRule     bool
	// Keyframes context: when inKeyframes, BeginRuleset is a keyframe
	// stop selector ("0%", "from", ...) rather than a CSS selector
	// list.
	inKeyframes bool
	inStop      bool
	skipMedia   bool
}

// ParseFull parses a CSS stylesheet, returning both top-level rules
// and any @keyframes blocks. Rules / keyframes whose body is empty
// are dropped. opts toggles environment-dependent rules; the only
// supported query is `@media (prefers-reduced-motion: reduce)` which
// is kept when opts.PrefersReducedMotion is true and dropped
// otherwise. Any other media query drops its block.
func ParseFull(src string, opts ParseOptions) Stylesheet {
	if strings.TrimSpace(src) == "" {
		return Stylesheet{}
	}
	src = stripLineComments(src)
	p := tdcss.NewParser(parse.NewInput(strings.NewReader(src)), false)
	var c parseCtx
	lastErrOff := -1
	for {
		gt, tt, data := p.Next()
		if gt == tdcss.ErrorGrammar {
			// tdewolff emits ErrorGrammar both at EOF and on
			// recoverable parse errors (e.g. a stray ":" mid-rule
			// inside an svg-spinners stylesheet). Skip recoverable
			// errors so the surrounding rule's good declarations
			// still reach the cascade; stop only at EOF. Guard
			// against a non-advancing parser by breaking when the
			// offset doesn't move between successive errors.
			if p.Err() == io.EOF {
				break
			}
			off := p.Offset()
			if off == lastErrOff {
				break
			}
			lastErrOff = off
			continue
		}
		if c.skipMedia {
			advanceSkippedMedia(gt, data, &c.mediaDepth, &c.skipMedia)
			continue
		}
		switch gt {
		case tdcss.BeginAtRuleGrammar:
			c.onBeginAtRule(data, p.Values(), opts)
		case tdcss.EndAtRuleGrammar:
			c.onEndAtRule()
		case tdcss.BeginRulesetGrammar:
			c.onBeginRuleset(p.Values())
		case tdcss.DeclarationGrammar, tdcss.CustomPropertyGrammar:
			c.onDeclaration(tt, data, p.Values())
		case tdcss.EndRulesetGrammar:
			c.onEndRuleset()
		}
	}
	return c.out
}

func (c *parseCtx) onBeginAtRule(
	data []byte, vals []tdcss.Token, opts ParseOptions,
) {
	if isMediaAtRule(data) {
		c.mediaDepth++
		if !mediaMatches(vals, opts) {
			c.skipMedia = true
		}
		return
	}
	if isKeyframesAtRule(data) {
		c.inKeyframes = true
		c.curKF = KeyframesDef{Name: keyframesName(vals)}
	}
}

func (c *parseCtx) onEndAtRule() {
	if c.mediaDepth > 0 {
		c.mediaDepth--
		return
	}
	if !c.inKeyframes {
		return
	}
	if c.curKF.Name != "" && len(c.curKF.Stops) > 0 &&
		len(c.out.Keyframes) < maxKeyframesDefs {
		sortKeyframeStops(c.curKF.Stops)
		c.out.Keyframes = append(c.out.Keyframes, c.curKF)
	}
	c.inKeyframes = false
	c.curKF = KeyframesDef{}
}

func (c *parseCtx) onBeginRuleset(vals []tdcss.Token) {
	if c.inKeyframes {
		offsets, ok := parseKeyframeSelectors(vals)
		if !ok {
			return
		}
		if len(offsets) > maxStopsPerKF {
			offsets = offsets[:maxStopsPerKF]
		}
		c.pendingOffsets = offsets
		c.pendingDecls = nil
		c.inStop = true
		return
	}
	sels := parseSelectorList(vals)
	if len(sels) > maxSelectorsRule {
		sels = sels[:maxSelectorsRule]
	}
	c.current = Rule{Selectors: sels, Source: c.nextOrder}
	c.nextOrder++
	c.inRule = true
}

func (c *parseCtx) onDeclaration(
	tt tdcss.TokenType, name []byte, vals []tdcss.Token,
) {
	if tt != tdcss.IdentToken && tt != tdcss.CustomPropertyNameToken {
		return
	}
	d, ok := parseDeclaration(name, vals)
	if !ok {
		return
	}
	switch {
	case c.inStop:
		if len(c.pendingDecls) < maxDeclsPerStop {
			c.pendingDecls = append(c.pendingDecls, d)
		}
	case c.inRule:
		if len(c.current.Decls) < maxDeclsPerRule {
			c.current.Decls = append(c.current.Decls, d)
		}
	}
}

func (c *parseCtx) onEndRuleset() {
	if c.inStop {
		if len(c.pendingDecls) > 0 &&
			len(c.curKF.Stops)+len(c.pendingOffsets) <= maxStopsPerKF {
			for _, off := range c.pendingOffsets {
				c.curKF.Stops = append(c.curKF.Stops, KeyframeStop{
					Offset: off,
					Decls:  c.pendingDecls,
				})
			}
		}
		c.pendingOffsets = nil
		c.pendingDecls = nil
		c.inStop = false
		return
	}
	if c.inRule && len(c.current.Decls) > 0 &&
		len(c.current.Selectors) > 0 &&
		len(c.out.Rules) < maxRules {
		c.out.Rules = append(c.out.Rules, c.current)
	}
	c.current = Rule{}
	c.inRule = false
}

// parseDeclaration converts a tdewolff declaration name + value
// token slice into a Decl. CustomPropertyNameToken (--name) becomes
// a CustomProp decl whose value is the raw text minus !important.
func parseDeclaration(name []byte, vals []tdcss.Token) (Decl, bool) {
	rawName := strings.TrimSpace(string(name))
	if rawName == "" {
		return Decl{}, false
	}
	custom := strings.HasPrefix(rawName, "--")
	lower := strings.ToLower(rawName)
	if !custom {
		lower = StripVendorPrefix(lower)
	}
	d := Decl{
		Name:       lower,
		CustomProp: custom,
	}
	for len(vals) > 0 && vals[len(vals)-1].TokenType == tdcss.WhitespaceToken {
		vals = vals[:len(vals)-1]
	}
	if len(vals) >= 2 {
		last := vals[len(vals)-1]
		prev := vals[len(vals)-2]
		if last.TokenType == tdcss.IdentToken &&
			bytes.EqualFold(last.Data, []byte("important")) &&
			prev.TokenType == tdcss.DelimToken &&
			len(prev.Data) == 1 && prev.Data[0] == '!' {
			d.Important = true
			vals = vals[:len(vals)-2]
			for len(vals) > 0 &&
				vals[len(vals)-1].TokenType == tdcss.WhitespaceToken {
				vals = vals[:len(vals)-1]
			}
		}
	}
	d.Value = strings.TrimSpace(joinTokens(vals))
	if d.Value == "" {
		return Decl{}, false
	}
	return d, true
}

// stripLineComments removes `//` line comments from CSS source —
// invalid per spec but commonly hand-shipped (e.g. pacman.svg from
// svg-spinners). A `//` is treated as the start of a comment when not
// inside a single- or double-quoted string AND when it is not part of
// a URL scheme like `http://` (i.e. the char before `//` is not `:`).
// Block comments `/* ... */` are left for the CSS tokenizer.
func stripLineComments(src string) string {
	if !strings.Contains(src, "//") {
		return src
	}
	var b strings.Builder
	b.Grow(len(src))
	var quote byte
	for i := 0; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				b.WriteByte(src[i+1])
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
			b.WriteByte(c)
			continue
		case '/':
			if i+1 < len(src) && src[i+1] == '/' {
				prev := byte(0)
				if i > 0 {
					prev = src[i-1]
				}
				if prev != ':' {
					for i < len(src) && src[i] != '\n' && src[i] != '\r' {
						i++
					}
					if i < len(src) {
						b.WriteByte(src[i])
					}
					continue
				}
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// vendorPrefixes lists the recognized vendor prefixes stripped from
// declaration names. Lowercase, leading-dash form.
var vendorPrefixes = [...]string{"-webkit-", "-moz-", "-ms-", "-o-"}

// StripVendorPrefix removes a leading vendor prefix from a lowercased
// CSS property name. So `-webkit-animation` becomes `animation`.
// Custom-property names ("--foo") must not be passed through here.
func StripVendorPrefix(name string) string {
	for _, p := range vendorPrefixes {
		if strings.HasPrefix(name, p) {
			return name[len(p):]
		}
	}
	return name
}

func joinTokens(toks []tdcss.Token) string {
	var b strings.Builder
	for _, t := range toks {
		b.Write(t.Data)
	}
	return b.String()
}

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
