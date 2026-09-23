//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
#include "metal_darwin.h"
*/
import "C"

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// --- Rotation ---

func (b *windowState) beginRotation(r *gui.RenderCmd) {
	b.mvpStack = append(b.mvpStack, b.mvp)
	s := b.dpiScale
	cx := r.RotCX * s
	cy := r.RotCY * s
	gpu.ApplyRotation(&b.mvp, r.RotAngle, cx, cy)
	C.metalSetPipeline(b.ctx, C.int(pipeSolid))
	C.metalSetMVP(b.ctx, (*C.float)(&b.mvp[0]))
}

func (b *windowState) endRotation() {
	n := len(b.mvpStack)
	if n == 0 {
		return
	}
	b.mvp = b.mvpStack[n-1]
	b.mvpStack = b.mvpStack[:n-1]
	C.metalSetPipeline(b.ctx, C.int(pipeSolid))
	C.metalSetMVP(b.ctx, (*C.float)(&b.mvp[0]))
}
