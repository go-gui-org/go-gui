package main

// Mode ids answers: does anything still compose a widget ID by hand?
//
// gui.ScopeID / gui.ScopeIDN are the one way to build an inner
// widget's ID (see docs/specs/widget-id-scoping.md). Before them the
// codebase spelled the same idea five different ways, and one renderer
// forgot to scope at all, which is how two markdown views came to share
// heading IDs. A hand-rolled concatenation is how that returns.
//
// A finding is a string concatenation, or an fmt.Sprintf, that mentions
// a separator and either
//
//   - lands in an ID position — the value of an ...ID field in a Cfg
//     literal, the right-hand side of an assignment to an ...ID variable
//     or field, or the result of a function whose name ends in ID; or
//   - is built off an ID — its leftmost operand is a variable or field
//     named like one (cfg.ID, gridID, tipID).
//
// The second rule matters more than it looks. The consumers are what
// drift: w.IsFocus(cfg.ID+"_popup") rebuilds an ID that the producer
// has already moved, and it sits in a call argument rather than any
// position the first rule can name.
//
// Two kinds of site are exempt, each with its own marker so the reason
// stays legible:
//
//   - "ergoaudit:id-part" — the value is a leaf *part* fed into a
//     composition, not a scope. Because IDs are not escaped, a part must
//     not contain the separator, so "__src_o_" + index is correct rather
//     than a leftover.
//   - "ergoaudit:not-an-id" — the string is not a widget ID at all. An
//     export filename or a spreadsheet column name only looks like one.

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// idPartMarker exempts one line: the value it builds is a leaf part of
// an ID, not a composed ID, so it keeps its own spelling.
const idPartMarker = "ergoaudit:id-part"

// idNotAnIDMarker exempts one line: the string it builds is not a
// widget ID, it only occupies a name that looks like one.
const idNotAnIDMarker = "ergoaudit:not-an-id"

// idSeparators are the characters a hand-rolled ID composition joins
// with. A concatenation mentioning none of them is not composing an ID
// path (a label, a message, a URL), so it is left alone.
const idSeparators = ":._-/"

// idFinding is one hand-rolled composition in an ID position.
type idFinding struct {
	path string
	line int
	pos  string // what made this an ID position
	expr string // the offending expression, shortened
}

// runIDs reports hand-rolled ID compositions under each repo's gui/
// tree. It returns an error when any survive, so the audit gates.
func runIDs(repos []string) error {
	var all []idFinding
	for _, repo := range repos {
		found, err := scanIDs(repo)
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

	fmt.Printf("ID composition audit (%d finding(s))\n", len(all))
	for _, f := range all {
		fmt.Printf("  %s:%d: %s composed by hand: %s\n",
			f.path, f.line, f.pos, f.expr)
	}
	if len(all) > 0 {
		fmt.Println()
		fmt.Println("Compose inner IDs with gui.ScopeID / gui.ScopeIDN.")
		fmt.Printf("If the value is a leaf part rather than a scope, mark the line %q;\n",
			idPartMarker)
		fmt.Printf("if it is not a widget ID at all, mark it %q.\n", idNotAnIDMarker)
		return fmt.Errorf("%d hand-rolled ID composition(s)", len(all))
	}
	return nil
}

func scanIDs(repo string) ([]idFinding, error) {
	var out []idFinding
	err := walkGo(filepath.Join(repo, "gui"), func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		exempt := markedLines(fset, f, idPartMarker, idNotAnIDMarker)
		rel := relPath(repo, path)

		// One expression can satisfy two rules at once (a Cfg ID field
		// whose value is also composed off cfg.ID). Report it once,
		// under whichever rule named it first.
		seen := map[token.Pos]bool{}
		report := func(pos string, expr ast.Expr) {
			line := fset.Position(expr.Pos()).Line
			if exempt[line] || seen[expr.Pos()] {
				return
			}
			seen[expr.Pos()] = true
			out = append(out, idFinding{
				path: rel, line: line, pos: pos, expr: shortExpr(fset, expr),
			})
		}

		inspectIDs(fset, f, report)
	})
	return out, err
}

// isIDName reports whether a field, variable or function name denotes a
// widget ID. Bare "ID" counts; otherwise the name must end in "ID" with
// a preceding word boundary, so "Valid" and "UUID" do not match.
func isIDName(name string) bool {
	if name == "ID" || name == "id" {
		return true
	}
	if !strings.HasSuffix(name, "ID") || len(name) < 3 {
		return false
	}
	prev := name[len(name)-3]
	return prev < 'A' || prev > 'Z'
}

