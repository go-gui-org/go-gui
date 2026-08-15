package main

// Mode a11y answers: does any code build an A11YCfg composite literal
// that forwards the label but drops the description?
//
// A11YCfg (gui/a11y_cfg.go) is the embed that every widget Cfg carries
// its accessible name and description in. Widgets that reach the
// accessibility tree forward the pair into an inner shape's ContainerCfg
// — the issue #316 class of bug was ten widgets whose forwarding site
// spelled A11YCfg{A11YLabel: a11yLabel(...)} and silently omitted
// A11YDescription. The compiler accepts it, nothing warns, and the
// description never reaches the accessibility tree: a screen-reader
// user hears the label and nothing else.
//
// A keyed A11YCfg literal is a *forwarding* site when its label comes
// from the caller — spelled as a selector into the widget's own cfg
// (cfg.A11YLabel, itemCfg.A11YLabel, a11yLabel(cfg.A11YLabel, ...)).
// Dropping A11YDescription there is never legitimate: the caller's
// description either has a value to carry or is empty and costs
// nothing. (An unkeyed A11YCfg{} is a different thing — the explicit
// "no a11y fields" embed — and a literal whose label is content-derived
// — row.Text, col.Title, toastA11YLabel(toast) — has no caller-supplied
// description to forward, so neither flags.) This mode makes the
// forwarding class of bug a compile-time gate.
//
// Findings exit non-zero, so the audit gates like modes ids and opt.
// The whole tree is scanned — examples and tests build the same
// literals as gui/ — except the definition file gui/a11y_cfg.go.
//
// Spelling notes: A11YCfg: cfg.A11YCfg is a value copy, not a
// composite literal, so it never flags (that is the correct spelling
// where no label fallback exists). At a forwarding site a deliberately
// description-less literal is still a bug by the rule above — write
// the key explicitly with the empty string if one must be sentinel.
// Known gap: a positional A11YCfg{label} literal drops the description
// with no key to inspect; the repo's keyed-literal convention keeps
// this out of the code, so the mode does not chase it.

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
)

// a11yCfgDefFile is the definition of the type; its own literals (in
// doc comments) are the type's canonical spelling, never forwarding
// sites.
const a11yCfgDefFile = "gui/a11y_cfg.go"

// a11yFinding is one A11YCfg literal that forwards a label but drops
// the description.
type a11yFinding struct {
	path string
	line int
	text string
}

// runA11Y reports every forwarding A11YCfg literal — one whose label
// is sourced from a caller's .A11YLabel — that omits the
// A11YDescription key, in each repo's tree. Any finding exits non-zero.
func runA11Y(repos []string) error {
	var findings []a11yFinding
	for _, repo := range repos {
		found, err := scanA11YCfgLiterals(repo)
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

	fmt.Printf("A11YCfg forwarding audit (%d finding(s))\n", len(findings))
	for _, f := range findings {
		fmt.Printf("%s:%d: %s forwards the label but drops A11YDescription —\n",
			f.path, f.line, f.text)
		fmt.Println("    the description never reaches the accessibility tree.")
	}
	if len(findings) > 0 {
		fmt.Println()
		return fmt.Errorf("%d A11YCfg literal(s) drop the description", len(findings))
	}
	return nil
}

// scanA11YCfgLiterals walks one repo's whole tree for forwarding
// A11YCfg literals missing the A11YDescription key.
func scanA11YCfgLiterals(repo string) ([]a11yFinding, error) {
	var findings []a11yFinding
	err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		inspectA11YCfgLiterals(fset, f, relPath(repo, path), &findings)
	})
	return findings, err
}

// inspectA11YCfgLiterals walks one parsed file and appends findings.
// Split out from scanA11YCfgLiterals so the rules can be exercised
// against in-memory source in tests.
func inspectA11YCfgLiterals(
	fset *token.FileSet, f *ast.File, rel string, findings *[]a11yFinding,
) {
	if rel == a11yCfgDefFile {
		return
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if !isA11YCfgLiteral(lit) {
			return true
		}
		// A11YCfg{} with no elements is the explicit "no a11y fields"
		// embed — nothing is forwarded, so nothing can be dropped.
		if len(lit.Elts) == 0 {
			return true
		}
		if hasA11YDescriptionKey(lit) {
			return true
		}
		// Only forwarding literals flag: the label must come from a
		// caller's .A11YLabel selector (cfg.A11YLabel, itemCfg.A11YLabel,
		// a11yLabel(cfg.A11YLabel, ...)). A content-derived label —
		// row.Text, col.Title, toastA11YLabel(toast) — has no caller
		// description to forward, so it never flags.
		if !forwardsCallerLabel(lit) {
			return true
		}
		*findings = append(*findings, a11yFinding{
			path: rel,
			line: fset.Position(lit.Pos()).Line,
			text: renderLit(lit),
		})
		return true
	})
}

// isA11YCfgLiteral reports whether a composite literal names A11YCfg.
// Bare idents match (in-package gui code); qualified forms match only
// when the package part is gui or its gg alias — a foreign type of
// the same name is a different type and never flags.
func isA11YCfgLiteral(lit *ast.CompositeLit) bool {
	switch t := lit.Type.(type) {
	case *ast.Ident:
		return t.Name == "A11YCfg"
	case *ast.SelectorExpr:
		id, ok := t.X.(*ast.Ident)
		if !ok || (id.Name != "gui" && id.Name != "gg") {
			return false
		}
		return t.Sel.Name == "A11YCfg"
	}
	return false
}

// hasA11YDescriptionKey reports whether the literal names the
// A11YDescription field, whether or not it is empty.
func hasA11YDescriptionKey(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok &&
			id.Name == "A11YDescription" {
			return true
		}
	}
	return false
}

// forwardsCallerLabel reports whether any label expression in the
// literal references a caller's A11YLabel — a selector whose Sel is
// exactly A11YLabel (cfg.A11YLabel, itemCfg.A11YLabel), including
// inside a fallback call like a11yLabel(cfg.A11YLabel, ...). Such a
// literal forwards caller identity, so it must forward the caller's
// description too.
func forwardsCallerLabel(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); !ok || id.Name != "A11YLabel" {
			continue
		}
		found := false
		ast.Inspect(kv.Value, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok &&
				sel.Sel.Name == "A11YLabel" {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}
