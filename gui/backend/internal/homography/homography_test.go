package homography

import (
	"math"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// persp is a turn about y with a perspective term, like
// RotateY(30).Perspective(400) resolves to.
var persp = Mat{0.87, 0, 10, 0, 1, -5, 0.00125, 0, 1}

func TestInverseRoundTrip(t *testing.T) {
	inv, ok := Inverse(&persp)
	if !ok {
		t.Fatal("no inverse")
	}
	for _, p := range [][2]float32{{0, 0}, {40, 25}, {-30, 90}} {
		x, y, ok1 := Apply(&persp, p[0], p[1])
		bx, by, ok2 := Apply(&inv, x, y)
		if !ok1 || !ok2 || !near(bx, p[0]) || !near(by, p[1]) {
			t.Fatalf("%v -> %v,%v -> %v,%v", p, x, y, bx, by)
		}
	}
}

func TestInverseSingular(t *testing.T) {
	if _, ok := Inverse(&Mat{}); ok {
		t.Fatal("zero matrix has no inverse")
	}
}

func TestApplyBehindViewer(t *testing.T) {
	m := Mat{1, 0, 0, 0, 1, 0, -0.01, 0, 1}
	if _, _, ok := Apply(&m, 200, 0); ok {
		t.Fatal("w < 0 must not map")
	}
}

func TestMulOrder(t *testing.T) {
	move := Mat{1, 0, 10, 0, 1, 0, 0, 0, 1}
	scale := Mat{2, 0, 0, 0, 2, 0, 0, 0, 1}
	m := Mul(&move, &scale) // scale, then move
	x, _, _ := Apply(&m, 5, 0)
	if !near(x, 20) {
		t.Fatalf("x = %v, want 20", x)
	}
}

func TestScaledMatchesLogical(t *testing.T) {
	const s = 2
	d := Scaled(&persp, s)
	lx, ly, _ := Apply(&persp, 30, 40)
	dx, dy, _ := Apply(&d, 30*s, 40*s)
	if !near(dx, lx*s) || !near(dy, ly*s) {
		t.Fatalf("device %v,%v, want %v,%v", dx, dy, lx*s, ly*s)
	}
}

func TestBounds(t *testing.T) {
	m := Mat{0, -1, 0, 1, 0, 0, 0, 0, 1} // a quarter turn
	x, y, w, h, ok := Bounds(&m, 0, 0, 10, 20)
	if !ok || !near(x, -20) || !near(y, 0) || !near(w, 20) || !near(h, 10) {
		t.Fatalf("got %v %v %v %v %v", x, y, w, h, ok)
	}
}

// AffineAt must agree with h at the point, and to first order near it.
func TestAffineAtMatchesNearPoint(t *testing.T) {
	const cx, cy = 60, 30
	a, b, c, d, e, f, ok := AffineAt(&persp, cx, cy)
	if !ok {
		t.Fatal("not ok")
	}
	for _, o := range [][2]float32{{0, 0}, {0.5, 0}, {0, 0.5}} {
		x, y := float32(cx)+o[0], float32(cy)+o[1]
		hx, hy, _ := Apply(&persp, x, y)
		ax, ay := a*x+c*y+e, b*x+d*y+f
		if math.Abs(float64(hx-ax)) > 1e-2 || math.Abs(float64(hy-ay)) > 1e-2 {
			t.Fatalf("at %v,%v: h %v,%v, affine %v,%v", x, y, hx, hy, ax, ay)
		}
	}
}

// With no perspective, AffineAt is h itself at any point.
func TestAffineAtExactWithoutPerspective(t *testing.T) {
	m := Mat{0.97, -0.26, 21.5, 0.26, 0.97, -23.5, 0, 0, 1}
	a, b, c, d, e, f, _ := AffineAt(&m, 500, -300)
	got := [6]float32{a, b, c, d, e, f}
	want := [6]float32{0.97, 0.26, -0.26, 0.97, 21.5, -23.5}
	for i := range got {
		if !near(got[i], want[i]) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