// idTargetName returns the ID-ish name an expression denotes, or "" if
// it denotes nothing that looks like a widget ID. Both a bare variable
// (gridID) and a field (cfg.ID, ts.popupID) count.
func idTargetName(fset *token.FileSet, expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		if isIDName(e.Name) {
			return e.Name
		}
	case *ast.SelectorExpr:
		if isIDName(e.Sel.Name) {
			return shortExpr(fset, e)
		}
	}
	return ""
}

// leftmostOperand returns the first operand of a + chain, which is the
// value everything else is appended to.
func leftmostOperand(expr ast.Expr) ast.Expr {
	for {
		bin, ok := expr.(*ast.BinaryExpr)
		if !ok || bin.Op != token.ADD {
			return expr
		}
		expr = bin.X
	}
}

// isHandRolled reports whether expr builds a string by concatenating a
// separator-bearing literal, or by fmt.Sprintf with a separator in the
// format. Concatenations with no literal at all (base + strconv.Itoa(i))
// are appends onto an already-composed ID, not compositions.
func isHandRolled(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return false
		}
		return concatHasSeparator(e)
	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Sprintf" {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "fmt" || len(e.Args) == 0 {
			return false
		}
		return hasSeparator(e.Args[0])
	}
	return false
}

// concatHasSeparator reports whether any operand of a + chain is a
// string literal containing a separator character.
func concatHasSeparator(expr ast.Expr) bool {
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		return concatHasSeparator(bin.X) || concatHasSeparator(bin.Y)
	}
	return hasSeparator(expr)
}

// hasSeparator reports whether expr is a string literal that contains
// one of idSeparators.
func hasSeparator(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return false
	}
	return strings.ContainsAny(s, idSeparators)
}

// markedLines returns the set of source lines carrying marker in a
// comment, so a deliberate exception can be annotated where it lives.
func markedLines(fset *token.FileSet, f *ast.File, markers ...string) map[int]bool {
	out := map[int]bool{}
	for _, group := range f.Comments {
		for _, c := range group.List {
			for _, marker := range markers {
				if strings.Contains(c.Text, marker) {
					out[fset.Position(c.Pos()).Line] = true
					break
				}
			}
		}
	}
	return out
}

// shortExpr renders expr as source, trimmed so one finding stays on one
// line even when the expression wraps.
func shortExpr(fset *token.FileSet, expr ast.Expr) string {
	s := strings.Join(strings.Fields(renderExpr(fset, expr)), " ")
	if len(s) > 70 {
		// Cut on a rune boundary: an ID literal may hold non-ASCII, and
		// slicing mid-rune would print replacement characters.
		cut := 67
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "..."
	}
	return s
}

// inspectIDs walks one parsed file and calls report for each
// hand-rolled ID composition it finds. Split out from scanIDs so the
// rules can be exercised against in-memory source in tests.
func inspectIDs(
	fset *token.FileSet, f *ast.File, report func(pos string, expr ast.Expr),
) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			// gui.ButtonCfg{ID: cfg.ID + ":x:" + k}
			if !strings.HasSuffix(structName(node), "Cfg") {
				return true
			}
			for _, el := range node.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || !isIDName(key.Name) {
					continue
				}
				if isHandRolled(kv.Value) {
					report("Cfg field "+key.Name, kv.Value)
				}
			}
		case *ast.AssignStmt:
			// animID := "prefix_" + id, and ts.popupID = id + "_popup"
			for i, lhs := range node.Lhs {
				name := idTargetName(fset, lhs)
				if name == "" || i >= len(node.Rhs) {
					continue
				}
				if isHandRolled(node.Rhs[i]) {
					report("assignment to "+name, node.Rhs[i])
				}
			}
		case *ast.BinaryExpr:
			// Anywhere at all, including call arguments: composing off
			// something already named like an ID.
			if !isHandRolled(node) {
				return true
			}
			if base := idTargetName(fset, leftmostOperand(node)); base != "" {
				report("value composed off "+base, node)
			}
		case *ast.FuncDecl:
			// func tabButtonID(...) string { return "tc_" + ... }
			if node.Name == nil || !isIDName(node.Name.Name) || node.Body == nil {
				return true
			}
			fnName := node.Name.Name
			ast.Inspect(node.Body, func(inner ast.Node) bool {
				ret, ok := inner.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for _, res := range ret.Results {
					if isHandRolled(res) {
						report("result of "+fnName, res)
					}
				}
				return true
			})
		}
		return true
	})
}
