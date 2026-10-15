package framestats

import (
	"math"
	"testing"
	"time"
)

// TestFPSIsFramesOverTime is the regression test for #1024. A mean of per-frame
// FPS values gives (500 + 33.3) / 2 = 266 FPS for one 2 ms and one 30 ms frame.
// The true rate is 2 frames in 32 ms = 62.5 FPS.
func TestFPSIsFramesOverTime(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(2 * time.Millisecond)
	s.Add(30 * time.Millisecond)
	got := s.Summary()
	if math.Abs(got.FPS-62.5) > 1e-9 {
		t.Errorf("FPS = %v, want 62.5", got.FPS)
	}
	if got.Mean != 16*time.Millisecond {
		t.Errorf("Mean = %v, want 16ms", got.Mean)
	}
}

// TestOneStall checks that one 30 ms stall in 500 frames shows in Max, while
// the mean and the median hardly move.
func TestOneStall(t *testing.T) {
	t.Parallel()
	var s Stats
	for range 499 {
		s.Add(4 * time.Millisecond)
	}
	s.Add(30 * time.Millisecond)
	got := s.Summary()
	if got.Frames != 500 {
		t.Errorf("Frames = %d, want 500", got.Frames)
	}
	if got.P50 != 4*time.Millisecond {
		t.Errorf("P50 = %v, want 4ms", got.P50)
	}
	if got.Max != 30*time.Millisecond {
		t.Errorf("Max = %v, want 30ms", got.Max)
	}
	// 500 frames in 2026 ms.
	if want := 500 / 2.026; math.Abs(got.FPS-want) > 0.01 {
		t.Errorf("FPS = %v, want %v", got.FPS, want)
	}
}

// TestP99 checks the nearest-rank p99: in 100 frames, the 99th smallest.
func TestP99(t *testing.T) {
	t.Parallel()
	var s Stats
	for i := range 100 {
		s.Add(time.Duration(i+1) * time.Millisecond)
	}
	got := s.Summary()
	if got.P99 != 99*time.Millisecond {
		t.Errorf("P99 = %v, want 99ms", got.P99)
	}
	if got.P50 != 50*time.Millisecond {
		t.Errorf("P50 = %v, want 50ms", got.P50)
	}
	if got.Max != 100*time.Millisecond {
		t.Errorf("Max = %v, want 100ms", got.Max)
	}
}

// TestRingKeepsNewest checks that, after the ring fills, the oldest frames
// drop out and the newest stay.
func TestRingKeepsNewest(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(time.Second) // goes out when the ring wraps
	for range Size {
		s.Add(time.Millisecond)
	}
	got := s.Summary()
	if got.Frames != Size {
		t.Errorf("Frames = %d, want %d", got.Frames, Size)
	}
	if got.Max != time.Millisecond {
		t.Errorf("Max = %v, want 1ms (the 1 s frame should be gone)", got.Max)
	}
	if s.Len() != Size {
		t.Errorf("Len = %d, want %d", s.Len(), Size)
	}
}

// TestSummaryKeepsOrder checks that Summary sorts a copy: a later Add still
// replaces the oldest frame, not the smallest one.
func TestSummaryKeepsOrder(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(time.Second)
	for range Size - 1 {
		s.Add(time.Millisecond)
	}
	_ = s.Summary()
	s.Add(time.Millisecond) // replaces the 1 s frame, the oldest
	if got := s.Summary().Max; got != time.Millisecond {
		t.Errorf("Max = %v, want 1ms", got)
	}
}

func TestEmpty(t *testing.T) {
	t.Parallel()
	var s Stats
	if got := s.Summary(); got != (Summary{}) {
		t.Errorf("empty Summary = %+v, want zero", got)
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(time.Millisecond)
	s.Reset()
	if s.Len() != 0 {
		t.Errorf("Len after Reset = %d, want 0", s.Len())
	}
	if got := s.Summary(); got != (Summary{}) {
		t.Errorf("Summary after Reset = %+v, want zero", got)
	}
}

// TestZeroTotal checks that frames of zero length do not divide by zero.
func TestZeroTotal(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(0)
	if got := s.Summary(); got.FPS != 0 || got.Frames != 1 {
		t.Errorf("Summary = %+v, want FPS 0, Frames 1", got)
	}
}

func TestSummaryNoAlloc(t *testing.T) {
	var s Stats
	for i := range Size + 10 {
		s.Add(time.Duration(i%37) * time.Millisecond)
	}
	if n := testing.AllocsPerRun(10, func() { _ = s.Summary() }); n != 0 {
		t.Errorf("Summary allocs = %v, want 0", n)
	}
}

func TestMs(t *testing.T) {
	t.Parallel()
	if got := Ms(1500 * time.Microsecond); got != 1.5 {
		t.Errorf("Ms(1.5ms) = %v, want 1.5", got)
	}
}

// TestNegativeIsZero checks that a negative frame time counts as 0.
func TestNegativeIsZero(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(-time.Second)
	s.Add(10 * time.Millisecond)
	got := s.Summary()
	if got.P50 != 0 || got.Mean != 5*time.Millisecond || math.Abs(got.FPS-200) > 1e-9 {
		t.Errorf("Summary = %+v, want P50 0, Mean 5ms, FPS 200", got)
	}
}

// TestOneFrame checks the single-frame case: every percentile is that frame.
func TestOneFrame(t *testing.T) {
	t.Parallel()
	var s Stats
	s.Add(7 * time.Millisecond)
	got := s.Summary()
	d := 7 * time.Millisecond
	if got.Frames != 1 || got.P50 != d || got.P99 != d || got.Max != d || got.Mean != d {
		t.Errorf("Summary = %+v, want every field 7ms, Frames 1", got)
	}
}
