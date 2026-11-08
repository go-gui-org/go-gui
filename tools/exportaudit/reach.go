package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// markReachable stamps a reach reference on every export that the public
// surface of its own package reaches by type, so that the gate judges
// orphans rather than payload shape (issue #1221).
//
// The closure starts at the package's exported functions, constants and
// variables and follows type names through signatures, exported struct
// fields (including callback fields), exported interface methods and the
// exported methods of every type it reaches. A type in the closure is
// public API whether or not any scan names it; so are its exported fields
// and methods and the exported constants of its type. The 2026-08 sweep
// (#230) only looked at function signatures, which is how
// DataGridDataSource's result structs lost every field a caller must fill.
//
// Exported types that nothing names stay outside the closure: a struct an
// app could build as a literal but never pass anywhere is still an orphan.
func markReachable(pkgs []guiPackage, exports map[string]*export) error {
	for _, p := range pkgs {
		files, err := parsePackageFiles(p.dir)
		if err != nil {
			return err
		}
		g := newReachGraph(files)
		g.mark(p.importPath, exports)
	}
	return nil
}

// parsePackageFiles parses the non-test Go files of one directory.
func parsePackageFiles(dir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if perr != nil {
			continue
		}
		files = append(files, f)
	}
	return files, nil
}

// valueDecl is one const or var spec with its resolved type expression
// (nil when untyped); implicit iota repetition inherits the group's type.
type valueDecl struct {
	names    []string
	typ      ast.Expr
	exported bool
}

type reachGraph struct {
	types   map[string]*ast.TypeSpec
	methods map[string][]*ast.FuncDecl // receiver base type → exported methods
	funcs   []*ast.FuncDecl            // exported top-level functions
	values  []valueDecl
}

func newReachGraph(files []*ast.File) *reachGraph {
	g := &reachGraph{types: map[string]*ast.TypeSpec{}, methods: map[string][]*ast.FuncDecl{}}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !ast.IsExported(d.Name.Name) {
					continue
				}
				if d.Recv == nil {
					g.funcs = append(g.funcs, d)
				} else if recv := recvTypeName(d.Recv); recv != "" {
					g.methods[recv] = append(g.methods[recv], d)
				}
			case *ast.GenDecl:
				g.addGen(d)
			}
		}
	}
	return g
}

func (g *reachGraph) addGen(d *ast.GenDecl) {
	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts := spec.(*ast.TypeSpec)
			g.types[ts.Name.Name] = ts
		}
	case token.CONST, token.VAR:
		var inherited ast.Expr
		for _, spec := range d.Specs {
			vs := spec.(*ast.ValueSpec)
			typ := vs.Type
			switch {
			case typ != nil:
				inherited = typ
			case d.Tok == token.CONST && len(vs.Values) == 0:
				typ = inherited // implicit repetition of the previous spec
			default:
				inherited = nil // untyped, or typed by its value
			}
			v := valueDecl{typ: typ}
			for _, n := range vs.Names {
				v.names = append(v.names, n.Name)
				v.exported = v.exported || ast.IsExported(n.Name)
			}
			g.values = append(g.values, v)
		}
	}
}

// closure returns the local type names reachable from the exported
// functions, constants and variables.
func (g *reachGraph) closure() map[string]bool {
	in := map[string]bool{}
	var queue []string
	addExpr := func(e ast.Expr) {
		names := map[string]bool{}
		typeIdents(e, names)
		for n := range names {
			if _, ok := g.types[n]; ok && !in[n] {
				in[n] = true
				queue = append(queue, n)
			}
		}
	}
	for _, f := range g.funcs {
		addExpr(f.Type)
	}
	for _, v := range g.values {
		if v.exported && v.typ != nil {
			addExpr(v.typ)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		ts := g.types[n]
		switch t := ts.Type.(type) {
		case *ast.StructType:
			for _, fld := range t.Fields.List {
				if len(fld.Names) == 0 || anyExported(fld.Names) {
					addExpr(fld.Type)
				}
			}
		case *ast.InterfaceType:
			for _, m := range t.Methods.List {
				if len(m.Names) == 0 || anyExported(m.Names) {
					addExpr(m.Type)
				}
			}
		default:
			addExpr(ts.Type)
		}
		for _, m := range g.methods[n] {
			addExpr(m.Type)
		}
	}
	return in
}

func anyExported(names []*ast.Ident) bool {
	for _, n := range names {
		if ast.IsExported(n.Name) {
			return true
		}
	}
	return false
}

// mark adds a reach ref to every export of pkgPath the closure covers:
// the reachable types, their exported fields and methods, and the
// exported constants and variables of a reachable type.
func (g *reachGraph) mark(pkgPath string, exports map[string]*export) {
	in := g.closure()
	bump := func(key string) {
		if e, ok := exports[key]; ok {
			e.refs[classReach]++
		}
	}
	fieldNames := map[string]bool{}
	for name := range in {
		bump(pkgPath + " " + name)
		ts := g.types[name]
		if st, ok := ts.Type.(*ast.StructType); ok {
			for _, fld := range st.Fields.List {
				for _, n := range fld.Names {
					if ast.IsExported(n.Name) {
						fieldNames[n.Name] = true
					}
				}
			}
		}
		for _, m := range g.methods[name] {
			bump(pkgPath + " " + m.Name.Name + " " + name)
		}
	}
	for name := range fieldNames {
		bump(pkgPath + " " + name + " *field")
	}
	for _, v := range g.values {
		if !v.exported || v.typ == nil {
			continue
		}
		names := map[string]bool{}
		typeIdents(v.typ, names)
		reachable := false
		for n := range names {
			if in[n] {
				reachable = true
				break
			}
		}
		if !reachable {
			continue
		}
		for _, n := range v.names {
			if ast.IsExported(n) {
				bump(pkgPath + " " + n)
			}
		}
	}
}

// typeIdents collects the bare identifiers a type expression names.
// Qualified names belong to another package and are skipped; parameter
// and field names are never visited, only their types.
func typeIdents(e ast.Expr, out map[string]bool) {
	switch t := e.(type) {
	case *ast.Ident:
		out[t.Name] = true
	case *ast.StarExpr:
		typeIdents(t.X, out)
	case *ast.ParenExpr:
		typeIdents(t.X, out)
	case *ast.ArrayType:
		typeIdents(t.Elt, out)
	case *ast.Ellipsis:
		typeIdents(t.Elt, out)
	case *ast.MapType:
		typeIdents(t.Key, out)
		typeIdents(t.Value, out)
	case *ast.ChanType:
		typeIdents(t.Value, out)
	case *ast.FuncType:
		fieldListTypes(t.Params, out)
		fieldListTypes(t.Results, out)
	case *ast.StructType:
		fieldListTypes(t.Fields, out)
	case *ast.InterfaceType:
		fieldListTypes(t.Methods, out)
	case *ast.IndexExpr:
		typeIdents(t.X, out)
		typeIdents(t.Index, out)
	case *ast.IndexListExpr:
		typeIdents(t.X, out)
		for _, i := range t.Indices {
			typeIdents(i, out)
		}
	}
}

func fieldListTypes(fl *ast.FieldList, out map[string]bool) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		typeIdents(f.Type, out)
	}
}
