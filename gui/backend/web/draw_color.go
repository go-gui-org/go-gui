//go:build js && wasm

package web

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

func (b *Backend) setFillColor(c gui.Color) {
	b.ctx2d.Set("fillStyle", b.cssColorCached(c))
}

func (b *Backend) setStrokeColor(c gui.Color) {
	b.ctx2d.Set("strokeStyle", b.cssColorCached(c))
}

func (b *Backend) cssColorCached(c gui.Color) string {
	for i := range b.colorCacheLen {
		if b.colorCache[i].color == c {
			return b.colorCache[i].css
		}
	}
	s := b.cssColorBuf(c)
	b.colorCache[b.colorCacheIdx] = colorCacheEntry{
		color: c, css: s,
	}
	b.colorCacheIdx = (b.colorCacheIdx + 1) % colorCacheSize
	if b.colorCacheLen < colorCacheSize {
		b.colorCacheLen++
	}
	return s
}

func (b *Backend) beginRotation(r *gui.RenderCmd) {
	b.ctx2d.Call("save")
	cx := float64(r.RotCX)
	cy := float64(r.RotCY)
	rad := float64(r.RotAngle) * math.Pi / 180
	b.ctx2d.Call("translate", cx, cy)
	b.ctx2d.Call("rotate", rad)
	b.ctx2d.Call("translate", -cx, -cy)
}

func (b *Backend) endRotation() {
	b.ctx2d.Call("restore")
}

func (b *Backend) fillRoundedRect(
	x, y, w, h, radius float32) {
	b.ctx2d.Call("beginPath")
	b.roundedRectPath(float64(x), float64(y),
		float64(w), float64(h), float64(radius))
	b.ctx2d.Call("fill")
}

// roundedRectPath adds a rounded-rectangle path to the current
// context. Uses the native roundRect method when available
// (Safari 15.4+, all Chrome/Firefox); falls back to arcTo
// construction for older browsers.
//
// NaN/Inf radius is clamped to 0. Degenerate dimensions (NaN,
// Inf, ≤0) produce an empty zero-area rect so the caller's
// subsequent fill/stroke/clip is a no-op.
func (b *Backend) roundedRectPath(
	x, y, w, h, radius float64,
) {
	// Clamp NaN/Inf radius to 0 — NaN comparisons always
	// return false, so they'd escape the radius≤0 fast-path.
	if math.IsNaN(radius) || math.IsInf(radius, 0) {
		radius = 0
	}
	if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 ||
		math.IsNaN(h) || math.IsInf(h, 0) || h <= 0 {
		b.ctx2d.Call("rect", 0, 0, 0, 0)
		return
	}
	if radius <= 0 || !b.hasRoundRect {
		// Fallback via arcTo — works everywhere.
		if radius <= 0 {
			b.ctx2d.Call("rect", x, y, w, h)
			return
		}
		b.ctx2d.Call("moveTo", x+radius, y)
		b.ctx2d.Call("arcTo", x+w, y, x+w, y+radius, radius)
		b.ctx2d.Call("arcTo", x+w, y+h, x+w-radius, y+h, radius)
		b.ctx2d.Call("arcTo", x, y+h, x, y+h-radius, radius)
		b.ctx2d.Call("arcTo", x, y, x+radius, y, radius)
		b.ctx2d.Call("closePath")
		return
	}
	b.ctx2d.Call("roundRect", x, y, w, h, radius)
}

// cssColorBuf formats c into b.colorBuf and returns the
// string. Reuses the buffer across calls, producing one
// allocation per call (the string conversion).
func (b *Backend) cssColorBuf(c gui.Color) string {
	buf := b.colorBuf[:0]
	if c.A == 255 {
		buf = append(buf, "rgb("...)
		buf = appendUint8(buf, c.R)
		buf = append(buf, ',')
		buf = appendUint8(buf, c.G)
		buf = append(buf, ',')
		buf = appendUint8(buf, c.B)
	} else {
		buf = append(buf, "rgba("...)
		buf = appendUint8(buf, c.R)
		buf = append(buf, ',')
		buf = appendUint8(buf, c.G)
		buf = append(buf, ',')
		buf = appendUint8(buf, c.B)
		buf = append(buf, ',')
		buf = appendAlpha(buf, c.A)
	}
	buf = append(buf, ')')
	b.colorBuf = buf
	return string(buf)
}

func appendUint8(buf []byte, v uint8) []byte {
	if v < 10 {
		return append(buf, byte('0'+v))
	}
	if v < 100 {
		return append(buf, byte('0'+v/10), byte('0'+v%10))
	}
	return append(buf, byte('0'+v/100),
		byte('0'+(v/10)%10), byte('0'+v%10))
}

func appendAlpha(buf []byte, a uint8) []byte {
	return append(buf, alphaLUT[a]...)
}

// isAllowedImageSrc validates that src uses a safe scheme.
// Allows data:, http(s):, blob:, and relative paths. Blocks
// exotic schemes like javascript:.
func isAllowedImageSrc(src string) bool {
	for i := range len(src) {
		switch src[i] {
		case ':':
			p := src[:i]
			return strings.EqualFold(p, "data") ||
				strings.EqualFold(p, "http") ||
				strings.EqualFold(p, "https") ||
				strings.EqualFold(p, "blob")
		case '/', '?', '#':
			return true // relative URL
		}
	}
	return len(src) > 0 // plain filename
}

func itoa(i int) string {
	if i < 0 {
		return "-" + uitoa(uint(-i))
	}
	return uitoa(uint(i))
}

func uitoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}

// ftoaGeneral formats an arbitrary non-negative float for CSS
// property values (e.g. blur radius).
func ftoaGeneral(f float64) string {
	if f <= 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
