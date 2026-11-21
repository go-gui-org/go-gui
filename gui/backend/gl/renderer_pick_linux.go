//go:build linux && !js && !android

package gl

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// This file is step 3 of #1265: with GOGUI_RENDERER unset, the X11 backend
// picks the CPU renderer (gui/backend/soft) by itself when GL would run on a
// software rasterizer anyway, or when there is no GPU context at all.
//
// The decision is made once per process, by a probe that runs before the first
// X11 window: open the EGL display, make a context with no surface, read
// GL_RENDERER, and tear the context down. It is not made per window after the
// real context exists, because by then the window is mapped with a visual that
// EGL chose, and switching it to soft would mean a second teardown path and a
// recreated window.
//
// The probe looks at the GL_RENDERER string, not at whether a GPU exists. On a
// Cinnamon Wayland session the compositor gives clients no GPU device, so EGL
// hands out llvmpipe on hardware whose GPU works under X11 (#1265, step 1).
//
// The native Wayland backend makes the same choice on its own EGL display
// (step 4, waylandAutoSoft in softpresent_wayland_linux.go). The two are not
// shared: on a Cinnamon Wayland session the Wayland display reports llvmpipe
// while the X11 one reports the GPU, so a Wayland result must not decide for
// an X11 window the Wayland backend fell back to.

// rendererChoice is what newX11 draws with.
type rendererChoice struct {
	// soft: draw with gui/backend/soft and present with MIT-SHM or PutImage.
	soft bool
	// auto: the probe chose, not GOGUI_RENDERER by name. An auto choice of
	// soft may still fall back to GL (softVisualRejected).
	auto bool
	// eglErr is the EGL failure that made auto-pick choose soft. It is nil
	// when soft was asked for by name or chosen from GL_RENDERER. newX11
	// keeps it so that a soft path that also fails reports the GPU cause,
	// which still matches ErrNoGPUContext.
	eglErr error
}

// pickRenderer decides how the next X11 window draws. GOGUI_RENDERER is read
// on each call; only the auto case is cached, because only it costs a probe.
//
//   - "soft": the CPU renderer, with no EGL at all.
//   - "gl": EGL/GL, with no probe. An EGL failure is an error
//     (ErrNoGPUContext), as before #1265.
//   - unset or empty: the probe decides (autoPick).
//   - anything else: GL, with one warning. A typo such as "Soft" must not
//     silently cost a probe or pick a renderer the user did not name.
func pickRenderer() rendererChoice {
	switch readRendererEnv() {
	case rendererEnvSoft:
		return rendererChoice{soft: true}
	case rendererEnvAuto:
		return autoPickCached()
	}
	return rendererChoice{}
}

// autoProbe caches the probe and the choice made from it. A test that needs
// a fresh probe resets it with resetAutoProbe.
var autoProbe struct {
	once   sync.Once
	choice rendererChoice
}

// glRendererProbeFunc is the seam tests replace: it runs the probe without
// EGL. The real probe must run on the OS thread that newX11 locked, because
// eglMakeCurrent binds the context to the calling thread.
var glRendererProbeFunc = probeGLRenderer

// autoPickCached runs the probe once per process and logs the choice when
// it is soft, so a user who sees the CPU renderer knows why and how to undo
// it.
func autoPickCached() rendererChoice {
	autoProbe.once.Do(func() {
		res := glRendererProbeFunc()
		c, why := autoPick(res)
		if why != "" {
			log.Print(why)
		}
		autoProbe.choice = c
	})
	return autoProbe.choice
}

// probeResult is what the probe found. At most one of eglErr and probeErr is
// set; renderer is set only when both are nil.
type probeResult struct {
	renderer string
	// eglErr: the EGL display could not be opened. The real window would
	// fail at the same call.
	eglErr error
	// probeErr: the probe itself could not finish (no context on the
	// probe's config, no surfaceless context, no glGetString). The real
	// window picks its own config and has a surface, so it may still work.
	probeErr error
}

// autoPick turns a probe result into a choice and the one log line that
// explains it ("" when the choice is the plain GL default).
//
// A probe that fails for its own reasons keeps GL: the probe uses a context
// with no surface, which the real window does not need, so a failure there
// says nothing about the window. An EGL failure picks soft, because the window
// would fail at the same EGL call and soft needs no EGL.
func autoPick(res probeResult) (rendererChoice, string) {
	switch {
	case res.eglErr != nil:
		return rendererChoice{soft: true, auto: true, eglErr: res.eglErr},
			fmt.Sprintf("gl: no GPU context (%v); drawing with the CPU renderer "+
				"(GOGUI_RENDERER=gl reports the error instead)", res.eglErr)
	case res.probeErr != nil:
		return rendererChoice{},
			fmt.Sprintf("gl: could not read GL_RENDERER (%v); using GL", res.probeErr)
	case isSoftwareRasterizer(res.renderer):
		return rendererChoice{soft: true, auto: true},
			fmt.Sprintf("gl: GL_RENDERER %q is a software rasterizer; drawing with "+
				"the CPU renderer (GOGUI_RENDERER=gl keeps GL; custom shaders are "+
				"not drawn)", res.renderer)
	}
	return rendererChoice{}, ""
}

// newGPUContextErrorNoSoft is newGPUContextError for a failure where the CPU
// renderer was the fallback and could not run either, so advising
// GOGUI_RENDERER=soft would be wrong.
func newGPUContextErrorNoSoft(op string, cause error) error {
	return &gpuContextError{op: op, cause: cause, noSoft: true}
}

