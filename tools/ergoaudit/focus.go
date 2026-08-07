package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
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
				if fld.Tag != nil && strings.Contains(fld.Tag.Value, `gui:"required"`) {
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

// runFocus derives the unguarded Cfg set, then counts its call sites.
// With fix set, it then rewrites the broken ones to carry a generated
// ID; dry reports those rewrites without performing them.
func runFocus(guiRoot string, repos []string, fix, dry bool, only, skip *regexp.Regexp) error {
	contracts, err := scanCfgFocus(guiRoot)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", guiRoot, err)
	}

	var unguarded, guarded, optIn, scrollUnguarded []string
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
		// in lockstep. Report the same contract gap for Scrollable.
		if c.scrollable && c.hasID && !c.idRequired {
			scrollUnguarded = append(scrollUnguarded, name)
		}
	}
	sort.Strings(unguarded)
	sort.Strings(guarded)
	sort.Strings(optIn)
	sort.Strings(scrollUnguarded)

	fmt.Printf("focus contracts derived from %s/gui\n\n", guiRoot)
	fmt.Printf("  opt-in (Focusable bool):            %d\n", len(optIn))
	fmt.Printf("  default-on, ID required:            %d  %s\n", len(guarded), strings.Join(guarded, " "))
	fmt.Printf("  default-on, ID NOT required:        %d  %s\n", len(unguarded), strings.Join(unguarded, " "))
	fmt.Printf("  scrollable, ID NOT required:        %d  %s\n\n", len(scrollUnguarded), strings.Join(scrollUnguarded, " "))

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
