//go:build js && wasm

package web

import (
	"math"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/homography"
)

// affine is a Canvas2D transform in logical pixels, in setTransform
// order: x' = a·x + c·y + e, y' = b·x + d·y + f.
type affine [6]float64

var affineIdentity = affine{1, 0, 0, 1, 0, 0}

// mul returns m·n: the map that applies n first, then m.
func (m affine) mul(n affine) affine {
	return affine{
		m[0]*n[0] + m[2]*n[1],
		m[1]*n[0] + m[3]*n[1],
		m[0]*n[2] + m[2]*n[3],
		m[1]*n[2] + m[3]*n[3],
		m[0]*n[4] + m[2]*n[5] + m[4],
		m[1]*n[4] + m[3]*n[5] + m[5],
	}
}

// The canvas transform is kept here, not on the canvas save stack.
// rebuildClipStack restores every clip save to rebuild the clips, and a
// transform saved above them would be popped with them. So rotation and
// view transform brackets push their matrix on xfStack, and every
// change, and every clip rebuild, sets the canvas transform from xfCur.

// setCanvasTransform sets the canvas to m, in logical pixels, on top of
// the DPI scale.
func (b *Backend) setCanvasTransform(m affine) {
	s := float64(b.dpiScale)
	if s <= 0 {
		s = 1
	}
	b.ctx2d.Call("setTransform",
		s*m[0], s*m[1], s*m[2], s*m[3], s*m[4], s*m[5])
}

// xfSaved is the transform state from before one open bracket.
type xfSaved struct {
	aff, view affine
	h         homography.Mat
	persp     bool
}

// pushTransform composes m into the current transform until the
// matching popTransform.
func (b *Backend) pushTransform(m affine) {
	b.xfStack = append(b.xfStack, xfSaved{b.xfCur, b.xfView, b.xfH, b.xfPersp})
	b.xfCur = b.xfCur.mul(m)
	b.setCanvasTransform(b.xfCur)
}

// pushViewTransform is pushTransform for a view transform or rotation
// bracket. h is the true matrix m stands in for. gui maps clips out to
// the window through both, so h and m are also composed on their own
// (xfH, xfView) to map those clips back. A DrawCanvas rotation opens
// no clip inside, so composing it too changes nothing.
func (b *Backend) pushViewTransform(m affine, h *homography.Mat) {
	b.pushTransform(m)
	b.xfView = b.xfView.mul(m)
	b.xfH = homography.Mul(&b.xfH, h)
	b.xfPersp = b.xfPersp || h[6] != 0 || h[7] != 0
}

// popTransform restores the transform from before the last push. A
// rect clip set under perspective was drawn in the view's frame, so it
// is set again from its window rect.
func (b *Backend) popTransform() {
	n := len(b.xfStack)
	if n == 0 {
		return
	}
	wasPersp := b.xfPersp
	top := b.xfStack[n-1]
	b.xfCur, b.xfView, b.xfH, b.xfPersp = top.aff, top.view, top.h, top.persp
	b.xfStack = b.xfStack[:n-1]
	b.setCanvasTransform(b.xfCur)
	if wasPersp {
		for i := range slices.Backward(b.clipStack) {
			if b.clipStack[i].kind == clipKindRect {
				b.clipStack[i] = b.rectClip(b.clipStack[i].win)
				b.rebuildClipStack()
				break
			}
		}
	}
}

// rectClip builds the clip region for a RenderClip window rect. gui
// maps a clip inside a transform to the window through the true matrix.
// With no perspective open, the canvas draws through that same matrix,
// so the window rect is exact and is drawn with no transform. Under
// perspective, the canvas draws through a flat stand-in, which puts the
// far side of a view somewhere else than the true matrix does. So the
// rect is mapped back into the view's frame through the true matrix,
// and drawn there under the stand-in, as the content is. The box of the
// mapped rect is bigger than the clip; gui adds a stencil there, which
// cuts exactly. Rotations are in the map on both sides, as gui puts
// quarter turns in it.
func (b *Backend) rectClip(win [4]float32) clipRegion {
	c := clipRegion{kind: clipKindRect, win: win, xf: affineIdentity,
		x: win[0], y: win[1], w: win[2], h: win[3]}
	if !b.xfPersp || !(win[2] > 0 && win[3] > 0) {
		return c
	}
	inv, ok := homography.Inverse(&b.xfH)
	if !ok {
		return c
	}
	x, y, w, h, okb := homography.Bounds(&inv, win[0], win[1], win[2], win[3])
	if !okb {
		// Part of the clip maps to behind the viewer: no finite rect
		// holds it in the view's frame. Keep the window rect.
		return c
	}
	c.x, c.y, c.w, c.h, c.xf = x, y, w, h, b.xfView
	return c
}

// resetTransforms drops every open bracket, at the start of a frame.
func (b *Backend) resetTransforms() {
	b.xfStack = b.xfStack[:0]
	b.xfCur = affineIdentity
	b.xfView = affineIdentity
	b.xfH = homography.Identity
	b.xfPersp = false
}

func (b *Backend) beginRotation(r *gui.RenderCmd) {
	rad := float64(r.RotAngle) * math.Pi / 180
	sin, cos := math.Sincos(rad)
	cx, cy := float64(r.RotCX), float64(r.RotCY)
	// Turn about (cx, cy): move it to the origin, turn, move it back.
	m := affine{
		cos, sin, -sin, cos,
		cx - cos*cx + sin*cy,
		cy - sin*cx - cos*cy,
	}
	// gui maps a clip inside a quarter turn out to the window through
	// the turn (#1019), so the turn joins the clip map too. A turn has
	// no perspective, so its flat map is its true matrix.
	h := homography.Mat{float32(m[0]), float32(m[2]), float32(m[4]),
		float32(m[1]), float32(m[3]), float32(m[5]), 0, 0, 1}
	b.pushViewTransform(m, &h)
}

func (b *Backend) endRotation() {
	b.popTransform()
}

// beginTransform opens a view transform bracket (#1019). Canvas2D has
// only flat (affine) transforms, so a matrix with perspective is drawn
// with the affine that matches it at the center of the view's box: the
// center lands where the GPU puts it, and the view is stretched and
// turned there as on the GPU, but its far side does not shrink. A
// center at or behind the viewer draws nothing.
func (b *Backend) beginTransform(r *gui.RenderCmd) {
	h := homography.Identity
	if r.Homography != nil {
		h = *r.Homography
	}
	pa, pb, pc, pd, pe, pf, ok := homography.AffineAt(&h,
		r.X+r.W/2, r.Y+r.H/2)
	if !ok {
		// A zero matrix flattens everything to a point: nothing draws.
		b.pushViewTransform(affine{}, &h)
		return
	}
	b.pushViewTransform(affine{float64(pa), float64(pb), float64(pc),
		float64(pd), float64(pe), float64(pf)}, &h)
}

func (b *Backend) endTransform() {
	b.popTransform()
}
