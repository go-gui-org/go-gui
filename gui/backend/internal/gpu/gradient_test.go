package gpu

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestPackGradientUniformsLinear(t *testing.T) {
	gdef := &gui.GradientDef{
		Type:      gui.GradientLinear,
		Direction: gui.GradientToBottom,
	}
	stops := []gui.GradientStop{
		{Color: gui.Red, Pos: 0},
		{Color: gui.Blue, Pos: 1},
	}
	tm := PackGradientUniforms(gdef, stops, 100, 50)

	// tm[3*4+3] = stop count = 2
	if tm[3*4+3] != 2 {
		t.Errorf("stop count = %v, want 2", tm[3*4+3])
	}
	// tm[3*4+2] = 0.0 (linear)
	if tm[3*4+2] != 0 {
		t.Errorf("grad type = %v, want 0 (linear)", tm[3*4+2])
	}
	// Direction for GradientToBottom: (0, 1)
	// tm[2*4+2] = dx, tm[2*4+3] = dy
	if tm[2*4+2] < -0.01 || tm[2*4+2] > 0.01 {
		t.Errorf("dx = %v, want ~0", tm[2*4+2])
	}
	if tm[2*4+3] != 1 {
		t.Errorf("dy = %v, want 1", tm[2*4+3])
	}
	// Half-width, half-height in col 3
	if tm[3*4+0] != 50 || tm[3*4+1] != 25 {
		t.Errorf("half-size = (%v, %v), want (50, 25)",
			tm[3*4+0], tm[3*4+1])
	}
}

func TestPackGradientUniformsRadial(t *testing.T) {
	gdef := &gui.GradientDef{
		Type: gui.GradientRadial,
	}
	stops := []gui.GradientStop{
		{Color: gui.White, Pos: 0},
	}
	tm := PackGradientUniforms(gdef, stops, 100, 80)

	// tm[3*4+2] = 1.0 (radial)
	if tm[3*4+2] != 1.0 {
		t.Errorf("grad type = %v, want 1 (radial)", tm[3*4+2])
	}
	// Radial radius = max(50, 40) = 50
	if tm[2*4+3] != 50 {
		t.Errorf("radial radius = %v, want 50", tm[2*4+3])
	}
}

func TestPackGradientUniformsNilDef(t *testing.T) {
	stops := []gui.GradientStop{
		{Color: gui.Gray, Pos: 0.5},
	}
	tm := PackGradientUniforms(nil, stops, 100, 100)
	// Nil gdef defaults to linear.
	if tm[3*4+2] != 0 {
		t.Errorf("nil gdef should default to linear (0), got %v",
			tm[3*4+2])
	}
	// Packed stop in col0, row 0+1.
	if tm[0] <= 0 {
		t.Errorf("stop color not packed: tm[0]=%v", tm[0])
	}
}
