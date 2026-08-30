package main

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// Procedural surface textures for the planets.
//
// The renderer has no textured-triangle path: DrawContext offers flat
// fills, gradients, an axis-aligned Image, and FillTrianglesColors —
// per-vertex colors with no texture coordinates anywhere in the
// pipeline. Pre-baking a sphere image per frame is worse than it
// sounds, because gui.UseImage is content-keyed with no invalidation
// and the backend texture cache is a small LRU; nine spinning planets
// would evict it every frame.
//
// So the surface is sampled on the CPU, once per mesh vertex, and
// folded into the vertex color the mesh already carries (see
// appendRing). Nothing in gui/ changes and every backend keeps
// working. The price is that detail is bounded by vertex density and
// Gouraud-interpolated between vertices, which is why drawBody's
// tessellation caps are set where they are.
//
// Generation runs once at init and may use whatever math it likes;
// only sampling is on the frame path, and sampling is a clamp, a
// multiply and an array index.

const (
	// texW and texH are longitude and latitude resolution. 128x64 is
	// matched to the mesh's own ceiling — more texels than a body can
	// resolve at maximum tessellation would be detail nothing samples.
	texW = 128
	texH = 64
)

// bodyTexture is one planet's surface albedo on a lat/lon grid.
//
// Rows are uniform in sin(latitude), not in latitude. That makes every
// row band cover the same area of the sphere, so texel density is even
// instead of crowding the poles — and, because the mesh hands sampling
// a direction whose sin(latitude) it already has, it removes an asin
// from the per-vertex path entirely.
type texLevel struct {
	w, h  int
	texel []gui.Color
}

// bodyTexture is that grid at a pyramid of resolutions: lod[0] is full
// detail and each level after it is a box filter of the one above, half
// the columns and half the rows.
//
// The pyramid exists because the mesh cannot carry full detail. Vertices
// are laid out in the *light* basis, so at a small body's tessellation
// one vertex step spans about four texels — four times past the rate at
// which the surface can be point-sampled without aliasing. The grid also
// stands still on screen while the body turns underneath it, so the
// alias pattern does not merely look wrong, it crawls: rings of moire
// concentric with the sub-solar point, sweeping the lit side. Mercury
// showed it worst because its craters are a threshold on a noise field,
// and a step edge has no highest frequency to be under.
//
// Prefiltering is the fix and more vertices are not. Doubling the mesh
// buys one octave for four times the triangles; picking a level matched
// to the vertex spacing removes every octave the mesh cannot represent,
// for eight texel reads.
type bodyTexture struct {
	lod []texLevel
}

// maxLodDim stops the pyramid while a level still has enough rows to
// interpolate over. Below this the surface is its mean color and a
// further level would say nothing new.
const maxLodDim = 4

