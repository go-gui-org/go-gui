// Package shaderport translates a Shadertoy GLSL shader into the two
// bodies of a gui.Shader: GLSL for the GL, web and Android backends and
// MSL for the Metal and iOS backends (#573).
//
// It is a token-stream rewriter, not a compiler. It splits the source
// into program-scope items (functions, structs, constants, directives)
// and rewrites tokens in place, so the output keeps the author's
// layout, comments and macros and reads as a reviewable diff. It needs
// no type checker: the GLSL ES 3.00 that Shadertoy accepts has no
// implicit conversions, and MSL accepts the rest of GLSL's mixed
// vector and scalar calls as they are. What it cannot translate it
// rejects with a named error and a source position, never with
// silent output.
package shaderport

import (
	"slices"
	"strings"
)

// Result holds the translated shader, one field per gui.Shader field.
type Result struct {
	Metal      string // MSL body, spliced inside fs_main
	MetalDecls string // MSL program-scope declarations
	GLSL       string // GLSL body, spliced inside main
	GLSLDecls  string // GLSL program-scope declarations
	Animate    bool   // the shader reads iTime, iTimeDelta or iFrame
}

// fragCoordExpr rebuilds Shadertoy's fragCoord from go-gui's uv. uv
// spans -1..1 across the container with y growing down
// (gui/backend/internal/gpu/vertex.go). fragCoord is in pixels with y
// growing up from the bottom edge, and iResolution is the container's
// size in the same pixels, so the two scale together.
const fragCoordExpr = "(%[1]s.x * 0.5 + 0.5) * iResolution.x, " +
	"(0.5 - %[1]s.y * 0.5) * iResolution.y"

// Port translates src. src is either a Shadertoy shader with a
// mainImage(out vec4, in vec2) function, or a go-gui body that declares
// frag_color (see gui.Shader); a body is translated to MSL as is.
func Port(src string) (*Result, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	p := &porter{toks: lex(src)}
	if p.isBareBody() {
		return p.portBody()
	}
	if err := p.parse(); err != nil {
		return nil, err
	}
	if err := p.analyze(); err != nil {
		return nil, err
	}
	p.collectVarTypes()
	return p.portShadertoy()
}

type itemKind uint8

const (
	itemDirective itemKind = iota
	itemStruct
	itemFunc  // function definition
	itemProto // function prototype
	itemConst // const global
	itemPrecision
)

// item is one program-scope declaration: tokens [start, end).
type item struct {
	kind   itemKind
	start  int
	end    int
	name   string // function or struct name
	lparen int    // function: index of the parameter list's "("
	rparen int    // function: index of its ")"
}

type porter struct {
	toks  []token
	items []item
	// funcs maps a function name to the built-ins it reads, directly
	// or through the functions and macros it uses (builtinBit bits).
	// Overloads share one entry: every overload gets the union.
	funcs map[string]uint8
	// macros maps a #define name to the built-ins its body reads.
	macros map[string]uint8
	// structs maps a struct name to its field names; fields lists
	// every field name of every struct, for member access.
	structs map[string][]string
	fields  map[string]bool
	// topNames are the names declared at program scope: functions,
	// structs, constants and macros. Only these can collide with a
	// name the wrapper declares at program scope.
	topNames map[string]bool
	// varTypes maps a variable, parameter or field name to the types
	// it is declared with anywhere in the source. A name after "."
	// that is both a struct field and a stpq swizzle (s, t, st, ...)
	// is a field only when the object is declared with a struct type.
	varTypes map[string][]string
	usesMod  bool // MSL output calls _gg_mod
	anim     bool // the source reads a built-in that changes per frame
	bare     bool // a go-gui body, not a Shadertoy shader
}

// isBareBody reports whether the source is a go-gui body: it names
// frag_color and has no mainImage.
func (p *porter) isBareBody() bool {
	var frag, main bool
	for _, t := range p.toks {
		if t.kind == tkIdent {
			frag = frag || t.text == "frag_color"
			main = main || t.text == "mainImage"
		}
	}
	return frag && !main
}

// next returns the index of the first non-trivia token at or after i
// in ts, or len(ts).
func next(ts []token, i int) int {
	for i < len(ts) && ts[i].trivia() {
		i++
	}
	return i
}

// prev returns the index of the last non-trivia token before i in ts,
// or -1.
func prev(ts []token, i int) int {
	i--
	for i >= 0 && ts[i].trivia() {
		i--
	}
	return i
}

