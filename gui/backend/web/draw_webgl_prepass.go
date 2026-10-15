//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// The WebGL SVG pre-pass and its atlas placement (draw_webgl.go).

// prepassSvgGL is the first pass of a frame in WebGL mode. It runs the
// frame's clip, transform, filter and SVG commands through the same
// batching as the drawing pass, so the batches come out the same. The
// batch split depends only on the clip, transform and bracket stacks,
// so while preparing the helpers that replay them into the 2D context
// (rebuildState, pushBracket, setCanvasTransform) keep the stacks and
// skip the JS calls (#1051). prepareSvgGL also sets the 2D context to
// null, so a 2D call that a later change adds to the pre-pass panics in
// the tests and does not draw on the visible canvas.
// Each batch is rendered into the WebGL canvas where placeSvgGL puts
// it, and the drawn area is copied once into the staging canvas. The
// drawing pass then composites each batch from the copy.
func (b *Backend) prepassSvgGL(cmds []gui.RenderCmd) {
	g := &b.gl
	g.prepared = g.prepared[:0]
	g.next = 0
	g.placed = g.placed[:0]
	g.shelfX, g.shelfY, g.shelfH = 0, 0, 0
	g.usedX0, g.usedY0, g.usedX1, g.usedY1 = math.MaxInt, math.MaxInt, 0, 0
	g.transientN = 0
	if g.lost {
		return
	}
	b.prepareSvgGL(cmds)
	if g.usedX1 > g.usedX0 {
		g.api.Call("stage", g.state, g.usedX0, g.usedY0,
			g.usedX1-g.usedX0, g.usedY1-g.usedY0)
	}
}

// prepareSvgGL runs the pre-pass's commands with the 2D context set to
// null. However it ends, a panic included, the real context and the
// frame's starting state come back: a panic is recovered by the render
// loop. A later frame still marked preparing would skip every clip,
// rotation and filter, and would composite no SVG batch.
func (b *Backend) prepareSvgGL(cmds []gui.RenderCmd) {
	g := &b.gl
	ctx := b.ctx2d
	b.ctx2d = js.Null()
	g.preparing = true
	defer func() {
		g.preparing = false
		b.ctx2d = ctx
		g.resetBatch()
		// Back to the frame's start, as renderFrame left it. No save
		// reached the context, so stateDepth did not move.
		b.resetTransforms()
		b.clipStack = b.clipStack[:0]
		b.brackets = b.brackets[:0]
		b.clipDirty = false
	}()
	for i := range cmds {
		r := &cmds[i]
		b.beforeCmdSvgGL(r)
		switch r.Kind {
		case gui.RenderClip:
			b.drawClip(r)
		case gui.RenderSvg:
			b.drawSvg(r)
		case gui.RenderFilterBegin:
			b.beginFilter(r)
		case gui.RenderFilterEnd:
			b.endFilter()
		case gui.RenderStencilBegin:
			b.beginStencilClip(r)
		case gui.RenderStencilEnd:
			b.endStencilClip()
		case gui.RenderRotateBegin:
			b.beginRotation(r)
		case gui.RenderRotateEnd:
			b.endRotation()
		case gui.RenderTransformBegin:
			b.beginTransform(r)
		case gui.RenderTransformEnd:
			b.endTransform()
		default:
		}
	}
	b.settleSvgGL()
}

// placeSvgGL picks where in the WebGL canvas the pre-pass renders p: at
// its own place unless it has none (away) or a batch took part of it,
// else on the next shelf of the overflow half, else nowhere (direct).
func (g *glSvg) placeSvgGL(p *glPrepared) {
	r := [4]int{p.x0, p.y0, p.x0 + p.w, p.y0 + p.h}
	free := !p.away
	for _, q := range g.placed {
		if !free {
			break
		}
		if r[0] < q[2] && q[0] < r[2] && r[1] < q[3] && q[1] < r[3] {
			free = false
		}
	}
	if !free {
		g.overflowUsed = true
	}
	switch {
	case free:
		g.placed = append(g.placed, r)
	case !g.overflow:
		if !g.overflowDenied {
			g.wantOverflow = true
		}
		p.direct = true
		return
	default:
		if g.shelfX+p.w > g.w {
			g.shelfX, g.shelfY, g.shelfH = 0, g.shelfY+g.shelfH, 0
		}
		if g.shelfY+p.h > g.h {
			p.direct = true
			return
		}
		p.ox, p.oy = g.shelfX-p.x0, g.h+g.shelfY-p.y0
		g.shelfX += p.w
		g.shelfH = max(g.shelfH, p.h)
	}
	g.usedX0, g.usedY0 = min(g.usedX0, p.x0+p.ox), min(g.usedY0, p.y0+p.oy)
	g.usedX1 = max(g.usedX1, p.x0+p.ox+p.w)
	g.usedY1 = max(g.usedY1, p.y0+p.oy+p.h)
}
