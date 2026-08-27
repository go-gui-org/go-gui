# Orrery enhancements for `examples/solar_system`

## Context

The example is an interactive orrery: eight tilted elliptical orbits, a
procedurally textured sun, per-planet noise textures, a screen-fixed starfield.
A reference antique-orrery rendering supplies four visual elements it lacks:

1. A **calendar ring** — month names and day ticks around the outside, with a
   marker at the current date.
2. An **asteroid belt** between Mars and Jupiter.
3. A **Milky Way** band behind the whole scene.
4. **Rotation axes** on the planets — a line through each pole.

Art beats accuracy. The calendar ring lies in the orbital plane, so its month
text is squashed and rotated with the plane rather than sitting upright on
screen.

Decisions taken with the user:

- The date marker tracks Earth's orbital angle, the way a real orrery reads the
  date off Earth's position.
- The Milky Way is procedural and generated once, not an image asset.
- The galaxy is a fixed backdrop (screen-space, like the starfield); the ring
  and the belt live in world space and pan/zoom with the camera.
- Month labels use a **stroke font built in the example**. `dc.Text` honors
  `TextStyle.RotationRadians` (`gui/render_draw_canvas.go:124`) but not skew or
  squash, so real glyphs cannot lie in the plane. A stroke font's points go
  through the same 2x2 transform as the ticks, and all twelve labels merge into
  one mesh batch.

No change to `gui/` or any backend. Everything lands in the example.

## Existing machinery to reuse

| Need                           | Reuse                                                                  |
| ------------------------------ | ---------------------------------------------------------------------- |
| World→screen projection        | `worldToScreen`, `diskTilt`, `cosElev` (`camera.go:52,12,141`)         |
| Elliptical position            | `orbitPos` (`camera.go:41`) — generalize, see below                    |
| Zero-alloc vertex-colored mesh | `bodyMesh` + `appendTri` (`draw.go:984,1010`), scratch fields on `App` |
| One-batch point cloud          | the pattern in `drawStars` (`draw.go:209`)                             |
| Soft blob geometry             | `granuleRim` unit circle (`draw.go:431`)                               |
| Init-time noise                | `fbm3`, `valueNoise3`, `hash3` (`texture.go:163,146,130`)              |
| Screen-space spin axis         | the `ax, ay, az` derivation in `initSurface` (`draw.go:692`)           |
| Seeded, reproducible init      | `rand.NewPCG` as in `makeStars`/`makeGranules`                         |

`FillTrianglesColors(tris, cols)` (`gui/canvas_draw_colors.go:28`) is the
one-batch escape hatch; `dc.Arc(cx, cy, rx, ry, start, sweep, c, w)` is the
ellipse primitive already used for orbits.

## Files

New:

- `examples/solar_system/galaxy.go` — Milky Way band
- `examples/solar_system/belt.go` — asteroid belt
- `examples/solar_system/dial.go` — calendar ring
- `examples/solar_system/strokefont.go` — the segment font table
- `examples/solar_system/dial_test.go`, `belt_test.go`, `galaxy_test.go`,
  `strokefont_test.go`

Modified: `main.go` (state + scratch), `camera.go` (shared ellipse helper,
framing), `draw.go` (paint order, axis lines), `planets.go` (`earthIndex`),
`README.md`, `/CHANGELOG.md`.

`draw.go` is already 1438 lines; new subsystems get their own files rather than
growing it.

---

## 1. Shared ellipse helper (`camera.go`)

The belt and the ring both need `orbitPos`'s math without a `*Planet`. Extract
it and have `orbitPos` call it — no behavior change:

```go
// ellipsePos is orbitPos with the orbit terms supplied directly, so
// the asteroid belt can share it without a Planet table entry.
func ellipsePos(a, ecc, phase, periodS, t float32) (x, y float32)
```

Also add the period law the planet table documents but never encodes, so belt
rocks obey the same compression as the planets:

```go
// orbitPeriod is the PeriodS a body at world radius a would have under
// the table's compression: realAU = (a/195)^(1/0.38), then
// realDays^0.45 * 0.8. Checks out against the table — Mercury 6.0,
// Earth 11.4, Jupiter 34.6.
func orbitPeriod(a float32) float32
```

Init-time only; never on the frame path.

## 2. Rotation axes (`draw.go`)

`initSurface` already resolves the spin axis into camera coordinates:
`(ax, ay, az) = (sin θ, -cos θ · cosElev, cos θ · diskTilt)`, with x/y in screen
pixels and z toward the viewer. Lift that to a standalone helper so the axis
does not depend on the texture path (`initSurface` returns false when a planet
has no texture):

```go
// axisDir returns a planet's spin axis as a unit vector in camera
// coordinates: x right, y down, z toward the viewer. Same derivation
// as initSurface, which is where the basis is documented.
func axisDir(p *Planet) (ax, ay, az float32)
```

In `drawPlanet`, draw the axis in two halves so the sphere occludes the far one:

