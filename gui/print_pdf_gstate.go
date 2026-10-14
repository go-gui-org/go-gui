package gui

import (
	"math"
	"slices"

	"github.com/go-pdf/fpdf"
)

// PDF clips, rotations and view transforms (#1019).
//
// fpdf opens each clip and each transform with a PDF "q" and ends it
// with a "Q", and a Q restores the newest q, whichever kind opened it.
// So a RenderClip that ends the old clip inside a rotation used to end
// the rotation instead. pdfGState keeps the clips and transforms in Go
// and, on every change, ends all its q's and opens them again in order:
// each clip under the transform it belongs to, then the transform the
// content draws with. The web backend does the same with the canvas
// save stack.

// pdfAffine is a flat transform in setTransform / PDF "cm" order:
// x' = a·x + c·y + e, y' = b·x + d·y + f.
type pdfAffine [6]float64

var pdfAffineIdentity = pdfAffine{1, 0, 0, 1, 0, 0}

// mul returns m·n: the map that applies n first, then m.
func (m pdfAffine) mul(n pdfAffine) pdfAffine {
	return pdfAffine{
		m[0]*n[0] + m[2]*n[1],
		m[1]*n[0] + m[3]*n[1],
		m[0]*n[2] + m[2]*n[3],
		m[1]*n[2] + m[3]*n[3],
		m[0]*n[4] + m[2]*n[5] + m[4],
		m[1]*n[4] + m[3]*n[5] + m[5],
	}
}

// inverse returns the inverse of m. ok is false when m flattens the
// plane to a line or a point.
func (m pdfAffine) inverse() (pdfAffine, bool) {
	det := m[0]*m[3] - m[1]*m[2]
	if det == 0 || det != det {
		return pdfAffine{}, false
	}
	return pdfAffine{
		m[3] / det, -m[1] / det, -m[2] / det, m[0] / det,
		(m[2]*m[5] - m[3]*m[4]) / det,
		(m[1]*m[4] - m[0]*m[5]) / det,
	}, true
}

// pdfAffineAt returns the flat map that best matches h near (x, y): it
// maps (x, y) where h does, and stretches and turns the area around it
// as h does there. PDF has no perspective, so a view with perspective
// is drawn with this, as on the web backend. ok is false when (x, y)
// lands at or behind the viewer. It wraps xfMat.affineAt, widened to
// float64 for the PDF matrix.
func pdfAffineAt(h *xfMat, x, y float32) (pdfAffine, bool) {
	a, b, c, d, e, f, ok := h.affineAt(x, y)
	if !ok {
		return pdfAffine{}, false
	}
	return pdfAffine{float64(a), float64(b), float64(c), float64(d),
		float64(e), float64(f)}, true
}

// pdfGOps is the part of *fpdf.Fpdf that pdfGState drives. Tests pass a
// recorder.
type pdfGOps interface {
	TransformBegin()
	Transform(tm fpdf.TransformMatrix)
	TransformEnd()
	ClipRect(x, y, w, h float64, outline bool)
	ClipEnd()
}

type pdfClipKind uint8

const (
	pdfClipRect pdfClipKind = iota
	pdfClipStencil
)

// pdfClip is one open clip, in logical pixels, drawn under xf. A rect
// clip is a window rect, as a GPU scissor is, so xf is the identity;
// under perspective it is drawn in the view's frame (rectClip). A
// stencil goes through the transform it opened under, as the GPU
// stencil quad goes through the MVP. win is a rect clip's window rect,
// as gui sent it.
type pdfClip struct {
	x, y, w, h float32
	win        [4]float32
	xf         pdfAffine
	kind       pdfClipKind
}

// pdfXfSaved is the transform state from before one open bracket.
type pdfXfSaved struct {
	cur, view pdfAffine
	h         xfMat
	persp     bool
	views     int
}

// pdfGState holds the clips and transforms of one PDF page.
type pdfGState struct {
	ops pdfGOps
	ctx *pdfCtx

	// phi maps a logical point to PDF native space: points, y up, the
	// way fpdf places what px and py give it. A logical matrix M is
	// drawn with the PDF matrix phi·M·phi⁻¹.
	phi, phiInv pdfAffine

	// cur is the matrix the content draws with: every open rotation
	// and view transform. view composes them too, and h the true
	// matrices they stand in for. gui maps clips out to the window
	// through view transforms and quarter turns, so these map them back.
	// persp is set while a view transform with perspective is open.
	cur, view pdfAffine
	h         xfMat
	persp     bool
	views     int // open view transforms and rotations
	stack     []pdfXfSaved

	clips []pdfClip

	// opened lists the q's open now, oldest first: true for a clip
	// (ClipEnd), false for a transform (TransformEnd).
	opened []bool
}