// at samples the surface at a latitude given as its sine and a
// longitude given in turns. turn is wrapped, so a caller may add spin
// to it without normalizing; sinLat is clamped, which only matters for
// float error at the poles.
//
// The sample is bilinear, and that is not a quality nicety: it is what
// keeps a spinning planet from jittering. The mesh's vertices are laid
// out in the *light* basis, so they sit still on screen while the body
// turns underneath them. A nearest-texel lookup therefore holds each
// vertex's color constant for a whole texel of spin and then snaps —
// 1/128 of a turn, about 3 px of feature travel on a mid-size disc —
// and every vertex snaps at its own moment, which reads as the surface
// crawling rather than rotating. Interpolating makes each vertex's
// color a continuous function of spin, so the pattern slides.
//
// Longitude wraps between the last column and the first, because the
// grid is a full circle. Latitude clamps, because the rows stop at the
// poles.
func (t *texLevel) at(sinLat, turn float32) gui.Color {
	// Clamp before the float-to-int below, not after: NaN compares
	// false against both bounds, so a clamp on the integer row would
	// let it slip through with an unspecified value. Catch it with a
	// self-comparison and fold it to the north pole — any defined row
	// satisfies the contract, and the sampling path cannot produce
	// NaN in the first place. Overshoot folds to the nearest pole.
	if sinLat > 1 {
		sinLat = 1
	} else if sinLat < -1 {
		sinLat = -1
	} else if sinLat != sinLat {
		sinLat = 1
	}
	// Row centers sit at sinLat = 1 - 2*(row+0.5)/h (see makeTexture),
	// so inverting gives a continuous coordinate whose integers land on
	// centers once the half-texel is taken back off.
	ry := (1-sinLat)*0.5*float32(t.h) - 0.5
	r0 := int(floor32(ry))
	fy := ry - float32(r0)
	r1 := r0 + 1
	// Above the first center and below the last there is no second row
	// to blend with; clamping both ends onto the same row makes the
	// blend a no-op there instead of a wrap onto the far pole.
	if r0 < 0 {
		r0 = 0
	} else if r0 >= t.h {
		r0 = t.h - 1
	}
	if r1 < 0 {
		r1 = 0
	} else if r1 >= t.h {
		r1 = t.h - 1
	}

	// Wrap into [0,1) without math.Floor: the truncation toward zero
	// is corrected by the negative branch. NaN is caught here for the
	// same reason it is caught above, and it has to be caught *here*
	// rather than on the column: int(NaN) is implementation-defined,
	// and on amd64 it is math.MinInt64, which no clamp on the column
	// recovers from before the index.
	f := turn - float32(int(turn))
	if f != f {
		f = 0
	} else if f < 0 {
		f++
	}
	rx := f*float32(t.w) - 0.5
	c0 := int(floor32(rx))
	fx := rx - float32(c0)
	// f is in [0,1), so rx is in [-0.5, w-0.5) and c0 is in [-1, w-1]:
	// it can only be one step below zero, never past the far end. One
	// conditional is enough; no modulo needed.
	if c0 < 0 {
		c0 += t.w
	}
	c1 := c0 + 1
	if c1 >= t.w {
		c1 = 0
	}

	a := t.texel[r0*t.w+c0]
	b := t.texel[r0*t.w+c1]
	c := t.texel[r1*t.w+c0]
	d := t.texel[r1*t.w+c1]
	return gui.RGB(
		bilerp8(a.R, b.R, c.R, d.R, fx, fy),
		bilerp8(a.G, b.G, c.G, d.G, fx, fy),
		bilerp8(a.B, b.B, c.B, d.B, fx, fy),
	)
}

// lerp8 blends two bytes. Rounding is by the +0.5 truncation rather
// than chan8's clamp or mixColor, because a convex combination of two
// bytes cannot leave the byte range and the clamp would be dead work
// on a per-vertex path.
func lerp8(a, b uint8, t float32) uint8 {
	return uint8(lerp(float32(a), float32(b), t) + 0.5)
}

// bilerp8 blends the four corners of one texel cell on a single
// channel. It rounds the way lerp8 does but only once, at the end:
// rounding the two row blends first would cost a count of bias on
// every sample.
func bilerp8(a, b, c, d uint8, fx, fy float32) uint8 {
	top := lerp(float32(a), float32(b), fx)
	bot := lerp(float32(c), float32(d), fx)
	return uint8(lerp(top, bot, fy) + 0.5)
}

// at samples the finest level. Kept for callers with no footprint to
// offer — the tests, and any sampling that is not per mesh vertex.
func (t *bodyTexture) at(sinLat, turn float32) gui.Color {
	return t.lod[0].at(sinLat, turn)
}

// atLod samples at a fractional pyramid level, blending the two levels
// that bracket it.
//
// The blend is what keeps the level choice invisible. Adjacent rings of
// one body land on different footprints and so on different levels; a
// hard switch would draw that boundary as a visible ring of changing
// sharpness, which is the artifact this whole path exists to remove.
func (t *bodyTexture) atLod(lod, sinLat, turn float32) gui.Color {
	if lod <= 0 {
		return t.lod[0].at(sinLat, turn)
	}
	top := len(t.lod) - 1
	if lod >= float32(top) {
		return t.lod[top].at(sinLat, turn)
	}
	i := int(lod)
	f := lod - float32(i)
	a := t.lod[i].at(sinLat, turn)
	b := t.lod[i+1].at(sinLat, turn)
	return gui.RGB(
		lerp8(a.R, b.R, f), lerp8(a.G, b.G, f), lerp8(a.B, b.B, f))
}

// maxLod is the coarsest level the pyramid holds, as the caller's
// clamp bound.
func (t *bodyTexture) maxLod() float32 { return float32(len(t.lod) - 1) }

// lodFor turns a vertex spacing, in radians of arc on the unit sphere,
// into a pyramid level.
//
// The reference is one texel of the longitude grid at the equator,
// 2*pi/texW: level 0 resolves that, and every level after it doubles
// the span. A spacing at or below one texel asks for full detail and
// clamps to zero.
func (t *bodyTexture) lodFor(ds float32) float32 {
	l := log2f(ds * float32(texW) * (1 / (2 * math.Pi)))
	return clamp32(l, 0, t.maxLod())
}

