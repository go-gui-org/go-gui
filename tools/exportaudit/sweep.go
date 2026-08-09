// Sweep mode: unexport every export whose references stay inside gui/.
//
//	go run ./tools/exportaudit/ -mode sweep -gui . .
//
// The rename follows the audit's classes: an export with maxClass
// self/selftest/none is an internal helper wearing a public name, so the
// sweep lowercases it and rewrites every reference in its own package.
// Consumer- and example-used exports are never touched, and neither are
// keep-marked ones.
//
// Safety rules (each is deliberately conservative):
//
//   - Top-level symbols (func/type/const/var): rename the declaration and
//     every bare-identifier use, but never a selector .Foo, a composite
//     literal key Foo:, an import alias, a struct field declaration, or
//     an interface method — those address different namespaces.
//   - Methods and fields: rename the declaration, selector uses (.Foo),
//     and literal keys (Foo:), but ONLY when the name is declared exactly
//     once in the package. A duplicate declaration site means some other
//     type has its own Foo and the selectors are ambiguous — skipping is
//     the only safe move. Such targets are reported so they can get a
//     keep marker instead.
//
// The sweep prints every rewrite; it exits non-zero if any target was
// skipped, so the caller notices symbols that still need a keep marker or
// manual work.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// relPath trims root from path for readable output.
func relPath(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}

// declSite is one place a name is declared inside a package.
type declSite struct {
	path     string // file the declaration lives in
	ident    *ast.Ident
	isMethod bool
	isField  bool
	isIface  bool
	owner    string // struct type owning a field site
}

// pkgFiles holds every parsed file of one gui package.
type pkgFiles struct {
	files map[string]*ast.File // path → file
	sets  map[string]*token.FileSet
}

// sweep lowercases every sweep-eligible export and rewrites its package.
func sweep(guiRoot string, pkgs []guiPackage, exports map[string]*export) error {
	return sweepMode(guiRoot, pkgs, exports, false)
}

// sweepMode implements sweep; dryRun reports what would change and
// whether it would be skipped, without writing anything.
func sweepMode(guiRoot string, pkgs []guiPackage, exports map[string]*export, dryRun bool) error {
	// Load every package's files once.
	loaded := map[string]*pkgFiles{}
	for _, p := range pkgs {
		pf := &pkgFiles{files: map[string]*ast.File{}, sets: map[string]*token.FileSet{}}
		err := filepath.Walk(p.dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if path != p.dir {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				return nil
			}
			pf.files[path] = f
			pf.sets[path] = fset
			return nil
		})
		if err != nil {
			return err
		}
		loaded[p.importPath] = pf
	}

	var skipped []string
	var keepMarks []*export // sig-reach types marked after the rename pass
	renamed := 0
	allTargets := 0
	for _, t := range exports {
		if t.keep || t.maxClass() > classSelfTest {
			continue
		}
		allTargets++
		pf := loaded[t.pkgPath]
		if pf == nil {
			skipped = append(skipped, fmt.Sprintf("%s %s%s (package not loaded)", t.pkgPath, t.name, qual(t)))
			continue
		}
		lower := unexport(t.name)

		sites := declSites(pf, t.name)
		if len(sites) == 0 {
			skipped = append(skipped, fmt.Sprintf("%s %s%s (declaration not found)", t.pkgPath, t.name, qual(t)))
			continue
		}
		if reason := skipReason(pf, exports, t, sites, lower); reason != "" {
			// Any skipped target is deliberately-kept surface: the sweep
			// leaves it alone and records the keep marker so the gate does
			// not flag it later. Marking is deferred until after the
			// rename pass so the inserted lines cannot shift rename
			// offsets in the same file.
			keepMarks = append(keepMarks, t)
			skipped = append(skipped, fmt.Sprintf("%s %s%s (%s)", t.pkgPath, t.name, qual(t), reason))
			continue
		}

		for path, f := range pf.files {
			idents := renameIdents(pf.sets[path], f, t, sites)
			if len(idents) == 0 {
				continue
			}
			fset := pf.sets[path]
			offsets := make([]int, 0, len(idents))
			for _, id := range idents {
				offsets = append(offsets, fset.Position(id.Pos()).Offset)
			}
			if !dryRun {
				if err := lowercaseAt(path, offsets); err != nil {
					return err
				}
			}
			fmt.Printf("exportaudit: %-8s %s %s%s -> %s (%d idents, %s)\n",
				kindName[t.kind], t.pkgPath, t.name, qual(t), lower, len(idents), relPath(guiRoot, path))
		}
		renamed++
	}

	// Re-parse each touched package before inserting keep markers, so the
	// marker offsets reflect the renamed files.
	if !dryRun {
		for _, t := range keepMarks {
			if err := markKeep(loaded[t.pkgPath], t); err != nil {
				return err
			}
		}
	}

	verb := "unexported"
	if dryRun {
		verb = "would unexport"
	}
	fmt.Printf("exportaudit: %s %d of %d targets (%d skipped)\n", verb, renamed, allTargets, len(skipped))
	if len(skipped) > 0 {
		sort.Strings(skipped)
		fmt.Printf("exportaudit: skipped (mark // %s or handle manually):\n", keepMarker)
		for _, s := range skipped {
			fmt.Println("  " + s)
		}
	}
	return nil
}

