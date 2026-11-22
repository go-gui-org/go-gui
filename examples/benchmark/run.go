package main

import (
	"fmt"
	"io"
	"time"

	"github.com/go-gui-org/go-gui/examples/internal/framestats"
)

// runCSVHeader names the columns of the row a timed run writes. The
// times are frame wall time (p50, p99, max) and go-gui's mean CPU phase
// times. tools/renderbaseline joins this row with the backend's
// GOGUI_RENDER_STATS line (#1303).
const runCSVHeader = "type,count,frames,seconds,fps,p50_ms,p99_ms,max_ms,view_us,layout_us,render_us"

// run is a timed measurement with no user input (-seconds > 0). It waits
// for warmup, unmeasured, so that caches and GPU buffers settle, then
// measures for measure and writes one CSV row to out.
type run struct {
	out             io.Writer
	warmup, measure time.Duration
	start           time.Time // first frame
	measStart       time.Time // start of the measured part; zero in warmup
	done            bool
}

// step advances the run by one frame that ends at now, after app.sample
// has recorded it. It returns true once the row is written. Window.Close
// is asynchronous, so the view may run again: step keeps returning true.
func (r *run) step(app *App, now time.Time) bool {
	if r.done {
		return true
	}
	if r.start.IsZero() {
		r.start = now
		return false
	}
	if r.measStart.IsZero() {
		if now.Sub(r.start) >= r.warmup {
			r.measStart = now
			app.ResetAvgs()
		}
		return false
	}
	el := now.Sub(r.measStart)
	if el < r.measure {
		return false
	}
	f := app.Frames.Summary()
	v, l, rb := app.View.Summary(), app.Layout.Summary(), app.Render.Summary()
	// A failed write to stdout has nowhere better to be reported.
	_, _ = fmt.Fprintf(r.out, "%s,%d,%d,%.2f,%.1f,%.3f,%.3f,%.3f,%.1f,%.1f,%.1f\n",
		app.WidgetType, app.WidgetCount, f.Frames, el.Seconds(), f.FPS,
		framestats.Ms(f.P50), framestats.Ms(f.P99), framestats.Ms(f.Max),
		us(v.Mean), us(l.Mean), us(rb.Mean))
	r.done = true
	return true
}

// secs converts a seconds flag to a Duration. NaN and negative values
// become 0, and the value is capped at an hour so it cannot overflow.
func secs(s float64) time.Duration {
	if !(s > 0) { // also catches NaN
		return 0
	}
	return time.Duration(min(s, 3600) * float64(time.Second))
}
