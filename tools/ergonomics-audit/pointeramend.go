package main

// Mode pointeramend answers: does an AmendLayout hook read the pointer
// position without opting into pointer moves?
//
// Idle moves skip the rebuild when the last arranged frame proves nothing
// reacts to them (#973). An AmendLayout hook that tests the pointer marks
// its container pointerAmend, so a move into or out of the bounds rebuilds
// and the hook sees it. A hook that reads the position without the flag
// freezes its enter and leave logic, with no error at the site that wrote
// it. Only WithTooltip sets the flag today.
//
// The scan covers the files of package gui in gui/ (only that package can
// name viewState). It finds every function installed as AmendLayout: an
// AmendLayout key in any composite literal, an assignment to an AmendLayout
// field, an amendAll argument, or a factory that returns the hook. Then it
// follows same-package calls from each root into hook-shaped delegates.
// A read is mousePosX, mousePosY, pointerX or pointerY on viewState, in
// any spelling.
//
// A root is clean when each install sets pointerAmend in the same literal,
// or when each read it reaches carries the marker. The marker goes on the
// read line, or in the doc of the function that owns the read (as
// rtfAmendTooltip does). Marked reads print as deferred, not gating.
//
// Three shapes stay out of scope. Animate callbacks (animationTooltip reads
// the position from a timer, not from AmendLayout). arrangedMouseX/Y (the
// last arrange position, not the live one). Event coordinates (nil during
// AmendLayout, a separate fault). A hook installed through a func-typed
// variable or field (fw.ring) is invisible to the scan: mark the hook where
// it is defined.
import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// pointerAmendMarker exempts one read: the line it sits on, or the doc of
// the function that owns the read. The comment states why the read is safe
// without pointerAmend, so the next reader does not re-ask the question.
const pointerAmendMarker = "ergonomics-audit:pointeramend"

// pointerAmendFields are the viewState fields that hold the live pointer
// position. A hook that branches on one must see every move across its
// bounds, which is what pointerAmend buys.
var pointerAmendFields = map[string]bool{
	"mousePosX": true,
	"mousePosY": true,
	"pointerX":  true,
	"pointerY":  true,
}

// paRead is one position read, with the function that owns it (nil for a
// read in a literal installed ad hoc, which has no doc to mark).
type paRead struct {
	line  int
	expr  *ast.SelectorExpr
	owner *paFunc
}

// paFunc is one package-level function: its shape, its doc, and the reads
// and calls in its body. Reads and calls in nested closures are filed
// under the closure that holds them, so a factory keeps generation-time
// code apart from the hook it returns.
type paFunc struct {
	decl       *ast.FuncDecl
	file       *paFile
	hookShaped bool // func(EventCtx) with no results: the body runs at amend time
	docMarked  bool
	reads      []paRead // body outside nested closures
	calls      []string // body outside nested closures
	closures   []*ast.FuncLit
}

// paInstall is one AmendLayout installation: a named hook, a factory that
// returns one, or an ad-hoc literal. flag is set when the same literal
// sets pointerAmend: true.
type paInstall struct {
	name    string // named root, "" for an ad-hoc literal
	factory bool   // the named root returns the hook; its own body runs at generation
	lit     *ast.FuncLit
	file    *paFile // file that holds the install (and an ad-hoc literal)
	flag    bool
	line    int
}

// paFile is one scanned file: its marks, its imports, its functions, and
// its installs.
type paFile struct {
	rel     string
	fset    *token.FileSet
	marked  map[int]bool
	imports map[string]bool
	funcs   map[string][]*paFunc
	install []paInstall
}

// paFinding is one unmarked position read reached from AmendLayout.
type paFinding struct {
	path string
	line int
	root string // hook or install that the read runs under
	expr string
}