// match returns the index of the bracket that closes ts[open].
func match(ts []token, open int) (int, error) {
	depth := 0
	for i := open; i < len(ts); i++ {
		if ts[i].kind != tkPunct {
			continue
		}
		switch ts[i].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errAt(ts[open], ErrSyntax, "unclosed %q", ts[open].text)
}

// parse splits the token stream into program-scope items.
func (p *porter) parse() error {
	p.structs = map[string][]string{}
	p.fields = map[string]bool{}
	p.topNames = map[string]bool{}
	ts := p.toks
	for i := next(ts, 0); i < len(ts); i = next(ts, i) {
		if ts[i].kind == tkDirective {
			p.items = append(p.items, item{kind: itemDirective, start: i, end: i + 1})
			i++
			continue
		}
		it, err := p.parseItem(i)
		if err != nil {
			return err
		}
		p.items = append(p.items, it)
		i = it.end
	}
	return nil
}

// parseItem reads one declaration starting at ts[i]: up to a ";" at
// depth 0, or up to the "}" that closes a function body.
func (p *porter) parseItem(i int) (item, error) {
	ts := p.toks
	first := ts[i]
	if first.kind == tkPunct && first.text != "[" && first.text != "(" {
		return item{}, errAt(first, ErrSyntax, "unexpected %q at program scope", first.text)
	}
	// Find the end and the first top-level "(" and "{".
	lparen, lbrace := -1, -1
	end := -1
	for j := i; j < len(ts) && end < 0; j++ {
		t := ts[j]
		if t.kind != tkPunct {
			continue
		}
		switch t.text {
		case ";":
			end = j + 1
		case "(", "[":
			if t.text == "(" && lparen < 0 && lbrace < 0 {
				lparen = j
			}
			c, err := match(ts, j)
			if err != nil {
				return item{}, err
			}
			j = c
		case "{":
			lbrace = j
			c, err := match(ts, j)
			if err != nil {
				return item{}, err
			}
			j = c
			// A function body ends the item. A struct body is
			// followed by its ";".
			if first.text != "struct" {
				end = c + 1
			}
		case ")", "]", "}":
			return item{}, errAt(t, ErrSyntax, "unbalanced %q", t.text)
		}
	}
	if end < 0 {
		return item{}, errAt(first, ErrSyntax, "declaration has no closing ';' or '}'")
	}
	switch {
	case first.text == "struct":
		return p.parseStruct(i, end, lbrace)
	case first.text == "precision":
		return item{kind: itemPrecision, start: i, end: end}, nil
	case lbrace >= 0 && lparen >= 0 && lparen < lbrace:
		return p.parseFunc(itemFunc, i, end, lparen)
	case lparen >= 0 && p.isProto(i, end, lparen):
		return p.parseFunc(itemProto, i, end, lparen)
	case first.text == "const":
		p.constNames(i, end)
		return item{kind: itemConst, start: i, end: end}, nil
	}
	if r := rejectReason(first.text); r != "" {
		return item{}, errAt(first, ErrUnsupported, "%s", r)
	}
	switch first.text {
	case "in", "out", "varying", "attribute", "layout", "buffer":
		return item{}, errAt(first, ErrUnsupported,
			"%q globals are not supported: a shader reads only the go-gui built-ins", first.text)
	}
	return item{}, errAt(first, ErrMutableGlobal,
		"%q is not const; MSL has no mutable globals. Make it a local "+
			"of mainImage and pass it to the functions that need it",
		strings.TrimSpace(tokText(ts[i:end])))
}

// isProto reports whether ts[i:end] is `type name(params);`: nothing
// but names before the "(" and nothing but ";" after the ")".
func (p *porter) isProto(i, end, lparen int) bool {
	ts := p.toks
	for j := i; j < lparen; j++ {
		if !ts[j].trivia() && ts[j].kind != tkIdent {
			return false
		}
	}
	c, err := match(ts, lparen)
	return err == nil && next(ts, c+1) == end-1
}

func (p *porter) parseFunc(kind itemKind, i, end, lparen int) (item, error) {
	ts := p.toks
	n := prev(ts, lparen)
	if n < i || ts[n].kind != tkIdent {
		return item{}, errAt(ts[lparen], ErrSyntax, "function has no name")
	}
	rparen, err := match(ts, lparen)
	if err != nil {
		return item{}, err
	}
	p.topNames[ts[n].text] = true
	return item{kind: kind, start: i, end: end, name: ts[n].text,
		lparen: lparen, rparen: rparen}, nil
}

// parseStruct records a struct's name and field names. A field name is
// an identifier followed by ";", "," or "[" inside the braces.
func (p *porter) parseStruct(i, end, lbrace int) (item, error) {
	ts := p.toks
	n := next(ts, i+1)
	if n >= end || ts[n].kind != tkIdent || lbrace < 0 {
		return item{}, errAt(ts[i], ErrSyntax, "struct has no name or body")
	}
	name := ts[n].text
	rbrace, err := match(ts, lbrace)
	if err != nil {
		return item{}, err
	}
	if after := next(ts, rbrace+1); after != end-1 {
		return item{}, errAt(ts[after], ErrMutableGlobal,
			"struct %s declares a global variable; MSL has no mutable globals", name)
	}
	var fields []string
	for j := lbrace + 1; j < rbrace; j++ {
		if ts[j].kind != tkIdent {
			continue
		}
		if k := next(ts, j+1); k < rbrace && (ts[k].is(";") || ts[k].is(",") || ts[k].is("[")) {
			fields = append(fields, ts[j].text)
			p.fields[ts[j].text] = true
		}
	}
	p.structs[name] = fields
	p.topNames[name] = true
	return item{kind: itemStruct, start: i, end: end, name: name}, nil
}

// constNames records the names a const declaration declares: each
// identifier before a depth-0 "=" or "[".
func (p *porter) constNames(i, end int) {
	ts := p.toks
	depth := 0
	for j := i; j < end; j++ {
		t := ts[j]
		if t.kind != tkPunct {
			continue
		}
		switch t.text {
		case "(", "{":
			depth++
		case ")", "}":
			depth--
		case "=", "[":
			if depth == 0 {
				if n := prev(ts, j); n >= 0 && ts[n].kind == tkIdent {
					p.topNames[ts[n].text] = true
				}
			}
		}
	}
}

// collectVarTypes fills varTypes from every `Type name` pair in the
// source where Type is a GLSL type or a struct and name is not followed
// by "(" (that would be a function).
func (p *porter) collectVarTypes() {
	p.varTypes = map[string][]string{}
	ts := p.toks
	for i, t := range ts {
		if t.kind != tkIdent {
			continue
		}
		if _, isStruct := p.structs[t.text]; !isStruct && !isGLSLType(t.text) {
			continue
		}
		n := next(ts, i+1)
		if n >= len(ts) || ts[n].kind != tkIdent {
			continue
		}
		if f := next(ts, n+1); f < len(ts) && ts[f].is("(") {
			continue
		}
		name := ts[n].text
		if !slices.Contains(p.varTypes[name], t.text) {
			p.varTypes[name] = append(p.varTypes[name], t.text)
		}
	}
}

// isVectorVar reports whether every declaration of obj gives it a
// GLSL vector type, so a name after "obj." is a swizzle.
func (p *porter) isVectorVar(obj string) bool {
	types := p.varTypes[obj]
	if len(types) == 0 {
		return false
	}
	for _, t := range types {
		if _, isStruct := p.structs[t]; isStruct || !isGLSLType(t) {
			return false
		}
	}
	return true
}

// tokText joins token text.
func tokText(ts []token) string {
	var b strings.Builder
	for _, t := range ts {
		b.WriteString(t.text)
	}
	return b.String()
}

// define is a parsed #define line.
type define struct {
	name   string
	params string  // "(a, b)" for a function-like macro, else ""
	body   []token // lexed at its source position
}

// parseDefine splits a #define directive token. ok is false for any
// other directive.
func parseDefine(t token) (d define, ok bool) {
	rest := strings.TrimLeft(t.text[1:], " \t")
	rest, ok = strings.CutPrefix(rest, "define")
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return define{}, false
	}
	rest = strings.TrimLeft(rest, " \t")
	n := 0
	for n < len(rest) && isIdentByte(rest[n]) {
		n++
	}
	d.name = rest[:n]
	rest = rest[n:]
	// A "(" right after the name, with no space, makes it
	// function-like.
	if strings.HasPrefix(rest, "(") {
		if c := strings.IndexByte(rest, ')'); c >= 0 {
			d.params = rest[:c+1]
			rest = rest[c+1:]
		}
	}
	// Body position: the directive's start plus the bytes before the
	// body. A directive is one line unless it continues with "\".
	off := len(t.text) - len(rest)
	d.body = lexAt(rest, t.line, t.col+off)
	return d, true
}