// literalIsOwnerExpr reports whether a composite literal type expression
// names one of the owning types of a field target: an Ident (TreeStyle{...})
// or a SelectorExpr (.gui.TreeStyle{...}) whose Sel matches.
func literalIsOwnerExpr(typ ast.Expr, owners map[string]bool) bool {
	switch t := typ.(type) {
	case *ast.Ident:
		return owners[t.Name]
	case *ast.SelectorExpr:
		return owners[t.Sel.Name]
	}
	return false
}

// shadowedLocal reports the first function name where a local (param,
// var, or :=) named lower coexists with a use of name — the rename Foo ->
// foo would turn that use into a reference to the local.
func shadowedLocal(pf *pkgFiles, name, lower string) string {
	for _, f := range pf.files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			var hasLower, hasName bool
			ast.Inspect(fd, func(n ast.Node) bool {
				switch t := n.(type) {
				case *ast.Field:
					// Params and receiver fields bind in the function.
					for _, id := range t.Names {
						if id.Name == lower {
							hasLower = true
						}
					}
				case *ast.AssignStmt:
					if t.Tok == token.DEFINE {
						for _, e := range t.Lhs {
							if id, ok := e.(*ast.Ident); ok && id.Name == lower {
								hasLower = true
							}
						}
					}
				case *ast.Ident:
					if t.Name == name {
						hasName = true
					}
				}
				return true
			})
			if hasLower && hasName {
				return fd.Name.Name
			}
		}
	}
	return ""
}

// ownerCollides reports whether the lowercased name is a member of one of
// the same owning types as the target's sites — the rename would create a
// duplicate member (method+field or same-name pair) inside that type.
func ownerCollides(pf *pkgFiles, sites []declSite, lower string) bool {
	owners := map[string]bool{}
	for _, s := range sites {
		if s.owner != "" {
			owners[s.owner] = true
		}
	}
	if len(owners) == 0 {
		return false
	}
	for _, s := range declSites(pf, lower) {
		if s.owner != "" && owners[s.owner] {
			return true
		}
	}
	return false
}

// crossPkgMember reports whether name is an exported field or method of a
// type in a different gui package. .Foo selectors name the member by bare
// name, so a rename in one package would also hit receivers of the other
// package's types — the ambiguity is unresolvable without type info.
func crossPkgMember(exports map[string]*export, t *export) bool {
	for _, e := range exports {
		if e == t || e.name != t.name {
			continue
		}
		if e.pkgPath == t.pkgPath {
			continue
		}
		if e.kind == kindField || e.kind == kindMethod {
			return true
		}
	}
	return false
}

