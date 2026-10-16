package gui

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/gif"  // decode GIF file images for nearest PDF export
	_ "image/jpeg" // decode JPEG file images for nearest PDF export
	"image/png"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/go-pdf/fpdf"
)

// Nearest-neighbor images in PDF export (#1069).
//
// A PDF has no reliable way to ask for hard pixel edges. The image
// dictionary's /Interpolate key already defaults to false, and fpdf
// never writes it, yet most viewers smooth a scaled image anyway. So
// the pixels themselves are changed: each source pixel is repeated
// k x k times before embedding. A whole-number k keeps every edge on
// a pixel boundary, and at print resolution the viewer's own
// smoothing across one output pixel is too small to see.

const (
	// pdfNearestDPI is the resolution the repeated image aims for at
	// its drawn size: print quality, past what the eye resolves.
	pdfNearestDPI = 300
	// pdfNearestMaxSide caps each side of the repeated image, so a
	// small sprite drawn across a whole page cannot balloon the file.
	// 4096x4096 NRGBA is 64 MiB before PNG compression.
	pdfNearestMaxSide = 4096
)

// pdfNearestFactor returns how many times each source pixel repeats
// along each axis so the image reaches pdfNearestDPI at its drawn size
// in millimeters. The larger axis decides, and pdfNearestMaxSide caps
// the result. 1 means no repeat: the image is drawn at or below its
// own size, the cap leaves no room, or an input is not a positive
// finite number.
func pdfNearestFactor(srcW, srcH int, drawnWMM, drawnHMM float64) int {
	if srcW <= 0 || srcH <= 0 {
		return 1
	}
	// The negated test also rejects NaN. Inf is rejected on its own:
	// the cap below would turn it into the largest factor instead.
	if !(drawnWMM > 0) || !(drawnHMM > 0) ||
		math.IsInf(drawnWMM, 0) || math.IsInf(drawnHMM, 0) {
		return 1
	}
	const pxPerMM = pdfNearestDPI / 25.4
	k := math.Ceil(max(
		drawnWMM*pxPerMM/float64(srcW),
		drawnHMM*pxPerMM/float64(srcH)))
	k = min(k,
		float64(pdfNearestMaxSide/srcW),
		float64(pdfNearestMaxSide/srcH))
	if k < 2 {
		return 1
	}
	return int(k)
}

// pdfNearestKey names the repeated image in fpdf's image registry. It
// differs from the source key, so a linear draw of the same source in
// the same document still gets the original pixels. The NUL separator
// cannot appear in a file path, and a mem image name with one is not a
// real case, so the key never collides with another source's own key.
// fpdf uses the key only for map lookup; it is never written to the file.
func pdfNearestKey(src string, k int) string {
	return src + "\x00nearest@" + strconv.Itoa(k)
}

// pdfUpscaleNearest returns src with each pixel repeated k x k times.
// Each output row is built once, then copied for the other k-1 rows.
func pdfUpscaleNearest(src *image.NRGBA, k int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w*k, h*k))
	rowBytes := w * k * 4
	for y := range h {
		srcRow := src.Pix[y*src.Stride : y*src.Stride+w*4]
		first := dst.Pix[y*k*dst.Stride : y*k*dst.Stride+rowBytes]
		for x := range w {
			px := srcRow[x*4 : x*4+4]
			for i := range k {
				copy(first[(x*k+i)*4:], px)
			}
		}
		for r := 1; r < k; r++ {
			off := (y*k + r) * dst.Stride
			copy(dst.Pix[off:off+rowBytes], first)
		}
	}
	return dst
}

// pdfRegisterPNG PNG-encodes img and registers it in fpdf under key.
// It reports false when encoding fails, so the caller skips the draw.
func pdfRegisterPNG(pdf *fpdf.Fpdf, key string, img image.Image) bool {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return false
	}
	opts := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader(key, opts, &buf)
	return true
}

// pdfNearestFileImage registers the repeated form of the file image at
// path and returns its key. ok is false when no repeat applies or the
// file cannot be decoded: the caller then embeds the file as it is, so
// a nearest image never draws worse than a linear one. The header is
// read first, so a file whose size already meets the target is never
// fully decoded.
func pdfNearestFileImage(ctx *pdfCtx, cmd RenderCmd,
	path string) (key string, ok bool) {
	// #nosec G304 — path is the RenderCmd's image resource, the same file
	// fpdf opens on the fallback path; pdfRenderFileImage has already
	// checked it is a regular file within maxPDFImageBytes.
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return "", false
	}
	k := pdfNearestFactor(cfg.Width, cfg.Height, ctx.pw(cmd.W), ctx.ph(cmd.H))
	if k < 2 {
		return "", false
	}
	key = pdfNearestKey(path, k)
	if ctx.pdf.GetImageInfo(key) != nil {
		return key, true
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", false
	}
	decoded, _, err := image.Decode(f)
	if err != nil {
		return "", false
	}
	// pdfNearestFactor capped k against the header size; a decoder
	// that disagrees with its own header must not get past that cap.
	db := decoded.Bounds()
	if db.Dx() != cfg.Width || db.Dy() != cfg.Height {
		return "", false
	}
	src := image.NewNRGBA(image.Rect(0, 0, db.Dx(), db.Dy()))
	draw.Draw(src, src.Rect, decoded, db.Min, draw.Src)
	if !pdfRegisterPNG(ctx.pdf, key, pdfUpscaleNearest(src, k)) {
		return "", false
	}
	return key, true
}
