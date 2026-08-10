package main

// Mode literals answers: does any code build a Padding or Color with a
// raw composite literal instead of a constructor?
//
// Both types self-flag (issue #243, gui/padding.go and gui/color.go):
// the unexported set field distinguishes "not set" from an explicit
// value, and a raw literal — even one with nonzero contents — reads as
// UNSET, silently falling through to the theme default. A literal is
// therefore always a bug: the caller meant the values it wrote. Build
// with NewPadding / PadAll / PaddingNone, or RGBA() / RGB() / Hex().
//
// The hazard is worst outside the package: a keyed gui.Padding{...} or
// gui.RGBA(..., 0, 0, 0) in an example compiles fine and silently takes the
// theme default. So this mode scans the whole repo tree — gui/,
// examples/, tests, datagrid — not just gui/ like the other modes.
//
// Exemptions, per type:
//
//   - The defining files (gui/padding.go, gui/color.go,
//     gui/color_set.go) — PaddingNone itself is a Padding{set: true}
//     literal there.
//   - The empty Color{} form: it is the explicit spelling of "unset"
//     (zero-sentinel comparisons like bg == (gui.Color{}) and passing
//     Color{} to an optional color parameter). It behaves exactly like
//     omitting the value, so there is nothing to convert.
//   - Qualified literals of a foreign Color/Padding: only the go-gui
//     package and its gg alias match; glyph.Color{...} in examples is
//     a different type and never flags.
//
// Findings exit non-zero, so the audit gates like modes ids and opt.

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
)

// Defining files are exempt from the literal check: their own
// Padding{set: true} / Color{...} literals are the types' only legal
// ones. In a sibling repo these files do not exist, so every literal
// of the type flags.
const (
	paddingDefFile  = "gui/padding.go"
	colorDefFile    = "gui/color.go"
	colorSetDefFile = "gui/color_set.go"
)

// litKind classifies a composite literal as one of the self-flagging
// types the mode guards, or litOther.
type litKind int

const (
	litOther litKind = iota
	litPadding
	litColor
)

// litFinding is one raw Padding/Color literal outside the defining
// files.
type litFinding struct {
	path string
	line int
	text string
	kind litKind
}

// runLiterals reports every raw Padding{...}/Color{...} composite
// literal in each repo's tree. Any finding exits non-zero.
func runLiterals(repos []string) error {
	var findings []litFinding
	for _, repo := range repos {
		found, err := scanSelfFlaggedLiterals(repo)
		if err != nil {
			return err
		}
		findings = append(findings, found...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].path != findings[j].path {
			return findings[i].path < findings[j].path
		}
		return findings[i].line < findings[j].line
	})

	fmt.Printf("Raw Padding/Color literal audit (%d finding(s))\n", len(findings))
	for _, f := range findings {
		fmt.Printf("%s:%d: raw %s literal %s — reads as UNSET and silently\n",
			f.path, f.line, litKindName(f.kind), f.text)
		fmt.Printf("    takes the theme default. Use %s.\n", litKindHint(f.kind))
	}
	if len(findings) > 0 {
		fmt.Println()
		return fmt.Errorf("%d raw %s literal(s)", len(findings), litKindList(findings))
	}
	return nil
}

// litKindName is the type name as printed in a finding.
func litKindName(k litKind) string {
	if k == litColor {
		return "Color"
	}
	return "Padding"
}

// litKindHint names the constructors for the type in a finding.
func litKindHint(k litKind) string {
	if k == litColor {
		return "RGBA() / RGB() / Hex()"
	}
	return "NewPadding / PadAll / PaddingNone"
}

// litKindList renders the distinct kinds found for the exit error.
func litKindList(findings []litFinding) string {
	seen := map[litKind]bool{}
	out := ""
	for _, f := range findings {
		if !seen[f.kind] {
			seen[f.kind] = true
			if out != "" {
				out += "/"
			}
			out += litKindName(f.kind)
		}
	}
	return out
}

// scanSelfFlaggedLiterals walks one repo's whole tree (not just gui/)
// for raw Padding{...}/Color{...} literals outside the defining files.
func scanSelfFlaggedLiterals(repo string) ([]litFinding, error) {
	var findings []litFinding
	err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		inspectSelfFlaggedLiterals(fset, f, relPath(repo, path), &findings)
	})
	return findings, err
}

// inspectSelfFlaggedLiterals walks one parsed file and appends
// findings. Split out from scanSelfFlaggedLiterals so the rules can be
// exercised against in-memory source in tests.
func inspectSelfFlaggedLiterals(
	fset *token.FileSet, f *ast.File, rel string, findings *[]litFinding,
) {
	if rel == paddingDefFile || rel == colorDefFile || rel == colorSetDefFile {
		return
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		kind := litKindOf(lit)
		if kind == litOther {
			return true
		}
		// Color{} with no elements is the explicit spelling of
		// "unset" — a zero-sentinel, not a value the caller meant.
		// It behaves like omitting the field, so it is not a bug.
		if kind == litColor && len(lit.Elts) == 0 {
			return true
		}
		*findings = append(*findings, litFinding{
			path: rel,
			line: fset.Position(lit.Pos()).Line,
			text: renderLit(lit),
			kind: kind,
		})
		return true
	})
}

// litKindOf classifies a composite literal type. Bare idents match
// (in-package gui code); qualified forms match only when the package
// part is gui or its gg alias — a foreign type like glyph.Color is a
// different type and never flags.
func litKindOf(lit *ast.CompositeLit) litKind {
	switch t := lit.Type.(type) {
	case *ast.Ident:
		switch t.Name {
		case "Padding":
			return litPadding
		case "Color":
			return litColor
		}
	case *ast.SelectorExpr:
		id, ok := t.X.(*ast.Ident)
		if !ok || (id.Name != "gui" && id.Name != "gg") {
			return litOther
		}
		switch t.Sel.Name {
		case "Padding":
			return litPadding
		case "Color":
			return litColor
		}
	}
	return litOther
}

// renderLit renders a composite literal compactly for the report:
// gui.Padding{...} for the selector form, Padding{...} for the bare form.
func renderLit(lit *ast.CompositeLit) string {
	name := structName(lit)
	var pkg string
	if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			pkg = id.Name + "."
		}
	}
	return pkg + name + "{...}"
}
