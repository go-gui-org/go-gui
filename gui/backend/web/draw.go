//go:build js && wasm

package web

import (
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
)

const (
	maxImageCacheSize       = 256
	imageCacheEvictN        = 16
	maxFailedImageCacheSize = 256
	failedImageCacheEvictN  = 16
	colorCacheSize          = 8
)

// colorCacheEntry holds a cached Color→CSS-string mapping.
type colorCacheEntry struct {
	color gui.Color
	css   string
}

// alphaLUT maps byte alpha values 0-255 to CSS alpha strings.
// Pre-computed to avoid per-call allocations and rounding
// errors from integer arithmetic.
var alphaLUT [256]string

func init() {
	alphaLUT[0] = "0"
	alphaLUT[255] = "1"
	for i := 1; i < 255; i++ {
		alphaLUT[i] = strconv.FormatFloat(
			float64(i)/255, 'f', 3, 64)
	}
}

// renderersDraw iterates render commands and draws them.
func (b *Backend) renderersDraw(w *gui.Window) {
	cmds := w.Renderers()
	for i := range cmds {
		r := &cmds[i]
		switch r.Kind {
		case gui.RenderClip:
			b.drawClip(r)
		case gui.RenderRect:
			b.drawRect(r)
		case gui.RenderStrokeRect:
			b.drawStrokeRect(r)
		case gui.RenderText:
			b.drawText(r)
		case gui.RenderCircle:
			b.drawCircle(r)
		case gui.RenderLine:
			b.drawLine(r)
		case gui.RenderShadow:
			b.drawShadow(r)
		case gui.RenderBlur:
			b.drawBlur(r)
		case gui.RenderGradient:
			b.drawGradient(r)
		case gui.RenderGradientBorder:
			b.drawGradientBorder(r)
		case gui.RenderImage:
			b.drawImage(r)
		case gui.RenderSvg:
			b.drawSvg(r)
		case gui.RenderLayout:
			b.drawLayout(r)
		case gui.RenderLayoutTransformed:
			b.drawLayoutTransformed(r)
		case gui.RenderTextPath:
			b.drawTextPath(r)
		case gui.RenderRTF:
			b.drawRtf(r)
		case gui.RenderFilterBegin:
			b.beginFilter(r)
		case gui.RenderFilterEnd:
			b.endFilter()
		case gui.RenderStencilBegin:
			b.beginStencilClip(r)
		case gui.RenderStencilEnd:
			b.endStencilClip()

		case gui.RenderRotateBegin:
			b.beginRotation(r)
		case gui.RenderRotateEnd:
			b.endRotation()
		case gui.RenderCustomShader:
			b.drawCustomShader(r)

		case gui.RenderFilterComposite,
			gui.RenderLayoutPlaced:
			// Unsupported in Canvas2D backend.
		}
	}
}

// --- Individual draw commands ---

func (b *Backend) drawClip(r *gui.RenderCmd) {
	next := clipRegion{
		kind: clipKindRect,
		x:    r.X,
		y:    r.Y,
		w:    r.W,
		h:    r.H,
	}
	if replaced := false; len(b.clipStack) > 0 {
		for i := range slices.Backward(b.clipStack) {
			if b.clipStack[i].kind == clipKindRect {
				b.clipStack[i] = next
				replaced = true
				break
			}
		}
		if !replaced {
			b.clipStack = append(b.clipStack, next)
		}
	} else {
		b.clipStack = append(b.clipStack, next)
	}
	b.rebuildClipStack()
}

func (b *Backend) beginStencilClip(r *gui.RenderCmd) {
	b.clipStack = append(b.clipStack, clipRegion{
		kind:   clipKindStencil,
		x:      r.X,
		y:      r.Y,
		w:      r.W,
		h:      r.H,
		radius: r.Radius,
	})
	b.rebuildClipStack()
}

func (b *Backend) endStencilClip() {
	for i := range slices.Backward(b.clipStack) {
		if b.clipStack[i].kind == clipKindStencil {
			b.clipStack = append(
				b.clipStack[:i], b.clipStack[i+1:]...)
			break
		}
	}
	b.rebuildClipStack()
}