// directiveWord returns the directive name: "define", "if", ...
func directiveWord(t token) string {
	rest := strings.TrimLeft(t.text[1:], " \t")
	n := 0
	for n < len(rest) && isIdentByte(rest[n]) {
		n++
	}
	return rest[:n]
}

// analyze computes which built-ins each function and macro reads,
// through calls and macro uses, so the MSL output can pass them as
// parameters. It also notes whether the shader animates.
func (p *porter) analyze() error {
	p.funcs = map[string]uint8{}
	p.macros = map[string]uint8{}
	// refs maps a function or macro name to the identifiers it uses.
	refs := map[string][]string{}
	var defines []define
	addRefs := func(name string, ts []token) {
		for _, t := range ts {
			if t.kind == tkIdent {
				refs[name] = append(refs[name], t.text)
			}
		}
	}
	collectDefines := func(ts []token) {
		for _, t := range ts {
			if t.kind != tkDirective {
				continue
			}
			if d, ok := parseDefine(t); ok {
				defines = append(defines, d)
				p.topNames[d.name] = true
			}
		}
	}
	for _, it := range p.items {
		ts := p.toks[it.start:it.end]
		collectDefines(ts)
		if it.kind == itemFunc || it.kind == itemProto {
			if _, ok := p.funcs[it.name]; !ok {
				p.funcs[it.name] = 0
			}
			addRefs(it.name, ts)
		}
	}
	for _, d := range defines {
		p.macros[d.name] = 0
		addRefs(d.name, d.body)
	}
	if _, ok := p.funcs["mainImage"]; !ok {
		return &Error{Kind: ErrNoEntry, Msg: "no mainImage function. shaderport " +
			"takes a Shadertoy shader with mainImage(out vec4, in vec2), " +
			"or a go-gui body that declares frag_color"}
	}
	// Fixed point: a name reads a built-in if its own tokens name it,
	// or if it uses a function or macro that reads it.
	for changed := true; changed; {
		changed = false
		for name, rs := range refs {
			var bits uint8
			for _, r := range rs {
				bits |= builtinBit(r)
				if r == name {
					continue
				}
				if b, ok := p.funcs[r]; ok {
					bits |= b
				}
				if b, ok := p.macros[r]; ok {
					bits |= b
				}
			}
			m := p.funcs
			if _, ok := p.macros[name]; ok {
				if _, isFunc := p.funcs[name]; !isFunc {
					m = p.macros
				}
			}
			if bits|m[name] != m[name] {
				m[name] |= bits
				changed = true
			}
		}
	}
	for _, rs := range refs {
		for _, r := range rs {
			if bit := builtinBit(r); bit != 0 && builtins[bitIndex(bit)].anim {
				p.anim = true
			}
		}
	}
	return nil
}

