package soft

import (
	"image"
	"math"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/homography"
)

// beginTransform opens a view transform bracket (#1019). Like a
// rotation, the bracketed commands draw untransformed into a layer, in
// the view's own frame, and endTransform maps the finished layer
// through the matrix. The map is a full homography, so perspective is
// exact here, as it is on the GPU.
//
// A layer is the size of the window, so content that lies outside the
// window before the transform moves it is not kept: a view moved into
// the window from outside it stays blank on this backend.
func (r *renderer) beginTransform(cmd *gui.RenderCmd) {
	m := homography.Identity
	if cmd.Homography != nil {
		m = homography.Scaled(cmd.Homography, r.scale)
	}
	acc := m
	if outer := r.innerTransform(); outer != nil {
		acc = homography.Mul(&outer.acc, &m)
	}
	entry := r.buf.clip
	br := r.pushLayer(bracketTransform)
	br.m, br.acc, br.entry = m, acc, entry
	br.accInv, br.accInvOK = homography.Inverse(&acc)
	if br.layer != nil {
		// The parent's clip is a rect in the parent's frame, and in the
		// view's frame it is not a rect. Draw unclipped; the blit cuts
		// to entry.
		br.layer.clip = br.layer.img.Bounds()
	}
}

// endTransform closes the bracket and maps the layer into the parent.
func (r *renderer) endTransform() {
	br, ok := r.popLayer(bracketTransform)
	if !ok {
		return
	}
	// popLayer handed the parent the layer's last clip, which is in the
	// view's frame. The window clip it came from is mapped again, into
	// the parent's frame. With no clip yet this frame, the parent keeps
	// its clip from when the bracket opened.
	if r.winClipSet {
		r.setWindowClip(r.winClip[0], r.winClip[1], r.winClip[2], r.winClip[3])
	} else {
		r.buf.clip = br.entry
	}
	if br.layer == nil {
		return
	}
	defer r.releaseLayer(br.layer)

	src := br.layer.dirty
	if src.Empty() {
		return
	}
	inv, invOK := homography.Inverse(&br.m)
	if !invOK {
		return // the view is flattened to a line or a point
	}
	dst := br.entry
	x, y, w, h, okb := homography.Bounds(&br.m, float32(src.Min.X),
		float32(src.Min.Y), float32(src.Dx()), float32(src.Dy()))
	if okb {
		// One pixel of slack each way covers the bilinear tap footprint.
		dst = dst.Intersect(deviceRect(x-1, y-1, w+2, h+2))
	}
	blitHomography(r.buf, br.layer, dst, &inv)
}

// innerTransform returns the innermost open transform or rotation
// bracket, or nil. Both keep the map from their layer to the window.
func (r *renderer) innerTransform() *bracket {
	for i := len(r.brackets) - 1; i >= 0; i-- {
		if k := r.brackets[i].kind; k == bracketTransform || k == bracketRotate {
			return &r.brackets[i]
		}
	}
	return nil
}

// setWindowClip applies a RenderClip. The rect is in the window, as a
// GPU scissor is: gui maps a clip inside a transform or a quarter turn
// out to the window before it emits it. Inside one the target is a layer in the
// view's own frame, so the rect is mapped back into that frame. Under a
// turn, skew or perspective, the box of the mapped rect is bigger than
// the clip; gui adds a stencil there, which cuts exactly.
func (r *renderer) setWindowClip(x, y, w, h float32) {
	r.winClip = [4]float32{x, y, w, h}
	r.winClipSet = true
	br := r.innerTransform()
	if br == nil {
		r.buf.setClip(x, y, w, h)
		return
	}
	// Outside a transform, setClip reads a degenerate rect as "no
	// clip". Inside one, gui emits it for a clip that cuts the view
	// away entirely (xfFrame.toWindow), so it clips everything, as an
	// empty GPU scissor does.
	if !(w > 0 && h > 0) || !br.accInvOK {
		r.buf.clip = image.Rectangle{}
		return
	}
	lx, ly, lw, lh, ok := homography.Bounds(&br.accInv, x, y, w, h)
	if !ok {
		// Part of the clip maps to behind the viewer: no finite rect
		// holds it in the view's frame, so clip nothing here. The blit
		// still cuts to the entry clip.
		r.buf.clip = r.buf.img.Bounds()
		return
	}
	// setClip reads a degenerate rect as "no clip"; a clip mapped to a
	// sliver must still clip.
	r.buf.clip = deviceRect(lx, ly, max(lw, 0), max(lh, 0)).
		Intersect(r.buf.img.Bounds())
}

// blitHomography composites src over dst inside rect. Each destination
// pixel is mapped back through inv, which maps dst to src, and the
// source is sampled bilinearly. A pixel whose ray misses the view's
// plane in front of the viewer (w <= 0) is skipped.
func blitHomography(dst, src *buffer, rect image.Rectangle, inv *homography.Mat) {
	rect = rect.Intersect(dst.img.Bounds())
	if rect.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		fy := float32(y) + 0.5
		di := dst.img.PixOffset(rect.Min.X, y)
		for x := rect.Min.X; x < rect.Max.X; x++ {
			fx := float32(x) + 0.5
			w := inv[6]*fx + inv[7]*fy + inv[8]
			if !(w > 0) || w > math.MaxFloat32 {
				di += 4
				continue
			}
			sx := (inv[0]*fx + inv[1]*fy + inv[2]) / w
			sy := (inv[3]*fx + inv[4]*fy + inv[5]) / w
			sr, sg, sb, sa := sampleBilinear(src, sx-0.5, sy-0.5)
			if sa == 0 && sr == 0 && sg == 0 && sb == 0 {
				di += 4
				continue
			}
			inva := 255 - sa
			p := dst.img.Pix
			p[di] = overPremul(sr, uint32(p[di]), inva)
			p[di+1] = overPremul(sg, uint32(p[di+1]), inva)
			p[di+2] = overPremul(sb, uint32(p[di+2]), inva)
			p[di+3] = overPremul(sa, uint32(p[di+3]), inva)
			di += 4
		}
	}
	dst.markDirty(rect)
}
