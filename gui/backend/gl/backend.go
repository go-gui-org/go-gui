//go:build !js && !darwin

// Package gl provides an OpenGL 3.3 backend for go-gui.
//
// Windowing, GL-context creation, and the event loop are
// platform-specific and selected by build tag: native X11+EGL on Linux
// (platform_x11.go), native Win32+WGL on Windows (platform_win32.go).
// The rendering pipeline in this and the other shared files
// (draw.go, pipeline.go, buffers.go, textures.go, rotation.go,
// text.go) is pure OpenGL and platform-agnostic.
package gl

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Backend is the OpenGL 3.3 backend for go-gui. Platform-specific
// windowing state lives in plat (platformState), defined per platform.
type Backend struct {
	plat platformState

	pipelines pipelineSet

	textures          texcache.Cache[string, glTexture]
	imagePathCache    texcache.Cache[string, string]
	textSys           *glyph.TextSystem
	filterColorMatrix *[16]float32

	glyphBack    *glyphBackend
	iconFontPath string

	mvpStack [][16]float32

	textPathPlacements []glyph.GlyphPlacement
	svgVerts           []gpu.Vertex
	normBuf            []gui.GradientStop
	sampledBuf         []gui.GradientStop

	allowedImageRoots []string
	svgCap            int
	filterLayer       int
	maxImageBytes     int64
	maxImagePixels    int64
	mvp               [16]float32

	dpiScale float32
	physW    int32
	physH    int32
	quadVAO  uint32
	quadVBO  uint32
	quadIBO  uint32

	// Reusable buffers.
	svgVAO        uint32
	svgVBO        uint32
	filterFBO     uint32
	filterStencil uint32
	filterTexA    uint32
	filterTexB    uint32
	filterW       int32
	filterH       int32
	filterBlur    float32

	customOnce sync.Once
}

// initCaches initializes the platform-neutral caches and image
// limits from the window config.
func (b *Backend) initCaches(cfg gui.WindowCfg) {
	b.textures = newGLTexCacheLRU(128)
	b.imagePathCache = texcache.New[string, string](1024, nil)
	b.maxImageBytes = cfg.MaxImageBytes
	b.maxImagePixels = cfg.MaxImagePixels
	b.allowedImageRoots = imgpath.NormalizeRoots(cfg.AllowedImageRoots)
}

// Minimum OpenGL version the rendering pipeline requires. The shader
// and vertex-array-object calls in initGLResources are core 3.3.
const (
	minGLMajor = 3
	minGLMinor = 3
)

// checkGLVersion verifies the current context exposes at least OpenGL
// 3.3. A GL context must already be current. Some GPU-less environments
// (e.g. Windows' GDI-generic software OpenGL 1.1) hand back a usable
// context whose 2.0+/3.0+ entry points are absent; issuing the
// pipeline's shader and VAO calls against such a context aborts the
// process instead of failing gracefully. Returning an error here lets
// callers (and the headless smoke test) treat it as "no usable GL
// backend" and skip rather than crash.
func checkGLVersion() error {
	verPtr := gl.GetString(gl.VERSION)
	if verPtr == nil {
		return errors.New("gl: GL_VERSION unavailable; no usable OpenGL context")
	}
	ver := gl.GoStr(verPtr)
	var major, minor int
	if _, err := fmt.Sscanf(ver, "%d.%d", &major, &minor); err != nil {
		return fmt.Errorf("gl: cannot parse OpenGL version %q: %w", ver, err)
	}
	if major < minGLMajor || (major == minGLMajor && minor < minGLMinor) {
		return fmt.Errorf("gl: OpenGL %d.%d+ required, have %q",
			minGLMajor, minGLMinor, ver)
	}
	return nil
}

