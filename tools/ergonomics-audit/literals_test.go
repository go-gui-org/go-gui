package main

import (
	"go/parser"
	"go/token"
	"testing"
)

// scanLitSrc runs the literals-mode rules over one in-memory file and
// returns the findings as "kind:text" strings, so a case can be written
// as source.
func scanLitSrc(t *testing.T, rel, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found []litFinding
	inspectSelfFlaggedLiterals(fset, f, rel, &found)
	out := make([]string, len(found))
	for i, x := range found {
		out[i] = litKindName(x.kind) + ":" + x.text
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
	want := "Padding:Padding{...} Padding:gui.Padding{...} Padding:Padding{...}"
	if len(findings) != 3 || (findings[0]+" "+findings[1]+" "+findings[2]) != want {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
}

func TestLiteralsFlagsRawColor(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = Color{R: 80, G: 200, B: 80, A: 255}
var b = gui.Color{R: 1, G: 2, B: 3, A: 4}
var c = Color{80, 200, 80, 255, true}
var d = RGBA(80, 200, 80, 255)
var e = ColorTransparent
`
	findings := scanLitSrc(t, "x.go", src)
	want := "Color:Color{...} Color:gui.Color{...} Color:Color{...}"
	if len(findings) != 3 || (findings[0]+" "+findings[1]+" "+findings[2]) != want {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
}

func TestLiteralsFlagsRawSizing(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = Sizing{Width: sizingFill}
var b = gg.Sizing{Width: sizingFixed, Height: sizingFit}
var c = Sizing{}
var d = FitFit
var e = gg.FillFill
`
	findings := scanLitSrc(t, "x.go", src)
	want := "Sizing:Sizing{...} Sizing:gg.Sizing{...} Sizing:Sizing{...}"
	if len(findings) != 3 || (findings[0]+" "+findings[1]+" "+findings[2]) != want {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
}

func TestLiteralsExemptsEmptyColor(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = Color{}
var b = (gui.Color{})
`
	findings := scanLitSrc(t, "x.go", src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none (empty Color{} is the unset sentinel)", findings)
	}
}

func TestLiteralsIgnoresForeignColor(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = glyph.Color{R: 1, G: 2, B: 3, A: 4}
var b = SvgColor{R: 1, G: 2, B: 3, A: 4}
`
	findings := scanLitSrc(t, "x.go", src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none (foreign types are not go-gui colors)", findings)
	}
}

func TestLiteralsExemptsDefiningFiles(t *testing.T) {
	t.Parallel()
	paddingSrc := `package gui

var PaddingNone = Padding{set: true}
var paddingButton = PadAll(6)
`
	if f := scanLitSrc(t, "gui/padding.go", paddingSrc); len(f) != 0 {
		t.Fatalf("padding.go findings = %q, want none (defining file is exempt)", f)
	}
	colorSrc := `package gui

var Black = RGBA(0, 0, 0, 255)
`
	if f := scanLitSrc(t, "gui/color.go", colorSrc); len(f) != 0 {
		t.Fatalf("color.go findings = %q, want none (defining file is exempt)", f)
	}
}

func TestLiteralsNonSelfFlaggingLiteralsIgnored(t *testing.T) {
	t.Parallel()
	const src = `package p

var a = PaddingBox{Top: 1}
var b = Opt[Padding]{}
`
	findings := scanLitSrc(t, "x.go", src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none", findings)
	}
}
