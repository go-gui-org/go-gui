//go:build js && wasm

package web

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// svgAffine must map every vertex where transformTris does, for each
// combination of batch transform, rotation, scale and offset.
func TestSvgAffineMatchesTransformTris(t *testing.T) {
	t.Parallel()
	tris := []float32{0, 0, 10, 0, 0, 10, -3.5, 7, 12, -4, 5.25, 9}
	base := gui.RenderCmd{Triangles: tris, X: 40, Y: -12, Scale: 1.75}
	xform := func(r *gui.RenderCmd) {
		r.HasXform = true
		r.ScaleX, r.ScaleY = 1.3, 0.6
		r.XformXY, r.XformYX = 0.2, -0.45
		r.TransX, r.TransY = 5, -8
	}
	rot := func(r *gui.RenderCmd) {
		r.RotAngle, r.RotCX, r.RotCY = 37, 4, 6
	}
	cases := map[string]func(*gui.RenderCmd){
		"plain":    func(*gui.RenderCmd) {},
		"xform":    xform,
		"rotation": rot,
		"both":     func(r *gui.RenderCmd) { xform(r); rot(r) },
	}
	for name, set := range cases {
		r := base
		set(&r)
		want := transformTris(nil, &r)
		a, b, c, d, e, f := svgAffine(&r)
		for i := 0; i < len(tris); i += 2 {
			x, y := float64(tris[i]), float64(tris[i+1])
			gx, gy := a*x+c*y+e, b*x+d*y+f
			if math.Abs(gx-float64(want[i])) > 1e-3 ||
				math.Abs(gy-float64(want[i+1])) > 1e-3 {
				t.Errorf("%s: vertex %d = (%.4f, %.4f), want (%.4f, %.4f)",
					name, i/2, gx, gy, want[i], want[i+1])
			}
		}
	}
}
