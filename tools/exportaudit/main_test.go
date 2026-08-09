package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes a miniature go-gui tree into t.TempDir() and returns its
// root. The gui package declares an exported helper used only internally
// (sweepTarget) and one used by the consumer (keepTarget); the consumer
// imports gui and calls keepTarget.
func fixture(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	mustWrite(t, root, "go.mod", "module github.com/example/audit\n\ngo 1.26\n")
	mustWrite(t, root, "gui/exported.go", `package gui

// SweepTarget is exported but referenced only inside gui/.
func SweepTarget() {}

// KeepTarget is exported and referenced by the consumer.
func KeepTarget() int { return 1 }

// LocalName collides with nothing; referenced only via its own package.
const LocalName = 1
`)
	mustWrite(t, root, "gui/self_test.go", `package gui

import "testing"

func TestSelf(t *testing.T) {
	SweepTarget() // selftest reference only
}
`)
	mustWrite(t, root, "cons/use.go", `package main

import "github.com/example/audit/gui"

func main() {
	_ = gui.KeepTarget()
}
`)
	return root
}

func mustWrite(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFixtureClassification(t *testing.T) {
	root := fixture(t)

	pkgs, err := guiPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	exports, err := collectExports(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanUsage(root, []string{filepath.Join(root, "cons")}, pkgs, exports); err != nil {
		t.Fatal(err)
	}

	keep := exports["github.com/example/audit/gui KeepTarget"]
	sweep := exports["github.com/example/audit/gui SweepTarget"]
	local := exports["github.com/example/audit/gui LocalName"]
	if keep == nil || sweep == nil || local == nil {
		t.Fatalf("exports missing: keep=%v sweep=%v local=%v", keep != nil, sweep != nil, local != nil)
	}
	if keep.maxClass() != classConsumer {
		t.Errorf("KeepTarget maxClass = %v, want consumer", keep.maxClass())
	}
	if sweep.maxClass() != classSelfTest {
		t.Errorf("SweepTarget maxClass = %v, want selftest", sweep.maxClass())
	}
	if local.maxClass() != classNone {
		t.Errorf("LocalName maxClass = %v, want none", local.maxClass())
	}
}

func TestGateRuns(t *testing.T) {
	root := fixture(t)

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

	// Without the consumer, SweepTarget and LocalName are internal-only and
	// must trip the gate; KeepTarget is unreferenced here but is a function
	// that the gate treats like any other — mark it keep to see the marker
	// path works.
	exports["github.com/example/audit/gui KeepTarget"].keep = true

	failures, advisory := gate(exports, true)
	if failures == "" {
		t.Fatal("gate passed with internal-only exports")
	}
	if !strings.Contains(failures, "SweepTarget") || !strings.Contains(failures, "LocalName") {
		t.Errorf("gate output missing offenders:\n%s", failures)
	}
	if strings.Contains(failures, "KeepTarget") {
		t.Errorf("keep-marked export listed as offender:\n%s", failures)
	}
	if advisory != "" {
		t.Errorf("fixture has no internal-class exports, got advisory:\n%s", advisory)
	}
}

func TestKeySeparatesFuncFromMethod(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "go.mod", "module github.com/example/audit\n\ngo 1.26\n")
	mustWrite(t, root, "gui/decls.go", `package gui

// SetLocale is a package-level function.
func SetLocale() {}

// Window is a type with a same-named method.
type Window struct{}

// SetLocale is a method on Window, distinct from the function.
func (w *Window) SetLocale() {}
`)
	pkgs, err := guiPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	exports, err := collectExports(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != 3 { // SetLocale func, Window type, (Window) SetLocale method
		t.Errorf("got %d exports, want 3 (func, type, method)", len(exports))
	}
	var methodFound bool
	for _, e := range exports {
		if e.kind == kindMethod && e.name == "SetLocale" {
			methodFound = true
		}
	}
	if !methodFound {
		t.Error("method (Window) SetLocale missing; func/method key collision")
	}
}

func TestSweepRenamesAndKeeps(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "go.mod", "module github.com/example/audit\n\ngo 1.26\n")
	mustWrite(t, root, "gui/decls.go", `package gui

// SweepMe is exported but referenced only inside gui/.
func SweepMe() int { return 1 }

// KeepMe is exported and referenced by the consumer.
func KeepMe() int { return 2 }

// UseSweep calls SweepMe from the same package.
func UseSweep() int { return SweepMe() }

// Widget has an exported field and method used only internally.
type Widget struct{ Visible bool }

func (w *Widget) Hide() { w.Visible = false }
`)
	mustWrite(t, root, "gui/decls_test.go", `package gui

import "testing"

func TestSweepMe(t *testing.T) {
	if SweepMe() != 1 {
		t.Fatal("SweepMe broken")
	}
}
`)
	mustWrite(t, root, "cons/use.go", `package main

import "github.com/example/audit/gui"

func main() { _ = gui.KeepMe() }
`)

	pkgs, err := guiPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	exports, err := collectExports(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanUsage(root, []string{filepath.Join(root, "cons")}, pkgs, exports); err != nil {
		t.Fatal(err)
	}
	if err := sweep(root, pkgs, exports); err != nil {
		t.Fatal(err)
	}

	src := readFile(t, filepath.Join(root, "gui", "decls.go"))
	for _, want := range []string{"sweepMe", "KeepMe", "useSweep", "widget", "hide", "visible"} {
		if !strings.Contains(src, want) {
			t.Errorf("decls.go missing %q after sweep:\n%s", want, src)
		}
	}
	// Comments keep their prose; only code identifiers must be lowercase.
	for _, bad := range []string{"func SweepMe", "func UseSweep", "type Widget", "func (w *widget) Hide", "Visible bool"} {
		if strings.Contains(src, bad) {
			t.Errorf("decls.go still has %q after sweep:\n%s", bad, src)
		}
	}
	testSrc := readFile(t, filepath.Join(root, "gui", "decls_test.go"))
	if !strings.Contains(testSrc, "sweepMe()") {
		t.Errorf("test file not updated:\n%s", testSrc)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
