// Package eventctx implements the mechanical rewrite from the legacy
// three-argument event callbacks to the single-argument EventCtx form.
//
// Rewrites performed, in order:
//
//  1. Signature. func(l *Layout, e *Event, w *Window) becomes
//     func(ctx EventCtx). Body references to the old parameters become
//     ctx.Layout, ctx.Event and ctx.Window. Payload carriers keep their
//     payload as a leading argument: func(*Layout, string, *Window)
//     becomes func(string, EventCtx), func(T, *Event, *Window) becomes
//     func(T, EventCtx).
//  2. Notify-class callbacks: e.IsHandled = true becomes ctx.Consume().
//     It is never deleted.
//  3. Consume-class callbacks: e.IsHandled = true is deleted when it is
//     the last statement of the function or of a terminal branch. That
//     is the safe subset.
//  4. Everything else in a consume-class callback is reported, not
//     guessed at. A consume-class callback that returns early without
//     setting handled was relying on bubble-by-default and may now need
//     an explicit ctx.Bubble(); the old encoding cannot distinguish
//     "not mine, pass it on" from "done, nothing to do".
//
// The rewrite is textual: edits are computed from AST positions and
// spliced into the original source, so everything the rewrite does not
// touch stays byte-for-byte identical and comments never move.
package eventctx

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strings"
)

// consumeFields are the callback fields whose dispatch marks the event
// handled before the callback runs. Everything else is notify-class.
// Classification only drives cosmetic deletion and the rule-4 report:
// treating a callback as notify is always behaviour-preserving, because
// the surviving ctx.Consume() is a no-op under an auto-consuming
// dispatch.
var consumeFields = []string{
	"OnChar", "OnClick", "OnMouseUp", "OnGesture", "OnFileDrop",
}

// Finding is one entry in the rule-4 report: a return path in a
// consume-class callback that is not dominated by a handled assignment,
// and so is a human decision between ctx.Bubble() and accepting the new
// consume-by-default.
type Finding struct {
	Pos  token.Position
	Kind string // "early-return" or "falls-off-end"
	Func string // owning field or function name, best effort
}

func (f Finding) String() string {
	return fmt.Sprintf("%s: %s in consume-class callback %s",
		f.Pos, f.Kind, f.Func)
}

// Result is the outcome of rewriting one file.
type Result struct {
	Src      []byte // rewritten source, or the input unchanged
	Changed  bool
	Findings []Finding
}

// edit is a byte-range replacement in the original source.
type edit struct {
	start, end int
	text       string
}

// DeclPlan maps the name of a named callback function to the signature
// shape it converts under. Converting a declaration also means moving
// its call sites, so the two are planned together across the whole
// package before any file is rewritten. A nil plan leaves declarations
// and calls alone.
type DeclPlan map[string]sigKind

// ScanDecls finds the named functions in one file that must convert:
// those whose signature matches a callback shape and whose name is used
// as a value somewhere (assigned to a callback field, passed as an
// argument), not only called. A function that is only ever called
// directly is internal dispatch plumbing and keeps its three-argument
// form.
func ScanDecls(filename string, src []byte) (DeclPlan, map[string]bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	cands := DeclPlan{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Recv != nil {
			continue
		}
		if sig := classify(fd.Type); sig != sigNone {
			cands[fd.Name.Name] = sig
		} else if classifyTwo(fd.Type) {
			cands[fd.Name.Name] = sigLW
		}
	}
	// Names used as values: every ident occurrence that is not the
	// callee of a call and not the declaration's own name.
	used := map[string]bool{}
	skip := map[ast.Node]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			skip[c.Fun] = true
		}
		if fd, ok := n.(*ast.FuncDecl); ok {
			skip[fd.Name] = true
		}
		if id, ok := n.(*ast.Ident); ok && !skip[id] {
			used[id.Name] = true
		}
		return true
	})
	return cands, used, nil
}

