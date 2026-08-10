package main

import (
	"go/parser"
	"go/token"
	"testing"
)

// scanLitSrc runs the literals-mode rules over one in-memory file and
// returns the findings as "line:name" strings, so a case can be written
// as source.
func scanLitSrc(t *testing.T, rel, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found []litFinding
	inspectPaddingLiterals(fset, f, rel, &found)
	out := make([]string, len(found))
	for i, x := range found {
		out[i] = x.text
	}
	return out
}

func TestLiteralsFlagsRawPadding(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = Padding{Top: 1, Right: 2}
var b = gui.Padding{Left: 3}
var c = Padding{}
var d = NewPadding(1, 2, 3, 4)
var e = PaddingNone
`
	findings := scanLitSrc(t, "x.go", src)
	want := "Padding{...} gui.Padding{...} Padding{...}"
	if len(findings) != 3 || (findings[0]+" "+findings[1]+" "+findings[2]) != want {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
}

func TestLiteralsExemptsDefiningFile(t *testing.T) {
	t.Parallel()
	const src = `package gui

var PaddingNone = Padding{set: true}
var paddingButton = PadAll(6)
`
	findings := scanLitSrc(t, "gui/padding.go", src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none (defining file is exempt)", findings)
	}
}

func TestLiteralsNonPaddingLiteralsIgnored(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = Color{1, 2, 3, 4}
var b = PaddingBox{Top: 1}
var c = Opt[Padding]{}
`
	findings := scanLitSrc(t, "x.go", src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none", findings)
	}
}
