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
type bodyTexture struct {
	w, h  int
	texel []gui.Color
}

// at samples the surface at a latitude given as its sine and a
// longitude given in turns. turn is wrapped, so a caller may add spin
// to it without normalizing; sinLat is clamped, which only matters for
// float error at the poles.
func (t *bodyTexture) at(sinLat, turn float32) gui.Color {
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
	row := int((1 - sinLat) * 0.5 * float32(t.h))
	if row < 0 {
		row = 0
	} else if row >= t.h {
		row = t.h - 1
	}
	// Wrap into [0,1) without math.Floor: the truncation toward zero
	// is corrected by the negative branch.
	f := turn - float32(int(turn))
	if f < 0 {
		f++
	}
	col := int(f * float32(t.w))
	if col < 0 {
		col = 0
	} else if col >= t.w {
		col = t.w - 1
	}
	return t.texel[row*t.w+col]
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
	t := &bodyTexture{w: texW, h: texH}
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

	t.texel = make([]gui.Color, len(buf))
	for i, c := range buf {
		t.texel[i] = c.color()
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
