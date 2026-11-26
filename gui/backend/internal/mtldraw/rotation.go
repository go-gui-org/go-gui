//go:build darwin && arm64

package mtldraw

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// --- Rotation ---

func (b *Renderer) beginRotation(r *gui.RenderCmd) {
	b.mvpStack = append(b.mvpStack, b.mvp)
	s := b.dpiScale
	cx := r.RotCX * s
	cy := r.RotCY * s
	gpu.ApplyRotation(&b.mvp, r.RotAngle, cx, cy)
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}

// beginTransform saves the MVP and multiplies in a view transform
// (ContainerCfg.Transform). RenderTransformEnd pops it through
// endRotation: both brackets share the MVP stack.
func (b *Renderer) beginTransform(r *gui.RenderCmd) {
	b.mvpStack = append(b.mvpStack, b.mvp)
	if r.Homography == nil {
		return
	}
	gpu.ApplyHomography(&b.mvp, r.Homography, b.dpiScale)
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}

func (b *Renderer) endRotation() {
	n := len(b.mvpStack)
	if n == 0 {
		return
	}
	b.mvp = b.mvpStack[n-1]
	b.mvpStack = b.mvpStack[:n-1]
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}
