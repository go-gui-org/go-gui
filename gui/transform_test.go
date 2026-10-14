package gui

import (
	"math"
	"testing"
)

// xfNear reports whether a and b differ by less than a tolerance that
// suits pixel coordinates after float32 trig.
func xfNear(a, b float32) bool {
	return math.Abs(float64(a-b)) < 1e-3
}

func TestTransformZeroIsIdentity(t *testing.T) {
	var tr Transform
	if !tr.IsIdentity() {
		t.Fatal("zero Transform must be the identity")
	}
	h, _ := tr.resolve(10, 20, 100, 50)
	x, y, ok := h.apply(37, 41)
	if !ok || !xfNear(x, 37) || !xfNear(y, 41) {
		t.Fatalf("identity moved point: got (%v,%v) ok=%v", x, y, ok)
	}
}

func TestTransformSetComponentsAreNotIdentity(t *testing.T) {
	cases := map[string]Transform{
		"translate":   Translate(1, 0),
		"rotate":      Rotate(1),
		"rotateX":     RotateX(1),
		"rotateY":     RotateY(1),
		"skew":        Skew(1, 0),
		"scale":       Scale(2),
		"scaleXY":     ScaleXY(1, 0.5),
		"scaleZero":   Scale(0),
		"perspective": RotateY(1).Perspective(500),
	}
	for name, tr := range cases {
		if tr.IsIdentity() {
			t.Errorf("%s: reported identity", name)
		}
	}
	// Origin and backface alone move nothing, so they stay identity.
	if !Origin(0, 0).BackfaceHidden().IsIdentity() {
		t.Error("origin + backface alone must be the identity")
	}
	// A view lies on z = 0, so perspective moves no point unless something
	// turns the view out of the screen plane.
	if !Perspective(500).Rotate(30).Rotate(0).IsIdentity() {
		t.Error("perspective with no rotateX/rotateY must be the identity")
	}
}

func TestTransformTranslate(t *testing.T) {
	h, _ := Translate(5, -3).resolve(0, 0, 10, 10)
	x, y, _ := h.apply(1, 2)
	if !xfNear(x, 6) || !xfNear(y, -1) {
		t.Fatalf("got (%v,%v), want (6,-1)", x, y)
	}
}

func TestTransformRotateAboutCenterClockwise(t *testing.T) {
	// Box at (0,0) 100x100. The center is (50,50). A 90° turn moves the
	// top-middle point (50,0) to the right-middle point (100,50): clockwise
	// on a y-down screen, the same sense as QuarterTurns.
	h, _ := Rotate(90).resolve(0, 0, 100, 100)
	x, y, _ := h.apply(50, 0)
	if !xfNear(x, 100) || !xfNear(y, 50) {
		t.Fatalf("got (%v,%v), want (100,50)", x, y)
	}
}

func TestTransformOriginTopLeft(t *testing.T) {
	// Origin (0,0) is the top-left corner. Scaling by 2 leaves that corner
	// in place and moves the bottom-right corner out.
	h, _ := Scale(2).Origin(0, 0).resolve(10, 10, 20, 20)
	x, y, _ := h.apply(10, 10)
	if !xfNear(x, 10) || !xfNear(y, 10) {
		t.Fatalf("origin moved: (%v,%v)", x, y)
	}
	x, y, _ = h.apply(30, 30)
	if !xfNear(x, 50) || !xfNear(y, 50) {
		t.Fatalf("corner: got (%v,%v), want (50,50)", x, y)
	}
}

func TestTransformSkewX(t *testing.T) {
	// skewX(45°) about the top-left: x' = x + y.
	h, _ := Skew(45, 0).Origin(0, 0).resolve(0, 0, 10, 10)
	x, y, _ := h.apply(0, 10)
	if !xfNear(x, 10) || !xfNear(y, 10) {
		t.Fatalf("got (%v,%v), want (10,10)", x, y)
	}
}

func TestTransformComponentOrder(t *testing.T) {
	// Translate is applied after rotate (CSS order: translate is outermost).
	// So the point turns about the origin first, then shifts by +10 in x.
	h, _ := Rotate(90).Translate(10, 0).Origin(0, 0).resolve(0, 0, 10, 10)
	x, y, _ := h.apply(1, 0)
	if !xfNear(x, 10) || !xfNear(y, 1) {
		t.Fatalf("got (%v,%v), want (10,1)", x, y)
	}
}

func TestTransformMethodReplacesComponent(t *testing.T) {
	if Rotate(10).Rotate(20) != Rotate(20) {
		t.Fatal("a second Rotate must replace the first")
	}
}

