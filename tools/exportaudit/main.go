// Exportaudit classifies every exported identifier in gui/ by who
// references it: sibling repos (consumers), go-gui's own examples, the
// external test package, other gui/ packages, or nothing at all. Its
// purpose is the v1.0 API freeze: an export referenced only from inside
// gui/ is an internal helper wearing a public name, and becomes a
// compatibility promise the moment a release ships. The sweep deletes or
// unexports those, so the surface that remains is the surface that is
// actually used.
//
// Usage:
//
//	go run ./tools/exportaudit/ -mode report .  ../go-charts ../go-edit ../go-kite ../go-term ../go-map
//	go run ./tools/exportaudit/ -mode gate .
//
// The -gui flag points at the go-gui repo root (default "."); remaining
// positional arguments are consumer repos. With no consumers the scan
// still classifies example/test/internal usage, which is how CI gates:
//
//	make export-audit
//
// Mode report prints the full table, dead exports first. Mode gate exits
// non-zero on any export whose only references live inside gui/ (self,
// selftest, internal) and that does not carry a keep marker. The marker
// is a `// exportaudit:keep` mention in the declaration's doc comment —
// use it for deliberately public surface that nothing consumes yet.
//
// Reference collection is deliberately conservative (safe direction: it
// errs toward "used", never toward "dead"):
//
//   - top-level identifiers: qualified selector `alias.Name` against a
//     resolved gui import, plus the bare name when the file dot-imports
//     the package;
//   - methods and fields: the selector name `Name` and composite-literal
//     key `Name:` appear anywhere in the file (any receiver type — an
//     unrelated type's same-named method counts, so a symbol flagged dead
//     here genuinely never appears);
//   - interface implementations: any func declaration named `Name` in a
//     consumer file counts as implementing the interface method.
//
// Files are classified by location: under a consumer root → consumer;
// under the module root outside gui/ → example; inside gui/ the package
// name decides the rest (self, selftest, testext, internal).
//
// The report order is fixed (dead classes first) so diffing two runs of
// the report shows exactly what a sweep changed.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const keepMarker = "exportaudit:keep"

type kind int

const (
	kindFunc kind = iota
	kindMethod
	kindType
	kindConst
	kindVar
	kindField
)

var kindName = [...]string{"func", "method", "type", "const", "var", "field"}

// class ranks how public an export's references are. Higher = used by a
// wider circle. Gate fails on maxClass <= internal.
type class int

const (
	classNone     class = -1 // referenced nowhere
	classSelf     class = 0
	classSelfTest class = 1
	classInternal class = 2
	classTestExt  class = 3
	classExample  class = 4
	classConsumer class = 5
)

var className = [...]string{"none", "self", "selftest", "internal", "testext", "example", "consumer"}

type export struct {
	pkgPath string
	name    string
	kind    kind
	recv    string
	keep    bool
	refs    [classConsumer + 1]int
}

func (e *export) maxClass() class {
	for c := classConsumer; c >= classSelf; c-- {
		if e.refs[c] > 0 {
			return c
		}
	}
	return -1
}

func (e *export) total() int {
	n := 0
	for _, r := range e.refs {
		n += r
	}
	return n
}

func main() {
	mode := flag.String("mode", "report", "audit to run: report | gate | sweep")
	guiRoot := flag.String("gui", ".", "path to the go-gui repo root")
	dryRun := flag.Bool("dry-run", false, "mode=sweep: report renames without writing")
	flag.Parse()

	rootAbs, err := filepath.Abs(*guiRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "exportaudit:", err)
		os.Exit(1)
	}

	pkgs, err := guiPackages(rootAbs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "exportaudit:", err)
		os.Exit(1)
	}

	exports, err := collectExports(pkgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "exportaudit:", err)
		os.Exit(1)
	}

	consumers := make([]string, 0, len(flag.Args()))
	for _, c := range flag.Args() {
		abs, aerr := filepath.Abs(c)
		if aerr != nil {
			fmt.Fprintln(os.Stderr, "exportaudit:", aerr)
			os.Exit(1)
		}
		if abs == rootAbs {
			continue // the repo itself is scanned as -gui; never a consumer
		}
		consumers = append(consumers, abs)
	}

	if err := scanUsage(rootAbs, consumers, pkgs, exports); err != nil {
		fmt.Fprintln(os.Stderr, "exportaudit:", err)
		os.Exit(1)
	}

	switch *mode {
	case "report":
		printReport(exports)
	case "gate":
		failures, advisory := gate(exports, len(consumers) > 0)
		if advisory != "" {
			fmt.Fprintln(os.Stderr, advisory)
		}
		if failures != "" {
			fmt.Fprint(os.Stderr, failures)
			os.Exit(1)
		}
		fmt.Println("exportaudit: surface clean")
	case "sweep":
		if err := sweepMode(rootAbs, pkgs, exports, *dryRun); err != nil {
			fmt.Fprintln(os.Stderr, "exportaudit:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "exportaudit: unknown -mode %q (want report, gate or sweep)\n", *mode)
		os.Exit(1)
	}
}

// guiPackage maps an absolute dir under gui/ to its import path.
type guiPackage struct {
	dir        string
	importPath string
	name       string
}

// guiPackages lists every package under guiRoot/gui via go list, skipping
// internal/ trees (Go already restricts them) — the API surface is only
// the importable leaf packages.
func guiPackages(guiRoot string) ([]guiPackage, error) {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}|{{.ImportPath}}|{{.Name}}", "./gui/...")
	cmd.Dir = guiRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list ./gui/...: %w", err)
	}
	var pkgs []guiPackage
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		dir, importPath, name := parts[0], parts[1], parts[2]
		if strings.Contains(dir, string(filepath.Separator)+"internal"+string(filepath.Separator)) ||
			strings.HasSuffix(dir, string(filepath.Separator)+"internal") {
			continue
		}
		pkgs = append(pkgs, guiPackage{dir: dir, importPath: importPath, name: name})
	}
	return pkgs, nil
}