// Rewrite parses and rewrites a single file. filename is used only for
// positions in diagnostics. plan may be nil.
func Rewrite(filename string, src []byte, plan DeclPlan) (Result, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return Result{}, err
	}
	r := &rewriter{
		fset: fset, src: src, base: fset.File(f.Pos()).Base(), plan: plan,
		guiAlias: guiAlias(f),
	}
	r.run(f)
	if len(r.edits) == 0 {
		return Result{Src: src, Findings: r.findings}, nil
	}
	return Result{
		Src:      r.apply(),
		Changed:  true,
		Findings: r.findings,
	}, nil
}

type rewriter struct {
	fset       *token.FileSet
	src        []byte
	base       int
	edits      []edit
	findings   []Finding
	plan       DeclPlan
	declRanges []declRange
	guiAlias   string // how this file spells the gui package, if at all
}

// declRange is the body of a converted named declaration, with the
// parameter names it had before conversion.
type declRange struct {
	start, end int
	names      [3]string
	cn         string
}

func (r *rewriter) offset(p token.Pos) int { return int(p) - r.base }

func (r *rewriter) replace(start, end token.Pos, text string) {
	r.edits = append(r.edits, edit{r.offset(start), r.offset(end), text})
}

// apply splices the collected edits into the original source. Edits are
// non-overlapping by construction: signature edits cover parameter
// lists, identifier edits cover single idents in bodies, and statement
// edits cover whole statements.
func (r *rewriter) apply() []byte {
	slices.SortFunc(r.edits, func(a, b edit) int { return a.start - b.start })
	var out []byte
	prev := 0
	for _, e := range r.edits {
		if e.start < prev { // defensive: drop an overlapping edit
			continue
		}
		out = append(out, r.src[prev:e.start]...)
		out = append(out, e.text...)
		prev = e.end
	}
	return append(out, r.src[prev:]...)
}

// run walks the file. Function literals are rewritten innermost-first
// so that a nested callback consumes its own parameter names before the
// enclosing callback's identifier rename runs over the same bytes.
//
// Named function declarations are deliberately left alone: converting
// one also means converting every direct call site, which the compiler
// finds reliably and a syntactic tool does not. Only closures and bare
// type expressions are rewritten here.
func (r *rewriter) run(f *ast.File) {
	// FuncTypes owned by a declaration or literal are handled in pass 2
	// (or not at all); pass 1 must not also edit their parameter lists.
	owned := map[*ast.FuncType]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.FuncDecl:
			owned[t.Type] = true
		case *ast.FuncLit:
			owned[t.Type] = true
		}
		return true
	})
	// Pass 1: bare type expressions (struct fields, parameters, results,
	// var declarations). These carry no body.
	ast.Inspect(f, func(n ast.Node) bool {
		ft, ok := n.(*ast.FuncType)
		if !ok || owned[ft] {
			return true
		}
		if sig := classify(ft); sig != sigNone {
			r.rewriteParams(ft, sig, false, "")
		} else if classifyTwo(ft) {
			r.replace(ft.Params.Pos(), ft.Params.End(), "("+ctxType(ft)+")")
		}
		return true
	})
	// Pass 3 (declaration mode) rewrites call sites from the original
	// source text, which pass 2 also edits, so the two modes run as
	// separate invocations: closures first, then declarations.
	if r.plan != nil {
		r.rewriteDecls(f)
		r.rewriteCalls(f)
		return
	}
	// Pass 2: bodies. Collect innermost-first, then rewrite.
	var lits []*litInfo
	collectLits(f, nil, "", &lits)
	for _, li := range lits {
		r.rewriteBody(li)
	}
}

// rewriteDecls converts the named functions named in the plan, using
// the same body rewrite as a closure.
func (r *rewriter) rewriteDecls(f *ast.File) {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Recv != nil {
			continue
		}
		if _, want := r.plan[fd.Name.Name]; !want {
			continue
		}
		li := makeLit(fd.Type, fd.Body, fd.Name.Name)
		if li == nil {
			continue
		}
		r.rewriteBody(li)
		// Record the body's parameter names so calls nested inside it
		// fold using the post-rename spelling, not the original one.
		r.declRanges = append(r.declRanges, declRange{
			start: r.offset(fd.Body.Pos()),
			end:   r.offset(fd.Body.End()),
			names: li.names,
			cn:    li.cn,
		})
	}
}

