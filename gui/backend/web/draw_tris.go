//go:build js && wasm

package web

import (
	_ "embed"
	"log"
	"math"
	"syscall/js"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Triangle meshes (RenderSvg: SVG, DrawCanvas, charts, the
// ThinkingOrb dots) are the heaviest thing the Canvas2D backend
// draws. Each syscall/js call costs far more than the Canvas2D work
// it asks for, and building a path one vertex at a time takes
// moveTo + 2×lineTo + closePath per triangle: a Regular ThinkingOrb
// alone is ~33k triangles, ~170k calls per frame. That blew the
// frame budget and made the orbs stutter.
//
// So a mesh crosses into JS once. Go transforms the vertices into
// triMesh.floats, copies them in one CopyBytesToJS into a
// Float32Array that JS keeps, and a small JS function (gogui.js)
// walks the range and builds the path. A same-color run then costs
// two calls from Go (set the fill color, call the helper) at any
// triangle count.

// goguiJS is the helper script pages load as gogui.js. It is
// embedded so a page that does not load it still gets the same
// helper, compiled at startup.
//
//go:embed gogui.js
var goguiJS string

// triPathName is the global gogui.js defines.
const triPathName = "goGuiFillTris"

// triMesh holds the Go and JS sides of the shared vertex buffer.
type triMesh struct {
	// fill is the path helper (see newTriPathFn). Undefined when the
	// page neither loaded gogui.js nor allows new Function; the
	// backend then falls back to one call per vertex.
	fill js.Value
	// floats holds the transformed vertices of the mesh being drawn,
	// in canvas coordinates. Kept across frames so a redraw does not
	// allocate.
	floats []float32
	// f32 and u8 are two views over one JS ArrayBuffer: the helper
	// reads f32, CopyBytesToJS writes u8. cap is their length in
	// floats.
	f32 js.Value
	u8  js.Value
	cap int
}

// newTriPathFn finds the path helper. It prefers the global a page
// defined by loading gogui.js, which works under any CSP that allows
// the page's own scripts. Otherwise it runs the embedded copy with
// new Function, which a CSP without 'unsafe-eval' blocks. It returns
// an undefined value when neither works, and the backend draws one
// call per vertex.
func newTriPathFn() (fn js.Value) {
	if fn = js.Global().Get(triPathName); fn.Type() == js.TypeFunction {
		return fn
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("web: %s unavailable (load gogui.js on a "+
				"strict-CSP page); using slow per-vertex path: %v",
				triPathName, r)
			fn = js.Undefined()
		}
	}()
	js.Global().Get("Function").New(goguiJS).Invoke()
	return js.Global().Get(triPathName)
}

// upload copies m.floats into the JS buffer, growing it to the next
// power of two when it is too small so growth is rare.
func (m *triMesh) upload() {
	n := len(m.floats)
	if n == 0 {
		return
	}
	if n > m.cap {
		c := max(m.cap, 1024)
		for c < n {
			c *= 2
		}
		buf := js.Global().Get("ArrayBuffer").New(c * 4)
		m.f32 = js.Global().Get("Float32Array").New(buf)
		m.u8 = js.Global().Get("Uint8Array").New(buf)
		m.cap = c
	}
	// WASM and JS typed arrays are both little-endian, so the raw
	// bytes of a []float32 are a valid Float32Array.
	b := unsafe.Slice((*byte)(unsafe.Pointer(&m.floats[0])), n*4)
	js.CopyBytesToJS(m.u8, b)
}

// transformTris appends the vertices of r to dst in canvas
// coordinates: the batch transform, then the rotation, then the
// command's scale and offset — the same order the per-vertex code
// used.
func transformTris(dst []float32, r *gui.RenderCmd) []float32 {
	hasXform := r.HasXform
	var sx, sy, tx, ty, sxy, syx float32
	if hasXform {
		sx, sy, tx, ty = r.ScaleX, r.ScaleY, r.TransX, r.TransY
		sxy, syx = r.XformXY, r.XformYX
	}
	hasRot := r.RotAngle != 0
	var sinA, cosA, rcx, rcy float32
	if hasRot {
		rad := float64(r.RotAngle) * math.Pi / 180
		sinA = float32(math.Sin(rad))
		cosA = float32(math.Cos(rad))
		rcx, rcy = r.RotCX, r.RotCY
	}
	for i := 0; i+1 < len(r.Triangles); i += 2 {
		vx := r.Triangles[i]
		vy := r.Triangles[i+1]
		if hasXform {
			ox, oy := vx, vy
			vx = ox*sx + oy*sxy + tx
			vy = ox*syx + oy*sy + ty
		}
		if hasRot {
			dx := vx - rcx
			dy := vy - rcy
			vx = rcx + dx*cosA - dy*sinA
			vy = rcy + dx*sinA + dy*cosA
		}
		dst = append(dst, r.X+vx*r.Scale, r.Y+vy*r.Scale)
	}
	return dst
}

// fillTris fills the triangles in floats [s, e) of the uploaded
// mesh as one path in the current fill style.
func (b *Backend) fillTris(s, e int) {
	if b.tris.fill.Type() == js.TypeFunction {
		b.tris.fill.Invoke(b.ctx2d, b.tris.f32, s, e)
		return
	}
	// Slow path: no helper, one call per vertex.
	xy := b.tris.floats
	b.ctx2d.Call("beginPath")
	for i := s; i+6 <= e; i += 6 {
		b.ctx2d.Call("moveTo", float64(xy[i]), float64(xy[i+1]))
		b.ctx2d.Call("lineTo", float64(xy[i+2]), float64(xy[i+3]))
		b.ctx2d.Call("lineTo", float64(xy[i+4]), float64(xy[i+5]))
		b.ctx2d.Call("closePath")
	}
	b.ctx2d.Call("fill")
}
