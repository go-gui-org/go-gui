package soft

import (
	"image"
	"image/color"
	"log"
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
)

// imageSrc scales a decoded source into a destination rect, sampling
// bilinearly in straight-alpha space at absolute device coordinates.
//
// nr is the scratch color each At returns: vector.Draw consumes the
// sample immediately, so one instance can be re-pointed (the same
// reasoning as buffer.solid's reusable image.Uniform).
type imageSrc struct {
	src            *image.NRGBA
	dstX, dstY     float32
	scaleX, scaleY float32 // source pixels per device pixel
	srcW, srcH     int
	stride         int
	nr             color.NRGBA
}

func newImageSrc(src *image.NRGBA, x, y, w, h float32) *imageSrc {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	return &imageSrc{
		src:    src,
		dstX:   x,
		dstY:   y,
		scaleX: float32(sw) / w,
		scaleY: float32(sh) / h,
		srcW:   sw,
		srcH:   sh,
		stride: src.Stride,
	}
}

func (s *imageSrc) ColorModel() color.Model { return color.NRGBAModel }

func (s *imageSrc) Bounds() image.Rectangle {
	const big = 1 << 29
	return image.Rect(-big, -big, big, big)
}

func (s *imageSrc) At(x, y int) color.Color {
	// Pixel centre → continuous source coordinate → texel centre.
	fx := (float32(x)+0.5-s.dstX)*s.scaleX - 0.5
	fy := (float32(y)+0.5-s.dstY)*s.scaleY - 0.5
	x0 := int(math.Floor(float64(fx)))
	y0 := int(math.Floor(float64(fy)))
	tx := fx - float32(x0)
	ty := fy - float32(y0)

	c00 := s.texel(x0, y0)
	c10 := s.texel(x0+1, y0)
	c01 := s.texel(x0, y0+1)
	c11 := s.texel(x0+1, y0+1)

	lerp := func(a, b uint8, t float32) float32 {
		return float32(a) + (float32(b)-float32(a))*t
	}
	mix := func(i int) uint8 {
		top := lerp(c00[i], c10[i], tx)
		bot := lerp(c01[i], c11[i], tx)
		return uint8(top + (bot-top)*ty + 0.5)
	}
	s.nr = color.NRGBA{R: mix(0), G: mix(1), B: mix(2), A: mix(3)}
	return &s.nr
}

// texel reads one source pixel with edge clamping. x and y are offsets
// into the source, so the index is plain row arithmetic — PixOffset
// subtracts Rect.Min, which x and y already are.
func (s *imageSrc) texel(x, y int) [4]uint8 {
	x = min(max(x, 0), s.srcW-1)
	y = min(max(y, 0), s.srcH-1)
	i := y*s.stride + x*4
	p := s.src.Pix
	return [4]uint8{p[i], p[i+1], p[i+2], p[i+3]}
}

// drawImage paints the optional background fill, then the image itself,
// rounded to ClipRadius.
func (r *renderer) drawImage(cmd *gui.RenderCmd) {
	if cmd.W <= 0 || cmd.H <= 0 {
		return
	}
	s := r.scale
	x, y, w, h := cmd.X*s, cmd.Y*s, cmd.W*s, cmd.H*s
	region := r.buf.region(x, y, w, h)

	if cmd.Color.A > 0 {
		r.buf.fillPath(region, r.buf.solid(cmd.Color),
			func(z *vector.Rasterizer, ox, oy float32) {
				pathRect(z, ox, oy, x, y, w, h)
			})
	}

	src, ok := r.resolveImage(cmd.Resource)
	if !ok {
		return
	}
	rad := cmd.ClipRadius * s
	r.buf.fillPath(region, newImageSrc(src, x, y, w, h),
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
		})
}

// resolveImage decodes and caches the source named by a RenderCmd's
// Resource.
//
// A mem: source names a buffer in gui's in-memory registry: it has no
// path, so it skips path resolution and the AllowedImageRoots sandbox,
// exactly as the GPU backends do (see gui.LookupImage on why that is
// safe).
func (r *renderer) resolveImage(res string) (*image.NRGBA, bool) {
	if img, ok := r.images[res]; ok {
		return img, img != nil
	}
	img := r.decodeImage(res)
	if r.images == nil {
		r.images = make(map[string]*image.NRGBA)
	}
	// A failed decode is cached as nil so one bad path is logged once.
	r.images[res] = img
	return img, img != nil
}

func (r *renderer) decodeImage(res string) *image.NRGBA {
	if iw, ih, pix, ok := gui.LookupImage(res); ok {
		if iw <= 0 || ih <= 0 || len(pix) < iw*ih*4 {
			return nil
		}
		return &image.NRGBA{
			Pix:    pix,
			Stride: iw * 4,
			Rect:   image.Rect(0, 0, iw, ih),
		}
	}
	path, err := imgload.ResolveValidatedPath(res, r.allowedImageRoots)
	if err != nil {
		log.Printf("soft: drawImage: %v", err)
		return nil
	}
	f, err := imgload.OpenSafe(path, r.allowedImageRoots)
	if err != nil {
		log.Printf("soft: drawImage: %v", err)
		return nil
	}
	defer func() { _ = f.Close() }()
	img, err := imgload.DecodeNRGBA(path, f,
		r.maxImageBytes, r.maxImagePixels)
	if err != nil {
		log.Printf("soft: drawImage: %v", err)
		return nil
	}
	return img
}