// renameArg maps an argument's original source text through the rename
// of whichever converted declaration encloses it.
func (r *rewriter) renameArg(pos token.Pos, text string) string {
	off := r.offset(pos)
	for _, d := range r.declRanges {
		if off < d.start || off >= d.end {
			continue
		}
		switch text {
		case d.names[0]:
			return d.cn + ".Layout"
		case d.names[1]:
			return d.cn + ".Event"
		case d.names[2]:
			return d.cn + ".Window"
		}
	}
	return text
}

// rewriteCalls folds the argument list of every call to a planned
// function into an EventCtx. Calls already inside a converted callback
// collapse to the bare ctx, which is both shorter and free.
func (r *rewriter) rewriteCalls(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		sig, want := r.plan[id.Name]
		if !want || call.Ellipsis.IsValid() {
			return true
		}
		var layout, event, window, payload string
		src := func(i int) string {
			return r.renameArg(call.Args[i].Pos(), r.srcOf(call.Args[i]))
		}
		switch sig {
		case sigLEW:
			if len(call.Args) != 3 {
				return true
			}
			layout, event, window = src(0),
				src(1), src(2)
		case sigLW:
			if len(call.Args) != 2 {
				return true
			}
			layout, event, window = src(0), "nil",
				src(1)
		case sigLPW:
			if len(call.Args) != 3 {
				return true
			}
			layout, event, window = src(0), "nil",
				src(2)
			payload = src(1)
		case sigPEW:
			if len(call.Args) != 3 {
				return true
			}
			layout, event, window = "nil", src(1),
				src(2)
			payload = src(0)
		default:
			return true
		}
		ctxArg := "EventCtx{" + layout + ", " + event + ", " + window + "}"
		if r.guiAlias != "" {
			// Keyed form: go vet rejects unkeyed literals of a type
			// from another package.
			ctxArg = r.guiAlias + ".EventCtx{Layout: " + layout +
				", Event: " + event + ", Window: " + window + "}"
		}
		if layout == "ctx.Layout" && window == "ctx.Window" &&
			(event == "ctx.Event" || event == "nil") {
			ctxArg = "ctx"
		}
		if payload != "" {
			ctxArg = payload + ", " + ctxArg
		}
		r.replace(call.Lparen+1, call.Rparen, ctxArg)
		return true
	})
}

type sigKind int

const (
	sigNone sigKind = iota
	sigLEW          // (*Layout, *Event, *Window)
	sigLW           // (*Layout, *Window)
	sigLPW          // (*Layout, payload, *Window)
	sigPEW          // (payload, *Event, *Window) — payload is not *Layout
)

// isPtrTo reports whether expr is *Name or *pkg.Name.
func isPtrTo(expr ast.Expr, name string) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch t := star.X.(type) {
	case *ast.Ident:
		return t.Name == name
	case *ast.SelectorExpr:
		return t.Sel.Name == name
	}
	return false
}

// flatParams expands grouped parameters ("a, b *T") into one entry per
// name so positions line up with the signature shapes above.
type param struct {
	name  string // "" when the parameter is unnamed
	typ   ast.Expr
	blank bool
}

func flatParams(ft *ast.FuncType) []param {
	var ps []param
	if ft.Params == nil {
		return ps
	}
	for _, fld := range ft.Params.List {
		if len(fld.Names) == 0 {
			ps = append(ps, param{typ: fld.Type})
			continue
		}
		for _, nm := range fld.Names {
			ps = append(ps, param{
				name:  nm.Name,
				typ:   fld.Type,
				blank: nm.Name == "_",
			})
		}
	}
	return ps
}

// classify matches a function type against the four convertible shapes.
func classify(ft *ast.FuncType) sigKind {
	ps := flatParams(ft)
	if len(ps) != 3 || ft.Results != nil {
		return sigNone
	}
	l, m, w := ps[0].typ, ps[1].typ, ps[2].typ
	if !isPtrTo(w, "Window") {
		return sigNone
	}
	switch {
	case isPtrTo(l, "Layout") && isPtrTo(m, "Event"):
		return sigLEW
	case isPtrTo(l, "Layout"):
		return sigLPW
	case isPtrTo(m, "Event"):
		return sigPEW
	}
	return sigNone
}

