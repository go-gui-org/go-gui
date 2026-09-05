package main

// Mode deadcfg answers: is an exported *Cfg field actually consumed by
// anything, or does the caller set it into a void?
//
// This is the gate that #503 needed and did not have.
// NumericStepCfg.Keyboard and NumericStepCfg.MouseWheel shipped for
// months as fields a caller could set with no effect whatsoever. A
// plain "nothing references it" check does not find them, because
// numericStepCfgNormalize *does* read cfg.MouseWheel — only to write it
// straight back into another NumericStepCfg that nobody inspects. The
// value moves; it never arrives anywhere that acts on it.
//
// So the analysis classifies every read, not merely counts them:
//
//   - FORWARDING — the value is copied into a field of the SAME NAME,
//     either as a keyed value in a composite literal
//     (MouseWheel: cfg.MouseWheel) or as dst.Field = src.Field. This is
//     the "copy into another struct of the same shape" case: the value
//     moved, but nothing looked at it.
//   - CONSUMING — everything else. A condition, a call argument,
//     arithmetic, a return, or a copy into a differently-named field.
//
// The same-name restriction is what keeps the mode honest. A copy into a
// different name is a real use: mono.Family = cfg.MonoFontFamily builds
// a font spec that the theme then renders with, and field.initialValue =
// cfg.InitialValue hands the value to form state that reads it back. An
// earlier draft treated every dst.X = src.Y as a forward and reported
// six such fields as dead. They were not.
//
// A forward is not evidence of use. Because a same-name copy always
// lands back on the same field name, and field identity here IS the
// name (below), forwarding can never make a field live no matter how
// many hops it takes: NumericStepCfg.MouseWheel is copied from one
// NumericStepCfg into the next forever and arrives nowhere. So a field
// is live exactly when it has one consuming read, and the mode needs no
// reachability pass — only the discipline not to count a forward.
//
// # Field identity is the field NAME
//
// Every other mode in this tool parses with go/ast alone, and this one
// keeps that. Without go/types a selector cannot be resolved to the
// struct it belongs to, so all fields sharing a name are treated as one
// field. The error is one-directional and worth stating plainly:
//
//   - Never a false alarm from this. A field is only cleared when a
//     read somewhere genuinely consumes that name.
//   - Silence is not proof. A dead ListBoxCfg.Scrollable hides behind a
//     live TableCfg.Scrollable, because the two names are one name here.
//
// So a finding is worth acting on, and a clean run means "nothing
// uniquely-named is dead" — not "nothing is dead". Widening this needs
// go/types and a package load, which is a different tool.
//
// # What counts as a read
//
// Declarations are collected from gui/ only. Reads are collected from
// the whole tree EXCEPT _test.go files: a field whose only consumer is
// its own test is not wired into the product, which is the bug class
// being hunted. Composite-literal keys (MouseWheel: true in an example)
// are writes, not reads, and correctly contribute nothing — a field the
// world sets and no one reads is the definition of dead.
//
// A whole-struct copy (b := a) is invisible to this analysis, which
// only makes it quieter, never louder.
//
// # Marking an exception
//
// Put "ergonomics-audit:deadcfg-keep <reason>" in the field's doc
// comment or on its own line. Marked fields print as deferred and do not
// gate. The reason is the point: a field kept for a sibling repo to read
// back should say so.
//
// Unmarked findings exit non-zero, so this gates like modes ids, opt,
// literals and a11y.

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// deadCfgMarker exempts a field the audit would otherwise report. It may
// sit in the field's doc comment or on the field's own line, matching
// the exportaudit:keep style these Cfg structs already use.
const deadCfgMarker = "ergonomics-audit:deadcfg-keep"

// deadCfgField is one exported field declared on a gui/ *Cfg type.
type deadCfgField struct {
	path   string
	line   int
	cfg    string
	field  string
	typ    string
	reason string // text after the marker, for a deferred field
	marked bool
}

// deadCfgAnalysis accumulates declarations and classified reads across
// every file in the tree. It is a type rather than a pile of maps so a
// test can feed it in-memory files in any order and then ask for the
// findings, the same way the real two-pass walk does.
type deadCfgAnalysis struct {
	// declared holds every exported gui/ *Cfg field, in source order.
	declared []deadCfgField
	// consuming holds field names with at least one read that acts on
	// the value. A field is live exactly when it appears here.
	consuming map[string]bool
}

