package gui

// DrawCanvasCache holds retained tessellation output keyed by
// widget id + version + scale. Cache hit skips OnDraw entirely.
type drawCanvasCache struct {
	Batches []DrawCanvasTriBatch
	// spare is the batch list the redraw before last wrote into. The
	// two arrays ping-pong: a redraw writes into spare while claiming
	// buffers out of Batches, then the two swap roles. That keeps the
	// header array itself off the per-frame allocation path, and the
	// stale headers left in spare past its new length are never read —
	// takeBatch only ever indexes below len(pool).
	spare  []DrawCanvasTriBatch
	Texts  []DrawCanvasTextEntry
	Images []DrawCanvasImageEntry
	// Gradients holds the radial fills that were lowered to a shader
	// quad instead of a ring mesh, and gradSpare is their pool. The
	// two ping-pong exactly as Batches and spare do.
	Gradients []DrawCanvasGradientEntry
	gradSpare []DrawCanvasGradientEntry
	Version   uint64
	// pass is the Window.renderPass that last wrote this entry. A
	// redraw recycles the entry's buffers only when it belongs to an
	// earlier pass, so a canvas rendered twice in one list — which
	// takes duplicate effective IDs — falls back to allocating rather
	// than writing over triangles an already-emitted command points at.
	pass       uint64
	tessWidth  float32
	tessHeight float32
	Scale      float32
}

// DrawCanvasGradientEntry is a radial gradient fill the canvas hands
// to the backend as one RenderGradient quad rather than tessellating
// it. X/Y/W/H are the quad in the same content-relative coordinates
// the triangle batches use; the fill is a circle inscribed in it, so
// the command's corner radius is W/2 and the shader's radial ramp,
// which is always centered on the quad with radius max(W,H)/2, lands
// exactly on the circle.
//
// Def carries only Stops and Type: GradientDef has no center, radius,
// focal or spread of its own, which is why only the concentric case
// can take this path at all.
//
// afterBatch is how many triangle batches had been recorded when this
// fill arrived. It stays unexported because it is bookkeeping for the
// emission walk, not something a consumer of Gradients() can act on:
// renderDrawCanvas walks batches and gradients together, emitting a
// gradient before the batch its counter names.
//
// Lifetime matches DrawCanvasTriBatch: Def.Stops belongs to the
// canvas's cache entry and is recycled by the next redraw.
// exportaudit:keep — reachable from an exported signature
type DrawCanvasGradientEntry struct {
	Def        GradientDef
	X, Y, W, H float32
	afterBatch int
}

// DrawCanvasTextEntry stores a deferred text drawing command.
type DrawCanvasTextEntry struct {
	Style TextStyle
	Text  string
	X, Y  float32
}

// DrawCanvasTriBatch is one triangle batch.
//
// A flat batch leaves VertexColors nil and paints every triangle in
// Color. A gradient batch carries one color per vertex — exactly
// len(Triangles)/2 of them — which every backend already consumes
// through RenderCmd.VertexColors; its Color is then the gradient
// sampled at its midpoint, kept only so a flat-only consumer degrades
// to something reasonable rather than to nothing.
//
// The two never mix inside one batch: a gradient fill always starts a
// fresh batch, so the length relation above holds per batch and
// validSvgCmd can enforce it.
//
// Lifetime: both slices belong to the canvas's cache entry and are
// recycled by its next redraw (DrawContext.resetFor). A consumer that
// keeps a RenderCmd past the frame it was emitted in — the print export
// is the one in-tree case — must copy the geometry out first.
type DrawCanvasTriBatch struct {
	Triangles    []float32
	VertexColors []Color
	Color        Color
}