// classifyTwo matches func(*Layout, *Window).
func classifyTwo(ft *ast.FuncType) bool {
	ps := flatParams(ft)
	return len(ps) == 2 && ft.Results == nil &&
		isPtrTo(ps[0].typ, "Layout") && isPtrTo(ps[1].typ, "Window")
}

// ctxType returns the EventCtx type expression spelled the same way the
// *Window parameter was — "EventCtx" inside package gui, "gui.EventCtx"
// outside it.
func ctxType(ft *ast.FuncType) string {
	ps := flatParams(ft)
	w := ps[len(ps)-1].typ
	if star, ok := w.(*ast.StarExpr); ok {
		if sel, ok := star.X.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				return id.Name + ".EventCtx"
			}
		}
	}
	return "EventCtx"
}

// srcOf returns the original source text of a node.
func (r *rewriter) srcOf(n ast.Node) string {
	return string(r.src[r.offset(n.Pos()):r.offset(n.End())])
}

// rewriteParams replaces the parameter list. named controls whether the
// EventCtx parameter gets the name "ctx"; bare type expressions leave it
// unnamed.
func (r *rewriter) rewriteParams(
	ft *ast.FuncType, sig sigKind, named bool, cn string,
) {
	ps := flatParams(ft)
	ct := ctxType(ft)
	ctxParam := ct
	if named {
		ctxParam = cn + " " + ct
	}
	var text string
	switch sig {
	case sigLEW, sigLW:
		text = "(" + ctxParam + ")"
	case sigLPW:
		// Payload is the middle parameter; the *Layout is folded away.
		text = "(" + r.payload(ps[1], named) + ", " + ctxParam + ")"
	case sigPEW:
		// Payload is the leading parameter; the *Event is folded away.
		text = "(" + r.payload(ps[0], named) + ", " + ctxParam + ")"
	default:
		return
	}
	r.replace(ft.Params.Pos(), ft.Params.End(), text)
}

func (r *rewriter) payload(p param, named bool) string {
	t := r.srcOf(p.typ)
	if !named {
		return t
	}
	// Go forbids mixing named and unnamed parameters, so an anonymous
	// payload needs a blank name once the context parameter is named.
	if p.name == "" {
		return "_ " + t
	}
	return p.name + " " + t
}

// litInfo is one function body due for conversion.
type litInfo struct {
	ft    *ast.FuncType
	body  *ast.BlockStmt
	sig   sigKind
	two   bool // matched func(*Layout, *Window)
	names [3]string
	owner string // enclosing field or func name, for the report
	cls   bool   // true when consume-class
	cn    string // context parameter name, "ctx" unless that collides
}

// collectLits gathers convertible function literals and declarations
// innermost-first. owner is the nearest enclosing struct field key or
// assignment target, which is how the class is determined.
func collectLits(n ast.Node, _ ast.Node, owner string, out *[]*litInfo) {
	switch t := n.(type) {
	case *ast.KeyValueExpr:
		key := ""
		if id, ok := t.Key.(*ast.Ident); ok {
			key = id.Name
		}
		collectLits(t.Value, t, key, out)
		return
	case *ast.AssignStmt:
		for i, rhs := range t.Rhs {
			key := owner
			if i < len(t.Lhs) {
				if sel, ok := t.Lhs[i].(*ast.SelectorExpr); ok {
					key = sel.Sel.Name
				}
			}
			collectLits(rhs, t, key, out)
		}
		for _, lhs := range t.Lhs {
			collectLits(lhs, t, owner, out)
		}
		return
	case *ast.FuncDecl:
		// Body only — the declaration's own signature is left for a
		// human, since its call sites move with it.
		for _, c := range childNodes(t) {
			collectLits(c, t, t.Name.Name, out)
		}
		return
	case *ast.FuncLit:
		for _, c := range childNodes(t) {
			collectLits(c, t, owner, out)
		}
		if li := makeLit(t.Type, t.Body, owner); li != nil {
			*out = append(*out, li)
		}
		return
	}
	for _, c := range childNodes(n) {
		collectLits(c, n, owner, out)
	}
}