// collectExports gathers every exported identifier — top-level decls,
// methods on exported types, fields of exported struct types — across all
// gui packages, keyed for the reference scan.
func collectExports(pkgs []guiPackage) (map[string]*export, error) {
	exports := map[string]*export{}
	for _, p := range pkgs {
		err := filepath.Walk(p.dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if path != p.dir {
					return filepath.SkipDir // subdirs are other packages
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				return nil
			}
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Recv != nil {
						if name := recvTypeName(d.Recv); name != "" && ast.IsExported(name) {
							if ast.IsExported(d.Name.Name) {
								add(exports, p.importPath, d.Name.Name, kindMethod, name, hasKeep(d.Doc))
							}
						}
						continue
					}
					if ast.IsExported(d.Name.Name) {
						add(exports, p.importPath, d.Name.Name, kindFunc, "", hasKeep(d.Doc))
					}
				case *ast.GenDecl:
					collectGen(exports, p, d)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return exports, nil
}

// collectGen handles const/var/type declarations, including grouped
// const blocks and fields of exported struct types.
func collectGen(exports map[string]*export, p guiPackage, d *ast.GenDecl) {
	switch d.Tok {
	case token.CONST:
		for _, spec := range d.Specs {
			vs := spec.(*ast.ValueSpec)
			for _, name := range vs.Names {
				if ast.IsExported(name.Name) {
					add(exports, p.importPath, name.Name, kindConst, "", hasKeep(vs.Doc) || hasKeep(d.Doc))
				}
			}
		}
	case token.VAR:
		for _, spec := range d.Specs {
			vs := spec.(*ast.ValueSpec)
			for _, name := range vs.Names {
				if ast.IsExported(name.Name) {
					add(exports, p.importPath, name.Name, kindVar, "", hasKeep(vs.Doc) || hasKeep(d.Doc))
				}
			}
		}
	case token.TYPE:
		for _, spec := range d.Specs {
			ts := spec.(*ast.TypeSpec)
			if !ast.IsExported(ts.Name.Name) {
				continue
			}
			add(exports, p.importPath, ts.Name.Name, kindType, "", hasKeep(ts.Doc) || hasKeep(d.Doc))
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						if ast.IsExported(name.Name) {
							add(exports, p.importPath, name.Name, kindField, "", hasKeep(field.Doc))
						}
					}
				}
			}
		}
	}
}

// recvTypeName returns the (possibly generic) base type name of a
// receiver: *DataGrid[T] → DataGrid.
func recvTypeName(recv *ast.FieldList) string {
	expr := recv.List[0].Type
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		default:
			id, ok := t.(*ast.Ident)
			if !ok {
				return ""
			}
			return id.Name
		}
	}
}

// hasKeep reports whether a doc comment carries the keep marker.
func hasKeep(doc *ast.CommentGroup) bool {
	return doc != nil && strings.Contains(doc.Text(), keepMarker)
}

func add(exports map[string]*export, pkgPath, name string, k kind, recv string, keep bool) {
	key := pkgPath + " " + name
	switch k {
	case kindMethod:
		key += " " + recv // a func SetLocale and a method (Window) SetLocale coexist
	case kindField:
		key += " *field" // a field and a top-level decl can share a name
	}
	e, ok := exports[key]
	if !ok {
		e = &export{pkgPath: pkgPath, name: name, kind: k, recv: recv}
		exports[key] = e
	}
	if recv != "" {
		e.recv = recv
	}
	e.keep = e.keep || keep
}