func TestTransformRotateYPerspective(t *testing.T) {
	// rotateY(60°) with perspective 200 about the center of a 100x100 box.
	// The right edge goes back (away from the viewer), so it gets shorter
	// than the left edge, and the center does not move.
	h, _ := RotateY(60).Perspective(200).resolve(0, 0, 100, 100)
	cx, cy, _ := h.apply(50, 50)
	if !xfNear(cx, 50) || !xfNear(cy, 50) {
		t.Fatalf("center moved: (%v,%v)", cx, cy)
	}
	_, lt, _ := h.apply(0, 0)
	_, lb, _ := h.apply(0, 100)
	_, rt, _ := h.apply(100, 0)
	_, rb, _ := h.apply(100, 100)
	left, right := lb-lt, rb-rt
	if !(right < left) {
		t.Fatalf("right edge %v must be shorter than left edge %v", right, left)
	}
}

func TestTransformInverseRoundTrip(t *testing.T) {
	trs := []Transform{
		Translate(7, -2),
		Rotate(33),
		Skew(20, -10),
		ScaleXY(1.5, 0.75),
		RotateX(40).Perspective(300),
		RotateY(-50).Perspective(150).Rotate(15).Translate(4, 9),
	}
	for i, tr := range trs {
		h, _ := tr.resolve(20, 30, 120, 80)
		inv, ok := h.inverse()
		if !ok {
			t.Fatalf("case %d: not invertible", i)
		}
		for _, p := range [][2]float32{{20, 30}, {80, 70}, {140, 110}} {
			x, y, ok1 := h.apply(p[0], p[1])
			bx, by, ok2 := inv.apply(x, y)
			if !ok1 || !ok2 || !xfNear(bx, p[0]) || !xfNear(by, p[1]) {
				t.Errorf("case %d: %v -> (%v,%v) -> (%v,%v)", i, p, x, y, bx, by)
			}
		}
	}
}

func TestTransformScaleZeroNotInvertible(t *testing.T) {
	h, _ := Scale(0).resolve(0, 0, 10, 10)
	if _, ok := h.inverse(); ok {
		t.Fatal("scale 0 must not be invertible")
	}
}

func TestTransformBackface(t *testing.T) {
	cases := []struct {
		name string
		tr   Transform
		back bool
	}{
		{"identity", Transform{}.BackfaceHidden(), false},
		{"rotY 90 edge-on is not back", RotateY(90).BackfaceHidden(), false},
		{"rotY 180", RotateY(180).BackfaceHidden(), true},
		{"rotX 120", RotateX(120).BackfaceHidden(), true},
		{"rotY 180 twice is front", RotateY(180).RotateX(180).BackfaceHidden(), false},
		// A mirror in 2D keeps the front face toward the viewer, as in CSS.
		{"scaleX -1 is front", ScaleXY(-1, 1).BackfaceHidden(), false},
		// Without the flag the face is never hidden.
		{"rotY 180 visible", RotateY(180), false},
		// The edge-on cutoff is relative to the scale, so a flipped card
		// that a spring is shrinking toward Scale(0) stays hidden.
		{"rotY 180 tiny scale", RotateY(180).Scale(1e-5).BackfaceHidden(), true},
		{"rotY 90 tiny scale edge-on", RotateY(90).Scale(1e-5).BackfaceHidden(), false},
	}
	for _, c := range cases {
		_, hidden := c.tr.resolve(0, 0, 10, 10)
		if hidden != c.back {
			t.Errorf("%s: hidden=%v, want %v", c.name, hidden, c.back)
		}
	}
}

func TestTransformLerp(t *testing.T) {
	a := Translate(0, 0).Rotate(0).Scale(1)
	b := Translate(10, 20).Rotate(90).Scale(3)
	m := a.Lerp(b, 0.5)
	if m != Translate(5, 10).Rotate(45).Scale(2) {
		t.Fatalf("midpoint wrong: %+v", m)
	}
	if a.Lerp(b, 0) != a || a.Lerp(b, 1) != b {
		t.Fatal("endpoints must match the inputs")
	}
	// The backface flag switches at the midpoint.
	if Rotate(0).Lerp(Rotate(0).BackfaceHidden(), 0.4).backHidden {
		t.Fatal("flag must stay off before the midpoint")
	}
	if !Rotate(0).Lerp(Rotate(0).BackfaceHidden(), 0.5).backHidden {
		t.Fatal("flag must be on from the midpoint")
	}
}