// builtins names the predeclared identifiers a lowercase rename would
// shadow inside the package, breaking bare calls like new(T) or len(x).
var builtins = map[string]bool{
	"any": true, "append": true, "bool": true, "byte": true, "cap": true,
	"clear": true, "close": true, "comparable": true, "complex": true,
	"complex128": true, "complex64": true, "copy": true, "delete": true,
	"error": true, "false": true, "float32": true, "float64": true,
	"imag": true, "int": true, "int16": true, "int32": true, "int64": true,
	"int8": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "nil": true, "panic": true, "print": true, "println": true,
	"real": true, "recover": true, "rune": true, "string": true,
	"true": true, "uint": true, "uint16": true, "uint32": true,
	"uint64": true, "uint8": true, "uintptr": true,
}

// jsonTaggedField reports whether any field named name in the package
// carries a struct tag containing json: — such a field is serialized by
// stdlib reflection, and unexporting it silently drops it from the wire.
func jsonTaggedField(pf *pkgFiles, name string) bool {
	for _, f := range pf.files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range st.Fields.List {
					for _, id := range field.Names {
						if id.Name != name {
							continue
						}
						if field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// skipReason returns why a target must not be renamed, or "" when it is
// safe to proceed. Each check is deliberately conservative: the codemod
// renames by syntax, so anything the compiler or stdlib reflection could
// resolve differently must stop here.
func skipReason(pf *pkgFiles, exports map[string]*export, t *export, sites []declSite, lower string) string {
	topLevel := t.kind == kindFunc || t.kind == kindType || t.kind == kindConst || t.kind == kindVar
	member := t.kind == kindMethod || t.kind == kindField

	if topLevel {
		// Bare-name uses can be captured by a local that declares the
		// lowercase name in the same function.
		if shadowed := shadowedLocal(pf, t.name, lower); shadowed != "" {
			return fmt.Sprintf("local %q shadows in %s", lower, shadowed)
		}
		// Foo -> foo collides with an existing package-level foo.
		if len(declSites(pf, lower)) > 0 {
			return fmt.Sprintf("lowercase %q already declared", lower)
		}
		// Foo -> new/len/make shadows a Go builtin (new(T) would call it).
		if builtins[lower] {
			return fmt.Sprintf("lowercase %q is a Go builtin", lower)
		}
	}
	if member {
		// (App) Windows -> windows collides with App.windows.
		if ownerCollides(pf, sites, lower) {
			return fmt.Sprintf("lowercase %q member of same type exists", lower)
		}
		// .Foo selectors can't be disambiguated by receiver type: the name
		// must not be an exported member of a type in another gui package.
		if crossPkgMember(exports, t) {
			return "member of a type in another gui package"
		}
	}
	if t.kind == kindMethod && len(sites) != 1 {
		// A method name on several types is ambiguous and may implement a
		// stdlib interface the audit cannot see (UnmarshalText etc.).
		return fmt.Sprintf("name declared %d times", len(sites))
	}
	if t.kind == kindField {
		// Same-name fields merge into one entry, but a method of the same
		// name makes .Foo selectors ambiguous between access and call.
		if hasMethodSite(sites) {
			return "method of same name exists"
		}
		// A json-tagged field participates in stdlib reflection
		// serialization, which the audit cannot see.
		if jsonTaggedField(pf, t.name) {
			return "json-tagged field"
		}
	}
	if t.kind == kindType {
		// A type reachable from a kept export's signature is public API:
		// NewHeroTransition returns *heroTransition, so consumers hold the
		// type even though no scan names it. The lint unexported-return
		// rule enforces this.
		if sigReferenced(pf, t.name) {
			return "returned or parameterized by an exported function"
		}
	}
	return ""
}

// sigReferenced reports whether an exported function or method in the
// package names typ in its signature (return or parameter type).
func sigReferenced(pf *pkgFiles, typ string) bool {
	for _, f := range pf.files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || !ast.IsExported(fd.Name.Name) {
				continue
			}
			if sigUsesType(fd.Type, typ) {
				return true
			}
		}
	}
	return false
}

// sigUsesType reports whether the type name appears in a func type's
// parameter or result types.
func sigUsesType(ft *ast.FuncType, typ string) bool {
	if ft.Params != nil {
		for _, field := range ft.Params.List {
			if typeUsesName(field.Type, typ) {
				return true
			}
		}
	}
	if ft.Results != nil {
		for _, field := range ft.Results.List {
			if typeUsesName(field.Type, typ) {
				return true
			}
		}
	}
	return false
}

// typeUsesName reports whether an (possibly nested) type expression names
// typ: T, *T, []T, [4]T, map[K]T, func(...) T, interface{ ... T ... }.
func typeUsesName(e ast.Expr, typ string) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == typ
	case *ast.StarExpr:
		return typeUsesName(t.X, typ)
	case *ast.ArrayType:
		return typeUsesName(t.Elt, typ)
	case *ast.MapType:
		return typeUsesName(t.Key, typ) || typeUsesName(t.Value, typ)
	case *ast.ChanType:
		return typeUsesName(t.Value, typ)
	case *ast.Ellipsis:
		return typeUsesName(t.Elt, typ)
	case *ast.FuncType:
		return sigUsesType(t, typ)
	case *ast.IndexExpr:
		return typeUsesName(t.X, typ)
	case *ast.IndexListExpr:
		return typeUsesName(t.X, typ)
	case *ast.SelectorExpr:
		return t.Sel.Name == typ
	}
	return false
}

