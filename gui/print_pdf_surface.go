package gui

import (
	"fmt"
	"image"

	"github.com/go-pdf/fpdf"
)

// pdfRenderSurface embeds the surface's current frame. It encodes the pixels
// while ReadPixels holds the surface lock, so the PDF never keeps a reference
// to mutable storage. The key includes the frame version: the same frame
// drawn twice is embedded once, a newer one is embedded again.
func pdfRenderSurface(ctx *pdfCtx, cmd RenderCmd) {
	cmd.Surface.ReadPixels(0, func(w, h int, pix []byte, version uint64) {
		key := fmt.Sprintf("surface:%p:%d", cmd.Surface, version)
		factor := 1
		if cmd.ImageFilter == ImageFilterNearest {
			factor = pdfNearestFactor(w, h, ctx.pw(cmd.W), ctx.ph(cmd.H))
			key = pdfNearestKey(key, factor)
		}
		if ctx.pdf.GetImageInfo(key) == nil {
			img := &image.NRGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
			if factor > 1 {
				img = pdfUpscaleNearest(img, factor)
			}
			if !pdfRegisterPNG(ctx.pdf, key, img) {
				return
			}
		}
		ctx.pdf.ImageOptions(key, ctx.px(cmd.X), ctx.py(cmd.Y),
			ctx.pw(cmd.W), ctx.ph(cmd.H), false,
			fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	})
}