func TestTransformLerpPerspectiveNeverReverses(t *testing.T) {
	// A spring that overshoots toward "no perspective" (f > 1) or
	// undershoots from it (f < 0) must not make 1/d negative.
	a := Perspective(200)
	if got := a.Lerp(Transform{}, 1.5).perspInv; got < 0 {
		t.Fatalf("overshoot: perspInv=%v", got)
	}
	if got := (Transform{}).Lerp(a, -0.5).perspInv; got < 0 {
		t.Fatalf("undershoot: perspInv=%v", got)
	}
}

func TestTransformTinyPerspectiveStaysFinite(t *testing.T) {
	// 1/d overflows float32 for a tiny positive d. Like CSS, d is clamped
	// to 1 px, so the matrix stays finite.
	h, _ := RotateY(30).Perspective(1e-39).resolve(0, 0, 100, 100)
	for i, v := range h {
		if !f32IsFinite(v) {
			t.Fatalf("h[%d]=%v", i, v)
		}
	}
}

func TestTransformNonFiniteIsIgnored(t *testing.T) {
	nan := float32(math.NaN())
	if !Rotate(nan).IsIdentity() || !Translate(nan, 0).IsIdentity() {
		t.Fatal("non-finite input must leave the component unset")
	}
}

func TestTransformLerpNonFiniteFractionKeepsStart(t *testing.T) {
	a := Translate(4, 2).Rotate(10)
	b := Rotate(90).Scale(2)
	for _, f := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		if got := a.Lerp(b, f); got != a {
			t.Errorf("Lerp(f=%v) = %+v, want the start %+v", f, got, a)
		}
	}
}

func TestTransformResolveNoAlloc(t *testing.T) {
	tr := RotateY(30).Perspective(400).Rotate(10).Scale(1.1)
	n := testing.AllocsPerRun(100, func() {
		h, _ := tr.resolve(1, 2, 300, 200)
		inv, _ := h.inverse()
		_, _, _ = inv.apply(5, 5)
	})
	if n != 0 {
		t.Fatalf("resolve/inverse/apply allocated %v times", n)
	}
}

// affineAt is the flat stand-in both flat backends draw with: it maps the
// fit point where the homography does, and its slopes match the
// homography's there. pdfAffineAt must wrap it exactly.
func TestXfMatAffineAt(t *testing.T) {
	h, _ := RotateY(30).Perspective(400).resolve(0, 0, 100, 100)
	const x, y = float32(50), float32(50)
	a, b, c, d, e, f, ok := h.affineAt(x, y)
	if !ok {
		t.Fatal("affineAt failed on a visible point")
	}
	px, py, _ := h.apply(x, y)
	if got := a*x + c*y + e; !xfNear(got, px) {
		t.Errorf("affine x at fit point = %v, want %v", got, px)
	}
	if got := b*x + d*y + f; !xfNear(got, py) {
		t.Errorf("affine y at fit point = %v, want %v", got, py)
	}
	// Slopes match the homography: a nearby point maps near both.
	const eps = 0.5
	for _, dx := range []float32{-eps, eps} {
		for _, dy := range []float32{-eps, eps} {
			wx, wy, _ := h.apply(x+dx, y+dy)
			if got := a*(x+dx) + c*(y+dy) + e; !xfNear(got, wx) {
				t.Errorf("affine x at (%v,%v) = %v, want %v", x+dx, y+dy, got, wx)
			}
			if got := b*(x+dx) + d*(y+dy) + f; !xfNear(got, wy) {
				t.Errorf("affine y at (%v,%v) = %v, want %v", x+dx, y+dy, got, wy)
			}
		}
	}
	pa, ok := pdfAffineAt(&h, x, y)
	if !ok {
		t.Fatal("pdfAffineAt failed where affineAt succeeded")
	}
	want := pdfAffine{float64(a), float64(b), float64(c), float64(d), float64(e), float64(f)}
	if pa != want {
		t.Errorf("pdfAffineAt = %v, want %v", pa, want)
	}
	// Identity stays the identity.
	ia, ib, ic, id, ie, ig, ok := xfIdentity.affineAt(3, 7)
	if !ok || ia != 1 || ib != 0 || ic != 0 || id != 1 || ie != 0 || ig != 0 {
		t.Errorf("identity affineAt = (%v,%v,%v,%v,%v,%v,%v), want (1,0,0,1,0,0,true)",
			ia, ib, ic, id, ie, ig, ok)
	}
}
