package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestComputeTextPathPlacements_NilTextSys(t *testing.T) {
	layout, placements, err := ComputeTextPathPlacements(
		&RenderCmd{TextPath: &TextPathData{}, TextStylePtr: &TextStyle{}},
		nil, nil, func(ts TextStyle) glyph.TextConfig { return glyph.TextConfig{} },
	)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(placements) != 0 {
		t.Errorf("expected 0 placements, got %d", len(placements))
	}
	_ = layout
}

func TestComputeTextPathPlacements_NilTextPath(t *testing.T) {
	layout, placements, err := ComputeTextPathPlacements(
		&RenderCmd{TextStylePtr: &TextStyle{}},
		nil, nil, func(ts TextStyle) glyph.TextConfig { return glyph.TextConfig{} },
	)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(placements) != 0 {
		t.Errorf("expected 0 placements, got %d", len(placements))
	}
	_ = layout
}

func TestComputeTextPathPlacements_NilTextStylePtr(t *testing.T) {
	layout, placements, err := ComputeTextPathPlacements(
		&RenderCmd{TextPath: &TextPathData{}},
		nil, nil, func(ts TextStyle) glyph.TextConfig { return glyph.TextConfig{} },
	)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(placements) != 0 {
		t.Errorf("expected 0 placements, got %d", len(placements))
	}
	_ = layout
}

func TestGradientBorderRects_NormalStops(t *testing.T) {
	r := &RenderCmd{
		X: 10, Y: 20, W: 100, H: 50, Thickness: 3,
		Gradient: &GradientDef{
			Stops: []GradientStop{
				{Color: Red, Pos: 0},
				{Color: Green, Pos: 0.33},
				{Color: Blue, Pos: 0.66},
				{Color: White, Pos: 1},
			},
		},
	}
	rects := GradientBorderRects(r)
	// Top edge.
	if rects[0].X != 10 || rects[0].Y != 20 || rects[0].W != 100 || rects[0].H != 3 {
		t.Errorf("top rect = (%v,%v,%v,%v), want (10,20,100,3)",
			rects[0].X, rects[0].Y, rects[0].W, rects[0].H)
	}
	// Bottom edge.
	wantBY := float32(20 + 50 - 3) // 67
	if rects[1].Y != wantBY {
		t.Errorf("bottom rect Y = %v, want %v", rects[1].Y, wantBY)
	}
	// Left edge: width = thickness.
	if rects[2].W != 3 || rects[2].H != 50 {
		t.Errorf("left rect size = (%v,%v), want (3,50)", rects[2].W, rects[2].H)
	}
	// Right edge.
	wantRX := float32(10 + 100 - 3) // 107
	if rects[3].X != wantRX {
		t.Errorf("right rect X = %v, want %v", rects[3].X, wantRX)
	}
	// Colors sampled at 0, 0.25, 0.5, 0.75.
	if rects[0].Color != Red {
		t.Errorf("top color = %v, want Red", rects[0].Color)
	}
}

func TestGradientBorderRects_EmptyStops_ReturnsZero(t *testing.T) {
	r := &RenderCmd{
		X: 10, Y: 20, W: 100, H: 50, Thickness: 3,
		Gradient: &GradientDef{},
	}
	rects := GradientBorderRects(r)
	if rects != [4]GradientBorderRect{} {
		t.Errorf("empty stops should return zero rects, got %v", rects)
	}
}

func TestGradientBorderRects_ZeroThickness(t *testing.T) {
	r := &RenderCmd{
		X: 0, Y: 0, W: 100, H: 50, Thickness: 0,
		Gradient: &GradientDef{
			Stops: []GradientStop{
				{Color: Black, Pos: 0},
				{Color: Black, Pos: 1},
			},
		},
	}
	rects := GradientBorderRects(r)
	// All edges have zero thickness dimension.
	if rects[0].H != 0 {
		t.Errorf("top edge H = %v, want 0 (zero thickness)", rects[0].H)
	}
	if rects[2].W != 0 {
		t.Errorf("left edge W = %v, want 0 (zero thickness)", rects[2].W)
	}
}