func (b *Backend) rebuildClipStack() {
	for b.clipDepth > 0 {
		b.ctx2d.Call("restore")
		b.clipDepth--
	}
	for _, clip := range b.clipStack {
		b.ctx2d.Call("save")
		b.clipDepth++
		b.ctx2d.Call("beginPath")
		if clip.kind == clipKindStencil && clip.radius > 0 {
			b.roundedRectPath(
				float64(clip.x), float64(clip.y),
				float64(clip.w), float64(clip.h),
				float64(clip.radius))
		} else {
			b.ctx2d.Call("rect",
				float64(clip.x), float64(clip.y),
				float64(clip.w), float64(clip.h))
		}
		b.ctx2d.Call("clip")
	}
}

func (b *Backend) drawRect(r *gui.RenderCmd) {
	if !r.Fill {
		return
	}
	b.setFillColor(r.Color)
	if r.Radius > 0 {
		b.fillRoundedRect(r.X, r.Y, r.W, r.H, r.Radius)
	} else {
		b.ctx2d.Call("fillRect",
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H))
	}
}

func (b *Backend) drawStrokeRect(r *gui.RenderCmd) {
	b.setStrokeColor(r.Color)
	b.ctx2d.Set("lineWidth", max(float64(r.Thickness), 1.0))
	if r.Radius > 0 {
		b.ctx2d.Call("beginPath")
		b.roundedRectPath(
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H),
			float64(r.Radius))
		b.ctx2d.Call("stroke")
	} else {
		b.ctx2d.Call("strokeRect",
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H))
	}
}

func (b *Backend) drawText(r *gui.RenderCmd) {
	if b.textSys == nil || len(r.Text) == 0 {
		return
	}
	if gui.DrawTextTransformed(r, b.textSys,
		glyphconv.GuiStyleToGlyphConfig,
		func(layout glyph.Layout, grad *glyph.GradientConfig) {
			// Web's glyph backend has no DrawTexturedQuadTransformed
			// support; emulate the transform via Canvas2D like
			// drawLayoutTransformed does.
			t := *r.LayoutTransform
			b.ctx2d.Call("save")
			b.ctx2d.Call("transform",
				float64(t.XX), float64(t.YX),
				float64(t.XY), float64(t.YY),
				float64(r.X)+float64(t.X0),
				float64(r.Y)+float64(t.Y0))
			if grad != nil {
				b.textSys.DrawLayoutWithGradient(layout, 0, 0, grad)
			} else {
				b.textSys.DrawLayout(layout, 0, 0)
			}
			b.ctx2d.Call("restore")
		}) {
		return
	}
	cfg := glyphconv.GuiTextConfigFromRender(r)
	if err := b.textSys.DrawText(r.X, r.Y, r.Text, cfg); err != nil && !b.textErrLogged {
		// Warn once per backend: a persistent failure (e.g. missing
		// font) would otherwise log every frame.
		b.textErrLogged = true
		log.Printf("web: DrawText: %v", err)
	}
}

func (b *Backend) drawCircle(r *gui.RenderCmd) {
	if !r.Fill || r.Radius <= 0 {
		return
	}
	b.setFillColor(r.Color)
	b.ctx2d.Call("beginPath")
	b.ctx2d.Call("arc",
		float64(r.X), float64(r.Y),
		float64(r.Radius), 0, 2*math.Pi)
	b.ctx2d.Call("fill")
}

func (b *Backend) drawLine(r *gui.RenderCmd) {
	b.setStrokeColor(r.Color)
	b.ctx2d.Set("lineWidth", max(float64(r.Thickness), 1.0))
	b.ctx2d.Call("beginPath")
	b.ctx2d.Call("moveTo", float64(r.X), float64(r.Y))
	b.ctx2d.Call("lineTo", float64(r.OffsetX), float64(r.OffsetY))
	b.ctx2d.Call("stroke")
}

