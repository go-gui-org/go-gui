//go:build linux && !js && !android

package gl

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

// TestSoftRequested pins the by-name check (#1265). Only the exact value
// "soft" asks for the CPU renderer by name; matching is case-sensitive, as
// GOGUI_WAYLAND's is. The unset case is auto-pick's (TestPickRendererEnv),
// which softRequested does not run: it decides only whether native Wayland
// is declined.
func TestSoftRequested(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", false},
		{"gl", false},
		{"soft", true},
		{"SOFT", false},
		{"soft ", false},
		{"llvmpipe", false},
	}
	for _, c := range cases {
		t.Setenv("GOGUI_RENDERER", c.env)
		if got := softRequested(); got != c.want {
			t.Errorf("GOGUI_RENDERER=%q: softRequested = %v, want %v", c.env, got, c.want)
		}
	}
}

// TestSoftDeclinesWayland: until the Wayland soft path lands (#1265 step 4),
// GOGUI_RENDERER=soft runs through XWayland, so waylandRequested reports false
// even on a Wayland session and even when GOGUI_WAYLAND=1 forces a try.
func TestSoftDeclinesWayland(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "soft")
	for _, env := range []string{"", "1"} {
		t.Setenv("GOGUI_WAYLAND", env)
		t.Setenv("WAYLAND_DISPLAY", "wayland-0")
		if waylandRequested() {
			t.Errorf("GOGUI_RENDERER=soft GOGUI_WAYLAND=%q: waylandRequested = true, want false", env)
		}
	}
}

