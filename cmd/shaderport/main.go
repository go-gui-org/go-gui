// shaderport turns a Shadertoy GLSL shader into a Go file declaring a
// *gui.Shader with both the Metal and the GLSL body filled in, so the
// shader runs on every GPU backend (#573).
//
// Usage:
//
//	shaderport [-pkg main] [-name shader] [-o out.go] [file.frag]
//
// With no file, shaderport reads stdin. With no -o, it writes stdout.
// The input is a Shadertoy shader with mainImage(out vec4, in vec2), or
// a go-gui body that declares frag_color. A construct outside the
// supported subset (iMouse, iChannel*, texture(), mutable globals, ...)
// stops it with the source position and exit status 1; it never writes
// a partial file. docs/shader-porting.md lists the subset.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/go-gui-org/go-gui/internal/shaderport"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "shaderport:", err)
		os.Exit(1)
	}
}

// run is main without the process: testable with buffers.
func run(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("shaderport", flag.ContinueOnError)
	pkg := fs.String("pkg", "main", "package name of the generated file")
	name := fs.String("name", "shader", "variable name of the *gui.Shader")
	out := fs.String("o", "", "output file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("want at most one input file, got %d", fs.NArg())
	}
	from := "stdin"
	var src []byte
	var err error
	if fs.NArg() == 1 {
		from = fs.Arg(0)
		src, err = os.ReadFile(from)
	} else {
		src, err = io.ReadAll(stdin)
	}
	if err != nil {
		return err
	}
	r, err := shaderport.Port(string(src))
	if err != nil {
		return fmt.Errorf("%s:%w", from, prefixColon(err))
	}
	code, err := shaderport.GoSource(r, *pkg, *name, from)
	if err != nil {
		return err
	}
	if *out == "" {
		_, err = stdout.Write(code)
		return err
	}
	// #nosec G306 — standard 0644 for generated Go source
	return os.WriteFile(*out, code, 0o644)
}

// prefixColon keeps "file:line:col: ..." readable when the error has a
// position, and "file: ..." when it has none.
func prefixColon(err error) error {
	var pe *shaderport.Error
	if errors.As(err, &pe) && pe.Line > 0 {
		return err
	}
	return fmt.Errorf(" %w", err)
}
