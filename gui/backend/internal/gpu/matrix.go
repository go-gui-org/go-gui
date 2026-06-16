package gpu

import "math"

// IdentityTM returns a 4×4 column-major identity matrix.
func IdentityTM() [16]float32 {
	return [16]float32{0: 1, 5: 1, 10: 1, 15: 1}
}

// Ortho builds a column-major orthographic projection matrix.
// Degenerate dimensions (zero-area viewport) produce identity to
// avoid NaN/Inf in the MVP.
func Ortho(m *[16]float32, l, r, b, t, n, f float32) {
	if r == l || t == b || f == n {
		*m = IdentityTM()
		return
	}
	m[0] = 2 / (r - l)
	m[1] = 0
	m[2] = 0
	m[3] = 0

	m[4] = 0
	m[5] = 2 / (t - b)
	m[6] = 0
	m[7] = 0

	m[8] = 0
	m[9] = 0
	m[10] = -2 / (f - n)
	m[11] = 0

	m[12] = -(r + l) / (r - l)
	m[13] = -(t + b) / (t - b)
	m[14] = -(f + n) / (f - n)
	m[15] = 1
}

// ApplyRotation multiplies mvp by a rotation matrix that rotates
// angleDeg degrees around the point (cx, cy).
func ApplyRotation(mvp *[16]float32, angleDeg, cx, cy float32) {
	rad := float64(angleDeg) * math.Pi / 180
	cosA := float32(math.Cos(rad))
	sinA := float32(math.Sin(rad))
	tx := cx*(1-cosA) + cy*sinA
	ty := cy*(1-cosA) - cx*sinA
	// Column-major rotation around (cx, cy).
	var rot [16]float32
	rot[0] = cosA
	rot[1] = sinA
	rot[4] = -sinA
	rot[5] = cosA
	rot[10] = 1
	rot[12] = tx
	rot[13] = ty
	rot[15] = 1
	var out [16]float32
	Mat4Mul(&out, mvp, &rot)
	*mvp = out
}

// Mat4Mul multiplies two 4×4 column-major matrices: out = a * b.
func Mat4Mul(out, a, b *[16]float32) {
	for col := range 4 {
		for row := range 4 {
			var sum float32
			for k := range 4 {
				sum += a[k*4+row] * b[col*4+k]
			}
			out[col*4+row] = sum
		}
	}
}