func newPDFGState(ops pdfGOps, ctx *pdfCtx, k, pageH float64) *pdfGState {
	ks := k * float64(ctx.scale)
	phi := pdfAffine{ks, 0, 0, -ks, k * ctx.ox, k * (pageH - ctx.oy)}
	phiInv, _ := phi.inverse()
	return &pdfGState{
		ops: ops, ctx: ctx, phi: phi, phiInv: phiInv,
		cur: pdfAffineIdentity, view: pdfAffineIdentity, h: xfIdentity,
	}
}

// setClip applies a RenderClip. Each one replaces the rect clip. A
// degenerate rect ends it, except inside a view transform: there gui
// emits one for a clip that cuts the view away (xfFrame.toWindow), so
// it clips everything, as an empty GPU scissor does.
func (g *pdfGState) setClip(cmd *RenderCmd) {
	idx := -1
	for i := range slices.Backward(g.clips) {
		if g.clips[i].kind == pdfClipRect {
			idx = i
			break
		}
	}
	if !(cmd.W > 0 && cmd.H > 0) && g.views == 0 {
		if idx >= 0 {
			g.clips = slices.Delete(g.clips, idx, idx+1)
		}
		g.rebuild()
		return
	}
	c := g.rectClip([4]float32{cmd.X, cmd.Y, cmd.W, cmd.H})
	if idx >= 0 {
		g.clips[idx] = c
	} else {
		g.clips = append(g.clips, c)
	}
	g.rebuild()
}

// rectClip builds the clip for a RenderClip window rect. With no
// perspective open, PDF draws through the same matrix gui mapped the
// clip with, so the window rect is exact. Under perspective, PDF draws
// through a flat stand-in, which puts the far side of a view somewhere
// else. So the rect is mapped back into the view's frame through the
// true matrix and drawn there under the stand-in, as the content is.
func (g *pdfGState) rectClip(win [4]float32) pdfClip {
	c := pdfClip{kind: pdfClipRect, win: win, xf: pdfAffineIdentity,
		x: win[0], y: win[1], w: win[2], h: win[3]}
	if !g.persp || !(win[2] > 0 && win[3] > 0) {
		return c
	}
	inv, ok := g.h.inverse()
	if !ok {
		return c
	}
	f := xfFrame{acc: inv, clip: xfUnboundedClip}
	r := f.toWindow(drawClip{X: win[0], Y: win[1], Width: win[2], Height: win[3]})
	if r == xfUnboundedClip {
		// Part of the clip maps to behind the viewer: no finite rect
		// holds it in the view's frame. Keep the window rect.
		return c
	}
	c.x, c.y, c.w, c.h, c.xf = r.X, r.Y, r.Width, r.Height, g.view
	return c
}

func (g *pdfGState) beginStencil(cmd *RenderCmd) {
	g.clips = append(g.clips, pdfClip{kind: pdfClipStencil,
		x: cmd.X, y: cmd.Y, w: cmd.W, h: cmd.H, xf: g.cur})
	g.rebuild()
}

func (g *pdfGState) endStencil() {
	for i := range slices.Backward(g.clips) {
		if g.clips[i].kind == pdfClipStencil {
			g.clips = slices.Delete(g.clips, i, i+1)
			g.rebuild()
			return
		}
	}
}

// push composes m into the content matrix until the matching pop.
func (g *pdfGState) push(m pdfAffine) {
	g.stack = append(g.stack, pdfXfSaved{g.cur, g.view, g.h, g.persp, g.views})
	g.cur = g.cur.mul(m)
}

// pop restores the state from before the last push. A rect clip set
// under perspective was drawn in the view's frame, so it is set again
// from its window rect.
func (g *pdfGState) pop() {
	n := len(g.stack)
	if n == 0 {
		return
	}
	wasPersp := g.persp
	s := g.stack[n-1]
	g.stack = g.stack[:n-1]
	g.cur, g.view, g.h, g.persp, g.views = s.cur, s.view, s.h, s.persp, s.views
	if wasPersp {
		for i := range slices.Backward(g.clips) {
			if g.clips[i].kind == pdfClipRect {
				g.clips[i] = g.rectClip(g.clips[i].win)
				break
			}
		}
	}
	g.rebuild()
}