// TestGPUContextErrorNamesSoftOnLinux: on Linux the no-GPU message points at
// GOGUI_RENDERER=soft, the one fix that needs no driver or package install.
// It stays a single line (TestGPUContextErrorMessageNamesFix).
func TestGPUContextErrorNamesSoftOnLinux(t *testing.T) {
	msg := newGPUContextError("init EGL display",
		errors.New("eglInitialize failed")).Error()
	if !strings.Contains(msg, "GOGUI_RENDERER=soft") {
		t.Fatalf("message %q does not name GOGUI_RENDERER=soft", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Fatalf("message must stay one line, got %q", msg)
	}
}

// resetAutoProbe forgets the cached auto-pick, so the next pickRenderer with
// GOGUI_RENDERER unset runs the probe again. It also restores the probe seam
// when the test ends.
func resetAutoProbe(t *testing.T, probe func() probeResult) {
	t.Helper()
	old := glRendererProbeFunc
	glRendererProbeFunc = probe
	autoProbe.once = sync.Once{}
	autoProbe.choice = rendererChoice{}
	t.Cleanup(func() {
		glRendererProbeFunc = old
		autoProbe.once = sync.Once{}
		autoProbe.choice = rendererChoice{}
	})
}

// TestPickRendererEnv pins how GOGUI_RENDERER drives the choice (#1265 step
// 3): "soft" and "gl" never probe, unknown values keep GL without probing,
// and only the unset case probes, once per process.
func TestPickRendererEnv(t *testing.T) {
	calls := 0
	resetAutoProbe(t, func() probeResult {
		calls++
		return probeResult{renderer: "llvmpipe (LLVM 15.0.7, 256 bits)"}
	})
	cases := []struct {
		env  string
		soft bool
	}{
		{"soft", true},
		{"gl", false},
		{"Soft", false},
		{"soft ", false},
		{"auto", false},
	}
	for _, c := range cases {
		t.Setenv("GOGUI_RENDERER", c.env)
		if got := pickRenderer().soft; got != c.soft {
			t.Errorf("GOGUI_RENDERER=%q: soft = %v, want %v", c.env, got, c.soft)
		}
	}
	if calls != 0 {
		t.Fatalf("a named or unknown GOGUI_RENDERER ran the probe %d times", calls)
	}
	t.Setenv("GOGUI_RENDERER", "")
	for range 2 {
		if !pickRenderer().soft {
			t.Fatal("unset GOGUI_RENDERER on llvmpipe did not pick soft")
		}
	}
	if calls != 1 {
		t.Fatalf("unset GOGUI_RENDERER probed %d times, want 1", calls)
	}
}

// TestAutoPick pins the decision table. An EGL failure picks soft and keeps
// the cause; a probe that fails for its own reasons keeps GL, because the
// real window does not need the probe's surfaceless context.
func TestAutoPick(t *testing.T) {
	eglErr := errors.New("eglInitialize failed (egl error 0x3001)")
	cases := []struct {
		name    string
		res     probeResult
		soft    bool
		keepErr bool
		logs    string // substring of the log line; "" means no line
	}{
		{"hardware", probeResult{renderer: "Mesa Intel(R) UHD Graphics 620 (KBL GT2)"},
			false, false, ""},
		{"llvmpipe", probeResult{renderer: "llvmpipe (LLVM 15.0.7, 256 bits)"},
			true, false, "GOGUI_RENDERER=gl keeps GL"},
		{"egl failure", probeResult{eglErr: eglErr},
			true, true, "no GPU context"},
		{"probe failure", probeResult{probeErr: errors.New("no glGetString")},
			false, false, "using GL"},
		// The probe's config can differ from the window's, so a context
		// failure on it must keep GL, not cache soft for the process.
		{"context failure", probeResult{probeErr: errors.New("eglCreateContext(3.3 core) failed")},
			false, false, "using GL"},
	}
	for _, c := range cases {
		got, why := autoPick(c.res)
		if got.soft != c.soft {
			t.Errorf("%s: soft = %v, want %v", c.name, got.soft, c.soft)
		}
		if got.auto != c.soft {
			t.Errorf("%s: auto = %v, want %v (only a soft choice is marked)",
				c.name, got.auto, c.soft)
		}
		if (got.eglErr != nil) != c.keepErr {
			t.Errorf("%s: eglErr = %v, want kept %v", c.name, got.eglErr, c.keepErr)
		}
		if c.logs == "" && why != "" {
			t.Errorf("%s: logs %q, want nothing", c.name, why)
		}
		if c.logs != "" && !strings.Contains(why, c.logs) {
			t.Errorf("%s: log %q does not contain %q", c.name, why, c.logs)
		}
	}
}

// TestIsSoftwareRasterizer pins the GL_RENDERER strings that count as a CPU
// driver. Real strings carry versions around the name, and a hardware driver
// must never match: that would put a GPU machine on the CPU renderer.
func TestIsSoftwareRasterizer(t *testing.T) {
	cases := []struct {
		renderer string
		want     bool
	}{
		{"llvmpipe (LLVM 15.0.7, 256 bits)", true},
		{"softpipe", true},
		{"zink Vulkan 1.3(llvmpipe (LLVM 17.0.6, 256 bits) (MESA_LLVMPIPE))", true},
		{"zink (lavapipe)", true},
		{"Google SwiftShader", true},
		{"SwiftShader Device (Subzero)", true},
		{"Mesa Intel(R) UHD Graphics 620 (KBL GT2)", false},
		{"AMD Radeon RX 6600 (radeonsi, navi23, LLVM 16.0.6, DRM 3.54)", false},
		{"NVIDIA GeForce RTX 3060/PCIe/SSE2", false},
		{"virgl (Mesa)", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isSoftwareRasterizer(c.renderer); got != c.want {
			t.Errorf("isSoftwareRasterizer(%q) = %v, want %v", c.renderer, got, c.want)
		}
	}
}

// TestSoftVisualRejected pins newX11's answer to an X server with no visual
// the CPU renderer can write into (a 16-bit or big-endian server). An
// auto-pick from GL_RENDERER falls back to GL, which drew there before
// auto-pick; an auto-pick after an EGL failure reports the GPU error first,
// so it still matches ErrNoGPUContext, and keeps both causes; a named
// GOGUI_RENDERER=soft reports why soft cannot run.
func TestSoftVisualRejected(t *testing.T) {
	useGL, err := softVisualRejected(rendererChoice{soft: true, auto: true})
	if !useGL || err != nil {
		t.Errorf("auto from GL_RENDERER: useGL=%v err=%v, want GL and no error", useGL, err)
	}

	eglErr := errors.New("eglInitialize failed (egl error 0x3001)")
	useGL, err = softVisualRejected(rendererChoice{soft: true, auto: true,
		eglErr: eglErr})
	if useGL {
		t.Error("auto after EGL failure fell back to GL, which cannot run")
	}
	if !errors.Is(err, ErrNoGPUContext) {
		t.Errorf("auto after EGL failure: %v does not match ErrNoGPUContext", err)
	}
	if !errors.Is(err, eglErr) {
		t.Errorf("auto after EGL failure: %v does not keep the EGL cause", err)
	}
	if err != nil && !strings.Contains(err.Error(), "gl: init EGL display:") {
		t.Errorf("auto after EGL failure: %q does not name the failed EGL step", err)
	}
	if err != nil && strings.Contains(err.Error(), "GOGUI_RENDERER=soft") {
		t.Errorf("auto after EGL failure: %q advises soft, which just failed", err)
	}
	if err != nil && !strings.Contains(err.Error(), errNoSoftVisual.Error()) {
		t.Errorf("auto after EGL failure: %q does not name the soft cause", err)
	}

	useGL, err = softVisualRejected(rendererChoice{soft: true})
	if useGL || !errors.Is(err, errNoSoftVisual) {
		t.Errorf("named soft: useGL=%v err=%v, want errNoSoftVisual", useGL, err)
	}
	if errors.Is(err, ErrNoGPUContext) {
		t.Errorf("named soft: %v matches ErrNoGPUContext, but GL was never tried", err)
	}
}
