package gpu

import "github.com/go-gui-org/go-gui/gui"

// GradientStopSlots is how many stops the GPU shaders' uniforms carry.
// It must match gui's gradientShaderStopLimit, which is what feeds
// this packer through gui.NormalizeGradientStopsInto; the packer
// clamps to it either way so a mismatch drops stops rather than
// reading slots nothing wrote.
const GradientStopSlots = 8

// PackGradientUniforms packs gradient stop data into the two
// [16]float32 uniform matrices the GPU shaders read. stops must be
// pre-normalized via gui.NormalizeGradientStopsInto.
//
// Four stops per matrix, two floats each (PackRGB + PackAlphaPos):
// tm holds stops 0-3 in tm[0..7], tm2 holds stops 4-7 in tm2[0..7].
//
// The split is a budget, not a preference. tm's tail is already spoken
// for — the direction/radius pair at tm[10..11] and four metadata
// floats at tm[12..15] — which leaves exactly room for four stops and
// two spare slots. That metadata layout is unchanged from when tm
// carried five stops, so the vertex shaders' varyings keep their
// meaning; only the fifth stop moved out. tm2's upper half is free, so
// raising the limit to twelve later costs no new uniform.
func PackGradientUniforms(
	gdef *gui.GradientDef,
	stops []gui.GradientStop,
	w, h float32,
) (tm, tm2 [16]float32) {
	n := min(len(stops), GradientStopSlots)
	for i := range n {
		dst, slot := &tm, i
		if i >= GradientStopSlots/2 {
			dst, slot = &tm2, i-GradientStopSlots/2
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
