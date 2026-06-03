package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Phase E: pre-tess bbox per shape kind and transform-origin
// resolution into rotate animation centers.

func nearlyEq(a, b float32) bool {
	const eps = 1e-4
	return math.Abs(float64(a-b)) < eps
}

func TestPhaseE_BBox_Rect(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<rect x="10" y="20" width="30" height="40"/>
	</svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vg.Paths) != 1 {
		t.Fatalf("paths: %d", len(vg.Paths))
	}
	b := vg.Paths[0].Bbox
	if !b.Set {
		t.Fatal("bbox unset")
	}
	if b.MinX != 10 || b.MinY != 20 || b.MaxX != 40 || b.MaxY != 60 {
		t.Errorf("rect bbox: %+v", b)
	}
}

func TestPhaseE_BBox_Circle(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<circle cx="50" cy="50" r="20"/>
	</svg>`
	vg, _ := parseSvg(src)
	b := vg.Paths[0].Bbox
	if b.MinX != 30 || b.MinY != 30 || b.MaxX != 70 || b.MaxY != 70 {
		t.Errorf("circle bbox: %+v", b)
	}
}

func TestPhaseE_BBox_Ellipse(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<ellipse cx="50" cy="40" rx="20" ry="10"/>
	</svg>`
	vg, _ := parseSvg(src)
	b := vg.Paths[0].Bbox
	if b.MinX != 30 || b.MinY != 30 || b.MaxX != 70 || b.MaxY != 50 {
		t.Errorf("ellipse bbox: %+v", b)
	}
}

func TestPhaseE_BBox_Line(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<line x1="10" y1="80" x2="90" y2="20" stroke="black"/>
	</svg>`
	vg, _ := parseSvg(src)
	b := vg.Paths[0].Bbox
	if b.MinX != 10 || b.MinY != 20 || b.MaxX != 90 || b.MaxY != 80 {
		t.Errorf("line bbox: %+v", b)
	}
}

func TestPhaseE_BBox_Polygon(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<polygon points="10,10 90,20 50,80"/>
	</svg>`
	vg, _ := parseSvg(src)
	b := vg.Paths[0].Bbox
	if b.MinX != 10 || b.MinY != 10 || b.MaxX != 90 || b.MaxY != 80 {
		t.Errorf("polygon bbox: %+v", b)
	}
}

func TestPhaseE_BBox_Path(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<path d="M 5 10 L 95 10 L 50 90 Z"/>
	</svg>`
	vg, _ := parseSvg(src)
	b := vg.Paths[0].Bbox
	if b.MinX != 5 || b.MinY != 10 || b.MaxX != 95 || b.MaxY != 90 {
		t.Errorf("path bbox: %+v", b)
	}
}

func TestPhaseE_BBox_PathCubic_ControlPoly(t *testing.T) {
	// Cubic control points dominate the curve extents.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<path d="M 0 50 C 0 0, 100 0, 100 50"/>
	</svg>`
	vg, _ := parseSvg(src)
	b := vg.Paths[0].Bbox
	if b.MinX != 0 || b.MaxX != 100 {
		t.Errorf("cubic X bounds: %+v", b)
	}
	if b.MinY != 0 || b.MaxY != 50 {
		t.Errorf("cubic Y bounds: %+v", b)
	}
}

func TestPhaseE_TransformOrigin_Resolve(t *testing.T) {
	b := bbox{MinX: 10, MinY: 20, MaxX: 50, MaxY: 60, Set: true}

	cases := []struct {
		v            string
		wantX, wantY float32
	}{
		{"", 30, 40}, // default = center
		{"center", 30, 40},
		{"50% 50%", 30, 40},
		{"left top", 10, 20},
		{"top left", 10, 20}, // keyword swap
		{"right bottom", 50, 60},
		{"100% 100%", 50, 60},
		{"0% 0%", 10, 20},
		{"25% 75%", 20, 50},
		{"5px 15px", 5, 15},
		{"5 15", 5, 15},
		{"left", 10, 40},
		{"top", 30, 20},
	}
	for _, c := range cases {
		x, y := resolveTransformOrigin(c.v, b)
		if !nearlyEq(x, c.wantX) || !nearlyEq(y, c.wantY) {
			t.Errorf("%q -> (%v,%v) want (%v,%v)",
				c.v, x, y, c.wantX, c.wantY)
		}
	}
}