// childNodes returns the direct children of n. ast.Inspect cannot be
// used here because the traversal has to be depth-first post-order with
// an owner threaded through.
func childNodes(n ast.Node) []ast.Node {
	var kids []ast.Node
	first := true
	ast.Inspect(n, func(c ast.Node) bool {
		if first {
			first = false
			return true
		}
		if c == nil {
			return false
		}
		kids = append(kids, c)
		return false
	})
	return kids
}

func makeLit(ft *ast.FuncType, body *ast.BlockStmt, owner string) *litInfo {
	if body == nil {
		return nil
	}
	sig := classify(ft)
	two := false
	if sig == sigNone {
		if !classifyTwo(ft) {
			return nil
		}
		two = true
	}
	ps := flatParams(ft)
	li := &litInfo{
		ft: ft, body: body, sig: sig, two: two, owner: owner,
		cls: isConsumeOwner(owner),
		cn:  contextName(body, ps),
	}
	switch {
	case two:
		li.names = [3]string{ps[0].name, "", ps[1].name}
	case sig == sigLEW:
		li.names = [3]string{ps[0].name, ps[1].name, ps[2].name}
	case sig == sigLPW:
		li.names = [3]string{ps[0].name, "", ps[2].name}
	case sig == sigPEW:
		li.names = [3]string{"", ps[1].name, ps[2].name}
	}
	return li
}

// isConsumeOwner reports whether owner names a consume-class callback.
// Factories are matched by suffix: makeInputOnChar and
// dataGridMakeOnChar both produce an OnChar callback, and their bodies
// carry exactly the filtering risk rule 4 exists to surface.
func isConsumeOwner(owner string) bool {
	if slices.Contains(consumeFields, owner) {
		return true
	}
	for _, f := range consumeFields {
		if strings.HasSuffix(owner, f) {
			return true
		}
	}
	return false
}

// contextName picks the name for the EventCtx parameter. "ctx" is the
// convention, but some bodies already use that name for something else
// (a grid jump context, say), so fall back to "ectx" rather than
// shadowing it.
func contextName(body *ast.BlockStmt, ps []param) string {
	for _, p := range ps {
		if p.name == "ctx" {
			return "ctx" // it is one of ours; renaming it is the point
		}
	}
	taken := false
	ast.Inspect(body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "ctx" {
			taken = true
		}
		return true
	})
	if taken {
		return "ectx"
	}
	return "ctx"
}

func (r *rewriter) rewriteBody(li *litInfo) {
	if li.two {
		ct := ctxType(li.ft)
		r.replace(li.ft.Params.Pos(), li.ft.Params.End(), "("+li.cn+" "+ct+")")
	} else {
		r.rewriteParams(li.ft, li.sig, true, li.cn)
	}
	r.rewriteHandled(li)
	r.renameIdents(li)
	if li.cls {
		r.reportRule4(li)
	}
}

// handledSel reports whether expr is <eventParam>.IsHandled.
func handledSel(expr ast.Expr, ev string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "IsHandled" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && ev != "" && id.Name == ev
}