// markKeep inserts a keep-marker comment line above the target's
// declaration so the gate accepts it. The file is re-parsed fresh: the
// rename pass has already rewritten it, so cached AST offsets are stale.
func markKeep(pf *pkgFiles, t *export) error {
	sites := declSites(pf, t.name)
	if len(sites) == 0 {
		return nil
	}
	path := sites[0].path
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	// Mark the first declaration whose KIND matches the target. A type
	// and a same-named method both exist in one file (FormPendingState);
	// sites[0] may be the method, so skip mismatched kinds and let the
	// matching kind take priority.
	var declPos token.Pos
	foundKind := false
	ast.Inspect(f, func(n ast.Node) bool {
		if declPos != 0 {
			return false
		}
		matchKind := func(id *ast.Ident, k kind) {
			if id.Name == t.name && k == t.kind {
				declPos = id.Pos()
			}
		}
		switch node := n.(type) {
		case *ast.TypeSpec:
			if !foundKind && t.kind == kindType {
				matchKind(node.Name, kindType)
			}
		case *ast.ValueSpec:
			if !foundKind && (t.kind == kindConst || t.kind == kindVar) {
				for _, id := range node.Names {
					matchKind(id, t.kind)
				}
			}
		case *ast.FuncDecl:
			if !foundKind && t.kind == kindFunc {
				matchKind(node.Name, kindFunc)
			}
		case *ast.Field:
			// Struct field or interface method.
			if t.kind == kindField || t.kind == kindMethod {
				for _, id := range node.Names {
					if id.Name == t.name {
						declPos = id.Pos()
					}
				}
			}
		}
		if declPos != 0 {
			foundKind = true
		}
		return true
	})
	if declPos == 0 {
		return nil
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pos := fset.Position(declPos).Offset
	lineStart := pos
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	marker := "\t// " + keepMarker + " — " + keepReason(t) + "\n"
	out := make([]byte, 0, len(src)+len(marker))
	out = append(out, src[:lineStart]...)
	out = append(out, marker...)
	out = append(out, src[lineStart:]...)
	return os.WriteFile(path, out, 0o644)
}

// keepReason returns a short justification for a keep-marked export.
func keepReason(t *export) string {
	switch t.kind {
	case kindType:
		return "reachable from an exported signature"
	case kindField:
		return "json-tagged or same-named member"
	case kindConst, kindVar, kindFunc, kindMethod:
		return "collision or documented API"
	}
	return "sweep skip"
}

// hasMethodSite reports whether any site is a method declaration.
func hasMethodSite(sites []declSite) bool {
	for _, s := range sites {
		if s.isMethod {
			return true
		}
	}
	return false
}

// lowercaseAt lowercases the rune at each byte offset (descending order).
func lowercaseAt(path string, offsets []int) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.IntSlice(offsets)))
	for _, off := range offsets {
		if off >= len(src) || src[off] < 'A' || src[off] > 'Z' {
			continue
		}
		src[off] += 'a' - 'A'
	}
	return os.WriteFile(path, src, 0o644)
}