// runPointerAmend reports AmendLayout hooks that read the pointer without
// pointerAmend. It returns an error when any survive, so the audit gates
// like modes ids, opt and literals.
func runPointerAmend(repos []string) error {
	var gating []paFinding
	var deferred []paFinding
	for _, repo := range repos {
		g, d, err := scanPointerAmend(repo)
		if err != nil {
			return err
		}
		gating = append(gating, g...)
		deferred = append(deferred, d...)
	}
	sort.Slice(gating, func(i, j int) bool {
		if gating[i].path != gating[j].path {
			return gating[i].path < gating[j].path
		}
		return gating[i].line < gating[j].line
	})
	sort.Slice(deferred, func(i, j int) bool {
		if deferred[i].path != deferred[j].path {
			return deferred[i].path < deferred[j].path
		}
		return deferred[i].line < deferred[j].line
	})

	fmt.Printf("Pointer-amend audit (%d finding(s), %d deferred)\n",
		len(gating), len(deferred))
	for _, f := range gating {
		fmt.Printf("  %s:%d: %s reads the pointer without pointerAmend: %s\n",
			f.path, f.line, f.root, f.expr)
	}
	for _, f := range deferred {
		fmt.Printf("  %s:%d: %s [deferred] %s\n",
			f.path, f.line, f.root, f.expr)
	}
	if len(gating) > 0 {
		fmt.Println()
		fmt.Println("An AmendLayout hook that reads the pointer position must")
		fmt.Println("see moves across its bounds. Set pointerAmend in the same")
		fmt.Println("literal, or mark the read safe with")
		fmt.Printf("  %q and the reason.\n", pointerAmendMarker)
		return fmt.Errorf("%d pointer-amend finding(s)", len(gating))
	}
	return nil
}

// scanPointerAmend runs the audit over one repo and splits the reached
// reads into gating findings and deferred (marked or flag-covered) ones.
func scanPointerAmend(repo string) ([]paFinding, []paFinding, error) {
	var files []*paFile
	err := walkGo(filepath.Join(repo, "gui"), func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		if f.Name.Name != "gui" {
			return
		}
		rel := relPath(repo, path)
		if filepath.Dir(rel) != "gui" {
			return
		}
		files = append(files, paScanFile(rel, fset, f))
	})
	if err != nil {
		return nil, nil, err
	}
	index := map[string][]*paFunc{}
	for _, fl := range files {
		for name, list := range fl.funcs {
			index[name] = append(index[name], list...)
		}
	}

	// One hook can install in several places. The flag covers only the
	// literal that sets it, so a hook is clean only when each install
	// sets it. Group named installs by hook first.
	byName := map[string][]paInstall{}
	var anon []paInstall
	for _, fl := range files {
		for _, in := range fl.install {
			if in.name == "" {
				anon = append(anon, in)
				continue
			}
			byName[in.name] = append(byName[in.name], in)
		}
	}

	seen := map[string]bool{}
	var gating, deferred []paFinding
	report := func(fl *paFile, r paRead, root string, ok bool) {
		key := fl.rel + "\x00" + strconv.Itoa(r.line)
		if seen[key] {
			return
		}
		seen[key] = true
		found := paFinding{
			path: fl.rel, line: r.line, root: root,
			expr: shortExpr(fl.fset, r.expr),
		}
		if ok {
			deferred = append(deferred, found)
		} else {
			gating = append(gating, found)
		}
	}

	// Map order is random, so visit hooks by name: a read reached from
	// two installs must report the same root on each run.
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		installs := byName[name]
		reads := paClosureReads(index, name, installs)
		allFlag := true
		for _, in := range installs {
			if !in.flag {
				allFlag = false
			}
		}
		for _, r := range reads {
			fl := r.file
			report(fl, r.read, "hook "+name, paExempt(fl, r.read) || allFlag)
		}
	}
	for _, in := range anon {
		reads := paLitsClosure(index, in.file, in.lit)
		for _, r := range reads {
			fl := r.file
			loc := fmt.Sprintf("AmendLayout literal at %s:%d", fl.rel, in.line)
			report(fl, r.read, loc, paExempt(fl, r.read) || in.flag)
		}
	}
	return gating, deferred, nil
}

// paExempt reports whether a reached read carries the marker: on its own
// line, or in the doc of the function that owns it.
func paExempt(fl *paFile, r paRead) bool {
	return fl.marked[r.line] || (r.owner != nil && r.owner.docMarked)
}

// paFileRead is a read tagged with the file that holds it.
type paFileRead struct {
	file *paFile
	read paRead
}

