//go:build js && wasm

package web

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/homography"
)

// fakeXfCtx2D is a JS object that logs the calls the transform and
// clip code makes. setTransform is logged with its six numbers rounded
// to two decimals.
func fakeXfCtx2D(t *testing.T, log *[]string) js.Value {
	t.Helper()
	obj := js.Global().Get("Object").New()
	var funcs []js.Func
	add := func(name string) {
		f := js.FuncOf(func(this js.Value, args []js.Value) any {
			parts := []string{name}
			for _, a := range args {
				parts = append(parts, fmt.Sprintf("%.2f", a.Float()))
			}
			*log = append(*log, strings.Join(parts, " "))
			return nil
		})
		funcs = append(funcs, f)
		obj.Set(name, f)
	}
	for _, n := range []string{"save", "restore", "setTransform",
		"beginPath", "rect", "clip"} {
		add(n)
	}
	t.Cleanup(func() {
		for _, f := range funcs {
			f.Release()
		}
	})
	return obj
}

func newXfBackend(t *testing.T, log *[]string) *Backend {
	b := &Backend{ctx2d: fakeXfCtx2D(t, log), dpiScale: 2}
	b.resetTransforms()
	return b
}

func lastSetTransform(log []string) string {
	for _, l := range slices.Backward(log) {
		if strings.HasPrefix(l, "setTransform") {
			return l
		}
	}
	return ""
}

func TestWebTransformSetsCanvasTransform(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	h := [9]float32{1, 0, 10, 0, 1, 5, 0, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h, W: 20, H: 20})
	// The DPI scale (2) sits on top of the logical matrix.
	if got, want := lastSetTransform(log),
		"setTransform 2.00 0.00 0.00 2.00 20.00 10.00"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	b.endTransform()
	if got, want := lastSetTransform(log),
		"setTransform 2.00 0.00 0.00 2.00 0.00 0.00"; got != want {
		t.Fatalf("after end got %q, want %q", got, want)
	}
}

// A rect clip is a window rect, as a GPU scissor is: it is drawn with
// no transform, and the transform comes back after it.
func TestWebClipInsideTransformIsWindowSpace(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	h := [9]float32{1, 0, 10, 0, 1, 0, 0, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h})
	log = log[:0]
	b.drawClip(&gui.RenderCmd{X: 1, Y: 2, W: 3, H: 4})
	want := []string{
		"save",
		"setTransform 2.00 0.00 0.00 2.00 0.00 0.00",
		"beginPath",
		"rect 1.00 2.00 3.00 4.00",
		"clip",
		"setTransform 2.00 0.00 0.00 2.00 20.00 0.00",
	}
	if strings.Join(log, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(log, "\n"),
			strings.Join(want, "\n"))
	}
}

// A stencil clip goes through the transform it opened under, as the GPU
// stencil quad goes through the MVP. It keeps that transform when a
// later rect clip rebuilds the stack.
func TestWebStencilInsideTransformKeepsTransform(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	h := [9]float32{1, 0, 10, 0, 1, 0, 0, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h})
	b.beginStencilClip(&gui.RenderCmd{X: 0, Y: 0, W: 5, H: 5})
	log = log[:0]
	b.drawClip(&gui.RenderCmd{X: 1, Y: 1, W: 9, H: 9})
	joined := strings.Join(log, "\n")
	if !strings.Contains(joined,
		"setTransform 2.00 0.00 0.00 2.00 20.00 0.00\nbeginPath\nrect 0.00 0.00 5.00 5.00") {
		t.Fatalf("stencil not drawn under its transform:\n%s", joined)
	}
}

// Regression: a clip inside a rotation used to restore the rotation's
// own canvas save, so everything after it drew unrotated.
func TestWebClipInsideRotationKeepsRotation(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	b.beginRotation(&gui.RenderCmd{RotAngle: 90})
	b.drawClip(&gui.RenderCmd{X: 0, Y: 0, W: 10, H: 10})
	if got, want := lastSetTransform(log),
		"setTransform 0.00 2.00 -2.00 0.00 0.00 0.00"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	for _, l := range log {
		if l == "restore" {
			t.Fatal("rotation must not touch the canvas save stack")
		}
	}
}

