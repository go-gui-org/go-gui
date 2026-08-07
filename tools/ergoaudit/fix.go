package main

// Mode focus's -fix pass: insert a generated ID into every literal the
// audit classifies as broken.
//
// Phase 1 of docs/specs/developer-ergonomics.md tags nine Cfg types
// with gui:"required" and adds RequireID to their factories, which
// turns 111 literals inside this repo into build failures at once.
// Hand-editing 111 literals is how a typo'd ID reaches main looking
// like intent, so the insertion is mechanical and reviewable as a
// single diff.
//
// This does not reopen spec §5.1, which rejects positional IDs. §5.1's
// objection is to IDs *computed at runtime from tree position*: they
// have no source-level existence and silently change when a sibling is
// inserted, so a list insert hands every row below it the previous
// occupant's scroll offset and open-dropdown state. An ID generated
// into the source is an ordinary ID that a human reads, reviews, and
// edits, and it stops moving the moment it is written.

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// edit is one splice into a file's bytes. end == start is a pure
// insertion; end > start replaces that span.
type edit struct {
	text       string
	start, end int
}

// runFixFocus rewrites every broken literal of the unguarded Cfg set
// across repos. With dry set, it reports what it would write and
// touches nothing.
func runFixFocus(unguarded []string, repos []string, dry bool, only, skip *regexp.Regexp) error {
	var files, inserted int
	for _, repo := range repos {
		paths, err := goFiles(repo)
		if err != nil {
			return fmt.Errorf("walking %s: %w", repo, err)
		}
		for _, path := range paths {
			// The path filters are how phase 1 keeps the codemod off
			// go-gui's own widgets: those defects are hand-fixed with
			// meaningful IDs, because a shipped widget's ID is public
			// identity, not scaffolding.
			if !fixWants(only, skip, relPath(repo, path)) {
				continue
			}
			n, err := fixFile(repo, path, unguarded, dry)
			if err != nil {
				return err
			}
			if n > 0 {
				files++
				inserted += n
			}
		}
	}
	verb := "inserted"
	if dry {
		verb = "would insert"
	}
	fmt.Printf("\n%s %d ID field(s) across %d file(s)\n", verb, inserted, files)
	return nil
}

// fixWants reports whether a path survives both filters: -fix-only is
// a whitelist when set, -fix-exclude a blacklist applied after it.
func fixWants(only, skip *regexp.Regexp, rel string) bool {
	if only != nil && !only.MatchString(rel) {
		return false
	}
	return skip == nil || !skip.MatchString(rel)
}

// goFiles lists every non-vendored .go file under root, sorted so the
// output order is stable between runs.
func goFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", ".git", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// fixFile rewrites one file, returning how many IDs it added. A file
// that does not parse is skipped rather than failed: a repo may hold
// sources for another GOOS that do not build here.
func fixFile(repo, path string, unguarded []string, dry bool) (int, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	fset := token.NewFileSet()
	f, perr := parser.ParseFile(fset, path, src, parser.ParseComments)
	if perr != nil {
		return 0, nil
	}

	scope := idScope(path)
	taken := existingIDs(f)
	var edits []edit

	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || lit.Type == nil {
			return true
		}
		name := structName(lit)
		if !slices.Contains(unguarded, name) {
			return true
		}
		if classifyFocusLiteral(lit) != "broken" {
			return true
		}
		id := uniqueID(taken, scope, idHint(lit, enclosingFunc(f, lit.Pos()), name))
		e, ok := idEdit(fset, src, lit, id)
		if !ok {
			return true
		}
		edits = append(edits, e)
		if dry {
			pos := fset.Position(lit.Pos())
			fmt.Printf("    %s:%d %s -> ID: %q\n",
				relPath(repo, pos.Filename), pos.Line, name, id)
		}
		return true
	})

	if len(edits) == 0 {
		return 0, nil
	}
	if dry {
		return len(edits), nil
	}
	out, err := applyEdits(src, edits)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return 0, fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("  %s: +%d\n", relPath(repo, path), len(edits))
	return len(edits), nil
}

// idEdit builds the splice that gives lit the given ID. Two shapes,
// because classifyFocusLiteral calls both "broken": ID absent, which
// needs a new field, and ID present as "", which needs its value
// replaced. Inserting in the second case would produce a duplicate key
// and a file that does not compile.
func idEdit(fset *token.FileSet, src []byte, lit *ast.CompositeLit, id string) (edit, bool) {
	if v, ok := litField(lit, "ID"); ok {
		b, isLit := v.(*ast.BasicLit)
		if !isLit {
			return edit{}, false // computed ID: not ours to rewrite
		}
		return edit{
			start: fset.Position(b.Pos()).Offset,
			end:   fset.Position(b.End()).Offset,
			text:  strconv.Quote(id),
		}, true
	}

	at := fset.Position(lit.Lbrace).Offset + 1
	field := "ID: " + strconv.Quote(id) + ","

	// A literal written on one line stays on one line; expanding it
	// would reformat code the change is not about.
	if fset.Position(lit.Lbrace).Line == fset.Position(lit.Rbrace).Line {
		if len(lit.Elts) == 0 {
			field = strings.TrimSuffix(field, ",")
		} else {
			field += " "
		}
		return edit{start: at, end: at, text: field}, true
	}

	// Multi-line: give the field its own line at the indentation of
	// whatever follows the brace. gofmt re-aligns the block afterwards.
	indent := "\t"
	if len(lit.Elts) > 0 {
		indent = lineIndent(src, fset.Position(lit.Elts[0].Pos()).Offset)
	}
	return edit{start: at, end: at, text: "\n" + indent + field}, true
}