func (b *Backend) drawShadow(r *gui.RenderCmd) {
	// Canvas has no native shadow spread; the source shape is
	// inflated by Spread instead, so its shadow grows beyond the
	// caster on every side. The container's background (drawn next)
	// covers the caster area, leaving the ring visible.
	s := r.Spread
	if r.BlurRadius <= 0 {
		// Hard shadow.
		b.setFillColor(r.Color)
		x := r.X + r.OffsetX - s
		y := r.Y + r.OffsetY - s
		if r.Radius > 0 {
			b.fillRoundedRect(x, y, r.W+2*s, r.H+2*s, r.Radius+s)
		} else {
			b.ctx2d.Call("fillRect",
				float64(x), float64(y),
				float64(r.W+2*s), float64(r.H+2*s))
		}
		return
	}

	b.ctx2d.Call("save")
	b.ctx2d.Set("shadowColor", b.cssColorCached(r.Color))
	b.ctx2d.Set("shadowBlur", float64(r.BlurRadius))
	b.ctx2d.Set("shadowOffsetX", float64(r.OffsetX))
	b.ctx2d.Set("shadowOffsetY", float64(r.OffsetY))
	// Opaque source so shadow opacity = shadowColor alpha
	// alone, not multiplied by fill alpha. The container's
	// background (drawn next) covers this opaque fill.
	b.ctx2d.Set("fillStyle", "#000")
	if r.Radius > 0 {
		b.fillRoundedRect(r.X-s, r.Y-s, r.W+2*s, r.H+2*s,
			r.Radius+s)
	} else {
		b.ctx2d.Call("fillRect",
			float64(r.X-s), float64(r.Y-s),
			float64(r.W+2*s), float64(r.H+2*s))
	}
	b.ctx2d.Call("restore")
}

func (b *Backend) drawBlur(r *gui.RenderCmd) {
	b.ctx2d.Call("save")
	b.ctx2d.Set("filter",
		"blur("+ftoaGeneral(float64(r.BlurRadius))+"px)")
	b.setFillColor(r.Color)
	b.ctx2d.Call("fillRect",
		float64(r.X), float64(r.Y),
		float64(r.W), float64(r.H))
	b.ctx2d.Call("restore")
}

func (b *Backend) drawGradient(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 ||
		r.W <= 0 || r.H <= 0 {
		return
	}
	stops := gui.NormalizeGradientStops(
		r.Gradient.Stops, &b.normBuf)
	if len(stops) == 0 {
		return
	}

	var grad js.Value
	if r.Gradient.Type == gui.GradientRadial {
		cx := float64(r.X + r.W/2)
		cy := float64(r.Y + r.H/2)
		radius := math.Max(float64(r.W/2), float64(r.H/2))
		grad = b.ctx2d.Call("createRadialGradient",
			cx, cy, 0, cx, cy, radius)
	} else {
		dx, dy := gui.GradientDir(r.Gradient, r.W, r.H)
		cx := float64(r.X + r.W/2)
		cy := float64(r.Y + r.H/2)
		// Scale unit direction vector to span the rectangle.
		halfLen := math.Abs(float64(dx))*float64(r.W)/2 +
			math.Abs(float64(dy))*float64(r.H)/2
		hx := float64(dx) * halfLen
		hy := float64(dy) * halfLen
		grad = b.ctx2d.Call("createLinearGradient",
			cx-hx, cy-hy, cx+hx, cy+hy)
	}

	for _, s := range stops {
		grad.Call("addColorStop",
			float64(s.Pos), b.cssColorBuf(s.Color))
	}

	b.ctx2d.Set("fillStyle", grad)
	if r.Radius > 0 {
		b.ctx2d.Call("beginPath")
		b.roundedRectPath(
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H),
			float64(r.Radius))
		b.ctx2d.Call("fill")
	} else {
		b.ctx2d.Call("fillRect",
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H))
	}
}

func (b *Backend) drawGradientBorder(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 {
		return
	}
	stops := gui.NormalizeGradientStops(
		r.Gradient.Stops, &b.normBuf)
	if len(stops) == 0 {
		return
	}
	th := r.Thickness
	positions := [4]float32{0.0, 0.25, 0.5, 0.75}
	type rect struct{ x, y, w, h float32 }
	rects := [4]rect{
		{r.X, r.Y, r.W, th},            // top
		{r.X, r.Y + r.H - th, r.W, th}, // bottom
		{r.X, r.Y, th, r.H},            // left
		{r.X + r.W - th, r.Y, th, r.H}, // right
	}
	for i := range 4 {
		c := gui.SampleGradientStopColor(stops, positions[i])
		b.setFillColor(c)
		rc := rects[i]
		b.ctx2d.Call("fillRect",
			float64(rc.x), float64(rc.y),
			float64(rc.w), float64(rc.h))
	}
}

