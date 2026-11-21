//go:build windows && !js && !cgo

package gl

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// keepsGLChildEnv marks the subprocess TestUnsetWithMesaKeepsGL starts.
const keepsGLChildEnv = "GOGUI_TEST_KEEPS_GL_CHILD"

// TestUnsetWithMesaKeepsGL: with GOGUI_RENDERER unset, a machine whose GL
// comes up stays on GL: no fallback, the process is on GL, and a custom
// shader's fill is in the read-back pixels. Under tools/glrendertest that GL
// is Mesa's opengl32.dll beside the test binary, which an application ships
// to keep its shaders. Not built with cgo, like the other tests that draw
// with GL (see render_backend_cgo_test.go).
//
// The window is the first of its process, the automatic attempt, with no
// opengl32.dll loaded and no context made before it: that is where a Mesa
// beside the exe gets no context when the DLL loads after SetPixelFormat
// (#1310, phase 2). In a test process where an earlier test loaded the DLL,
// the test reruns itself alone in a subprocess. With Mesa beside the binary
// the context must be Mesa's; with GOGUI_REQUIRE_GL set (the Mesa GL
// selection) Mesa must be there. Elsewhere it may be the machine's own GL,
// and a machine whose GL does not come up skips.
func TestUnsetWithMesaKeepsGL(t *testing.T) {
	if openGL32Loaded() {
		if os.Getenv(keepsGLChildEnv) != "" {
			t.Fatal("opengl32.dll was loaded before the test started, in a process that runs only this test")
		}
		cmd := exec.Command(os.Args[0], "-test.run", "^TestUnsetWithMesaKeepsGL$", "-test.v") // #nosec G204 -- this test binary, a fixed test name
		cmd.Env = append(os.Environ(), keepsGLChildEnv+"=1")
		out, err := cmd.CombinedOutput()
		t.Logf("opengl32.dll already loaded in this process; ran in a subprocess:\n%s", out)
		if err == nil && strings.Contains(string(out), "--- SKIP: TestUnsetWithMesaKeepsGL") {
			t.Skip("the subprocess skipped")
		}
		if err != nil || !strings.Contains(string(out), "--- PASS: TestUnsetWithMesaKeepsGL") {
			t.Fatalf("subprocess did not pass: %v", err)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	mesa := filepath.Join(filepath.Dir(exe), "opengl32.dll")
	_, statErr := os.Stat(mesa)
	mesaBeside := statErr == nil
	requireGL := os.Getenv("GOGUI_REQUIRE_GL") != ""
	if requireGL && !mesaBeside {
		t.Fatalf("GOGUI_REQUIRE_GL is set, as in the Mesa GL selection, but %s does not exist", mesa)
	}

	l := newAutoProcess(t)
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	b, err := New(w)
	skipNoWindow(t, err)
	if err != nil {
		t.Fatalf("New with GOGUI_RENDERER unset: %v", err)
	}
	t.Cleanup(b.Destroy)
	if b.plat.softMode {
		if requireGL || mesaBeside {
			t.Fatalf("a GL that comes up was replaced: the first window fell back; log:\n%s", l)
		}
		t.Skipf("no GL comes up here: the first window fell back; log:\n%s", l)
	}
	if b.plat.hglrc == 0 {
		t.Fatal("a GL window with no context")
	}
	if lines := l.fallbackLines(); len(lines) != 0 {
		t.Errorf("logged a fallback: %q", lines)
	}
	if got := chosenRenderer(); got != win32RendererGL {
		t.Errorf("process renderer = %d, want GL", got)
	}
	if !w.RendersShaders() {
		t.Error("the window does not report shaders")
	}
	lib := openGL32().dll
	identity := glIdentity()
	t.Logf("%s, %s", lib.Name, identity)
	if mesaBeside {
		if lib.System || !strings.EqualFold(lib.Name, mesa) {
			t.Fatalf("loaded %s (system %v), not the Mesa beside the binary, %s", lib.Name, lib.System, mesa)
		}
		if version := glVersionString(); !strings.Contains(version, "Mesa") {
			t.Fatalf("GL_VERSION %q does not name Mesa: the context is not Mesa's (%s)",
				version, identity)
		}
	}

	// Draw straight into the read-back framebuffer, as newBatchProbeBackend
	// sets up for the GL tests.
	b.msaaMain.destroy()
	b.msaaFilter.destroy()
	b.msaaSamples = 1
	target := newReadbackTarget(t, b)
	cmd := gui.RenderCmd{Kind: gui.RenderCustomShader,
		Shader: &gui.Shader{GLSL: "vec4 frag_color = vec4(0.0, 1.0, 0.0, 1.0);"},
		W:      float32(b.physW) / b.dpiScale, H: float32(b.physH) / b.dpiScale}
	pixels := target.render(b, func() { b.drawCustomShader(&cmd) })
	if got := pixelAt(b, pixels, int(b.physW)/2, int(b.physH)/2); !nearPixel(got, green) {
		t.Errorf("custom shader fill = %v, want %v", got, green)
	}
}

// glVersionString is GL_VERSION of the current context, where Mesa names
// itself.
func glVersionString() string {
	getString := glProc("glGetString")
	if getString == 0 {
		return ""
	}
	r, _, _ := syscall.SyscallN(getString, glVersion)
	return cGoString(r)
}
