package main

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// callbackShape buckets one On* field declaration by what its last
// parameter carries.
type callbackShape int

const (
	shapeBareCtx     callbackShape = iota // func(EventCtx)
	shapePayload                          // func(T..., EventCtx)
	shapeWindow                           // func(T..., *Window)
	shapeRawEvent                         // mentions *Event in the parameter list
	shapeValueReturn                      // returns a value — fits neither target shape
	shapeOther                            // no trailing Window/EventCtx
)

func (s callbackShape) String() string {
	switch s {
	case shapeBareCtx:
		return "func(EventCtx)"
	case shapePayload:
		return "func(T..., EventCtx)"
	case shapeWindow:
		return "func(T..., *Window)"
	case shapeRawEvent:
		return "leaks raw *Event"
	case shapeValueReturn:
		return "returns a value"
	default:
		return "other"
	}
}

// allShapes is the report order. Value-returning callbacks come first
// because they are the ones no target signature accommodates.
var allShapes = []callbackShape{
	shapeValueReturn, shapeWindow, shapeRawEvent,
	shapeBareCtx, shapePayload, shapeOther,
}

// runCallbacks reports the On* callback surface of the go-gui packages,
// then per-repo call sites whose handler still takes a trailing
// *Window. Sibling repos declare their own On* fields mirroring the
// older convention; those are reported separately because renaming
// go-gui's signatures does not touch them.
func runCallbacks(guiRoot string, repos []string) error {
	decls, raw, err := scanCallbackDecls(filepath.Join(guiRoot, "gui"))
	if err != nil {
		return fmt.Errorf("scanning %s: %w", guiRoot, err)
	}

	byShape := map[callbackShape]int{}
	for _, sh := range decls {
		byShape[sh]++
	}
	fmt.Printf("On* callback declarations in %s/gui\n\n", guiRoot)
	fmt.Printf("  raw declarations (with duplicates): %d\n", raw)
	fmt.Printf("  distinct signatures:                %d\n\n", len(decls))
	for _, sh := range allShapes {
		fmt.Printf("  %-24s %d\n", sh.String(), byShape[sh])
	}
	fmt.Println()

	if *listShape != "" {
		for _, sh := range allShapes {
			if strings.EqualFold(sh.String(), *listShape) || *listShape == "all" {
				listSignatures(decls, sh)
			}
		}
	}

	// The go-gui repo is identified by path so unqualified literal
	// types can be attributed; every other repo's bare identifiers
	// name its own types.
	guiAbs, _ := filepath.Abs(guiRoot)
	for _, repo := range repos {
		repoAbs, _ := filepath.Abs(repo)
		if err := reportWindowTailSites(repo, repoAbs == guiAbs); err != nil {
			return err
		}
	}
	return nil
}

// scanCallbackDecls returns distinct On* field signatures mapped to
// their shape, plus the raw declaration count. Deduplication is by
// rendered signature text: OnEvent is declared twice, and several
// payload shapes repeat verbatim across widgets, so raw and distinct
// counts differ and must be quoted with their rule.
func scanCallbackDecls(root string) (map[string]callbackShape, int, error) {
	out := map[string]callbackShape{}
	raw := 0
	err := walkGo(root, func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		ast.Inspect(f, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, fld := range st.Fields.List {
				ft, isFunc := fld.Type.(*ast.FuncType)
				if !isFunc {
					continue
				}
				for _, name := range fld.Names {
					if !strings.HasPrefix(name.Name, "On") || !name.IsExported() {
						continue
					}
					raw++
					sig := name.Name + " " + renderFuncType(fset, ft)
					out[sig] = classifyCallback(fset, ft)
				}
			}
			return true
		})
	})
	return out, raw, err
}

