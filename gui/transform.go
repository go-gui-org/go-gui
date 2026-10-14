package gui

import "math"

// Transform is a visual transform on a view (#1019). Layout ignores it: the
// view keeps its laid-out box, and only the drawing and the pointer input go
// through the transform. Children inherit the transform of their parent.
//
// The zero value is the identity. Build a Transform with the package
// functions and chain the methods of the same names:
//
//	gui.Rotate(8).Scale(1.05)
//	gui.RotateY(180).Perspective(800).BackfaceHidden()
//
// The components are applied in one fixed order, the order of the CSS
// individual transform properties, outermost first:
//
//	translate → perspective → rotateX → rotateY → rotate → skew → scale
//
// all about the origin. Because the order is fixed, a method sets its
// component and replaces an earlier value; Rotate(10).Rotate(20) is
// Rotate(20). A fixed order covers the usual effects (lift, tilt, flip,
// tumble) with no slice, so a Transform is a plain value that costs no
// allocation, and Lerp between two transforms is always defined.
//
// A non-finite argument leaves its component unchanged.
//
// exportaudit:keep — reachable from an exported signature
type Transform struct {
	// Every field is stored so that 0 means "no effect". That makes the
	// zero value the identity with no Opt wrappers: scale is kept as
	// scale-1, the origin as an offset from the center, and perspective as
	// its inverse (1/d), which is also the space CSS interpolates it in.
	tx, ty       float32 // translate, logical px
	perspInv     float32 // 1/d; 0 is no perspective
	rotX, rotY   float32 // degrees, around the X and Y axes
	rotZ         float32 // degrees, clockwise on screen
	skewX, skewY float32 // degrees
	scaleXm1     float32 // scale X minus 1
	scaleYm1     float32 // scale Y minus 1
	originX      float32 // origin minus 0.5, as a fraction of the width
	originY      float32 // origin minus 0.5, as a fraction of the height
	backHidden   bool
}

// Translate returns a Transform that moves the view by (x, y) logical pixels.
// exportaudit:keep — caller-facing API (#1019)
func Translate(x, y float32) Transform { return Transform{}.Translate(x, y) }

// Rotate returns a Transform that turns the view deg degrees clockwise on
// screen, around the origin.
// exportaudit:keep — caller-facing API (#1019)
func Rotate(deg float32) Transform { return Transform{}.Rotate(deg) }

// RotateX returns a Transform that turns the view deg degrees around the
// horizontal axis. Use it with Perspective; without perspective the view
// only gets shorter.
// exportaudit:keep — caller-facing API (#1019)
func RotateX(deg float32) Transform { return Transform{}.RotateX(deg) }

// RotateY returns a Transform that turns the view deg degrees around the
// vertical axis. Use it with Perspective; without perspective the view only
// gets narrower.
// exportaudit:keep — caller-facing API (#1019)
func RotateY(deg float32) Transform { return Transform{}.RotateY(deg) }

// Skew returns a Transform that slants the view xDeg degrees along X and
// yDeg degrees along Y, as CSS skew().
// exportaudit:keep — caller-facing API (#1019)
func Skew(xDeg, yDeg float32) Transform { return Transform{}.Skew(xDeg, yDeg) }

// Scale returns a Transform that scales the view by s on both axes.
// exportaudit:keep — caller-facing API (#1019)
func Scale(s float32) Transform { return Transform{}.Scale(s) }

// ScaleXY returns a Transform that scales the view by sx and sy.
// exportaudit:keep — caller-facing API (#1019)
func ScaleXY(sx, sy float32) Transform { return Transform{}.ScaleXY(sx, sy) }

// Perspective returns a Transform with a viewing distance of d logical
// pixels, as CSS perspective(). A smaller d gives a stronger effect. A d
// that is not greater than 0 means no perspective.
// exportaudit:keep — caller-facing API (#1019)
func Perspective(d float32) Transform { return Transform{}.Perspective(d) }

