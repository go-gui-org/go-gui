package shaderport

import (
	"fmt"
	"go/format"
	gotoken "go/token"
	"strconv"
	"strings"
)

// GoSource writes r as a Go file declaring `var <name> = &gui.Shader{...}`
// in package pkg. from names the input in the header comment. The file
// is gofmt-formatted.
func GoSource(r *Result, pkg, name, from string) ([]byte, error) {
	if !gotoken.IsIdentifier(pkg) {
		return nil, fmt.Errorf("package name %q is not a Go identifier", pkg)
	}
	if !gotoken.IsIdentifier(name) {
		return nil, fmt.Errorf("variable name %q is not a Go identifier", name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// Ported by shaderport from %s.\n", from)
	b.WriteString("// Review it before use: see docs/shader-porting.md for what the\n")
	b.WriteString("// translation covers.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import \"github.com/go-gui-org/go-gui/gui\"\n\n")
	fmt.Fprintf(&b, "// %s draws the ported shader. Set it as a container's Shader.\n", name)
	fmt.Fprintf(&b, "var %s = &gui.Shader{\n", name)
	for _, f := range []struct{ field, value string }{
		{"Metal", r.Metal},
		{"MetalDecls", r.MetalDecls},
		{"GLSL", r.GLSL},
		{"GLSLDecls", r.GLSLDecls},
	} {
		if f.value != "" {
			fmt.Fprintf(&b, "%s: %s,\n", f.field, goString(f.value))
		}
	}
	if r.Animate {
		b.WriteString("Animate: true,\n")
	}
	b.WriteString("}\n")
	return format.Source([]byte(b.String()))
}

// goString quotes s as a raw string, which keeps shader source readable
// in review. A backtick (possible only in a comment) cannot appear in a
// raw string, so that source falls back to an interpreted literal.
func goString(s string) string {
	if strings.Contains(s, "`") {
		return strconv.Quote(s)
	}
	return "`\n" + s + "`"
}
