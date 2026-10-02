//go:build !js && !darwin && !android

package gl

import (
	"math"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// maxSvgTriangleFloats caps a RenderSvg triangle list in floats,
// mirroring the gui package's emit-side cap. It bounds the
// per-frame vertex allocation an oversized command would force.
const maxSvgTriangleFloats = 1_200_000

// maxSvgBatchVerts caps one queued SVG run, in vertices: the most a
// single command may carry. A run never holds more than the largest
// command already could, so batching adds no new worst case to the
// vertex slice or the GL buffer.
const maxSvgBatchVerts = maxSvgTriangleFloats / 2

// drawSvg transforms a RenderSvg command's triangles into backend
// vertices and queues them; flushSvg draws the run (#895).
//
// Every vertex carries its own color and is already in physical
// pixels, so meshes from different commands need no per-command
// state to draw together: a run of them is one upload and one
// DrawArrays instead of a bind, upload and draw per command. A
// ThinkingOrb frame emits one small mesh per dot, each its own color,
// so that round trip through purego used to cost a draw per dot.
func (b *Backend) drawSvg(r *gui.RenderCmd) {
	if r.IsClipMask {
		return // clip masks not yet supported in render pipeline
	}
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 ||
		len(r.Triangles) > maxSvgTriangleFloats {
		return
	}
	s := b.dpiScale
	numVerts := len(r.Triangles) / 2
	hasVCols := len(r.VertexColors) == numVerts
	vAlpha := float32(1)
	if r.HasVertexAlpha {
		vAlpha = max(0, min(r.VertexAlphaScale, 1))
	}

	hasXform := r.HasXform
	var sx, sy, tx, ty, sxy, syx float32
	if hasXform {
		sx, sy, tx, ty = r.ScaleX, r.ScaleY, r.TransX, r.TransY
		sxy, syx = r.XformXY, r.XformYX
	}
	hasRot := r.RotAngle != 0
	var sinA, cosA, rcx, rcy float32
	if hasRot {
		rad := float64(r.RotAngle) * math.Pi / 180
		sinA = float32(math.Sin(rad))
		cosA = float32(math.Cos(rad))
		rcx, rcy = r.RotCX, r.RotCY
	}

	// Draw the queued run first if this mesh would push it past the
	// cap. A single command is at most the cap, so it always fits an
	// empty run.
	if len(b.svgVerts)+numVerts > maxSvgBatchVerts {
		b.flushSvg()
	}
	// Append in place: the slice keeps its capacity across frames,
	// so a steady frame allocates nothing here.
	start := len(b.svgVerts)
	b.svgVerts = slices.Grow(b.svgVerts, numVerts)[:start+numVerts]
	verts := b.svgVerts[start:]
	for i := range numVerts {
		vx := r.Triangles[i*2]
		vy := r.Triangles[i*2+1]
		if hasXform {
			ox, oy := vx, vy
			vx = ox*sx + oy*sxy + tx
			vy = ox*syx + oy*sy + ty
		}
		if hasRot {
			dx := vx - rcx
			dy := vy - rcy
			vx = rcx + dx*cosA - dy*sinA
			vy = rcy + dx*sinA + dy*cosA
		}
		v := &verts[i]
		v.X = (r.X + vx*r.Scale) * s
		v.Y = (r.Y + vy*r.Scale) * s
		v.U = 0
		v.V = 0
		if hasVCols {
			vc := r.VertexColors[i]
			alpha := vc.A
			if r.HasVertexAlpha {
				alpha = uint8(float32(alpha) * vAlpha)
			}
			cr, cg, cb, ca := gpu.NormColor(vc.R, vc.G, vc.B, alpha)
			v.R = cr
			v.G = cg
			v.B = cb
			v.A = ca
		} else {
			cr, cg, cb, ca := gpu.NormColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)
			v.R = cr
			v.G = cg
			v.B = cb
			v.A = ca
		}
	}
}

// flushSvg draws the queued SVG run, if any, and empties it. drawCmd
// calls it before every non-SVG command, renderersDraw once more at
// the end of the stream.
func (b *Backend) flushSvg() {
	if len(b.svgVerts) == 0 {
		return
	}
	b.usePipeline(&b.pipelines.solid)
	b.uploadSvgVerts(b.svgVerts)
	b.svgVerts = b.svgVerts[:0]
	b.svgFlushes++
}