// Origin returns a Transform whose origin is at (fx, fy), given as fractions
// of the view's box: (0, 0) is the top-left corner, (1, 1) the bottom-right.
// The default origin is the center, (0.5, 0.5).
// exportaudit:keep — caller-facing API (#1019)
func Origin(fx, fy float32) Transform { return Transform{}.Origin(fx, fy) }

// Translate sets the translation to (x, y) logical pixels.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Translate(x, y float32) Transform {
	if f32IsFinite(x) && f32IsFinite(y) {
		t.tx, t.ty = x, y
	}
	return t
}

// Rotate sets the rotation to deg degrees clockwise on screen.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Rotate(deg float32) Transform {
	if f32IsFinite(deg) {
		t.rotZ = deg
	}
	return t
}

// RotateX sets the rotation around the horizontal axis to deg degrees.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) RotateX(deg float32) Transform {
	if f32IsFinite(deg) {
		t.rotX = deg
	}
	return t
}

// RotateY sets the rotation around the vertical axis to deg degrees.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) RotateY(deg float32) Transform {
	if f32IsFinite(deg) {
		t.rotY = deg
	}
	return t
}

// Skew sets the slant to xDeg degrees along X and yDeg degrees along Y.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Skew(xDeg, yDeg float32) Transform {
	if f32IsFinite(xDeg) && f32IsFinite(yDeg) {
		t.skewX, t.skewY = xDeg, yDeg
	}
	return t
}

// Scale sets the scale to s on both axes. Scale(0) collapses the view to
// its origin; it then draws nothing and takes no pointer.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Scale(s float32) Transform { return t.ScaleXY(s, s) }

// ScaleXY sets the scale to sx and sy. A negative value mirrors the view.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) ScaleXY(sx, sy float32) Transform {
	if f32IsFinite(sx) && f32IsFinite(sy) {
		t.scaleXm1, t.scaleYm1 = sx-1, sy-1
	}
	return t
}

// Perspective sets the viewing distance to d logical pixels. A d that is
// not greater than 0 removes the perspective.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Perspective(d float32) Transform {
	switch {
	case !f32IsFinite(d):
	case d <= 0:
		t.perspInv = 0
	default:
		// CSS clamps the distance to 1 px. Without the clamp, 1/d
		// overflows float32 for a tiny d and the matrix fills with NaN.
		t.perspInv = 1 / max(d, 1)
	}
	return t
}

// Origin sets the origin to (fx, fy), as fractions of the view's box.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Origin(fx, fy float32) Transform {
	if f32IsFinite(fx) && f32IsFinite(fy) {
		t.originX, t.originY = fx-0.5, fy-0.5
	}
	return t
}

// BackfaceHidden marks the view so that it draws nothing while its back
// faces the viewer, as CSS backface-visibility: hidden. Use it for the
// two faces of a card flip. A mirror (a negative scale) does not show
// the back face.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) BackfaceHidden() Transform {
	t.backHidden = true
	return t
}

// IsIdentity reports whether t moves no point. The origin and the backface
// flag do not count: on their own they change nothing. Perspective does not
// count either unless RotateX or RotateY is set: a view lies on z = 0, so
// perspective moves it only after a turn takes it out of the screen plane.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) IsIdentity() bool {
	return t.tx == 0 && t.ty == 0 &&
		t.rotX == 0 && t.rotY == 0 && t.rotZ == 0 &&
		t.skewX == 0 && t.skewY == 0 &&
		t.scaleXm1 == 0 && t.scaleYm1 == 0
}

