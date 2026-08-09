package markdown

// highlight.go provides syntax tokenization for code blocks.
// Returns token spans only — palette application is done by the
// gui styling bridge.

import "strings"

func normalizeLanguageHint(language string) string {
	lower := strings.TrimSpace(strings.ToLower(language))
	if len(lower) == 0 {
		return ""
	}
	// Take first word only.
	if idx := strings.IndexAny(lower, " \t"); idx >= 0 {
		lower = lower[:idx]
	}
	switch lower {
	case "v", "vlang":
		return "v"
	case "js", "javascript", "node", "nodejs",
		"jsx", "mjs", "cjs":
		return "js"
	case "ts", "typescript", "tsx":
		return "ts"
	case "py", "python", "python3":
		return "py"
	case "json", "jsonc":
		return "json"
	case "go", "golang":
		return "go"
	case "rs", "rust":
		return "rust"
	case "c", "cpp", "c++", "cc", "cxx",
		"h", "hpp", "hxx":
		return "c"
	case "sh", "bash", "shell", "zsh", "fish":
		return "shell"
	case "html", "htm", "css", "xml",
		"svg", "xhtml":
		return "html"
	case "math", "latex", "tex":
		return "math"
	case "mermaid":
		return "mermaid"
	default:
		return lower
	}
}

// LangFromHint maps a language hint string to a CodeLanguage.
func langFromHint(language string) CodeLanguage {
	switch normalizeLanguageHint(language) {
	case "v":
		return langV
	case "js":
		return langJavaScript
	case "ts":
		return langTypeScript
	case "py":
		return langPython
	case "json":
		return langJSON
	case "go":
		return langGo
	case "rust":
		return langRust
	case "c":
		return langC
	case "shell":
		return langShell
	case "html":
		return langHTML
	default:
		return langGeneric
	}
}

