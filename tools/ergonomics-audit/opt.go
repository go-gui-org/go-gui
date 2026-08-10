package main

// Mode opt answers: which *Cfg fields are plain — not Opt[T] — but sit
// in a family whose zero value is a real user choice?
//
// The rule (CLAUDE.md, "Opt[T] vs plain fields"): use Opt[T] when the
// zero value is a legitimate user choice that must be distinguishable
// from "unset". A plain field silently collapses "unset" onto zero —
// SizeBorder is the worked example: a border width of 0 is something a
// caller means, so a plain float32 cannot tell "no border" from "not
// specified" and silently applies the theme default.
//
// The decision is made per field at authoring time, by a human, with
// no check; the documented failure mode — "copying whichever neighbor
// was nearest" — is exactly what this audit exists to catch. It flags
// plain fields whose name or type puts them in a zero-meaningful
// family:
//
//   - exact name Padding, Radius, Spacing, or Opacity
//   - name starting Size (SizeBorder, Size, ...)
//   - name of, starting, or ending with Align (HAlign, VAlign,
//     AlignButtons, ...) with a named enum type, whose zero is a real
//     member (HAlignStart == 0)
//
// and leaves the legitimate plain cases alone — most widths, heights,
// counts, and indices, where zero is not a meaningful choice.
//
// Three classes are exempt without a marker:
//
//   - Opt-wrapped fields (Opt[T], gg.Opt[T]) — the rule satisfied.
//   - nil-able fields (pointers, slices, maps, funcs, chans,
//     interfaces) — nil is already an explicit "unset".
//   - ThemeCfg — the theme is a concrete spec, not a widget knob:
//     zero padding/radius are real values (a dense theme), there is
//     no theme default for the theme itself to fall back to, and
//     ThemeMaker merges nothing behind the caller's back.
//
// A deliberate plain field carries a same-line marker so the decision
// is recorded where it lives:
//
//	Size float32 // ergonomics-audit:opt-plain — zero-size is meaningless; 0 means "use the default"
//
// Marked fields print as deferred (advisory) and do not gate; unmarked
// findings exit non-zero, so the audit gates like mode ids.
//
// Colors are out of scope by design: Color carries its own set flag
// (gui/color.go), so Opt[Color] would be wrong; Color fields never
// match the signals above. Padding is out of scope the same way since
// #243: Padding self-flags (gui/padding.go), so a plain Padding field
// already distinguishes "unset" from an explicit zero (PaddingNone).

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// optPlainMarker exempts one line: the plain field is deliberate, its
// zero value is the choice, and "unset" would add nothing. The marker
// must sit on the field's own line, like the ids-mode markers.
const optPlainMarker = "ergonomics-audit:opt-plain"

// themeCfgName is exempt from the rule: ThemeCfg is the theme's raw
// spec, where every value is concrete and zero is meaningful, and the
// theme is the thing other fields fall back to — it has no default of
// its own to silently apply.
const themeCfgName = "ThemeCfg"

// optFinding is one plain field in a zero-meaningful family.
type optFinding struct {
	path   string
	line   int
	cfg    string
	field  string
	typ    string
	signal string
}

// classifyOptField returns the signal a plain Cfg field matches, or ""
// if it is legitimately plain. Opt-wrapped and nil-able fields are
// handled by the caller.
func classifyOptField(name string, typ ast.Expr) string {
	switch {
	case name == "Padding" || name == "Radius" || name == "Spacing" || name == "Opacity":
		return "family"
	case strings.HasPrefix(name, "Size"):
		return "size"
	case name == "Align" || strings.HasPrefix(name, "Align") || strings.HasSuffix(name, "Align"):
		// Only a named enum type makes the zero a real member; a
		// plain numeric Align field would be a different mistake.
		if !namedType(typ) {
			return ""
		}
		return "align"
	}
	return ""
}

// basicTypes are the predeclared type names, which are Idents but not
// named types: an Align field typed float32 is not an enum.
var basicTypes = map[string]bool{
	"bool": true, "string": true, "rune": true, "byte": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "float32": true, "float64": true,
	"complex64": true, "complex128": true,
}

// namedType reports whether a field type is a named type — an ident
// that is not predeclared, or any qualified reference — rather than a
// bare basic type.
func namedType(typ ast.Expr) bool {
	switch t := typ.(type) {
	case *ast.SelectorExpr:
		return true
	case *ast.Ident:
		return !basicTypes[t.Name]
	}
	return false
}

// optSignalLabel renders a signal id as the one-line reason printed
// with the finding.
func optSignalLabel(sig string) string {
	switch sig {
	case "family":
		return "Padding/Radius/Spacing/Opacity: zero is a real value"
	case "size":
		return "Size*: zero is a real value"
	case "align":
		return "*Align enum: zero is a real member"
	}
	return sig
}