// fileCtx carries everything the reference scan needs to classify one
// file's refs.
type fileCtx struct {
	class      class // consumer or example when outside gui/
	isGuipkg   bool
	ownPkgPath string // import path of the gui package owning this dir
	ownPkgName string
	filePkg    string // declared package name
	isTest     bool
	aliases    map[string]string // local alias → gui import path
	dotPkgs    []string          // dot-imported gui import paths
}

// classFor resolves the class of a ref targeting pkgPath from this file.
func (c *fileCtx) classFor(pkgPath string) class {
	if !c.isGuipkg {
		return c.class
	}
	if pkgPath != c.ownPkgPath {
		return classInternal
	}
	if c.filePkg != c.ownPkgName {
		return classTestExt
	}
	if c.isTest {
		return classSelfTest
	}
	return classSelf
}

// scanUsage walks every scanned file once, resolving imports, and stamps
// reference counts onto the exports.
func scanUsage(guiRoot string, consumerRoots []string, pkgs []guiPackage, exports map[string]*export) error {
	byPath := map[string]*guiPackage{}
	dirToPkg := map[string]*guiPackage{}
	for i := range pkgs {
		byPath[pkgs[i].importPath] = &pkgs[i]
		dirToPkg[pkgs[i].dir] = &pkgs[i]
	}

	roots := append([]string{guiRoot}, consumerRoots...)
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				switch info.Name() {
				case "vendor", ".git", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}

			ctx := fileCtx{class: dirClass(guiRoot, consumerRoots, path), filePkg: f.Name.Name, aliases: map[string]string{}}
			rel, rerr := filepath.Rel(guiRoot, path)
			if rerr == nil && strings.HasPrefix(rel, "gui"+string(filepath.Separator)) {
				ctx.isGuipkg = true
				if pkg := pkgFor(path, dirToPkg); pkg != nil {
					ctx.ownPkgPath = pkg.importPath
					ctx.ownPkgName = pkg.name
				}
			}
			ctx.isTest = strings.HasSuffix(path, "_test.go")

			for _, imp := range f.Imports {
				impPath, uerr := strconvUnquote(imp.Path.Value)
				if uerr != nil {
					continue
				}
				if _, ok := byPath[impPath]; !ok {
					continue
				}
				if imp.Name == nil {
					ctx.aliases[filepath.Base(impPath)] = impPath
				} else if imp.Name.Name == "." {
					ctx.dotPkgs = append(ctx.dotPkgs, impPath)
				} else {
					ctx.aliases[imp.Name.Name] = impPath
				}
			}
			visitFile(f, &ctx, exports, byPath)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func strconvUnquote(s string) (string, error) { return s[1 : len(s)-1], nil }

// pkgFor returns the gui package whose dir contains path.
func pkgFor(path string, dirToPkg map[string]*guiPackage) *guiPackage {
	var best *guiPackage
	bestLen := -1
	for dir, pkg := range dirToPkg {
		rel, err := filepath.Rel(dir, path)
		if err == nil && !strings.HasPrefix(rel, "..") && len(dir) > bestLen {
			best = pkg
			bestLen = len(dir)
		}
	}
	return best
}

// visitFile records references: qualified selectors against resolved gui
// imports, every selector name and composite-literal key (method/field
// coverage), dot-imported bare names, and func decl names (interface
// implementations).
func visitFile(f *ast.File, ctx *fileCtx, exports map[string]*export, byPath map[string]*guiPackage) {
	selectorNames := map[string]bool{}
	litKeys := map[string]bool{}
	funcNames := map[string]bool{}
	bareIdents := map[string]bool{}
	// declIdents holds the *declaration* identifier nodes; a name used
	// elsewhere in the same file is still a use, only the decl position is
	// not.
	declIdents := map[*ast.Ident]bool{}

	ast.Inspect(f, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := t.X.(*ast.Ident); ok {
				if impPath, ok := ctx.aliases[id.Name]; ok {
					if pkg, ok := byPath[impPath]; ok {
						ref(exports, pkg.importPath, t.Sel.Name, ctx)
					}
				}
			}
			selectorNames[t.Sel.Name] = true
		case *ast.KeyValueExpr:
			if id, ok := t.Key.(*ast.Ident); ok {
				litKeys[id.Name] = true
			}
		case *ast.FuncDecl:
			funcNames[t.Name.Name] = true
			declIdents[t.Name] = true
		case *ast.TypeSpec:
			declIdents[t.Name] = true
		case *ast.ValueSpec:
			for _, n := range t.Names {
				declIdents[n] = true
			}
		case *ast.Ident:
			if !declIdents[t] {
				bareIdents[t.Name] = true
			}
			for _, dot := range ctx.dotPkgs {
				if pkg, ok := byPath[dot]; ok {
					ref(exports, pkg.importPath, t.Name, ctx)
				}
			}
		}
		return true
	})

	// Same-package bare use of a top-level identifier is a self-ref. The
	// declaration position is excluded by pointer above, so a name used in
	// the same file as its declaration still counts.
	samePkg := ctx.ownPkgPath != "" && ctx.filePkg == ctx.ownPkgName

	for _, e := range exports {
		if e.kind == kindMethod || e.kind == kindField {
			if selectorNames[e.name] || litKeys[e.name] || funcNames[e.name] {
				e.refs[ctx.classFor(e.pkgPath)]++
			}
			continue
		}
		if samePkg && e.pkgPath == ctx.ownPkgPath && bareIdents[e.name] {
			e.refs[ctx.classFor(e.pkgPath)]++
		}
	}
}

