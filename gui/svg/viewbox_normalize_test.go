package svg

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A non-zero viewBox origin with animateMotion + animateTransform rotate
// must produce the same effective rendered geometry and animation state
// as the same content authored with pre-shifted coords in a zero-origin
// viewBox. Render applies the viewBox origin as an outer translate, so
// effective = tessellated_vertex - ViewBoxXY (after any deferred Base
// transform is composed in).
func TestViewBoxNormalize_ParityWithPreShiftedAsset(t *testing.T) {
	shifted := `<svg viewBox="100 100 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="112" cy="112" r="4" fill="red">
			<animateMotion dur="1s" path="M100,100 L124,124" repeatCount="indefinite"/>
		</circle>
		<rect x="105" y="105" width="6" height="6" fill="blue"
			transform="rotate(30 112 112)"/>
	</svg>`
	preShifted := `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
		<circle cx="12" cy="12" r="4" fill="red">
			<animateMotion dur="1s" path="M0,0 L24,24" repeatCount="indefinite"/>
		</circle>
		<rect x="5" y="5" width="6" height="6" fill="blue"
			transform="rotate(30 12 12)"/>
	</svg>`

	pa := New()
	pb := New()
	parsedA, err := pa.ParseSvg(shifted)
	if err != nil {
		t.Fatalf("shifted parse: %v", err)
	}
	parsedB, err := pb.ParseSvg(preShifted)
	if err != nil {
		t.Fatalf("preShifted parse: %v", err)
	}
	if parsedA.ViewBoxX != 100 || parsedA.ViewBoxY != 100 {
		t.Fatalf("shifted viewBox not propagated: (%v,%v)",
			parsedA.ViewBoxX, parsedA.ViewBoxY)
	}
	if parsedB.ViewBoxX != 0 || parsedB.ViewBoxY != 0 {
		t.Fatalf("preShifted viewBox should be zero: (%v,%v)",
			parsedB.ViewBoxX, parsedB.ViewBoxY)
	}

	// Tessellate both. After subtracting each parsed's ViewBoxX/Y,
	// the effective screen-space vertex coords must match.
	trisA := pa.Tessellate(parsedA, 1)
	trisB := pb.Tessellate(parsedB, 1)
	if len(trisA) != len(trisB) {
		t.Fatalf("path count mismatch: %d vs %d", len(trisA), len(trisB))
	}
	for i := range trisA {
		compareEffectiveWithShift(t, i, trisA[i], parsedA.ViewBoxX, parsedA.ViewBoxY,
			trisB[i], parsedB.ViewBoxX, parsedB.ViewBoxY)
	}

	// Rotation centers and motion paths live in raw viewBox space in
	// each parsed output. Effective = field - ViewBoxXY must match.
	if len(parsedA.Animations) != len(parsedB.Animations) {
		t.Fatalf("animation count mismatch: %d vs %d",
			len(parsedA.Animations), len(parsedB.Animations))
	}
	for i := range parsedA.Animations {
		a := parsedA.Animations[i]
		b := parsedB.Animations[i]
		if a.Kind != b.Kind {
			t.Fatalf("anim[%d] kind: %v vs %v", i, a.Kind, b.Kind)
		}
		switch a.Kind {
		case gui.SvgAnimRotate:
			ax := a.CenterX - parsedA.ViewBoxX
			ay := a.CenterY - parsedA.ViewBoxY
			bx := b.CenterX - parsedB.ViewBoxX
			by := b.CenterY - parsedB.ViewBoxY
			if !approx(ax, bx, 1e-3) || !approx(ay, by, 1e-3) {
				t.Fatalf("rotate center drift: (%v,%v) vs (%v,%v)",
					ax, ay, bx, by)
			}
		case gui.SvgAnimMotion:
			if len(a.MotionPath) != len(b.MotionPath) {
				t.Fatalf("motion len: %d vs %d",
					len(a.MotionPath), len(b.MotionPath))
			}
			for j := 0; j+1 < len(a.MotionPath); j += 2 {
				ax := a.MotionPath[j] - parsedA.ViewBoxX
				ay := a.MotionPath[j+1] - parsedA.ViewBoxY
				bx := b.MotionPath[j] - parsedB.ViewBoxX
				by := b.MotionPath[j+1] - parsedB.ViewBoxY
				if !approx(ax, bx, 1e-3) || !approx(ay, by, 1e-3) {
					t.Fatalf("motion[%d] drift: (%v,%v) vs (%v,%v)",
						j/2, ax, ay, bx, by)
				}
			}
		}
	}
}

func compareEffectiveWithShift(t *testing.T, idx int,
	a gui.TessellatedPath, vbAX, vbAY float32,
	b gui.TessellatedPath, vbBX, vbBY float32,
) {
	t.Helper()
	if len(a.Triangles) != len(b.Triangles) {
		t.Fatalf("path[%d] tri count: %d vs %d",
			idx, len(a.Triangles), len(b.Triangles))
	}
	for i := 0; i+1 < len(a.Triangles); i += 2 {
		ax, ay := effectiveVertex(a, a.Triangles[i], a.Triangles[i+1])
		bx, by := effectiveVertex(b, b.Triangles[i], b.Triangles[i+1])
		ax -= vbAX
		ay -= vbAY
		bx -= vbBX
		by -= vbBY
		if !approx(ax, bx, 1e-2) || !approx(ay, by, 1e-2) {
			t.Fatalf("path[%d] vert[%d] drift: (%v,%v) vs (%v,%v)",
				idx, i/2, ax, ay, bx, by)
		}
	}
}

// effectiveVertex mirrors the backend TRS pipeline: scale, translate,
// then rotate about (rcx, rcy) — pivot==BaseRotCX/CY when a non-zero
// rotation pivot is present (authored rotate-about-(cx,cy)), else
// pivot==(BaseTransX, BaseTransY) (authored translate-then-rotate).
// Identity when !HasBaseXform.
func effectiveVertex(p gui.TessellatedPath, x, y float32) (float32, float32) {
	if !p.HasBaseXform {
		return x, y
	}
	x = x*p.BaseScaleX + p.BaseTransX
	y = y*p.BaseScaleY + p.BaseTransY
	if p.BaseRotAngle == 0 {
		return x, y
	}
	rad := float64(p.BaseRotAngle) * math.Pi / 180
	sinA := float32(math.Sin(rad))
	cosA := float32(math.Cos(rad))
	rcx, rcy := p.BaseRotCX, p.BaseRotCY
	if rcx == 0 && rcy == 0 {
		rcx, rcy = p.BaseTransX, p.BaseTransY
	}
	dx := x - rcx
	dy := y - rcy
	return rcx + dx*cosA - dy*sinA,
		rcy + dx*sinA + dy*cosA
}

func approx(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return float64(d) < float64(eps) ||
		math.IsNaN(float64(a)) && math.IsNaN(float64(b))
}
