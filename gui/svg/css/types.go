// Package css implements the SVG-CSS subset used by the SVG renderer:
// compound selectors with descendant/child combinators,
// :nth-child(an+b), :root, custom properties, and an Origin-aware
// cascade.
package css

// Origin tags a declaration's source layer in the cascade. Values
// run lowest precedence to highest.
type Origin uint8

// Origin constants.
const (
	OriginPresAttr Origin = iota
	OriginRule
	OriginInline

	// numOrigins is the count of origin tiers; cascadeLayer uses it
	// to lift !important decls into a parallel high-precedence band.
	numOrigins = 3
)

// Specificity is the CSS specificity tuple (a, b, c) where a counts
// ID selectors, b counts class+pseudo selectors, and c counts type
// selectors. Inline style adds an extra "d" tier handled at the
// cascade layer via Origin.
type Specificity [3]uint16

// Less reports whether s sorts before other under the cascade
// comparison: lower specificity loses.
func (s Specificity) Less(o Specificity) bool {
	for i := range 3 {
		if s[i] != o[i] {
			return s[i] < o[i]
		}
	}
	return false
}

// Add returns the sum of two specificities (combinator chains add
// per-compound specificities).
func (s Specificity) Add(o Specificity) Specificity {
	return Specificity{s[0] + o[0], s[1] + o[1], s[2] + o[2]}
}

// NthFormula encodes :nth-child(an+b) for evaluation against a
// 1-based child index.
type NthFormula struct {
	A, B int
}

// Matches reports whether the formula matches a 1-based child index.
// A==0 selects exact index B; otherwise (idx - B) must be a
// non-negative multiple of A (with sign of A factored in).
func (f NthFormula) Matches(idx int) bool {
	if f.A == 0 {
		return idx == f.B
	}
	d := idx - f.B
	if f.A > 0 {
		return d >= 0 && d%f.A == 0
	}
	return d <= 0 && (-d)%(-f.A) == 0
}

// AttrOp identifies the operator in an attribute selector.
type AttrOp uint8

// AttrOp constants. Values mirror the CSS Selectors L4 operator set.
const (
	AttrOpExists    AttrOp = iota // [name]
	AttrOpEqual                   // [name=value]
	AttrOpInclude                 // [name~=value] (whitespace-separated word)
	AttrOpDashMatch               // [name|=value] (value or value-prefixed)
	AttrOpPrefix                  // [name^=value]
	AttrOpSuffix                  // [name$=value]
	AttrOpSubstring               // [name*=value]
)

// AttrSel is one [name op value] attribute constraint on a compound
// selector. Name is lowercased. Op == AttrOpExists ignores Value.
type AttrSel struct {
	Name  string
	Value string
	Op    AttrOp
}

// Compound is a compound selector: an optional tag, an optional id,
// zero or more classes, attribute constraints, and pseudo-class
// constraints. Tag == "" matches any element when no other constraints
// are present; "*" is the explicit universal form.
type Compound struct {
	NthChild *NthFormula
	// Not is an inner compound for :not(inner). Single-compound scope:
	// :not(.a, .b) and nested :not(:not(...)) are not supported.
	Not         *Compound
	Tag         string
	ID          string
	Classes     []string
	Attrs       []AttrSel
	Spec        Specificity
	Root        bool
	HoverPseudo bool
	FocusPseudo bool
}

// Combinator joins two compound selectors in a complex selector.
type Combinator byte

// Combinator constants. CombStart marks the leftmost compound in a
// complex selector (no left-hand neighbor). The single-byte values
// for descendant/child/adjacent/general-sibling are the same as the
// CSS source-form delim characters so combinatorFromDelim can map
// directly.
const (
	CombStart          Combinator = 0
	CombDescendant     Combinator = ' '
	CombChild          Combinator = '>'
	CombAdjacent       Combinator = '+'
	CombGeneralSibling Combinator = '~'
)

// SelectorPart is one compound in a complex selector together with
// the combinator that joins it to the previous part.
type SelectorPart struct {
	Compound   Compound
	Combinator Combinator
}

// ComplexSelector is a chain of compound selectors joined by
// combinators. Parts[len-1] is the rightmost compound (the one that
// must match the candidate element); preceding parts must satisfy
// the combinator chain against ancestors.
type ComplexSelector struct {
	Parts []SelectorPart
	Spec  Specificity
}

// Decl is one CSS declaration. Name is lowercased; Value is the raw
// declaration text minus trailing whitespace and the optional
// "!important" suffix. CustomProp marks "--name" custom properties
// so the cascade can route them into the variable map.
type Decl struct {
	Name       string
	Value      string
	Important  bool
	CustomProp bool
}

// Rule is one ruleset: a list of complex selectors that share a
// declaration block. Source is the rule's index in source order;
// the cascade uses it as a tiebreaker after specificity.
type Rule struct {
	Selectors []ComplexSelector
	Decls     []Decl
	Source    int
}

// KeyframeStop is one keyframe in a @keyframes timeline. Offset is
// the resolved [0,1] position (0% → 0, from → 0, 50% → 0.5, to/100%
// → 1). Decls are the property writes for that stop.
type KeyframeStop struct {
	Decls  []Decl
	Offset float32
}

// KeyframesDef is one parsed @keyframes block. Stops are sorted
// ascending by Offset; duplicate offsets keep last-written-wins
// semantics by source order.
type KeyframesDef struct {
	Name  string
	Stops []KeyframeStop
}

// Stylesheet is the complete parsed CSS source: top-level rules
// plus any @keyframes definitions. Lookup helpers index by name
// (case-insensitive on the @keyframes side).
type Stylesheet struct {
	Rules     []Rule
	Keyframes []KeyframesDef
}

// ParseOptions are environment toggles consulted while parsing the
// stylesheet. PrefersReducedMotion is the snapshot fed to
// `@media (prefers-reduced-motion: reduce)` evaluation: when true,
// rules inside that block are kept; when false, dropped. All other
// media queries are dropped unconditionally.
type ParseOptions struct {
	PrefersReducedMotion bool
}

// MatchState carries the runtime UI state pseudo-classes consult.
// Hover and Focus mirror the user-agent's element-state bits and are
// toggled by the renderer's mouse / focus dispatcher. Zero value =
// neutral (no element hovered or focused).
type MatchState struct {
	Hover bool
	Focus bool
}

// ElementInfo is the per-element identity the matcher needs.
// Callers populate Index (1-based child position in the parent)
// and IsRoot (true for the root <svg>) for pseudo-class evaluation.
// Attrs feeds attribute selectors; nil map disables attr matching
// for the element. State carries hover/focus flags.
type ElementInfo struct {
	Attrs   map[string]string
	Tag     string
	ID      string
	Classes []string
	Index   int
	State   MatchState
	IsRoot  bool
}
