package main

// Mode visual answers: does widget code still spell a dimming alpha or a
// type-size step as a literal, when the theme defines named roles for
// both?
//
// Issue #335 measured the divergence (docs/specs/widget-visual-consistency-
// audit.md): seven dimming values and three size-step systems coexisted,
// and nothing told an author which to use. The fix shipped named roles —
// the four TextStyle roles (gui/theme_text_roles.go), withRoleAlpha,
// TextStyleLabel's size step, PaddingField — and this mode keeps a literal
// from creeping back where a role is the one named source. A literal there
// is the divergence issue #335 exists to end.
//
// It scans only gui/view_*.go: that is the widget-authoring surface, where
// the audit says every decision was made per widget. theme_maker.go defines
// the roles and styles.go the mirrors; a literal there is a definition, not
// a divergence.
//
// Two rules:
//
//  1. Dimming literal — a numeric alpha in a color call: RGBA(r, g, b, N)
//     with 1 <= N <= 254, WithOpacity(f) with 0 < f < 1, or a uint8(N)
//     cast with 1 <= N <= 254. Opaque colors (alpha 255) do not dim and
//     pass; so does a named constant, like the swatch edge alpha.
//  2. Size-step literal — arithmetic on a text size that produces a new
//     text style: a TextStyle composite literal whose Size is X.Size + N,
//     an assignment to X.Size of the same shape, or the audit's "inline
//     floor" pattern f32Max(X.Size-N, M). Row heights and icon widths
//     derived from a text size (view_tree_rows, view_select) are geometry,
//     not type steps, and pass.
//
// A deliberate exception carries a same-line marker and prints as deferred
// rather than gating:
//
//	alpha := uint8(150) // ergonomics-audit:visual
//
// The audit's §1.2 ramps (date-picker roller distance, math-spinner ghost,
// non-text fills) carry it, and so do the two §2 inline size steps, which
// are tracked in the audit's "Left open" list rather than silently
// re-created.
import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// visualMarker exempts one line: the literal is a deliberate deviation
// (a ramp, a non-text fill, a tracked §2 size step), not a re-invented
// role.
const visualMarker = "ergonomics-audit:visual"

// visualFinding is one literal dimming alpha or size step.
type visualFinding struct {
	path     string
	line     int
	fn       string
	verb     string // what the literal does
	expr     string // the offending expression, one line
	deferred bool   // carried the marker: advisory, does not gate
}

// runVisual reports visual literals in gui/view_*.go. It returns an error
// when any unmarked finding survives, so the audit gates like modes ids,
// opt and literals.
func runVisual(repos []string) error {
	var all []visualFinding
	for _, repo := range repos {
		found, err := scanVisual(repo)
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

	fmt.Printf("Visual-literal audit (%d finding(s), %d deferred)\n",
		gating, len(all)-gating)
	for _, f := range all {
		tag := ""
		if f.deferred {
			tag = " [deferred]"
		}
		loc := f.path + ":" + strconv.Itoa(f.line) + ":"
		if f.fn != "" {
			loc += " " + f.fn
		}
		fmt.Printf("  %s %s %s%s\n", loc, f.verb, f.expr, tag)
	}
	if gating > 0 {
		fmt.Println()
		fmt.Println("A dimming alpha or type-size step is a theme role, not a")
		fmt.Println("widget-local literal. If this one is a deliberate deviation")
		fmt.Printf("(a ramp, a non-text fill, a tracked §2 size step), mark the line %q.\n",
			visualMarker)
		return fmt.Errorf("%d visual literal(s)", gating)
	}
	return nil
}

func scanVisual(repo string) ([]visualFinding, error) {
	var out []visualFinding
	err := walkGo(filepath.Join(repo, "gui"), func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		rel := relPath(repo, path)
		if filepath.Dir(rel) != "gui" || !strings.HasPrefix(filepath.Base(rel), "view_") {
			return
		}
		marked := markedLines(fset, f, visualMarker)
		inspectVisual(fset, f, func(fn string, line int, verb, expr string) {
			out = append(out, visualFinding{
				path: rel, line: line, fn: fn, verb: verb, expr: expr,
				deferred: marked[line],
			})
		})
	})
	return out, err
}

// inspectVisual walks one parsed file and calls report for each literal
// dimming alpha or size step, naming the enclosing function ("" for
// package-level code). Split out from scanVisual so the rules can be
// exercised against in-memory source in tests.
func inspectVisual(
	fset *token.FileSet, f *ast.File, report func(fn string, line int, verb, expr string),
) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			ast.Inspect(d.Body, func(n ast.Node) bool {
				return visualVisit(fset, d.Name.Name, n, report)
			})
		default:
			// Package-level code (a style builder var, a const) has no
			// function to name.
			ast.Inspect(d, func(n ast.Node) bool {
				return visualVisit(fset, "", n, report)
			})
		}
	}
}

