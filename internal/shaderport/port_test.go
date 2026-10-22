package shaderport

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

var update = flag.Bool("update", false, "rewrite the testdata/*.go.golden files")

// fixtures returns the corpus: testdata/*.frag.
func fixtures(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("testdata/*.frag")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	return files
}

// TestPortGolden ports every fixture and compares the Go file with
// testdata/<name>.go.golden. Re-record with -update after reading the
// diff.
func TestPortGolden(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			r, err := Port(string(src))
			if err != nil {
				t.Fatalf("Port: %v", err)
			}
			got, err := GoSource(r, "shaders", "ported", filepath.Base(f))
			if err != nil {
				t.Fatalf("GoSource: %v", err)
			}
			golden := strings.TrimSuffix(f, ".frag") + ".go.golden"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (record with -update)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from %s; re-record with -update after "+
					"reading the diff.\n--- got ---\n%s", golden, got)
			}
		})
	}
}

// TestPortGLSLCompiles wraps every fixture's GLSL output exactly as the
// GL (desktop 3.30) and web/Android (ES 3.00) backends do, and checks
// it with glslangValidator. The MSL side is compiled by
// TestShaderportCompiles in gui/backend/metal. Skipped when
// glslangValidator is not installed.
func TestPortGLSLCompiles(t *testing.T) {
	validator, err := exec.LookPath("glslangValidator")
	if err != nil {
		t.Skip("glslangValidator not installed")
	}
	dir := t.TempDir()
	for _, f := range fixtures(t) {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Port(string(src))
		if err != nil {
			t.Fatalf("%s: Port: %v", f, err)
		}
		for name, wrapped := range map[string]string{
			"330": gui.BuildGLSLFragmentDecls(r.GLSLDecls, r.GLSL),
			"es":  gui.BuildGLSLESFragmentDecls(r.GLSLDecls, r.GLSL),
		} {
			path := filepath.Join(dir, strings.TrimSuffix(filepath.Base(f), ".frag")+"_"+name+".frag")
			if err := os.WriteFile(path, []byte(wrapped), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(validator, path).CombinedOutput()
			if err != nil {
				t.Errorf("%s (%s) does not compile: %v\n%s", f, name, err, out)
			}
		}
	}
}

func TestPortAnimate(t *testing.T) {
	for src, want := range map[string]bool{
		"void mainImage(out vec4 o, in vec2 c) { o = vec4(iTime); }":                   true,
		"void mainImage(out vec4 o, in vec2 c) { o = vec4(float(iFrame)); }":           true,
		"void mainImage(out vec4 o, in vec2 c) { o = vec4(iResolution, 1.); }":         false,
		"#define T iTimeDelta\nvoid mainImage(out vec4 o, in vec2 c) { o = vec4(T); }": true,
		"vec4 frag_color = vec4(iTime);":                                               true,
		"vec4 frag_color = vec4(uv, 0.0, 1.0);":                                        false,
	} {
		r, err := Port(src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if r.Animate != want {
			t.Errorf("%q: Animate = %v, want %v", src, r.Animate, want)
		}
	}
}

// TestPortRejects checks that each construct outside the subset stops
// the port with a named error at the right position, and no output.
func TestPortRejects(t *testing.T) {
	const main = "void mainImage(out vec4 o, in vec2 c) {\n"
	for _, tc := range []struct {
		name, src string
		kind      error
		line, col int
		mention   string
	}{
		{"iMouse", main + "  o = iMouse;\n}", ErrUnsupported, 2, 7, "iMouse"},
		{"iChannel", main + "  o = texture(iChannel0, c);\n}", ErrUnsupported, 2, 7, "texture"},
		{"uniform", "uniform float speed;\n" + main + "}", ErrUnsupported, 1, 1, "uniforms"},
		{"mutable global", "float t;\n" + main + "}", ErrMutableGlobal, 1, 1, "float t;"},
		{"struct variable", "struct S { float a; } s;\n" + main + "}", ErrMutableGlobal, 1, 23, "struct S"},
		{"no entry", "void main() {}", ErrNoEntry, 0, 0, "mainImage"},
		{"inverse", main + "  mat2 m = inverse(mat2(1.));\n}", ErrUnsupported, 2, 12, "inverse"},
		{"xor", main + "  bool b = true ^^ false;\n}", ErrUnsupported, 2, 17, "^^"},
		{"unbalanced", main + "  o = vec4(1.0;\n}", ErrSyntax, 1, 39, "unclosed"},
		{"shadowed builtin", "float f(float iTime) { return iTime * iResolution.x; }\n" +
			main + "  o = vec4(f(1.));\n}", ErrUnsupported, 1, 8, "iTime"},
		{"array type first", main + "  float[2] a;\n}", ErrUnsupported, 2, 3, "name[N]"},
		{"include", "#include \"x.glsl\"\n" + main + "}", ErrUnsupported, 1, 1, "#include"},
		{"gl_FragCoord", main + "  o = gl_FragCoord;\n}", ErrUnsupported, 2, 7, "fragCoord"},
		{"in macro", "#define M iMouse.xy\n" + main + "  o = vec4(M, 0., 1.);\n}", ErrUnsupported, 1, 11, "iMouse"},
		{"out array", "void f(out float a[2]) { a[0] = 1.; }\n" + main + "}", ErrUnsupported, 1, 12, "array"},
		{"body texture", "vec4 frag_color = texture(tex, uv);", ErrUnsupported, 1, 19, "texture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Port(tc.src)
			if err == nil {
				t.Fatalf("no error; Metal output:\n%s%s", r.MetalDecls, r.Metal)
			}
			if !errors.Is(err, tc.kind) {
				t.Errorf("error %v is not %v", err, tc.kind)
			}
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("error %T is not *Error", err)
			}
			if pe.Line != tc.line || pe.Col != tc.col {
				t.Errorf("at %d:%d, want %d:%d (%v)", pe.Line, pe.Col, tc.line, tc.col, err)
			}
			if !strings.Contains(err.Error(), tc.mention) {
				t.Errorf("error %q does not mention %q", err, tc.mention)
			}
		})
	}
}

func TestGoSourceRejectsBadNames(t *testing.T) {
	r := &Result{GLSL: "vec4 frag_color = vec4(1.0);\n"}
	if _, err := GoSource(r, "main", "2bad", "x"); err == nil {
		t.Error("variable name 2bad accepted")
	}
	if _, err := GoSource(r, "my-pkg", "ok", "x"); err == nil {
		t.Error("package name my-pkg accepted")
	}
}

// A comment holding a backtick cannot sit in a raw string literal.
func TestGoSourceBacktick(t *testing.T) {
	r := &Result{GLSL: "// see `x`\nvec4 frag_color = vec4(1.0);\n"}
	out, err := GoSource(r, "main", "s", "x")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"// see `+"`x`"+`\nvec4`)) {
		t.Errorf("backtick source not quoted:\n%s", out)
	}
}
