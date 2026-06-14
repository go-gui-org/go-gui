package highlight

import (
	"sync"

	"github.com/alecthomas/chroma/v2"
)

// chromaHighlighter is the default Highlighter implementation.
type chromaHighlighter struct{}

// New returns a Highlighter backed by chroma. The returned value is
// stateless and safe for concurrent use across windows.
func New() Highlighter { return chromaHighlighter{} }

var defaultHL = sync.OnceValue(New)

// Default returns a process-wide singleton Highlighter. Use this
// unless a custom Highlighter is needed.
func Default() Highlighter { return defaultHL() }

// Caps guard against pathological input: chroma uses backtracking
// regex (regexp2), so adversarial source could trigger quadratic
// behavior. Beyond these limits, fall back to a single plain token
// rather than risk a long stall or unbounded allocation.
const (
	maxTokenizeBytes  = 256 * 1024
	maxTokenizeTokens = 100_000
)

// Tokenize implements Highlighter.
func (chromaHighlighter) Tokenize(lang, src string) []Token {
	if src == "" {
		return nil
	}
	if len(src) > maxTokenizeBytes {
		return []Token{{Kind: KindPlain, Text: src}}
	}
	lx := lookupLexer(lang)
	if lx == nil {
		return []Token{{Kind: KindPlain, Text: src}}
	}
	it, err := lx.Tokenise(nil, src)
	if err != nil || it == nil {
		return []Token{{Kind: KindPlain, Text: src}}
	}
	// Pull via the iterator function to avoid Tokens()'s
	// intermediate slice allocation.
	var out []Token
	for t := it(); t != chroma.EOF; t = it() {
		if t.Value == "" {
			continue
		}
		out = append(out, Token{
			Kind: mapKind(t.Type),
			Text: t.Value,
		})
		if len(out) >= maxTokenizeTokens {
			break
		}
	}
	if len(out) == 0 {
		return []Token{{Kind: KindPlain, Text: src}}
	}
	return out
}

// mapKind reduces chroma's rich token taxonomy to the small Kind
// enum used by callers. Specific overrides are checked before
// category fallbacks so that e.g. KeywordType becomes KindType
// rather than KindKeyword.
func mapKind(t chroma.TokenType) Kind {
	switch t {
	case chroma.KeywordType:
		return KindType
	case chroma.NameFunction, chroma.NameFunctionMagic:
		return KindFunction
	case chroma.NameClass, chroma.NameNamespace:
		return KindType
	case chroma.NameBuiltin, chroma.NameBuiltinPseudo:
		return KindBuiltin
	}
	// LiteralString and LiteralNumber share chroma's top-level
	// Literal category (3000), so InCategory can't distinguish
	// them — use InSubCategory for Literal children.
	switch {
	case t.InSubCategory(chroma.LiteralString):
		return KindString
	case t.InSubCategory(chroma.LiteralNumber):
		return KindNumber
	case t.InCategory(chroma.Keyword):
		return KindKeyword
	case t.InCategory(chroma.Comment):
		return KindComment
	case t.InCategory(chroma.Operator):
		return KindOperator
	case t.InCategory(chroma.Punctuation):
		return KindPunctuation
	}
	return KindPlain
}