// beginRotate turns about (RotCX, RotCY). Screen turns are clockwise
// (RotatedBoxCfg); in y-down logical space that is a positive angle.
//
// gui maps a clip inside a quarter turn out to the window through the
// turn (#1019), as it does through a view transform. So the turn is
// composed into view and h too, which rectClip maps a clip back with,
// and it counts as a view: an empty clip inside it clips everything.
func (g *pdfGState) beginRotate(cmd *RenderCmd) {
	sin, cos := math.Sincos(float64(cmd.RotAngle) * math.Pi / 180)
	cx, cy := float64(cmd.RotCX), float64(cmd.RotCY)
	m := pdfAffine{
		cos, sin, -sin, cos,
		cx - cos*cx + sin*cy,
		cy - sin*cx - cos*cy,
	}
	g.push(m)
	g.view = g.view.mul(m)
	h := xfMat{float32(m[0]), float32(m[2]), float32(m[4]),
		float32(m[1]), float32(m[3]), float32(m[5]), 0, 0, 1}
	g.h = g.h.mul(&h)
	g.views++
	g.rebuild()
}

// beginTransform opens a view transform. PDF has only flat transforms,
// so a matrix with perspective is drawn with the flat map that matches
// it at the center of the view's box, which TransformBegin carries. A
// center at or behind the viewer draws nothing.
func (g *pdfGState) beginTransform(cmd *RenderCmd) {
	h := xfIdentity
	if cmd.Homography != nil {
		h = xfMat(*cmd.Homography)
	}
	m, ok := pdfAffineAt(&h, cmd.X+cmd.W/2, cmd.Y+cmd.H/2)
	if !ok {
		m = pdfAffine{} // flattens everything: nothing draws
	}
	g.push(m)
	g.view = g.view.mul(m)
	g.h = g.h.mul(&h)
	g.persp = g.persp || h[6] != 0 || h[7] != 0
	g.views++
	g.rebuild()
}

// native returns the PDF matrix for logical matrix m.
func (g *pdfGState) native(m pdfAffine) fpdf.TransformMatrix {
	n := g.phi.mul(m).mul(g.phiInv)
	return fpdf.TransformMatrix{A: n[0], B: n[1], C: n[2], D: n[3], E: n[4], F: n[5]}
}

// unwind ends every q this state opened, newest first.
func (g *pdfGState) unwind() {
	for i := range slices.Backward(g.opened) {
		if g.opened[i] {
			g.ops.ClipEnd()
		} else {
			g.ops.TransformEnd()
		}
	}
	g.opened = g.opened[:0]
}

// rebuild ends every q and opens them again: each clip under its own
// matrix, then the content matrix. "cm" multiplies onto the matrix in
// force, so each step moves from the last matrix by last⁻¹·next.
//
// A Q also drops the font, colors, line width and alpha set since its
// q. That is safe: every PDF draw call sets these again before it
// draws, and fpdf writes each setter out even when the value did not
// change (TestFPDFSetFontAlwaysWritesTf).
func (g *pdfGState) rebuild() {
	g.unwind()
	last := pdfAffineIdentity
	// moveTo makes m the matrix in force. A matrix that flattens the
	// plane cannot be drawn through, and nothing under it shows, so
	// an empty clip stands in for it. It reports false then.
	moveTo := func(m pdfAffine) bool {
		if m == last {
			return true
		}
		inv, okL := last.inverse()
		_, okM := m.inverse()
		if !okL || !okM {
			g.ops.ClipRect(0, 0, 0, 0, false)
			g.opened = append(g.opened, true)
			return false
		}
		g.ops.TransformBegin()
		g.opened = append(g.opened, false)
		g.ops.Transform(g.native(inv.mul(m)))
		last = m
		return true
	}
	for _, c := range g.clips {
		if !moveTo(c.xf) {
			return
		}
		g.ops.ClipRect(g.ctx.px(c.x), g.ctx.py(c.y),
			g.ctx.pw(max(c.w, 0)), g.ctx.ph(max(c.h, 0)), false)
		g.opened = append(g.opened, true)
	}
	moveTo(g.cur)
}
