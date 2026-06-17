package gui

import (
	"math"
	"testing"
)

func makeScrollLayout(idScroll uint32, width, height float32, contentW, contentH float32) (*Layout, *Window) {
	w := &Window{}
	child := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     contentW,
			Height:    contentH,
		},
	}
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			IDScroll:  idScroll,
			Width:     width,
			Height:    height,
			Axis:      AxisTopToBottom,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{layout},
	}
	return &w.layout.Children[0], w
}

func TestScrollVerticalClampsWithinBounds(t *testing.T) {
	layout, w := makeScrollLayout(1, 100, 100, 100, 300)
	guiTheme.ScrollMultiplier = 1

	ok := scrollVertical(layout, -50, w)
	if !ok {
		t.Fatal("expected true")
	}
	sy := w.scrollY()
	v, _ := sy.Get(uint32(1))
	if v != -50 {
		t.Errorf("expected -50, got %v", v)
	}

	scrollVertical(layout, -500, w)
	v, _ = sy.Get(uint32(1))
	if v != -200 {
		t.Errorf("expected -200, got %v", v)
	}
}

func TestScrollHorizontalClampsWithinBounds(t *testing.T) {
	w := &Window{}
	guiTheme.ScrollMultiplier = 1
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 50},
	}
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			IDScroll:  2,
			Width:     100,
			Height:    50,
			Axis:      AxisLeftToRight,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{layout},
	}

	ok := scrollHorizontal(&w.layout.Children[0], -50, w)
	if !ok {
		t.Fatal("expected true")
	}
	sx := w.scrollX()
	v, _ := sx.Get(uint32(2))
	if v != -50 {
		t.Errorf("expected -50, got %v", v)
	}

	scrollHorizontal(&w.layout.Children[0], -500, w)
	v, _ = sx.Get(uint32(2))
	if v != -300 {
		t.Errorf("expected -300, got %v", v)
	}
}

func TestScrollNoIDScroll(t *testing.T) {
	t.Parallel()
	t.Run("vertical", func(t *testing.T) {
		t.Parallel()
		layout := &Layout{Shape: &Shape{IDScroll: 0}}
		if scrollVertical(layout, -10, &Window{}) {
			t.Error("should return false with IDScroll=0")
		}
	})
	t.Run("horizontal", func(t *testing.T) {
		t.Parallel()
		layout := &Layout{Shape: &Shape{IDScroll: 0}}
		if scrollHorizontal(layout, -10, &Window{}) {
			t.Error("should return false with IDScroll=0")
		}
	})
}

func TestScrollToView(t *testing.T) {
	t.Parallel()
	w := &Window{}
	target := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			ID:        "target",
			Y:         150,
			Height:    20,
		},
	}
	filler := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Height:    300,
		},
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			IDScroll:  5,
			Y:         10,
			Height:    100,
			Padding:   Padding{Top: 5},
			Axis:      AxisTopToBottom,
		},
		Children: []Layout{target, filler},
	}
	scroll.Children[0].Parent = &scroll
	scroll.Children[1].Parent = &scroll
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}
	w.layout.Children[0].Parent = &w.layout

	w.ScrollToView("target")
	sy := w.scrollY()
	v, ok := sy.Get(uint32(5))
	if !ok {
		t.Fatal("scroll offset not set")
	}
	if v != -135 {
		t.Errorf("expected -135, got %v", v)
	}
}

func TestScrollToViewNotFound(t *testing.T) {
	t.Parallel()
	w := &Window{}
	w.layout = Layout{Shape: &Shape{shapeType: shapeRectangle}}
	w.ScrollToView("nonexistent") // should not panic
}

func TestScrollByAndTo(t *testing.T) {
	t.Parallel()
	t.Run("horizontal", func(t *testing.T) {
		t.Parallel()
		w := &Window{}
		w.ScrollHorizontalBy(10, -30)
		sx := w.scrollX()
		v, _ := sx.Get(uint32(10))
		if v != -30 {
			t.Errorf("expected -30, got %v", v)
		}
		w.ScrollHorizontalTo(10, -100)
		v, _ = sx.Get(uint32(10))
		if v != -100 {
			t.Errorf("expected -100, got %v", v)
		}
	})
	t.Run("vertical", func(t *testing.T) {
		t.Parallel()
		w := &Window{}
		w.ScrollVerticalBy(10, -20)
		sy := w.scrollY()
		v, _ := sy.Get(uint32(10))
		if v != -20 {
			t.Errorf("expected -20, got %v", v)
		}
		w.ScrollVerticalTo(10, -50)
		v, _ = sy.Get(uint32(10))
		if v != -50 {
			t.Errorf("expected -50, got %v", v)
		}
	})
}

