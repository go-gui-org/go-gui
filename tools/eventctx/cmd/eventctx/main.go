// Command eventctx rewrites legacy three-argument event callbacks to
// the single-argument EventCtx form across a tree of Go files.
//
// Usage:
//
//	eventctx [-w] [-report file] path...
//
// Without -w the rewritten source is printed to stdout and nothing is
// modified. The rule-4 report (return paths in consume-class callbacks
// that may now need an explicit ctx.Bubble()) goes to stderr, or to the
// file named by -report.
package main

import (
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-gui-org/go-gui/tools/eventctx"
)

func main() {
	write := flag.Bool("w", false, "write result to source file")
	report := flag.String("report", "", "write rule-4 report to this file")
	decls := flag.Bool("decls", false,
		"declaration mode: convert named callback functions and their "+
			"call sites instead of closures. Run after the closure pass.")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr,
			"usage: eventctx [-w] [-decls] [-report file] path...")
		os.Exit(2)
	}

	files, err := collect(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "eventctx:", err)
		os.Exit(1)
	}

	// Declaration mode needs a whole-package view before touching any
	// file: a function converts only if some other file uses its name
	// as a value rather than only calling it.
	var plan eventctx.DeclPlan
	if *decls {
		plan, err = buildPlan(files)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eventctx:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "eventctx: converting %d declarations\n",
			len(plan))
	}

	var findings []eventctx.Finding
	changed := 0
	for _, p := range files {
		n, fs2, err := processFile(p, *write, plan)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eventctx:", err)
			os.Exit(1)
		}
		changed += n
		findings = append(findings, fs2...)
	}

	out := os.Stderr
	if *report != "" {
		f, err := os.Create(*report)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eventctx:", err)
			os.Exit(1)
		}
		defer func() { _ = f.Close() }()
		out = f
	}
	for _, f := range findings {
		_, _ = fmt.Fprintln(out, f)
	}
	fmt.Fprintf(os.Stderr, "eventctx: %d files changed, %d review items\n",
		changed, len(findings))
}

// writeBack rewrites a file in place, keeping the mode it already had
// rather than imposing one.
func writeBack(path string, content []byte) error {
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(path, content, mode)
}

func skipDir(name string) bool {
	return name == "testdata" || name == ".git" || name == "vendor"
}

// collect walks the roots and returns every .go file under them.
func collect(roots []string) ([]string, error) {
	var out []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDir(d.Name()) && p != root {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// buildPlan unions the candidate declarations and the value-uses across
// every file, then keeps only the candidates that are used as values.
//
// #nosec G304 — paths come from the developer-supplied CLI arguments
func buildPlan(files []string) (eventctx.DeclPlan, error) {
	cands := eventctx.DeclPlan{}
	used := map[string]bool{}
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		c, u, err := eventctx.ScanDecls(p, src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		maps.Copy(cands, c)
		for k := range u {
			used[k] = true
		}
	}
	plan := eventctx.DeclPlan{}
	for name, sig := range cands {
		if used[name] {
			plan[name] = sig
		}
	}
	return plan, nil
}

// #nosec G304 — path comes from the developer-supplied CLI arguments
func processFile(
	path string, write bool, plan eventctx.DeclPlan,
) (int, []eventctx.Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, err
	}
	if !eventctx.NeedsRewrite(src) {
		return 0, nil, nil
	}
	res, err := eventctx.Rewrite(path, src, plan)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %w", path, err)
	}
	if !res.Changed {
		return 0, res.Findings, nil
	}
	formatted, err := format.Source(res.Src)
	if err != nil {
		// Emit the unformatted result so the breakage is inspectable.
		formatted = res.Src
		fmt.Fprintf(os.Stderr, "eventctx: %s: gofmt failed: %v\n", path, err)
	}
	if !write {
		_, _ = os.Stdout.Write(formatted)
		return 1, res.Findings, nil
	}
	if err := writeBack(path, formatted); err != nil {
		return 0, nil, err
	}
	return 1, res.Findings, nil
}
