package soft

import (
	"image"
	"image/color"
	"log"
	"math"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/memtex"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// imageSrc scales a decoded source into a destination rect, sampling
// bilinearly (or nearest, see nearest) in straight-alpha space at
// absolute device coordinates.
//
// nr is the scratch color each At returns: vector.Draw consumes the
// sample immediately, so one instance can be re-pointed (the same
// reasoning as buffer.solid's reusable image.Uniform).
type imageSrc struct {
	src            *image.NRGBA
	dstX, dstY     float32
	scaleX, scaleY float32 // source pixels per device pixel
	srcW, srcH     int
	minX, minY     int     // source Rect.Min: texel offsets are relative to it
	alpha          float32 // texel alpha multiplier 0..1 from RenderCmd.Opacity
	nr             color.NRGBA
	// nearest takes the one texel under the pixel centre instead of
	// blending four (RenderCmd.ImageFilter == ImageFilterNearest).
	nearest bool
}

// newImageSrc returns a value: drawImage copies it into the renderer's
// scratch sampler, because a returned pointer escaped once per draw
// (#1282).
func newImageSrc(
	src *image.NRGBA, x, y, w, h, alpha float32,
) imageSrc {
	// A nil source samples transparent rather than panicking in
	// Bounds: resolveImage never returns one, but the sampler is
	// one assignment away from it.
	if src == nil {
		src = &image.NRGBA{}
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	// Commands arriving via emitRenderer are validated to 0..1,
	// but drawAll is callable directly: NaN means opaque like
	// renderShape, anything else clamps.
	if alpha != alpha {
		alpha = 1
	} else if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}
	// A degenerate dest rect would divide by zero below; the
	// caller skips empty draws, so clamp defensively instead
	// of producing Inf scales that poison the sampler.
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return imageSrc{
		src:    src,
		dstX:   x,
		dstY:   y,
		scaleX: float32(sw) / w,
		scaleY: float32(sh) / h,
		srcW:   sw,
		srcH:   sh,
		minX:   b.Min.X,
		minY:   b.Min.Y,
		alpha:  alpha,
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
	if s.nearest {
		// The texel whose centre is closest: floor(f + 0.5) undoes the
		// -0.5 above, so this is the texel the pixel centre falls in.
		c := s.texel(int(math.Floor(float64(fx)+0.5)),
			int(math.Floor(float64(fy)+0.5)))
		s.nr = color.NRGBA{
			R: c[0], G: c[1], B: c[2],
			A: uint8(float32(c[3])*s.alpha + 0.5),
		}
		return &s.nr
	}
	x0 := int(math.Floor(float64(fx)))
	y0 := int(math.Floor(float64(fy)))
	tx := fx - float32(x0)
	ty := fy - float32(y0)

	c00 := s.texel(x0, y0)
	c10 := s.texel(x0+1, y0)
	c01 := s.texel(x0, y0+1)
	c11 := s.texel(x0+1, y0+1)

	s.nr = color.NRGBA{
		R: lerpU8(c00[0], c10[0], c01[0], c11[0], tx, ty),
		G: lerpU8(c00[1], c10[1], c01[1], c11[1], tx, ty),
		B: lerpU8(c00[2], c10[2], c01[2], c11[2], tx, ty),
		A: uint8(float32(lerpU8(
			c00[3], c10[3], c01[3], c11[3], tx, ty,
		))*s.alpha + 0.5),
	}
	return &s.nr
}

// lerpU8 bilinearly interpolates one channel over the four
// surrounding texels. A free function so the per-pixel path
// holds no closures.
func lerpU8(c00, c10, c01, c11 uint8, tx, ty float32) uint8 {
	top := float32(c00) + (float32(c10)-float32(c00))*tx
	bot := float32(c01) + (float32(c11)-float32(c01))*tx
	return uint8(top + (bot-top)*ty + 0.5)
}

// texel reads one source pixel with edge clamping. x and y are
// offsets into the source rect, rebased through Rect.Min via
// PixOffset — the canonical index for images whose storage
// starts at a non-zero origin.
func (s *imageSrc) texel(x, y int) [4]uint8 {
	if s.srcW <= 0 || s.srcH <= 0 {
		return [4]uint8{}
	}
	x = min(max(x, 0), s.srcW-1)
	y = min(max(y, 0), s.srcH-1)
	i := s.src.PixOffset(s.minX+x, s.minY+y)
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

	// A fully transparent image still draws its (equally faded)
	// bg above, but skips decode and sampling entirely.
	if cmd.Opacity <= 0 {
		return
	}
	var src *image.NRGBA
	var ok bool
	if cmd.Surface != nil {
		src, ok = r.surfaces.Resolve(cmd.Surface, uploadSurfaceImage)
	} else {
		src, ok = r.resolveImage(cmd.Resource)
	}
	if !ok {
		return
	}
	rad := cmd.ClipRadius * s
	is := &r.imgSrc
	*is = newImageSrc(src, x, y, w, h, cmd.Opacity)
	is.nearest = cmd.ImageFilter == gui.ImageFilterNearest
	r.buf.fillPath(region, is,
		func(z *vector.Rasterizer, ox, oy float32) {
			pathRoundRect(z, ox, oy, x, y, w, h, rad, false)
		})
}

// cachedImage is one entry of the renderer's image cache. img is nil for a
// failed decode, cached so one bad path is logged once. serial is the gui
// registry serial a mem: entry was wrapped from; 0 for every other entry.
type cachedImage struct {
	img    *image.NRGBA
	serial uint64
}

// imageCacheEntries bounds the image cache's entry count, as the GL
// backend's texture cache is bounded (newGLTexCacheLRU).
const imageCacheEntries = 128

// newImageCache returns the bounded image cache for a window whose
// WindowCfg.MaxTextureBytes is maxBytes (zero or negative: the desktop
// default). No destroy function: an evicted image is garbage collected.
func newImageCache(maxBytes int64) texcache.Cache[string, cachedImage] {
	return texcache.NewSized[string, cachedImage](imageCacheEntries,
		texcache.Budget(maxBytes, texcache.DesktopTextureBudget),
		cachedImageBytes, nil)
}

// cachedImageBytes is the image cache's size function. A plain function,
// not a closure, so Set allocates nothing for it.
func cachedImageBytes(c cachedImage) int64 {
	if c.img == nil {
		return 0
	}
	return int64(c.img.Rect.Dx()) * int64(c.img.Rect.Dy()) * 4
}

// keepCachedImage is the image cache's sweep predicate: a mem: entry stays
// only while its key is registered under the serial it was wrapped from.
func keepCachedImage(key string, c cachedImage) bool {
	return memtex.Current(key, c.serial)
}

// resolveImage returns the decoded source a RenderCmd's Resource names,
// from the cache or by decoding it.
//
// A mem: source names a buffer in gui's in-memory registry: it has no
// path, so it skips path resolution and the AllowedImageRoots sandbox,
// exactly as the GPU backends do (see gui.LookupImage on why that is
// safe). It is looked up on every draw, as the GL backend does: the
// cached wrapper is reused only while the registration's serial matches,
// so a key dropped and registered again with new pixels draws the new
// ones. A dropped key draws nothing rather than being opened as a file
// path.
func (r *renderer) resolveImage(res string) (*image.NRGBA, bool) {
	if gui.IsMemImage(res) {
		mi, ok := gui.LookupImage(res)
		if !ok || mi.W <= 0 || mi.H <= 0 || len(mi.Pix) < mi.W*mi.H*4 {
			return nil, false
		}
		if c, hit := r.imageCache.Get(res); hit && c.serial == mi.Serial {
			return c.img, true
		}
		img := &image.NRGBA{
			Pix:    mi.Pix,
			Stride: mi.W * 4,
			Rect:   image.Rect(0, 0, mi.W, mi.H),
		}
		r.imageCache.Set(res, cachedImage{img: img, serial: mi.Serial})
		return img, true
	}
	if c, hit := r.imageCache.Get(res); hit {
		return c.img, c.img != nil
	}
	img := r.decodeImage(res)
	r.imageCache.Set(res, cachedImage{img: img})
	return img, img != nil
}

// decodeImage opens and decodes a file source inside the window's
// AllowedImageRoots and image limits. nil means it failed; the error is
// logged.
func (r *renderer) decodeImage(res string) *image.NRGBA {
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