// lineIndent returns the leading whitespace of the line containing
// offset.
func lineIndent(src []byte, offset int) string {
	start := offset
	for start > 0 && src[start-1] != '\n' {
		start--
	}
	end := start
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return string(src[start:end])
}

// applyEdits splices in descending offset order, so an earlier edit
// never shifts a later one, then gofmts the result.
func applyEdits(src []byte, edits []edit) ([]byte, error) {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := slices.Clone(src)
	for _, e := range edits {
		out = append(out[:e.start:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	formatted, err := format.Source(out)
	if err != nil {
		return nil, fmt.Errorf("rewritten source does not parse: %w", err)
	}
	return formatted, nil
}

// existingIDs collects every string-literal ID already present in the
// file, so a generated ID never collides with a hand-written one.
func existingIDs(f *ast.File) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if v, ok := litField(lit, "ID"); ok {
			if b, isLit := v.(*ast.BasicLit); isLit && b.Kind == token.STRING {
				out[unquote(b.Value)] = true
			}
		}
		return true
	})
	return out
}

// idHint derives the readable part of a generated ID, most specific
// source first:
//
//  1. a string-literal Text in a TextCfg nested in this literal. Most
//     widgets carry their label that way rather than in a Text field
//     of their own — ButtonCfg, the dominant type in the broken set,
//     has no Text field at all, it has Content []View.
//  2. the enclosing function's name, which is unique within a file.
//  3. the Cfg type with "Cfg" trimmed.
//
// The nested-Text search takes the first match in DFS order, which for
// a literal containing other widgets can pick a child's label. That is
// a legibility heuristic, not a correctness one: uniqueness comes from
// uniqueID, and every generated ID is reviewed in the diff.
func idHint(lit *ast.CompositeLit, enclosing, cfgName string) string {
	if s := slugify(nestedText(lit)); s != "" {
		return s
	}
	if s := slugify(enclosing); s != "" {
		return s
	}
	return slugify(strings.TrimSuffix(cfgName, "Cfg"))
}

// nestedText finds the first string-literal TextCfg.Text inside lit.
func nestedText(lit *ast.CompositeLit) string {
	var found string
	ast.Inspect(lit, func(n ast.Node) bool {
		if found != "" {
			return false
		}
		inner, ok := n.(*ast.CompositeLit)
		if !ok || structName(inner) != "TextCfg" {
			return true
		}
		if v, ok := litField(inner, "Text"); ok {
			if b, isLit := v.(*ast.BasicLit); isLit && b.Kind == token.STRING {
				found = unquote(b.Value)
			}
		}
		return true
	})
	return found
}

// enclosingFunc names the top-level function containing pos.
func enclosingFunc(f *ast.File, pos token.Pos) string {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if ok && fd.Pos() <= pos && pos < fd.End() {
			return fd.Name.Name
		}
	}
	return ""
}

// idScope names the file a literal lives in. Every example has a
// main.go, so for those the directory carries the meaning; elsewhere
// the file stem does, minus the view_ prefix every widget file shares.
func idScope(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), ".go")
	if stem == "main" || stem == "main_test" {
		return slugify(filepath.Base(filepath.Dir(path)))
	}
	return slugify(strings.TrimPrefix(stem, "view_"))
}

// slugify reduces a name to lower_snake_case over [a-z0-9_], splitting
// camelCase but keeping acronym runs whole (RTFView -> rtf_view).
func slugify(s string) string {
	var b strings.Builder
	runes := []rune(s)
	sep := true // true suppresses an underscore in this position
	for i, r := range runes {
		switch {
		case unicode.IsUpper(r):
			prevLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			prevUpper := i > 0 && unicode.IsUpper(runes[i-1])
			if !sep && (prevLower || (prevUpper && nextLower)) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			sep = false
		case unicode.IsLower(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			sep = false
		default:
			if !sep {
				b.WriteByte('_')
				sep = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// uniqueID makes an ID unique within one file and records it.
//
// IDs must be unique per window, and a file is the closest thing to a
// window the tool can see. That is the safe direction to err: a file
// holding two windows' views gets IDs that are unique anyway, whereas
// a window assembled from several files can still collide — which the
// gui.Debug duplicate-ID check reports at runtime.
//
// hintOnly IDs (from a function name) skip the scope prefix, because
// a function name is already file-unique and "command_test_test_x"
// reads worse than "test_x".
func uniqueID(taken map[string]bool, scope, hint string) string {
	base := hint
	if scope != "" && !strings.HasPrefix(hint, scope) {
		base = scope + "_" + hint
	}
	if !taken[base] {
		taken[base] = true
		return base
	}
	for n := 2; ; n++ {
		cand := base + "_" + strconv.Itoa(n)
		if !taken[cand] {
			taken[cand] = true
			return cand
		}
	}
}

// unquote strips the quotes from a Go string literal, falling back to
// the raw text when it does not unquote (an unterminated escape, say).
func unquote(s string) string {
	if v, err := strconv.Unquote(s); err == nil {
		return v
	}
	return strings.Trim(s, "`\"")
}