// log2f is log2 for positive floats, taken from the exponent field with
// the mantissa folded in linearly. Worst case is about 0.086 of a level,
// which moves the blend weight and never the choice of levels — and it
// costs no library call on a path that runs once per ring.
func log2f(v float32) float32 {
	if v <= 0 {
		return 0
	}
	b := math.Float32bits(v)
	e := float32(int32(b>>23) - 127)
	m := math.Float32frombits((b & 0x007FFFFF) | 0x3F800000)
	return e + (m - 1)
}

// downsample box-filters a surface into one of half the columns and
// half the rows, in place of a new buffer.
//
// It works on the unquantized floats rather than on the level above's
// bytes, and that is deliberate: rounding to bytes at every level
// compounds its own bias, so a five-level pyramid built from bytes
// walks its mean off the planet's table color. Filtering the source
// once per level keeps every level's mean the source's mean, and each
// is quantized exactly once.
//
// A plain 2x2 average is area-correct here and would not be on most
// lat/lon grids: rows are uniform in sin(latitude), so every texel of
// every row covers the same area of the sphere and none of them needs
// weighting.
func downsample(src []rgbF, w, h int) ([]rgbF, int, int) {
	dw, dh := w/2, h/2
	dst := make([]rgbF, dw*dh)
	for row := range dh {
		s0 := (row * 2) * w
		s1 := s0 + w
		for col := range dw {
			c0, c1 := col*2, col*2+1
			sum := src[s0+c0].add(src[s0+c1]).
				add(src[s1+c0]).add(src[s1+c1])
			dst[row*dw+col] = sum.scale(0.25)
		}
	}
	return dst, dw, dh
}

// quantize freezes a float surface into one pyramid level.
func quantize(src []rgbF, w, h int) texLevel {
	l := texLevel{w: w, h: h, texel: make([]gui.Color, len(src))}
	for i, c := range src {
		l.texel[i] = c.color()
	}
	return l
}

// planetTextures is built once for the process, not once per App:
// newApp runs in nearly every test, and the tables below are the same
// every time. This follows sunGranules exactly, and for the reason
// stated there — a surface regenerated per frame would boil.
var planetTextures = makeTextures()

// rgbF is a texel under construction. Generation works in unclamped
// floats so a chain of tints and darkenings does not quantize at every
// step, and only the final write clamps to bytes.
type rgbF struct{ r, g, b float32 }

func rgbOf(c gui.Color) rgbF {
	return rgbF{float32(c.R), float32(c.G), float32(c.B)}
}

func (c rgbF) scale(f float32) rgbF {
	return rgbF{c.r * f, c.g * f, c.b * f}
}

func (c rgbF) add(o rgbF) rgbF {
	return rgbF{c.r + o.r, c.g + o.g, c.b + o.b}
}

func (c rgbF) mix(o rgbF, t float32) rgbF {
	return rgbF{lerp(c.r, o.r, t), lerp(c.g, o.g, t), lerp(c.b, o.b, t)}
}

func (c rgbF) color() gui.Color {
	return gui.RGB(chan8(c.r), chan8(c.g), chan8(c.b))
}

// --- noise kernel ---
//
// Noise is evaluated on the 3D direction vector rather than on the
// (u,v) grid. That costs one extra multiply and buys two things no 2D
// scheme gets for free: the field is seamless in longitude because
// the direction at u=0 and u=1 is literally the same point, and there
// is no polar pinch, because the sphere has no distinguished axis in
// the noise's coordinates.

// hash3 hashes an integer lattice point to [0,1).
func hash3(x, y, z int32, seed uint32) float32 {
	h := uint32(x)*0x8DA6B343 ^ uint32(y)*0xD8163841 ^ uint32(z)*0xCB1AB31F
	h += seed * 0x9E3779B1
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 12
	h *= 0x297A2D39
	h ^= h >> 15
	return float32(h&0xFFFFFF) / float32(0x1000000)
}

func ifloor(v float32) int32 { return int32(floor32(v)) }