// DrawCanvasImageEntry stores a deferred image drawing command.
// Src matches the forms accepted by ImageCfg.Src: local path,
// http/https URL, or data URL.
//
// BgOpacity modulates BgColor's alpha; it has no effect when
// BgColor is unset.
//
// Fetcher, when non-nil, overrides WindowCfg.ImageFetcher for this
// entry's http/https download. Typical use is map-tile rendering
// where each tile source wants its own User-Agent. Known limit:
// downloads are URL-keyed and deduped process-wide, so the first
// entry observed for a given URL binds the fetcher for that URL's
// in-flight download. Consumers wiring two fetchers to overlapping
// URL namespaces must route via URL prefix themselves.
// ClipX/ClipY/ClipW/ClipH restrict drawing to a sub-rectangle of the
// canvas, in the same content-relative coordinates as X/Y/W/H. They
// are honored only when Clipped is set (a zero rect would otherwise
// be indistinguishable from "no clip"). Use it to show part of an
// image without cropping the file: the texture still maps to the
// full X/Y/W/H rect, the scissor decides what is visible.
// exportaudit:keep — reachable from an exported signature
type DrawCanvasImageEntry struct {
	fetcher                    ImageFetcher
	Src                        string
	bgOpacity                  Opt[float32]
	X, Y, W, H                 float32
	ClipX, ClipY, ClipW, ClipH float32
	BgColor                    Color
	Clipped                    bool
}

// DrawRecorder receives high-level draw commands before
// tessellation. Attach via DrawContext.SetRecorder to capture
// structured primitives (e.g. for SVG export).
// exportaudit:keep — reachable from an exported signature
type DrawRecorder interface {
	Line(x0, y0, x1, y1 float32, color Color, width float32)
	Polyline(points []float32, color Color, width float32)
	FilledRect(x, y, w, h float32, color Color)
	Rect(x, y, w, h float32, color Color, width float32)
	FilledCircle(cx, cy, radius float32, color Color)
	Circle(cx, cy, radius float32, color Color, width float32)
	FilledArc(cx, cy, rx, ry, start, sweep float32, color Color)
	Arc(cx, cy, rx, ry, start, sweep float32, color Color, width float32)
	FilledPolygon(points []float32, color Color)
	FilledRoundedRect(x, y, w, h, radius float32, color Color)
	RoundedRect(x, y, w, h, radius float32, color Color, width float32)
	DashedLine(x0, y0, x1, y1 float32, color Color, width, dashLen, gapLen float32)
	DashedPolyline(points []float32, color Color, width, dashLen, gapLen float32)
	PolylineJoined(points []float32, color Color, width float32)
	QuadBezier(x0, y0, cx, cy, x1, y1 float32, color Color, width float32)
	CubicBezier(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32, color Color, width float32)
	Text(x, y float32, text string, style TextStyle)
}

// DrawGradientRecorder is an optional extension to DrawRecorder: a
// recorder implementing it receives gradient fills as tessellated
// geometry plus the gradient that shades it.
//
// It is a separate interface rather than more methods on DrawRecorder
// because DrawRecorder is exported and implemented outside this repo;
// widening it would break every existing implementer. A recorder that
// does not implement this still receives the fill, as the equivalent
// flat primitive shaded with the gradient's midpoint color, so an
// export path never silently drops a gradient fill.
// exportaudit:keep — reachable from an exported signature
type DrawGradientRecorder interface {
	FillTrianglesGradient(tris []float32, g *CanvasGradient)
}

// DrawVertexColorRecorder is the per-vertex-color sibling of
// DrawGradientRecorder: a recorder implementing it receives a
// FillTrianglesColors fill as the caller's geometry plus the caller's
// one-color-per-vertex slice.
//
// It is separate from DrawGradientRecorder rather than a second method
// on it for the same reason DrawGradientRecorder is separate from
// DrawRecorder — both are exported and implemented outside this repo —
// and because a per-vertex fill has no CanvasGradient to hand over.
// A recorder that does not implement this still receives the fill, one
// flat polygon per triangle shaded with that triangle's mean color, so
// an export path never silently drops a shaded mesh.
// exportaudit:keep — reachable from an exported signature
type DrawVertexColorRecorder interface {
	FillTrianglesColors(tris []float32, colors []Color)
}
