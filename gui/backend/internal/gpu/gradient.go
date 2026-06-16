package gpu

import "github.com/go-gui-org/go-gui/gui"

// PackGradientUniforms packs gradient stop data into a [16]float32
// TM uniform matrix following the GPU shader convention.
// stops must be pre-normalized via gui.NormalizeGradientStopsInto.
func PackGradientUniforms(
	gdef *gui.GradientDef,
	stops []gui.GradientStop,
	w, h float32,
) [16]float32 {
	var tm [16]float32
	// Columns 0-2: packed stop data (rgb + alpha+pos pairs).
	for i := range min(len(stops), 5) {
		col := i / 2
		row := (i % 2) * 2
		tm[col*4+row] = gui.PackRGB(stops[i].Color)
		tm[col*4+row+1] = gui.PackAlphaPos(
			stops[i].Color, stops[i].Pos)
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

	// Column 3: metadata.
	tm[3*4+0] = w / 2 // half-width
	tm[3*4+1] = h / 2 // half-height
	tm[3*4+3] = float32(len(stops))

	return tm
}