func newDeadCfgAnalysis() *deadCfgAnalysis {
	return &deadCfgAnalysis{consuming: map[string]bool{}}
}

// runDeadCfg reports every exported gui/ *Cfg field that no code
// consumes. Unmarked findings exit non-zero.
func runDeadCfg(repos []string) error {
	var findings, deferred []deadCfgField
	for _, repo := range repos {
		found, def, err := scanDeadCfg(repo)
		if err != nil {
			return err
		}
		findings = append(findings, found...)
		deferred = append(deferred, def...)
	}
	sortDeadCfg(findings)
	sortDeadCfg(deferred)

	fmt.Printf("Unconsumed Cfg field audit (%d finding(s), %d deferred)\n\n",
		len(findings), len(deferred))
	for _, f := range findings {
		printDeadCfgFinding("", f)
	}
	if len(deferred) > 0 {
		fmt.Printf("\ndeferred (deliberately kept, marked %q): %d\n",
			deadCfgMarker, len(deferred))
		for _, f := range deferred {
			printDeadCfgFinding("  ", f)
		}
	}
	if len(findings) > 0 {
		fmt.Println()
		fmt.Printf("Each field above can be set by a caller and changes nothing.\n")
		fmt.Printf("Wire it up or delete it; if it is read outside this module,\n")
		fmt.Printf("mark the field %q with the reason.\n", deadCfgMarker)
		return fmt.Errorf("%d unconsumed Cfg field(s)", len(findings))
	}
	return nil
}

func sortDeadCfg(fs []deadCfgField) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].path != fs[j].path {
			return fs[i].path < fs[j].path
		}
		return fs[i].line < fs[j].line
	})
}

func printDeadCfgFinding(indent string, f deadCfgField) {
	fmt.Printf("%s%s:%d: %s.%s %s — set by callers, consumed by nothing\n",
		indent, f.path, f.line, f.cfg, f.field, f.typ)
	if f.reason != "" {
		fmt.Printf("%s    kept: %s\n", indent, f.reason)
	}
}

// scanDeadCfg walks one repo twice: gui/ for the field declarations,
// then the whole tree for reads of them. Reads have to be collected
// after the declarations, because a selector is only interesting when
// its name matches a declared field.
func scanDeadCfg(repo string) (findings, deferred []deadCfgField, err error) {
	a := newDeadCfgAnalysis()

	err = walkGo(filepath.Join(repo, "gui"), func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		a.collectFields(fset, f, relPath(repo, path))
	})
	if err != nil {
		return nil, nil, err
	}

	err = walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		// A field read only by its own test is not wired into the
		// product, which is the case this mode exists to catch.
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		a.collectReads(f)
	})
	if err != nil {
		return nil, nil, err
	}

	found, def := a.findings()
	return found, def, nil
}

// collectFields records every exported field of every *Cfg type in one
// gui/ file.
func (a *deadCfgAnalysis) collectFields(fset *token.FileSet, f *ast.File, rel string) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(ts.Name.Name, "Cfg") {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, fld := range st.Fields.List {
				// An embedded field has no name of its own; its
				// members are reached through the outer type and are
				// audited where they are declared.
				if len(fld.Names) == 0 {
					continue
				}
				reason, marked := deadCfgReason(fld)
				for _, n := range fld.Names {
					if !n.IsExported() {
						continue
					}
					a.declared = append(a.declared, deadCfgField{
						path:   rel,
						line:   fset.Position(fld.Pos()).Line,
						cfg:    ts.Name.Name,
						field:  n.Name,
						typ:    deadCfgType(fld.Type),
						reason: reason,
						marked: marked,
					})
				}
			}
		}
	}
}

