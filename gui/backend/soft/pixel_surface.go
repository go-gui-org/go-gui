package soft

import "image"

// uploadSurfaceImage copies a surface frame into img, reusing it at a fixed
// size and allocating a new one on a resize.
func uploadSurfaceImage(img *image.NRGBA, w, h int, pix []byte) *image.NRGBA {
	if img == nil || img.Rect.Dx() != w || img.Rect.Dy() != h {
		img = image.NewNRGBA(image.Rect(0, 0, w, h))
	}
	copy(img.Pix, pix)
	return img
}
