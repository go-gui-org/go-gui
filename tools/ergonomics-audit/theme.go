package main

// Mode theme answers: does post-generation code still read the theme
// from the frame-scoped cache instead of naming its window?
//
// The theme is window-owned (docs/specs/per-window-theme.md). guiTheme
// and the ~30 default*Style package vars are a frame-scoped cache that
// FrameFn installs before the view function runs, so the ~420
// factory-time reads resolve to the right window without naming it.
// That is by design and must stay: Themed(t, build) scopes a theme to a
// subtree by push/pop of the *installed* theme, so a generation-time
// read that called w.Theme() would silently ignore the scope.
//
// Code that runs OUTSIDE generation has no such excuse. An event
// handler, a post-arrange injection, or a backend's clear-color read
// resolves against whichever window generated last — right by timing in
// a one-window app, wrong as soon as there are two. Issue #301 migrated
// those sites to w.Theme(); this mode keeps them migrated.
//
// Distinguishing "runs during generation" from "runs after it" is not
// decidable from one function's syntax, so the mode does not try. It
// scans only paths that are post-generation by construction:
//
//   - gui/backend/**    — runs after FrameFn, on the same thread
//   - gui/scroll*.go    — scroll math, driven by input events
//   - gui/event*.go     — dispatch and handlers
//   - gui/native_*.go   — native dialogs, print, export
//   - gui/window_*.go   — window lifecycle and update
//
// Within those, a finding is a read of guiTheme, CurrentTheme(), or a
// default*Style var from a function that has a *Window (or *gui.Window)
// receiver or parameter — that is, one that could name its window and
// did not.
//
// Known gap: handlers that live in a mixed-phase file (view_*.go holds
// both the factory and its handlers) are not scanned, because the same
// file's generation-time reads must stay bare. themePickerSyncHighlight,
// selectScrollTo and toastEnforceMaxVisible are migrated but ungated;
// the rule in CLAUDE.md covers them.
//
// A deliberate exception carries a same-line marker and prints as
// deferred rather than gating:
//
//	bg = gui.CurrentTheme().ColorBackground // ergonomics-audit:theme-global

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// themeGlobalMarker exempts one line: the read of the installed theme is
// deliberate, because the code genuinely wants whatever is installed
// rather than one window's theme.
const themeGlobalMarker = "ergonomics-audit:theme-global"

// themeStylePattern matches the default*Style / Default*Style mirrors
// that applyTheme writes. They are the same frame-scoped cache as
// guiTheme, one style struct at a time.
var themeStylePattern = regexp.MustCompile(`^[Dd]efault[A-Za-z]*Style$`)

// themeScannedPaths are the repo-relative paths that are
// post-generation by construction. A path outside this set is not
// scanned at all — see the mode comment for why the boundary cannot be
// computed from syntax.
func themeScanned(rel string) bool {
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "gui/") {
		return false
	}
	if strings.HasPrefix(rel, "gui/backend/") {
		return true
	}
	base := filepath.Base(rel)
	if filepath.Dir(rel) != "gui" {
		return false
	}
	switch {
	case strings.HasPrefix(base, "scroll"),
		strings.HasPrefix(base, "event"),
		strings.HasPrefix(base, "native_"),
		strings.HasPrefix(base, "window_"):
		return true
	}
	return false
}

// themeFinding is one frame-cache read from a function that had a
// window to name.
type themeFinding struct {
	path     string
	line     int
	fn       string
	read     string
	deferred bool // carried the marker: advisory, does not gate
}