// paClosureReads returns each position read that runs under the named hook:
// the hook body (when hook-shaped) or the closures it returns (when a
// factory), plus what those reach through same-package calls.
//
// Calls are followed only into hook-shaped delegates: a wrapper installed
// as AmendLayout that calls another hook (rtfSelectAmendLayout calling
// rtfAmendTooltip). Pipeline helpers (scroll, hover bookkeeping) also read
// the position at amend time, but the idle-move check already models the
// pipeline; the audit targets widget hooks. Their reads stay out.
func paClosureReads(
	index map[string][]*paFunc, name string, installs []paInstall,
) []paFileRead {
	factory := true
	for _, in := range installs {
		if !in.factory {
			factory = false
		}
	}
	var out []paFileRead
	seen := map[string]bool{}
	var queue []string
	add := func(fl *paFile, r paRead) {
		out = append(out, paFileRead{file: fl, read: r})
	}
	for _, info := range index[name] {
		fl := info.file
		if !factory && info.hookShaped {
			for _, r := range info.reads {
				add(fl, r)
			}
			queue = append(queue, info.calls...)
		}
		for _, lit := range info.closures {
			rs, cs := paScanClosure(fl, lit, info)
			for _, r := range rs {
				add(fl, r)
			}
			queue = append(queue, cs...)
		}
	}
	out = append(out, paFollowCalls(index, queue, seen)...)
	return out
}

// paLitsClosure returns each position read under an ad-hoc literal and
// what it reaches through same-package calls.
func paLitsClosure(
	index map[string][]*paFunc, fl *paFile, lit *ast.FuncLit,
) []paFileRead {
	out := []paFileRead{}
	rs, cs := paScanClosure(fl, lit, nil)
	for _, r := range rs {
		out = append(out, paFileRead{file: fl, read: r})
	}
	out = append(out, paFollowCalls(index, cs, map[string]bool{})...)
	return out
}

// paFollowCalls walks same-package calls to a fixpoint and returns the
// position reads in each reached hook-shaped body, closures included.
// Reads in non-hook helpers stay out (see paClosureReads). Calls are
// still traversed through them, so a hook reached through a helper
// still reports.
func paFollowCalls(
	index map[string][]*paFunc, queue []string, seen map[string]bool,
) []paFileRead {
	var out []paFileRead
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, info := range index[name] {
			fl := info.file
			if info.hookShaped {
				for _, r := range info.reads {
					out = append(out, paFileRead{file: fl, read: r})
				}
			}
			queue = append(queue, info.calls...)
			for _, lit := range info.closures {
				rs, cs := paScanClosure(fl, lit, info)
				for _, r := range rs {
					out = append(out, paFileRead{file: fl, read: r})
				}
				queue = append(queue, cs...)
			}
		}
	}
	return out
}

// paScanFile parses one file into functions and AmendLayout installs.
func paScanFile(rel string, fset *token.FileSet, f *ast.File) *paFile {
	fl := &paFile{
		rel:     rel,
		fset:    fset,
		marked:  markedLines(fset, f, pointerAmendMarker),
		imports: map[string]bool{},
		funcs:   map[string][]*paFunc{},
	}
	for _, imp := range f.Imports {
		fl.imports[paImportName(imp)] = true
	}
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok || d.Body == nil {
			continue
		}
		info := &paFunc{decl: d, file: fl}
		info.hookShaped = paHookShaped(d)
		if d.Doc != nil {
			for _, c := range d.Doc.List {
				if strings.Contains(c.Text, pointerAmendMarker) {
					info.docMarked = true
				}
			}
		}
		paScanTop(fl, info)
		fl.funcs[d.Name.Name] = append(fl.funcs[d.Name.Name], info)
	}
	paScanInstalls(fl, f)
	return fl
}

// paImportName reports the local name of one import.
func paImportName(imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	path, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		return ""
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// paHookShaped reports whether the declaration is func(EventCtx) with no
// results: the body runs at amend time, not at generation.
func paHookShaped(d *ast.FuncDecl) bool {
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		return false
	}
	if d.Type.Params == nil || len(d.Type.Params.List) != 1 {
		return false
	}
	return paIsEventCtx(d.Type.Params.List[0].Type)
}

// paIsEventCtx reports whether the type is EventCtx, qualified or bare.
func paIsEventCtx(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "EventCtx"
	case *ast.SelectorExpr:
		return t.Sel.Name == "EventCtx"
	}
	return false
}

// paScanTop files the top-level reads, calls and closures of one function.
// Nested closures are collected, not descended into: they run at amend
// time only when the function returns them, which the resolution step
// decides per install shape.
func paScanTop(fl *paFile, info *paFunc) {
	writes := paWritePos(info.decl.Body)
	ast.Inspect(info.decl.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			info.closures = append(info.closures, lit)
			return false
		}
		paNote(fl, info, n, writes, &info.reads, &info.calls)
		return true
	})
}