// Lerp returns the transform at fraction f of the way from t to to. Each
// component is interpolated on its own, so Lerp is defined for any two
// transforms; f outside [0, 1] extrapolates, which is what a spring that
// overshoots needs. The backface flag switches at f = 0.5. A non-finite
// f returns t unchanged.
// exportaudit:keep — caller-facing API (#1019)
func (t Transform) Lerp(to Transform, f float32) Transform {
	if !f32IsFinite(f) {
		return t
	}
	// a*(1-f) + b*f gives exactly a at f=0 and exactly b at f=1.
	l := func(a, b float32) float32 { return a*(1-f) + b*f }
	out := Transform{
		tx:       l(t.tx, to.tx),
		ty:       l(t.ty, to.ty),
		perspInv: l(t.perspInv, to.perspInv),
		rotX:     l(t.rotX, to.rotX),
		rotY:     l(t.rotY, to.rotY),
		rotZ:     l(t.rotZ, to.rotZ),
		skewX:    l(t.skewX, to.skewX),
		skewY:    l(t.skewY, to.skewY),
		scaleXm1: l(t.scaleXm1, to.scaleXm1),
		scaleYm1: l(t.scaleYm1, to.scaleYm1),
		originX:  l(t.originX, to.originX),
		originY:  l(t.originY, to.originY),
	}
	// An overshoot past "no perspective" (perspInv 0) would make 1/d
	// negative, which reverses the perspective: the far edge grows.
	out.perspInv = max(out.perspInv, 0)
	out.backHidden = t.backHidden
	if f >= 0.5 {
		out.backHidden = to.backHidden
	}
	return out
}

// xfMat is a 2D projective transform (a homography) in row-major order. It
// maps (x, y) to ((h0 x + h1 y + h2)/w, (h3 x + h4 y + h5)/w) with
// w = h6 x + h7 y + h8.
//
// A 3x3 matrix is enough for a 3D transform because every view is flat: it
// lies on z = 0 and its output is flattened back onto the screen plane, the
// CSS default (transform-style: flat). With z = 0 in and z dropped out, the
// third row and column of the 4x4 matrix never matter. What is left is
// exact for perspective, and its inverse maps the pointer back with no ray
// cast.
type xfMat [9]float32

// xfIdentity is the identity homography.
var xfIdentity = xfMat{1, 0, 0, 0, 1, 0, 0, 0, 1}

// xfMinW is the smallest w that apply accepts. A point at or behind the
// viewer (w <= 0) has no place on screen.
const xfMinW = 1e-6

// apply maps (x, y) through m. ok is false when the point falls at or
// behind the viewer.
func (m *xfMat) apply(x, y float32) (float32, float32, bool) {
	w := m[6]*x + m[7]*y + m[8]
	if !(w > xfMinW) {
		return 0, 0, false
	}
	return (m[0]*x + m[1]*y + m[2]) / w, (m[3]*x + m[4]*y + m[5]) / w, true
}

// inverse returns the inverse of m. ok is false when m collapses the plane
// (a zero scale, or a view turned exactly edge-on).
// backend/internal/homography has its own Inverse for the CPU backends:
// this one stays separate because the pointer mapping needs the float64
// working precision and the epsilon gate below, and package gui cannot
// import a backend-internal package.
func (m *xfMat) inverse() (xfMat, bool) {
	// Work in float64: a near-edge-on turn makes the determinant small,
	// and float32 loses the digits the pointer mapping needs.
	a, b, c := float64(m[0]), float64(m[1]), float64(m[2])
	d, e, f := float64(m[3]), float64(m[4]), float64(m[5])
	g, h, i := float64(m[6]), float64(m[7]), float64(m[8])
	c00 := e*i - f*h
	c01 := f*g - d*i
	c02 := d*h - e*g
	det := a*c00 + b*c01 + c*c02
	if math.Abs(det) < 1e-12 || math.IsNaN(det) || math.IsInf(det, 0) {
		return xfMat{}, false
	}
	inv := 1 / det
	return xfMat{
		float32(c00 * inv),
		float32((c*h - b*i) * inv),
		float32((b*f - c*e) * inv),
		float32(c01 * inv),
		float32((a*i - c*g) * inv),
		float32((c*d - a*f) * inv),
		float32(c02 * inv),
		float32((b*g - a*h) * inv),
		float32((a*e - b*d) * inv),
	}, true
}

// mul returns a·b: b is applied first, then a.
func (a *xfMat) mul(b *xfMat) xfMat {
	var out xfMat
	for r := range 3 {
		for c := range 3 {
			out[r*3+c] = a[r*3]*b[c] + a[r*3+1]*b[3+c] + a[r*3+2]*b[6+c]
		}
	}
	return out
}

