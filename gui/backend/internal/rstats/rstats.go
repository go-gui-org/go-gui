// Package rstats counts what a GPU backend sends to the driver each frame:
// draw calls, the reason each quad batch ended, state binds, uploaded bytes,
// CPU submit time and GPU time. It exists for the batching investigation
// (#1303), so that claims about draw-call volume or state churn rest on
// numbers and not on reading the code.
//
// It is off by default. Set GOGUI_RENDER_STATS=1 and each backend prints
// one summary line per second to stderr. The line has a fixed set of
// key=value fields, so a script can turn it into a CSV row.
//
// When the variable is not set, FromEnv returns nil, and every method on
// a nil *Recorder returns at once. A backend keeps the pointer and calls
// the methods without its own check. When the variable is set, the
// Recorder still does not allocate per frame: samples go to fixed arrays,
// and the line is built in a buffer that is kept.
//
// It is a separate package so that Metal and GL count the same way, and
// so that the gui package gets no new exported surface.
package rstats

import (
	"io"
	"os"
	"slices"
	"strconv"
	"time"
)

// Env is the variable that turns the counters on.
const Env = "GOGUI_RENDER_STATS"

// Cause is the reason a quad batch was drawn. Each cause is one field in
// the summary line, so the line shows which state change splits the
// batches the most.
type Cause uint8

// The causes. A backend uses the ones that apply to it.
const (
	CausePipeline  Cause = iota // a different pipeline or program
	CauseMVP                    // a different projection matrix
	CauseUniform                // a different per-draw vertex uniform (tm)
	CauseFrag0                  // a different fragment uniform block
	CauseScissor                // a different or empty scissor rect
	CauseTexture                // a different texture
	CauseSampler                // a different sampler
	CauseStride                 // a different vertex layout
	CauseCap                    // the batch is full
	CauseChunk                  // the next quad is in a new vertex buffer
	CauseTriangles              // a triangle mesh is drawn on its own
	CausePass                   // an encoder or render pass ends
	CauseText                   // a text command ends (GL glyph batch)
	CauseOther                  // anything else
	causeCount
)

// causeNames are the summary field suffixes, in Cause order.
var causeNames = [causeCount]string{
	"pipeline", "mvp", "uniform", "frag0", "scissor", "texture", "sampler",
	"stride", "cap", "chunk", "triangles", "pass", "text", "other",
}

// String returns the field suffix of c.
func (c Cause) String() string {
	if c >= causeCount {
		return "other"
	}
	return causeNames[c]
}

// Counts are the totals over one summary period. They are exported to
// the backends' tests through Recorder.Counts.
type Counts struct {
	Frames    uint64
	Draws     uint64             // draw calls of any kind
	Quads     uint64             // quads drawn, batched or not
	Verts     uint64             // triangle-mesh vertices drawn
	Flush     [causeCount]uint64 // batched draws, by the cause that ended them
	PipeBinds uint64             // pipeline or program binds sent
	TexBinds  uint64             // texture binds sent
	Uniforms  uint64             // uniform uploads sent
	Upload    uint64             // vertex bytes copied to the GPU
	Grows     uint64             // GPU buffers created or grown
}

// maxSamples bounds the per-period samples of submit and GPU time. At
// 240 Hz a 1 s period holds 240 of them. Past the bound, later frames
// still count but add no sample.
const maxSamples = 1024

// Recorder collects one backend's counters. It is used from the thread
// that renders, so it has no locks.
type Recorder struct {
	name   string
	out    io.Writer
	now    func() time.Time
	period time.Duration

	sum     Counts
	submit  [maxSamples]int64 // CPU ns from BeginFrame to EndFrame
	nSubmit int
	gpu     [maxSamples]int64 // GPU ns, reported late by the backend
	nGPU    int

	frameStart  time.Time
	periodStart time.Time
	buf         []byte // the summary line, kept between periods
}

// FromEnv returns a Recorder that prints to stderr once a second when
// GOGUI_RENDER_STATS is set to anything other than "" or "0". Otherwise
// it returns nil. name is the backend, for example "metal".
func FromEnv(name string) *Recorder {
	if v := os.Getenv(Env); v == "" || v == "0" {
		return nil
	}
	return New(name, os.Stderr, time.Now, time.Second)
}

// New returns a Recorder that writes a summary to out at the end of the
// first frame after each period. Tests pass a fake clock as now.
func New(name string, out io.Writer, now func() time.Time, period time.Duration) *Recorder {
	return &Recorder{
		name: name, out: out, now: now, period: period,
		buf: make([]byte, 0, 1024),
	}
}

// Flush records one draw call of a batch of quads, ended by cause.
func (r *Recorder) Flush(cause Cause, quads int) {
	if r == nil || quads <= 0 {
		return
	}
	if cause >= causeCount {
		cause = CauseOther
	}
	r.sum.Draws++
	r.sum.Quads += uint64(quads)
	r.sum.Flush[cause]++
}

// Draw records one draw call of quads that was never part of a batch.
func (r *Recorder) Draw(quads int) {
	if r == nil {
		return
	}
	r.sum.Draws++
	r.sum.Quads += uint64(max(quads, 0))
}