// paScanClosure files the reads and calls anywhere in one closure,
// including closures nested in it. A nested closure runs when the outer
// one runs it, so both belong to the same amend-time code.
func paScanClosure(
	fl *paFile, lit *ast.FuncLit, owner *paFunc,
) ([]paRead, []string) {
	var reads []paRead
	var calls []string
	writes := paWritePos(lit.Body)
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		paNote(fl, owner, n, writes, &reads, &calls)
		return true
	})
	return reads, calls
}

// paNote files one node as a position read or a same-package call.
func paNote(
	fl *paFile, owner *paFunc, n ast.Node,
	writes map[token.Pos]bool, reads *[]paRead, calls *[]string,
) {
	switch node := n.(type) {
	case *ast.SelectorExpr:
		if pointerAmendFields[node.Sel.Name] && !writes[node.Pos()] {
			*reads = append(*reads, paRead{
				line:  fl.fset.Position(node.Pos()).Line,
				expr:  node,
				owner: owner,
			})
		}
	case *ast.CallExpr:
		if name := paCallee(node, fl.imports); name != "" {
			*calls = append(*calls, name)
		}
	}
}

// paWritePos returns the positions of selectors that an assignment writes:
// mousePosX = x is framework bookkeeping, not a read the audit flags.
func paWritePos(body *ast.BlockStmt) map[token.Pos]bool {
	out := map[token.Pos]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range as.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok &&
				pointerAmendFields[sel.Sel.Name] {
				out[sel.Pos()] = true
			}
		}
		return true
	})
	return out
}

// paCallee names the same-package function a call can reach, or "" when
// the call leaves the package (a qualified import, a builtin, a method on
// an imported type). A selector on a local value is a method call and
// resolves by method name.
func paCallee(call *ast.CallExpr, imports map[string]bool) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		if paBuiltin(fn.Name) {
			return ""
		}
		return fn.Name
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok && !imports[id.Name] {
			return fn.Sel.Name
		}
	}
	return ""
}

// paBuiltin reports the predeclared functions: they resolve to no FuncDecl.
func paBuiltin(name string) bool {
	switch name {
	case "append", "cap", "clear", "close", "complex", "copy", "delete",
		"imag", "len", "make", "max", "min", "new", "panic", "print",
		"println", "real", "recover":
		return true
	}
	return false
}

// paScanInstalls files each AmendLayout install in the file: a composite
// literal key in any struct, or an assignment to an AmendLayout field.
func paScanInstalls(fl *paFile, f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			var value ast.Expr
			flag := false
			for _, el := range node.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				if key.Name == "AmendLayout" {
					value = kv.Value
				}
				if key.Name == "pointerAmend" && isTrue(kv.Value) {
					flag = true
				}
			}
			if value != nil {
				paAddValue(fl, value, flag,
					fl.fset.Position(node.Pos()).Line)
			}
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "AmendLayout" {
					continue
				}
				if i < len(node.Rhs) {
					paAddValue(fl, node.Rhs[i], false,
						fl.fset.Position(node.Pos()).Line)
				}
			}
		}
		return true
	})
}

// paAddValue files the hook an install value names: a bare hook, an ad-hoc
// literal, a factory that returns the hook, or the members of amendAll.
func paAddValue(fl *paFile, value ast.Expr, flag bool, line int) {
	push := func(name string, factory bool) {
		if name == "" || name == "nil" {
			return
		}
		fl.install = append(fl.install, paInstall{
			name: name, factory: factory, file: fl, flag: flag, line: line,
		})
	}
	switch t := value.(type) {
	case *ast.Ident:
		push(t.Name, false)
	case *ast.FuncLit:
		fl.install = append(fl.install, paInstall{
			lit: t, file: fl, flag: flag, line: line,
		})
	case *ast.CallExpr:
		callee := paCallee(t, fl.imports)
		if callee == "amendAll" {
			for _, arg := range t.Args {
				switch a := arg.(type) {
				case *ast.Ident:
					push(a.Name, false)
				case *ast.FuncLit:
					fl.install = append(fl.install, paInstall{
						lit: a, file: fl, flag: flag, line: line,
					})
				case *ast.CallExpr:
					push(paCallee(a, fl.imports), true)
				}
			}
			return
		}
		// A bare factory call: only an unqualified name resolves
		// in-package, so a qualified one installs nothing visible.
		if _, ok := t.Fun.(*ast.Ident); ok {
			push(callee, true)
		}
	}
	// Any other shape (a func-typed field like fw.ring) is invisible to
	// the scan. The mode doc names this limit.
}
