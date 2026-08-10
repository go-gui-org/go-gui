package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// focusStat counts literals of one Cfg type by whether they can reach
// the tab order.
type focusStat struct {
	withID   int // supplies a non-empty ID: reachable
	optedOut int // no ID but FocusDisabled: true: deliberately inert
	broken   int // no ID, not opted out: renders, clicks, never focusable
}

// cfgFocus describes one widget Cfg's focus contract, read from source.
type cfgFocus struct {
	name       string
	defaultOn  bool // has a FocusDisabled field: focusable unless opted out
	optIn      bool // has a Focusable field: inert unless opted in
	scrollable bool // has a Scrollable field: scroll offsets keyed by ID
	idRequired bool // ID carries the gui:"required" tag
	hasID      bool
}

// scanCfgFocus derives every widget Cfg's focus contract from the go-gui
// source. Deriving beats hardcoding: the guarded set changes whenever a
// gui:"required" tag is added, and a per-file (rather than per-struct)
// scan silently picks the wrong Cfg in files declaring several.
func scanCfgFocus(guiRoot string) (map[string]cfgFocus, error) {
	out := map[string]cfgFocus{}
	err := walkGo(filepath.Join(guiRoot, "gui"), func(path string, _ *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(ts.Name.Name, "Cfg") {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if c, ok := readCfgFocus(ts.Name.Name, st); ok {
					out[c.name] = c
				}
			}
		}
	})
	return out, err
}

// readCfgFocus inspects one struct's fields for the focus contract.
// Returns false for structs that take no part in focus.
func readCfgFocus(name string, st *ast.StructType) (cfgFocus, bool) {
	c := cfgFocus{name: name}
	for _, fld := range st.Fields.List {
		for _, fn := range fld.Names {
			switch fn.Name {
			case "FocusDisabled":
				c.defaultOn = true
			case "Focusable":
				c.optIn = true
			case "Scrollable":
				c.scrollable = true
			case "ID":
				c.hasID = true
				if fld.Tag != nil && tagRequires(fld.Tag.Value) {
					c.idRequired = true
				}
			}
		}
	}
	if !c.defaultOn && !c.optIn && !c.scrollable {
		return c, false
	}
	return c, true
}

// tagRequires reports whether a struct tag literal marks the field
// required. The tag takes options — `gui:"required,focus"` scopes the
// rule to widgets that join focus traversal — so this parses the value
// rather than matching the bare `gui:"required"` string, which would
// read every tagged-with-options field as unguarded.
//
// raw includes the surrounding backquotes, as it comes from the AST.
func tagRequires(raw string) bool {
	tag, err := strconv.Unquote(raw)
	if err != nil {
		return false
	}
	opts := reflect.StructTag(tag).Get("gui")
	return slices.Contains(strings.Split(opts, ","), "required")
}

// runFocus derives the unguarded Cfg set, then counts its call sites.
// With fix set, it then rewrites the broken ones to carry a generated
// ID; dry reports those rewrites without performing them.
func runFocus(guiRoot string, repos []string, fix, dry bool, only, skip *regexp.Regexp) error {
	contracts, err := scanCfgFocus(guiRoot)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", guiRoot, err)
	}

	var unguarded, guarded, optIn, scrollCovered []string
	for name, c := range contracts {
		switch {
		case c.defaultOn && c.hasID && !c.idRequired:
			unguarded = append(unguarded, name)
		case c.defaultOn:
			guarded = append(guarded, name)
		case c.optIn:
			optIn = append(optIn, name)
		}
		// Scroll offsets are keyed by Shape.ID (gui/layout_position.go),
		// so every ID-less scrollable shares the key "" and they scroll
		// in lockstep.
		//
		// A tag cannot express this contract: most containers have no
		// ID and need none, so `gui:"required"` on ContainerCfg.ID
		// would flag the common case. It is enforced instead by
		// requiredid's checkScrollableID, which keys on Scrollable in
		// the literal — so every Cfg here is covered whether or not its
		// ID carries a tag, and the list is inventory, not a gap.
		if c.scrollable && c.hasID {
			scrollCovered = append(scrollCovered, name)
		}
	}
	sort.Strings(unguarded)
	sort.Strings(guarded)
	sort.Strings(optIn)
	sort.Strings(scrollCovered)

	fmt.Printf("focus contracts derived from %s/gui\n\n", guiRoot)
	fmt.Printf("  opt-in (Focusable bool):            %d\n", len(optIn))
	fmt.Printf("  default-on, ID required:            %d  %s\n", len(guarded), strings.Join(guarded, " "))
	fmt.Printf("  default-on, ID NOT required:        %d  %s\n", len(unguarded), strings.Join(unguarded, " "))
	fmt.Printf("  scrollable, ID enforced statically: %d  %s\n\n", len(scrollCovered), strings.Join(scrollCovered, " "))

	if len(unguarded) == 0 {
		fmt.Println("no unguarded Cfgs: nothing to audit")
		return nil
	}
	if err := auditFocusLiterals(unguarded, repos); err != nil {
		return err
	}
	if !fix {
		return nil
	}
	return runFixFocus(unguarded, repos, dry, only, skip)
}