// tokenizeCode tokenizes source code and returns token spans.
// Returns nil if code exceeds maxBytes.
//
//nolint:gocyclo // language token-type switch
func tokenizeCode(
	code string, lang CodeLanguage, maxBytes int,
) []codeToken {
	if len(code) == 0 || len(code) > maxBytes {
		return nil
	}
	tokens := make([]codeToken, 0, 128)
	pos := 0
	for pos < len(code) {
		if len(tokens) >= maxHighlightTokensPerBlock {
			appendTailToken(&tokens, code, pos)
			return tokens
		}
		startPos := pos
		ch := code[pos]

		switch {
		case isCodeWhitespace(ch):
			end := pos + 1
			for end < len(code) && isCodeWhitespace(code[end]) {
				if end-pos >= maxHighlightTokenBytes {
					appendTailToken(&tokens, code, pos)
					return tokens
				}
				end++
			}
			appendToken(&tokens, TokenPlain, pos, end)
			pos = end

		case hasLineCommentStart(code, pos, lang):
			end := pos + lineCommentPrefixLen(code, pos, lang)
			for end < len(code) && code[end] != '\n' {
				if end-pos >= maxHighlightTokenBytes {
					appendTailToken(&tokens, code, pos)
					return tokens
				}
				end++
			}
			appendToken(&tokens, TokenComment, pos, end)
			pos = end

		case hasBlockCommentStart(code, pos, lang):
			if lang == langHTML {
				end := pos + 4
				for end+2 < len(code) {
					if end-pos >= maxHighlightStringScanBytes ||
						end-pos >= maxHighlightTokenBytes {
						appendTailToken(&tokens, code, pos)
						return tokens
					}
					if code[end] == '-' && code[end+1] == '-' &&
						code[end+2] == '>' {
						end += 3
						break
					}
					end++
				}
				if end > len(code) {
					end = len(code)
				}
				appendToken(&tokens, TokenComment, pos, end)
				pos = end
			} else {
				end := pos + 2
				depth := 1
				for end < len(code) {
					if end-pos >= maxHighlightStringScanBytes ||
						end-pos >= maxHighlightTokenBytes {
						appendTailToken(&tokens, code, pos)
						return tokens
					}
					if blockCommentsNested(lang) &&
						end+1 < len(code) &&
						code[end] == '/' && code[end+1] == '*' {
						depth++
						if depth > maxHighlightCommentDepth {
							appendTailToken(&tokens, code, pos)
							return tokens
						}
						end += 2
						continue
					}
					if end+1 < len(code) &&
						code[end] == '*' && code[end+1] == '/' {
						depth--
						end += 2
						if depth == 0 {
							break
						}
						continue
					}
					end++
				}
				if end > len(code) {
					end = len(code)
				}
				appendToken(&tokens, TokenComment, pos, end)
				pos = end
			}

		case isStringDelim(ch, lang):
			end, ok := scanString(code, pos, lang)
			if !ok {
				appendTailToken(&tokens, code, pos)
				return tokens
			}
			appendToken(&tokens, TokenString, pos, end)
			pos = end

		case isNumberStart(code, pos):
			end, ok := scanNumber(code, pos)
			if !ok {
				appendTailToken(&tokens, code, pos)
				return tokens
			}
			appendToken(&tokens, TokenNumber, pos, end)
			pos = end

		case isIdentifierStart(ch, lang):
			end, ok := scanIdentifier(code, pos, lang)
			if !ok {
				appendTailToken(&tokens, code, pos)
				return tokens
			}
			ident := code[pos:end]
			kind := TokenPlain
			if isKeyword(ident, lang) {
				kind = TokenKeyword
			}
			appendToken(&tokens, kind, pos, end)
			pos = end

		case isOperatorChar(ch):
			end := pos + 1
			for end < len(code) && isOperatorChar(code[end]) {
				if end-pos >= maxHighlightTokenBytes {
					appendTailToken(&tokens, code, pos)
					return tokens
				}
				end++
			}
			appendToken(&tokens, TokenOperator, pos, end)
			pos = end

		default:
			appendToken(&tokens, TokenPlain, pos, pos+1)
			pos++
		}

		if pos <= startPos {
			appendToken(&tokens, TokenPlain,
				startPos, startPos+1)
			pos = startPos + 1
		}
	}
	return tokens
}

func appendTailToken(
	tokens *[]codeToken, code string, pos int,
) {
	if pos < len(code) {
		appendToken(tokens, TokenPlain, pos, len(code))
	}
}

func appendToken(
	tokens *[]codeToken,
	kind CodeTokenKind, start, end int,
) {
	if start == end {
		return
	}
	n := len(*tokens)
	if n > 0 && (*tokens)[n-1].Kind == kind &&
		(*tokens)[n-1].End == start {
		(*tokens)[n-1].End = end
		return
	}
	*tokens = append(*tokens, codeToken{
		Kind: kind, Start: start, End: end,
	})
}

func isIdentifierStart(ch byte, lang CodeLanguage) bool {
	if (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') || ch == '_' {
		return true
	}
	if (lang == langJavaScript ||
		lang == langTypeScript ||
		lang == langGeneric) && ch == '$' {
		return true
	}
	return lang == langHTML && ch == '-'
}

func isIdentifierContinue(ch byte, lang CodeLanguage) bool {
	if isIdentifierStart(ch, lang) {
		return true
	}
	if ch >= '0' && ch <= '9' {
		return true
	}
	return lang == langHTML && ch == '-'
}

func scanIdentifier(
	code string, pos int, lang CodeLanguage,
) (int, bool) {
	end := pos + 1
	for end < len(code) && isIdentifierContinue(code[end], lang) {
		if end-pos >= maxHighlightIdentifierBytes ||
			end-pos >= maxHighlightTokenBytes {
			return 0, false
		}
		end++
	}
	return end, true
}

func isNumberStart(code string, pos int) bool {
	if code[pos] >= '0' && code[pos] <= '9' {
		return true
	}
	return code[pos] == '.' &&
		pos+1 < len(code) &&
		code[pos+1] >= '0' && code[pos+1] <= '9'
}

