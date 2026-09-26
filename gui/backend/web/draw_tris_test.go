//go:build js && wasm

package web

import (
	"fmt"
	"math"
	"slices"
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// fakeCtx2D is a JS object with the path methods drawSvg uses. Each
// call appends to log; fill records the fillStyle in force, so the
// log shows which color each path was filled with.
func fakeCtx2D(t *testing.T, log *[]string) js.Value {
	t.Helper()
	obj := js.Global().Get("Object").New()
	var funcs []js.Func
	add := func(name string, fn func(args []js.Value) string) {
		f := js.FuncOf(func(this js.Value, args []js.Value) any {
			*log = append(*log, fn(args))
			return nil
		})
		funcs = append(funcs, f)
		obj.Set(name, f)
	}
	pt := func(name string) func([]js.Value) string {
		return func(a []js.Value) string {
			return fmt.Sprintf("%s %g %g", name, a[0].Float(), a[1].Float())
		}
	}
	add("beginPath", func([]js.Value) string { return "beginPath" })
	add("closePath", func([]js.Value) string { return "closePath" })
	add("moveTo", pt("moveTo"))
	add("lineTo", pt("lineTo"))
	add("fill", func([]js.Value) string {
		return "fill " + obj.Get("fillStyle").String()
	})
	t.Cleanup(func() {
		for _, f := range funcs {
			f.Release()
		}
	})
	return obj
}

// meshCmd is two quads (four triangles): the first quad red, the
// second blue, with a transform, rotation and offset so every step
// of transformTris is exercised.
func meshCmd() gui.RenderCmd {
	red := gui.RGB(255, 0, 0)
	blue := gui.RGB(0, 0, 255)
	return gui.RenderCmd{
		Triangles: []float32{
			0, 0, 1, 0, 1, 1,
			0, 0, 1, 1, 0, 1,
			2, 0, 3, 0, 3, 1,
			2, 0, 3, 1, 2, 1,
		},
		VertexColors: []gui.Color{
			red, red, red, red, red, red,
			blue, blue, blue, blue, blue, blue,
		},
		X: 10, Y: 20, Scale: 2,
		HasXform: true, ScaleX: 2, ScaleY: 3, TransX: 1, TransY: 1,
		RotAngle: 90, RotCX: 0, RotCY: 0,
	}
}

func drawLog(t *testing.T, helper bool) []string {
	t.Helper()
	var log []string
	b := &Backend{ctx2d: fakeCtx2D(t, &log)}
	if helper {
		b.tris.fill = newTriPathFn()
		if b.tris.fill.Type() != js.TypeFunction {
			t.Fatal("path helper did not compile")
		}
	}
	r := meshCmd()
	b.drawSvg(&r)
	return log
}

// The bulk path must emit exactly the path calls the per-vertex path
// does, in the same order and with the same fill colors, so moving
// the loop into JS changes no pixels.
func TestDrawSvgBulkMatchesPerVertex(t *testing.T) {
	bulk := drawLog(t, true)
	slow := drawLog(t, false)
	if !slices.Equal(bulk, slow) {
		t.Fatalf("bulk and per-vertex paths differ\nbulk: %q\nslow: %q",
			bulk, slow)
	}
	// Two color runs: one path and one fill per run.
	fills := 0
	for _, s := range bulk {
		if len(s) >= 4 && s[:4] == "fill" {
			fills++
		}
	}
	if fills != 2 {
		t.Fatalf("fills = %d, want 2 (one per color run): %q", fills, bulk)
	}
	// 4 triangles × (moveTo + 2 lineTo + closePath) + 2 × (beginPath
	// + fill).
	if len(bulk) != 4*4+2*2 {
		t.Fatalf("call count = %d, want 20: %q", len(bulk), bulk)
	}
}

// transformTris applies the batch transform, then the rotation, then
// the command's scale and offset.
func TestTransformTris(t *testing.T) {
	r := meshCmd()
	got := transformTris(nil, &r)
	if len(got) != len(r.Triangles) {
		t.Fatalf("len = %d, want %d", len(got), len(r.Triangles))
	}
	// Vertex (1, 1): xform → (3, 4); rotate 90° about 0 → (-4, 3);
	// scale 2 + offset (10, 20) → (2, 26).
	const eps = 1e-4
	x, y := got[4], got[5]
	if math.Abs(float64(x-2)) > eps || math.Abs(float64(y-26)) > eps {
		t.Fatalf("vertex (1,1) → (%g, %g), want (2, 26)", x, y)
	}
	// Reuse keeps the buffer: a second pass into got[:0] must not
	// allocate a new array.
	again := transformTris(got[:0], &r)
	if &again[0] != &got[0] {
		t.Fatal("transformTris reallocated a big-enough buffer")
	}
}

// upload grows the JS buffer to a power of two and copies the floats
// byte for byte.
func TestTriMeshUpload(t *testing.T) {
	var m triMesh
	m.floats = []float32{1.5, -2.25, 3e6}
	m.upload()
	if m.cap != 1024 {
		t.Fatalf("cap = %d, want 1024", m.cap)
	}
	for i, want := range m.floats {
		if got := float32(m.f32.Index(i).Float()); got != want {
			t.Fatalf("f32[%d] = %g, want %g", i, got, want)
		}
	}
	m.floats = make([]float32, 1500)
	m.floats[1499] = 7
	m.upload()
	if m.cap != 2048 {
		t.Fatalf("cap after grow = %d, want 2048", m.cap)
	}
	if got := m.f32.Index(1499).Float(); got != 7 {
		t.Fatalf("f32[1499] = %g, want 7", got)
	}
}

// newTriPathFn uses the global a page defined by loading gogui.js,
// and compiles the embedded copy when the page did not load it.
func TestNewTriPathFnSource(t *testing.T) {
	g := js.Global()
	t.Cleanup(func() { g.Delete(triPathName) })

	// Page did not load gogui.js: the embedded copy defines it.
	g.Delete(triPathName)
	fn := newTriPathFn()
	if fn.Type() != js.TypeFunction {
		t.Fatal("embedded helper did not compile")
	}
	if !g.Get(triPathName).Equal(fn) {
		t.Fatal("embedded helper not published as the global")
	}

	// Page loaded gogui.js: that exact function is used.
	pageFn := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	t.Cleanup(pageFn.Release)
	g.Set(triPathName, pageFn)
	if got := newTriPathFn(); !got.Equal(pageFn.Value) {
		t.Fatal("page's goGuiFillTris not preferred")
	}
}
