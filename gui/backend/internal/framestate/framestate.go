// Package framestate holds backend-neutral render-frame state
// shared by the mobile GPU backends (Android GLES, iOS Metal).
// It deliberately contains no C or platform code: the backends
// inject their C bridges through the Bridge interface and keep
// all platform-specific concerns (textures, shader sources,
// event loops) in their own files.
package framestate

import (
	"log"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// Bridge abstracts the platform C calls that mutate GPU state.
// Implementations convert the plain int pipe IDs and the MVP
// matrix into their C-facing calls (e.g. glesSetPipeline /
// metalSetPipeline). Both calls are always issued together,
// matching the pipeline-switch semantics of the backends.
type Bridge interface {
	// SetPipeline switches the active pipeline and uploads the
	// current MVP matrix.
	SetPipeline(pipe int, mvp *[16]float32)

	// Resize notifies the GPU of new physical pixel dimensions.
	Resize(physW, physH int32)
}

// FrameState is the platform-neutral render-frame state for a
// single window: DPI/physical dimensions, the MVP matrix and its
// rotation stack, reusable vertex/placement buffers, pipeline
// dirty tracking, filter (glow) parameters, and the image path
// cache. Backends embed a FrameState and promote its methods;
// exported fields share names with the former Backend fields so
// platform draw code reads the same way as before.
type FrameState struct {
	DPIScale float32
	PhysW    int32
	PhysH    int32
	MVP      [16]float32

	// MVPStack holds pushed MVP matrices for RenderRotateBegin /
	// RenderRotateEnd nesting.
	MVPStack [][16]float32

	// Reusable scratch buffers kept across frames to avoid
	// per-frame heap allocation.
	SVGVerts           []gpu.Vertex
	TextPathPlacements []glyph.GlyphPlacement
	NormBuf            []gui.GradientStop
	SampledBuf         []gui.GradientStop

	// Filter (glow) parameters for RenderFilterBegin / End.
	FilterBlur        float32
	FilterLayer       int
	FilterColorMatrix *[16]float32

	// Pipeline state tracking to skip redundant C bridge calls.
	LastPipe   int
	MVPDirty   bool
	TextQueued bool

	// AllowedImageRoots and ImagePathCache back drawImage path
	// resolution; see ResolveImagePath.
	AllowedImageRoots []string
	ImagePathCache    texcache.Cache[string, string]

	// SolidPipe and GlyphTexPipe are the platform pipeline IDs
	// (plain ints; the Bridge converts them) used after rotation,
	// stencil, and text-flush state changes.
	SolidPipe    int
	GlyphTexPipe int

	bridge Bridge
}

// New returns a FrameState ready for the first frame. LastPipe is
// seeded with -1 so the first SetPipeline always issues the bridge
// calls. The image path cache holds up to 1024 entries.
func New(solidPipe, glyphTexPipe int, bridge Bridge) *FrameState {
	fs := &FrameState{
		SolidPipe:    solidPipe,
		GlyphTexPipe: glyphTexPipe,
		LastPipe:     -1,
		bridge:       bridge,
	}
	fs.ImagePathCache = texcache.New[string, string](1024, nil)
	return fs
}

// UpdateProjection rebuilds the orthographic MVP from the current
// physical dimensions. The Y axis points down (top-left origin).
func (fs *FrameState) UpdateProjection() {
	gpu.Ortho(&fs.MVP,
		0, float32(fs.PhysW),
		float32(fs.PhysH), 0,
		-1, 1)
}

// SetPipeline switches the pipeline and uploads the MVP, skipping
// redundant bridge calls when nothing changed. An invalidation
// (InvalidatePipelineState) or a pending rotation (MVPDirty) forces
// the bridge calls on the next call.
func (fs *FrameState) SetPipeline(pipe int) {
	if pipe == fs.LastPipe && !fs.MVPDirty {
		return
	}
	fs.bridge.SetPipeline(pipe, &fs.MVP)
	fs.LastPipe = pipe
	fs.MVPDirty = false
}

// InvalidatePipelineState forces the next SetPipeline to issue
// bridge calls even for the same pipeline (used after direct C
// state mutation such as stencil or filter begin/end).
func (fs *FrameState) InvalidatePipelineState() {
	fs.LastPipe = -1
}

// UseGlyphPipeline switches to the glyph-text pipeline.
func (fs *FrameState) UseGlyphPipeline() {
	fs.SetPipeline(fs.GlyphTexPipe)
}

// HandleResize updates DPI and physical dimensions, notifies the
// GPU bridge, and rebuilds the projection. The logical size and
// scale arrive from the platform's surface-change callback.
func (fs *FrameState) HandleResize(w, h int32, scale float32) {
	fs.DPIScale = scale
	fs.PhysW = int32(float32(w) * scale)
	fs.PhysH = int32(float32(h) * scale)
	fs.bridge.Resize(fs.PhysW, fs.PhysH)
	fs.UpdateProjection()
}

// FrameBg resolves the frame clear color: the window's configured
// background, falling back to the window's theme background when
// unset. Returns normalized 0..1 components ready for the C
// begin-frame call.
func (fs *FrameState) FrameBg(w *gui.Window) (r, g, b, a float32) {
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		// Runs after FrameFn on the same thread, so the installed
		// theme happens to be right; w.Theme() makes it right by
		// construction instead of by timing.
		bg = w.Theme().ColorBackground
	}
	return float32(bg.R) / 255.0,
		float32(bg.G) / 255.0,
		float32(bg.B) / 255.0,
		float32(bg.A) / 255.0
}

// BeginRotation pushes the current MVP, applies a rotation about
// (cx, cy) in physical pixels, and re-issues the solid pipeline so
// subsequent draws use the rotated matrix.
func (fs *FrameState) BeginRotation(angleDeg, cx, cy float32) {
	fs.MVPStack = append(fs.MVPStack, fs.MVP)
	gpu.ApplyRotation(&fs.MVP, angleDeg, cx, cy)
	fs.MVPDirty = true
	fs.SetPipeline(fs.SolidPipe)
}

// EndRotation pops the MVP stack, restoring the matrix in effect
// before the matching BeginRotation. A no-op on an empty stack.
func (fs *FrameState) EndRotation() {
	n := len(fs.MVPStack)
	if n == 0 {
		return
	}
	fs.MVP = fs.MVPStack[n-1]
	fs.MVPStack = fs.MVPStack[:n-1]
	fs.MVPDirty = true
	fs.SetPipeline(fs.SolidPipe)
}

// FlushText commits queued glyph text to the GPU when any text was
// drawn this frame, and clears the queued flag.
func (fs *FrameState) FlushText(textSys *glyph.TextSystem) {
	if fs.TextQueued {
		fs.UseGlyphPipeline()
		textSys.Commit()
		fs.TextQueued = false
	}
}

// ResolveImagePath returns the validated filesystem path for a
// render resource, caching the result (including the "-" failure
// sentinel) so validation runs once per resource. Failures are
// logged with the backend's log prefix.
func (fs *FrameState) ResolveImagePath(resource, logPrefix string) string {
	if path, ok := fs.ImagePathCache.Get(resource); ok {
		return path
	}
	path, err := imgload.ResolveValidatedPath(
		resource, fs.AllowedImageRoots)
	if err != nil {
		log.Printf("%s: drawImage: %v", logPrefix, err)
		path = "-"
	}
	fs.ImagePathCache.Set(resource, path)
	return path
}
