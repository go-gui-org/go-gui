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
		if err := os.WriteFile(file, formatted, 0o644); err != nil {
			fatal(err)
		}
		total += len(sp)
	}
	fmt.Fprintf(os.Stderr, "eventctxfold: folded %d calls in %d files\n",
		total, len(specs))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "eventctxfold:", err)
	os.Exit(1)
}