func bitIndex(bit uint8) int {
	for i := range builtins {
		if bit == 1<<i {
			return i
		}
	}
	return -1
}

// portShadertoy emits both languages. mainImage moves to the decls
// whole, so an early return in it works, and each body is one call.
func (p *porter) portShadertoy() (*Result, error) {
	m := &emitter{p: p, msl: true}
	mslDecls, err := m.items()
	if err != nil {
		return nil, err
	}
	g := &emitter{p: p}
	glslDecls, err := g.items()
	if err != nil {
		return nil, err
	}
	if p.usesMod {
		mslDecls = modHelper + "\n" + mslDecls
	}
	var args strings.Builder
	for i, b := range builtins {
		if p.funcs["mainImage"]&(1<<i) != 0 {
			args.WriteString(", ")
			args.WriteString(b.name)
		}
	}
	r := &Result{
		MetalDecls: strings.TrimSpace(mslDecls) + "\n",
		GLSLDecls:  strings.TrimSpace(glslDecls) + "\n",
		Animate:    p.anim,
	}
	// Shadertoy draws its canvas opaque and ignores alpha. go-gui blends
	// with it, so the body pins it to 1: a shader that leaves alpha
	// at 0 would otherwise draw nothing.
	r.Metal = "float4 frag_color = float4(0.0);\n" +
		"mainImage(frag_color, float2(" + sprintfCoord("in.uv") + ")" + args.String() + ");\n" +
		"frag_color.a = 1.0;\n"
	r.GLSL = "vec4 frag_color = vec4(0.0);\n" +
		"mainImage(frag_color, vec2(" + sprintfCoord("uv") + "));\n" +
		"frag_color.a = 1.0;\n"
	return r, nil
}