func ref(exports map[string]*export, pkgPath, name string, ctx *fileCtx) {
	e, ok := exports[pkgPath+" "+name]
	if !ok {
		return
	}
	e.refs[ctx.classFor(pkgPath)]++
}

// dirClass is the coarse location class: consumer root wins, then module
// root outside gui/, then gui/ itself (refined per-package later).
func dirClass(guiRoot string, consumerRoots []string, path string) class {
	for _, root := range consumerRoots {
		if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
			return classConsumer
		}
	}
	rel, err := filepath.Rel(guiRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return classConsumer
	}
	if !strings.HasPrefix(rel, "gui"+string(filepath.Separator)) {
		return classExample
	}
	return classSelf
}

// printReport prints the full table, dead classes first, grouped by class
// then package, with a keep marker suffix.
func printReport(exports map[string]*export) {
	list := make([]*export, 0, len(exports))
	for _, e := range exports {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.maxClass() != b.maxClass() {
			return a.maxClass() < b.maxClass()
		}
		if a.pkgPath != b.pkgPath {
			return a.pkgPath < b.pkgPath
		}
		return a.name < b.name
	})

	var byClass [classConsumer + 2][]*export
	for _, e := range list {
		byClass[e.maxClass()+1] = append(byClass[e.maxClass()+1], e)
	}
	for c := classNone; c <= classConsumer; c++ {
		fmt.Printf("== %s (%d) ==\n", className[c+1], len(byClass[c+1]))
		for _, e := range byClass[c+1] {
			fmt.Printf("  %-8s %s %s%s%s%s\n", kindName[e.kind], e.pkgPath, e.name, qual(e), keepSuffix(e), refsSuffix(e))
		}
	}
}

func qual(e *export) string {
	if e.recv != "" {
		return " (" + e.recv + ")"
	}
	return ""
}

func keepSuffix(e *export) string {
	if e.keep {
		return " [keep]"
	}
	return ""
}

func refsSuffix(e *export) string {
	if e.total() > 0 {
		return fmt.Sprintf("  (%d refs)", e.total())
	}
	return ""
}

// gate returns the offender report, or "" when the surface is clean.
// Hard failures are the swept classes: an export used only inside its own
// package, its tests, or nowhere at all. Cross-package plumbing (internal:
// used by other gui/ packages) is reported as advisory — it is shared
// implementation, not public API, and the sweep treats it as deferred.
//
// With consumers present (hasConsumers), the scan is authoritative and
// self/selftest/none are hard failures. CI cannot see sibling repos, so
// the in-repo run is advisory only: an export used only by consumers
// would misclassify as none, and no class can be trusted.
func gate(exports map[string]*export, hasConsumers bool) (failures, advisory string) {
	var offenders, adv []string
	for _, e := range exports {
		if e.keep {
			continue
		}
		mc := e.maxClass()
		if hasConsumers && mc <= classSelfTest {
			offenders = append(offenders, fmt.Sprintf("  %-8s %s %s%s (%s)", kindName[e.kind], e.pkgPath, e.name, qual(e), className[mc+1]))
		} else if mc <= classInternal {
			adv = append(adv, fmt.Sprintf("  %-8s %s %s%s", kindName[e.kind], e.pkgPath, e.name, qual(e)))
		}
	}
	if len(offenders) == 0 && len(adv) == 0 {
		return "", ""
	}
	sort.Strings(offenders)
	sort.Strings(adv)
	var b strings.Builder
	if len(offenders) > 0 {
		fmt.Fprintf(&b, "exportaudit: %d exported identifiers used only inside gui/ (mark // %s or unexport):\n%s",
			len(offenders), keepMarker, strings.Join(offenders, "\n"))
		failures = b.String()
		b.Reset()
	}
	if len(adv) > 0 {
		fmt.Fprintf(&b, "exportaudit: %d exports need a consumer scan to classify (advisory, deferred):\n%s",
			len(adv), strings.Join(adv, "\n"))
		advisory = b.String()
	}
	return failures, advisory
}
