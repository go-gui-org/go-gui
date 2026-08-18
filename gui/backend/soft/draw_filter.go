package soft

import (
	"image"

	"github.com/go-gui-org/go-gui/gui"
)

// minFilterBlur is the blur below which the filter's blur passes are
// skipped, matching the GPU path's `filterBlur >= 1` gate: under a
// device pixel there is nothing to spread.
const minFilterBlur = 1

// maxFilterLayers caps the composite repeat count. Layers is the count
// of feMergeNode elements in an SVG filter, so an untrusted document
// can name an arbitrary number of them; past a handful the glow is
// already saturated and each extra pass is a full-layer blend.
const maxFilterLayers = 32

// beginFilter opens a filter (glow) bracket: everything up to the
// matching RenderFilterEnd is drawn into an offscreen layer instead of
// the target, exactly as the GPU backends bind an FBO.
func (r *renderer) beginFilter(cmd *gui.RenderCmd) {
	br := r.pushLayer(bracketFilter)
	br.blur = clampBlur(cmd.BlurRadius * r.scale)
	br.layers = min(max(cmd.Layers, 1), maxFilterLayers)
	br.matrix = cmd.ColorMatrix
}

// endFilter blurs the layer, applies the color matrix, and composites
// the result once per layer — the same three passes, in the same
// order, as the GPU path's blur / color / composite pipelines.
//
// The composite really is repeated: drawing the result N times is how
// a glow gains its intensity, and repeating the blend reproduces it
// rather than approximating it with a single scaled draw.
func (r *renderer) endFilter() {
	br, ok := r.popLayer(bracketFilter)
	if !ok || br.layer == nil {
		return
	}
	defer r.releaseLayer(br.layer)

	rect := br.layer.dirty
	if rect.Empty() {
		return
	}
	if br.blur >= minFilterBlur {
		// Blurring spreads paint past the drawn box, so the working
		// rect has to grow before the passes run.
		grow := int(br.blur*blurExpand) + 1
		rect = rect.Inset(-grow).Intersect(br.layer.img.Bounds())
		boxBlurRGBA(br.layer.img, rect, &r.blurTmp, br.blur)
		br.layer.markDirty(rect)
	}
	if br.matrix != nil {
		applyColorMatrix(br.layer.img, rect, br.matrix)
	}
	// The composite is clipped to the current clip rect, as the GPU
	// backends' scissor survives the FBO bind: the blur may have spread
	// the layer's paint past the clip, and that spread must not leak
	// onto the window.
	out := rect.Intersect(r.buf.clip)
	for range br.layers {
		composite(r.buf, br.layer, out, nil)
	}
}

// applyColorMatrix multiplies every pixel by a column-major 4x4 matrix
// and clamps, which is what FsFilterColorGLSL does. The pixels are
// premultiplied on both sides of the multiply, as they are in the GPU
// backends' filter texture.
func applyColorMatrix(img *image.RGBA, rect image.Rectangle,
	m *[16]float32) {

	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		i := img.PixOffset(rect.Min.X, y)
		for x := rect.Min.X; x < rect.Max.X; x++ {
			var in [4]float32
			for c := range 4 {
				in[c] = float32(img.Pix[i+c]) / 255
			}
			for row := range 4 {
				v := m[row] * in[0] // column 0
				v += m[4+row] * in[1]
				v += m[8+row] * in[2]
				v += m[12+row] * in[3]
				img.Pix[i+row] = uint8(min(max(v, 0), 1)*255 + 0.5)
			}
			i += 4
		}
	}
}
