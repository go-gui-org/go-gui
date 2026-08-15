package main

import (
	"go/parser"
	"go/token"
	"testing"
)

// scanA11ySrc runs the a11y-mode rules over one in-memory file and
// returns the finding texts in source order, so a case can be written
// as source.
func scanA11ySrc(t *testing.T, rel, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found []a11yFinding
	inspectA11YCfgLiterals(fset, f, rel, &found)
	out := make([]string, len(found))
	for i, x := range found {
		out[i] = x.text
	}
	return out
}

func TestA11YFlagsLabelOnlyForwardingLiteral(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = A11YCfg{A11YLabel: cfg.A11YLabel}
var b = gui.A11YCfg{A11YLabel: cfg.A11YLabel}
var c = gg.A11YCfg{A11YLabel: a11yLabel(cfg.A11YLabel, "x")}
var d = A11YCfg{A11YLabel: cfg.A11YLabel, A11YDescription: "y"}
var e = A11YCfg{}
var f = A11YCfg{A11YLabel: "literal string"}
var g = A11YCfg{A11YLabel: row.Text}
var h = A11YCfg{A11YLabel: toastA11YLabel(toast)}
`
	findings := scanA11ySrc(t, "x.go", src)
	want := "A11YCfg{...} gui.A11YCfg{...} gg.A11YCfg{...}"
	if len(findings) != 3 || (findings[0]+" "+findings[1]+" "+findings[2]) != want {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
}

func TestA11YFlagsForeignTypeNever(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = glyph.A11YCfg{A11YLabel: cfg.A11YLabel}
var b = Widget{A11YCfg: gui.A11YCfg{A11YLabel: cfg.A11YLabel}}
`
	findings := scanA11ySrc(t, "x.go", src)
	if len(findings) != 1 {
		t.Fatalf("findings = %q, want the nested gui literal only", findings)
	}
	if findings[0] != "gui.A11YCfg{...}" {
		t.Fatalf("finding = %q, want gui.A11YCfg{...}", findings[0])
	}
}

func TestA11YSkipsDefinitionFile(t *testing.T) {
	t.Parallel()
	const src = `package gui

// doc example: A11YCfg{A11YLabel: "Save"}
var d = A11YCfg{A11YLabel: "Save"}
`
	findings := scanA11ySrc(t, a11yCfgDefFile, src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none (definition file is exempt)", findings)
	}
}

func TestA11YFindingsCarryLineNumbers(t *testing.T) {
	t.Parallel()
	const src = `package p

func f() {
	var x = A11YCfg{A11YLabel: cfg.A11YLabel}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found []a11yFinding
	inspectA11YCfgLiterals(fset, f, "x.go", &found)
	if len(found) != 1 || found[0].line != 4 {
		t.Fatalf("findings = %+v, want one finding at line 4", found)
	}
}
