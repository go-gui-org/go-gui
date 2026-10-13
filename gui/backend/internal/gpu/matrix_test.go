package gpu

import (
	"math"
	"testing"
)

func TestIdentityTM(t *testing.T) {
	m := IdentityTM()
	for i := range 16 {
		if i%5 == 0 {
			if m[i] != 1 {
				t.Errorf("IdentityTM[%d] = %v, want 1", i, m[i])
			}
		} else {
			if m[i] != 0 {
				t.Errorf("IdentityTM[%d] = %v, want 0", i, m[i])
			}
		}
	}
}

func TestOrtho(t *testing.T) {
	var m [16]float32
	Ortho(&m, 0, 800, 600, 0, -1, 1)
	// m[0] = 2/(800-0) = 0.0025
	want00 := float32(2.0 / 800.0)
	if m[0] != want00 {
		t.Errorf("m[0] = %v, want %v", m[0], want00)
	}
	// m[5] = 2/(0-600) = -0.00333...
	want05 := float32(2.0 / -600.0)
	if m[5] != want05 {
		t.Errorf("m[5] = %v, want %v", m[5], want05)
	}
	// m[12] = -(800+0)/(800-0) = -1
	if m[12] != -1 {
		t.Errorf("m[12] = %v, want -1", m[12])
	}
	// m[13] = -(0+600)/(0-600) = 1
	if m[13] != 1 {
		t.Errorf("m[13] = %v, want 1", m[13])
	}
	// m[15] = 1
	if m[15] != 1 {
		t.Errorf("m[15] = %v, want 1", m[15])
	}
}

func TestApplyRotation90(t *testing.T) {
	mvp := IdentityTM()
	ApplyRotation(&mvp, 90, 0, 0)
	// cos90 ≈ 0, sin90 ≈ 1
	// rot[0]=cosA≈0, rot[1]=sinA≈1, rot[4]=-sinA≈-1, rot[5]=cosA≈0
	// After Mat4Mul(mvp * rot): mvp[0] ≈ 0, mvp[1] ≈ 1
	if math.Abs(float64(mvp[0])) > 1e-6 {
		t.Errorf("mvp[0] = %v, want ~0", mvp[0])
	}
	if math.Abs(float64(mvp[1]-1)) > 1e-6 {
		t.Errorf("mvp[1] = %v, want ~1", mvp[1])
	}
}

func TestMat4MulIdentity(t *testing.T) {
	a := IdentityTM()
	b := IdentityTM()
	var out [16]float32
	Mat4Mul(&out, &a, &b)
	if out != IdentityTM() {
		t.Errorf("identity * identity != identity")
	}
}

func TestMat4MulKnown(t *testing.T) {
	// a = translation by (3, 4, 0), b = scale by (2, 2, 1)
	a := [16]float32{0: 1, 5: 1, 10: 1, 12: 3, 13: 4, 15: 1}
	b := [16]float32{0: 2, 5: 2, 10: 1, 15: 1}
	var out [16]float32
	Mat4Mul(&out, &a, &b)
	// Column-major: out = a * b
	// col 0: (2,0,0,0)
	if out[0] != 2 || out[1] != 0 || out[2] != 0 || out[3] != 0 {
		t.Errorf("col0 = [%v,%v,%v,%v], want [2,0,0,0]",
			out[0], out[1], out[2], out[3])
	}
	// col 1: (0,2,0,0)
	if out[4] != 0 || out[5] != 2 || out[6] != 0 || out[7] != 0 {
		t.Errorf("col1 = [%v,%v,%v,%v], want [0,2,0,0]",
			out[4], out[5], out[6], out[7])
	}
	// col 3: (3,4,0,1) — translation preserved
	if out[12] != 3 || out[13] != 4 || out[14] != 0 || out[15] != 1 {
		t.Errorf("col3 = [%v,%v,%v,%v], want [3,4,0,1]",
			out[12], out[13], out[14], out[15])
	}
}

func TestOrtho_DegenerateViewport_ReturnsIdentity(t *testing.T) {
	tests := []struct {
		name             string
		l, r, b, t, n, f float32
	}{
		{"zero width", 100, 100, 0, 100, -1, 1},
		{"zero height", 0, 100, 50, 50, -1, 1},
		{"zero depth", 0, 100, 0, 100, 5, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var m [16]float32
			Ortho(&m, tc.l, tc.r, tc.b, tc.t, tc.n, tc.f)
			if m != IdentityTM() {
				t.Errorf("got %v, want identity", m)
			}
		})
	}
}

// applyMVP maps a 2D point through a column-major MVP, with the
// perspective divide the GPU does.
func applyMVP(m *[16]float32, x, y float32) (float32, float32) {
	cx := m[0]*x + m[4]*y + m[12]
	cy := m[1]*x + m[5]*y + m[13]
	cw := m[3]*x + m[7]*y + m[15]
	return cx / cw, cy / cw
}

func TestApplyHomographyIdentity(t *testing.T) {
	mvp := IdentityTM()
	h := [9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1}
	ApplyHomography(&mvp, &h, 2)
	if mvp != IdentityTM() {
		t.Fatalf("identity changed the MVP: %v", mvp)
	}
}

func TestApplyHomographyScalesToPhysicalPixels(t *testing.T) {
	// A move of (5, 7) logical pixels is (10, 14) physical at 2x.
	mvp := IdentityTM()
	h := [9]float32{1, 0, 5, 0, 1, 7, 0, 0, 1}
	ApplyHomography(&mvp, &h, 2)
	x, y := applyMVP(&mvp, 20, 30)
	if x != 30 || y != 44 {
		t.Fatalf("got (%v,%v), want (30,44)", x, y)
	}
}

func TestApplyHomographyPerspectiveDivide(t *testing.T) {
	// w = 0.01*x + 1 in logical pixels. At x = 100 logical (200
	// physical at 2x) w is 2, so the point lands at half its distance.
	mvp := IdentityTM()
	h := [9]float32{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	ApplyHomography(&mvp, &h, 2)
	x, y := applyMVP(&mvp, 200, 40)
	if math.Abs(float64(x-100)) > 1e-4 || math.Abs(float64(y-20)) > 1e-4 {
		t.Fatalf("got (%v,%v), want (100,20)", x, y)
	}
}
