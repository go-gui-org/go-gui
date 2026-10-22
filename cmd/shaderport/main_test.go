package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/internal/shaderport"
)

const shader = "void mainImage(out vec4 o, in vec2 c) { o = vec4(sin(iTime)); }\n"

func TestRunStdinToStdout(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-pkg", "shaders", "-name", "Wave"}, strings.NewReader(shader), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package shaders", "var Wave = &gui.Shader{",
		"MetalDecls:", "GLSLDecls:", "Animate: true", "from stdin"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestRunFileToFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "wave.frag")
	outPath := filepath.Join(dir, "wave.go")
	if err := os.WriteFile(in, []byte(shader), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{"-o", outPath, in}, nil, &stdout); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Errorf("wrote to stdout with -o: %q", stdout.String())
	}
	code, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(code, []byte("var shader = &gui.Shader{")) {
		t.Errorf("default name not used:\n%s", code)
	}
}

// A rejected shader fails with file:line:col and writes nothing.
func TestRunRejectWritesNothing(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "mouse.frag")
	outPath := filepath.Join(dir, "mouse.go")
	src := "void mainImage(out vec4 o, in vec2 c) {\n  o = iMouse;\n}\n"
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"-o", outPath, in}, nil, &bytes.Buffer{})
	if !errors.Is(err, shaderport.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if !strings.HasPrefix(err.Error(), in+":2:7: ") {
		t.Errorf("err = %q, want prefix %q", err, in+":2:7: ")
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Errorf("output file written for a rejected shader")
	}
}

func TestRunNoPositionError(t *testing.T) {
	err := run(nil, strings.NewReader("void main() {}"), &bytes.Buffer{})
	if !errors.Is(err, shaderport.ErrNoEntry) {
		t.Fatalf("err = %v, want ErrNoEntry", err)
	}
	if !strings.HasPrefix(err.Error(), "stdin: ") {
		t.Errorf("err = %q, want prefix %q", err, "stdin: ")
	}
}

func TestRunBadArgs(t *testing.T) {
	if err := run([]string{"a.frag", "b.frag"}, nil, &bytes.Buffer{}); err == nil {
		t.Error("two inputs accepted")
	}
	if err := run([]string{"-name", "1x"}, strings.NewReader(shader), &bytes.Buffer{}); err == nil {
		t.Error("bad -name accepted")
	}
}