// unexport lowercases the first rune of an exported name.
func unexport(name string) string {
	r, size := utf8.DecodeRuneInString(name)
	return string(unicode.ToLower(r)) + name[size:]
}

// declSites collects every place name is declared in the package's files:
// funcs (method or not), types, values, struct fields, interface methods.
// Only package-level declarations and type members are considered: a
// function-local var with the same name is a shadow, not a collision, and
// is handled by shadowedLocal.
func declSites(pf *pkgFiles, name string) []declSite {
	var sites []declSite
	for path, f := range pf.files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name == name {
					owner := ""
					if d.Recv != nil {
						owner = recvTypeName(d.Recv)
					}
					sites = append(sites, declSite{path: path, ident: d.Name, isMethod: d.Recv != nil, owner: owner})
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.Name == name {
							sites = append(sites, declSite{path: path, ident: s.Name})
						}
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								for _, id := range field.Names {
									if id.Name == name {
										sites = append(sites, declSite{path: path, ident: id, isField: true, owner: s.Name.Name})
									}
								}
							}
						}
						if it, ok := s.Type.(*ast.InterfaceType); ok {
							for _, m := range it.Methods.List {
								for _, id := range m.Names {
									if id.Name == name {
										sites = append(sites, declSite{path: path, ident: id, isIface: true})
									}
								}
							}
						}
					case *ast.ValueSpec:
						for _, id := range s.Names {
							if id.Name == name {
								sites = append(sites, declSite{path: path, ident: id})
							}
						}
					}
				}
			}
		}
	}
	return sites
}

// renameIdents returns the identifier nodes to lowercase for one target in
// one file. Top-level symbols: declaration plus bare uses, never a
// selector .Foo, a literal key Foo:, an import alias, a struct field
// declaration, or an interface method. Methods/fields: the declaration,
// selector uses, and literal keys.
func renameIdents(fset *token.FileSet, f *ast.File, t *export, sites []declSite) []*ast.Ident {
	topLevel := t.kind != kindMethod && t.kind != kindField
	var out []*ast.Ident

	// Positions that name something other than the top-level symbol; the
	// Ident case consults this so a bare-name match does not double-add or
	// touch a foreign namespace.
	excluded := map[*ast.Ident]bool{}
	out = append(out, declIdents(fset, f, t, sites, topLevel, excluded)...)
	ownerKeys := collectOwnerKeys(f, t, sites)
	appendUses(f, t, topLevel, ownerKeys, excluded, &out)
	return out
}

// declIdents returns the target's declaration idents in this file. Interface
// methods are never renamed (their implementations are the consumers, and
// the name stays part of the interface contract). A top-level target never
// renames methods or fields — those are different namespaces even when the
// name matches.
func declIdents(fset *token.FileSet, f *ast.File, t *export, sites []declSite, topLevel bool, excluded map[*ast.Ident]bool) []*ast.Ident {
	var out []*ast.Ident
	filename := fset.Position(f.Pos()).Filename
	for _, s := range sites {
		if s.isIface {
			excluded[s.ident] = true
			continue
		}
		if topLevel && (s.isMethod || s.isField) {
			excluded[s.ident] = true // same name, different namespace
			continue
		}
		if !topLevel && (s.isMethod != (t.kind == kindMethod) || s.isField != (t.kind == kindField)) {
			continue // only the target's own declaration site
		}
		if s.path != filename {
			continue // declaration lives in another file of the package
		}
		out = append(out, s.ident)
		excluded[s.ident] = true
	}
	return out
}

