package soft

import (
	"image"
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// beginStencil opens a rounded-rect clip. The GPU path increments a
// stencil buffer and tests every later fragment against it; the CPU
// path collects the bracketed drawing in a layer and composites it
// through the same rounded rect as a coverage mask.
//
// Nesting therefore needs no depth counter: an inner bracket composites
// into the outer bracket's layer, which is already masked. RenderCmd's
// StencilDepth is what the GL backend uses to unwind its shared stencil
// buffer, and a layer stack has no equivalent to unwind.
func (r *renderer) beginStencil(cmd *gui.RenderCmd) {
	s := r.scale
	br := r.pushLayer(bracketStencil)
	br.x, br.y = cmd.X*s, cmd.Y*s
	br.w, br.h = cmd.W*s, cmd.H*s
	br.radius = cmd.Radius * s
}

// endStencil closes the clip and composites the layer through it.
func (r *renderer) endStencil() {
	br, ok := r.popLayer(bracketStencil)
	if !ok || br.layer == nil {
		return
	}
	defer r.releaseLayer(br.layer)

	region := deviceRect(br.x, br.y, br.w, br.h).
		Intersect(r.buf.img.Bounds())
	mask := r.coverageRoundRect(&r.maskPix, region,
		br.x, br.y, br.w, br.h, br.radius)
	if mask == nil {
		return
	}
	composite(r.buf, br.layer, br.layer.dirty, mask)
}

// beginRotation opens a rotation bracket. Where the GPU backends push a
// rotated MVP, this renders the bracketed commands upright into a layer
// and rotates the finished layer on the way back — so text, gradients
// and images rotate exactly as rectangles do, and no phase 1 draw path
// has to know a transform is in effect.
func (r *renderer) beginRotation(cmd *gui.RenderCmd) {
	s := r.scale
	br := r.pushLayer(bracketRotate)
	br.angle = cmd.RotAngle
	br.cx, br.cy = cmd.RotCX*s, cmd.RotCY*s
}

// endRotation closes the bracket and blits the layer back rotated.
func (r *renderer) endRotation() {
	br, ok := r.popLayer(bracketRotate)
	if !ok || br.layer == nil {
		return
	}
	defer r.releaseLayer(br.layer)

	src := br.layer.dirty
	if src.Empty() {
		return
	}
	if br.angle == 0 {
		composite(r.buf, br.layer, src, nil)
		return
	}
	rad := float64(br.angle) * math.Pi / 180
	sin := float32(math.Sin(rad))
	cos := float32(math.Cos(rad))

	dst := rotatedBounds(src, sin, cos, br.cx, br.cy).
		Intersect(r.buf.clip)
	blitTransformed(r.buf, br.layer, dst, sin, cos, br.cx, br.cy)
}

// rotatedBounds is the pixel box the rotated content can land in: the
// four corners of src mapped forward, then bounded.
func rotatedBounds(src image.Rectangle, sin, cos, cx, cy float32,
) image.Rectangle {

	corners := [4][2]float32{
		{float32(src.Min.X), float32(src.Min.Y)},
		{float32(src.Max.X), float32(src.Min.Y)},
		{float32(src.Max.X), float32(src.Max.Y)},
		{float32(src.Min.X), float32(src.Max.Y)},
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -float32(math.MaxFloat32), -float32(math.MaxFloat32)
	for _, c := range corners {
		dx, dy := c[0]-cx, c[1]-cy
		x := cx + dx*cos - dy*sin
		y := cy + dx*sin + dy*cos
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	// One pixel of slack each way covers the bilinear tap footprint.
	return deviceRect(minX-1, minY-1, maxX-minX+2, maxY-minY+2)
}
