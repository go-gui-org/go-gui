package soft

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// xfBegin opens a view transform bracket with matrix h.
func xfBegin(h [9]float32) gui.RenderCmd {
	return gui.RenderCmd{Kind: gui.RenderTransformBegin, Homography: &h}
}

func clipCmd(x, y, w, h float32) gui.RenderCmd {
	return gui.RenderCmd{Kind: gui.RenderClip, X: x, Y: y, W: w, H: h}
}

var green = gui.RGB(0, 255, 0)

func TestTransformMovesContent(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 15, 0, 1, 0, 0, 0, 1}),
		rectCmd(0, 10, 10, 10, green),
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 20, 15); g != 255 {
		t.Errorf("moved rect green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 5, 15); g != 0 {
		t.Errorf("old place green = %d, want 0", g)
	}
}

// The matrix is in logical pixels; the renderer draws in physical ones.
func TestTransformScalesMatrixToDevice(t *testing.T) {
	r := newRenderer(40, 40, 2)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 5, 0, 1, 0, 0, 0, 1}),
		rectCmd(0, 0, 5, 5, green), // device 0..10, moved to 10..20
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 15, 5); g != 255 {
		t.Errorf("moved rect green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 5, 5); g != 0 {
		t.Errorf("old place green = %d, want 0", g)
	}
}

// A RenderClip is a window rect, as a GPU scissor is, even inside a
// transform: gui maps it out through the transform before it emits it.
func TestTransformClipIsInWindowSpace(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 20, 0, 1, 0, 0, 0, 1}),
		clipCmd(20, 0, 10, 40), // local 0..10
		rectCmd(0, 10, 20, 10, green),
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 25, 15); g != 255 {
		t.Errorf("inside clip green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 35, 15); g != 0 {
		t.Errorf("outside clip green = %d, want 0", g)
	}
}

// The clip in effect when the bracket opens cuts everything drawn in it.
func TestTransformCutByEntryClip(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		clipCmd(0, 0, 25, 40),
		xfBegin([9]float32{1, 0, 10, 0, 1, 0, 0, 0, 1}),
		rectCmd(0, 10, 20, 10, green), // window 10..30
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 20, 15); g != 255 {
		t.Errorf("inside entry clip green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 27, 15); g != 0 {
		t.Errorf("outside entry clip green = %d, want 0", g)
	}
}

// The last clip set inside the bracket stays in effect after it, in the
// window, as a GPU scissor does.
func TestTransformClipCarriesPastEnd(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 20, 0, 1, 0, 0, 0, 1}),
		clipCmd(0, 0, 10, 40),
		{Kind: gui.RenderTransformEnd},
		rectCmd(0, 10, 40, 10, green),
	})
	if _, g, _, _ := at(r.buf.img, 5, 15); g != 255 {
		t.Errorf("inside clip green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 25, 15); g != 0 {
		t.Errorf("outside clip green = %d, want 0", g)
	}
}

// Nested brackets compose: the inner matrix maps to the outer frame.
func TestTransformNests(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 10, 0, 1, 0, 0, 0, 1}),
		xfBegin([9]float32{1, 0, 0, 0, 1, 15, 0, 0, 1}),
		rectCmd(0, 0, 5, 5, green),
		{Kind: gui.RenderTransformEnd},
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 12, 17); g != 255 {
		t.Errorf("moved rect green = %d, want 255", g)
	}
}

// Perspective is exact: a turn about y with depth makes the near side
// taller than the far side.
func TestTransformPerspective(t *testing.T) {
	r := newRenderer(60, 60, 1)
	// w grows with x: the right side is farther, so it shrinks toward
	// the y = 0 line.
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 0, 0, 1, 0, 0.02, 0, 1}),
		rectCmd(0, 0, 40, 40, green),
		{Kind: gui.RenderTransformEnd},
	})
	// x = 10 in local space lands at 10/1.2 ≈ 8.3 with height 40/1.2.
	if _, g, _, _ := at(r.buf.img, 8, 30); g < 200 {
		t.Errorf("near column green = %d, want near 255", g)
	}
	// Local x = 40 lands at 40/1.8 ≈ 22 with height 40/1.8 ≈ 22.
	if _, g, _, _ := at(r.buf.img, 21, 30); g > 40 {
		t.Errorf("far column below its height green = %d, want ~0", g)
	}
}

// A matrix that sends every point behind the viewer draws nothing and
// does not panic.
func TestTransformBehindViewerDrawsNothing(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 0, 0, 1, 0, 0, 0, -1}),
		rectCmd(0, 0, 40, 40, green),
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 20, 20); g != 0 {
		t.Errorf("green = %d, want 0", g)
	}
}

// A stream cut off inside a bracket still lands its content.
func TestTransformUnclosedFinishes(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 15, 0, 1, 0, 0, 0, 1}),
		rectCmd(0, 10, 10, 10, green),
	})
	r.finishBrackets()
	if _, g, _, _ := at(r.buf.img, 20, 15); g != 255 {
		t.Errorf("moved rect green = %d, want 255", g)
	}
}

// Inside a transform an empty clip cuts everything, as an empty GPU
// scissor does: gui emits one when a clip cuts the view away.
func TestTransformEmptyClipCutsAll(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		xfBegin([9]float32{1, 0, 5, 0, 1, 0, 0, 0, 1}),
		clipCmd(10, 10, 0, 0),
		rectCmd(0, 0, 30, 30, green),
		{Kind: gui.RenderTransformEnd},
	})
	if _, g, _, _ := at(r.buf.img, 15, 15); g != 0 {
		t.Errorf("green = %d, want 0", g)
	}
}

// A clip inside a quarter turn is a window rect: gui maps it out through
// the turn (#1019). The layer under the turn is in the unturned frame,
// so the rect is mapped back into it. A vertical strip in the window
// must stay vertical after the layer is turned back out.
func TestRotationClipIsInWindowSpace(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 20, RotCY: 20},
		clipCmd(16, 0, 8, 40),
		rectCmd(0, 0, 40, 40, green),
		{Kind: gui.RenderRotateEnd},
	})
	if _, g, _, _ := at(r.buf.img, 20, 5); g < 200 {
		t.Errorf("inside clip green = %d, want near 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 5, 20); g > 40 {
		t.Errorf("outside clip green = %d, want ~0", g)
	}
}

// A view transform inside a quarter turn maps its clip back through
// both: the turn is part of the frame the window rect came from.
func TestTransformInRotationClip(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 20, RotCY: 20},
		xfBegin([9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1}),
		clipCmd(16, 0, 8, 40),
		rectCmd(0, 0, 40, 40, green),
		{Kind: gui.RenderTransformEnd},
		{Kind: gui.RenderRotateEnd},
	})
	if _, g, _, _ := at(r.buf.img, 20, 5); g < 200 {
		t.Errorf("inside clip green = %d, want near 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 5, 20); g > 40 {
		t.Errorf("outside clip green = %d, want ~0", g)
	}
}
