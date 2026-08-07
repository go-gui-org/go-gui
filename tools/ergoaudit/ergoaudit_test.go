package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestBaseType pins the package-qualifier handling. Classifying on the
// rendered suffix alone reported *gg.Event as "not an event", which
// undercounted the raw-Event callbacks in the datagrid subpackage.
func TestBaseType(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"*Event":     "Event",
		"*gg.Event":  "Event",
		"*Window":    "Window",
		"*gg.Window": "Window",
		"EventCtx":   "EventCtx",
		"string":     "string",
		"[]GridRow":  "[]GridRow",
	}
	for in, want := range cases {
		if got := baseType(in); got != want {
			t.Errorf("baseType(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestReadCfgFocusPerStruct pins per-struct scanning. A per-file scan
// takes the first ID field it sees, which in a file declaring several
// Cfgs is the wrong struct — that is how ListBoxCfg was once reported
// as lacking the required tag it actually has.
func TestReadCfgFocusPerStruct(t *testing.T) {
	t.Parallel()
	const src = `package p

type ItemCfg struct {
	ID string
}

type ListBoxCfg struct {
	ID            string ` + "`" + `gui:"required"` + "`" + `
	FocusDisabled bool
}

type ColumnCfg struct {
	ID string
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var got cfgFocus
	var found bool
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "ListBoxCfg" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		got, found = readCfgFocus(ts.Name.Name, st)
		return false
	})

	if !found {
		t.Fatal("ListBoxCfg not recognised as taking part in focus")
	}
	if !got.defaultOn {
		t.Error("defaultOn = false, want true (has FocusDisabled)")
	}
	if !got.idRequired {
		t.Error(`idRequired = false, want true (ID has gui:"required")`)
	}
}

// TestClassifyFocusLiteral pins the three-way split, including the
// documented floor: a computed ID counts as present because whether it
// evaluates empty is a runtime question.
func TestClassifyFocusLiteral(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src  string
		want string
	}{
		{`ButtonCfg{ID: "save"}`, "withID"},
		{`ButtonCfg{ID: computed()}`, "withID"},
		{`ButtonCfg{ID: ""}`, "broken"},
		{`ButtonCfg{}`, "broken"},
		{`ButtonCfg{OnClick: h}`, "broken"},
		{`ButtonCfg{FocusDisabled: true}`, "optedOut"},
		{`ButtonCfg{ID: "", FocusDisabled: true}`, "optedOut"},
	}
	for _, c := range cases {
		expr, err := parser.ParseExpr(c.src)
		if err != nil {
			t.Fatalf("parse %s: %v", c.src, err)
		}
		lit, ok := expr.(*ast.CompositeLit)
		if !ok {
			t.Fatalf("%s is not a composite literal", c.src)
		}
		if got := classifyFocusLiteral(lit); got != c.want {
			t.Errorf("classifyFocusLiteral(%s) = %s, want %s", c.src, got, c.want)
		}
	}
}

// TestClassifyCallback pins the shape buckets that drive the §4.3
// convert-or-keep decision.
func TestClassifyCallback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src  string
		want callbackShape
	}{
		{"func(EventCtx)", shapeBareCtx},
		{"func(string, EventCtx)", shapePayload},
		{"func(*Window)", shapeWindow},
		{"func(float32, *Window)", shapeWindow},
		{"func(GridRow, *gg.Window) gg.View", shapeWindow},
		{"func(*Event, *Window)", shapeRawEvent},
		{"func(string, GridColumnPin, *gg.Event, *gg.Window)", shapeRawEvent},
		{"func(*DrawContext)", shapeOther},
	}
	fset := token.NewFileSet()
	for _, c := range cases {
		expr, err := parser.ParseExpr(c.src)
		if err != nil {
			t.Fatalf("parse %s: %v", c.src, err)
		}
		ft, ok := expr.(*ast.FuncType)
		if !ok {
			t.Fatalf("%s is not a func type", c.src)
		}
		if got := classifyCallback(fset, ft); got != c.want {
			t.Errorf("classifyCallback(%s) = %s, want %s",
				c.src, got.String(), c.want.String())
		}
	}
}

// TestRenderExprNormalises pins signature dedupe. gofmt aligns field
// types differently per struct, so deduping on source text counts one
// signature many times; the AST render must normalise that away.
func TestRenderExprNormalises(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	var rendered []string
	for _, src := range []string{"func(string,EventCtx)", "func(string, EventCtx)"} {
		expr, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("parse %s: %v", src, err)
		}
		rendered = append(rendered, renderExpr(fset, expr))
	}
	if rendered[0] != rendered[1] {
		t.Errorf("renderExpr did not normalise spacing: %q vs %q",
			rendered[0], rendered[1])
	}
	if !strings.Contains(rendered[0], "EventCtx") {
		t.Errorf("renderExpr lost type text: %q", rendered[0])
	}
}