// visualVisit reports one finding for n, if it is one, and reports
// whether to keep descending. Findings inside func literals are reported
// under their enclosing named function.
func visualVisit(
	fset *token.FileSet, fn string, n ast.Node, report func(fn string, line int, verb, expr string),
) bool {
	switch node := n.(type) {
	case *ast.CallExpr:
		switch {
		case dimAlphaCall(node):
			report(fn, fset.Position(node.Pos()).Line,
				"spells a dimming alpha", shortExpr(fset, node))
		case uint8AlphaCall(node):
			report(fn, fset.Position(node.Pos()).Line,
				"spells a dimming alpha", shortExpr(fset, node))
		case floorSizeStep(node):
			report(fn, fset.Position(node.Pos()).Line,
				"sizes a text step inline", shortExpr(fset, node))
		}
	case *ast.CompositeLit:
		if textStepLiteral(node) {
			report(fn, fset.Position(node.Pos()).Line,
				"sizes a text step inline", shortExpr(fset, node))
		}
	case *ast.AssignStmt:
		if sizeStepAssign(node) {
			report(fn, fset.Position(node.Pos()).Line,
				"sizes a text step inline", shortExpr(fset, node.Rhs[0]))
		}
	}
	return true
}

// dimAlphaCall reports RGBA(r, g, b, N) with a literal 1 <= N <= 254, or
// WithOpacity(f) with a literal 0 < f < 1: a dimming alpha spelled as a
// number where a theme role is the one named source.
func dimAlphaCall(call *ast.CallExpr) bool {
	name := callName(call)
	if name != "RGBA" && name != "WithOpacity" {
		return false
	}
	if name == "RGBA" {
		if len(call.Args) != 4 {
			return false
		}
		v, ok := intLiteral(call.Args[3])
		return ok && v >= 1 && v <= 254
	}
	if len(call.Args) != 1 {
		return false
	}
	f, ok := floatLiteral(call.Args[0])
	return ok && f > 0 && f < 1
}

// uint8AlphaCall reports a uint8(N) cast with a literal 1 <= N <= 254:
// the roller-ramp spelling of a dimming alpha.
func uint8AlphaCall(call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	if !ok || id.Name != "uint8" || len(call.Args) != 1 {
		return false
	}
	v, ok := intLiteral(call.Args[0])
	return ok && v >= 1 && v <= 254
}

// callName reports the bare name of a call: RGBA, gui.RGBA, WithOpacity.
func callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// intLiteral returns the value of a positive integer literal, or ok=false
// for anything else.
func intLiteral(e ast.Expr) (int, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	v, err := strconv.Atoi(lit.Value)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// floatLiteral returns the value of a non-negative float literal.
func floatLiteral(e ast.Expr) (float64, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || (lit.Kind != token.FLOAT && lit.Kind != token.INT) {
		return 0, false
	}
	f, err := strconv.ParseFloat(lit.Value, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return f, true
}

// sizeStepExpr reports an expression that is X.Size + N or X.Size - N: one
// type step taken by arithmetic instead of a named handle. The selector's
// final ident must be exactly "Size" (cfg.TextStyle.Size), so geometry
// fields named like HandleSize or ThumbSize do not match.
func sizeStepExpr(e ast.Expr) bool {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || (bin.Op != token.ADD && bin.Op != token.SUB) {
		return false
	}
	if _, ok := sizeSelector(bin.X); !ok {
		return false
	}
	lit, ok := bin.Y.(*ast.BasicLit)
	return ok && lit.Kind == token.INT
}

// sizeSelector reports whether e is a selector whose final ident is Size.
func sizeSelector(e ast.Expr) (string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Size" {
		return "", false
	}
	return sel.Sel.Name, true
}

// floorSizeStep reports f32Max(X.Size + N, floor) or f32Max(X.Size - N,
// floor): the audit's "inline floor" pattern, which produces a text size
// by arithmetic and then clamps it.
//
// The floor's spelling is not what makes this a finding — the arithmetic
// on the left is. Naming the floor (f32Max(X.Size-4, theme.N6.Size)) is
// an improvement, not a resolution, so the rule matches any floor
// expression rather than only a literal one. Keying on a literal floor
// let a named-floor rewrite silently leave the rule's view.
func floorSizeStep(call *ast.CallExpr) bool {
	if callName(call) != "f32Max" || len(call.Args) != 2 {
		return false
	}
	return sizeStepExpr(call.Args[0])
}

// textStepLiteral reports a composite literal whose Size key is X.Size + N
// arithmetic — a text style stepped by hand.
func textStepLiteral(lit *ast.CompositeLit) bool {
	expr, ok := litField(lit, "Size")
	if !ok {
		return false
	}
	return sizeStepExpr(expr)
}

// sizeStepAssign reports X.Size = X.Size + N, an assignment that steps a
// text style by arithmetic.
func sizeStepAssign(stmt *ast.AssignStmt) bool {
	if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return false
	}
	if _, ok := sizeSelector(stmt.Lhs[0]); !ok {
		return false
	}
	return sizeStepExpr(stmt.Rhs[0])
}