func (b *Backend) drawImage(r *gui.RenderCmd) {
	img, ok := b.resolveImageSource(r.Resource)
	if !ok {
		return
	}
	// Fill background.
	if r.Color.A > 0 {
		b.setFillColor(r.Color)
		b.ctx2d.Call("fillRect",
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H))
	}
	if r.ClipRadius > 0 {
		b.ctx2d.Call("save")
		b.ctx2d.Call("beginPath")
		b.roundedRectPath(
			float64(r.X), float64(r.Y),
			float64(r.W), float64(r.H),
			float64(r.ClipRadius))
		b.ctx2d.Call("clip")
	}
	b.ctx2d.Call("drawImage", img,
		float64(r.X), float64(r.Y),
		float64(r.W), float64(r.H))
	if r.ClipRadius > 0 {
		b.ctx2d.Call("restore")
	}
}

// resolveImageSource returns a canvas drawImage source for a render
// command's Resource: an offscreen canvas for a mem: buffer, an Image
// element for a path, URL, or data URL. ok=false means nothing is
// drawable this frame — blocked scheme, download in flight, or a
// permanently broken source.
func (b *Backend) resolveImageSource(res string) (js.Value, bool) {
	// A mem: source names a buffer in gui's in-memory registry. It has
	// no URL to load, so it is painted into an offscreen canvas once
	// and cached alongside the Image elements; a canvas is a valid
	// drawImage source. Callers content-key, so a changed buffer
	// arrives as a new key and never reuses this canvas.
	if iw, ih, pix, ok := gui.LookupImage(res); ok {
		if c, hit := b.imgCache[res]; hit {
			return c, true
		}
		b.evictImageCache()
		c := b.memImageCanvas(iw, ih, pix)
		b.imgCache[res] = c
		return c, true
	}

	if _, failed := b.failedImages[res]; failed {
		return js.Value{}, false
	}
	img, ok := b.imgCache[res]
	if !ok {
		if !isAllowedImageSrc(res) {
			scheme := res
			if s, _, ok := strings.Cut(res, ":"); ok {
				scheme = s
			}
			log.Printf("web: blocked image src scheme: %q",
				scheme)
			return js.Value{}, false
		}
		b.evictImageCache()
		img = js.Global().Get("Image").New()
		img.Set("src", res)
		b.imgCache[res] = img
	}
	if !img.Get("complete").Bool() {
		return js.Value{}, false
	}
	// Loaded but broken (e.g. 404) — track in failedImages
	// to prevent eternal retry. Keep the negative cache bounded
	// so a stream of unique bad URLs cannot grow session memory
	// without limit.
	if img.Get("naturalWidth").Int() == 0 {
		if len(b.failedImages) >= maxFailedImageCacheSize {
			n := 0
			for k := range b.failedImages {
				delete(b.failedImages, k)
				n++
				if n >= failedImageCacheEvictN {
					break
				}
			}
		}
		b.failedImages[res] = struct{}{}
		delete(b.imgCache, res)
		return js.Value{}, false
	}
	return img, true
}

// evictImageCache drops a batch of random entries when the cache is
// full. Random eviction is O(1) with no bookkeeping; batch eviction
// amortizes the overhead for image-heavy UIs.
func (b *Backend) evictImageCache() {
	if len(b.imgCache) < maxImageCacheSize {
		return
	}
	n := 0
	for k := range b.imgCache {
		delete(b.imgCache, k)
		n++
		if n >= imageCacheEvictN {
			break
		}
	}
}

// memImageCanvas paints an NRGBA8 buffer into an offscreen canvas.
// ImageData is non-premultiplied RGBA, the same layout the registry
// stores, so the bytes transfer with no conversion.
func (b *Backend) memImageCanvas(
	w, h int, pix []byte,
) js.Value {
	canvas := js.Global().Get("document").
		Call("createElement", "canvas")
	canvas.Set("width", w)
	canvas.Set("height", h)
	ctx := canvas.Call("getContext", "2d")

	buf := js.Global().Get("Uint8Array").New(len(pix))
	js.CopyBytesToJS(buf, pix)
	data := ctx.Call("createImageData", w, h)
	data.Get("data").Call("set", buf)
	ctx.Call("putImageData", data, 0, 0)
	return canvas
}

