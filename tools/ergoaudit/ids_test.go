package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"unicode/utf8"
)

// scanSrc runs the ids-mode rules over one in-memory file and returns
// the reported positions, so a case can be written as source.
func scanSrc(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	exempt := markedLines(fset, f, idPartMarker, idNotAnIDMarker)
	seen := map[token.Pos]bool{}
	var out []string
	report := func(pos string, expr ast.Expr) {
		if exempt[fset.Position(expr.Pos()).Line] || seen[expr.Pos()] {
			return
		}
		seen[expr.Pos()] = true
		out = append(out, pos+": "+shortExpr(fset, expr))
	}
	inspectIDs(fset, f, report)
	return out
}

func TestIDsFlagsHandRolledCompositions(t *testing.T) {
	t.Parallel()
	const src = `package p

type ButtonCfg struct{ ID string }

func f(cfg ButtonCfg, gridID string, ts *struct{ popupID string }) {
	_ = ButtonCfg{ID: cfg.ID + ":handle"}
	animID := gridID + "_indefinite"
	ts.popupID = gridID + "_popup"
	isFocus(cfg.ID + "_popup")
	_ = animID
}

func tabButtonID(controlID, tabID string) string {
	return "tc_" + controlID + "_" + tabID
}

func isFocus(string) {}
`
	got := scanSrc(t, src)
	if len(got) != 5 {
		t.Fatalf("got %d findings, want 5:\n%s", len(got), strings.Join(got, "\n"))
	}
}

// The consumer shapes are the ones that drift, and neither sits in a
// position the Cfg-field rule can name.
func TestIDsFlagsConsumerShapes(t *testing.T) {
	t.Parallel()
	const src = `package p

func f(cfg struct{ ID string }, ts *struct{ popupID string }) {
	isFocus(cfg.ID + "_popup")
	ts.popupID = cfg.ID + "_popup"
}

func isFocus(string) {}
`
	got := scanSrc(t, src)
	if len(got) != 2 {
		t.Errorf("got %d findings, want 2:\n%s", len(got), strings.Join(got, "\n"))
	}
}

// The Sprintf branch of isHandRolled had no coverage: a format string
// carrying a separator is a composition just as much as a + chain, and
// it is how two of the migrated sites used to be written.
func TestIDsFlagsSprintfComposition(t *testing.T) {
	t.Parallel()
	const src = `package p

import "fmt"

func f(cfgID string, idx int) {
	inputID := fmt.Sprintf("%s.rgb.%d", cfgID, idx)
	_ = inputID
}
`
	got := scanSrc(t, src)
	if len(got) != 1 {
		t.Errorf("got %d findings, want 1:\n%s", len(got), strings.Join(got, "\n"))
	}
}

// A Sprintf whose format holds no separator is building a label or a
// message, not an ID path, and must stay quiet.
func TestIDsIgnoresSprintfWithoutSeparator(t *testing.T) {
	t.Parallel()
	const src = `package p

import "fmt"

func f(gridID string, n int) {
	msg := fmt.Sprintf("%s has %d rows", gridID, n)
	_ = msg
}
`
	if got := scanSrc(t, src); len(got) != 0 {
		t.Errorf("want no findings, got:\n%s", strings.Join(got, "\n"))
	}
}

// shortExpr truncates long expressions for one-line output. Cutting at
// a fixed byte offset would split a multi-byte rune and print
// replacement characters, so the cut backs up to a rune boundary.
func TestShortExprTruncatesOnRuneBoundary(t *testing.T) {
	t.Parallel()
	// Leading ":" makes it a composition; the multi-byte padding puts a
	// rune astride the 67-byte cut.
	lit := `":` + strings.Repeat("é", 60) + `"`
	src := "package p\n\nfunc f(cfgID string) { _ = cfgID + " + lit + " }\n"

	got := scanSrc(t, src)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(got), strings.Join(got, "\n"))
	}
	if !utf8.ValidString(got[0]) {
		t.Errorf("truncated expression is not valid UTF-8: %q", got[0])
	}
}

func TestIDsIgnoresNonCompositions(t *testing.T) {
	t.Parallel()
	const src = `package p

import "strconv"

// ScopeID-composed values, appends onto an already-composed ID, and
// plain prose must all stay quiet.
func f(cfg struct{ ID string }, base string, i int) {
	_ = ScopeID(cfg.ID, "handle")
	_ = base + strconv.Itoa(i)
	msg := "widget " + cfg.ID + " is unreachable"
	_ = msg
}

func ScopeID(string, ...string) string { return "" }
`
	if got := scanSrc(t, src); len(got) != 0 {
		t.Errorf("want no findings, got:\n%s", strings.Join(got, "\n"))
	}
}

func TestIDsRespectsExemptionMarkers(t *testing.T) {
	t.Parallel()
	const src = `package p

import "strconv"

func rowID(i int) string {
	return "__src_o_" + strconv.Itoa(i) // ergoaudit:id-part
}

func colName(i int) string {
	id := "col_" + strconv.Itoa(i) // ergoaudit:not-an-id
	return id
}
`
	if got := scanSrc(t, src); len(got) != 0 {
		t.Errorf("want no findings, got:\n%s", strings.Join(got, "\n"))
	}
}

func TestIsIDName(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"ID":       true,
		"id":       true,
		"gridID":   true,
		"cfgID":    true,
		"popupID":  true,
		"ScrollID": true,
		"UUID":     false, // an all-caps acronym, not an ID field
		"Valid":    false,
		"width":    false,
	}
	for name, want := range cases {
		if got := isIDName(name); got != want {
			t.Errorf("isIDName(%q) = %v, want %v", name, got, want)
		}
	}
}
