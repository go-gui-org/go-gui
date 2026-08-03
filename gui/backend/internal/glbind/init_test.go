//go:build !js && !darwin

package glbind

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// TestBindingsWellFormed checks the shape of the binding table. Every entry
// must be a pointer to a nil-able func variable, and no function pointer or
// entry-point name may appear twice — a duplicated pointer would leave one
// entry point permanently nil, a duplicated name would bind two different
// signatures to the same driver function.
func TestBindingsWellFormed(t *testing.T) {
	seenName := map[string]bool{}
	seenPtr := map[any]bool{}

	for _, b := range bindings() {
		if !strings.HasPrefix(b.name, "gl") {
			t.Errorf("%s: entry-point name must start with %q", b.name, "gl")
		}
		if seenName[b.name] {
			t.Errorf("%s: duplicate entry-point name", b.name)
		}
		seenName[b.name] = true

		if seenPtr[b.fptr] {
			t.Errorf("%s: duplicate function pointer", b.name)
		}
		seenPtr[b.fptr] = true

		v := reflect.ValueOf(b.fptr)
		if v.Kind() != reflect.Pointer {
			t.Fatalf("%s: fptr is %s, want pointer", b.name, v.Kind())
		}
		if v.Elem().Kind() != reflect.Func {
			t.Errorf("%s: fptr points at %s, want func", b.name, v.Elem().Kind())
		}
	}
}

// TestBindingNamesMatchVars is the check that matters most and the one a
// human reviewer is worst at: that {&pfnFoo, "glBar"} never slips through
// with Foo != Bar. Signatures are declared next to the variable but bound by
// the string, so a mismatched pair compiles, links, and then marshals a draw
// call's arguments into the wrong driver function at runtime.
//
// It re-parses this package's own source rather than using reflection,
// because Go offers no way to recover a package-level variable's name from
// its address.
func TestBindingNamesMatchVars(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "init.go", nil, 0)
	if err != nil {
		t.Fatalf("parse init.go: %v", err)
	}

	var checked int
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || len(lit.Elts) != 2 {
			return true
		}
		// &pfnActiveTexture
		unary, ok := lit.Elts[0].(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			return true
		}
		ident, ok := unary.X.(*ast.Ident)
		if !ok {
			return true
		}
		// "glActiveTexture"
		str, ok := lit.Elts[1].(*ast.BasicLit)
		if !ok || str.Kind != token.STRING {
			return true
		}

		varName := strings.TrimPrefix(ident.Name, "pfn")
		entryPoint := strings.TrimPrefix(strings.Trim(str.Value, `"`), "gl")
		if varName != entryPoint {
			t.Errorf("%s is bound to gl%s: variable and entry point disagree",
				ident.Name, entryPoint)
		}
		checked++
		return true
	})

	if want := len(bindings()); checked != want {
		t.Errorf("parsed %d binding literals, table has %d entries", checked, want)
	}
}

// TestInitReportsMissingSymbol covers the nil-proc path: a driver that does
// not export an entry point must produce a named error, not a nil-pointer
// panic on the first draw call.
func TestInitReportsMissingSymbol(t *testing.T) {
	err := InitWithProcAddrFunc(func(string) uintptr { return 0 })
	if err == nil {
		t.Fatal("InitWithProcAddrFunc(always 0) = nil, want error")
	}
	if !strings.Contains(err.Error(), "glActiveTexture") {
		t.Errorf("error %q does not name the missing symbol", err)
	}
}

// TestInitFailureLeavesNothingBound covers the two-pass contract: when a
// symbol is missing, resolution stops and nothing is registered, so a caller
// that mishandles the error can never observe a half-bound package. In the
// single-pass form, every entry point resolved before the failure would have
// stayed bound.
func TestInitFailureLeavesNothingBound(t *testing.T) {
	err := InitWithProcAddrFunc(func(name string) uintptr {
		if name == "glBindBuffer" {
			return 0
		}
		return 1
	})
	if err == nil {
		t.Fatal("InitWithProcAddrFunc = nil, want error")
	}
	if pfnActiveTexture != nil || pfnAttachShader != nil || pfnBindBuffer != nil {
		t.Fatal("failed init left function pointers bound")
	}
}

// TestTableCoversEveryPointer guards the completeness direction the other
// tests cannot see: a pfn variable (and its wrapper) added without a
// {&pfn, "gl..."} table entry would leave the wrapper permanently nil at
// runtime, and the table-shape tests cannot catch it because they never look
// at glbind.go. Re-parse both files and require every pfn name to appear
// exactly twice: once as a declaration, once as a table entry. (A table
// entry without a declaration cannot compile.)
func TestTableCoversEveryPointer(t *testing.T) {
	fset := token.NewFileSet()
	counts := map[string]int{}
	for _, f := range []string{"glbind.go", "init.go"} {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				// pfnActiveTexture func(...) — the declaration side.
				for _, name := range v.Names {
					if strings.HasPrefix(name.Name, "pfn") {
						counts[name.Name]++
					}
				}
			case *ast.UnaryExpr:
				// &pfnActiveTexture — the table-entry side.
				ident, ok := v.X.(*ast.Ident)
				if ok && v.Op == token.AND && strings.HasPrefix(ident.Name, "pfn") {
					counts[ident.Name]++
				}
			}
			return true
		})
	}
	for name, n := range counts {
		if n != 2 {
			t.Errorf("%s appears %d times, want 2 (var decl + table entry)", name, n)
		}
	}
}