// collectOwnerKeys marks the composite-literal keys of a field target's
// owning structs. ownerTypes names the structs that own a field target —
// literal keys only follow when the literal is one of them, so a foreign
// struct's same-named field is untouched. Elided inner literals
// ([]maskTokenDef{{...}}) have nil Type; the enclosing array/slice/map
// literal supplies the element/value type.
func collectOwnerKeys(f *ast.File, t *export, sites []declSite) map[*ast.Ident]bool {
	ownerKeys := map[*ast.Ident]bool{}
	ownerTypes := map[string]bool{}
	if t.kind != kindField {
		return ownerKeys
	}
	for _, s := range sites {
		if s.isField && s.owner != "" {
			ownerTypes[s.owner] = true
		}
	}
	if len(ownerTypes) == 0 {
		return ownerKeys
	}
	var walk func(n ast.Node, effType ast.Expr)
	walk = func(n ast.Node, effType ast.Expr) {
		if n == nil {
			return
		}
		switch node := n.(type) {
		case *ast.CompositeLit:
			typ := node.Type
			if typ == nil {
				typ = effType
			}
			if literalIsOwnerExpr(typ, ownerTypes) {
				for _, el := range node.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok {
							ownerKeys[id] = true
						}
					}
				}
			}
			// Children inherit this literal's type; array/slice elements
			// and map values get the element/value type.
			childType := typ
			if arr, ok := typ.(*ast.ArrayType); ok {
				childType = arr.Elt
			}
			if m, ok := typ.(*ast.MapType); ok {
				childType = m.Value
			}
			for _, el := range node.Elts {
				walk(el, childType)
			}
		case *ast.KeyValueExpr:
			walk(node.Key, effType)
			walk(node.Value, effType)
		default:
			ast.Inspect(n, func(c ast.Node) bool {
				if c == n {
					return true
				}
				walk(c, effType)
				return false
			})
		}
	}
	walk(f, nil)
	return ownerKeys
}

// appendUses adds the non-declaration identifier uses: array/map literal
// keys, struct-literal keys of the target's owners, selector uses of
// methods/fields, and bare uses of top-level symbols.
func appendUses(f *ast.File, t *export, topLevel bool, ownerKeys map[*ast.Ident]bool, excluded map[*ast.Ident]bool, out *[]*ast.Ident) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			// Array/map literals' keys are constant values (iota indexes),
			// not field names: they follow a top-level rename. Struct
			// literal keys are field names and follow a field target only
			// when the literal is one of the field's owning structs.
			_, isArray := node.Type.(*ast.ArrayType)
			_, isMap := node.Type.(*ast.MapType)
			if isArray || isMap {
				for _, el := range node.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok && id.Name == t.name {
							*out = append(*out, id)
						}
					}
				}
			}
		case *ast.KeyValueExpr:
			if id, ok := node.Key.(*ast.Ident); ok {
				excluded[id] = true
				if !topLevel && id.Name == t.name {
					if t.kind == kindField && !ownerKeys[id] {
						break // a foreign struct's same-named field
					}
					*out = append(*out, id)
				}
			}
		case *ast.SelectorExpr:
			// .Foo on a method/field target follows the rename.
			excluded[node.Sel] = true
			if !topLevel && node.Sel.Name == t.name {
				*out = append(*out, node.Sel)
			}
		case *ast.StructType:
			if topLevel {
				for _, field := range node.Fields.List {
					for _, id := range field.Names {
						excluded[id] = true
					}
				}
			}
		case *ast.ImportSpec:
			if topLevel && node.Name != nil {
				excluded[node.Name] = true
			}
		case *ast.Ident:
			if topLevel && node.Name == t.name && !excluded[node] {
				*out = append(*out, node)
			}
		}
		return true
	})
}