// mapRectBox returns the bounding box of r mapped through mapPt: the box
// of its four mapped corners. ok is false when a corner has no place on
// screen. It is the shared shape behind xfFrame.box and xfBoxToWindowOK,
// which differ only in the map (a render-walk frame vs a layout path).
// backend/internal/homography.Bounds is the same shape for the CPU
// backends; it stays separate because package gui cannot import a
// backend-internal package.
func mapRectBox(r drawClip, mapPt func(float32, float32) (float32, float32, bool)) (drawClip, bool) {
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for i := range 4 {
		cx, cy := r.X, r.Y
		if i&1 != 0 {
			cx += r.Width
		}
		if i&2 != 0 {
			cy += r.Height
		}
		px, py, ok := mapPt(cx, cy)
		if !ok {
			return drawClip{}, false
		}
		minX, maxX = min(minX, px), max(maxX, px)
		minY, maxY = min(minY, py), max(maxY, py)
	}
	return drawClip{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}, true
}

// affineAt returns the flat (affine) map that best matches m near
// (x, y): it maps (x, y) where m does, and it stretches and turns the
// area around that point the way m does there. The result is in Canvas2D
// order: x' = a·x + c·y + e, y' = b·x + d·y + f. ok is false when (x, y)
// lands at or behind the viewer. This is the canonical form of the
// derivative of (u/w, v/w); pdfAffineAt wraps it, and
// backend/internal/homography.AffineAt repeats it for the CPU backends,
// which cannot import package gui.
func (m *xfMat) affineAt(x, y float32) (a, b, c, d, e, f float32, ok bool) {
	w := m[6]*x + m[7]*y + m[8]
	if !(w > xfMinW) {
		return 0, 0, 0, 0, 0, 0, false
	}
	px := (m[0]*x + m[1]*y + m[2]) / w
	py := (m[3]*x + m[4]*y + m[5]) / w
	// The derivative of (u/w, v/w): (du - p·dw) / w.
	a = (m[0] - px*m[6]) / w // d px / d x
	c = (m[1] - px*m[7]) / w // d px / d y
	b = (m[3] - py*m[6]) / w // d py / d x
	d = (m[4] - py*m[7]) / w // d py / d y
	e = px - a*x - c*y
	f = py - b*x - d*y
	return a, b, c, d, e, f, true
}

// axisAligned reports whether m only moves and scales (a mirror counts),
// or does that after a quarter turn: a rect then maps to a rect. The
// off-axis terms are compared against the scale, so float trig noise
// from a turn of 360° does not count as a turn.
func (m *xfMat) axisAligned() bool {
	if m[6] != 0 || m[7] != 0 {
		return false
	}
	// Moves and scales only, or a quarter turn (the axes swap).
	tol := 1e-6 * (f32Abs(m[0]) + f32Abs(m[4]))
	if f32Abs(m[1]) <= tol && f32Abs(m[3]) <= tol {
		return true
	}
	tol = 1e-6 * (f32Abs(m[1]) + f32Abs(m[3]))
	return f32Abs(m[0]) <= tol && f32Abs(m[4]) <= tol
}

// mat4 is a row-major 4x4 matrix in float64, used only while resolve
// builds a transform.
type mat4 [16]float64

// mul returns a*b.
func (a *mat4) mul(b *mat4) mat4 {
	var out mat4
	for r := range 4 {
		for c := range 4 {
			var s float64
			for k := range 4 {
				s += a[r*4+k] * b[k*4+c]
			}
			out[r*4+c] = s
		}
	}
	return out
}

// mat4Identity returns the 4x4 identity.
func mat4Identity() mat4 { return mat4{0: 1, 5: 1, 10: 1, 15: 1} }

// mat4Translate returns a translation by (x, y).
func mat4Translate(x, y float64) mat4 {
	m := mat4Identity()
	m[3], m[7] = x, y
	return m
}