func scanNumber(code string, pos int) (int, bool) {
	end := pos
	seenExp := false
	for end < len(code) {
		ch := code[end]
		isNum := (ch >= '0' && ch <= '9') ||
			ch == '.' || ch == '_'
		isBase := (ch >= 'a' && ch <= 'f') ||
			(ch >= 'A' && ch <= 'F') ||
			ch == 'x' || ch == 'X' ||
			ch == 'b' || ch == 'B' ||
			ch == 'o' || ch == 'O'
		isExp := (ch == 'e' || ch == 'E') && !seenExp
		if isExp {
			seenExp = true
			end++
			if end < len(code) &&
				(code[end] == '+' || code[end] == '-') {
				end++
			}
			continue
		}
		if isNum || isBase {
			end++
			if end-pos >= maxHighlightNumberBytes ||
				end-pos >= maxHighlightTokenBytes {
				return 0, false
			}
			continue
		}
		break
	}
	if end == pos {
		return 0, false
	}
	return end, true
}

func isStringDelim(ch byte, lang CodeLanguage) bool {
	switch lang {
	case langJSON:
		return ch == '"'
	case langPython:
		return ch == '"' || ch == '\''
	case langJavaScript, langTypeScript:
		return ch == '"' || ch == '\'' || ch == '`'
	case langGo:
		return ch == '"' || ch == '\'' || ch == '`'
	case langRust, langC:
		return ch == '"' || ch == '\''
	case langShell:
		return ch == '"' || ch == '\''
	case langHTML:
		return ch == '"' || ch == '\''
	default:
		return ch == '"' || ch == '\'' || ch == '`'
	}
}

func scanString(
	code string, pos int, lang CodeLanguage,
) (int, bool) {
	quote := code[pos]
	// Python triple-quote.
	if lang == langPython && pos+2 < len(code) &&
		code[pos+1] == quote && code[pos+2] == quote {
		end := pos + 3
		for end+2 < len(code) {
			if end-pos >= maxHighlightStringScanBytes ||
				end-pos >= maxHighlightTokenBytes {
				return 0, false
			}
			if code[end] == quote &&
				code[end+1] == quote &&
				code[end+2] == quote {
				return end + 3, true
			}
			end++
		}
		return len(code), true
	}
	end := pos + 1
	for end < len(code) {
		if end-pos >= maxHighlightStringScanBytes ||
			end-pos >= maxHighlightTokenBytes {
			return 0, false
		}
		ch := code[end]
		if ch == '\\' {
			if end+1 < len(code) {
				end += 2
				continue
			}
			end++
			break
		}
		if ch == quote {
			end++
			break
		}
		end++
	}
	return end, true
}

func hasLineCommentStart(
	code string, pos int, lang CodeLanguage,
) bool {
	if pos+1 < len(code) && code[pos] == '/' &&
		code[pos+1] == '/' {
		switch lang {
		case langGeneric, langV, langJavaScript,
			langTypeScript, langGo, langRust, langC:
			return true
		}
	}
	if code[pos] == '#' {
		switch lang {
		case langGeneric, langPython, langShell:
			return true
		}
	}
	return false
}

func lineCommentPrefixLen(
	code string, pos int, lang CodeLanguage,
) int {
	if pos+1 < len(code) && code[pos] == '/' &&
		code[pos+1] == '/' {
		switch lang {
		case langGeneric, langV, langJavaScript,
			langTypeScript, langGo, langRust, langC:
			return 2
		}
	}
	return 1
}

func hasBlockCommentStart(
	code string, pos int, lang CodeLanguage,
) bool {
	if pos+1 < len(code) && code[pos] == '/' &&
		code[pos+1] == '*' {
		switch lang {
		case langGeneric, langV, langJavaScript,
			langTypeScript, langGo, langRust, langC:
			return true
		}
	}
	if pos+3 < len(code) && code[pos] == '<' &&
		code[pos+1] == '!' && code[pos+2] == '-' &&
		code[pos+3] == '-' && lang == langHTML {
		return true
	}
	return false
}