// classifyCallback buckets a callback signature by its parameters.
//
// Results are checked before parameters. A callback that returns a
// value fits neither `func(EventCtx)` nor `func(T..., EventCtx)`, so
// it is a distinct migration problem regardless of what it takes —
// and bucketing it by its parameters hid exactly that. OnCellFormat
// and OnDetailRowView were counted as ordinary Window-tailed
// callbacks, and OnCopyRows as an ordinary Event-leaking one, so the
// audit could not surface the category it most needed to.
func classifyCallback(fset *token.FileSet, ft *ast.FuncType) callbackShape {
	if ft.Results != nil && len(ft.Results.List) > 0 {
		return shapeValueReturn
	}
	params := ft.Params.List
	if len(params) == 0 {
		return shapeOther
	}
	for _, p := range params {
		if baseType(renderExpr(fset, p.Type)) == "Event" {
			return shapeRawEvent
		}
	}
	switch baseType(renderExpr(fset, params[len(params)-1].Type)) {
	case "EventCtx":
		if len(params) == 1 && len(params[0].Names) <= 1 {
			return shapeBareCtx
		}
		return shapePayload
	case "Window":
		return shapeWindow
	default:
		return shapeOther
	}
}

// guiModulePath is the module prefix that marks a composite literal's
// type as belonging to go-gui rather than to the repo being scanned.
const guiModulePath = "github.com/go-gui-org/go-gui"

// reportWindowTailSites lists callback literals in one repo whose
// signature ends in *Window, split by which module declares the field.
//
// The split is the point. Without it the audit counts a sibling's own
// callbacks as go-gui migration work: go-map's
// mapview.InfoWindowAction.OnClick and go-edit's own OnFileDrop were
// both reported as sibling exposure, inflating the estimate with code
// that renaming go-gui's signatures cannot touch. Only the go-gui
// column is a migration cost.
func reportWindowTailSites(repo string, isGuiRepo bool) error {
	guiSites := map[string]int{}
	foreign := map[string]int{}
	guiTotal, foreignTotal := 0, 0

	err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		imports := importAliases(f)
		// ownedByGui reports whether a literal of type t declares its
		// fields in go-gui. An unqualified name is go-gui's only when
		// the repo under scan is go-gui itself.
		var ownedByGui func(t ast.Expr) bool
		ownedByGui = func(t ast.Expr) bool {
			switch v := t.(type) {
			case *ast.StarExpr:
				return ownedByGui(v.X)
			case *ast.Ident:
				return isGuiRepo
			case *ast.SelectorExpr:
				pkg, ok := v.X.(*ast.Ident)
				if !ok {
					return false
				}
				return strings.HasPrefix(imports[pkg.Name], guiModulePath)
			}
			return false
		}

		var visitLit func(lit *ast.CompositeLit, inherited ast.Expr)
		visitLit = func(lit *ast.CompositeLit, inherited ast.Expr) {
			// A nested literal may elide its type: []T{{...}} leaves
			// the inner literal's Type nil, which is precisely the
			// shape go-map's menu actions take.
			typ := lit.Type
			if typ == nil {
				typ = inherited
			}
			gui := ownedByGui(typ)
			child := elemType(typ)
			for _, el := range lit.Elts {
				if inner, ok := el.(*ast.CompositeLit); ok {
					visitLit(inner, child)
					continue
				}
				kv, isKV := el.(*ast.KeyValueExpr)
				if !isKV {
					ast.Inspect(el, inspectNested(visitLit))
					continue
				}
				if inner, ok := kv.Value.(*ast.CompositeLit); ok {
					visitLit(inner, nil)
				}
				key, isIdent := kv.Key.(*ast.Ident)
				if !isIdent || !strings.HasPrefix(key.Name, "On") {
					ast.Inspect(kv.Value, inspectNested(visitLit))
					continue
				}
				fn, isFunc := kv.Value.(*ast.FuncLit)
				if !isFunc {
					continue
				}
				// A callback body can hold further literals.
				ast.Inspect(fn.Body, inspectNested(visitLit))
				params := fn.Type.Params.List
				if len(params) == 0 {
					continue
				}
				last := renderExpr(fset, params[len(params)-1].Type)
				if baseType(last) != "Window" {
					continue
				}
				if gui {
					guiSites[key.Name]++
					guiTotal++
				} else {
					foreign[key.Name]++
					foreignTotal++
				}
			}
		}
		ast.Inspect(f, inspectNested(visitLit))
	})
	if err != nil {
		return fmt.Errorf("walking %s: %w", repo, err)
	}

	base := filepath.Base(repo)
	fmt.Printf("=== %s: callback literals with a trailing *Window ===\n", base)
	fmt.Printf("  go-gui fields (migration cost): %d\n", guiTotal)
	printCounts(guiSites)
	fmt.Printf("  fields declared elsewhere (not a cost): %d\n", foreignTotal)
	printCounts(foreign)
	fmt.Println()
	return nil
}

