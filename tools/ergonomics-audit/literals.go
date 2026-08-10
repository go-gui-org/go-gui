package main

// Mode literals answers: does any code build a Padding with a raw
// composite literal instead of a constructor?
//
// Padding self-flags (issue #243, gui/padding.go): the unexported set
// field distinguishes "not set" from an explicit value, and a raw
// Padding{...} literal — even one with nonzero sides — reads as UNSET,
// silently falling through to the theme default. A literal is therefore
// always a bug: the caller meant the sides it wrote. Build with
// NewPadding / PadAll / PaddingNone instead.
//
// The hazard is worst outside the package: a keyed gui.Padding{...} in
// an example compiles fine and silently takes the theme default. So
// this mode scans the whole repo tree — gui/, examples/, tests,
// datagrid — not just gui/ like the other modes.
//
// The one exempt file is the type's defining file (gui/padding.go):
// PaddingNone itself is a Padding{set: true} literal there. In a
// sibling repo that file does not exist, so every Padding{...} flags.
//
// Findings exit non-zero, so the audit gates like modes ids and opt.

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
)

// paddingDefFile is the file that defines Padding and is exempt from
// the literal check: its own Padding{set: true} literal is the type's
// only legal one.
const paddingDefFile = "gui/padding.go"

// litFinding is one raw Padding literal outside the defining file.
type litFinding struct {
	path string
	line int
	text string
}

// runLiterals reports every raw Padding{...} composite literal in each
// repo's tree. Any finding exits non-zero.
func runLiterals(repos []string) error {
	var findings []litFinding
	for _, repo := range repos {
		found, err := scanPaddingLiterals(repo)
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

	fmt.Printf("Raw Padding literal audit (%d finding(s))\n", len(findings))
	for _, f := range findings {
		fmt.Printf("%s:%d: raw Padding literal %s — reads as UNSET and silently\n", f.path, f.line, f.text)
		fmt.Printf("    takes the theme default. Use NewPadding / PadAll / PaddingNone.\n")
	}
	if len(findings) > 0 {
		fmt.Println()
		return fmt.Errorf("%d raw Padding literal(s)", len(findings))
	}
	return nil
}

// scanPaddingLiterals walks one repo's whole tree (not just gui/) for
// Padding{...} composite literals outside gui/padding.go.
func scanPaddingLiterals(repo string) ([]litFinding, error) {
	var findings []litFinding
	err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		inspectPaddingLiterals(fset, f, relPath(repo, path), &findings)
	})
	return findings, err
}

// inspectPaddingLiterals walks one parsed file and appends findings.
// Split out from scanPaddingLiterals so the rules can be exercised
// against in-memory source in tests.
func inspectPaddingLiterals(
	fset *token.FileSet, f *ast.File, rel string, findings *[]litFinding,
) {
	if rel == paddingDefFile {
		return
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || structName(lit) != "Padding" {
			return true
		}
		*findings = append(*findings, litFinding{
			path: rel,
			line: fset.Position(lit.Pos()).Line,
			text: renderLit(lit),
		})
		return true
	})
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