// DrawVerts records one draw call of a triangle mesh with n vertices.
func (r *Recorder) DrawVerts(n int) {
	if r == nil {
		return
	}
	r.sum.Draws++
	r.sum.Verts += uint64(max(n, 0))
}

// PipelineBind records a pipeline or program bind sent to the driver.
func (r *Recorder) PipelineBind() {
	if r != nil {
		r.sum.PipeBinds++
	}
}

// TextureBind records a texture bind sent to the driver.
func (r *Recorder) TextureBind() {
	if r != nil {
		r.sum.TexBinds++
	}
}

// Uniform records a uniform upload sent to the driver.
func (r *Recorder) Uniform() {
	if r != nil {
		r.sum.Uniforms++
	}
}

// Upload records n vertex bytes copied to the GPU.
func (r *Recorder) Upload(n int) {
	if r != nil && n > 0 {
		r.sum.Upload += uint64(n)
	}
}

// Grow records a GPU buffer that was created or grown during a frame.
func (r *Recorder) Grow() {
	if r != nil {
		r.sum.Grows++
	}
}

// BeginFrame starts the CPU submit timer of a frame.
func (r *Recorder) BeginFrame() {
	if r == nil {
		return
	}
	r.frameStart = r.now()
	if r.periodStart.IsZero() {
		r.periodStart = r.frameStart
	}
}

// EndFrame stops the submit timer and, when the period is over, writes
// the summary and starts a new period.
func (r *Recorder) EndFrame() {
	if r == nil || r.frameStart.IsZero() {
		return
	}
	t := r.now()
	r.sum.Frames++
	if r.nSubmit < maxSamples {
		r.submit[r.nSubmit] = int64(t.Sub(r.frameStart))
		r.nSubmit++
	}
	r.frameStart = time.Time{}
	if t.Sub(r.periodStart) >= r.period {
		r.report()
		r.periodStart = t
	}
}

// GPU records the GPU time of a frame. A backend reads it some frames
// late, when the GPU is done, so it may land in the next period.
func (r *Recorder) GPU(d time.Duration) {
	if r == nil || d <= 0 || r.nGPU >= maxSamples {
		return
	}
	r.gpu[r.nGPU] = int64(d)
	r.nGPU++
}

// Counts returns the totals of the open period. Tests use it.
func (r *Recorder) Counts() Counts {
	if r == nil {
		return Counts{}
	}
	return r.sum
}

// report writes the summary of the period and clears the counters. Each
// count is a mean per frame, so periods with different frame counts
// compare directly. Times are in microseconds; gpu_us is -1 when the
// backend reported no GPU time.
func (r *Recorder) report() {
	n := float64(r.sum.Frames)
	b := r.buf[:0]
	b = append(b, "rstats backend="...)
	b = append(b, r.name...)
	b = appendInt(b, " frames=", int64(r.sum.Frames))
	b = appendMean(b, " draws=", r.sum.Draws, n)
	b = appendMean(b, " quads=", r.sum.Quads, n)
	b = appendMean(b, " verts=", r.sum.Verts, n)
	b = appendMean(b, " pipe_binds=", r.sum.PipeBinds, n)
	b = appendMean(b, " tex_binds=", r.sum.TexBinds, n)
	b = appendMean(b, " uniforms=", r.sum.Uniforms, n)
	b = appendMean(b, " upload_kib=", r.sum.Upload/1024, n)
	b = appendMean(b, " grows=", r.sum.Grows, n)
	b = appendPct(b, " submit_us", r.submit[:r.nSubmit])
	b = appendPct(b, " gpu_us", r.gpu[:r.nGPU])
	for i := range causeCount {
		b = append(b, " flush_"...)
		b = append(b, causeNames[i]...)
		b = appendMean(b, "=", r.sum.Flush[i], n)
	}
	b = append(b, '\n')
	_, _ = r.out.Write(b)
	r.buf = b
	r.sum = Counts{}
	r.nSubmit, r.nGPU = 0, 0
}

// appendInt appends key and v.
func appendInt(b []byte, key string, v int64) []byte {
	b = append(b, key...)
	return strconv.AppendInt(b, v, 10)
}

// appendMean appends key and total/frames with one decimal.
func appendMean(b []byte, key string, total uint64, frames float64) []byte {
	b = append(b, key...)
	if frames <= 0 {
		return append(b, '0')
	}
	return strconv.AppendFloat(b, float64(total)/frames, 'f', 1, 64)
}

// appendPct appends key_p50 and key_p99 of the ns samples, in µs. It
// sorts the samples in place: they are cleared after the report.
func appendPct(b []byte, key string, s []int64) []byte {
	p50, p99 := int64(-1), int64(-1)
	if len(s) > 0 {
		slices.Sort(s)
		p50 = s[(len(s)-1)*50/100] / 1000
		p99 = s[(len(s)-1)*99/100] / 1000
	}
	b = append(b, key...)
	b = appendInt(b, "_p50=", p50)
	b = append(b, key...)
	return appendInt(b, "_p99=", p99)
}