// runOpt reports every plain zero-meaningful Cfg field under each
// repo's gui/ tree. Findings gate: any unmarked field exits non-zero.
// Deliberately plain fields, marked with the opt-plain marker, are
// listed as deferred and do not gate.
func runOpt(repos []string) error {
	var findings, deferred []optFinding
	for _, repo := range repos {
		found, def, err := scanOptFields(repo)
		if err != nil {
			return err
		}
		findings = append(findings, found...)
		deferred = append(deferred, def...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].path != findings[j].path {
			return findings[i].path < findings[j].path
		}
		return findings[i].line < findings[j].line
	})
	sort.Slice(deferred, func(i, j int) bool {
		if deferred[i].path != deferred[j].path {
			return deferred[i].path < deferred[j].path
		}
		return deferred[i].line < deferred[j].line
	})

	fmt.Printf("Opt[T] vs plain-field audit (%d finding(s), %d deferred)\n\n",
		len(findings), len(deferred))
	for _, f := range findings {
		printOptFinding("", f)
	}
	if len(deferred) > 0 {
		fmt.Printf("\ndeferred (deliberately plain, marked %q): %d\n", optPlainMarker, len(deferred))
		for _, f := range deferred {
			printOptFinding("  ", f)
		}
	}
	if len(findings) > 0 {
		fmt.Println()
		fmt.Printf("Wrap the field in Opt[T] (construct with gui.Some), or, if plain is\n")
		fmt.Printf("deliberate, mark the field line %q.\n", optPlainMarker)
		return fmt.Errorf("%d plain zero-meaningful Cfg field(s)", len(findings))
	}
	return nil
}

func printOptFinding(indent string, f optFinding) {
	fmt.Printf("%s%s:%d: %s.%s %s — %s\n",
		indent, f.path, f.line, f.cfg, f.field, f.typ, optSignalLabel(f.signal))
}

// scanOptFields walks one repo's gui/ tree for plain zero-meaningful
// Cfg fields, split into gating findings and marked deferrals.
func scanOptFields(repo string) (findings, deferred []optFinding, err error) {
	err = walkGo(filepath.Join(repo, "gui"), func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		exempt := markedLines(fset, f, optPlainMarker)
		inspectOptFields(fset, f, relPath(repo, path), exempt, &findings, &deferred)
	})
	return findings, deferred, err
}

// inspectOptFields walks one parsed file and appends its findings and
// deferrals. Split out from scanOptFields so the rules can be
// exercised against in-memory source in tests.
func inspectOptFields(
	fset *token.FileSet, f *ast.File, rel string,
	exempt map[int]bool, findings, deferred *[]optFinding,
) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(ts.Name.Name, "Cfg") || ts.Name.Name == themeCfgName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, fld := range st.Fields.List {
				if len(fld.Names) == 0 {
					continue // embedded field: no name to decide on
				}
				if optWrapped(fld.Type) || nilableType(fld.Type) || paddingTyped(fld.Type) {
					continue
				}
				for _, n := range fld.Names {
					sig := classifyOptField(n.Name, fld.Type)
					if sig == "" {
						continue
					}
					f := optFinding{
						path:   rel,
						line:   fset.Position(fld.Pos()).Line,
						cfg:    ts.Name.Name,
						field:  n.Name,
						typ:    renderOptType(fld.Type),
						signal: sig,
					}
					if exempt[fset.Position(fld.Pos()).Line] {
						*deferred = append(*deferred, f)
					} else {
						*findings = append(*findings, f)
					}
				}
			}
		}
	}
}

// optWrapped reports whether a field type is Opt[T] — in-package
// (Opt[float32]) or qualified from the datagrid subpackage (gg.Opt).
func optWrapped(typ ast.Expr) bool {
	switch t := typ.(type) {
	case *ast.IndexExpr:
		return optName(t.X)
	case *ast.SelectorExpr:
		// gg.Opt is a selector, and Opt alone would be an ident, so
		// this covers a bare qualified reference.
		return optName(t)
	}
	return false
}

// optName reports whether an expression names Opt — either the bare
// ident or a qualified form like gg.Opt.
func optName(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == "Opt"
	case *ast.SelectorExpr:
		return e.Sel.Name == "Opt"
	}
	return false
}

// paddingTyped reports whether a field type is Padding — bare ident or
// qualified (gg.Padding). Padding self-flags (#243), so a plain Padding
// field is exempt from the Opt rule the same way Color is.
func paddingTyped(typ ast.Expr) bool {
	switch t := typ.(type) {
	case *ast.Ident:
		return t.Name == "Padding"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Padding"
	}
	return false
}

// nilableType reports whether a field type's zero is nil, which
// already distinguishes "unset" from a meaningful value — pointers,
// slices, maps, funcs, chans, and interfaces. Only value types need
// the Opt wrapper to carry that distinction.
func nilableType(typ ast.Expr) bool {
	switch typ.(type) {
	case *ast.StarExpr, *ast.ArrayType, *ast.MapType,
		*ast.FuncType, *ast.ChanType, *ast.InterfaceType:
		return true
	}
	return false
}

// renderOptType renders a field type for the report: plain idents and
// qualified names as written, Opt[T] as written.
func renderOptType(typ ast.Expr) string {
	switch t := typ.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return renderOptType(t.X) + "." + t.Sel.Name
	case *ast.IndexExpr:
		return renderOptType(t.X) + "[" + renderOptType(t.Index) + "]"
	case *ast.StarExpr:
		return "*" + renderOptType(t.X)
	}
	return "?"
}
