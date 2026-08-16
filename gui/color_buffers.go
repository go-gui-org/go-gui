package gui

import "strconv"

// Pixel generators for the color components.
//
// Every generator follows one shape: build a *content key* naming the
// inputs the buffer depends on, hand it to memImageSrc, and only on a miss
// allocate a buffer and fill it. The key is built into a stack scratch
// buffer and the hit path returns the string stored at registration, so a
// steady frame — the common case, since one drag moves one channel —
// costs a map lookup and nothing else.
//
// Keys quantize: hue to whole degrees, saturation/lightness/alpha to whole
// percent. A key that tracked a float exactly would miss on every frame of
// a drag and rebuild the buffer each time, which is the failure this design
// exists to avoid.

// imgScale is how many pixels of generated imagery back one logical point.
// Buffers are built at 2× and displayed at logical size: sharp on HiDPI,
// cheaply downscaled on 1×. Fixed rather than read from BackingScale so the
// scale stays out of every content key.
const imgScale = 2

// colorKeyBuf is the scratch a content key is built into. 64 bytes holds
// the longest key any generator below produces.
type colorKeyBuf [64]byte

// appendQ appends a channel value quantized to an integer, plus a
// separator. Channels the buffer does not depend on pass -1, which keeps
// the key's shape fixed while marking the axis as "swept".
func appendQ(b []byte, v int) []byte {
	b = strconv.AppendInt(b, int64(v), 10)
	return append(b, '-')
}

// quantHSLA renders an HSLA to the integers its keys are built from:
// hue in degrees, the rest in percent.
func quantHSLA(v HSLA) (h, s, l, a int) {
	n := v.Normalized()
	return int(n.H + 0.5), int(n.S*100 + 0.5),
		int(n.L*100 + 0.5), int(n.A*100 + 0.5)
}

// ColorChannel names the HSLA component a color control edits.
type ColorChannel uint8

// Channel constants for ColorChannelSliderCfg.Channel.
const (
	ChannelHue ColorChannel = iota
	ChannelSaturation
	ChannelLightness
	ChannelAlpha
)

// channelSpan is the value range each channel sweeps, in the units the
// caller sees: degrees for hue, percent for saturation and lightness, and
// 0–1 for alpha.
func (ch ColorChannel) span() float32 {
	switch ch {
	case ChannelHue:
		return 360
	case ChannelAlpha:
		return 1
	default:
		return 100
	}
}

// value reads this channel out of an HSLA in the units span() describes.
func (ch ColorChannel) value(v HSLA) float32 {
	switch ch {
	case ChannelHue:
		return v.H
	case ChannelSaturation:
		return v.S * 100
	case ChannelLightness:
		return v.L * 100
	default:
		return v.A
	}
}

// with returns v with this channel set to t (again in span() units).
func (ch ColorChannel) with(v HSLA, t float32) HSLA {
	switch ch {
	case ChannelHue:
		v.H = t
	case ChannelSaturation:
		v.S = t / 100
	case ChannelLightness:
		v.L = t / 100
	default:
		v.A = t
	}
	return v
}

// channelStripSrc returns the track image for one channel: the color the
// slider would pick at each step along it, holding the other channels where
// they are. A vertical strip sweeps bottom to top, so its low end is at the
// bottom of the screen where a vertical slider's low end belongs.
//
// The gradient is inset by half the thumb at each end so the thumb's centre
// always sits on the exact color it selects; the end caps pin to the
// extreme values. Hue, saturation and lightness tracks are drawn opaque —
// alpha is irrelevant to what they pick — while the alpha track composites
// over the checkerboard, which is the only honest way to show it.
func channelStripSrc(
	ch ColorChannel, v HSLA, w, h, inset int, vertical bool,
) string {
	var kb colorKeyBuf
	key := kb[:0]
	key = append(key, "gui/cstrip/"...)
	key = appendQ(key, int(ch))
	key = appendQ(key, w)
	key = appendQ(key, h)
	key = appendQ(key, inset)
	key = appendQ(key, boolInt(vertical))

	qh, qs, ql, _ := quantHSLA(v)
	// A channel does not affect its own strip: the strip sweeps it.
	// Excluding it is what keeps a hue drag from rebuilding the hue
	// strip on every frame.
	switch ch {
	case ChannelHue:
		qh = -1
	case ChannelSaturation:
		qs = -1
	case ChannelLightness:
		ql = -1
	}
	key = appendQ(key, qh)
	key = appendQ(key, qs)
	key = appendQ(key, ql)
	// Alpha is in no strip's key: the H/S/L tracks are drawn opaque, and
	// the alpha track is the one sweeping it.

	if src, ok := memImageSrc(key); ok {
		return src
	}
	return UseImage(string(key), w, h,
		channelStripPixels(ch, v, w, h, inset, vertical))
}