// initGLResources sets up GL state, shader pipelines, buffers, and
// the glyph text system, then wires the platform-neutral injected
// interfaces onto the window. b.physW, b.physH, and b.dpiScale must
// be set before calling. The GL context must already be current.
func (b *Backend) initGLResources(w *gui.Window) error {
	if err := checkGLVersion(); err != nil {
		return err
	}

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.Viewport(0, 0, b.physW, b.physH)

	if err := b.initPipelines(); err != nil {
		return err
	}
	b.initQuadBuffers()
	b.initSvgBuffers()
	b.updateProjection()

	b.glyphBack = newGlyphBackend(b.dpiScale)
	textSys, err := glyph.NewTextSystem(b.glyphBack)
	if err != nil {
		return err
	}
	b.textSys = textSys

	// Load embedded icon font. File must persist because FontConfig
	// registers the path; FreeType reads it lazily.
	if data := gui.IconFontData; len(data) > 0 {
		tmp, ferr := tempfont.Write("go_gui_feathericon", data)
		if ferr != nil {
			log.Printf("gl: write icon font: %v", ferr)
		} else if aerr := textSys.AddFontFile(tmp); aerr != nil {
			log.Printf("gl: load icon font: %v", aerr)
			_ = os.Remove(tmp)
		} else {
			b.iconFontPath = tmp
		}
	}
	for _, p := range gui.AppFontPaths {
		if aerr := textSys.AddFontFile(p); aerr != nil {
			log.Printf("gl: load app font %q: %v", filepath.Base(p), aerr)
		}
	}

	w.SetTextMeasurer(&textMeasurer{textSys: textSys})
	w.SetSvgParser(svg.New())
	w.SetNativePlatform(&nativePlatform{})
	return nil
}

// destroyGLResources releases all GL and glyph resources. Safe to
// call with partially-initialized state.
func (b *Backend) destroyGLResources() {
	b.textures.DestroyAll()
	b.destroyPipelines()
	if b.quadVAO != 0 {
		gl.DeleteVertexArrays(1, &b.quadVAO)
	}
	if b.quadVBO != 0 {
		gl.DeleteBuffers(1, &b.quadVBO)
	}
	if b.quadIBO != 0 {
		gl.DeleteBuffers(1, &b.quadIBO)
	}
	if b.svgVAO != 0 {
		gl.DeleteVertexArrays(1, &b.svgVAO)
	}
	if b.svgVBO != 0 {
		gl.DeleteBuffers(1, &b.svgVBO)
	}
	b.destroyFilterFBO()
	if b.glyphBack != nil {
		b.glyphBack.destroy()
	}
	if b.textSys != nil {
		b.textSys.Free()
	}
	if b.iconFontPath != "" {
		_ = os.Remove(b.iconFontPath)
		b.iconFontPath = ""
	}
}

// renderFrame clears the screen, draws the current layout, and swaps
// buffers. Makes this window's GL context current first.
func (b *Backend) renderFrame(w *gui.Window) {
	b.plat.makeCurrent()
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		t := gui.CurrentTheme()
		bg = t.ColorBackground
	}
	gl.ClearColor(
		float32(bg.R)/255.0,
		float32(bg.G)/255.0,
		float32(bg.B)/255.0,
		float32(bg.A)/255.0,
	)
	gl.Disable(gl.SCISSOR_TEST)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)

	w.Lock()
	w.BackingScale = b.dpiScale
	b.renderersDraw(w)
	w.Unlock()

	b.textSys.Commit()
	b.plat.swap()
}

// handleResize refreshes the drawable size and DPI scale from the
// platform window and updates the viewport and projection.
func (b *Backend) handleResize() {
	b.physW, b.physH = b.plat.drawableSize()
	b.dpiScale = b.plat.dpiScale()
	gl.Viewport(0, 0, b.physW, b.physH)
	b.updateProjection()
}

func (b *Backend) updateProjection() {
	gpu.Ortho(&b.mvp,
		0, float32(b.physW),
		float32(b.physH), 0,
		-1, 1)
}
