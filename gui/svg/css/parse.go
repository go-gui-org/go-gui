package css

import (
	"bytes"
	"io"
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