- far half (the end whose `az < 0`) — before `drawBody`, dimmer
- near half (`az > 0`) — after `drawBody`, brighter

Saturn's order becomes: far axis, back rings, body, front rings, near axis.

Length `axisOverhang = 1.35` × screen radius. `dc.Line` at 1px, two calls per
planet, same color → they merge into the flat batch run. Gate on
`r >= axisMinRadius` (about 5px) so the full-system view is not a hedgehog. Two
new palette entries, `colorAxis` / `colorAxisFar`.

## 3. Asteroid belt (`belt.go`)

Built once with a fixed PCG seed, `beltCount ≈ 600`:

```go
type rock struct {
    orbitA, ecc, phase, periodS float32
    zOff                        float32 // out-of-plane offset, world units
    size                        float32
    tint                        float32 // brightness variance
}
```

- `orbitA` spread over `[beltInner, beltOuter] = [250, 340]` world units —
  between Mars (229) and Jupiter (365) — with a density hump in the middle
  rather than a flat spread, and a couple of thin gaps for the Kirkwood look.
- `periodS` from `orbitPeriod(orbitA)`, so a rock at 250 takes ~17.7s and one at
  340 takes ~30.6s: strictly between Mars's 15.1 and Jupiter's 34.6, and inner
  rocks visibly lap outer ones.
- `zOff` gives the belt vertical thickness. The camera basis has screen-down
  `(0, sinE, -cosE)`, so a world +z point contributes `-zOff · cosElev · zoom`
  to screen y. That is the one line of new projection, and it is consistent with
  `lightVecAt`.

`drawBelt(a, dc)` walks the rocks, culls anything off-canvas, and emits each as
a 2-triangle square into an `App.belt bodyMesh` scratch → **one**
`FillTrianglesColors` call. Same shape as `drawStars`, which is why the pattern
is worth copying rather than inventing.

Color: a desaturated cyan-grey, per-rock brightness from `tint`, matching the
reference's cyan dots without the full saturation.

Paint order: after `drawOrbits`, before `drawSun`. Depth sorting is unnecessary
— a rock's world radius is at least 250, so its projected position never lands
on the sun's 52-unit disc.

## 4. Milky Way (`galaxy.go`)

Screen-fixed, like the starfield, and baked once per canvas size rather than per
frame.

Two components:

**Cloud.** A band across the canvas at a fixed diagonal. `puffCount` soft blobs
placed along it: position `u` along the band, perpendicular offset `v` from a
gaussian, alpha from `fbm3` sampled at `(u, v)` times a perpendicular falloff,
minus a dark rift band to break the shape up. Each puff is a triangle fan over
the existing `granuleRim` unit circle — opaque-ish at the center,
`WithOpacity(0)` at the rim — exactly the granulation cell's construction. All
puffs go into one vertex-colored mesh, so the whole cloud is **one batch**.

Puff placement is normalized `[0,1)` canvas coordinates, seeded once. The mesh
itself is pixel geometry, so it is rebuilt only when `dc.Width`/`dc.Height`
change — cached on `App` alongside the scratch, guarded by the stashed size
`drawSystem` already tracks. Zero per-frame cost beyond the one draw call.

**Star concentration.** A second, denser star population whose positions are
band-biased, appended to `App.Stars` by `makeStars`. It reuses `Star` and
`drawStars` unchanged, so it costs no new draw call. Note `drawStars` currently
bounds its loop by `min(len(a.Stars), starCount)`; that bound becomes
`len(a.Stars)` and `starCount` keeps naming the uniform field alone.

Paint order: `FilledRect` background, galaxy cloud, then `drawStars`.

## 5. Calendar ring (`dial.go`)

Lives in the orbital plane, so every point goes through `a.worldToScreen`. Radii
in world units: `dialInner = 780`, `dialOuter = 830`, just outside Neptune's
aphelion (~730).

**Framing.** `fullSystemTarget` currently sizes to Neptune's extent × 1.12; it
must size to `dialOuter` instead, with the margin trimmed to about 1.04 so the
system does not shrink. No test asserts the current value.

**Angle ↔ date.** Earth's parameter angle is the date, directly:

```go
const earthIndex = 2
// dialAngle is the heliocentric longitude the calendar reads as "now".
func dialAngle(t float32) float32 // planets[earthIndex].Phase + 2πt/PeriodS
```

`frac(angle / 2π) × 365` is the day of year; month boundaries are the cumulative
non-leap month lengths. Angle 0 (world +x) is Jan 1, and angle increases the way
Earth travels, so the marker and the labels cannot disagree.

**Parts**, painted in this order:

1. Three concentric rails — `dc.Arc` with `ry = rx · diskTilt`, the same squash
   `drawOrbits` applies.
2. **Day ticks** — 365 short radial segments between the rails. Built as quads
   into an `App.dial bodyMesh` scratch → one batch. Dropped below a screen-size
   gate, leaving only the month ticks when zoomed out.