// errNoSoftVisual: the X server has no visual the CPU renderer can write into
// (a 32 bpp little-endian TrueColor ZPixmap with 8-bit channels).
var errNoSoftVisual = errors.New("gl: CPU renderer: the X server " +
	"offers no 32 bpp little-endian TrueColor visual")

// softVisualRejected decides what newX11 does when the soft choice meets an X
// server with no visual soft can write into. useGL is true when GL should draw
// instead; otherwise err is what newX11 returns.
//
//   - Auto-pick from GL_RENDERER: GL works on this machine, only slowly, so
//     it draws. Failing here would break a server (16-bit depth, big-endian)
//     on which GL drew before auto-pick existed.
//   - Auto-pick after an EGL failure: neither renderer can run. The GPU
//     error comes first, so the error still matches ErrNoGPUContext, and
//     the soft cause follows.
//   - GOGUI_RENDERER=soft by name: the user asked for soft; say why it
//     cannot run rather than quietly drawing with GL.
func softVisualRejected(pick rendererChoice) (useGL bool, err error) {
	switch {
	case pick.eglErr != nil:
		return false, fmt.Errorf("%w (CPU renderer fallback: %w)",
			newGPUContextErrorNoSoft("init EGL display", pick.eglErr), errNoSoftVisual)
	case pick.auto:
		softVisualOnce.Do(func() {
			log.Printf("%v; using GL", errNoSoftVisual)
		})
		return true, nil
	}
	return false, errNoSoftVisual
}

// softVisualOnce keeps softVisualRejected's GL fallback notice to one line
// per process: the server, and so the answer, is the same for every window.
var softVisualOnce sync.Once

// softRasterizers are the GL_RENDERER substrings, in lower case, of drivers
// that draw on the CPU. Mesa reports "llvmpipe (LLVM 15.0.7, 256 bits)",
// "softpipe" and, for Vulkan-backed GL through Zink, "zink (lavapipe)".
// SwiftShader reports "SwiftShader Device ..." or "Google SwiftShader".
var softRasterizers = []string{"llvmpipe", "softpipe", "lavapipe", "swiftshader"}

// isSoftwareRasterizer reports whether a GL_RENDERER string names a driver
// that draws on the CPU. Matching is a case-insensitive substring test,
// because the strings carry version and build details around the name.
func isSoftwareRasterizer(renderer string) bool {
	r := strings.ToLower(renderer)
	for _, s := range softRasterizers {
		if strings.Contains(r, s) {
			return true
		}
	}
	return false
}

// probeGLRenderer opens the EGL display the X11 window would use, makes a GL
// 3.3 core context current with no surface (EGL_KHR_surfaceless_context,
// which Mesa offers on every driver), and reads GL_RENDERER from it.
//
// When it finds a hardware GL, the display stays initialized: newX11
// initializes the same display again, which EGL treats as a no-op, so the
// probe adds only one context create and destroy to a GPU start. When it
// finds a software rasterizer, or EGL fails, the display is terminated. That
// frees the display's objects, not the driver: Mesa keeps the driver library
// mapped (about 70 MB for llvmpipe, measured at #1265 step 4), which
// GOGUI_RENDERER=soft avoids by loading no EGL at all.
func probeGLRenderer() (res probeResult) {
	dpy, cands, err := eglInitDisplayNFunc()
	if err != nil {
		return probeResult{eglErr: err}
	}
	// Registered first, so it runs last: after the context is released
	// and destroyed. A probe error keeps the display, as GL is kept then.
	defer func() {
		if res.eglErr != nil || isSoftwareRasterizer(res.renderer) {
			eglTerminate(dpy)
		}
	}()
	return probeContext(dpy, cands[0].config)
}

// probeContext makes a GL 3.3 core context current on dpy with no surface,
// reads GL_RENDERER and tears the context down. It never sets eglErr: the
// display is already open. The X11 probe (probeGLRenderer) and the Wayland
// one (openWaylandDisplay) share it; each owns its display.
func probeContext(dpy, config uintptr) probeResult {
	ctxAttribs := []int32{
		eglContextMajorVersion, 3,
		eglContextMinorVersion, 3,
		eglContextOpenGLProfileMask, eglContextOpenGLCoreProfileBit,
		eglNone,
	}
	ctx := eglCreateContext(dpy, config, eglNoContext, unsafe.Pointer(&ctxAttribs[0]))
	if ctx == 0 {
		// Not an eglErr: newX11 creates its context on the config pickVisual
		// chooses (an ARGB one for a transparent window), which can differ
		// from cands[0]. A failure on this config says nothing sure about
		// the window's, and an eglErr here would cache soft for the process.
		return probeResult{probeErr: fmt.Errorf(
			"eglCreateContext(3.3 core) failed (egl error 0x%x)", eglGetError())}
	}
	defer eglDestroyContext(dpy, ctx)
	if eglMakeCurrent(dpy, 0, 0, ctx) == 0 {
		return probeResult{probeErr: fmt.Errorf(
			"surfaceless eglMakeCurrent failed (egl error 0x%x)", eglGetError())}
	}
	// Release the context before it is destroyed, so this thread holds no
	// current context when newX11 makes its own.
	defer eglMakeCurrent(dpy, 0, 0, 0)

	addr := eglProc("glGetString")
	if addr == 0 {
		return probeResult{probeErr: errors.New("no glGetString")}
	}
	var getString func(uint32) uintptr
	purego.RegisterFunc(&getString, addr)
	name := cGoString(getString(glRenderer))
	if name == "" {
		return probeResult{probeErr: errors.New("glGetString(GL_RENDERER) returned nothing")}
	}
	return probeResult{renderer: name}
}