// resolve returns the homography that t applies to a view whose box is at
// (x, y) with size (w, h), in window coordinates. backHidden is true when
// the view asked for BackfaceHidden and its back now faces the viewer.
func (t Transform) resolve(x, y, w, h float32) (m xfMat, backHidden bool) {
	if t.IsIdentity() {
		return xfIdentity, false
	}
	const rad = math.Pi / 180
	ox := float64(x) + float64(w)*(0.5+float64(t.originX))
	oy := float64(y) + float64(h)*(0.5+float64(t.originY))

	// Build M = T(origin) T(tx,ty) P Rx Ry Rz K S T(-origin), outermost
	// first, so a point goes through S first and T(origin) last.
	acc := mat4Translate(ox+float64(t.tx), oy+float64(t.ty))
	if t.perspInv != 0 {
		// CSS perspective(d): w' = w - z/d.
		p := mat4Identity()
		p[14] = -float64(t.perspInv)
		acc = acc.mul(&p)
	}
	if t.rotX != 0 {
		s, c := math.Sincos(float64(t.rotX) * rad)
		r := mat4Identity()
		r[5], r[6], r[9], r[10] = c, -s, s, c
		acc = acc.mul(&r)
	}
	if t.rotY != 0 {
		s, c := math.Sincos(float64(t.rotY) * rad)
		r := mat4Identity()
		r[0], r[2], r[8], r[10] = c, s, -s, c
		acc = acc.mul(&r)
	}
	if t.rotZ != 0 {
		// The standard rotation matrix turns clockwise on a y-down screen,
		// the same sense as QuarterTurns and gpu.ApplyRotation.
		s, c := math.Sincos(float64(t.rotZ) * rad)
		r := mat4Identity()
		r[0], r[1], r[4], r[5] = c, -s, s, c
		acc = acc.mul(&r)
	}
	kx := math.Tan(float64(t.skewX) * rad)
	ky := math.Tan(float64(t.skewY) * rad)
	if kx != 0 || ky != 0 {
		k := mat4Identity()
		k[1], k[4] = kx, ky
		acc = acc.mul(&k)
	}
	sx := 1 + float64(t.scaleXm1)
	sy := 1 + float64(t.scaleYm1)
	if sx != 1 || sy != 1 {
		s := mat4Identity()
		s[0], s[5] = sx, sy
		acc = acc.mul(&s)
	}
	back := mat4Translate(-ox, -oy)
	acc = acc.mul(&back)

	// Flatten: keep rows and columns 0, 1 and 3 (x, y, w).
	m = xfMat{
		float32(acc[0]), float32(acc[1]), float32(acc[3]),
		float32(acc[4]), float32(acc[5]), float32(acc[7]),
		float32(acc[12]), float32(acc[13]), float32(acc[15]),
	}

	if t.backHidden {
		// CSS: the back face shows when m33 of the inverse of the 4x4 is
		// negative. m33 of the inverse is cofactor(2,2)/det(M), and
		// cofactor(2,2) is exactly det of the flattened 3x3. det(M) is the
		// product of the parts' determinants: translations, perspective and
		// rotations have det 1, the skew 1-kx*ky, the scale sx*sy. So the
		// test is a sign comparison, and a mirror (det(M) < 0 with
		// det(m) < 0) stays a front face.
		detM := (1 - kx*ky) * sx * sy
		detH := acc[0]*(acc[5]*acc[15]-acc[7]*acc[13]) -
			acc[1]*(acc[4]*acc[15]-acc[7]*acc[12]) +
			acc[3]*(acc[4]*acc[13]-acc[5]*acc[12])
		// An edge-on view (rotateY(90)) has detH ≈ 0, and the sign of that
		// residue depends on trig rounding. Treat it as a front face so the
		// result does not flip between platforms. detH scales with detM
		// (it is about detM·cosθ), so the cutoff is relative: a flipped card
		// that shrinks toward Scale(0) must stay hidden.
		const edgeOn = 1e-9
		backHidden = detM != 0 && math.Abs(detH) > edgeOn*math.Abs(detM) &&
			detH/detM < 0
	}
	return m, backHidden
}
