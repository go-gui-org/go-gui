package gui

import "testing"

// benchStops returns a 5-stop gradient typical of palette ramps.
func benchStops() []GradientStop {
	return []GradientStop{
		{Color: RGBA(255, 200, 80, 255), Pos: 0},
		{Color: RGBA(255, 120, 40, 255), Pos: 0.25},
		{Color: RGBA(220, 60, 20, 255), Pos: 0.5},
		{Color: RGBA(180, 30, 10, 255), Pos: 0.75},
		{Color: RGBA(120, 10, 5, 255), Pos: 1},
	}
}

func BenchmarkSampleGradientStopColor(b *testing.B) {
	stops := benchStops()
	norm := make([]GradientStop, 0, len(stops))
	norm = NormalizeGradientStops(stops, &norm)
	// 5k vertices like one solar_system batch; positions sweep the ramp.
	positions := make([]float32, 5000)
	for i := range positions {
		positions[i] = float32(i%1000) / 999.0
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range positions {
			_ = SampleGradientStopColor(norm, p)
		}
	}
}

func BenchmarkSampleGradRamp(b *testing.B) {
	stops := benchStops()
	norm := make([]GradientStop, 0, len(stops))
	norm = NormalizeGradientStops(stops, &norm)
	var segs []gradRampSegment
	segs = prepareGradRamp(norm, &segs)
	positions := make([]float32, 5000)
	for i := range positions {
		positions[i] = float32(i%1000) / 999.0
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range positions {
			_ = sampleGradRamp(norm, segs, p)
		}
	}
}

func BenchmarkPrepareGradRamp(b *testing.B) {
	stops := benchStops()
	norm := make([]GradientStop, 0, len(stops))
	norm = NormalizeGradientStops(stops, &norm)
	var segs []gradRampSegment
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = prepareGradRamp(norm, &segs)
	}
}
