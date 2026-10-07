package main

// Mode spacing answers: does a call site still spell a gap, an inset, a
// corner radius or a border width as a number, when the theme names each as
// a role?
//
// Issue #851 counted about 186 Spacing literals and 178 padding literals in
// go-gui's examples alone, most of them between the ladder steps (8, 12, 16
// against 2 / 6 / 14 / 28). The scale has a stated reason (visual-refresh
// §3.3: each step doubles, so group membership reads at a glance), so the
// call sites move to the steps, and this mode keeps new literals out.
//
// Four rules:
//
//  1. Gap literal — SpacingPx(n) with a literal n > 0, wherever it appears.
//     SpacingPx builds only a gap, so no field key is needed. Zero is a real
//     choice and passes. A role (gui.SpacingMedium) passes, and so does a
//     gap computed from a theme step (SpacingPx(2 * t.SpacingLarge)).
//  2. Inset literal — PadAll, NewPadding or PadVH whose arguments are all
//     numeric literals, at least one > 0. A call that mixes in a role
//     (NewPadding(0, t.SpacingSmall, 0, t.SpacingSmall)) already names its
//     step and passes.
//  3. Button inset — rule 2 as the Padding of a ButtonCfg. Theme.PaddingButton
//     is the one button inset (issue #850); the fix is to delete the field,
//     not to snap it.
//  4. Radius or border literal — RadiusPx(n) or BorderPx(n) with a literal
//     n > 0, on the same terms as rule 1 (issue #867). The roles are
//     RadiusSmall/Medium/Large and BorderThin; a pill (RadiusPx(h/2)) is
//     computed and passes.
//
// A literal whose nearest enclosing composite literal is a Cfg in
// spacingExemptCfgs is layout, not spacing, and passes: a DrawCanvasCfg
// padding is a chart margin, like nestIndent.
//
// Scope depends on the repo. In go-gui the roles are defined in gui/, so only
// gui/view_*.go (the widget call sites) and examples/ are scanned. In any
// other repo — a sibling consumer — every non-test file is a call site.
//
// A deliberate exception (a 1px hairline, an indent) carries a same-line
// marker and prints as deferred rather than gating:
//
//	Spacing: SpacingPx(1), // ergonomics-audit:spacing — 1px hairline, not a gap
import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// spacingMarker exempts one line: the number is not a gap between siblings
// or a surface inset (a hairline, an indent, a fixed geometry).
const spacingMarker = "ergonomics-audit:spacing"

// goGuiModule is the module whose gui/ package defines the roles. Its
// scan scope is narrower than a consumer's; see scanSpacing.
const goGuiModule = "github.com/go-gui-org/go-gui"

// Finding verbs. Tests compare against these so the wording can change in
// one place.
const (
	verbGap         = "spells a gap"
	verbInset       = "spells an inset"
	verbButtonInset = "spells a button inset (delete it; Theme.PaddingButton applies)"
	verbRadius      = "spells a radius"
	verbBorder      = "spells a border width"
)

// spacingExemptCfgs names Cfg types whose padding is layout geometry, not a
// ladder inset. Keyed on the bare type name, so gui.DrawCanvasCfg and
// DrawCanvasCfg both match.
var spacingExemptCfgs = map[string]bool{
	// A canvas padding is a plot margin: room for axes and labels, sized
	// by the drawing, not by relatedness.
	"DrawCanvasCfg": true,
}

// paddingCtors are the constructors that build a Padding from numbers.
var paddingCtors = map[string]bool{
	"PadAll":     true,
	"NewPadding": true,
	"PadVH":      true,
}

// spacingFinding is one literal gap or inset.
type spacingFinding struct {
	path     string
	line     int
	fn       string
	verb     string
	expr     string
	deferred bool // carried the marker: advisory, does not gate
}

// runSpacing reports literal gaps and insets. It returns an error when any
// unmarked finding survives, so the audit gates like mode visual.
func runSpacing(repos []string) error {
	var all []spacingFinding
	for _, repo := range repos {
		found, err := scanSpacing(repo)
		if err != nil {
			return err
		}
		all = append(all, found...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].path != all[j].path {
			return all[i].path < all[j].path
		}
		return all[i].line < all[j].line
	})

	var gating int
	for _, f := range all {
		if !f.deferred {
			gating++
		}
	}

	fmt.Printf("Spacing-literal audit (%d finding(s), %d deferred)\n",
		gating, len(all)-gating)
	for _, f := range all {
		tag := ""
		if f.deferred {
			tag = " [deferred]"
		}
		loc := f.path + ":" + strconv.Itoa(f.line) + ":"
		if f.fn != "" {
			loc += " " + f.fn
		}
		fmt.Printf("  %s %s %s%s\n", loc, f.verb, f.expr, tag)
	}
	if gating > 0 {
		fmt.Println()
		fmt.Println("A gap is a Theme.Spacing* step and an inset a Theme.Padding* step,")
		fmt.Println("chosen by how related the things are (docs/style-guide.md). If this")
		fmt.Printf("number is not a gap or inset (a hairline, an indent), mark the line %q.\n",
			spacingMarker)
		return fmt.Errorf("%d spacing literal(s)", gating)
	}
	return nil
}

