package svg

import (
	"math"
	"testing"
)

// Matrix layout: [a b c d e f]; point map
// (x',y') = (a*x + c*y + e, b*x + d*y + f).

func approxEq(a, b, eps float32) bool {
	if a-b > eps || b-a > eps {
		return false
	}
	return true
}

func TestDecomposeTRSIdentity(t *testing.T) {
	tx, ty, sx, sy, rot, ok := decomposeTRS(identityTransform)
	if !ok {
		t.Fatalf("identity must decompose ok")
	}
	if tx != 0 || ty != 0 || sx != 1 || sy != 1 || rot != 0 {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSPureTranslate(t *testing.T) {
	m := [6]float32{1, 0, 0, 1, 12, -3}
	tx, ty, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("pure translate must decompose ok")
	}
	if tx != 12 || ty != -3 || sx != 1 || sy != 1 || rot != 0 {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSPureScale(t *testing.T) {
	m := [6]float32{2, 0, 0, 3, 0, 0}
	tx, ty, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("pure scale must decompose ok")
	}
	if tx != 0 || ty != 0 || !approxEq(sx, 2, 1e-5) ||
		!approxEq(sy, 3, 1e-5) || rot != 0 {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSZeroScale(t *testing.T) {
	// Placeholder scale(0): all linear entries zero.
	m := [6]float32{0, 0, 0, 0, 12, 12}
	tx, ty, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("scale(0) must decompose ok")
	}
	if tx != 12 || ty != 12 || sx != 0 || sy != 0 || rot != 0 {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSPureRotate90(t *testing.T) {
	// 90° CCW in math sense, CW in SVG (y-down): matrix [0 1 -1 0 0 0].
	m := [6]float32{0, 1, -1, 0, 0, 0}
	tx, ty, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("pure rotate must decompose ok")
	}
	if !approxEq(sx, 1, 1e-5) || !approxEq(sy, 1, 1e-5) ||
		!approxEq(rot, 90, 1e-4) || tx != 0 || ty != 0 {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSTranslateThenScale(t *testing.T) {
	// translate(10,20) scale(2,3): applied to point (x,y) yields
	// (2x+10, 3y+20). Matrix: [2 0 0 3 10 20].
	m := [6]float32{2, 0, 0, 3, 10, 20}
	tx, ty, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("translate+scale must decompose ok")
	}
	if tx != 10 || ty != 20 || !approxEq(sx, 2, 1e-5) ||
		!approxEq(sy, 3, 1e-5) || rot != 0 {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSTranslateRotate(t *testing.T) {
	// translate(5,7) then rotate(90): applying to (1,0) gives (0+5, 1+7).
	// Matrix (x', y') = (a+c*y+e, b+d*y+f):
	// For rotate 90 as above: a=0,b=1,c=-1,d=0, plus translation e=5,f=7.
	m := [6]float32{0, 1, -1, 0, 5, 7}
	tx, ty, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("translate+rotate must decompose ok")
	}
	if tx != 5 || ty != 7 || !approxEq(sx, 1, 1e-5) ||
		!approxEq(sy, 1, 1e-5) || !approxEq(rot, 90, 1e-4) {
		t.Fatalf("got tx=%v ty=%v sx=%v sy=%v rot=%v", tx, ty, sx, sy, rot)
	}
}

func TestDecomposeTRSShearFallback(t *testing.T) {
	// skewX(45): matrix [1 0 tan(45)=1 1 0 0]. Not a pure TRS.
	m := [6]float32{1, 0, 1, 1, 0, 0}
	_, _, _, _, _, ok := decomposeTRS(m)
	if ok {
		t.Fatalf("shear must fail decomposition")
	}
}

func TestDecomposeTRSRejectsNonFiniteTX(t *testing.T) {
	m := [6]float32{1, 0, 0, 1, float32(math.NaN()), 0}
	_, _, _, _, _, ok := decomposeTRS(m)
	if ok {
		t.Fatalf("matrix with NaN tx must not decompose ok")
	}
}

func TestDecomposeTRSRejectsNonFiniteTY(t *testing.T) {
	m := [6]float32{1, 0, 0, 1, 0, float32(math.Inf(1))}
	_, _, _, _, _, ok := decomposeTRS(m)
	if ok {
		t.Fatalf("matrix with +Inf ty must not decompose ok")
	}
}

func TestDecomposeTRSRejectsNegativeInfTX(t *testing.T) {
	m := [6]float32{1, 0, 0, 1, float32(math.Inf(-1)), 0}
	_, _, _, _, _, ok := decomposeTRS(m)
	if ok {
		t.Fatalf("matrix with -Inf tx must not decompose ok")
	}
}

func TestDecomposeTRSRejectsNaN_TY(t *testing.T) {
	m := [6]float32{1, 0, 0, 1, 0, float32(math.NaN())}
	_, _, _, _, _, ok := decomposeTRS(m)
	if ok {
		t.Fatalf("matrix with NaN ty must not decompose ok")
	}
}

func TestDecomposeTRSRotate45Scale2(t *testing.T) {
	// rotate(45) * scale(2): a=2*cos45, b=2*sin45, c=-2*sin45, d=2*cos45.
	c45 := float32(math.Cos(math.Pi / 4))
	s45 := float32(math.Sin(math.Pi / 4))
	m := [6]float32{2 * c45, 2 * s45, -2 * s45, 2 * c45, 0, 0}
	_, _, sx, sy, rot, ok := decomposeTRS(m)
	if !ok {
		t.Fatalf("rotate*scale must decompose ok")
	}
	if !approxEq(sx, 2, 1e-4) || !approxEq(sy, 2, 1e-4) ||
		!approxEq(rot, 45, 1e-3) {
		t.Fatalf("got sx=%v sy=%v rot=%v", sx, sy, rot)
	}
}