// inspectNested returns an ast.Inspect visitor that hands every
// top-level composite literal to visitLit and does not descend into it
// again — visitLit walks its own children so it can carry the elided
// element type down.
func inspectNested(visitLit func(*ast.CompositeLit, ast.Expr)) func(ast.Node) bool {
	return func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		visitLit(lit, nil)
		return false
	}
}

// elemType returns the element type of a slice, array or map type, so
// a nested literal that elided its own type can inherit it.
func elemType(t ast.Expr) ast.Expr {
	switch v := t.(type) {
	case *ast.ArrayType:
		return v.Elt
	case *ast.MapType:
		return v.Value
	}
	return nil
}

// importAliases maps each imported package's local name to its path.
func importAliases(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, im := range f.Imports {
		p, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			continue
		}
		name := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			name = p[i+1:]
		}
		if im.Name != nil {
			name = im.Name.Name
		}
		out[name] = p
	}
	return out
}

// printCounts writes a name/count block in descending count order.
func printCounts(m map[string]int) {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if m[names[i]] != m[names[j]] {
			return m[names[i]] > m[names[j]]
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		fmt.Println("    " + n + " " + strconv.Itoa(m[n]))
	}
}

// renderFuncType renders a func type using parameter and result types
// only, discarding parameter names.
//
// Deduplicating on the printed source instead counts
// `func(movedID, beforeID string, w *Window)` and
// `func(string, string, *Window)` as two distinct signatures when they
// are one type. That is where the "both OnReorder variants" claim came
// from: four declarations of a single signature, three of them named,
// reported as two. Naming is a documentation choice and must not move
// the count.
func renderFuncType(fset *token.FileSet, ft *ast.FuncType) string {
	var b strings.Builder
	b.WriteString("func(")
	b.WriteString(renderFieldTypes(fset, ft.Params))
	b.WriteString(")")
	if ft.Results != nil && len(ft.Results.List) > 0 {
		res := renderFieldTypes(fset, ft.Results)
		if len(ft.Results.List) > 1 || len(ft.Results.List[0].Names) > 0 {
			res = "(" + res + ")"
		}
		b.WriteString(" " + res)
	}
	return b.String()
}

// renderFieldTypes joins a field list's types, repeating a type once
// per name so `a, b string` renders as `string, string` rather than
// collapsing to one parameter.
func renderFieldTypes(fset *token.FileSet, fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		t := renderExpr(fset, f.Type)
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for range n {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ", ")
}

// renderExpr prints an AST expression back to source text.
func renderExpr(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, e); err != nil {
		return ""
	}
	return b.String()
}

// listSignatures prints every distinct signature of the given shape.
// Used to audit which callbacks legitimately carry no event.
func listSignatures(decls map[string]callbackShape, want callbackShape) {
	var out []string
	for sig, sh := range decls {
		if sh == want {
			out = append(out, sig)
		}
	}
	sort.Strings(out)
	fmt.Printf("\n--- %s (%d distinct) ---\n", want.String(), len(out))
	for _, s := range out {
		fmt.Println("  " + s)
	}
}

// baseType strips pointer and package qualifier from a rendered type,
// so *Event and *gg.Event both reduce to "Event". Matching on the
// rendered suffix alone misses the qualified form.
func baseType(s string) string {
	s = strings.TrimPrefix(s, "*")
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return s
}
