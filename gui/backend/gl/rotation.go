//go:build !js

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// beginRotation saves the current MVP and applies a rotation
// transform around the given center point.
func (b *Backend) beginRotation(r *gui.RenderCmd) {
	b.mvpStack = append(b.mvpStack, b.mvp)
	s := b.dpiScale
	cx := r.RotCX * s
	cy := r.RotCY * s
	gpu.ApplyRotation(&b.mvp, r.RotAngle, cx, cy)
	b.usePipeline(&b.pipelines.solid)
}

// endRotation restores the pre-rotation MVP.
func (b *Backend) endRotation() {
	n := len(b.mvpStack)
	if n == 0 {
		return
	}
	b.mvp = b.mvpStack[n-1]
	b.mvpStack = b.mvpStack[:n-1]
	b.usePipeline(&b.pipelines.solid)
}