func sprintfCoord(uv string) string {
	return strings.ReplaceAll(fragCoordExpr, "%[1]s", uv)
}

// portBody translates a go-gui body: GLSL as is, MSL token by token,
// with the wrapper varyings read through `in`.
func (p *porter) portBody() (*Result, error) {
	p.bare = true
	p.funcs = map[string]uint8{}
	p.macros = map[string]uint8{}
	p.structs = map[string][]string{}
	p.fields = map[string]bool{}
	p.topNames = map[string]bool{}
	for _, t := range p.toks {
		if t.kind == tkIdent {
			if bit := builtinBit(t.text); bit != 0 && builtins[bitIndex(bit)].anim {
				p.anim = true
			}
		}
	}
	m := &emitter{p: p, msl: true}
	body, err := m.rewrite(p.toks)
	if err != nil {
		return nil, err
	}
	g := &emitter{p: p}
	glsl, err := g.rewrite(p.toks)
	if err != nil {
		return nil, err
	}
	r := &Result{
		Metal:   strings.TrimSpace(body) + "\n",
		GLSL:    strings.TrimSpace(glsl) + "\n",
		Animate: p.anim,
	}
	if p.usesMod {
		r.MetalDecls = modHelper
	}
	return r, nil
}

// emitter writes one language.
type emitter struct {
	p   *porter
	msl bool
}

// items writes every program-scope item, and the trivia between them,
// in source order.
func (e *emitter) items() (string, error) {
	var b strings.Builder
	at := 0
	for _, it := range e.p.items {
		b.WriteString(tokText(e.p.toks[at:it.start]))
		s, err := e.item(it)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
		at = it.end
	}
	b.WriteString(tokText(e.p.toks[at:]))
	return b.String(), nil
}

func (e *emitter) item(it item) (string, error) {
	ts := e.p.toks
	switch it.kind {
	case itemPrecision:
		return "", nil // MSL has no precision; the GLSL wrapper sets it
	case itemConst:
		s, err := e.rewrite(ts[it.start:it.end])
		if err != nil || !e.msl {
			return s, err
		}
		// A program-scope MSL variable lives in the constant address
		// space.
		return "constant" + strings.TrimPrefix(s, "const"), nil
	case itemFunc, itemProto:
		if !e.msl {
			return e.rewrite(ts[it.start:it.end])
		}
		return e.mslFunc(it)
	}
	return e.rewrite(ts[it.start:it.end])
}

// mslFunc writes a function with its parameter list translated and
// the built-ins it reads appended as parameters.
func (e *emitter) mslFunc(it item) (string, error) {
	ts := e.p.toks
	head, err := e.rewrite(ts[it.start:it.lparen])
	if err != nil {
		return "", err
	}
	var params []string
	for _, prm := range splitArgs(ts[it.lparen+1 : it.rparen]) {
		s, err := e.mslParam(prm)
		if err != nil {
			return "", err
		}
		if s != "" {
			params = append(params, s)
		}
	}
	bits := e.p.funcs[it.name]
	for i, b := range builtins {
		if bits&(1<<i) == 0 {
			continue
		}
		for _, prm := range params {
			if strings.HasSuffix(prm, " "+b.name) {
				return "", errAt(ts[it.lparen], ErrUnsupported,
					"%s has a parameter named %s, which hides the built-in it also reads; rename the parameter",
					it.name, b.name)
			}
		}
		params = append(params, b.mslType+" "+b.name)
	}
	tail, err := e.rewrite(ts[it.rparen+1 : it.end])
	if err != nil {
		return "", err
	}
	return head + "(" + strings.Join(params, ", ") + ")" + tail, nil
}