3. **Month ticks** — 12 longer segments crossing both rails.
4. **Month labels** — stroke font, below.
5. **Date marker** — a small filled triangle at `dialAngle(a.Time)`, built
   in-plane and pointing inward, like the reference's pointer.

**Culling.** Skip the whole dial when its projected radius exceeds a few canvas
widths — a planet selection zooms to 30x, where the ring is nowhere near the
viewport and would still cost its full geometry.

### Stroke font (`strokefont.go`)

```go
// strokeGlyph is one character as polylines in a unit em box: x in
// [0,1] rightward, y in [0,1] upward.
type strokeGlyph [][]float32 // each entry is x0,y0,x1,y1,...

var strokeFont = [26]strokeGlyph{ /* A..Z */ }
```

Simple vector capitals, ~26 entries. Only twenty letters appear in the month
names, but the full alphabet costs a few lines and makes the table usable for
anything else the example wants engraved.

Placing a label:

- Center the name on its month's mid-angle. Advance is fixed per glyph
  (`emAdvance`), so the font is monospace and needs no width table.
- A glyph point `(gx, gy)` maps to world polar coordinates: `gy` offsets
  radially outward from the ring's text radius, and `gx` converts to an angular
  offset by arc length — `dθ = x_arc / R` — so the name curves along the ring
  instead of running off a tangent chord. Month names span roughly 25° of arc,
  which is far too much for a chord.
- Glyph "up" points radially outward everywhere, so labels read from outside the
  whole way around. This is what the reference does.
- Project both endpoints of a segment with `worldToScreen`, **then** thicken
  perpendicular in screen space by a fixed pixel width. That is what gives the
  glyph its in-plane squash while keeping the stroke weight uniform — a
  world-space thickness would fatten the top and bottom of the ring.

Every segment becomes a quad in the dial's mesh, so all twelve labels plus every
tick land in **one** `FillTrianglesColors` batch.

Gate labels on a legibility threshold: below a few pixels of em height, skip
them.

## Cost

Ticks ≈ 730 triangles, labels ≈ 850, belt ≈ 1200, galaxy cloud ≈ 2500 (static).
That is roughly one extra planet body's worth of geometry, in three or four
additional batches. Scratch meshes on `App` keep the frame allocation-free,
which is the standing constraint here.

## New `App` fields (`main.go`)

`belt`, `dial`, `galaxy` `bodyMesh` scratch buffers — separate so their
capacities settle independently, the reason already documented for
`body`/`corona`/`granules`. Plus `galaxyW, galaxyH float32` recording the size
the galaxy mesh was baked at.

## Verification

Run it and look:

```fish
go run ./examples/solar_system/
```

Check by eye: the ring frames the system in the default view, its text is
squashed at the top and bottom and upright at the sides, the marker advances a
year in Earth's 11.4 s, the belt sits between Mars and Jupiter with inner rocks
lapping outer ones, the galaxy stays put while the camera zooms, and every
planet carries a pole line that the sphere occludes on its far half — Uranus's
nearly horizontal, Venus's nearly vertical.

Tests:

```fish
cd /Users/mikeward/Documents/github/go-gui
go test ./examples/solar_system/
go test ./examples/solar_system/ -run 'PrimitiveCounts|BatchCount' -v
go test ./examples/solar_system/ -bench 'WholeFrame' -benchmem
make check-all
```

`zz_effbench_test.go` only logs counts, so it will not fail — but read its
output before and after and record the delta in the changelog, the way #429 and
#432 did.

New tests to add:

- every rune in the twelve month names has a stroke glyph
- `orbitPeriod` reproduces the table: Mercury 6.0, Earth 11.4, Jupiter 34.6
  within tolerance
- every rock's period falls strictly between Mars's and Jupiter's
- `dialAngle` maps a full Earth period to exactly one turn, and month boundaries
  are monotonic and sum to 365
- `axisDir` returns a unit vector; Uranus's is near-horizontal on screen and
  Mercury's near-vertical; the two halves have opposite `az` sign
- the galaxy mesh is rebuilt on a size change and not otherwise
- `TestDuplicateIDs`-style sanity: `drawSystem` still allocates nothing per
  frame (covered by the existing benchmarks' `ReportAllocs`)

Docs (per the doc-sync rule):

- `examples/solar_system/README.md` — new "Notes on technique" subsections for
  the stroke font in a projected plane, the belt's shared period law, and the
  baked galaxy band; update "What it demonstrates".
- `/CHANGELOG.md` — one entry in the same voice as the existing
  `examples/solar_system` entries.
- `prettier --prose-wrap always --print-width 80 --write` on both, or
  `make fmt-md`.

## Unresolved

- The reference's ring carries a zodiac track as well. Out of scope unless you
  want it.
- Belt rocks are not depth-sorted against each other or against the planets. At
  1-2px each this is invisible; say so if you want them interleaved into
  `drawPlanets`' back-to-front order.