func TestScrollToPctAndPct(t *testing.T) {
	t.Run("vertical", func(t *testing.T) {
		_, w := makeScrollLayout(3, 100, 100, 100, 300)
		w.ScrollVerticalToPct(3, 0.5)
		sy := w.scrollY()
		v, _ := sy.Get(uint32(3))
		if v != -100 {
			t.Errorf("expected -100, got %v", v)
		}
		pct := w.ScrollVerticalPct(3)
		if math.Abs(float64(pct-0.5)) > 0.01 {
			t.Errorf("expected ~0.5, got %v", pct)
		}
	})
	t.Run("horizontal", func(t *testing.T) {
		w := &Window{}
		child := Layout{
			Shape: &Shape{shapeType: shapeRectangle,
				Width: 400, Height: 50},
		}
		layout := Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				IDScroll:  4,
				Width:     100,
				Height:    50,
				Axis:      AxisLeftToRight,
			},
			Children: []Layout{child},
		}
		w.layout = Layout{
			Shape:    &Shape{shapeType: shapeRectangle},
			Children: []Layout{layout},
		}

		w.ScrollHorizontalToPct(4, 1.0)
		sx := w.scrollX()
		v, _ := sx.Get(uint32(4))
		if v != -300 {
			t.Errorf("expected -300, got %v", v)
		}
		pct := w.ScrollHorizontalPct(4)
		if math.Abs(float64(pct-1.0)) > 0.01 {
			t.Errorf("expected ~1.0, got %v", pct)
		}
	})
}

func TestScrollPctNoScrollNeeded(t *testing.T) {
	t.Parallel()
	_, w := makeScrollLayout(6, 200, 200, 100, 100)
	pct := w.ScrollVerticalPct(6)
	if pct != 0 {
		t.Errorf("expected 0, got %v", pct)
	}
	pct = w.ScrollHorizontalPct(6)
	if pct != 0 {
		t.Errorf("expected 0, got %v", pct)
	}
}

func TestScrollVerticalFiresOnScroll(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	fired := false
	layout, w := makeScrollLayout(7, 100, 100, 100, 300)
	layout.Shape.events = &eventHandlers{
		OnScroll: func(_ *Layout, _ *Window) { fired = true },
	}
	scrollVertical(layout, -10, w)
	if !fired {
		t.Error("OnScroll callback not fired")
	}
}

func TestScrollReturnsFalseAtBoundary(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	t.Run("vertical", func(t *testing.T) {
		layout, w := makeScrollLayout(10, 100, 100, 100, 300)
		scrollVertical(layout, -500, w)
		sy := w.scrollY()
		v, _ := sy.Get(uint32(10))
		if v != -200 {
			t.Fatalf("expected -200, got %v", v)
		}
		if scrollVertical(layout, -10, w) {
			t.Error("expected false at max boundary")
		}
		scrollVertical(layout, 500, w)
		v, _ = sy.Get(uint32(10))
		if v != 0 {
			t.Fatalf("expected 0, got %v", v)
		}
		if scrollVertical(layout, 10, w) {
			t.Error("expected false at min boundary")
		}
	})
	t.Run("horizontal", func(t *testing.T) {
		w := &Window{}
		child := Layout{
			Shape: &Shape{shapeType: shapeRectangle,
				Width: 400, Height: 50},
		}
		layout := Layout{
			Shape: &Shape{
				shapeType: shapeRectangle,
				IDScroll:  11,
				Width:     100,
				Height:    50,
				Axis:      AxisLeftToRight,
			},
			Children: []Layout{child},
		}
		w.layout = Layout{
			Shape:    &Shape{shapeType: shapeRectangle},
			Children: []Layout{layout},
		}
		ly := &w.layout.Children[0]

		scrollHorizontal(ly, -500, w)
		sx := w.scrollX()
		v, _ := sx.Get(uint32(11))
		if v != -300 {
			t.Fatalf("expected -300, got %v", v)
		}
		if scrollHorizontal(ly, -10, w) {
			t.Error("expected false at max boundary")
		}
		scrollHorizontal(ly, 500, w)
		v, _ = sx.Get(uint32(11))
		if v != 0 {
			t.Fatalf("expected 0, got %v", v)
		}
		if scrollHorizontal(ly, 10, w) {
			t.Error("expected false at min boundary")
		}
	})
}

