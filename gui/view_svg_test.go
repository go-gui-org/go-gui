package gui

import "testing"

func TestSvgFactory(t *testing.T) {
	v := Svg(SvgCfg{ID: "svg1", SvgData: "<svg></svg>"})
	if v == nil {
		t.Fatal("Svg factory returned nil")
	}
	if _, ok := v.(*svgView); !ok {
		t.Fatal("expected *svgView")
	}
}

func TestSvgContent(t *testing.T) {
	v := Svg(SvgCfg{ID: "svg1"})
	if v.Content() != nil {
		t.Fatal("expected nil Content")
	}
}

func TestSvgGenerateLayoutErrorFallback(t *testing.T) {
	w := &Window{}
	v := Svg(SvgCfg{
		ID:       "svg1",
		FileName: "/nonexistent.svg",
		Width:    100,
		Height:   100,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	// No parser → error fallback → text shape.
	if layout.Shape.shapeType != shapeText {
		t.Fatalf("expected shapeText for error, got %d",
			layout.Shape.shapeType)
	}
}

func TestSvgGenerateLayoutNeedsDimensions(t *testing.T) {
	w := &Window{}
	v := Svg(SvgCfg{
		ID:      "svg1",
		SvgData: "<svg></svg>",
		// Width/Height = 0 → needs dimension lookup.
	})
	layout := v.GenerateLayout(w)
	// No parser → dimension lookup fails → error text.
	if layout.Shape.shapeType != shapeText {
		t.Fatalf("expected error fallback, got %d",
			layout.Shape.shapeType)
	}
}

// mockSvgParser is a minimal SvgParser for testing.
type mockSvgParser struct {
	width, height float32
}

func (m *mockSvgParser) ParseSvg(_ string) (*SvgParsed, error) {
	return &SvgParsed{
		Width:  m.width,
		Height: m.height,
	}, nil
}

func (m *mockSvgParser) ParseSvgFile(_ string) (*SvgParsed, error) {
	return &SvgParsed{
		Width:  m.width,
		Height: m.height,
	}, nil
}

func (m *mockSvgParser) ParseSvgDimensions(_ string) (float32, float32, error) {
	return m.width, m.height, nil
}

func (m *mockSvgParser) Tessellate(_ *SvgParsed, _ float32) []TessellatedPath {
	return []TessellatedPath{
		{
			Triangles: []float32{0, 0, 10, 0, 5, 10, 5, 10, 10, 0, 10, 10},
			Color:     SvgColor{0, 0, 0, 255},
		},
	}
}

func TestSvgGenerateLayoutWithParser(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 64, height: 64})
	v := Svg(SvgCfg{
		ID:      "svg1",
		SvgData: "<svg></svg>",
		Width:   100,
		Height:  100,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.shapeType != shapeSVG {
		t.Fatalf("expected shapeSVG, got %d",
			layout.Shape.shapeType)
	}
	if layout.Shape.Width != 100 {
		t.Fatalf("expected width 100, got %f",
			layout.Shape.Width)
	}
}

func TestSvgGenerateLayoutAutoSize(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 200, height: 150})
	v := Svg(SvgCfg{
		ID:      "svg1",
		SvgData: "<svg></svg>",
		// No explicit width/height.
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.shapeType != shapeSVG {
		t.Fatalf("expected shapeSVG, got %d",
			layout.Shape.shapeType)
	}
	if layout.Shape.Width != 200 || layout.Shape.Height != 150 {
		t.Fatalf("expected 200x150, got %fx%f",
			layout.Shape.Width, layout.Shape.Height)
	}
}

func TestSvgWithOnClick(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 64, height: 64})
	clicked := false
	v := Svg(SvgCfg{
		ID:      "svg1",
		SvgData: "<svg></svg>",
		Width:   50,
		Height:  50,
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.events == nil {
		t.Fatal("expected events")
	}
	layout.Shape.events.OnClick(&layout, &Event{
		MouseButton: MouseLeft,
	}, w)
	if !clicked {
		t.Fatal("click not handled")
	}
}

func TestSvgColor(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 64, height: 64})
	v := Svg(SvgCfg{
		ID:      "svg1",
		SvgData: "<svg></svg>",
		Width:   50,
		Height:  50,
		Color:   Color{255, 0, 0, 255, true},
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.Color != (Color{255, 0, 0, 255, true}) {
		t.Fatalf("expected red, got %+v", layout.Shape.Color)
	}
}

func TestSvgCaching(t *testing.T) {
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 64, height: 64})
	// First load.
	cached1, err := w.LoadSvg("<svg></svg>", 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Second load — should return cached.
	cached2, err := w.LoadSvg("<svg></svg>", 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if cached1 != cached2 {
		t.Fatal("expected same cached pointer")
	}
}