// scanSpacing scans one repo. go-gui itself is scanned at its call sites
// only (gui/view_*.go and examples/); any other repo is scanned whole.
func scanSpacing(repo string) ([]spacingFinding, error) {
	isGoGui := repoModule(repo) == goGuiModule
	var out []spacingFinding
	err := walkGo(repo, func(path string, fset *token.FileSet, f *ast.File) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		rel := relPath(repo, path)
		if isGoGui && !goGuiCallSite(rel) {
			return
		}
		marked := markedLines(fset, f, spacingMarker)
		inspectSpacing(fset, f, func(fn string, line int, verb, expr string) {
			out = append(out, spacingFinding{
				path: rel, line: line, fn: fn, verb: verb, expr: expr,
				deferred: marked[line],
			})
		})
	})
	return out, err
}

// goGuiCallSite reports whether rel (slash-separated, repo-relative) is a
// call site in go-gui: a widget view file directly in gui/, or anything
// under examples/.
func goGuiCallSite(rel string) bool {
	if strings.HasPrefix(rel, "examples/") {
		return true
	}
	dir, base := filepath.Split(rel)
	return dir == "gui/" && strings.HasPrefix(base, "view_")
}

// repoModule returns the module path from repo/go.mod, or "" when there is
// none. A missing go.mod scans as a consumer: the wider scope.
//
// #nosec G304 — repo is a developer-supplied CLI argument; this is a local
// audit, not a service reading user input
func repoModule(repo string) string {
	data, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// inspectSpacing walks one parsed file and calls report for each literal
// gap or inset, naming the enclosing function ("" for package-level code).
// Split out from scanSpacing so the rules run against in-memory source in
// tests.
func inspectSpacing(
	fset *token.FileSet, f *ast.File, report func(fn string, line int, verb, expr string),
) {
	for _, decl := range f.Decls {
		fn := ""
		if d, ok := decl.(*ast.FuncDecl); ok {
			if d.Body == nil {
				continue
			}
			fn = d.Name.Name
		}
		// The stack holds every open ancestor, so a finding can ask for its
		// parent key and its nearest enclosing composite literal. ast.Inspect
		// calls the visitor with nil when it leaves a node.
		var stack []ast.Node
		ast.Inspect(decl, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if verb, ok := spacingVerb(call, stack); ok {
					report(fn, fset.Position(call.Pos()).Line, verb, shortExpr(fset, call))
				}
			}
			stack = append(stack, n)
			return true
		})
	}
}

// spacingVerb reports whether call is a finding, given its open ancestors,
// and which verb describes it.
func spacingVerb(call *ast.CallExpr, stack []ast.Node) (string, bool) {
	key := parentKey(stack)
	lit := nearestLit(stack)
	if lit != nil && spacingExemptCfgs[structName(lit)] {
		return "", false
	}
	switch {
	case pxLiteralCall(call, "SpacingPx"):
		return verbGap, true
	case pxLiteralCall(call, "RadiusPx"):
		return verbRadius, true
	case pxLiteralCall(call, "BorderPx"):
		return verbBorder, true
	case insetLiteralCall(call):
		if key == "Padding" && lit != nil && structName(lit) == "ButtonCfg" {
			return verbButtonInset, true
		}
		return verbInset, true
	}
	return "", false
}

// parentKey returns the key when the top of stack is a KeyValueExpr (the
// call is a field value), else "".
func parentKey(stack []ast.Node) string {
	if len(stack) == 0 {
		return ""
	}
	kv, ok := stack[len(stack)-1].(*ast.KeyValueExpr)
	if !ok {
		return ""
	}
	if id, ok := kv.Key.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// nearestLit returns the innermost open composite literal, or nil.
func nearestLit(stack []ast.Node) *ast.CompositeLit {
	for _, v := range slices.Backward(stack) {
		if lit, ok := v.(*ast.CompositeLit); ok {
			return lit
		}
	}
	return nil
}

// pxLiteralCall reports ctor(n) with a literal n > 0, where ctor is one of
// the fixed-pixel constructors (SpacingPx, RadiusPx, BorderPx). Each builds
// only its own kind of value, so no field key is needed.
func pxLiteralCall(call *ast.CallExpr, ctor string) bool {
	if len(call.Args) != 1 || callName(call) != ctor {
		return false
	}
	v, ok := floatLiteral(call.Args[0])
	return ok && v > 0
}

// insetLiteralCall reports a padding constructor whose arguments are all
// numeric literals, at least one of them > 0.
func insetLiteralCall(call *ast.CallExpr) bool {
	if !paddingCtors[callName(call)] || len(call.Args) == 0 {
		return false
	}
	var nonZero bool
	for _, arg := range call.Args {
		v, ok := floatLiteral(arg)
		if !ok {
			return false
		}
		if v > 0 {
			nonZero = true
		}
	}
	return nonZero
}