// runTheme reports frame-cache theme reads in post-generation code. It
// returns an error when any unmarked finding survives, so the audit
// gates like modes ids, opt and literals.
func runTheme(repos []string) error {
	var all []themeFinding
	for _, repo := range repos {
		found, err := scanTheme(repo)
		if err != nil {
			return err
		}
		all = append(all, found...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].path != all[j].path {
			return all[i].path < all[j].path
		}
		return all[i].line < all[j].line
	})

	var gating int
	for _, f := range all {
		if !f.deferred {
			gating++
		}
	}

	fmt.Printf("Window-theme read audit (%d finding(s), %d deferred)\n",
		gating, len(all)-gating)
	for _, f := range all {
		tag := ""
		if f.deferred {
			tag = " [deferred]"
		}
		fmt.Printf("  %s:%d: %s reads %s with a window in hand%s\n",
			f.path, f.line, f.fn, f.read, tag)
	}
	if gating > 0 {
		fmt.Println()
		fmt.Println("Post-generation code with a *Window in hand reads w.Theme().")
		fmt.Printf("If the installed theme is genuinely what is wanted, mark the line %q.\n",
			themeGlobalMarker)
		return fmt.Errorf("%d frame-cache theme read(s)", gating)
	}
	return nil
}

func scanTheme(repo string) ([]themeFinding, error) {
	var out []themeFinding
	err := walkGo(filepath.Join(repo, "gui"), func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		rel := relPath(repo, path)
		if !themeScanned(rel) {
			return
		}
		marked := markedLines(fset, f, themeGlobalMarker)
		inspectTheme(fset, f, func(fn string, line int, read string) {
			out = append(out, themeFinding{
				path: rel, line: line, fn: fn, read: read,
				deferred: marked[line],
			})
		})
	})
	return out, err
}

// hasWindowParam reports whether fn could name a window: a receiver or
// parameter typed *Window or *gui.Window.
func hasWindowParam(fn *ast.FuncDecl) bool {
	if fn.Recv != nil {
		for _, fld := range fn.Recv.List {
			if isWindowPtr(fld.Type) {
				return true
			}
		}
	}
	if fn.Type != nil && fn.Type.Params != nil {
		for _, fld := range fn.Type.Params.List {
			if isWindowPtr(fld.Type) {
				return true
			}
		}
	}
	return false
}

// isWindowPtr reports whether a type expression is *Window (in package
// gui) or *gui.Window (in a backend package).
func isWindowPtr(typ ast.Expr) bool {
	star, ok := typ.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch t := star.X.(type) {
	case *ast.Ident:
		return t.Name == "Window"
	case *ast.SelectorExpr:
		pkg, ok := t.X.(*ast.Ident)
		return ok && pkg.Name == "gui" && t.Sel.Name == "Window"
	}
	return false
}

// themeRead names the frame-scoped cache an expression reads, or "" if
// it reads none.
func themeRead(n ast.Node) string {
	switch e := n.(type) {
	case *ast.Ident:
		if e.Name == "guiTheme" {
			return "guiTheme"
		}
		if themeStylePattern.MatchString(e.Name) {
			return e.Name
		}
	case *ast.CallExpr:
		switch fn := e.Fun.(type) {
		case *ast.Ident:
			if fn.Name == "CurrentTheme" {
				return "CurrentTheme()"
			}
		case *ast.SelectorExpr:
			pkg, ok := fn.X.(*ast.Ident)
			if ok && pkg.Name == "gui" && fn.Sel.Name == "CurrentTheme" {
				return "gui.CurrentTheme()"
			}
		}
	}
	return ""
}

// inspectTheme walks one parsed file and calls report for each
// frame-cache read inside a function that has a window in hand. Split
// out from scanTheme so the rules can be exercised against in-memory
// source in tests.
func inspectTheme(
	fset *token.FileSet, f *ast.File, report func(fn string, line int, read string),
) {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !hasWindowParam(fn) {
			continue
		}
		name := fn.Name.Name
		seen := map[int]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			read := themeRead(n)
			if read == "" {
				return true
			}
			line := fset.Position(n.Pos()).Line
			// One line can hold a call and its ident; report once.
			if seen[line] {
				return true
			}
			seen[line] = true
			report(name, line, read)
			return true
		})
	}
}
