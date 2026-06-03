package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// curvyPath returns a VectorGraphic whose single path is a deeply
// curving cubic Bezier — a high-tolerance floor flattens it with
// fewer vertices than the default 0.15 floor.
func curvyPath() *VectorGraphic {
	return &VectorGraphic{
		Paths: []VectorPath{
			{
				FillColor: gui.SvgColor{R: 255, A: 255},
				Opacity:   1,
				Segments: []PathSegment{
					{Cmd: CmdMoveTo, Points: []float32{0, 0}},
					{Cmd: CmdCubicTo, Points: []float32{
						50, 200, 100, -200, 200, 0}},
					{Cmd: CmdClose},
				},
			},
		},
	}
}

func vertexCount(out []gui.TessellatedPath) int {
	n := 0
	for i := range out {
		n += len(out[i].Triangles) / 2
	}
	return n
}

func TestTessellateFlatnessToleranceRaisesFloor(t *testing.T) {
	defaultVG := curvyPath()
	coarseVG := curvyPath()
	coarseVG.FlatnessTolerance = 8

	defaultOut := defaultVG.tessellatePaths(defaultVG.Paths, 1)
	coarseOut := coarseVG.tessellatePaths(coarseVG.Paths, 1)

	dv := vertexCount(defaultOut)
	cv := vertexCount(coarseOut)
	if cv >= dv {
		t.Fatalf("higher FlatnessTolerance should reduce vertex count; "+
			"default=%d coarse=%d", dv, cv)
	}
	if cv == 0 {
		t.Fatalf("coarse tessellation should still produce geometry")
	}
}
