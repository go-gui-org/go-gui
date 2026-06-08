package gui

import "testing"

type benchSvgParser struct{}

func (benchSvgParser) ParseSvg(_ string) (*SvgParsed, error) {
	return &SvgParsed{
		TextPaths: []SvgTextPath{
			{Text: "bench", PathID: "p1", FontFamily: "sans", FontSize: 12},
		},
		DefsPaths: map[string]string{
			"p1": "M 0 0 L 100 0 L 100 100",
		},
		Paths: []TessellatedPath{
			{
				Triangles: []float32{0, 0, 100, 0, 50, 100},
				Color:     SvgColor{R: 255, G: 0, B: 0, A: 255},
				PathID:    1,
			},
		},
		Animations: []SvgAnimation{
			{
				Kind:    SvgAnimOpacity,
				GroupID: "g1",
				DurSec:  1,
				Values:  []float32{1, 0.5},
			},
		},
		Width:  100,
		Height: 100,
	}, nil
}

func (benchSvgParser) ParseSvgFile(_ string) (*SvgParsed, error) {
	return benchSvgParser{}.ParseSvg("")
}

func (benchSvgParser) ParseSvgDimensions(_ string) (float32, float32, error) {
	return 100, 100, nil
}

func (benchSvgParser) Tessellate(parsed *SvgParsed, _ float32) []TessellatedPath {
	return parsed.Paths
}

func BenchmarkRenderSvgAnimated(b *testing.B) {
	w := NewWindow(WindowCfg{Width: 200, Height: 200})
	w.SetSvgParser(benchSvgParser{})
	shape := &Shape{
		shapeType: shapeSVG,
		X:         0,
		Y:         0,
		Width:     100,
		Height:    100,
		Resource:  "<svg/>",
		Color:     White,
	}
	clip := drawClip{X: 0, Y: 0, Width: 200, Height: 200}
	b.ReportAllocs()
	for b.Loop() {
		w.renderers = w.renderers[:0]
		renderSvg(shape, clip, w)
	}
}

func BenchmarkBuildDefsPathDataCache(b *testing.B) {
	textPaths := []SvgTextPath{{PathID: "p1"}, {PathID: "p2"}}
	filtered := []SvgParsedFilteredGroup{
		{TextPaths: []SvgTextPath{{PathID: "p3"}}},
	}
	defs := map[string]string{
		"p1": "M 0 0 L 100 0",
		"p2": "M 0 0 L 0 100",
		"p3": "M 0 0 L 100 100",
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = buildDefsPathDataCache(textPaths, filtered, defs, 1.0)
	}
}
