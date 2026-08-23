package gui

// DrawCanvasCache holds retained tessellation output keyed by
// widget id + version + scale. Cache hit skips OnDraw entirely.
type drawCanvasCache struct {
	Batches    []DrawCanvasTriBatch
	Texts      []DrawCanvasTextEntry
	Images     []DrawCanvasImageEntry
	Version    uint64
	tessWidth  float32
	tessHeight float32
	Scale      float32
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