func TestScrollMode(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	t.Run("vertical_only_blocks_horizontal", func(t *testing.T) {
		w := &Window{}
		child := Layout{
			Shape: &Shape{shapeType: shapeRectangle,
				Width: 400, Height: 50},
		}
		layout := Layout{
			Shape: &Shape{
				shapeType:  shapeRectangle,
				IDScroll:   12,
				Width:      100,
				Height:     50,
				Axis:       AxisLeftToRight,
				ScrollMode: ScrollVerticalOnly,
			},
			Children: []Layout{child},
		}
		w.layout = Layout{
			Shape:    &Shape{shapeType: shapeRectangle},
			Children: []Layout{layout},
		}
		if scrollHorizontal(&w.layout.Children[0], -10, w) {
			t.Error("expected false for ScrollVerticalOnly")
		}
	})
	t.Run("horizontal_only_blocks_vertical", func(t *testing.T) {
		layout, w := makeScrollLayout(13, 100, 100, 100, 300)
		layout.Shape.ScrollMode = ScrollHorizontalOnly
		if scrollVertical(layout, -10, w) {
			t.Error("expected false for ScrollHorizontalOnly")
		}
	})
}

func TestScrollToViewClampsOffset(t *testing.T) {
	t.Parallel()
	w := &Window{}
	target := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			ID:        "above",
			Y:         5,
			Height:    20,
		},
	}
	child := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Height:    300,
		},
	}
	scroll := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			IDScroll:  14,
			Y:         10,
			Height:    100,
			Padding:   Padding{Top: 5},
			Axis:      AxisTopToBottom,
		},
		Children: []Layout{target, child},
	}
	scroll.Children[0].Parent = &scroll
	scroll.Children[1].Parent = &scroll
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{scroll},
	}
	w.layout.Children[0].Parent = &w.layout

	w.ScrollToView("above")
	sy := w.scrollY()
	v, ok := sy.Get(uint32(14))
	if !ok {
		t.Fatal("scroll offset not set")
	}
	if v > 0 {
		t.Errorf("expected clamped <=0, got %v", v)
	}
}

func TestScrollVerticalByAndToWithClampAndOnScroll(t *testing.T) {
	layout, w := makeScrollLayout(15, 100, 100, 100, 300)
	w.layout.Children[0].Parent = &w.layout

	fired := 0
	layout.Shape.events = &eventHandlers{
		OnScroll: func(_ *Layout, _ *Window) { fired++ },
	}

	w.ScrollVerticalBy(15, -50)
	sy := w.scrollY()
	v, _ := sy.Get(uint32(15))
	if v != -50 {
		t.Errorf("expected -50, got %v", v)
	}
	if fired != 1 {
		t.Errorf("expected OnScroll fired 1, got %d", fired)
	}

	w.ScrollVerticalTo(15, -999)
	v, _ = sy.Get(uint32(15))
	if v != -200 {
		t.Errorf("expected -200, got %v", v)
	}
	if fired != 2 {
		t.Errorf("expected OnScroll fired 2, got %d", fired)
	}
}

func TestScrollHorizontalByAndToWithClampAndOnScroll(t *testing.T) {
	w := &Window{}
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 50},
	}
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			IDScroll:  16,
			Width:     100,
			Height:    50,
			Axis:      AxisLeftToRight,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{layout},
	}
	w.layout.Children[0].Parent = &w.layout

	fired := 0
	w.layout.Children[0].Shape.events = &eventHandlers{
		OnScroll: func(_ *Layout, _ *Window) { fired++ },
	}

	w.ScrollHorizontalBy(16, -50)
	sx := w.scrollX()
	v, _ := sx.Get(uint32(16))
	if v != -50 {
		t.Errorf("expected -50, got %v", v)
	}
	if fired != 1 {
		t.Errorf("expected OnScroll fired 1, got %d", fired)
	}

	w.ScrollHorizontalTo(16, -999)
	v, _ = sx.Get(uint32(16))
	if v != -300 {
		t.Errorf("expected -300, got %v", v)
	}
	if fired != 2 {
		t.Errorf("expected OnScroll fired 2, got %d", fired)
	}
}
