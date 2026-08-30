package main

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// --- the sampling geometry ---

// testSurface is initSurface in the shape the tests want. The
// production call site keeps the value on its own stack to stay off
// the frame path's allocation budget; a test does not care.
func testSurface(i int, t float32) *surface {
	var s surface
	if !initSurface(&s, i, t) {
		return nil
	}
	return &s
}

// worldAxes returns planet i's spin axis and equator frame in *world*
// coordinates, built independently of newSurface so the camera
// projection there is checked against something rather than restated.
func worldAxes(tilt float32) (a, e1, e2 [3]float32) {
	sa, ca := sin32(tilt), cos32(tilt)
	return [3]float32{sa, 0, ca},
		[3]float32{ca, 0, -sa},
		[3]float32{0, 1, 0}
}

func dot3(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// camToWorld maps a camera-space vector back to world coordinates,
// inverting the basis lightVec documents. The basis is orthonormal, so
// the inverse is the transpose.
func camToWorld(x, y, z float32) [3]float32 {
	sinE, cosE := float32(diskTilt), cosElev
	return [3]float32{
		x,
		y*sinE + z*cosE,
		-y*cosE + z*sinE,
	}
}

// TestSurfaceAxesAreOrthonormal pins the frame newSurface builds: unit
// length, mutually perpendicular, and right-handed, for every tilt in
// the table. A left-handed frame would run longitude backwards on some
// planets and not others.
func TestSurfaceAxesAreOrthonormal(t *testing.T) {
	const eps = 1e-5
	for i := range planets {
		s := testSurface(i, 3.5)
		if s == nil {
			t.Fatalf("%s: no surface", planets[i].Name)
		}
		a := [3]float32{s.ax, s.ay, s.az}
		e1 := [3]float32{s.e1x, s.e1y, s.e1z}
		e2 := [3]float32{s.e2x, s.e2y, s.e2z}
		for _, v := range []struct {
			name string
			v    [3]float32
		}{{"a", a}, {"e1", e1}, {"e2", e2}} {
			if d := abs32(dot3(v.v, v.v) - 1); d > eps {
				t.Errorf("%s: %s is not unit (err %v)",
					planets[i].Name, v.name, d)
			}
		}
		for _, pr := range []struct {
			name string
			x, y [3]float32
		}{{"a.e1", a, e1}, {"a.e2", a, e2}, {"e1.e2", e1, e2}} {
			if d := abs32(dot3(pr.x, pr.y)); d > eps {
				t.Errorf("%s: %s = %v, want 0",
					planets[i].Name, pr.name, d)
			}
		}
		// e1 x e2 == a is what makes longitude increase the same way
		// for every planet.
		cross := [3]float32{
			e1[1]*e2[2] - e1[2]*e2[1],
			e1[2]*e2[0] - e1[0]*e2[2],
			e1[0]*e2[1] - e1[1]*e2[0],
		}
		for k := range 3 {
			if d := abs32(cross[k] - a[k]); d > eps {
				t.Errorf("%s: e1 x e2 component %d is %v, want %v",
					planets[i].Name, k, cross[k], a[k])
			}
		}
	}
}

// TestInitSurfaceSkipsWithoutRotation pins the RotS == 0 guard: with
// no rotation there is no texture and no body frame, and the spin term
// t/RotS would divide by zero. Every table planet has a rotation, so
// the branch is only reachable here, by construction. None of the
// tests that read RotS run in parallel, so mutating the table with
// restore is safe.
func TestInitSurfaceSkipsWithoutRotation(t *testing.T) {
	orig := planets[0].RotS
	planets[0].RotS = 0
	defer func() { planets[0].RotS = orig }()
	var s surface
	if initSurface(&s, 0, 5) {
		t.Fatal("initSurface reported a surface for a body with RotS 0")
	}
}

// TestRingLinearizationMatchesWorldNormal is the headline check on the
// whole texturing scheme. appendRing never forms a normal vector: it
// evaluates three linear functions of cos(t) and sin(t) whose
// coefficients were folded per ring. This recomputes the same three
// quantities the long way — build the camera-space normal, rotate it
// into world coordinates, dot it against the world axes — and requires
// they agree.
func TestRingLinearizationMatchesWorldNormal(t *testing.T) {
	const eps = 1e-5
	for i := range planets {
		s := testSurface(i, 2.25)
		wa, we1, we2 := worldAxes(planets[i].Tilt)

		for _, l := range [][3]float32{
			{0.6, 0.2, -0.77}, {-0.3, 0.9, 0.31}, {0.1, 0.05, 0.99},
		} {
			b := newLightBasis(l[0], l[1], l[2])
			proj := s.project(b)

			for pi := range 9 {
				phi := float32(pi) * math.Pi / 8
				cosPhi, sinPhi := cos32(phi), sin32(phi)
				rt := proj.ringTexFor(s, cosPhi, sinPhi, 1, 0, 0)

				for ti := range 12 {
					tt := float32(ti) * 2 * math.Pi / 12
					ct, st := cos32(tt), sin32(tt)

					// The long way: the camera-space normal, then the
					// world dot products.
					nx := cosPhi*b.lx + sinPhi*(ct*b.ux+st*b.vx)
					ny := cosPhi*b.ly + sinPhi*(ct*b.uy+st*b.vy)
					nz := cosPhi*b.lz + sinPhi*ct*b.uz
					n := camToWorld(nx, ny, nz)

					got := [3]float32{
						rt.a0 + ct*rt.a1 + st*rt.a2,
						rt.b0 + ct*rt.b1 + st*rt.b2,
						rt.c0 + ct*rt.c1 + st*rt.c2,
					}
					want := [3]float32{
						dot3(n, wa), dot3(n, we1), dot3(n, we2),
					}
					for k := range 3 {
						if d := abs32(got[k] - want[k]); d > eps {
							t.Fatalf("%s l=%v phi=%v t=%v axis %d: "+
								"linearized %v, direct %v",
								planets[i].Name, l, phi, tt, k,
								got[k], want[k])
						}
					}
				}
			}
		}
	}
}

// TestAtan2TurnsMatchesLibrary bounds the polynomial stand-in. The
// bound that matters is texels, not radians: one texel of a 128-wide
// texture spans 1/128 of a turn, and the approximation's worst case is
// under a quarter of that, so no sample can land in the wrong texel by
// more than a fraction of one.
func TestAtan2TurnsMatchesLibrary(t *testing.T) {
	const tol = 0.25 / texW
	for i := range 720 {
		ang := float64(i)*2*math.Pi/720 - math.Pi
		x, y := float32(math.Cos(ang)), float32(math.Sin(ang))
		want := float32(ang / (2 * math.Pi))
		got := atan2Turns(y, x)
		d := abs32(got - want)
		if d > 0.5 {
			d = abs32(d - 1) // the +/-0.5 wrap point
		}
		if d > tol {
			t.Fatalf("atan2Turns(%v,%v) = %v, want %v (err %v turns)",
				y, x, got, want, d)
		}
	}
}

// TestAtan2TurnsAtOrigin pins the zero guard: atan2 of (0,0) is
// undefined, so the polynomial must return *some* defined value rather
// than NaN or Inf — the 1e-10 term exists for exactly this call.
func TestAtan2TurnsAtOrigin(t *testing.T) {
	got := atan2Turns(0, 0)
	if got != got {
		t.Fatalf("atan2Turns(0,0) = NaN")
	}
	if got < -0.5 || got > 0.5 {
		t.Fatalf("atan2Turns(0,0) = %v, outside the turn range", got)
	}
}

// TestRingRampMatchesSphereTone pins the folded ramp against the
// function it replaces, across the whole intensity range. The
// end-to-end uniform-texture test only reaches the night branch: its
// light lies mostly along the view axis, so no ring's intensity
// exceeds baseStop. This one walks both branches directly, with the
// same at-most-one-count allowance the end-to-end test uses.
func TestRingRampMatchesSphereTone(t *testing.T) {
	base := gui.RGB(180, 140, 90)
	night := scaleColor(base, nightScale)
	lit := lightenColor(base, bodyLitLift)
	for i := 0; i <= 200; i++ {
		it := float32(i) / 200
		k, w := ringRamp(it)
		got := gui.RGBA(
			chan8(float32(base.R)*k+w),
			chan8(float32(base.G)*k+w),
			chan8(float32(base.B)*k+w),
			base.A)
		want := sphereTone(night, base, lit, it)
		for _, ch := range []struct {
			name string
			x, y uint8
		}{{"R", got.R, want.R}, {"G", got.G, want.G},
			{"B", got.B, want.B}} {
			d := int(ch.x) - int(ch.y)
			if d < 0 {
				d = -d
			}
			if d > 1 {
				t.Fatalf("intensity %v channel %s: ramp %d, sphereTone %d",
					it, ch.name, ch.x, ch.y)
			}
		}
	}
}

// TestTextureAtClampsAndWraps walks the index math of at() directly:
// the row mapping at the poles, the turn wrap for negative and
// overshooting values, and the NaN contract (a defined in-bounds texel
// rather than an out-of-range index).
//
// at() is bilinear, so the exact-equality checks below all sample at
// texel *centers*, where the blend weights are 0 or 1 and the result is
// one stored texel. Centers sit at (n+0.5)/w in longitude, hence the
// odd-looking turn constants. TestTextureAtIsContinuousInTurn covers
// the between-centers behavior.
func TestTextureAtClampsAndWraps(t *testing.T) {
	tex := &texLevel{w: 128, h: 64}
	tex.texel = make([]gui.Color, 128*64)
	for i := range tex.texel {
		tex.texel[i] = gui.RGB(uint8(i%128), uint8(i/128), 0)
	}
	at := func(sinLat, turn float32) gui.Color { return tex.at(sinLat, turn) }
	// Column 0's center. Rows clamp rather than wrap, so a pole
	// samples its own row on both taps.
	const col0 = 0.5 / 128
	if got := at(1, col0); got != tex.texel[0] {
		t.Errorf("north pole sampled %v, want row 0", got)
	}
	if got := at(-1, col0); got != tex.texel[63*128] {
		t.Errorf("south pole sampled %v, want row 63", got)
	}
	if got := at(1.5, col0); got != tex.texel[0] {
		t.Errorf("overshoot at +1.5 sampled %v, want row 0", got)
	}
	if got := at(-1.5, col0); got != tex.texel[63*128] {
		t.Errorf("overshoot at -1.5 sampled %v, want row 63", got)
	}
	// Longitude: column 32's center, reached from below zero and from
	// past one, must land on the same stored texel.
	const col32 = 32.5 / 128
	mid := 32 * 128 // row 32, the equator
	// sinLat for row 32's center, so the row blend is a no-op too.
	const midLat = 1 - 2*(32+0.5)/64.0
	if got := at(midLat, col32); got != tex.texel[mid+32] {
		t.Errorf("turn %v sampled %v, want column 32", float32(col32), got)
	}
	if got := at(midLat, col32-1); got != tex.texel[mid+32] {
		t.Errorf("turn %v sampled %v, want column 32", float32(col32-1), got)
	}
	if got := at(midLat, col32+1); got != tex.texel[mid+32] {
		t.Errorf("turn %v sampled %v, want column 32", float32(col32+1), got)
	}
	// A NaN turn must not panic either, and that one is sharper than
	// it looks: int(NaN) is implementation-defined and on amd64 it is
	// math.MinInt64, which no clamp on the column recovers from before
	// the index. It has to be caught on the wrapped fraction.
	if got, want := at(0, float32(math.NaN())), at(0, 0); got != want {
		t.Errorf("NaN turn sampled %v, want the turn-0 sample %v", got, want)
	}
	// NaN must not panic or produce an out-of-range index.
	got := at(float32(math.NaN()), 0)
	if int(got.R) >= tex.w || int(got.G) >= tex.h {
		t.Errorf("NaN sinLat sampled (%d,%d), an out-of-range texel",
			got.R, got.G)
	}
}

// TestTextureAtIsContinuousInTurn is the anti-jitter property.
//
// A spinning body's mesh vertices sit still in the light basis while
// the surface turns under them, so a vertex's color is a function of
// spin alone. Nearest sampling made that function a staircase with
// 1/texW-turn treads, which is what the surface crawling looked like.
// Sweeping a full turn in steps far finer than a texel, no single step
// may move a channel by more than a texel's own worth of difference —
// which on this ramp is one count per step at most, plus one for
// rounding.
func TestTextureAtIsContinuousInTurn(t *testing.T) {
	tex := &texLevel{w: 128, h: 64}
	tex.texel = make([]gui.Color, 128*64)
	for i := range tex.texel {
		// A triangle wave along longitude. Adjacent columns differ by
		// 2 everywhere *including across the wrap*, so the ramp itself
		// contributes no discontinuity for the check to excuse.
		col := i % 128
		d := col - 64
		if d < 0 {
			d = -d
		}
		tex.texel[i] = gui.RGB(uint8(d*2), 0, 0)
	}
	const steps = 128 * 8
	prev := tex.at(0, 0)
	for k := 1; k <= steps; k++ {
		turn := float32(k) / steps
		got := tex.at(0, turn)
		d := int(got.R) - int(prev.R)
		if d < 0 {
			d = -d
		}
		if d > 2 {
			t.Fatalf("turn %v: R jumped %d counts (%d -> %d)",
				turn, d, prev.R, got.R)
		}
		prev = got
	}
}

// TestUVCoordsStayInRange walks every planet over a spread of times and
// light directions and requires the sampled row and column to land
// inside the texture. at() clamps and wraps, so a failure here would be
// silent smearing rather than a panic.
func TestUVCoordsStayInRange(t *testing.T) {
	for i := range planets {
		for step := range 90 {
			s := testSurface(i, float32(step)*0.7)
			b := newLightBasis(0.6, 0.2, -0.77)
			proj := s.project(b)
			for pi := range 12 {
				phi := float32(pi) * math.Pi / 11
				cosPhi, sinPhi := cos32(phi), sin32(phi)
				rt := proj.ringTexFor(s, cosPhi, sinPhi, 1, 0, 0)
				for ti := range 16 {
					tt := float32(ti) * 2 * math.Pi / 16
					ct, st := cos32(tt), sin32(tt)
					sinLat := rt.a0 + ct*rt.a1 + st*rt.a2
					if sinLat < -1.001 || sinLat > 1.001 {
						t.Fatalf("%s: sin(lat) = %v, outside [-1,1]",
							planets[i].Name, sinLat)
					}
					p1 := rt.b0 + ct*rt.b1 + st*rt.b2
					p2 := rt.c0 + ct*rt.c1 + st*rt.c2
					if turn := atan2Turns(p2, p1); turn < -0.501 || turn > 0.501 {
						t.Fatalf("%s: turn = %v, outside [-0.5,0.5]",
							planets[i].Name, turn)
					}
				}
			}
		}
	}
}

// --- spin ---

// subViewerTurn is the texture longitude, in turns, currently facing
// the given camera-space direction.
func subViewerTurn(s *surface, x, y, z float32) float32 {
	p1 := x*s.e1x + y*s.e1y + z*s.e1z
	p2 := x*s.e2x + y*s.e2y + z*s.e2z
	return atan2Turns(p2, p1) - s.spin
}

// TestSpinAdvancesLongitude requires the face presented to the camera
// to walk steadily around the body, and to walk the *other* way for
// Venus — which is the whole content of its negative RotS.
//
// The expected sign is negative for a prograde planet, which is right
// rather than a fudge. e1 and e2 are inertial, so a body-fixed feature
// at body longitude L appears at inertial longitude L + spin; the
// texture is indexed by body longitude, so sampling subtracts spin.
// As a prograde planet turns, the feature facing the camera is one
// with a progressively *smaller* body longitude.
func TestSpinAdvancesLongitude(t *testing.T) {
	for i := range planets {
		var prev float32
		var total float32
		for step := range 24 {
			s := testSurface(i, float32(step)*abs32(planets[i].RotS)/24)
			turn := subViewerTurn(s, 0, 0, 1)
			if step > 0 {
				d := turn - prev
				// Unwrap across the +/-0.5 seam.
				for d > 0.5 {
					d--
				}
				for d < -0.5 {
					d++
				}
				total += d
			}
			prev = turn
		}
		// A full rotation was sampled, so the unwrapped travel should
		// be close to one turn, negative for a prograde planet.
		want := float32(-1)
		if planets[i].RotS < 0 {
			want = 1
		}
		// 23/24 of the way round: the loop takes 24 samples spanning
		// one period, so it accumulates one step short of a full turn.
		if d := abs32(total - want*23.0/24.0); d > 0.02 {
			t.Errorf("%s: longitude travelled %v turns, want about %v",
				planets[i].Name, total, want*23.0/24.0)
		}
	}
}

// TestRotationTableIsSane guards the compressed periods the same way
// the orbit table is guarded: ordering and bounds, not exact values.
func TestRotationTableIsSane(t *testing.T) {
	for i := range planets {
		p := planets[i]
		if r := abs32(p.RotS); r < 2 || r > 60 {
			t.Errorf("%s: |RotS| = %v, outside the compressed range",
				p.Name, r)
		}
		if p.Tilt < 0 || p.Tilt > math.Pi {
			t.Errorf("%s: Tilt = %v, outside [0,pi]", p.Name, p.Tilt)
		}
	}
	// Venus is the only retrograde spinner in the table, and Uranus
	// the only one tipped past a right angle.
	for i := range planets {
		retro := planets[i].RotS < 0
		if want := planets[i].Name == "Venus"; retro != want {
			t.Errorf("%s: retrograde = %v, want %v",
				planets[i].Name, retro, want)
		}
		if tipped := planets[i].Tilt > math.Pi/2; tipped !=
			(planets[i].Name == "Uranus") {
			t.Errorf("%s: tilt past 90 deg = %v", planets[i].Name, tipped)
		}
	}
}

// --- the textures themselves ---

// TestTextureMeanMatchesPlanetColor pins the normalization that makes
// texturing safe to add: the nav dot, the label, the tooltip and the
// flat tone a tiny body draws all still use Planet.Color, so a
// textured planet has to average out to it.
func TestTextureMeanMatchesPlanetColor(t *testing.T) {
	for i := range planets {
		tex := &planetTextures[i].lod[0]
		var sr, sg, sb float64
		for _, c := range tex.texel {
			sr += float64(c.R)
			sg += float64(c.G)
			sb += float64(c.B)
		}
		n := float64(len(tex.texel))
		want := planets[i].Color
		for _, ch := range []struct {
			name string
			got  float64
			want uint8
		}{
			{"R", sr / n, want.R}, {"G", sg / n, want.G},
			{"B", sb / n, want.B},
		} {
			if d := math.Abs(ch.got - float64(ch.want)); d > 2 {
				t.Errorf("%s: mean %s = %.2f, want %d",
					planets[i].Name, ch.name, ch.got, ch.want)
			}
		}
	}
}

// TestTextureIsSeamlessInLongitude guards the choice to build noise on
// the 3D direction rather than on the (u,v) grid. The wrap column is
// adjacent to column 0 on the sphere, so the step across it must be no
// worse than the largest step anywhere else in the same texture; a
// regression to 2D noise would put a visible line down every planet.
func TestTextureIsSeamlessInLongitude(t *testing.T) {
	for i := range planets {
		tex := &planetTextures[i].lod[0]
		delta := func(a, b gui.Color) int {
			d := func(x, y uint8) int {
				if x > y {
					return int(x) - int(y)
				}
				return int(y) - int(x)
			}
			return d(a.R, b.R) + d(a.G, b.G) + d(a.B, b.B)
		}
		var seam, interior int
		for row := range tex.h {
			r := tex.texel[row*tex.w : (row+1)*tex.w]
			if v := delta(r[0], r[tex.w-1]); v > seam {
				seam = v
			}
			for col := range tex.w - 1 {
				if v := delta(r[col], r[col+1]); v > interior {
					interior = v
				}
			}
		}
		if seam > interior {
			t.Errorf("%s: worst seam step %d exceeds worst interior "+
				"step %d — the wrap is visible", planets[i].Name,
				seam, interior)
		}
	}
}

// TestTexturedBodyMatchesFlatWhenTextureIsUniform pins the folded ramp
// against sphereTone, which is the function it replaces on the
// textured path. Feeding a constant albedo must reproduce the flat
// shading to within the rounding the fold gives up: sphereTone
// quantizes its night and lit tones to bytes before mixing and
// ringRamp does not, which is worth at most one count.
func TestTexturedBodyMatchesFlatWhenTextureIsUniform(t *testing.T) {
	base := gui.RGB(180, 140, 90)
	flatTex := &bodyTexture{lod: []texLevel{{w: 4, h: 2, texel: make([]gui.Color, 8)}}}
	for i := range flatTex.lod[0].texel {
		flatTex.lod[0].texel[i] = base
	}

	s := testSurface(2, 1.0) // Earth, for a non-trivial tilt
	s.tex = flatTex

	dcFlat := gui.NewDrawContext(400, 400, nil)
	dcTex := gui.NewDrawContext(400, 400, nil)
	var mf, mt bodyMesh
	drawBody(dcFlat, &mf, 200, 200, 90, base, 0.6, 0.2, -0.77, nil)
	drawBody(dcTex, &mt, 200, 200, 90, base, 0.6, 0.2, -0.77, s)

	if len(mf.cols) != len(mt.cols) {
		t.Fatalf("vertex counts differ: flat %d, textured %d",
			len(mf.cols), len(mt.cols))
	}
	for i := range mf.cols {
		a, b := mf.cols[i], mt.cols[i]
		for _, ch := range []struct {
			name string
			x, y uint8
		}{{"R", a.R, b.R}, {"G", a.G, b.G}, {"B", a.B, b.B},
			{"A", a.A, b.A}} {
			d := int(ch.x) - int(ch.y)
			if d < 0 {
				d = -d
			}
			if d > 1 {
				t.Fatalf("vertex %d channel %s: flat %d, textured %d",
					i, ch.name, ch.x, ch.y)
			}
		}
	}
}

// TestTexturedBodyDoesNotAllocatePerFrame is the end-to-end form of
// the gate in main_test.go: sampling a surface must not allocate
// either, or nine textured planets a tick would allocate for the
// session.
func TestTexturedBodyDoesNotAllocatePerFrame(t *testing.T) {
	var m bodyMesh
	dc := gui.NewDrawContext(400, 400, nil)
	s := testSurface(4, 2.0) // Jupiter
	drawBody(dc, &m, 200, 200, 120, planets[4].Color, 0.6, 0.2, -0.77, s)

	got := testing.AllocsPerRun(20, func() {
		m.tris, m.cols = m.tris[:0], m.cols[:0]
		b := newLightBasis(0.6, 0.2, -0.77)
		proj := s.project(b)
		k, w := ringRamp(0.5)
		rt := proj.ringTexFor(s, 0.3, 0.95, k, w, 0)
		m.cur, m.curCol = appendRing(m.cur[:0], m.curCol[:0], b,
			120, 200, 200, 0.3, 0.95, 64, gui.RGB(1, 2, 3), rt)
		rt = proj.ringTexFor(s, 0.2, 0.98, k, w, 0)
		m.prev, m.prevCol = appendRing(m.prev[:0], m.prevCol[:0], b,
			120, 200, 200, 0.2, 0.98, 64, gui.RGB(4, 5, 6), rt)
		m.appendStrip(64)
	})
	if got != 0 {
		t.Errorf("textured mesh rebuild allocated %v times per run, want 0", got)
	}
}

// TestTexturedBodyStaysNearPlanetColor is the end-to-end form of the
// mean check: a mean can be right across the texture while individual
// texels are wild, and what actually has to hold is that putting a
// texture on a body does not change its overall tone.
//
// The comparison is against the *untextured* draw of the same body,
// not against Planet.Color directly. A drawn body is much darker than
// its table color and should be — the mesh spans the whole visible
// hemisphere, most of whose vertices sit well down the Lambert ramp
// toward the limb — so the table color is not the number a drawn mean
// is supposed to match.
func TestTexturedBodyStaysNearPlanetColor(t *testing.T) {
	drawnMean := func(i int, s *surface) (float64, float64, float64, int) {
		var m bodyMesh
		dc := gui.NewDrawContext(400, 400, nil)
		drawBody(dc, &m, 200, 200, 100, planets[i].Color,
			0.6, 0.2, -0.77, s)
		var sr, sg, sb float64
		for _, c := range m.cols {
			sr += float64(c.R)
			sg += float64(c.G)
			sb += float64(c.B)
		}
		n := float64(len(m.cols))
		return sr / n, sg / n, sb / n, len(m.cols)
	}

	for i := range planets {
		fr, fg, fb, nf := drawnMean(i, nil)
		tr, tg, tb, nt := drawnMean(i, testSurface(i, 1.0))
		if nf != nt {
			t.Fatalf("%s: textured mesh has %d vertices, untextured %d",
				planets[i].Name, nt, nf)
		}
		for _, ch := range []struct {
			name     string
			flat, ha float64
		}{{"R", fr, tr}, {"G", fg, tg}, {"B", fb, tb}} {
			if d := math.Abs(ch.flat - ch.ha); d > 8 {
				t.Errorf("%s: textured mean %s = %.1f, untextured %.1f",
					planets[i].Name, ch.name, ch.ha, ch.flat)
			}
		}
	}
}

// TestLodPyramidShapeAndMean pins the two properties the pyramid has
// to hold: every level is half the one above it down to the stopping
// size, and the box filter preserves the mean normalizeMean
// established. A coarse level whose mean had drifted would make a
// small planet a different color from its own nav dot.
func TestLodPyramidShapeAndMean(t *testing.T) {
	for i := range planets {
		tex := planetTextures[i]
		if len(tex.lod) < 2 {
			t.Fatalf("%s: pyramid has %d level(s), want at least 2",
				planets[i].Name, len(tex.lod))
		}
		mean := func(l *texLevel) (float64, float64, float64) {
			var r, g, b float64
			for _, c := range l.texel {
				r, g, b = r+float64(c.R), g+float64(c.G), b+float64(c.B)
			}
			n := float64(len(l.texel))
			return r / n, g / n, b / n
		}
		wr, wg, wb := mean(&tex.lod[0])
		for k := 1; k < len(tex.lod); k++ {
			up, l := &tex.lod[k-1], &tex.lod[k]
			if l.w != up.w/2 || l.h != up.h/2 {
				t.Fatalf("%s level %d: %dx%d, want %dx%d",
					planets[i].Name, k, l.w, l.h, up.w/2, up.h/2)
			}
			if len(l.texel) != l.w*l.h {
				t.Fatalf("%s level %d: %d texels for %dx%d",
					planets[i].Name, k, len(l.texel), l.w, l.h)
			}
			// Every level is filtered from the unquantized source and
			// rounded once, so the only drift is that one rounding.
			gr, gg, gb := mean(l)
			for _, ch := range []struct {
				name      string
				got, want float64
			}{{"R", gr, wr}, {"G", gg, wg}, {"B", gb, wb}} {
				if d := ch.got - ch.want; d > 0.5 || d < -0.5 {
					t.Errorf("%s level %d channel %s: mean %.2f, want %.2f",
						planets[i].Name, k, ch.name, ch.got, ch.want)
				}
			}
		}
	}
}

// TestLodSuppressesDetailTheMeshCannotCarry is the anti-moire property.
//
// Sampling a surface at the mesh's own vertex spacing must not swing
// more, step to step, than sampling it at a spacing fine enough to
// resolve it. Point sampling fails this badly: it reads whatever texel
// it lands on, so a coarse walk across a threshold-edged surface like
// Mercury's jumps by the full contrast of the feature it skipped over,
// and those jumps move as the body turns. Choosing a level matched to
// the spacing removes the octaves the walk cannot represent, so the
// coarse walk becomes a smooth version of the fine one.
func TestLodSuppressesDetailTheMeshCannotCarry(t *testing.T) {
	tex := planetTextures[0] // Mercury, the worst case
	// A vertex spacing of four texels, which is what a small body's
	// tessellation actually gives on the lit side.
	const stepTexels = 4
	lod := tex.lodFor(stepTexels * 2 * math.Pi / texW)
	if lod < 1 {
		t.Fatalf("lodFor(%d texels) = %v, want at least 1", stepTexels, lod)
	}

	swing := func(sample func(turn float32) gui.Color) int {
		var worst int
		prev := sample(0)
		for k := 1; k <= texW/stepTexels; k++ {
			turn := float32(k) * stepTexels / texW
			got := sample(turn)
			for _, d := range []int{
				int(got.R) - int(prev.R),
				int(got.G) - int(prev.G),
				int(got.B) - int(prev.B),
			} {
				if d < 0 {
					d = -d
				}
				worst = max(worst, d)
			}
			prev = got
		}
		return worst
	}

	// Along the equator, where a texel is widest and the walk skips
	// the most.
	raw := swing(func(turn float32) gui.Color { return tex.at(0, turn) })
	filtered := swing(func(turn float32) gui.Color {
		return tex.atLod(lod, 0, turn)
	})
	if filtered >= raw {
		t.Errorf("worst step: filtered %d, unfiltered %d — the level "+
			"choice removed nothing", filtered, raw)
	}
}

// TestRingPlaneIsPerpendicularToSpinAxis pins the fact that makes
// Saturn's rings right: they lie in its equatorial plane, so both
// in-plane basis vectors are perpendicular to the spin axis.
//
// The drawn pole line and the rings come from the same tilt, and an
// axis-aligned ellipse — which is what this replaced — left them
// disagreeing by about 30 degrees.
func TestRingPlaneIsPerpendicularToSpinAxis(t *testing.T) {
	for i := range planets {
		p := &planets[i]
		ax, ay, az := axisDir(p)
		e1x, e1y, e2x, e2y := ringBasis(p)
		// ringBasis returns the screen halves; e1 lies flat in the
		// screen by construction and e2's depth is the axis's own
		// screen-plane length.
		m := sqrt32(ax*ax + ay*ay)
		for _, e := range []struct {
			name    string
			x, y, z float32
		}{
			{"e1", e1x, e1y, 0},
			{"e2", e2x, e2y, m},
		} {
			if d := ax*e.x + ay*e.y + az*e.z; abs32(d) > 1e-5 {
				t.Errorf("%s: %s . axis = %v, want 0",
					p.Name, e.name, d)
			}
		}
		// Both are also the ellipse's semi-axes, so their lengths are
		// what the projection says: e1 is a unit vector and e2's screen
		// length is |az|.
		if l := sqrt32(e1x*e1x + e1y*e1y); abs32(l-1) > 1e-5 {
			t.Errorf("%s: |e1| = %v, want 1", p.Name, l)
		}
		if l := sqrt32(e2x*e2x + e2y*e2y); abs32(l-abs32(az)) > 1e-5 {
			t.Errorf("%s: |e2| = %v, want |az| = %v", p.Name, l, abs32(az))
		}
	}
}

// TestRingHalvesSplitByDepth walks the points drawRings emits and
// requires each half to be the one it claims.
//
// The drawing order is far rings, body, near rings, and that is the
// whole of what makes the rings pass behind Saturn. If a half carried
// a point from the other side, that point would be drawn on the wrong
// side of the sphere.
func TestRingHalvesSplitByDepth(t *testing.T) {
	p := &planets[saturnIndex]
	ax, ay, _ := axisDir(p)
	m := sqrt32(ax*ax + ay*ay)
	e1x, e1y, e2x, e2y := ringBasis(p)
	// The two screen basis vectors are not orthogonal, so recovering a
	// point's parameters means inverting the 2x2 they form.
	det := e1x*e2y - e1y*e2x
	if abs32(det) < 1e-6 {
		t.Fatalf("ring basis is degenerate: det %v", det)
	}

	const cx, cy, r = 200, 200, 40
	for _, near := range []bool{true, false} {
		var a App
		dc := gui.NewDrawContext(400, 400, nil)
		drawRings(&a, dc, p, cx, cy, r, near, 1)
		if len(a.ringPts) < 4 {
			t.Fatalf("near=%v: drawRings emitted %d coords",
				near, len(a.ringPts))
		}
		// a.ringPts holds the last (outermost) ring, radius r*1.96.
		const outer = r * 1.96
		for k := 0; k+1 < len(a.ringPts); k += 2 {
			dx := (a.ringPts[k] - cx) / outer
			dy := (a.ringPts[k+1] - cy) / outer
			// Solve dx,dy = c*e1 + sn*e2 for c and sn.
			c := (dx*e2y - dy*e2x) / det
			sn := (e1x*dy - e1y*dx) / det
			// The point is on the ring: cos^2 + sin^2 == 1.
			if q := c*c + sn*sn; abs32(q-1) > 1e-3 {
				t.Fatalf("near=%v point %d: cos^2+sin^2 = %v, want 1",
					near, k/2, q)
			}
			// Depth is r*sin(theta)*m, and the near half is the
			// positive one. The two endpoints sit at sin == 0, where
			// the ring crosses the plane of the limb.
			depth := sn * m
			if near && depth < -1e-4 {
				t.Errorf("near half point %d has depth %v", k/2, depth)
			}
			if !near && depth > 1e-4 {
				t.Errorf("far half point %d has depth %v", k/2, depth)
			}
		}
	}
}

// TestLog2fMatchesLibrary checks the bit-trick log2 against the real
// one over the range lodFor actually feeds it. The claim in log2f's
// comment is a worst case of about 0.086 of a level, which is small
// enough to move a blend weight and never the choice of levels; a
// regression past that would start snapping between levels.
func TestLog2fMatchesLibrary(t *testing.T) {
	var worst float64
	// A texel spacing from a thousandth of a texel to a thousand, which
	// covers everything between a zoomed Jupiter and a one-pixel Mercury.
	for v := 0.001; v <= 1000; v *= 1.01 {
		got := float64(log2f(float32(v)))
		want := math.Log2(v)
		if d := math.Abs(got - want); d > worst {
			worst = d
		}
	}
	if worst > 0.09 {
		t.Errorf("log2f worst error %.4f levels, want at most 0.09", worst)
	}
	// Non-positive input has no logarithm; lodFor clamps to level 0
	// anyway, so the contract is that it returns 0 rather than -Inf.
	for _, v := range []float32{0, -1, float32(math.Inf(-1))} {
		if got := log2f(v); got != 0 {
			t.Errorf("log2f(%v) = %v, want 0", v, got)
		}
	}
}

// TestLodForClampsToThePyramid pins the two ends of the level choice.
// A spacing the finest level already resolves must ask for full detail,
// and a spacing coarser than the whole pyramid must stop at the last
// level rather than index past it.
func TestLodForClampsToThePyramid(t *testing.T) {
	tex := planetTextures[0]
	oneTexel := float32(2 * math.Pi / texW)
	for _, c := range []struct {
		name string
		ds   float32
		want float32
	}{
		{"far below a texel", oneTexel / 1000, 0},
		{"exactly a texel", oneTexel, 0},
		{"the whole sphere", math.Pi, tex.maxLod()},
		{"non-positive", 0, 0},
	} {
		if got := tex.lodFor(c.ds); got != c.want {
			t.Errorf("lodFor(%s) = %v, want %v", c.name, got, c.want)
		}
	}
	// And every level the clamp can return must be samplable.
	for _, lod := range []float32{0, 0.5, 1, tex.maxLod(), tex.maxLod() + 5} {
		tex.atLod(lod, 0, 0.25)
	}
}