func (b *Backend) drawSvg(r *gui.RenderCmd) {
	if r.IsClipMask {
		return
	}
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 {
		return
	}
	numVerts := len(r.Triangles) / 2
	hasVCols := len(r.VertexColors) == numVerts
	vAlpha := float32(1)
	if r.HasVertexAlpha {
		vAlpha = max(0, min(r.VertexAlphaScale, 1))
	}

	hasXform := r.HasXform
	var sx, sy, tx, ty float32
	if hasXform {
		sx, sy, tx, ty = r.ScaleX, r.ScaleY, r.TransX, r.TransY
	}
	hasRot := r.RotAngle != 0
	var sinA, cosA, rcx, rcy float32
	if hasRot {
		rad := float64(r.RotAngle) * math.Pi / 180
		sinA = float32(math.Sin(rad))
		cosA = float32(math.Cos(rad))
		rcx, rcy = r.RotCX, r.RotCY
	}

	addTri := func(i int) {
		for j := range 3 {
			vi := i + j
			vx := r.Triangles[vi*2]
			vy := r.Triangles[vi*2+1]
			if hasXform {
				vx = vx*sx + tx
				vy = vy*sy + ty
			}
			if hasRot {
				dx := vx - rcx
				dy := vy - rcy
				vx = rcx + dx*cosA - dy*sinA
				vy = rcy + dx*sinA + dy*cosA
			}
			px := float64(r.X + vx*r.Scale)
			py := float64(r.Y + vy*r.Scale)
			if j == 0 {
				b.ctx2d.Call("moveTo", px, py)
			} else {
				b.ctx2d.Call("lineTo", px, py)
			}
		}
		b.ctx2d.Call("closePath")
	}

	if !hasVCols {
		// All triangles share the same color — single path.
		b.setFillColor(r.Color)
		b.ctx2d.Call("beginPath")
		for i := 0; i < numVerts; i += 3 {
			addTri(i)
		}
		b.ctx2d.Call("fill")
		return
	}

	// Batch consecutive same-color triangles into one path.
	var batchColor gui.Color
	batchOpen := false
	for i := 0; i < numVerts; i += 3 {
		vc := r.VertexColors[i]
		alpha := vc.A
		if r.HasVertexAlpha {
			alpha = uint8(float32(alpha) * vAlpha)
		}
		c := gui.RGBA(vc.R, vc.G, vc.B, alpha)
		if !batchOpen || c != batchColor {
			if batchOpen {
				b.ctx2d.Call("fill")
			}
			batchColor = c
			b.setFillColor(c)
			b.ctx2d.Call("beginPath")
			batchOpen = true
		}
		addTri(i)
	}
	if batchOpen {
		b.ctx2d.Call("fill")
	}
}

func (b *Backend) drawLayout(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, r.X, r.Y, r.TextGradient)
		return
	}
	b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
}

func (b *Backend) drawLayoutTransformed(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil ||
		r.LayoutTransform == nil {
		return
	}
	// Apply the affine transform at the go-gui canvas level,
	// then render through the identity DrawLayout/WithGradient
	// path which is known to work in Canvas2D.
	t := *r.LayoutTransform
	b.ctx2d.Call("save")
	b.ctx2d.Call("transform",
		float64(t.XX), float64(t.YX),
		float64(t.XY), float64(t.YY),
		float64(r.X)+float64(t.X0),
		float64(r.Y)+float64(t.Y0))
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, 0, 0, r.TextGradient)
	} else {
		b.textSys.DrawLayout(*r.LayoutPtr, 0, 0)
	}
	b.ctx2d.Call("restore")
}

func (b *Backend) drawTextPath(r *gui.RenderCmd) {
	layout, placements, err := gui.ComputeTextPathPlacements(
		r, b.textSys, &b.textPathPlacements,
		glyphconv.GuiStyleToGlyphConfig)
	if err != nil {
		log.Printf("web: drawTextPath: %v", err)
		return
	}
	if len(placements) == 0 {
		return
	}
	b.textPathPlacements = placements
	b.textSys.DrawLayoutPlaced(layout, placements)
}

func (b *Backend) drawRtf(r *gui.RenderCmd) {
	b.drawLayout(r)
}

func (b *Backend) beginFilter(r *gui.RenderCmd) {
	b.ctx2d.Call("save")
	if r.BlurRadius > 0 {
		b.ctx2d.Set("filter",
			"blur("+ftoaGeneral(float64(r.BlurRadius))+"px)")
	}
}

func (b *Backend) endFilter() {
	b.ctx2d.Call("restore")
}
