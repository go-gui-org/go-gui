// Command eventctxfold reads `go build -gcflags=-e` output on stdin and
// folds every reported over-long callback call into an EventCtx.
//
// Calls made through a callback-typed variable or struct field cannot be
// found syntactically, because the callee's type is declared elsewhere.
// The compiler already locates them exactly and reports the argument
// list it saw, which is all the rewrite needs.
//
// Usage:
//
//	go build -gcflags=-e ./... 2>&1 | eventctxfold
package main

import (
	"fmt"
	"go/format"
	"io"
	"os"

	"github.com/go-gui-org/go-gui/tools/eventctx"
)

// #nosec G304 — paths come from compiler diagnostics for the tree the
// developer asked to rewrite
func main() {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal(err)
	}
	specs := eventctx.ParseBuildErrors(string(in))
	total := 0
	for file, sp := range specs {
		src, err := os.ReadFile(file)
		if err != nil {
			fatal(err)
		}
		// A spec the rewriter cannot make sense of is reported and
		// skipped; one odd call site must not block the rest.
		out, err := eventctx.FoldCalls(file, src, sp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "eventctxfold: skipped:", err)
			continue
		}
		formatted, ferr := format.Source(out)
		if ferr != nil {
			fmt.Fprintf(os.Stderr, "eventctxfold: %s: gofmt failed: %v\n", file, ferr)
			formatted = out
		}
		if err := writeBack(file, formatted); err != nil {
			fatal(err)
		}
		total += len(sp)
	}
	fmt.Fprintf(os.Stderr, "eventctxfold: folded %d calls in %d files\n",
		total, len(specs))
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

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "eventctxfold:", err)
	os.Exit(1)
}