func blockCommentsNested(lang CodeLanguage) bool {
	return lang == langV || lang == langRust ||
		lang == langGeneric
}

// Keyword sets per language.
var (
	kwV = toSet([]string{
		"as", "asm", "assert", "atomic", "break", "const",
		"continue", "defer", "else", "enum", "false", "fn",
		"for", "global", "go", "goto", "if", "import", "in",
		"interface", "is", "lock", "match", "module", "mut",
		"none", "or", "pub", "return", "rlock", "select",
		"shared", "sizeof", "spawn", "static", "struct",
		"true", "type", "typeof", "union", "unsafe",
		"volatile",
	})
	kwJS = toSet([]string{
		"async", "await", "break", "case", "catch", "class",
		"const", "continue", "debugger", "default", "delete",
		"do", "else", "export", "extends", "false", "finally",
		"for", "function", "if", "import", "in", "instanceof",
		"let", "new", "null", "return", "super", "switch",
		"this", "throw", "true", "try", "typeof", "var",
		"void", "while", "with", "yield",
	})
	kwTS = toSet([]string{
		"abstract", "any", "as", "asserts", "async", "await",
		"bigint", "boolean", "break", "case", "catch", "class",
		"const", "constructor", "continue", "debugger",
		"declare", "default", "delete", "do", "else", "enum",
		"export", "extends", "false", "finally", "for", "from",
		"function", "get", "if", "implements", "import", "in",
		"infer", "instanceof", "interface", "is", "keyof",
		"let", "module", "namespace", "never", "new", "null",
		"number", "object", "package", "private", "protected",
		"public", "readonly", "return", "set", "static",
		"string", "super", "switch", "symbol", "this", "throw",
		"true", "try", "type", "typeof", "undefined", "unique",
		"unknown", "var", "void", "while", "with", "yield",
	})
	kwPy = toSet([]string{
		"False", "None", "True", "and", "as", "assert",
		"async", "await", "break", "class", "continue", "def",
		"del", "elif", "else", "except", "finally", "for",
		"from", "global", "if", "import", "in", "is", "lambda",
		"nonlocal", "not", "or", "pass", "raise", "return",
		"try", "while", "with", "yield",
	})
	kwGo = toSet([]string{
		"break", "case", "chan", "const", "continue", "default",
		"defer", "else", "fallthrough", "for", "func", "go",
		"goto", "if", "import", "interface", "map", "package",
		"range", "return", "select", "struct", "switch", "type",
		"var", "true", "false", "nil", "iota", "append", "cap",
		"close", "copy", "delete", "len", "make", "new",
		"panic", "print", "println", "recover", "error",
		"string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "complex64", "complex128",
		"bool", "byte", "rune", "any",
	})
	kwRust = toSet([]string{
		"as", "async", "await", "break", "const", "continue",
		"crate", "dyn", "else", "enum", "extern", "false", "fn",
		"for", "if", "impl", "in", "let", "loop", "match",
		"mod", "move", "mut", "pub", "ref", "return", "self",
		"Self", "static", "struct", "super", "trait", "true",
		"type", "unsafe", "use", "where", "while", "yield",
		"Box", "Option", "Result", "Some", "None", "Ok", "Err",
		"Vec", "String", "str", "i8", "i16", "i32", "i64",
		"i128", "isize", "u8", "u16", "u32", "u64", "u128",
		"usize", "f32", "f64", "bool", "char", "println",
		"eprintln", "format", "panic", "todo", "unimplemented",
		"unreachable", "macro_rules",
	})
	kwC = toSet([]string{
		"auto", "break", "case", "char", "const", "continue",
		"default", "do", "double", "else", "enum", "extern",
		"float", "for", "goto", "if", "inline", "int", "long",
		"register", "restrict", "return", "short", "signed",
		"sizeof", "static", "struct", "switch", "typedef",
		"union", "unsigned", "void", "volatile", "while",
		"_Bool", "_Complex", "_Imaginary", "bool", "true",
		"false", "NULL", "nullptr", "class", "namespace",
		"template", "typename", "public", "private",
		"protected", "virtual", "override", "final", "new",
		"delete", "this", "throw", "try", "catch", "using",
		"constexpr", "noexcept", "decltype", "static_cast",
		"dynamic_cast", "reinterpret_cast", "const_cast",
		"operator", "friend", "mutable", "explicit", "export",
		"concept", "requires", "co_await", "co_return",
		"co_yield", "include", "define", "ifdef", "ifndef",
		"endif", "pragma", "std", "cout", "cin", "endl",
		"string", "vector", "map", "set", "pair", "unique_ptr",
		"shared_ptr", "weak_ptr", "size_t", "uint8_t",
		"uint16_t", "uint32_t", "uint64_t", "int8_t", "int16_t",
		"int32_t", "int64_t",
	})
	kwShell = toSet([]string{
		"if", "then", "else", "elif", "fi", "for", "while",
		"until", "do", "done", "case", "esac", "in", "function",
		"select", "time", "coproc", "return", "exit", "break",
		"continue", "shift", "export", "readonly", "declare",
		"local", "typeset", "unset", "eval", "exec", "source",
		"set", "true", "false", "echo", "printf", "read",
		"test", "cd", "pwd", "ls", "cp", "mv", "rm", "mkdir",
		"rmdir", "chmod", "chown", "grep", "sed", "awk", "find",
		"xargs", "cat", "head", "tail", "sort", "uniq", "wc",
		"tr", "cut", "tee", "curl", "wget", "tar", "gzip",
		"git", "docker", "sudo", "apt", "yum", "brew", "pip",
		"npm", "yarn",
	})
	kwHTML = toSet([]string{
		"html", "head", "body", "div", "span", "p", "a", "img",
		"ul", "ol", "li", "table", "tr", "td", "th", "form",
		"input", "button", "select", "option", "textarea",
		"label", "h1", "h2", "h3", "h4", "h5", "h6", "header",
		"footer", "nav", "main", "section", "article", "aside",
		"script", "style", "link", "meta", "title", "br", "hr",
		"pre", "code", "strong", "em", "blockquote", "iframe",
		"canvas", "svg", "video", "audio", "source", "template",
		"slot", "details", "summary", "dialog", "color",
		"background", "margin", "padding", "border", "display",
		"position", "width", "height", "font", "text", "align",
		"flex", "grid", "float", "clear", "overflow", "opacity",
		"transform", "transition", "animation", "cursor", "none",
		"block", "inline", "absolute", "relative", "fixed",
		"sticky", "static", "inherit", "initial", "unset",
		"auto", "important", "media", "keyframes", "import",
		"var", "calc", "min", "max", "clamp", "rgb", "rgba",
		"hsl", "hsla",
	})
	kwJSON = toSet([]string{"true", "false", "null"})
)

func toSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

func isKeyword(ident string, lang CodeLanguage) bool {
	switch lang {
	case langV:
		return kwV[ident]
	case langJavaScript:
		return kwJS[ident]
	case langTypeScript:
		return kwTS[ident]
	case langPython:
		return kwPy[ident]
	case langGo:
		return kwGo[ident]
	case langRust:
		return kwRust[ident]
	case langC:
		return kwC[ident]
	case langShell:
		return kwShell[ident]
	case langHTML:
		return kwHTML[ident]
	case langJSON:
		return kwJSON[ident]
	}
	return false
}

func isOperatorChar(ch byte) bool {
	switch ch {
	case '+', '-', '*', '/', '%', '=', '&', '|', '^', '!',
		'<', '>', '?', ':', '.', ',', ';', '(', ')', '[',
		']', '{', '}', '~':
		return true
	}
	return false
}

func isCodeWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' ||
		ch == '\n' || ch == '\r'
}
