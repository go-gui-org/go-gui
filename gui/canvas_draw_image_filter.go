package gui

// ImageFilter selects how an image's texels are sampled when the image
// is drawn at a size other than its own. Set it on a DrawContext with
// SetImageFilter.
type ImageFilter uint8

const (
	// ImageFilterLinear blends the nearest texels, so a scaled image
	// looks smooth. It is the zero value and the default.
	ImageFilterLinear ImageFilter = iota
	// ImageFilterNearest takes the single nearest texel, so a scaled
	// image keeps hard pixel edges. Use it for pixel art and small
	// sprites drawn at 2x, 3x or more.
	ImageFilterNearest
)

// canvasSaved is one Save: the transform and the image filter, which
// Restore puts back together.
type canvasSaved struct {
	xf          canvasXform
	imageFilter ImageFilter
}

// known returns f, or ImageFilterLinear for a value no backend knows.
// A backend then only has to test for ImageFilterNearest.
func (f ImageFilter) known() ImageFilter {
	if f == ImageFilterNearest {
		return f
	}
	return ImageFilterLinear
}

// SetImageFilter sets how the image calls that follow sample their
// texels: Image, ImageWithFetcher and ImageClipped all use it. The
// filter is part of the state Save and Restore keep, so a scoped change
// is:
//
//	dc.Save()
//	dc.SetImageFilter(gui.ImageFilterNearest)
//	dc.Image(x, y, w, h, "sprite.png", gui.Opt[float32]{}, gui.Color{})
//	dc.Restore()
//
// Each OnDraw starts with ImageFilterLinear. A value that is not one of
// the ImageFilter constants draws as ImageFilterLinear. Recorders (SVG
// and PDF export) do not get the filter.
func (dc *DrawContext) SetImageFilter(f ImageFilter) {
	dc.imageFilter = f
}

// ImageFilter returns the filter SetImageFilter last set.
func (dc *DrawContext) ImageFilter() ImageFilter {
	return dc.imageFilter
}