// deadCfgType names a field's type for the report. renderOptType covers
// the shapes the opt mode cares about and prints "?" for the rest, but a
// callback field is exactly the shape that goes dead unnoticed, so the
// composite kinds get named here.
func deadCfgType(typ ast.Expr) string {
	switch t := typ.(type) {
	case *ast.FuncType:
		return "func(...)"
	case *ast.ArrayType:
		return "[]" + deadCfgType(t.Elt)
	case *ast.MapType:
		return "map[" + deadCfgType(t.Key) + "]" + deadCfgType(t.Value)
	case *ast.InterfaceType:
		return "interface{...}"
	case *ast.StarExpr:
		return "*" + deadCfgType(t.X)
	}
	return renderOptType(typ)
}

// deadCfgReason reports whether a field carries the keep marker, and
// the reason text following it.
func deadCfgReason(fld *ast.Field) (string, bool) {
	for _, group := range []*ast.CommentGroup{fld.Doc, fld.Comment} {
		if group == nil {
			continue
		}
		for _, c := range group.List {
			idx := strings.Index(c.Text, deadCfgMarker)
			if idx < 0 {
				continue
			}
			rest := c.Text[idx+len(deadCfgMarker):]
			return strings.TrimSpace(strings.TrimLeft(rest, " \t-—:")), true
		}
	}
	return "", false
}

// collectReads walks one file and classifies every selector that names
// a declared Cfg field. The walk carries an explicit parent stack
// because classification is a question about a node's context, and
// ast.Inspect alone does not expose the parent.
func (a *deadCfgAnalysis) collectReads(f *ast.File) {
	names := map[string]bool{}
	for _, d := range a.declared {
		names[d.field] = true
	}

	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			// Every non-nil visit pushes exactly once and always
			// returns true, so the stack is never empty here; the
			// guard keeps a future early-return from panicking.
			if len(stack) == 0 {
				return true
			}
			stack = stack[:len(stack)-1]
			return true
		}
		defer func() { stack = append(stack, n) }()

		sel, ok := n.(*ast.SelectorExpr)
		if !ok || !names[sel.Sel.Name] {
			return true
		}
		a.classify(sel, stack)
		return true
	})
}

// classify records one selector as a consuming read, a forward, or
// nothing at all (a write).
func (a *deadCfgAnalysis) classify(sel *ast.SelectorExpr, stack []ast.Node) {
	if len(stack) == 0 {
		a.consuming[sel.Sel.Name] = true
		return
	}
	parent := stack[len(stack)-1]

	switch p := parent.(type) {
	case *ast.KeyValueExpr:
		// A key is not a read at all; a value is.
		if p.Key == ast.Expr(sel) {
			return
		}
		if p.Value != ast.Expr(sel) {
			break
		}
		if key, ok := p.Key.(*ast.Ident); ok && key.Name == sel.Sel.Name {
			return // forwarded, not consumed
		}
	case *ast.AssignStmt:
		if dst, isWrite := assignRole(p, sel); isWrite {
			return // the field is being written, not read
		} else if dst == sel.Sel.Name {
			return // forwarded, not consumed
		}
	}
	a.consuming[sel.Sel.Name] = true
}

// assignRole reports what sel is doing in an assignment: a write when it
// is on the left, otherwise the name of the field it is copied into when
// the matching left-hand side is itself a field selector. An empty
// destination with isWrite false means the read is consuming.
func assignRole(as *ast.AssignStmt, sel *ast.SelectorExpr) (dst string, isWrite bool) {
	if slices.Contains(as.Lhs, ast.Expr(sel)) {
		return "", true
	}
	// Positional pairing only holds when the arity matches; a multi-value
	// call on the right has no per-name correspondence.
	if len(as.Lhs) != len(as.Rhs) {
		return "", false
	}
	for i, rhs := range as.Rhs {
		if rhs != ast.Expr(sel) {
			continue
		}
		if lsel, ok := as.Lhs[i].(*ast.SelectorExpr); ok {
			return lsel.Sel.Name, false
		}
	}
	return "", false
}

// findings splits the declared fields into gating findings and marked
// deferrals, after resolving liveness.
func (a *deadCfgAnalysis) findings() (gating, deferred []deadCfgField) {
	for _, d := range a.declared {
		if a.consuming[d.field] {
			continue
		}
		if d.marked {
			deferred = append(deferred, d)
		} else {
			gating = append(gating, d)
		}
	}
	return gating, deferred
}