// isHandledAssign reports whether s is `<ev>.IsHandled = <bool literal>`
// and which literal.
func isHandledAssign(s ast.Stmt, ev string) (val bool, ok bool) {
	as, isAssign := s.(*ast.AssignStmt)
	if !isAssign || as.Tok != token.ASSIGN ||
		len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return false, false
	}
	if !handledSel(as.Lhs[0], ev) {
		return false, false
	}
	id, isIdent := as.Rhs[0].(*ast.Ident)
	if !isIdent {
		return false, false
	}
	switch id.Name {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// rewriteHandled applies rules 2 and 3 to handled assignments in the
// body. Assignments in nested non-convertible closures are left alone —
// they belong to a different event, if any.
func (r *rewriter) rewriteHandled(li *litInfo) {
	ev := li.names[1]
	if ev == "" || ev == "_" {
		return
	}
	terminal := terminalStmts(li.body)
	forEachStmtList(li.body, func(list []ast.Stmt, i int) {
		s := list[i]
		val, ok := isHandledAssign(s, ev)
		if !ok {
			return
		}
		if !val {
			r.replace(s.Pos(), s.End(), li.cn+".Bubble()")
			return
		}
		// Rule 3: in a consume-class callback the pre-mark already
		// happened, so a terminal assignment is pure redundancy.
		if li.cls && terminal[s] {
			r.deleteStmt(list, i)
			return
		}
		// Rule 2 (and the non-terminal consume-class case): keep the
		// intent, just spell it through the context.
		r.replace(s.Pos(), s.End(), li.cn+".Consume()")
	})
}

// deleteStmt removes a whole statement including its line. The end of
// the previous statement (or the block's opening brace) is the splice
// point, so the newline and indentation go with it.
func (r *rewriter) deleteStmt(list []ast.Stmt, i int) {
	start := r.offset(list[i].Pos())
	// Walk back over the indentation to the end of the previous line.
	for start > 0 && (r.src[start-1] == ' ' || r.src[start-1] == '\t') {
		start--
	}
	if start > 0 && r.src[start-1] == '\n' {
		start--
	}
	end := r.offset(list[i].End())
	r.edits = append(r.edits, edit{start, end, ""})
}

// terminalStmts returns the set of statements that sit in a terminal
// position: the last statement of the body, or the last statement of a
// branch that is itself terminal, or the statement immediately before a
// bare return.
func terminalStmts(body *ast.BlockStmt) map[ast.Stmt]bool {
	set := map[ast.Stmt]bool{}
	var markLast func(list []ast.Stmt)
	markLast = func(list []ast.Stmt) {
		if len(list) == 0 {
			return
		}
		last := list[len(list)-1]
		if ret, ok := last.(*ast.ReturnStmt); ok && len(ret.Results) == 0 {
			// `x; return` — x is still terminal.
			if len(list) >= 2 {
				set[list[len(list)-2]] = true
			}
			markBranches(list[:len(list)-1], markLast)
			return
		}
		set[last] = true
		markBranches(list, markLast)
	}
	markLast(body.List)
	return set
}

// markBranches recurses into the terminal branches of the last
// statement of list, so that the tail of an if/else or switch case that
// ends the function counts as terminal too.
func markBranches(list []ast.Stmt, markLast func([]ast.Stmt)) {
	if len(list) == 0 {
		return
	}
	switch t := list[len(list)-1].(type) {
	case *ast.IfStmt:
		markLast(t.Body.List)
		switch e := t.Else.(type) {
		case *ast.BlockStmt:
			markLast(e.List)
		case *ast.IfStmt:
			markBranches([]ast.Stmt{e}, markLast)
		}
	case *ast.SwitchStmt:
		for _, c := range t.Body.List {
			if cc, ok := c.(*ast.CaseClause); ok {
				markLast(cc.Body)
			}
		}
	case *ast.TypeSwitchStmt:
		for _, c := range t.Body.List {
			if cc, ok := c.(*ast.CaseClause); ok {
				markLast(cc.Body)
			}
		}
	case *ast.BlockStmt:
		markLast(t.List)
	}
}

// forEachStmtList visits every statement list in the body, skipping
// nested function literals (their statements belong to another frame).
func forEachStmtList(body *ast.BlockStmt, fn func(list []ast.Stmt, i int)) {
	var walkList func(list []ast.Stmt)
	var walkStmt func(s ast.Stmt)
	walkList = func(list []ast.Stmt) {
		for i := range list {
			fn(list, i)
			walkStmt(list[i])
		}
	}
	walkStmt = func(s ast.Stmt) {
		switch t := s.(type) {
		case *ast.BlockStmt:
			walkList(t.List)
		case *ast.IfStmt:
			walkList(t.Body.List)
			if t.Else != nil {
				walkStmt(t.Else)
			}
		case *ast.ForStmt:
			walkList(t.Body.List)
		case *ast.RangeStmt:
			walkList(t.Body.List)
		case *ast.SwitchStmt:
			walkList(t.Body.List)
		case *ast.TypeSwitchStmt:
			walkList(t.Body.List)
		case *ast.SelectStmt:
			walkList(t.Body.List)
		case *ast.CaseClause:
			walkList(t.Body)
		case *ast.CommClause:
			walkList(t.Body)
		case *ast.LabeledStmt:
			walkStmt(t.Stmt)
		}
	}
	walkList(body.List)
}

// renameIdents rewrites references to the old parameters. Subtrees of
// nested function literals that redeclare one of the names are skipped
// for that name, so a `func(w *Window)` closure keeps its own w.
func (r *rewriter) renameIdents(li *litInfo) {
	repl := map[string]string{}
	add := func(name, field string) {
		if name != "" && name != "_" {
			repl[name] = li.cn + "." + field
		}
	}
	add(li.names[0], "Layout")
	add(li.names[1], "Event")
	add(li.names[2], "Window")
	if len(repl) == 0 {
		return
	}
	r.walkRename(li.body, repl)
}

func (r *rewriter) walkRename(n ast.Node, repl map[string]string) {
	ast.Inspect(n, func(c ast.Node) bool {
		switch t := c.(type) {
		case nil:
			return false
		case *ast.SelectorExpr:
			// Rewrite the qualifier but never the selected field name.
			r.walkRename(t.X, repl)
			return false
		case *ast.FuncLit:
			shadowed := maps.Clone(repl)
			for _, p := range flatParams(t.Type) {
				delete(shadowed, p.name)
			}
			if t.Type.Results != nil {
				for _, fld := range t.Type.Results.List {
					for _, nm := range fld.Names {
						delete(shadowed, nm.Name)
					}
				}
			}
			if len(shadowed) > 0 {
				r.walkRename(t.Body, shadowed)
			}
			return false
		case *ast.KeyValueExpr:
			// Struct-literal keys are field names, not references.
			r.walkRename(t.Value, repl)
			return false
		case *ast.Ident:
			if v, ok := repl[t.Name]; ok {
				r.replace(t.Pos(), t.End(), v)
			}
			return false
		}
		return true
	})
}

// reportRule4 lists the return paths of a consume-class callback that
// are not dominated by a handled assignment. Each entry is a human
// decision: insert ctx.Bubble(), or confirm consume-by-default.
func (r *rewriter) reportRule4(li *litInfo) {
	ev := li.names[1]
	sawHandled := false
	if ev != "" {
		forEachStmtList(li.body, func(list []ast.Stmt, i int) {
			if v, ok := isHandledAssign(list[i], ev); ok && v {
				sawHandled = true
			}
		})
	}
	// A callback that never mentions handled and has no branching
	// returns is unconditional: consume-by-default is what it meant.
	var returns []*ast.ReturnStmt
	forEachStmtList(li.body, func(list []ast.Stmt, i int) {
		if ret, ok := list[i].(*ast.ReturnStmt); ok {
			returns = append(returns, ret)
		}
	})
	if len(returns) == 0 && !sawHandled {
		return
	}
	for _, ret := range returns {
		r.findings = append(r.findings, Finding{
			Pos:  r.fset.Position(ret.Pos()),
			Kind: "early-return",
			Func: li.owner,
		})
	}
	if sawHandled {
		r.findings = append(r.findings, Finding{
			Pos:  r.fset.Position(li.body.Rbrace),
			Kind: "conditional-consume",
			Func: li.owner,
		})
	}
}

// NeedsRewrite reports whether src mentions anything the rewrite acts
// on, so callers can skip untouched files cheaply.
func NeedsRewrite(src []byte) bool {
	// Match the bare and the package-qualified spellings alike: a file
	// outside package gui writes *gui.Window (or its own import alias).
	s := string(src)
	return strings.Contains(s, "Window") || strings.Contains(s, "IsHandled")
}
