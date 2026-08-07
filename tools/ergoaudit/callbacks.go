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
	shapeBareCtx  callbackShape = iota // func(EventCtx)
	shapePayload                       // func(T..., EventCtx)
	shapeWindow                        // func(T..., *Window)
	shapeRawEvent                      // mentions *Event in the parameter list
	shapeOther                         // no trailing Window/EventCtx
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
	default:
		return "other"
	}
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
	for _, sh := range []callbackShape{shapeBareCtx, shapePayload, shapeWindow, shapeRawEvent, shapeOther} {
		fmt.Printf("  %-24s %d\n", sh.String(), byShape[sh])
	}
	fmt.Println()

	if *listShape != "" {
		for _, sh := range []callbackShape{shapeWindow, shapeRawEvent, shapeOther, shapeBareCtx, shapePayload} {
			if strings.EqualFold(sh.String(), *listShape) || *listShape == "all" {
				listSignatures(decls, sh)
			}
		}
	}

	for _, repo := range repos {
		if err := reportWindowTailSites(repo); err != nil {
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
					sig := name.Name + " " + renderExpr(fset, ft)
					out[sig] = classifyCallback(fset, ft)
				}
			}
			return true
		})
	})
	return out, raw, err
}

// classifyCallback buckets a callback signature by its parameters.
func classifyCallback(fset *token.FileSet, ft *ast.FuncType) callbackShape {
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

// reportWindowTailSites lists callback literals in one repo whose
// signature ends in *Window, grouped by field name so lifecycle and
// animation callbacks (which legitimately carry no event) are visible
// as their own category.
func reportWindowTailSites(repo string) error {
	byName := map[string]int{}
	total := 0
	err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, el := range lit.Elts {
				kv, isKV := el.(*ast.KeyValueExpr)
				if !isKV {
					continue
				}
				key, isIdent := kv.Key.(*ast.Ident)
				if !isIdent || !strings.HasPrefix(key.Name, "On") {
					continue
				}
				fn, isFunc := kv.Value.(*ast.FuncLit)
				if !isFunc {
					continue
				}
				params := fn.Type.Params.List
				if len(params) == 0 {
					continue
				}
				if baseType(renderExpr(fset, params[len(params)-1].Type)) == "Window" {
					byName[key.Name]++
					total++
				}
			}
			return true
		})
	})
	if err != nil {
		return fmt.Errorf("walking %s: %w", repo, err)
	}

	fmt.Printf("=== %s: callback literals with a trailing *Window (%d) ===\n",
		filepath.Base(repo), total)
	names := make([]string, 0, len(byName))
	for k := range byName {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if byName[names[i]] != byName[names[j]] {
			return byName[names[i]] > byName[names[j]]
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		fmt.Println("  " + n + " " + strconv.Itoa(byName[n]))
	}
	fmt.Println()
	return nil
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