// mslParam translates one parameter declaration. GLSL out and inout
// become a thread reference; in is the default and is dropped.
func (e *emitter) mslParam(ts []token) (string, error) {
	var words []token
	for _, t := range ts {
		if !t.trivia() {
			words = append(words, t)
		}
	}
	if len(words) == 0 || (len(words) == 1 && words[0].is("void")) {
		return "", nil
	}
	isConst, isRef := false, false
	for len(words) > 0 && words[0].kind == tkIdent {
		switch w := words[0].text; {
		case w == "in":
		case w == "out" || w == "inout":
			isRef = true
		case w == "const":
			isConst = true
		case slices.Contains(precisionWords, w):
		default:
			goto done
		}
		words = words[1:]
	}
done:
	if len(words) == 0 {
		return "", errAt(ts[0], ErrSyntax, "parameter has no type")
	}
	if isRef && len(words) > 2 {
		return "", errAt(words[0], ErrUnsupported,
			"out or inout array parameters are not supported")
	}
	var b strings.Builder
	if isConst {
		b.WriteString("const ")
	}
	if isRef {
		b.WriteString("thread ")
	}
	for i, w := range words {
		s, err := e.rewrite([]token{w})
		if err != nil {
			return "", err
		}
		if i == 1 {
			if isRef {
				b.WriteString("&")
			}
			b.WriteString(" ")
		}
		b.WriteString(s)
	}
	if isRef && len(words) == 1 {
		b.WriteString("&")
	}
	return b.String(), nil
}

// splitArgs splits a token list at depth-0 commas. An empty or
// trivia-only list has no arguments.
func splitArgs(ts []token) [][]token {
	if next(ts, 0) == len(ts) {
		return nil
	}
	var args [][]token
	depth, start := 0, 0
	for i, t := range ts {
		if t.kind != tkPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ",":
			if depth == 0 {
				args = append(args, ts[start:i])
				start = i + 1
			}
		}
	}
	return append(args, ts[start:])
}

// rewrite translates a token run.
func (e *emitter) rewrite(ts []token) (string, error) {
	var b strings.Builder
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		switch t.kind {
		case tkDirective:
			s, err := e.directive(t)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		case tkPunct:
			if t.text == "^^" && e.msl {
				return "", errAt(t, ErrUnsupported, "^^ (logical xor) is not supported; write a != b")
			}
			b.WriteString(t.text)
		case tkIdent:
			s, end, err := e.ident(ts, i)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
			i = end
		default:
			b.WriteString(t.text)
		}
	}
	return b.String(), nil
}

// ident translates the identifier at ts[i], with the call or array
// constructor it starts. It returns the text and the index of the last
// token consumed.
func (e *emitter) ident(ts []token, i int) (string, int, error) {
	t := ts[i]
	name := t.text
	if pv := prev(ts, i); pv >= 0 && ts[pv].is(".") {
		obj := ""
		if o := prev(ts, pv); o >= 0 && ts[o].kind == tkIdent {
			obj = ts[o].text
		}
		return e.member(obj, name), i, nil
	}
	if r := rejectReason(name); r != "" {
		return "", 0, errAt(t, ErrUnsupported, "%s", r)
	}
	n := next(ts, i+1)
	// An array constructor T[N](...) or T[](...): MSL initializes an
	// array from a brace list.
	if isGLSLType(name) && n < len(ts) && ts[n].is("[") {
		c, err := match(ts, n)
		if err != nil {
			return "", 0, err
		}
		lp := next(ts, c+1)
		if lp < len(ts) && ts[lp].is("(") {
			rp, err := match(ts, lp)
			if err != nil {
				return "", 0, err
			}
			args, err := e.args(ts[lp+1 : rp])
			if err != nil {
				return "", 0, err
			}
			if e.msl {
				return "{" + strings.Join(args, ",") + "}", rp, nil
			}
			dims, err := e.rewrite(ts[i+1 : lp])
			if err != nil {
				return "", 0, err
			}
			return name + dims + "(" + strings.Join(args, ",") + ")", rp, nil
		}
		if e.msl {
			return "", 0, errAt(t, ErrUnsupported,
				"array type %s[...] before the name is not supported; write %s name[N]", name, name)
		}
	}
	if n < len(ts) && ts[n].is("(") && !slices.Contains(statementWords, name) {
		rp, err := match(ts, n)
		if err != nil {
			return "", 0, err
		}
		s, err := e.call(t, tokText(ts[i+1:n]), ts[n+1:rp])
		return s, rp, err
	}
	return e.name(name), i, nil
}

