//go:build darwin && arm64

// Package mtldraw is the Metal renderer that the macOS and iOS backends
// share (#1342). It turns a window's []gui.RenderCmd into Metal draws
// on a CAMetalLayer that the backend owns. It calls Metal through
// purego (gui/internal/darwinffi), so the same code builds for both.
//
// The package knows nothing about windows or views. A backend makes the
// layer (an NSView's on macOS, a UIView's on iOS), hands it to New, and
// calls RenderFrame, Resize and SetDPIScale from its own event code.
package mtldraw

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/ebitengine/purego/objc"
	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/backend/internal/memtex"
	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
	"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

const maxCustomPipelines = 32

// Renderer holds the per-window Metal state: the device context, the
// text system, the texture caches and the per-draw scratch. One window
// (or one iOS view) has one Renderer. It is not safe for concurrent
// use: every call comes from the backend's main thread.
type Renderer struct {
	ctx      *metalCtx
	textSys  *glyph.TextSystem
	dpiScale float32
	physW    int32
	physH    int32
	mvp      [16]float32

	// Per-draw scratch handed to the renderer by pointer: see drawQuad.
	quad [4]vertex
	tm   [16]float32
	tm2  [16]float32

	mvpStack [][16]float32

	svgVerts           []gpu.Vertex
	textPathPlacements []glyph.GlyphPlacement

	// textErrLogged warns once for a persistent DrawText failure
	// instead of spamming stderr every frame.
	textErrLogged bool
	normBuf       []gui.GradientStop
	sampledBuf    []gui.GradientStop

	textures          texcache.Cache[string, metalTexture]
	memSweep          memtex.Sweeper // when to check mem: textures (#1230)
	surfaces          metalSurfaces
	glyphBack         *metalGlyphBackend
	filterBlur        float32
	filterLayer       int
	filterColorMatrix *[16]float32
	customCache       texcache.Cache[uint64, int32]
	shaderHashes      gui.ShaderHashCache // memoizes ShaderHash per draw
	iconFontPath      string

	allowedImageRoots []string
	imagePathCache    texcache.Cache[string, string]
	maxImageBytes     int64
	maxImagePixels    int64
}

// New makes a Renderer that draws into layer, a CAMetalLayer the
// backend owns. physW and physH are the drawable size in pixels, and
// scale is pixels per point. cfg supplies the texture and image limits
// and the image roots. logTag prefixes log lines ("metal", "ios") and
// names the backend to gui.LoadAppFonts.
//
// The layer stays the backend's: Destroy does not release it.
func New(layer objc.ID, physW, physH int, scale float32,
	cfg gui.WindowCfg, logTag string) (*Renderer, error) {
	if layer == 0 {
		return nil, errors.New("mtldraw: no CAMetalLayer")
	}
	// Shader source is owned by Go so the build cache tracks edits to
	// it. See gui/backend/internal/msl.
	ctx := newMetalCtx(layer, msl.Source)
	if ctx == nil {
		return nil, errors.New("mtldraw: renderer setup failed")
	}
	ctx.resize(physW, physH)

	r := &Renderer{
		ctx:      ctx,
		dpiScale: scale,
		physW:    int32(physW),
		physH:    int32(physH),
		textures: newMetalTexCacheLRU(ctx, 128, cfg.MaxTextureBytes),
		customCache: texcache.New[uint64, int32](
			maxCustomPipelines,
			func(idx int32) {
				// A cached compile failure holds a negative
				// idx with no pipeline behind it.
				if idx >= 0 {
					ctx.deleteCustomPipeline(idx)
				}
			},
		),
		imagePathCache: texcache.New[string, string](1024, nil),
		maxImageBytes:  cfg.MaxImageBytes,
		maxImagePixels: cfg.MaxImagePixels,
	}
	r.surfaces.setup(r)
	r.allowedImageRoots = imgpath.NormalizeRoots(cfg.AllowedImageRoots)
	r.updateProjection()

	r.glyphBack = newMetalGlyphBackend(ctx, scale)
	textSys, err := glyph.NewTextSystem(r.glyphBack)
	if err != nil {
		r.Destroy()
		return nil, fmt.Errorf("NewTextSystem: %w", err)
	}
	r.textSys = textSys

	// Load embedded icon font.
	if data := gui.IconFontData; len(data) > 0 {
		tmp, werr := tempfont.Write("go_gui_feathericon", data)
		if werr != nil {
			log.Printf("%s: write icon font: %v", logTag, werr)
		} else if aerr := textSys.AddFontFile(tmp); aerr != nil {
			log.Printf("%s: load icon font: %v", logTag, aerr)
			_ = os.Remove(tmp)
		} else {
			r.iconFontPath = tmp
		}
	}

	gui.LoadAppFonts(textSys, logTag)
	return r, nil
}