// valueNoise3 is trilinear value noise with a smoothstep fade, so the
// field is continuous and has a continuous first derivative — enough
// for fbm, and cheaper than gradient noise to state.
func valueNoise3(x, y, z float32, seed uint32) float32 {
	xi, yi, zi := ifloor(x), ifloor(y), ifloor(z)
	fx, fy, fz := x-float32(xi), y-float32(yi), z-float32(zi)
	ux := fx * fx * (3 - 2*fx)
	uy := fy * fy * (3 - 2*fy)
	uz := fz * fz * (3 - 2*fz)

	c00 := lerp(hash3(xi, yi, zi, seed), hash3(xi+1, yi, zi, seed), ux)
	c10 := lerp(hash3(xi, yi+1, zi, seed), hash3(xi+1, yi+1, zi, seed), ux)
	c01 := lerp(hash3(xi, yi, zi+1, seed), hash3(xi+1, yi, zi+1, seed), ux)
	c11 := lerp(hash3(xi, yi+1, zi+1, seed), hash3(xi+1, yi+1, zi+1, seed), ux)

	return lerp(lerp(c00, c10, uy), lerp(c01, c11, uy), uz)
}

// fbm3 sums octaves at doubling frequency and halving amplitude, and
// normalizes back to roughly [0,1].
func fbm3(x, y, z float32, oct int, seed uint32) float32 {
	amp, freq := float32(0.5), float32(1)
	var sum, norm float32
	for range oct {
		sum += amp * valueNoise3(x*freq, y*freq, z*freq, seed)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return sum / norm
}

// warp3 displaces a sample point by a vector-valued noise field.
// Domain warping is what turns the smooth blobs of plain fbm into
// something that reads as flow, and it is the whole of Venus.
func warp3(x, y, z, amt float32, seed uint32) (float32, float32, float32) {
	dx := fbm3(x+11.3, y+7.1, z+3.7, 3, seed^0x1234) - 0.5
	dy := fbm3(x+5.9, y+19.2, z+13.1, 3, seed^0x5678) - 0.5
	dz := fbm3(x+2.3, y+31.7, z+23.9, 3, seed^0x9ABC) - 0.5
	return x + amt*2*dx, y + amt*2*dy, z + amt*2*dz
}

// spot is an elliptical feature — a storm — placed by latitude and
// longitude, returning 1 at its center and falling to 0 at its edge.
// wLon and wLat are its half-widths in radians.
func spot(lat, lon, atLat, atLon, wLon, wLat float32) float32 {
	dLon := lon - atLon
	for dLon > math.Pi {
		dLon -= 2 * math.Pi
	}
	for dLon < -math.Pi {
		dLon += 2 * math.Pi
	}
	// Longitude degrees converge toward the poles; scaling by cos(lat)
	// keeps the storm round on the globe rather than in the table.
	dLon *= cos32(lat)
	dx, dy := dLon/wLon, (lat-atLat)/wLat
	d := dx*dx + dy*dy
	if d >= 1 {
		return 0
	}
	return 1 - d
}

// --- per-body surfaces ---
//
// Each takes the planet's own base color and the sample direction, and
// returns an albedo near that base. Staying near the base is not
// decoration: the same color drives the nav dots, the labels and the
// flat tone a sub-flatBodyRadius body draws, and a texture that
// wandered off it would make a zoomed-out planet a different planet.

type texFn func(base rgbF, x, y, z, sinLat, lat, lon float32) rgbF

const (
	seedMercury = 0x4D4552
	seedVenus   = 0x56454E
	seedEarth   = 0x454152
	seedMars    = 0x4D4152
	seedJupiter = 0x4A5550
	seedSaturn  = 0x534154
	seedUranus  = 0x555241
	seedNeptune = 0x4E4550
)

// texMercury: battered grey rock. The craters are a proxy, not a
// simulation — a high-frequency band of the same noise, darkened in
// its floors and lifted just outside them, which reads as rims at the
// size these ever get drawn.
func texMercury(base rgbF, x, y, z, _, _, _ float32) rgbF {
	w := fbm3(x*3.1, y*3.1, z*3.1, 5, seedMercury)
	c := base.scale(0.84 + 0.32*w)
	switch k := fbm3(x*8, y*8, z*8, 2, seedMercury^0x77); {
	case k > 0.62:
		c = c.scale(0.80)
	case k > 0.55:
		c = c.scale(1.12)
	}
	return c
}

// texVenus: a featureless sulphuric overcast. Heavy domain warp on a
// field stretched along longitude gives the smeared banding the real
// cloud deck shows, and the contrast stays tiny (+/-8%) because Venus
// genuinely has almost none.
func texVenus(base rgbF, x, y, z, _, _, _ float32) rgbF {
	wx, wy, wz := warp3(x*2.0, y*6.0, z*2.0, 0.5, seedVenus)
	w := fbm3(wx, wy, wz, 4, seedVenus)
	return base.scale(0.92 + 0.16*w)
}

// texEarth: continents by threshold, then tinted by a second field
// standing in for aridity and modulated by latitude, so deserts sit in
// the subtropics and tundra near the poles. Ice caps are a hard cut in
// sin(latitude), which is where they read best at this size.
func texEarth(base rgbF, x, y, z, sinLat, lat, _ float32) rgbF {
	var (
		deepSea = rgbF{28, 62, 128}
		shallow = rgbF{74, 138, 206}
		forest  = rgbF{58, 112, 62}
		desert  = rgbF{176, 154, 96}
		tundra  = rgbF{150, 158, 150}
		ice     = rgbF{238, 244, 248}
	)
	wx, wy, wz := warp3(x*1.8, y*1.8, z*1.8, 0.35, seedEarth)
	m := fbm3(wx, wy, wz, 5, seedEarth)

	var c rgbF
	if m > 0.52 {
		// Aridity drives forest->desert; |lat| pulls the high
		// latitudes toward tundra whatever the aridity says.
		arid := fbm3(x*2.6+40, y*2.6, z*2.6, 4, seedEarth^0x3131)
		c = forest.mix(desert, clamp32(arid*1.5-0.25, 0, 1))
		if a := abs32(lat); a > 0.9 {
			c = c.mix(tundra, clamp32((a-0.9)*2.4, 0, 1))
		}
		// Shade the interior so the land is not a flat cutout.
		c = c.scale(0.90 + 0.20*fbm3(x*5, y*5, z*5, 3, seedEarth^0x9))
	} else {
		c = deepSea.mix(shallow, clamp32((m-0.30)*3.2, 0, 1))
	}
	if a := abs32(sinLat); a > 0.86 {
		c = c.mix(ice, clamp32((a-0.86)*7, 0, 1))
	}
	// Pull the whole thing toward the table's blue so the drawn planet
	// still matches its dot and its label.
	return c.mix(base, 0.30)
}

// texMars: rusty mottling with dark maria where the field runs low,
// and small bright polar caps.
func texMars(base rgbF, x, y, z, sinLat, _, _ float32) rgbF {
	w := fbm3(x*2.4, y*2.4, z*2.4, 5, seedMars)
	c := base.scale(0.86 + 0.30*w)
	if w < 0.45 {
		c = c.mix(rgbF{112, 62, 48}, clamp32((0.45-w)*3.4, 0, 1))
	}
	if a := abs32(sinLat); a > 0.93 {
		c = c.mix(rgbF{236, 238, 234}, clamp32((a-0.93)*11, 0, 1))
	}
	return c
}

// texJupiter: the bands are the planet. A sine in latitude sets the
// belts and zones; a field stretched 4:1 along longitude pushes their
// edges around so they are turbulent rather than ruled. The Great Red
// Spot sits where it belongs, in the South Equatorial Belt.
func texJupiter(base rgbF, x, y, z, _, lat, lon float32) rgbF {
	turb := fbm3(x*1.6, y*6.4, z*1.6, 3, seedJupiter) - 0.5
	b := float32(math.Sin(float64(lat*9 + 1.2*turb)))
	c := base.scale(1 + 0.17*b)
	// Belts (the dark half) get a brown cast, zones a cream one.
	if b < 0 {
		c = c.mix(rgbF{150, 108, 74}, -b*0.35)
	} else {
		c = c.mix(rgbF{232, 210, 176}, b*0.25)
	}
	if s := spot(lat, lon, -0.35, 2.2, 0.42, 0.13); s > 0 {
		c = c.mix(rgbF{198, 104, 72}, s*0.85)
	}
	return c
}

// texSaturn: the same construction as Jupiter with fewer bands, half
// the contrast and a brighter polar zone — which is most of what
// separates the two by eye.
func texSaturn(base rgbF, x, y, z, sinLat, lat, _ float32) rgbF {
	turb := fbm3(x*1.4, y*5.6, z*1.4, 3, seedSaturn) - 0.5
	b := float32(math.Sin(float64(lat*6 + 1.0*turb)))
	c := base.scale(1 + 0.09*b)
	c = c.mix(rgbF{236, 220, 176}, clamp32(abs32(sinLat)-0.55, 0, 1)*0.5)
	return c
}

// texUranus: nearly featureless is the correct look, so this is two
// faint bands and a mild polar brightening. Anything stronger would be
// a prettier planet and the wrong one.
func texUranus(base rgbF, x, y, z, sinLat, lat, _ float32) rgbF {
	turb := fbm3(x*1.2, y*4.0, z*1.2, 2, seedUranus) - 0.5
	b := float32(math.Sin(float64(lat*3 + 0.8*turb)))
	c := base.add(rgbF{6 * b, 6 * b, 5 * b})
	return c.scale(1 + 0.05*clamp32(abs32(sinLat)-0.5, 0, 1))
}

// texNeptune: three soft bands, white cirrus streaks pulled out of a
// stretched high-frequency field, and the Great Dark Spot.
func texNeptune(base rgbF, x, y, z, _, lat, lon float32) rgbF {
	turb := fbm3(x*1.5, y*5.0, z*1.5, 3, seedNeptune) - 0.5
	b := float32(math.Sin(float64(lat*4 + 1.0*turb)))
	c := base.add(rgbF{10 * b, 10 * b, 8 * b})
	if k := fbm3(x*2.2, y*9.0, z*2.2, 3, seedNeptune^0x5A); k > 0.70 {
		c = c.mix(rgbF{226, 236, 248}, clamp32((k-0.70)*2.6, 0, 1))
	}
	if s := spot(lat, lon, -0.38, 4.1, 0.40, 0.12); s > 0 {
		c = c.mix(rgbF{28, 48, 116}, s*0.75)
	}
	return c
}

// texFns is indexed like planets.
var texFns = [len(planets)]texFn{
	texMercury, texVenus, texEarth, texMars,
	texJupiter, texSaturn, texUranus, texNeptune,
}

// makeTextures builds every planet's surface.
func makeTextures() [len(planets)]*bodyTexture {
	var out [len(planets)]*bodyTexture
	for i := range planets {
		out[i] = makeTexture(rgbOf(planets[i].Color), texFns[i])
	}
	return out
}

// makeTexture rasterizes one surface function over the lat/lon grid
// and then normalizes its mean back onto the planet's table color.
//
// The normalization is what makes textures safe to add at all. Every
// other use of Planet.Color — the nav dot, the label, the flat tone
// below flatBodyRadius, the tooltip swatch — stays exactly as it was,
// and a planet too small to show any texture still reads as the same
// planet it did before.
func makeTexture(base rgbF, fn texFn) *bodyTexture {
	buf := make([]rgbF, texW*texH)

	for row := range texH {
		// The row's own band, sampled at its center: sin(lat) uniform,
		// matching what at() inverts.
		sinLat := 1 - 2*(float32(row)+0.5)/float32(texH)
		lat := float32(math.Asin(float64(clamp32(sinLat, -1, 1))))
		cosLat := sqrt32(max(0, 1-sinLat*sinLat))
		for col := range texW {
			lon := 2 * math.Pi * (float32(col) + 0.5) / float32(texW)
			x := cosLat * cos32(lon)
			y := sinLat
			z := cosLat * sin32(lon)
			buf[row*texW+col] = fn(base, x, y, z, sinLat, lat, lon)
		}
	}

	normalizeMean(buf, base)

	// Build down until a level is too small to interpolate over. The
	// whole pyramid costs a third again of the base level.
	t := &bodyTexture{lod: []texLevel{quantize(buf, texW, texH)}}
	for w, h := texW, texH; w/2 >= maxLodDim && h/2 >= maxLodDim; {
		buf, w, h = downsample(buf, w, h)
		t.lod = append(t.lod, quantize(buf, w, h))
	}
	return t
}

// normalizeMean shifts the field so its mean lands on want.
//
// The shift is additive, which preserves contrast where a multiply
// would stretch it, and it is iterated because clamping at the byte
// range moves the mean back a little — a texture with ice caps or a
// dark spot has texels sitting against a limit. Four passes is well
// past where this stops moving.
func normalizeMean(buf []rgbF, want rgbF) {
	for range 4 {
		var sr, sg, sb float32
		for _, c := range buf {
			sr += clamp32(c.r, 0, 255)
			sg += clamp32(c.g, 0, 255)
			sb += clamp32(c.b, 0, 255)
		}
		n := float32(len(buf))
		d := rgbF{want.r - sr/n, want.g - sg/n, want.b - sb/n}
		for i := range buf {
			buf[i] = buf[i].add(d)
		}
	}
	for i := range buf {
		buf[i] = rgbF{
			clamp32(buf[i].r, 0, 255),
			clamp32(buf[i].g, 0, 255),
			clamp32(buf[i].b, 0, 255),
		}
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
