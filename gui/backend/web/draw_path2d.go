//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// The Path2D cache keeps one Path2D per same-color run of an SVG mesh,
// built once in mesh coordinates, and fills it under a canvas transform
// each frame. It saves the Go-side vertex transform, the copy into JS
// and the path rebuild of drawSvgCanvas; the browser still rasterizes
// the path. It draws the light WebGL batches (draw_webgl.go) and every
// SVG when WebGL2 is unavailable (#1045).

// svgModeGlobal is the global a page may set before Go starts to opt
// out of WebGL2 for SVG ("canvas2d"). The backend overwrites it with
// the mode in effect: "webgl", "path2d" or "canvas2d".
const svgModeGlobal = "goGuiSvgMode"

type svgMode int

const (
	svgModeCanvas2D svgMode = iota
	svgModePath2D
	svgModeWebGL // draw_webgl.go
)

func (m svgMode) String() string {
	switch m {
	case svgModePath2D:
		return "path2d"
	case svgModeWebGL:
		return "webgl"
	default:
		return "canvas2d"
	}
}

// pathCacheIdleFrames is how many frames an unused mesh is kept.
const pathCacheIdleFrames = 120

type pathRun struct {
	vert int // first vertex of the run, indexes VertexColors
	path js.Value
}

type pathEntry struct {
	tris     []float32 // pinned, see meshKey
	runs     []pathRun
	lastUsed uint64
}

type pathCache struct {
	build   js.Value // goGuiBuildTrisPath
	entries map[meshKey]*pathEntry
	vars    meshVariants // mesh_cache.go
	frame   uint64
}

// initSvgMode picks how SVG meshes are drawn: WebGL2 when available
// (draw_webgl.go), else the Path2D cache, else plain Canvas2D paths.
// A page that sets goGuiSvgMode = "canvas2d" before Go starts gets
// plain Canvas2D. The light WebGL batches use the Path2D cache too.
func (b *Backend) initSvgMode() {
	g := js.Global()
	v := g.Get(svgModeGlobal)
	optOut := v.Type() == js.TypeString && v.String() == "canvas2d"
	if !optOut {
		if fn := g.Get("goGuiBuildTrisPath"); fn.Type() == js.TypeFunction {
			b.svgMode = svgModePath2D
			b.paths.build = fn
			b.paths.entries = map[meshKey]*pathEntry{}
		}
		if b.initSvgGL() {
			b.svgMode = svgModeWebGL
			b.gl.minTris = svgGLMinTrisDefault
			// The bench (examples/web_svg_bench) tunes the threshold.
			if n := g.Get("goGuiSvgGLMinTris"); n.Type() == js.TypeNumber {
				b.gl.minTris = n.Int()
			}
		}
	}
	g.Set(svgModeGlobal, b.svgMode.String())
}

// endFramePaths evicts meshes not drawn for pathCacheIdleFrames.
func (b *Backend) endFramePaths() {
	c := &b.paths
	c.frame++
	if c.frame%60 != 0 {
		return
	}
	for k, e := range c.entries {
		if c.frame-e.lastUsed > pathCacheIdleFrames {
			c.drop(k)
		}
	}
}

func (c *pathCache) drop(k meshKey) {
	delete(c.entries, k)
	c.vars.remove(k)
}

// pathEntryFor returns the cached runs of r's mesh, building them when
// the mesh is new or its content changed.
func (b *Backend) pathEntryFor(r *gui.RenderCmd) *pathEntry {
	c := &b.paths
	key := meshKeyFor(r)
	if e, ok := c.entries[key]; ok {
		e.lastUsed = c.frame
		return e
	}
	e := &pathEntry{tris: r.Triangles, lastUsed: c.frame}
	b.tris.floats = append(b.tris.floats[:0], r.Triangles...)
	b.tris.upload()
	numVerts := len(r.Triangles) / 2
	addRun := func(vert, endVert int) {
		e.runs = append(e.runs, pathRun{
			vert: vert,
			path: c.build.Invoke(b.tris.f32, vert*2, endVert*2),
		})
	}
	if len(r.VertexColors) != numVerts {
		addRun(0, numVerts)
	} else {
		// Same split as drawSvg, on the raw colors: alpha scaling is
		// monotonic, so equal raw colors stay equal once scaled.
		start := 0
		for i := 3; i < numVerts; i += 3 {
			if r.VertexColors[i] != r.VertexColors[start] {
				addRun(start, i)
				start = i
			}
		}
		addRun(start, numVerts)
	}
	c.entries[key] = e
	if c.vars == nil {
		c.vars = meshVariants{}
	}
	c.vars.add(key)
	trimMeshVariants(c.entries, c.vars, key.base(), c.frame,
		func(e *pathEntry) uint64 { return e.lastUsed },
		func(k meshKey, _ *pathEntry) { c.drop(k) })
	return e
}

// drawSvgPath2D fills r from the path cache. The caller has validated
// r as drawSvg does.
func (b *Backend) drawSvgPath2D(r *gui.RenderCmd) {
	e := b.pathEntryFor(r)
	a, bb, cc, d, ee, f := svgAffine(r)
	b.ctx2d.Call("save")
	b.ctx2d.Call("transform", a, bb, cc, d, ee, f)
	hasVCols := len(r.VertexColors) == len(r.Triangles)/2
	vAlpha := float32(1)
	if r.HasVertexAlpha {
		vAlpha = max(0, min(r.VertexAlphaScale, 1))
	}
	for _, run := range e.runs {
		col := r.Color
		if hasVCols {
			vc := r.VertexColors[run.vert]
			alpha := vc.A
			if r.HasVertexAlpha {
				alpha = uint8(float32(alpha) * vAlpha)
			}
			col = gui.RGBA(vc.R, vc.G, vc.B, alpha)
		}
		b.setFillColor(col)
		b.ctx2d.Call("fill", run.path)
	}
	b.ctx2d.Call("restore")
}

// svgAffine composes transformTris's three steps — batch transform,
// rotation about (RotCX, RotCY), then Scale and offset — into one
// canvas transform(a, b, c, d, e, f): x' = a·x + c·y + e,
// y' = b·x + d·y + f.
func svgAffine(r *gui.RenderCmd) (a, b, c, d, e, f float64) {
	// M1: batch transform, identity unless HasXform.
	a, b, c, d = 1, 0, 0, 1
	if r.HasXform {
		a, c, e = float64(r.ScaleX), float64(r.XformXY), float64(r.TransX)
		b, d, f = float64(r.XformYX), float64(r.ScaleY), float64(r.TransY)
	}
	// M2 · M1: rotate about the pivot.
	if r.RotAngle != 0 {
		rad := float64(r.RotAngle) * math.Pi / 180
		sin, cos := math.Sin(rad), math.Cos(rad)
		px, py := float64(r.RotCX), float64(r.RotCY)
		a, b, c, d, e, f =
			cos*a-sin*b, sin*a+cos*b,
			cos*c-sin*d, sin*c+cos*d,
			px+cos*(e-px)-sin*(f-py), py+sin*(e-px)+cos*(f-py)
	}
	// M3 · M2 · M1: uniform scale, then the command offset.
	s := float64(r.Scale)
	return a * s, b * s, c * s, d * s, float64(r.X) + e*s, float64(r.Y) + f*s
}