func TestPhaseE_TransformOrigin_UnsetBBox(t *testing.T) {
	x, y := resolveTransformOrigin("50% 50%", bbox{})
	if x != 0 || y != 0 {
		t.Errorf("unset bbox: (%v,%v)", x, y)
	}
}

func TestPhaseE_RotateAnim_DefaultsToBBoxCenter(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin {
				from { transform: rotate(0deg) }
				to   { transform: rotate(360deg) }
			}
			.r { animation: spin 1s linear }
		</style>
		<rect class="r" x="10" y="20" width="40" height="60"/>
	</svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var rot *gui.SvgAnimation
	for i := range vg.Animations {
		if vg.Animations[i].Kind == gui.SvgAnimRotate {
			rot = &vg.Animations[i]
			break
		}
	}
	if rot == nil {
		t.Fatal("no rotate animation compiled")
	}
	// CSS default = 50% 50% of bbox: (10+40/2, 20+60/2) = (30, 50).
	if !nearlyEq(rot.CenterX, 30) || !nearlyEq(rot.CenterY, 50) {
		t.Errorf("default center: (%v,%v) want (30,50)",
			rot.CenterX, rot.CenterY)
	}
}

func TestPhaseE_RotateAnim_TransformOriginKeyword(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin {
				from { transform: rotate(0deg) }
				to   { transform: rotate(360deg) }
			}
			.r {
				animation: spin 1s linear;
				transform-origin: top left;
			}
		</style>
		<rect class="r" x="10" y="20" width="40" height="60"/>
	</svg>`
	vg, _ := parseSvg(src)
	var rot *gui.SvgAnimation
	for i := range vg.Animations {
		if vg.Animations[i].Kind == gui.SvgAnimRotate {
			rot = &vg.Animations[i]
			break
		}
	}
	if rot == nil {
		t.Fatal("no rotate animation")
	}
	if !nearlyEq(rot.CenterX, 10) || !nearlyEq(rot.CenterY, 20) {
		t.Errorf("top-left origin: (%v,%v) want (10,20)",
			rot.CenterX, rot.CenterY)
	}
}

func TestPhaseE_RotateAnim_TransformOriginPercent(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<style>
			@keyframes spin { from {transform:rotate(0)} to {transform:rotate(360deg)} }
			.r { animation: spin 1s linear; transform-origin: 25% 75%; }
		</style>
		<rect class="r" x="0" y="0" width="40" height="40"/>
	</svg>`
	vg, _ := parseSvg(src)
	var rot *gui.SvgAnimation
	for i := range vg.Animations {
		if vg.Animations[i].Kind == gui.SvgAnimRotate {
			rot = &vg.Animations[i]
			break
		}
	}
	if rot == nil {
		t.Fatal("no rotate animation")
	}
	// 25% of 40 = 10; 75% of 40 = 30.
	if !nearlyEq(rot.CenterX, 10) || !nearlyEq(rot.CenterY, 30) {
		t.Errorf("percent origin: (%v,%v) want (10,30)",
			rot.CenterX, rot.CenterY)
	}
}

func TestPhaseE_RotateAnim_SmilUnaffected(t *testing.T) {
	// SMIL animateTransform does not consult transform-origin —
	// default pivot stays at (0,0) when values omits cx,cy.
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<rect x="10" y="20" width="40" height="60">
			<animateTransform attributeName="transform" type="rotate"
				by="360" dur="1s"/>
		</rect>
	</svg>`
	vg, _ := parseSvg(src)
	if len(vg.Animations) == 0 {
		t.Fatal("no animations")
	}
	a := vg.Animations[0]
	if a.Kind != gui.SvgAnimRotate {
		t.Fatalf("kind: %v", a.Kind)
	}
	if a.CenterX != 0 || a.CenterY != 0 {
		t.Errorf("smil rotate center: (%v,%v) want (0,0)",
			a.CenterX, a.CenterY)
	}
}
