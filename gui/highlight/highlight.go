// Package highlight provides language-agnostic syntax highlighting
// for go-gui widgets. Backed by chroma; curated lexer subset keeps
// binary cost bounded.
package highlight

// Kind classifies a token for color mapping. Callers map Kind to
// their own palette; this package has no knowledge of gui types.
type Kind uint8

// Kind values — the classes a Token may belong to.
const (
	KindPlain Kind = iota
	KindKeyword
	KindString
	KindNumber
	KindComment
	KindOperator
	KindPunctuation
	KindType
	KindFunction
	KindBuiltin
)

// Token is a contiguous run of source text with one Kind.
type Token struct {
	Text string
	Kind Kind
}

// Highlighter tokenizes source into a slice of Tokens. Implementations
// must be safe for concurrent use.
type Highlighter interface {
	// Tokenize returns tokens for src in the given language. If lang
	// is empty or unknown, the implementation may fall back to
	// content analysis or return a single KindPlain token.
	Tokenize(lang, src string) []Token
}
