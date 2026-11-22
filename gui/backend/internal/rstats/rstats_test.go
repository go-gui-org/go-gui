package rstats

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// fakeClock advances by step on each call, so every frame takes step.
type fakeClock struct {
	t    time.Time
	step time.Duration
}

func (f *fakeClock) now() time.Time {
	f.t = f.t.Add(f.step)
	return f.t
}

func TestNilRecorderIsNoOp(t *testing.T) {
	var r *Recorder
	r.BeginFrame()
	r.Flush(CausePipeline, 3)
	r.Draw(1)
	r.DrawVerts(9)
	r.PipelineBind()
	r.TextureBind()
	r.Uniform()
	r.Upload(10)
	r.Grow()
	r.GPU(time.Millisecond)
	r.EndFrame()
	if c := r.Counts(); c != (Counts{}) {
		t.Fatalf("nil recorder counted %+v", c)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(Env, "")
	if FromEnv("x") != nil {
		t.Fatal("empty env: want nil")
	}
	t.Setenv(Env, "0")
	if FromEnv("x") != nil {
		t.Fatal(`"0": want nil`)
	}
	t.Setenv(Env, "1")
	if FromEnv("x") == nil {
		t.Fatal(`"1": want a recorder`)
	}
}

func TestCountsByCause(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0), step: time.Millisecond}
	r := New("t", io.Discard, clk.now, time.Hour)
	r.BeginFrame()
	r.Flush(CausePipeline, 4)
	r.Flush(CauseScissor, 2)
	r.Flush(CauseScissor, 1)
	r.Flush(CauseTexture, 0) // an empty batch is not a draw
	r.Flush(Cause(200), 1)   // out of range goes to other
	r.Draw(1)
	r.DrawVerts(30)
	r.EndFrame()
	c := r.Counts()
	if c.Draws != 6 || c.Quads != 9 || c.Verts != 30 || c.Frames != 1 {
		t.Fatalf("counts = %+v", c)
	}
	if c.Flush[CausePipeline] != 1 || c.Flush[CauseScissor] != 2 ||
		c.Flush[CauseTexture] != 0 || c.Flush[CauseOther] != 1 {
		t.Fatalf("flush = %v", c.Flush)
	}
}

func TestReportLine(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0), step: 100 * time.Microsecond}
	var out bytes.Buffer
	r := New("metal", &out, clk.now, time.Second)
	// Each frame is two clock reads (Begin, End), 100 µs apart: 100 µs of
	// submit time, 200 µs of clock per frame.
	// The period closes on the first EndFrame at least 1 s after the
	// first BeginFrame: just over 5000 frames of 2 reads each.
	for range 6000 {
		r.BeginFrame()
		r.Flush(CauseScissor, 10)
		r.Upload(2048)
		r.GPU(500 * time.Microsecond)
		r.EndFrame()
		if out.Len() > 0 {
			break
		}
	}
	line := out.String()
	for _, want := range []string{
		"rstats backend=metal ", " draws=1.0 ", " quads=10.0 ",
		" upload_kib=2.0 ", " submit_us_p50=100 ", " gpu_us_p50=500 ",
		" flush_scissor=1.0", " flush_pipeline=0.0", "\n",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %q:\n%s", want, line)
		}
	}
	if c := r.Counts(); c.Frames != 0 {
		t.Fatalf("counters not cleared after report: %+v", c)
	}
}

func TestReportWithoutGPUSamples(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0), step: time.Second}
	var out bytes.Buffer
	r := New("gl", &out, clk.now, time.Second)
	r.BeginFrame()
	r.EndFrame()
	if !strings.Contains(out.String(), " gpu_us_p50=-1 gpu_us_p99=-1") {
		t.Fatalf("want -1 GPU fields:\n%s", out.String())
	}
}

// TestRecorderDoesNotAllocate holds the promise in the package doc: an
// enabled Recorder adds no per-frame allocations, the report included.
func TestRecorderDoesNotAllocate(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0), step: 300 * time.Millisecond}
	r := New("t", io.Discard, clk.now, time.Second)
	frame := func() {
		r.BeginFrame()
		r.Flush(CausePipeline, 5)
		r.Draw(1)
		r.DrawVerts(3)
		r.PipelineBind()
		r.TextureBind()
		r.Uniform()
		r.Upload(144)
		r.Grow()
		r.GPU(time.Millisecond)
		r.EndFrame() // reports every other frame at this clock step
	}
	frame()
	frame() // warm the line buffer
	if n := testing.AllocsPerRun(100, frame); n != 0 {
		t.Fatalf("enabled recorder: %v allocs/frame, want 0", n)
	}
	var nilRec *Recorder
	if n := testing.AllocsPerRun(100, func() {
		nilRec.BeginFrame()
		nilRec.Flush(CausePipeline, 5)
		nilRec.EndFrame()
	}); n != 0 {
		t.Fatalf("nil recorder: %v allocs/frame, want 0", n)
	}
}