func channelStripPixels(
	ch ColorChannel, v HSLA, w, h, inset int, vertical bool,
) []byte {
	pix := make([]byte, w*h*4)
	// The swept axis is x when horizontal and y when vertical; the other
	// axis just repeats the column (or row) of colors.
	steps, cross := w, h
	if vertical {
		steps, cross = h, w
	}
	track := float32(steps - 2*inset)
	if track <= 0 {
		track = float32(steps)
	}
	span := ch.span()

	for n := range steps {
		t := (float32(n) + 0.5 - float32(inset)) / track
		if vertical {
			// Screen y grows downward; a vertical slider's maximum is
			// at the top, so the sweep runs the other way.
			t = 1 - t
		}
		c := ch.with(v, f32Clamp(t, 0, 1)*span)
		if ch != ChannelAlpha {
			c.A = 1
		}
		col := c.Color()
		for m := range cross {
			x, y := n, m
			if vertical {
				x, y = m, n
			}
			r, g, b := col.R, col.G, col.B
			if ch == ChannelAlpha {
				// Composite onto the checker here rather than layering
				// two views: the strip is one texture, so the checker
				// must not slide independently of it.
				bg := checkerShade(x, y)
				a := float32(col.A) / 255
				r = mix8(bg, r, a)
				g = mix8(bg, g, a)
				b = mix8(bg, b, a)
			}
			i := (y*w + x) * 4
			pix[i+0] = r
			pix[i+1] = g
			pix[i+2] = b
			pix[i+3] = 0xff
		}
	}
	return pix
}

// boolInt renders a flag as a key segment.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// planeSrc returns the saturation(→) × lightness(↑) plane at v's hue.
// Keyed on hue alone: moving saturation or lightness moves the marker
// across a plane that has not changed.
func planeSrc(v HSLA, size int) string {
	var kb colorKeyBuf
	key := kb[:0]
	key = append(key, "gui/cplane/"...)
	key = appendQ(key, size)
	qh, _, _, _ := quantHSLA(v)
	key = appendQ(key, qh)

	if src, ok := memImageSrc(key); ok {
		return src
	}
	return UseImage(string(key), size, size, planePixels(v.H, size))
}

func planePixels(hue float32, size int) []byte {
	pix := make([]byte, size*size*4)
	d := float32(size)
	for y := range size {
		l := 1 - (float32(y)+0.5)/d
		for x := range size {
			s := (float32(x) + 0.5) / d
			col := HSLA{H: hue, S: s, L: l, A: 1}.Color()
			i := (y*size + x) * 4
			pix[i+0] = col.R
			pix[i+1] = col.G
			pix[i+2] = col.B
			pix[i+3] = 0xff
		}
	}
	return pix
}

// wheelSrc returns the hue(angle) × saturation(radius) disc at v's
// lightness. Keyed on lightness alone, quantized to whole percent, which
// caps the disc at 101 distinct buffers however long a drag runs.
func wheelSrc(v HSLA, size int) string {
	var kb colorKeyBuf
	key := kb[:0]
	key = append(key, "gui/cwheel/"...)
	key = appendQ(key, size)
	_, _, ql, _ := quantHSLA(v)
	key = appendQ(key, ql)

	if src, ok := memImageSrc(key); ok {
		return src
	}
	return UseImage(string(key), size, size,
		wheelPixels(float32(ql)/100, size))
}

func wheelPixels(light float32, size int) []byte {
	pix := make([]byte, size*size*4)
	radius := float32(size) / 2
	for y := range size {
		dy := float32(y) + 0.5 - radius
		for x := range size {
			dx := float32(x) + 0.5 - radius
			r := f32Sqrt(dx*dx + dy*dy)
			// One device pixel of edge coverage, so the rim fades into
			// transparency instead of showing a staircase.
			cov := f32Clamp(radius-r+0.5, 0, 1)
			if cov == 0 {
				continue // leaves the pixel fully transparent
			}
			col := HSLA{
				H: hueOf(dx, dy),
				S: f32Clamp(r/radius, 0, 1),
				L: light,
				A: 1,
			}.Color()
			i := (y*size + x) * 4
			pix[i+0] = col.R
			pix[i+1] = col.G
			pix[i+2] = col.B
			pix[i+3] = uint8(255*cov + 0.5)
		}
	}
	return pix
}

// hueOf maps a vector from the wheel centre to a hue angle in degrees:
// 0° points right (+x) and hue increases counterclockwise. Y grows
// downward on screen, hence the negated dy.
func hueOf(dx, dy float32) float32 {
	h := f32Atan2(-dy, dx) * 180 / f32Pi
	if h < 0 {
		h += 360
	}
	return h
}

// checkerSrc returns the transparency checkerboard drawn behind a swatch.
// One buffer serves every swatch on screen, so it is keyed on size alone.
func checkerSrc(w, h int) string {
	var kb colorKeyBuf
	key := kb[:0]
	key = append(key, "gui/checker/"...)
	key = appendQ(key, w)
	key = appendQ(key, h)

	if src, ok := memImageSrc(key); ok {
		return src
	}
	return UseImage(string(key), w, h, checkerPixels(w, h))
}

func checkerPixels(w, h int) []byte {
	pix := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			s := checkerShade(x, y)
			i := (y*w + x) * 4
			pix[i+0] = s
			pix[i+1] = s
			pix[i+2] = s
			pix[i+3] = 0xff
		}
	}
	return pix
}

// checkerCell is the checkerboard square size in generated pixels — 8
// logical points at imgScale 2.
const checkerCell = 8 * imgScale

// checkerShade returns the light or dark shade of the checkerboard cell
// covering a generated pixel.
func checkerShade(x, y int) uint8 {
	if (x/checkerCell+y/checkerCell)%2 == 0 {
		return 0xff
	}
	return 0xcc
}

// mix8 blends fg over bg at alpha a (0–1).
func mix8(bg, fg uint8, a float32) uint8 {
	return uint8(float32(bg)*(1-a) + float32(fg)*a + 0.5)
}
