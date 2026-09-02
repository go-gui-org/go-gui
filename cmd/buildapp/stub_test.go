package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// crossStub compiles a trivial main package for goos/goarch and returns
// the path of the resulting executable.  The Windows and Linux
// packagers both need a real, well-formed binary of the target format,
// and cross-compiling one is cheaper and more faithful than checking in
// a fixture.
func crossStub(t *testing.T, goos, goarch, name string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "stub.go")
	const prog = "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(src, []byte(prog), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out, src)
	cmd.Env = append(os.Environ(),
		"GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot cross-compile for %s/%s: %v: %s", goos, goarch, err, b)
	}
	return out
}

// writeTestPNG writes a solid px-by-px PNG and returns its path.
func writeTestPNG(t *testing.T, px int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, px, px))
	for y := range px {
		for x := range px {
			img.Set(x, y, color.RGBA{R: 0x30, G: 0x60, B: 0xc0, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
