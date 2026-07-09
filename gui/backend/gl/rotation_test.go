//go:build !js && !darwin

package gl

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

func identityMat() [16]float32 {
	var m [16]float32
	m[0] = 1
	m[5] = 1
	m[10] = 1
	m[15] = 1
	return m
}

func approxEq(a, b [16]float32, tol float32) bool {
	for i := range 16 {
		if diff := a[i] - b[i]; diff < -tol || diff > tol {
			return false
		}
	}
	return true
}

func TestMat4MulIdentity(t *testing.T) {
	id := identityMat()
	a := [16]float32{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}
	var out [16]float32
	gpu.Mat4Mul(&out, &a, &id)
	if out != a {
		t.Fatalf("A * I != A\ngot  %v\nwant %v", out, a)
	}
	gpu.Mat4Mul(&out, &id, &a)
	if out != a {
		t.Fatalf("I * A != A\ngot  %v\nwant %v", out, a)
	}
}

func TestMat4MulKnownProduct(t *testing.T) {
	a := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		2, 3, 4, 1,
	}
	b := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		5, 6, 7, 1,
	}
	want := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		7, 9, 11, 1,
	}
	var out [16]float32
	gpu.Mat4Mul(&out, &a, &b)
	if out != want {
		t.Fatalf("translation compose failed\ngot  %v\nwant %v",
			out, want)
	}
}

func TestApplyRotationZero(t *testing.T) {
	mvp := identityMat()
	orig := mvp
	gpu.ApplyRotation(&mvp, 0, 100, 200)
	if !approxEq(mvp, orig, 1e-6) {
		t.Fatalf("0-deg rotation changed MVP\ngot  %v\nwant %v",
			mvp, orig)
	}
}

func TestApplyRotation360(t *testing.T) {
	mvp := identityMat()
	orig := mvp
	gpu.ApplyRotation(&mvp, 360, 50, 50)
	if !approxEq(mvp, orig, 1e-5) {
		t.Fatalf("360-deg rotation changed MVP\ngot  %v\nwant %v",
			mvp, orig)
	}
}

func TestApplyRotation90(t *testing.T) {
	mvp := identityMat()
	gpu.ApplyRotation(&mvp, 90, 0, 0)
	const tol = 1e-6
	if diff := mvp[0] - 0; diff < -tol || diff > tol {
		t.Fatalf("mvp[0] = %v, want ~0", mvp[0])
	}
	if diff := mvp[1] - 1; diff < -tol || diff > tol {
		t.Fatalf("mvp[1] = %v, want ~1", mvp[1])
	}
	if diff := mvp[4] - (-1); diff < -tol || diff > tol {
		t.Fatalf("mvp[4] = %v, want ~-1", mvp[4])
	}
	if diff := mvp[5] - 0; diff < -tol || diff > tol {
		t.Fatalf("mvp[5] = %v, want ~0", mvp[5])
	}
}

func TestApplyRotation180(t *testing.T) {
	mvp := identityMat()
	gpu.ApplyRotation(&mvp, 180, 0, 0)
	const tol = 1e-5
	if diff := mvp[0] - (-1); diff < -tol || diff > tol {
		t.Fatalf("mvp[0] = %v, want ~-1", mvp[0])
	}
	if diff := mvp[5] - (-1); diff < -tol || diff > tol {
		t.Fatalf("mvp[5] = %v, want ~-1", mvp[5])
	}
}

func TestApplyRotationCenterOffset(t *testing.T) {
	mvp := identityMat()
	gpu.ApplyRotation(&mvp, 90, 10, 0)
	const tol = 1e-5
	wantTx := float32(10)
	wantTy := float32(-10)
	if diff := mvp[12] - wantTx; diff < -tol || diff > tol {
		t.Fatalf("tx = %v, want %v", mvp[12], wantTx)
	}
	if diff := mvp[13] - wantTy; diff < -tol || diff > tol {
		t.Fatalf("ty = %v, want %v", mvp[13], wantTy)
	}
	_ = math.Pi
}
