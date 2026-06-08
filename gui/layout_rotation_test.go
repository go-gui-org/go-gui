package gui

import "testing"

func TestRotationSwapQuarterTurn1(t *testing.T) {
	shape := &Shape{
		Width: 100, Height: 50,
		MinWidth: 80, MinHeight: 30,
		MaxWidth: 200, MaxHeight: 100,
		QuarterTurns: 1,
	}
	layout := &Layout{Shape: shape}
	layoutRotationSwap(layout)

	if shape.Width != 50 || shape.Height != 100 {
		t.Errorf("W/H = %f/%f, want 50/100", shape.Width, shape.Height)
	}
	if shape.MinWidth != 30 || shape.MinHeight != 80 {
		t.Errorf("MinW/MinH = %f/%f, want 30/80",
			shape.MinWidth, shape.MinHeight)
	}
	if shape.MaxWidth != 100 || shape.MaxHeight != 200 {
		t.Errorf("MaxW/MaxH = %f/%f, want 100/200",
			shape.MaxWidth, shape.MaxHeight)
	}
}

func TestRotationSwapQuarterTurn3(t *testing.T) {
	shape := &Shape{
		Width: 100, Height: 50,
		QuarterTurns: 3,
	}
	layout := &Layout{Shape: shape}
	layoutRotationSwap(layout)

	if shape.Width != 50 || shape.Height != 100 {
		t.Errorf("W/H = %f/%f, want 50/100", shape.Width, shape.Height)
	}
}

func TestRotationSwapQuarterTurn0NoOp(t *testing.T) {
	shape := &Shape{
		Width: 100, Height: 50,
		QuarterTurns: 0,
	}
	layout := &Layout{Shape: shape}
	layoutRotationSwap(layout)

	if shape.Width != 100 || shape.Height != 50 {
		t.Errorf("W/H = %f/%f, want 100/50 (no swap)",
			shape.Width, shape.Height)
	}
}

func TestRotationSwapQuarterTurn2NoOp(t *testing.T) {
	shape := &Shape{
		Width: 100, Height: 50,
		QuarterTurns: 2,
	}
	layout := &Layout{Shape: shape}
	layoutRotationSwap(layout)

	if shape.Width != 100 || shape.Height != 50 {
		t.Errorf("W/H = %f/%f, want 100/50 (no swap for turn=2)",
			shape.Width, shape.Height)
	}
}

func TestRotationSwapRecursive(t *testing.T) {
	child := &Shape{
		Width: 60, Height: 30,
		QuarterTurns: 1,
	}
	parent := &Shape{
		Width: 200, Height: 100,
		QuarterTurns: 0,
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}
	layoutParents(layout, nil)
	layoutRotationSwap(layout)

	// Child should be swapped.
	if child.Width != 30 || child.Height != 60 {
		t.Errorf("child W/H = %f/%f, want 30/60",
			child.Width, child.Height)
	}
	// Parent (turn=0) should not be swapped.
	if parent.Width == 100 && parent.Height == 200 {
		t.Error("parent should not be swapped for turn=0")
	}
}

func TestReaccumulateAncestorsFitWidth(t *testing.T) {
	child := &Shape{
		Width: 60, Height: 30,
	}
	parent := &Shape{
		Width: 100, Height: 50,
		Axis:   AxisLeftToRight,
		Sizing: Sizing{Width: SizingFit, Height: SizingFixed},
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}
	layoutParents(layout, nil)

	// Change child width to trigger reaccumulation.
	child.Width = 80
	reaccumulateAncestors(layout.Children[0].Parent)

	if parent.Width != 80 {
		t.Errorf("parent Width = %f, want 80", parent.Width)
	}
}

func TestReaccumulateAncestorsStopsAtFixed(t *testing.T) {
	child := &Shape{Width: 60, Height: 30}
	parent := &Shape{
		Width: 100, Height: 50,
		Sizing: Sizing{Width: SizingFixed, Height: SizingFixed},
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}
	layoutParents(layout, nil)

	child.Width = 80
	reaccumulateAncestors(layout.Children[0].Parent)

	// Fixed parent should not change.
	if parent.Width != 100 {
		t.Errorf("parent Width = %f, want 100 (Fixed)", parent.Width)
	}
}

func TestReaccumulateAncestorsStopsWhenNoChange(t *testing.T) {
	child := &Shape{Width: 60, Height: 30}
	parent := &Shape{
		Width: 60, Height: 50,
		Axis:   AxisLeftToRight,
		Sizing: Sizing{Width: SizingFit, Height: SizingFixed},
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}
	layoutParents(layout, nil)

	// No actual change — parent already matches child.
	reaccumulateAncestors(layout.Children[0].Parent)

	if parent.Width != 60 {
		t.Errorf("parent Width = %f, want 60 (unchanged)", parent.Width)
	}
}

