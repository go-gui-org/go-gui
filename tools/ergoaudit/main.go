// Ergoaudit reports the API-surface measurements behind
// docs/specs/developer-ergonomics.md, so the spec's counts stay
// re-derivable as the code moves.
//
// Usage:
//
//	go run ./tools/ergoaudit/ -mode focus [repo...]
//	go run ./tools/ergoaudit/ -mode callbacks [repo...]
//
// With no repo arguments both modes audit the current directory.
//
// Mode focus answers: which focusable-by-default widget Cfgs leave ID
// unenforced, and how many call sites therefore render a control that
// is not keyboard-reachable. The unguarded Cfg set is derived from the
// go-gui source rather than hardcoded — a per-file scan for the ID tag
// picks the wrong struct in files declaring several Cfgs, which is how
// ListBoxCfg was once misreported as unguarded.
//
// Mode callbacks answers: how many distinct On* callback shapes the
// public API exposes, split by whether they carry an EventCtx, a bare
// *Window, or a raw *Event. Counts are reported both deduplicated and
// raw, because the two differ (OnEvent alone is declared twice) and
// quoting one without saying which is how review disagreements start.
//
// Both modes parse with go/ast: composite literals and func literals
// span lines, and regex cannot bracket-match them.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var listShape *string

func main() {
	mode := flag.String("mode", "focus", "audit to run: focus | callbacks")
	guiRoot := flag.String("gui", ".", "path to the go-gui repo (source of truth for mode=focus)")
	listShape = flag.String("list", "", "mode=callbacks: also list distinct signatures of this shape, or \"all\"")
	flag.Parse()

	repos := flag.Args()
	if len(repos) == 0 {
		repos = []string{"."}
	}

	var err error
	switch *mode {
	case "focus":
		err = runFocus(*guiRoot, repos)
	case "callbacks":
		err = runCallbacks(*guiRoot, repos)
	default:
		err = fmt.Errorf("unknown -mode %q (want focus or callbacks)", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ergoaudit:", err)
		os.Exit(1)
	}
}

// walkGo parses every non-vendored Go file under root and calls visit
// with the parsed file. Parse errors are skipped: a repo may legitimately
// hold files for another GOOS that do not typecheck here.
func walkGo(root string, visit func(path string, fset *token.FileSet, f *ast.File)) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}
		visit(path, fset, f)
		return nil
	})
}

// structName reports the type name of a composite literal, matching
// both qualified (gui.ButtonCfg{}) and bare (ButtonCfg{}) forms.
func structName(lit *ast.CompositeLit) string {
	switch t := lit.Type.(type) {
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

// litField returns the value expression for key in a composite literal.
func litField(lit *ast.CompositeLit, key string) (ast.Expr, bool) {
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
			return kv.Value, true
		}
	}
	return nil, false
}

// isTrue reports whether expr is the literal identifier true.
func isTrue(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "true"
}

// relPath trims root from path for readable output.
func relPath(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}