// Ready reports whether r has a live Metal context. A nil Renderer, or
// one after Destroy, is not ready.
func (r *Renderer) Ready() bool { return r != nil && r.ctx != nil }

// DPIScale returns the pixels-per-point scale r draws at.
func (r *Renderer) DPIScale() float32 { return r.dpiScale }

// TextMeasurer returns the gui.TextMeasurer backed by r's text system,
// for w.SetTextMeasurer.
func (r *Renderer) TextMeasurer() gui.TextMeasurer {
	return &textMeasurer{textSys: r.textSys}
}

// Destroy frees every GPU and text resource r holds. It is safe to call
// on a Renderer that New only partly built.
func (r *Renderer) Destroy() {
	r.textures.DestroyAll()
	r.surfaces.clear()
	r.customCache.DestroyAll()
	if r.glyphBack != nil {
		r.glyphBack.destroy()
		r.glyphBack = nil
	}
	if r.textSys != nil {
		r.textSys.Free()
		r.textSys = nil
	}
	if r.iconFontPath != "" {
		_ = os.Remove(r.iconFontPath)
		r.iconFontPath = ""
	}
	if r.ctx != nil {
		r.ctx.destroy()
		r.ctx = nil
	}
}

// RenderFrame draws w's current render commands and presents the
// drawable. It takes w's lock for the draw. A frame with no drawable
// (the layer is hidden, or the GPU is busy) is skipped.
func (r *Renderer) RenderFrame(w *gui.Window) {
	bg := w.FrameBackground()
	if !r.ctx.beginFrame(float32(bg.R)/255, float32(bg.G)/255,
		float32(bg.B)/255, float32(bg.A)/255) {
		return
	}
	r.ctx.setPipeline(pipeSolid)
	r.ctx.setMVP(&r.mvp)

	w.Lock()
	w.BackingScale = r.dpiScale
	r.renderersDraw(w)
	w.Unlock()

	r.useGlyphPipeline()
	r.textSys.Commit()
	r.ctx.endFrame()
}

// Resize adopts a new drawable size in pixels. A zero or negative size
// is ignored: it can come during teardown, or before an occluded
// window's first draw, and Metal makes NaN projections from it.
func (r *Renderer) Resize(physW, physH int) {
	if physW <= 0 || physH <= 0 {
		return
	}
	r.physW = int32(physW)
	r.physH = int32(physH)
	r.ctx.resize(physW, physH)
	r.updateProjection()
}

// SetDPIScale adopts a new backing scale for the whole window. Text
// needs both halves: glyphBack.dpiScale places the quads, and the text
// system shapes and rasterizes at the new density. Without the second
// half a window dragged between a Retina and a non-Retina display keeps
// rendering glyphs at the density it was created at, so text drifts out
// of the boxes laid out for it.
func (r *Renderer) SetDPIScale(s float32) {
	if r == nil {
		return
	}
	if s != s || math.IsInf(float64(s), 0) || s <= 0 || s > 8 {
		// An implausible value means the backing-scale query failed.
		// Keeping the old scale beats adopting a broken one, but the
		// symptom is mis-sized text with no trace of the cause, so
		// leave evidence when the dev gate is on.
		if gui.DebugEnabled() {
			log.Printf("mtldraw: rejected implausible dpi scale %v", s)
		}
		return
	}
	if s == r.dpiScale {
		return
	}
	r.dpiScale = s
	if r.glyphBack != nil {
		r.glyphBack.dpiScale = s
	}
	if r.textSys != nil {
		r.textSys.SetDPIScale(s)
	}
}

func (r *Renderer) updateProjection() {
	gpu.Ortho(&r.mvp,
		0, float32(r.physW),
		float32(r.physH), 0,
		-1, 1)
}

func (r *Renderer) useGlyphPipeline() {
	r.ctx.setPipeline(pipeGlyphTex)
	r.ctx.setMVP(&r.mvp)
}
