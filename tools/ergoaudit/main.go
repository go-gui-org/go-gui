// Ergoaudit reports the API-surface measurements behind
// docs/specs/developer-ergonomics.md, so the spec's counts stay
// re-derivable as the code moves.
//
// Usage:
//
//	go run ./tools/ergoaudit/ -mode focus [repo...]
//	go run ./tools/ergoaudit/ -mode callbacks [repo...]
//	go run ./tools/ergoaudit/ -mode ids [repo...]
//
// With no repo arguments both modes audit the current directory.
//
// Mode focus also rewrites. With -fix it inserts a generated ID into
// every literal it classifies as broken; -fix-dry-run reports those
// rewrites without performing them. -fix-only and -fix-exclude are
// regexps over repo-relative paths, applied in that order. Phase 1 of
// the spec runs it over this repo's tests and examples only, because
// go-gui's own widgets get hand-chosen IDs — a shipped widget's ID is
// public identity, not scaffolding:
//
//	go run ./tools/ergoaudit/ -mode focus -fix -fix-only '_test\.go$|^examples/' .
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
// Mode ids answers: does anything still compose a widget ID by hand,
// rather than through gui.ScopeID? It exits non-zero on any finding, so
// it gates. See ids.go for what counts and how to mark an exception.
//
// All modes parse with go/ast: composite literals and func literals
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
	"regexp"
	"strings"
)

var listShape *string

func main() {
	mode := flag.String("mode", "focus", "audit to run: focus | callbacks | ids")
	guiRoot := flag.String("gui", ".", "path to the go-gui repo (source of truth for mode=focus)")
	listShape = flag.String("list", "", "mode=callbacks: also list distinct signatures of this shape, or \"all\"")
	fix := flag.Bool("fix", false, "mode=focus: rewrite broken literals in place, adding a generated ID")
	fixDry := flag.Bool("fix-dry-run", false, "mode=focus: report what -fix would write, changing nothing")
	fixOnly := flag.String("fix-only", "",
		"mode=focus: regexp of repo-relative paths -fix may touch (default: all)")
	fixSkip := flag.String("fix-exclude", "",
		"mode=focus: regexp of repo-relative paths -fix must not touch")
	flag.Parse()

	only, err := compileFilter("-fix-only", *fixOnly)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ergoaudit:", err)
		os.Exit(1)
	}
	skip, err := compileFilter("-fix-exclude", *fixSkip)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ergoaudit:", err)
		os.Exit(1)
	}

	repos := flag.Args()
	if len(repos) == 0 {
		repos = []string{"."}
	}

	switch *mode {
	case "focus":
		err = runFocus(*guiRoot, repos, *fix || *fixDry, *fixDry, only, skip)
	case "callbacks":
		err = runCallbacks(*guiRoot, repos)
	case "ids":
		err = runIDs(repos)
	default:
		err = fmt.Errorf("unknown -mode %q (want focus, callbacks or ids)", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ergoaudit:", err)
		os.Exit(1)
	}
}

// compileFilter compiles an optional path filter, naming the flag in
// the error so a bad regexp says which one it came from.
func compileFilter(flagName, expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("bad %s: %w", flagName, err)
	}
	return re, nil
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
