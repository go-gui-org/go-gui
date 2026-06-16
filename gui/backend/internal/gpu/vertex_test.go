package gpu

import (
	"math"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

func TestNormColor(t *testing.T) {
	r, g, b, a := NormColor(255, 128, 0, 64)
	if r != 1.0 {
		t.Errorf("r = %v, want 1.0", r)
	}
	if g != 128.0/255.0 {
		t.Errorf("g = %v, want %v", g, 128.0/255.0)
	}
	if b != 0 {
		t.Errorf("b = %v, want 0", b)
	}
	if a != 64.0/255.0 {
		t.Errorf("a = %v, want %v", a, 64.0/255.0)
	}
}

func TestPackParams(t *testing.T) {
	p := PackParams(5.0, 2.0)
	// radius = floor(5*4)*4096 = 20*4096 = 81920
	// thickness = floor(2*4) = 8
	// total = 81928
	if p != 81928 {
		t.Errorf("PackParams(5, 2) = %v, want 81928", p)
	}
}

func TestBuildQuadPositions(t *testing.T) {
	c := gui.White
	q := BuildQuad(10, 20, 100, 50, c, 4, 0)
	// TL
	if q[0].X != 10 || q[0].Y != 20 {
		t.Errorf("TL pos = (%v, %v), want (10, 20)", q[0].X, q[0].Y)
	}
	if q[0].U != -1 || q[0].V != -1 {
		t.Errorf("TL uv = (%v, %v), want (-1, -1)", q[0].U, q[0].V)
	}
	// BR
	if q[2].X != 110 || q[2].Y != 70 {
		t.Errorf("BR pos = (%v, %v), want (110, 70)", q[2].X, q[2].Y)
	}
	if q[2].U != 1 || q[2].V != 1 {
		t.Errorf("BR uv = (%v, %v), want (1, 1)", q[2].U, q[2].V)
	}
}

func TestBuildQuadColor(t *testing.T) {
	c := gui.Color{R: 255, G: 128, B: 64, A: 255}
	q := BuildQuad(0, 0, 10, 10, c, 0, 0)
	for i := range 4 {
		if q[i].R != 1.0 || q[i].G != 128.0/255.0 ||
			q[i].B != 64.0/255.0 || q[i].A != 1.0 {
			t.Errorf("vert[%d] color = (%v,%v,%v,%v), want (1,%.4f,%.4f,1)",
				i, q[i].R, q[i].G, q[i].B, q[i].A,
				128.0/255.0, 64.0/255.0)
		}
	}
}

func TestVertexLayout(t *testing.T) {
	// Verify struct is exactly 9 float32s (36 bytes) — no padding.
	if unsafe.Sizeof(Vertex{}) != 36 {
		t.Errorf("Vertex size = %d, want 36", unsafe.Sizeof(Vertex{}))
	}
}

func TestPackParams_NaN_ClampsToZero(t *testing.T) {
	nan := float32(math.NaN())
	if p := PackParams(nan, 5); p != PackParams(0, 5) {
		t.Errorf("PackParams(NaN, 5) = %v, want same as PackParams(0, 5)", p)
	}
	if p := PackParams(5, nan); p != PackParams(5, 0) {
		t.Errorf("PackParams(5, NaN) = %v, want same as PackParams(5, 0)", p)
	}
}

func TestPackParams_Inf_ClampsToZero(t *testing.T) {
	inf := float32(math.Inf(1))
	negInf := float32(math.Inf(-1))
	if p := PackParams(inf, 5); p != PackParams(0, 5) {
		t.Errorf("PackParams(+Inf, 5) = %v, want same as PackParams(0, 5)", p)
	}
	if p := PackParams(5, negInf); p != PackParams(5, 0) {
		t.Errorf("PackParams(5, -Inf) = %v, want same as PackParams(5, 0)", p)
	}
}

func TestNormColor_Zero(t *testing.T) {
	r, g, b, a := NormColor(0, 0, 0, 0)
	if r != 0 || g != 0 || b != 0 || a != 0 {
		t.Errorf("NormColor(0,0,0,0) = (%v,%v,%v,%v), want zeros", r, g, b, a)
	}
}

func TestNormColor_FullWhite(t *testing.T) {
	r, g, b, a := NormColor(255, 255, 255, 255)
	if r != 1 || g != 1 || b != 1 || a != 1 {
		t.Errorf("NormColor(255,255,255,255) = (%v,%v,%v,%v), want ones",
			r, g, b, a)
	}
}