// Perspective is drawn with the affine that matches the matrix at the
// box center: the center lands where the GPU puts it.
func TestWebTransformPerspectiveMatchesCenter(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	b.dpiScale = 1
	h := [9]float32{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	cmd := gui.RenderCmd{Homography: &h, X: 0, Y: 0, W: 100, H: 40}
	b.beginTransform(&cmd)
	m := b.xfCur
	cx, cy := 50.0, 20.0
	gx, gy := cx/1.5, cy/1.5 // w = 1 + 0.01·50
	ax := m[0]*cx + m[2]*cy + m[4]
	ay := m[1]*cx + m[3]*cy + m[5]
	if math.Abs(ax-gx) > 1e-3 || math.Abs(ay-gy) > 1e-3 {
		t.Fatalf("center at %v,%v, want %v,%v", ax, ay, gx, gy)
	}
}

// A center behind the viewer draws nothing: the canvas matrix is zero.
func TestWebTransformBehindViewerDrawsNothing(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	h := [9]float32{1, 0, 0, 0, 1, 0, 0, 0, -1}
	b.beginTransform(&gui.RenderCmd{Homography: &h, W: 10, H: 10})
	if b.xfCur != (affine{}) {
		t.Fatalf("xfCur = %v, want zero", b.xfCur)
	}
	b.endTransform()
	if b.xfCur != affineIdentity {
		t.Fatalf("after end xfCur = %v, want identity", b.xfCur)
	}
}

// Regression: under perspective, gui maps a clip to the window through
// the true matrix, but web draws through the flat stand-in. A window
// rect clip then cut off the far side of the view. The clip must be
// drawn in the view's frame, under the same flat map as the content.
func TestWebClipUnderPerspectiveUsesFlatMap(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	b.dpiScale = 1
	h := [9]float32{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h, W: 100, H: 40})
	// The view's box (0,0,100,40) mapped through the true matrix.
	wx, wy, ww, wh, _ := homography.Bounds(&h, 0, 0, 100, 40)
	log = log[:0]
	b.drawClip(&gui.RenderCmd{X: wx, Y: wy, W: ww, H: wh})
	clip := b.clipStack[len(b.clipStack)-1]
	if clip.xf != b.xfCur {
		t.Fatalf("clip xf = %v, want the flat map %v", clip.xf, b.xfCur)
	}
	// In the view's frame, the clip must hold the whole box.
	if clip.x > 0.01 || clip.y > 0.01 || clip.x+clip.w < 99.99 ||
		clip.y+clip.h < 39.99 {
		t.Fatalf("clip %v,%v %vx%v does not hold the box", clip.x,
			clip.y, clip.w, clip.h)
	}
}

// After the bracket closes, the clip set inside it is a window rect
// again, drawn with no transform.
func TestWebClipAfterPerspectiveEndIsWindowRect(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	b.dpiScale = 1
	h := [9]float32{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h, W: 100, H: 40})
	b.drawClip(&gui.RenderCmd{X: 0, Y: 0, W: 320, H: 240})
	b.endTransform()
	clip := b.clipStack[len(b.clipStack)-1]
	if clip.xf != affineIdentity || clip.x != 0 || clip.w != 320 {
		t.Fatalf("clip after end = %+v, want window rect", clip)
	}
}

// A rotation inside a perspective view is part of the clip map: gui
// maps a clip out through quarter turns too (#1019).
func TestWebClipUnderPerspectiveKeepsRotation(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	h := [9]float32{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h, W: 100, H: 40})
	b.beginRotation(&gui.RenderCmd{RotAngle: 90})
	b.drawClip(&gui.RenderCmd{X: 0, Y: 0, W: 50, H: 20})
	if clip := b.clipStack[len(b.clipStack)-1]; clip.xf != b.xfCur {
		t.Fatalf("clip xf = %v, want the turned view map %v", clip.xf, b.xfCur)
	}
}

// A perspective view inside a quarter turn: the clip gui mapped out
// through the turn and the view's matrix maps back through both, so in
// the view's frame it starts at the view's corner.
func TestWebClipUnderPerspectiveInRotation(t *testing.T) {
	var log []string
	b := newXfBackend(t, &log)
	b.dpiScale = 1
	b.beginRotation(&gui.RenderCmd{RotAngle: 90, RotCX: 50, RotCY: 20})
	h := [9]float32{1, 0, 0, 0, 1, 0, 0.01, 0, 1}
	b.beginTransform(&gui.RenderCmd{Homography: &h, W: 100, H: 40})
	// Turn 90° about (50, 20): (x, y) -> (70 - y, x - 30).
	turn := homography.Mat{0, -1, 70, 1, 0, -30, 0, 0, 1}
	acc := homography.Mul(&turn, &h)
	wx, wy, ww, wh, _ := homography.Bounds(&acc, 0, 0, 100, 40)
	b.drawClip(&gui.RenderCmd{X: wx, Y: wy, W: ww, H: wh})
	clip := b.clipStack[len(b.clipStack)-1]
	if clip.xf != b.xfCur || clip.x < -0.01 || clip.x > 0.01 ||
		clip.y < -0.01 || clip.y > 0.01 || clip.x+clip.w < 99.99 ||
		clip.y+clip.h < 39.99 {
		t.Fatalf("clip %+v does not hold the view under its flat map", clip)
	}
}
