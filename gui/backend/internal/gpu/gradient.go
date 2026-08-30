package gpu

import "github.com/go-gui-org/go-gui/gui"

// GradientStopSlots is how many stops the GPU shaders' uniforms carry.
// It must match gui's gradientShaderStopLimit, which is what feeds
// this packer through gui.NormalizeGradientStopsInto; the packer
// clamps to it either way so a mismatch drops stops rather than
// reading slots nothing wrote.
const GradientStopSlots = 12

// stopsInTM is how many of them live in the first matrix. Its tail is
// spoken for by the axis and metadata columns, so four stops and two
// spare slots is all that fits there; the second matrix carries the
// rest.
const stopsInTM = 4

// PackGradientUniforms packs gradient stop data into the two
// [16]float32 uniform matrices the GPU shaders read. stops must be
// pre-normalized via gui.NormalizeGradientStopsInto.
//
// Two floats each (PackRGB + PackAlphaPos): tm holds stops 0-3 in
// tm[0..7], tm2 holds stops 4-11 across all sixteen of its floats.
//
// The uneven split is a budget, not a preference. tm's tail is already
// spoken for — the direction/radius pair at tm[10..11] and four
// metadata floats at tm[12..15] — which leaves room for four stops and
// two spare slots. tm2 has no metadata to carry, so all of it is
// stops. That metadata layout is unchanged from when tm carried five
// stops, so the vertex shaders' varyings keep their meaning.
func PackGradientUniforms(
	gdef *gui.GradientDef,
	stops []gui.GradientStop,
	w, h float32,
) (tm, tm2 [16]float32) {
	n := min(len(stops), GradientStopSlots)
	for i := range n {
		dst, slot := &tm, i
		if i >= stopsInTM {
			dst, slot = &tm2, i-stopsInTM
		}
		dst[slot*2] = gui.PackRGB(stops[i].Color)
		dst[slot*2+1] = gui.PackAlphaPos(stops[i].Color, stops[i].Pos)
	}

	// Column 2, rows 2-3: direction or radial metadata.
	if gdef != nil && gdef.Type == gui.GradientRadial {
		tm[2*4+3] = max(w/2, h/2)
		tm[3*4+2] = 1.0 // radial
	} else {
		dx, dy := gui.GradientDir(gdef, w, h)
		tm[2*4+2] = dx
		tm[2*4+3] = dy
		tm[3*4+2] = 0.0 // linear
	}

	// Column 3: metadata. The count is clamped with the loop above:
	// telling the shader about a stop no slot holds would send it
	// reading zeroes as a black, fully transparent stop.
	tm[3*4+0] = w / 2 // half-width
	tm[3*4+1] = h / 2 // half-height
	tm[3*4+3] = float32(n)

	return tm, tm2
}