// auditFocusLiterals counts literals of the unguarded Cfgs per repo.
func auditFocusLiterals(unguarded []string, repos []string) error {
	grand := map[string]*focusStat{}
	for _, repo := range repos {
		per := map[string]*focusStat{}
		var broken []string

		err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
			isTest := strings.HasSuffix(path, "_test.go")
			parents := focusParentCalls(f)
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || lit.Type == nil {
					return true
				}
				name := structName(lit)
				if !slices.Contains(unguarded, name) {
					return true
				}
				if per[name] == nil {
					per[name] = &focusStat{}
				}
				if grand[name] == nil {
					grand[name] = &focusStat{}
				}
				switch classifyFocusLiteral(lit) {
				case "withID":
					per[name].withID++
					grand[name].withID++
				case "optedOut":
					per[name].optedOut++
					grand[name].optedOut++
				default:
					if wrapperArg(lit, name, parents) {
						return true
					}
					per[name].broken++
					grand[name].broken++
					pos := fset.Position(lit.Pos())
					tag := ""
					if isTest {
						tag = " [test]"
					}
					broken = append(broken, "    "+relPath(repo, pos.Filename)+
						":"+strconv.Itoa(pos.Line)+" "+name+tag)
				}
				return true
			})
		})
		if err != nil {
			return fmt.Errorf("walking %s: %w", repo, err)
		}
		printFocusRepo(filepath.Base(repo), per, broken)
	}
	printFocusTotals(grand)
	return nil
}

// classifyFocusLiteral reports whether a literal is reachable, opted
// out, or broken. An ID set to a non-constant expression counts as
// present: whether it evaluates empty is a runtime question, so the
// broken count is a floor.
func classifyFocusLiteral(lit *ast.CompositeLit) string {
	if v, ok := litField(lit, "ID"); ok {
		if b, isLit := v.(*ast.BasicLit); !isLit || b.Value != `""` {
			return "withID"
		}
	}
	if v, ok := litField(lit, "FocusDisabled"); ok && isTrue(v) {
		return "optedOut"
	}
	return "broken"
}

// wrapperArg reports whether the literal is handed to a factory other
// than its own — CommandButton(cmdID, ButtonCfg{}), say. Such a
// wrapper may fill the ID in itself, as CommandButton does, so an
// empty ID at the call site says nothing about reachability. Counting
// it as broken produces a false positive that neither the requiredid
// analyzer (which matches on the factory name) nor the runtime guard
// (which runs after the wrapper has filled the field) agrees with.
//
// A literal that is not a call argument at all — assigned to a
// variable, say — stays countable: the broken count is documented as
// a floor, and this only removes the cases actively known to be fine.
func wrapperArg(lit *ast.CompositeLit, cfgName string, parents map[*ast.CompositeLit]*ast.CallExpr) bool {
	call, ok := parents[lit]
	if !ok {
		return false
	}
	own := strings.TrimSuffix(cfgName, "Cfg")
	if own == cfgName {
		return false
	}
	var name string
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		name = fn.Name
	case *ast.SelectorExpr:
		name = fn.Sel.Name
	default:
		return false
	}
	return name != own
}

// focusParentCalls maps each composite literal used directly as a call
// argument to the enclosing call.
func focusParentCalls(f *ast.File) map[*ast.CompositeLit]*ast.CallExpr {
	out := map[*ast.CompositeLit]*ast.CallExpr{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, a := range call.Args {
			if lit, isLit := a.(*ast.CompositeLit); isLit {
				out[lit] = call
			}
		}
		return true
	})
	return out
}

func printFocusRepo(name string, per map[string]*focusStat, broken []string) {
	fmt.Printf("=== %s ===\n", name)
	if len(per) == 0 {
		fmt.Println("  (no literals of the unguarded Cfgs)")
		return
	}
	for _, k := range sortedKeys(per) {
		s := per[k]
		fmt.Printf("  %-22s withID=%-4d optedOut=%-3d broken=%d\n",
			k, s.withID, s.optedOut, s.broken)
	}
	if len(broken) > 0 {
		fmt.Printf("  broken (focusable, no ID, not opted out): %d\n", len(broken))
		sort.Strings(broken)
		for _, b := range broken {
			fmt.Println(b)
		}
	}
	fmt.Println()
}

func printFocusTotals(grand map[string]*focusStat) {
	fmt.Println("=== TOTAL ===")
	var tw, to, tb int
	for _, k := range sortedKeys(grand) {
		s := grand[k]
		fmt.Printf("  %-22s withID=%-4d optedOut=%-3d broken=%d\n",
			k, s.withID, s.optedOut, s.broken)
		tw += s.withID
		to += s.optedOut
		tb += s.broken
	}
	fmt.Printf("  %-22s withID=%-4d optedOut=%-3d broken=%d\n",
		"ALL", tw, to, tb)
}

func sortedKeys(m map[string]*focusStat) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