func TestRecomputeFitWidthLTR(t *testing.T) {
	c1 := &Shape{Width: 30, Height: 10, shapeType: shapeRectangle}
	c2 := &Shape{Width: 50, Height: 10, shapeType: shapeRectangle}
	parent := &Shape{
		Width: 0, Height: 50,
		Axis:    AxisLeftToRight,
		Sizing:  Sizing{Width: SizingFit},
		Spacing: 5,
	}
	layout := &Layout{
		Shape: parent,
		Children: []Layout{
			{Shape: c1},
			{Shape: c2},
		},
	}

	got := recomputeFitWidth(layout)
	// sum of children + spacing(1 gap) = 30 + 50 + 5 = 85
	want := float32(85)
	if !f32AreClose(got, want) {
		t.Errorf("recomputeFitWidth = %f, want %f", got, want)
	}
}

func TestRecomputeFitWidthTTB(t *testing.T) {
	c1 := &Shape{Width: 30, Height: 10}
	c2 := &Shape{Width: 50, Height: 10}
	parent := &Shape{
		Axis:   AxisTopToBottom,
		Sizing: Sizing{Width: SizingFit},
	}
	layout := &Layout{
		Shape: parent,
		Children: []Layout{
			{Shape: c1},
			{Shape: c2},
		},
	}

	got := recomputeFitWidth(layout)
	// max of children = 50
	if got != 50 {
		t.Errorf("recomputeFitWidth = %f, want 50", got)
	}
}

func TestRecomputeFitWidthClampsMinMax(t *testing.T) {
	child := &Shape{Width: 10, Height: 10}
	parent := &Shape{
		Axis:     AxisLeftToRight,
		Sizing:   Sizing{Width: SizingFit},
		MinWidth: 50,
		MaxWidth: 80,
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}

	got := recomputeFitWidth(layout)
	if got < 50 {
		t.Errorf("recomputeFitWidth = %f, want >= 50 (MinWidth)", got)
	}

	// Now test max clamp.
	child.Width = 200
	got = recomputeFitWidth(layout)
	if got > 80 {
		t.Errorf("recomputeFitWidth = %f, want <= 80 (MaxWidth)", got)
	}
}

func TestRecomputeFitHeightLTR(t *testing.T) {
	c1 := &Shape{Width: 30, Height: 10}
	c2 := &Shape{Width: 50, Height: 25}
	parent := &Shape{
		Axis:   AxisLeftToRight,
		Sizing: Sizing{Height: SizingFit},
	}
	layout := &Layout{
		Shape: parent,
		Children: []Layout{
			{Shape: c1},
			{Shape: c2},
		},
	}

	got := recomputeFitHeight(layout)
	// max of children = 25
	if got != 25 {
		t.Errorf("recomputeFitHeight = %f, want 25", got)
	}
}

func TestRecomputeFitHeightTTB(t *testing.T) {
	c1 := &Shape{Width: 30, Height: 10, shapeType: shapeRectangle}
	c2 := &Shape{Width: 50, Height: 25, shapeType: shapeRectangle}
	parent := &Shape{
		Axis:    AxisTopToBottom,
		Sizing:  Sizing{Height: SizingFit},
		Spacing: 5,
	}
	layout := &Layout{
		Shape: parent,
		Children: []Layout{
			{Shape: c1},
			{Shape: c2},
		},
	}

	got := recomputeFitHeight(layout)
	// sum + spacing = 10 + 25 + 5 = 40
	want := float32(40)
	if !f32AreClose(got, want) {
		t.Errorf("recomputeFitHeight = %f, want %f", got, want)
	}
}

func TestRecomputeFitHeightClampsMinMax(t *testing.T) {
	child := &Shape{Width: 10, Height: 5}
	parent := &Shape{
		Axis:      AxisTopToBottom,
		Sizing:    Sizing{Height: SizingFit},
		MinHeight: 20,
		MaxHeight: 50,
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}

	got := recomputeFitHeight(layout)
	if got < 20 {
		t.Errorf("recomputeFitHeight = %f, want >= 20 (MinHeight)", got)
	}

	child.Height = 200
	got = recomputeFitHeight(layout)
	if got > 50 {
		t.Errorf("recomputeFitHeight = %f, want <= 50 (MaxHeight)", got)
	}
}

func TestRecomputeFitWidthSkipsOverDraw(t *testing.T) {
	c1 := &Shape{Width: 30, Height: 10, shapeType: shapeRectangle}
	c2 := &Shape{Width: 50, Height: 10, shapeType: shapeRectangle,
		OverDraw: true}
	parent := &Shape{
		Axis:   AxisLeftToRight,
		Sizing: Sizing{Width: SizingFit},
	}
	layout := &Layout{
		Shape: parent,
		Children: []Layout{
			{Shape: c1},
			{Shape: c2},
		},
	}

	got := recomputeFitWidth(layout)
	// Only c1 contributes (c2 is OverDraw).
	if got != 30 {
		t.Errorf("recomputeFitWidth = %f, want 30 (skip OverDraw)", got)
	}
}
