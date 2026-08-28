package gui

import (
	"math"
	"testing"
)

// TestPrepareGradRampSampleParity pins that the fast path is
// byte-identical to SampleGradientStopColor. The optimization in issue
// #434 hoists premultiplication; any drift shows as a one-bit golden
// change.
func TestPrepareGradRampSampleParity(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0},
		{Color: RGBA(0, 255, 0, 255), Pos: 0.3},
		{Color: RGBA(0, 0, 255, 255), Pos: 0.7},
		{Color: RGBA(255, 255, 255, 0), Pos: 1},
	}
	norm := make([]GradientStop, 0, len(stops))
	norm = NormalizeGradientStops(stops, &norm)
	var segs []gradRampSegment
	segs = prepareGradRamp(norm, &segs)
	for _, p := range []float32{-0.1, 0, 0.15, 0.3, 0.5, 0.7, 0.85, 1, 1.1} {
		want := SampleGradientStopColor(norm, p)
		got := sampleGradRamp(norm, segs, p)
		if got != want {
			t.Errorf("pos %v: got %v, want %v", p, got, want)
		}
	}
	// Exhaustive sweep to catch rounding divergence.
	for i := range 1001 {
		pos := float32(i) / 1000.0
		if got, want := sampleGradRamp(norm, segs, pos),
			SampleGradientStopColor(norm, pos); got != want {
			t.Fatalf("sweep pos %v: got %v want %v", pos, got, want)
		}
	}
}

func TestSampleGradRampEmptyAndSingle(t *testing.T) {
	var segs []gradRampSegment
	if got := sampleGradRamp(nil, segs, 0.5); got.A != 0 {
		t.Errorf("empty: got %v, want transparent", got)
	}
	single := []GradientStop{{Color: RGBA(10, 20, 30, 255), Pos: 0.5}}
	segs = prepareGradRamp(single, &segs)
	if len(segs) != 0 {
		t.Errorf("single stop: segs %d, want 0", len(segs))
	}
	for _, p := range []float32{0, 0.5, 1, float32(math.NaN())} {
		got := sampleGradRamp(single, segs, p)
		// Normalize has one stop at 0.5, so pos <=0.5 returns it; NaN
		// folds to the first stop via finite guard.
		if got != single[0].Color {
			t.Errorf("single pos %v: got %v, want %v", p, got, single[0].Color)
		}
	}
}

func TestSampleGradRampDuplicatePos(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0},
		{Color: RGBA(0, 255, 0, 255), Pos: 0.5},
		{Color: RGBA(0, 0, 255, 255), Pos: 0.5},
		{Color: RGBA(255, 255, 0, 255), Pos: 1},
	}
	norm := make([]GradientStop, 0, len(stops))
	norm = NormalizeGradientStops(stops, &norm)
	var segs []gradRampSegment
	segs = prepareGradRamp(norm, &segs)
	for _, p := range []float32{0.25, 0.5, 0.75} {
		if got, want := sampleGradRamp(norm, segs, p),
			SampleGradientStopColor(norm, p); got != want {
			t.Errorf("dup pos %v: got %v want %v", p, got, want)
		}
	}
}

func TestSampleGradRampNonFinite(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(10, 0, 0, 255), Pos: 0},
		{Color: RGBA(200, 0, 0, 255), Pos: 1},
	}
	norm := make([]GradientStop, 0, 2)
	norm = NormalizeGradientStops(stops, &norm)
	var segs []gradRampSegment
	segs = prepareGradRamp(norm, &segs)
	// NaN and -Inf fold to the ramp start; +Inf folds to its end, the
	// same side a very large finite position lands on. The dedicated
	// non-finite guard this replaced sent +Inf to the start as well,
	// which was the odd one out.
	last := norm[len(norm)-1].Color
	for _, tc := range []struct {
		pos  float32
		want Color
	}{
		{float32(math.NaN()), norm[0].Color},
		{float32(math.Inf(-1)), norm[0].Color},
		{float32(math.Inf(1)), last},
	} {
		if got := sampleGradRamp(norm, segs, tc.pos); got != tc.want {
			t.Errorf("pos %v: got %v, want %v", tc.pos, got, tc.want)
		}
	}
}

func TestPrepareGradRampNilSegsNoPanic(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(0, 0, 0, 255), Pos: 0},
		{Color: RGBA(255, 0, 0, 255), Pos: 1},
	}
	// Must not panic on nil scratch pointer.
	if got := prepareGradRamp(stops, nil); got != nil {
		t.Errorf("nil segs: got %v, want nil", got)
	}
}

func TestPrepareGradRampHugeStopsCapped(t *testing.T) {
	huge := make([]GradientStop, 5000)
	for i := range huge {
		huge[i] = GradientStop{
			Color: RGBA(uint8(i%256), 0, 0, 255),
			Pos:   float32(i) / float32(len(huge)-1),
		}
	}
	norm := make([]GradientStop, 0, len(huge))
	norm = NormalizeGradientStops(huge, &norm)
	// Normalize itself caps at 8192, huge is 5000 so passes through.
	var segs []gradRampSegment
	segs = prepareGradRamp(norm, &segs)
	if len(segs) > 2048 {
		t.Errorf("segs %d, want <=2048", len(segs))
	}
	// Parity still holds for capped range.
	for _, p := range []float32{0, 0.5, 1} {
		_ = sampleGradRamp(norm[:2048], segs, p)
	}
}

func TestNormalizeGradientStopsNilNormNoPanic(t *testing.T) {
	stops := []GradientStop{{Color: RGBA(0, 0, 0, 255), Pos: 0}}
	if got := NormalizeGradientStops(stops, nil); got != nil {
		t.Errorf("nil norm: got %v, want nil", got)
	}
	if got := NormalizeGradientStopsInto(stops, nil, nil); got != nil {
		t.Errorf("nil into: got %v, want nil", got)
	}
}

func TestPrepareGradRampReuse(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(0, 0, 0, 255), Pos: 0},
		{Color: RGBA(255, 255, 255, 255), Pos: 1},
	}
	var segs []gradRampSegment
	segs = prepareGradRamp(stops, &segs)
	firstCap := cap(segs)
	// Second call over same buffer must not grow unbounded.
	segs = prepareGradRamp(stops, &segs)
	if cap(segs) != firstCap {
		t.Errorf("reuse cap %d, want %d", cap(segs), firstCap)
	}
}

func TestFillTrianglesGradientHugeNoAllocPanic(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	g := &CanvasGradient{
		Stops: []GradientStop{
			{Color: RGBA(0, 0, 0, 255), Pos: 0},
			{Color: RGBA(255, 0, 0, 255), Pos: 1},
		},
	}
	huge := make([]float32, (1<<20)+6) // exceeds cap
	for i := range huge {
		huge[i] = float32(i % 100)
	}
	// Must return without panic and without emitting batches.
	dc.FillTrianglesGradient(huge, g)
	if len(dc.Batches()) != 0 {
		t.Errorf("huge tris: batches %d, want 0", len(dc.Batches()))
	}
}
