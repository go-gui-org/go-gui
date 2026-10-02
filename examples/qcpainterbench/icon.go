package main

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// iconSize matches Qt's circle.png: 128×128. The benchmark draws it scaled down to
// about 35 px, so the GPU samples a minified texture, as Qt's does.
const iconSize = 128

// iconImage registers the icon in go-gui's in-memory image registry and returns
// its Src string.
//
// Qt's benchmark ships circle.png, a gray-and-alpha disc. This draws a similar
// image in code instead of copying the asset: a light disc with a darker ring and
// a soft edge. The texture size and the alpha blending are what matter for the
// benchmark, not the exact pixels.
func iconImage() string {
	// Registering a key again with the same size is a no-op, so a second scene
	// (tests build several) gets the same Src back.
	const key = "qcpainterbench/icon"
	pix := make([]byte, iconSize*iconSize*4)
	const c = iconSize / 2.0
	const outer = c - 2 // disc radius, leaving room for the soft edge
	const ring = 10.0   // ring width
	for y := range iconSize {
		for x := range iconSize {
			dx, dy := float64(x)+0.5-c, float64(y)+0.5-c
			d := math.Hypot(dx, dy)
			// Coverage falls from 1 to 0 across one pixel at the edge.
			a := math.Max(0, math.Min(1, outer-d+0.5))
			v := 230.0
			if d > outer-ring {
				v = 150
			}
			i := (y*iconSize + x) * 4
			pix[i+0] = uint8(v)
			pix[i+1] = uint8(v)
			pix[i+2] = uint8(v)
			pix[i+3] = uint8(255 * a)
		}
	}
	return gui.UseImage(key, iconSize, iconSize, pix)
}
