//go:build js && wasm

package web

import (
	"runtime/metrics"
	"syscall/js"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// benchHookName is the global a benchmark page defines, before it
// starts Go, to receive per-frame timings (examples/web_svg_bench). A
// page that defines none pays one lookup at startup.
const benchHookName = "goGuiOnFrame"

// benchHook returns the page's frame-timing callback, or an undefined
// value when the page defines none. Read once at startup: the check
// costs a JS call, and the hook is a page-load decision.
func benchHook() js.Value {
	if fn := js.Global().Get(benchHookName); fn.Type() == js.TypeFunction {
		return fn
	}
	return js.Undefined()
}

// gcSample is a cumulative runtime reading; differences between two
// samples attribute GC work and allocation to the phase between them.
// runtime/metrics is read without stopping the world, unlike
// ReadMemStats. Go's wasm runtime has one thread, so a GC cycle runs
// inside whichever phase triggered it.
type gcSample struct {
	cycles     uint64
	allocBytes uint64
	gcCPUSec   float64
}

var gcMetricNames = []string{
	"/gc/cycles/total:gc-cycles",
	"/gc/heap/allocs:bytes",
	"/cpu/classes/gc/total:cpu-seconds",
}

type gcSampler struct{ s []metrics.Sample }

func newGCSampler() *gcSampler {
	g := &gcSampler{s: make([]metrics.Sample, len(gcMetricNames))}
	for i, n := range gcMetricNames {
		g.s[i].Name = n
	}
	return g
}

func (g *gcSampler) read() gcSample {
	metrics.Read(g.s)
	return gcSample{
		cycles:     g.s[0].Value.Uint64(),
		allocBytes: g.s[1].Value.Uint64(),
		gcCPUSec:   g.s[2].Value.Float64(),
	}
}

// timedFrame runs one animation frame like the plain loop does, and
// reports to hook, in this order:
//
//	layoutMs, drawMs  time in FrameFn (layout) and renderFrame (draw)
//	painted           whether the frame painted
//	cmds, svgCmds, svgTris
//	composites        WebGL batches composited
//	layoutAllocB, drawAllocB   bytes allocated in each phase
//	layoutGC, drawGC           GC cycles completed in each phase
//	gcCPUMs           estimated GC CPU time over the frame
//	filled            WebGL batches drawn with Canvas2D instead
//	direct            WebGL batches composited from the WebGL canvas,
//	                  not the staging copy (draw_webgl.go)
//
// Canvas2D rasterizes after this returns, so the draw time is only the
// Go and JS call cost; the page measures what the browser adds from
// requestAnimationFrame deltas.
func (b *Backend) timedFrame(w *gui.Window, hook js.Value) bool {
	if b.gcs == nil {
		b.gcs = newGCSampler()
	}
	b.gl.composites = 0
	b.gl.filled = 0
	b.gl.direct = 0
	g0 := b.gcs.read()
	t0 := time.Now()
	changed := w.FrameFn()
	t1 := time.Now()
	g1 := b.gcs.read()
	painted := b.paintIfNeeded(changed, func() { b.renderFrame(w) })
	t2 := time.Now()
	g2 := b.gcs.read()
	var cmds, svgCmds, svgTris int
	if painted {
		w.Lock()
		rs := w.Renderers()
		cmds = len(rs)
		for i := range rs {
			if rs[i].Kind == gui.RenderSvg {
				svgCmds++
				svgTris += len(rs[i].Triangles) / 6
			}
		}
		w.Unlock()
	}
	hook.Invoke(
		float64(t1.Sub(t0).Microseconds())/1000,
		float64(t2.Sub(t1).Microseconds())/1000,
		painted, cmds, svgCmds, svgTris, b.gl.composites,
		float64(g1.allocBytes-g0.allocBytes),
		float64(g2.allocBytes-g1.allocBytes),
		float64(g1.cycles-g0.cycles),
		float64(g2.cycles-g1.cycles),
		(g2.gcCPUSec-g0.gcCPUSec)*1000,
		b.gl.filled, b.gl.direct)
	return painted
}