// args translates each call argument.
func (e *emitter) args(ts []token) ([]string, error) {
	var out []string
	for _, a := range splitArgs(ts) {
		s, err := e.rewrite(a)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// statementWords are keywords a "(" can follow that do not make a
// call: their parenthesized part is translated as plain tokens.
var statementWords = []string{"if", "for", "while", "switch", "return", "else", "do", "case"}

// call translates a call of t with the argument tokens between the
// parentheses. gap is the trivia between the name and the "(", kept so
// the layout survives.
func (e *emitter) call(t token, gap string, argToks []token) (string, error) {
	args, err := e.args(argToks)
	if err != nil {
		return "", err
	}
	name := t.text
	join := func(fn string) string { return fn + gap + "(" + strings.Join(args, ",") + ")" }
	if !e.msl {
		return join(e.name(name)), nil
	}
	if op, ok := mslCompareFuncs[name]; ok {
		if len(args) != 2 {
			return "", errAt(t, ErrSyntax, "%s takes 2 arguments", name)
		}
		return "(" + strings.TrimSpace(args[0]) + " " + op + " " + strings.TrimSpace(args[1]) + ")", nil
	}
	switch {
	case name == "not" && len(args) == 1:
		return "(!(" + strings.TrimSpace(args[0]) + "))", nil
	case name == "atan" && len(args) == 2:
		return join("atan2"), nil
	case name == "mod":
		e.p.usesMod = true
	}
	if fn, ok := mslRenamedFuncs[name]; ok {
		return join(fn), nil
	}
	if _, ok := e.p.structs[name]; ok {
		return e.name(name) + gap + "{" + strings.Join(args, ",") + "}", nil
	}
	if bits := e.p.funcs[name]; bits != 0 {
		for i, b := range builtins {
			if bits&(1<<i) != 0 {
				args = append(args, " "+b.name)
			}
		}
		if len(argToks) == 0 || next(argToks, 0) == len(argToks) {
			// No user arguments: drop the leading space.
			for i := range args {
				args[i] = strings.TrimSpace(args[i])
			}
		}
	}
	return join(e.name(name)), nil
}

// member translates a name after ".": a struct field keeps its name
// (renamed if its declaration was), a stpq swizzle becomes xyzw in MSL.
// obj is the identifier before the ".", or "" for an expression.
func (e *emitter) member(obj, name string) string {
	if e.p.fields[name] && !e.p.isVectorVar(obj) {
		return e.name(name)
	}
	if e.msl {
		if s, ok := stpqToXYZW(name); ok {
			return s
		}
	}
	return name
}

// name translates a bare identifier: a type name, a keyword, or a name
// that must be renamed to keep clear of MSL or the wrapper.
func (e *emitter) name(name string) string {
	p := e.p
	if p.bare && bareInputs[name] {
		if e.msl {
			return "in." + name
		}
		return name
	}
	if p.topNames[name] && (wrapperNames[name] || strings.HasPrefix(name, "_gg_")) {
		return name + "_"
	}
	if !e.msl {
		return name
	}
	if m, ok := mslTypes[name]; ok {
		return m
	}
	switch {
	case slices.Contains(precisionWords, name):
		return ""
	case name == "discard":
		return "discard_fragment()"
	case isMSLKeyword(name):
		return name + "_"
	case p.topNames[name] && mslStdlibNames[name]:
		return name + "_"
	}
	return name
}

// directive translates a preprocessor line. #version and #extension
// are dropped: the wrapper writes its own header. A #define body is
// translated like code. Other directives pass through.
//
// In GLSL a line continuation ("\" before the newline) is joined: GLSL
// ES 3.00 allows it, but desktop GLSL 3.30, which the GL backend
// compiles, does not.
func (e *emitter) directive(t token) (string, error) {
	s, err := e.directiveText(t)
	if err != nil || e.msl {
		return s, err
	}
	return joinContinuations(s), nil
}

func joinContinuations(s string) string {
	s = strings.ReplaceAll(s, "\\\r\n", " ")
	return strings.ReplaceAll(s, "\\\n", " ")
}

func (e *emitter) directiveText(t token) (string, error) {
	switch directiveWord(t) {
	case "version", "extension":
		return "", nil
	case "pragma":
		if e.msl {
			return "", nil
		}
		return t.text, nil
	case "include":
		return "", errAt(t, ErrUnsupported, "#include is not supported")
	case "define":
		d, ok := parseDefine(t)
		if !ok {
			return t.text, nil
		}
		body, err := e.rewrite(d.body)
		if err != nil {
			return "", err
		}
		params := d.params
		if params == "" {
			body = " " + strings.TrimLeft(body, " \t")
		}
		return "#define " + e.name(d.name) + params + body, nil
	}
	return t.text, nil
}
