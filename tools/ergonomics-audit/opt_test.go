package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// scanOptSrc runs the opt-mode rules over one in-memory file and
// returns (findings, deferred) as rendered strings, so a case can be
// written as source.
func scanOptSrc(t *testing.T, src string) (findings, deferred []string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	exempt := markedLines(fset, f, optPlainMarker)
	var fnd, def []optFinding
	inspectOptFields(fset, f, "x.go", exempt, &fnd, &def)
	for _, x := range fnd {
		findings = append(findings, x.cfg+"."+x.field)
	}
	for _, x := range def {
		deferred = append(deferred, x.cfg+"."+x.field)
	}
	return findings, deferred
}

func TestOptFlagsPlainZeroMeaningfulFields(t *testing.T) {
	t.Parallel()
	const src = `package p

type WidgetCfg struct {
	Padding Opt[float32]
	Radius  float32
	Size    float32
	HAlign  HorizontalAlign
	AlignButtons HorizontalAlign
	Width   float32
	Count   int
}
`
	findings, deferred := scanOptSrc(t, src)
	want := []string{"WidgetCfg.Radius", "WidgetCfg.Size", "WidgetCfg.HAlign", "WidgetCfg.AlignButtons"}
	if strings.Join(findings, " ") != strings.Join(want, " ") {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
	if len(deferred) != 0 {
		t.Fatalf("deferred = %q, want none", deferred)
	}
}

func TestOptIgnoresOptWrappedPointerAndTheme(t *testing.T) {
	t.Parallel()
	const src = `package p

type WidgetCfg struct {
	Padding Opt[float32]
	Radius  gg.Opt[float32]
	Size    *float32
	Spacing Opt[Padding]
	Foo     SomeOther[float32]
}

type ThemeCfg struct {
	Padding Padding
	Radius  float32
	Size    float32
}
`
	findings, _ := scanOptSrc(t, src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none", findings)
	}
}

func TestOptFlagsVAlignAndQualifiedEnum(t *testing.T) {
	t.Parallel()
	const src = `package p

type ACfg struct {
	VAlign verticalAlign
	Align  gg.HorizontalAlign
}

type BCfg struct {
	Opacity float32
}
`
	findings, _ := scanOptSrc(t, src)
	want := []string{"ACfg.VAlign", "ACfg.Align", "BCfg.Opacity"}
	if strings.Join(findings, " ") != strings.Join(want, " ") {
		t.Fatalf("findings = %q, want %q", findings, want)
	}
}

func TestOptAlignRuleNeedsNamedEnumType(t *testing.T) {
	t.Parallel()
	const src = `package p

type ACfg struct {
	HAlign float32
}
`
	findings, _ := scanOptSrc(t, src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none (plain numeric Align is a different mistake)", findings)
	}
}

func TestOptMarkerDefersNotGates(t *testing.T) {
	t.Parallel()
	const src = `package p

type ACfg struct {
	Size float32 // ergonomics-audit:opt-plain — a zero-size spinner is meaningless; 0 means the default
	Radius float32
}
`
	findings, deferred := scanOptSrc(t, src)
	if len(findings) != 1 || findings[0] != "ACfg.Radius" {
		t.Fatalf("findings = %q, want [ACfg.Radius]", findings)
	}
	if len(deferred) != 1 || deferred[0] != "ACfg.Size" {
		t.Fatalf("deferred = %q, want [ACfg.Size]", deferred)
	}
}

func TestOptIgnoresNilableTypes(t *testing.T) {
	t.Parallel()
	const src = `package p

type WidgetCfg struct {
	Radius []float32
	Size   map[string]float32
	Spacing func() float32
	Opacity interface{}
	Padding chan float32
}
`
	findings, _ := scanOptSrc(t, src)
	if len(findings) != 0 {
		t.Fatalf("findings = %q, want none (nil already distinguishes unset)", findings)
	}
}

func TestOptIgnoresEmbeddedFields(t *testing.T) {
	t.Parallel()
	const src = `package p

type ACfg struct {
	Radius float32
	Size
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var findings, deferred []optFinding
	inspectOptFields(fset, f, "x.go", nil, &findings, &deferred)
	if len(findings) != 1 || findings[0].field != "Radius" {
		t.Fatalf("findings = %+v, want [Radius]", findings)
	}
	if len(deferred) != 0 {
		t.Fatalf("deferred = %+v, want none", deferred)
	}
}
