//go:build !js && !darwin && !android

package gl

import (
	"time"

	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/rstats"
)

// gpuTimerRing is how many frames a GL_TIME_ELAPSED result may lag. A
// result is read only when the driver says it is available, so the ring
// never makes the CPU wait for the GPU. Three matches the Metal ring.
const gpuTimerRing = 3

// gpuTimer measures the GPU time of each frame with GL_TIME_ELAPSED
// queries (GL 3.3 core) for GOGUI_RENDER_STATS (#1303). It is used only
// when the stats are on. A frame's query is read gpuTimerRing frames
// later, if it is done then; a query that is not done is skipped, and
// that frame adds no sample.
//
// Queries cannot nest. Nothing else in the backend uses GL_TIME_ELAPSED,
// so one query spans the whole frame, offscreen passes included.
type gpuTimer struct {
	ids    [gpuTimerRing]uint32
	issued [gpuTimerRing]bool
	slot   int
	ready  bool
	open   bool
}

// begin reads the result that the next slot holds, if it is available,
// into stats, then starts that slot's query for this frame.
func (t *gpuTimer) begin(stats *rstats.Recorder) {
	if stats == nil {
		return
	}
	if !t.ready {
		gogl.GenQueries(gpuTimerRing, &t.ids[0])
		t.ready = true
	}
	t.slot = (t.slot + 1) % gpuTimerRing
	id := t.ids[t.slot]
	if t.issued[t.slot] {
		var avail int32
		gogl.GetQueryObjectiv(id, gogl.QUERY_RESULT_AVAILABLE, &avail)
		if avail != 0 {
			var ns uint64
			gogl.GetQueryObjectui64v(id, gogl.QUERY_RESULT, &ns)
			stats.GPU(time.Duration(ns))
		}
		// A result that is not available is dropped: beginning the
		// query again replaces it.
	}
	gogl.BeginQuery(gogl.TIME_ELAPSED, id)
	t.open = true
}

// end closes the query that begin opened.
func (t *gpuTimer) end() {
	if !t.open {
		return
	}
	gogl.EndQuery(gogl.TIME_ELAPSED)
	t.issued[t.slot] = true
	t.open = false
}

// destroy deletes the queries. The context must be current.
func (t *gpuTimer) destroy() {
	if t.ready {
		gogl.DeleteQueries(gpuTimerRing, &t.ids[0])
		*t = gpuTimer{}
	}
}
