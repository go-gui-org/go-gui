package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// reachFixture models the #1221 shape: a data-source interface whose
// result struct the app must fill, an enum set through a Cfg field, a
// method on a reachable type, and an orphan struct nothing names.
func reachFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, "go.mod", "module github.com/example/audit\n\ngo 1.26\n")
	mustWrite(t, root, "gui/reach.go", `package gui

// Source is implemented by apps.
type Source interface {
	Caps() Caps
}

// Caps is returned by Source.Caps; nothing in gui/ sets Supports.
type Caps struct {
	Supports bool
}

// Flip is a method on a reachable type.
func (c Caps) Flip() Caps { return Caps{Supports: !c.Supports} }

// Mode is set through Cfg.Mode.
type Mode uint8

// Mode values; ModeB is never mentioned inside gui/.
const (
	ModeA Mode = iota
	ModeB
)

// Cfg configures New.
type Cfg struct {
	Mode   Mode
	Source Source
	OnDone func(Result)
}

// Result reaches the app through the OnDone callback.
type Result struct {
	Count int
}

// New is the entry point.
func New(cfg Cfg) {}

// Orphan is exported but no exported signature, field or constant names it.
type Orphan struct {
	Hidden int
}

// Sentinel is a var of a non-local type; it needs a real reference.
var Sentinel = 1
`)
	return root
}

func TestReachClassification(t *testing.T) {
	root := reachFixture(t)
	pkgs, err := guiPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	exports, err := collectExports(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanUsage(root, nil, pkgs, exports); err != nil {
		t.Fatal(err)
	}
	if err := markReachable(pkgs, exports); err != nil {
		t.Fatal(err)
	}
	const pkg = "github.com/example/audit/gui"
	want := map[string]class{
		pkg + " Caps":            classReach, // result of an exported interface method
		pkg + " Supports *field": classReach, // field of a payload struct
		pkg + " Flip Caps":       classReach, // method on a reachable type
		pkg + " Mode":            classReach, // type of an exported Cfg field
		pkg + " ModeB":           classReach, // enum member of a reachable type
		pkg + " Result":          classReach, // callback parameter
		pkg + " Count *field":    classReach,
		pkg + " Orphan":          classNone,
		pkg + " Hidden *field":   classNone,
		pkg + " Sentinel":        classNone,
	}
	for key, wantClass := range want {
		e := exports[key]
		if e == nil {
			t.Errorf("%s: export missing", key)
			continue
		}
		if got := e.maxClass(); got != wantClass {
			t.Errorf("%s: maxClass = %s, want %s", key, className[got+1], className[wantClass+1])
		}
	}
	failures, _ := gate(exports, true)
	for _, name := range []string{"Caps", "Supports", "Flip", "Mode", "ModeB", "Result", "Count"} {
		if strings.Contains(failures, " "+name) {
			t.Errorf("gate flags reachable export %s:\n%s", name, failures)
		}
	}
	for _, name := range []string{"Orphan", "Hidden", "Sentinel"} {
		if !strings.Contains(failures, name) {
			t.Errorf("gate misses orphan %s:\n%s", name, failures)
		}
	}
}

func TestReachIgnoresParameterNames(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "go.mod", "module github.com/example/audit\n\ngo 1.26\n")
	// The parameter is named like the orphan type; names are not types.
	mustWrite(t, root, "gui/names.go", `package gui

// Orphan must stay unreachable.
type Orphan struct{ X int }

// Run names a parameter Orphan but takes an int.
func Run(Orphan int) {}
`)
	pkgs, err := guiPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	exports, err := collectExports(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if err := markReachable(pkgs, exports); err != nil {
		t.Fatal(err)
	}
	if e := exports["github.com/example/audit/gui Orphan"]; e == nil || e.maxClass() != classNone {
		t.Errorf("Orphan reached through a parameter name")
	}
	_ = filepath.Join
}
