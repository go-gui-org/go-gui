package shaderport

import "strings"

// tokKind classifies a token. The translator rewrites a token stream
// and writes back every token it does not change, spaces and comments
// included, so the output keeps the author's layout for review.
type tokKind uint8

const (
	tkSpace     tokKind = iota // spaces, tabs and newlines
	tkComment                  // // line or /* block */ comment
	tkIdent                    // identifier or keyword
	tkNumber                   // numeric literal, suffix included
	tkPunct                    // operator or punctuation
	tkDirective                // a whole preprocessor line, # to end of line
)

type token struct {
	kind tokKind
	text string
	line int // 1-based
	col  int // 1-based, in bytes
}

// trivia reports whether a token carries no meaning: a space or a
// comment.
func (t token) trivia() bool { return t.kind == tkSpace || t.kind == tkComment }

// is reports whether t is the punctuation or identifier s.
func (t token) is(s string) bool {
	return (t.kind == tkPunct || t.kind == tkIdent) && t.text == s
}

// puncts lists the multi-byte operators, longest first, so the lexer
// takes "<<=" before "<<" before "<".
var puncts = []string{
	"<<=", ">>=",
	"++", "--", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"==", "!=", "<=", ">=", "&&", "||", "^^", "<<", ">>",
}

// lex splits src into tokens. It never fails: a byte it does not know
// becomes a one-byte punctuation token, and the structural pass reports
// it if it matters.
func lex(src string) []token { return lexAt(src, 1, 1) }

// lexAt is lex for text that starts at line:col of the source, such as
// a #define body, so positions in errors point into the source.
func lexAt(src string, line, col int) []token {
	var toks []token
	atLineStart := true // only spaces since the last newline
	for i := 0; i < len(src); {
		kind, end := scan(src, i, atLineStart)
		text := src[i:end]
		toks = append(toks, token{kind: kind, text: text, line: line, col: col})
		// Move the position over the text.
		for j := range len(text) {
			if text[j] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		switch kind {
		case tkSpace:
			// Spaces keep the line-start state, unless they end a line.
			atLineStart = atLineStart || strings.Contains(text, "\n")
		default:
			atLineStart = false
		}
		i = end
	}
	return toks
}

// scan returns the kind and the end of the token at src[i]. A "#" is a
// directive only at the start of a line.
func scan(src string, i int, atLineStart bool) (tokKind, int) {
	c := src[i]
	switch {
	case strings.IndexByte(spaceBytes, c) >= 0:
		for i < len(src) && strings.IndexByte(spaceBytes, src[i]) >= 0 {
			i++
		}
		return tkSpace, i
	case strings.HasPrefix(src[i:], "//"):
		if end := strings.IndexByte(src[i:], '\n'); end >= 0 {
			return tkComment, i + end
		}
		return tkComment, len(src)
	case strings.HasPrefix(src[i:], "/*"):
		if end := strings.Index(src[i+2:], "*/"); end >= 0 {
			return tkComment, i + 2 + end + 2
		}
		return tkComment, len(src) // unterminated: runs to the end, as in GLSL
	case c == '#' && atLineStart:
		return tkDirective, scanDirective(src, i)
	case isIdentStart(c):
		for i < len(src) && isIdentByte(src[i]) {
			i++
		}
		return tkIdent, i
	case isDigit(c) || (c == '.' && i+1 < len(src) && isDigit(src[i+1])):
		return tkNumber, scanNumber(src, i)
	}
	for _, p := range puncts {
		if strings.HasPrefix(src[i:], p) {
			return tkPunct, i + len(p)
		}
	}
	return tkPunct, i + 1
}

const spaceBytes = " \t\r\n\f\v"

// scanDirective returns the end of the directive at src[i]: the end of
// the line. A backslash before the newline continues it onto the next
// line.
func scanDirective(src string, i int) int {
	for i < len(src) && src[i] != '\n' {
		switch {
		case strings.HasPrefix(src[i:], "\\\n"):
			i += 2
		case strings.HasPrefix(src[i:], "\\\r\n"):
			i += 3
		default:
			i++
		}
	}
	return i
}

// scanNumber returns the end of the numeric literal at src[i]: digits,
// a hex prefix, a fraction, an exponent with its sign, and a u/U/f/F
// suffix.
func scanNumber(src string, i int) int {
	if strings.HasPrefix(src[i:], "0x") || strings.HasPrefix(src[i:], "0X") {
		i += 2
		for i < len(src) && isHexDigit(src[i]) {
			i++
		}
	} else {
		for i < len(src) && (isDigit(src[i]) || src[i] == '.') {
			i++
		}
		if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
			j := i + 1
			if j < len(src) && (src[j] == '+' || src[j] == '-') {
				j++
			}
			if j < len(src) && isDigit(src[j]) {
				i = j
				for i < len(src) && isDigit(src[i]) {
					i++
				}
			}
		}
	}
	if i < len(src) && strings.IndexByte("uUfF", src[i]) >= 0 {
		i++
	}
	return i
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentByte(c byte) bool { return isIdentStart(c) || isDigit(c) }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
