package gui

import "testing"

func TestRotatedBoxDimensionSwap(t *testing.T) {
	v := RotatedBox(RotatedBoxCfg{
		QuarterTurns: 1,
		Content: Row(ContainerCfg{
			Width:      50,
			Height:     16,
			Sizing:     FixedFixed,
			SizeBorder: NoBorder,
			Padding:    NoPadding,
		}),
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	layoutParents(&layout, nil)
	layoutWidths(&layout)
	layoutHeights(&layout)
	layoutRotationSwap(&layout)
	if layout.Shape.Width != 16 {
		t.Errorf("width = %f, want 16", layout.Shape.Width)
	}
	if layout.Shape.Height != 50 {
		t.Errorf("height = %f, want 50", layout.Shape.Height)
	}
}

func TestRotatedBoxNoOpForZeroTurns(t *testing.T) {
	inner := Row(ContainerCfg{
		Width:      40,
		Height:     20,
		Sizing:     FixedFixed,
		SizeBorder: NoBorder,
		Padding:    NoPadding,
	})
	v := RotatedBox(RotatedBoxCfg{
		QuarterTurns: 0,
		Content:      inner,
	})
	// Should return child directly (passthrough).
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Width != 40 {
		t.Errorf("width = %f, want 40", layout.Shape.Width)
	}
	if layout.Shape.Height != 20 {
		t.Errorf("height = %f, want 20", layout.Shape.Height)
	}
}

func TestRotatedBox180SameDimensions(t *testing.T) {
	v := RotatedBox(RotatedBoxCfg{
		QuarterTurns: 2,
		Content: Row(ContainerCfg{
			Width:      50,
			Height:     16,
			Sizing:     FixedFixed,
			SizeBorder: NoBorder,
			Padding:    NoPadding,
		}),
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	layoutParents(&layout, nil)
	layoutWidths(&layout)
	layoutHeights(&layout)
	layoutRotationSwap(&layout)
	// 180° does not swap dimensions.
	if layout.Shape.Width != 50 {
		t.Errorf("width = %f, want 50", layout.Shape.Width)
	}
	if layout.Shape.Height != 16 {
		t.Errorf("height = %f, want 16", layout.Shape.Height)
	}
}

func TestRotatedBoxRenderBrackets(t *testing.T) {
	v := RotatedBox(RotatedBoxCfg{
		QuarterTurns: 1,
		Content: Row(ContainerCfg{
			Width:      30,
			Height:     10,
			Sizing:     FixedFixed,
			SizeBorder: NoBorder,
			Padding:    NoPadding,
		}),
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	layoutParents(&layout, nil)
	layoutPipeline(&layout, w)
	clip := drawClip{Width: 800, Height: 600}
	renderLayout(&layout, ColorTransparent, clip, w)
	cmds := w.Renderers()
	var foundBegin, foundEnd bool
	var angle float32
	for _, cmd := range cmds {
		if cmd.Kind == RenderRotateBegin {
			foundBegin = true
			angle = cmd.RotAngle
		}
		if cmd.Kind == RenderRotateEnd {
			foundEnd = true
		}
	}
	if !foundBegin {
		t.Error("RenderRotateBegin not found")
	}
	if !foundEnd {
		t.Error("RenderRotateEnd not found")
	}
	if angle != 90 {
		t.Errorf("angle = %f, want 90", angle)
	}
}

func TestRotatedBoxParentReaccumulation(t *testing.T) {
	v := Column(ContainerCfg{
		Sizing:     FitFit,
		SizeBorder: NoBorder,
		Padding:    NoPadding,
		Spacing:    SomeF(0),
		Content: []View{
			RotatedBox(RotatedBoxCfg{
				QuarterTurns: 1,
				Content: Row(ContainerCfg{
					Width:      50,
					Height:     16,
					Sizing:     FixedFixed,
					SizeBorder: NoBorder,
					Padding:    NoPadding,
				}),
			}),
		},
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	layoutParents(&layout, nil)
	layoutWidths(&layout)
	layoutHeights(&layout)
	layoutRotationSwap(&layout)
	// Column should reflect swapped child: 16 wide, 50 tall.
	if layout.Shape.Height != 50 {
		t.Errorf("column height = %f, want 50", layout.Shape.Height)
	}
	if layout.Shape.Width != 16 {
		t.Errorf("column width = %f, want 16", layout.Shape.Width)
	}
}

func TestRotatedBoxNegativeTurns(t *testing.T) {
	v := RotatedBox(RotatedBoxCfg{
		QuarterTurns: -1,
		Content: Row(ContainerCfg{
			Width:      50,
			Height:     16,
			Sizing:     FixedFixed,
			SizeBorder: NoBorder,
			Padding:    NoPadding,
		}),
	})
	w := &Window{}
	layout := GenerateViewLayout(v, w)
	// -1 normalizes to 3 (270° CW).
	if layout.Shape.QuarterTurns != 3 {
		t.Errorf("QuarterTurns = %d, want 3",
			layout.Shape.QuarterTurns)
	}
	layoutParents(&layout, nil)
	layoutWidths(&layout)
	layoutHeights(&layout)
	layoutRotationSwap(&layout)
	// 270° swaps dimensions.
	if layout.Shape.Width != 16 {
		t.Errorf("width = %f, want 16", layout.Shape.Width)
	}
	if layout.Shape.Height != 50 {
		t.Errorf("height = %f, want 50", layout.Shape.Height)
	}
}
