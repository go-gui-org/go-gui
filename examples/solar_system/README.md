# Solar System

An interactive orrery. Eight planets travel tilted elliptical orbits around a
glowing sun, over a starfield that twinkles. Mercury laps the sun in about seven
and a half seconds. Neptune takes a little over two and a half minutes.

Planets are shaded spheres lit from the sun, each labelled under its disc. They
show phases. A planet on the near side of its orbit turns its night side toward
you and reads as a crescent. Each one carries a procedural surface and turns on
a tilted axis. Jupiter's belts drift past. Earth shows continents and ice caps.
Venus turns backwards, and Uranus rolls along its orbit and does not spin
upright. Hover a body and it glows, with a tooltip that follows the cursor.
Click one and the camera zooms and pans to it. It follows the planet as it
orbits and opens a fact sheet with six stat cards and a fun fact. The sun is a
body like any other here: it hit-tests, it has its own fact sheet, and it takes
the first navigation dot. Navigation dots along the bottom jump straight to any
body, and the left and right arrow keys step through them with wraparound.
Scroll, pinch, or the `+` and `−` controls zoom in and out. Escape, or a click
on empty space, returns to the full system.

## Run

```fish
go run ./examples/solar_system/
```

## What it demonstrates

- **`DrawCanvas` as a full application surface** — the entire simulation is one
  canvas, with `Version` bumped every tick to invalidate the tessellation cache.
  Without that bump, the canvas renders frame one forever.
- **Continuous animation** via `Animate` with `Repeat`. It drives a single
  `tick` that advances time, the camera, and hover in one place.
- **A camera whose target moves.** A selected planet continues to orbit. The
  code re-reads the camera's goal every tick rather than capturing it at click
  time. Interrupting a transition restarts the tween from wherever the camera
  actually is, so there is no snap back to the previous anchor.
- **Hit-testing drawn geometry.** Screen positions are computed once per tick,
  and both the hit-test and the painting read them. That is what guarantees the
  two agree. A minimum hit radius keeps sub-pixel bodies clickable.
- **Per-callback coordinate spaces.** `OnMouseMove` and `OnClick` are
  shape-relative. `OnHover` and `OnMouseLeave` are window-absolute. The canvas
  takes no padding, so shape-relative coordinates are also content coordinates.
- **Gesture and scroll input.** `GesturePinch` reports a _cumulative_ scale, so
  the app recovers the per-event delta by tracking the previous value. Scroll
  arrives in lines or in points depending on `ScrollPrecise`.
- **The float system.** The zoom controls and the info panel are ordinary
  widgets floated over the canvas. Float children are excluded from parent
  sizing, so the canvas keeps the whole window and the camera math never reacts
  when a panel appears.
- **Batch-aware drawing.** `DrawContext` merges only _consecutive_ same-color
  triangles, so the starfield quantizes its twinkle to eight brightness levels
  and draws one level at a time. That is eight batches per frame instead of 220.
  A shaded sphere goes the other way: it is one mesh that carries a color per
  vertex. The whole body is then a single batch, however fine its ramp.

- **Texturing without a texture path.** The renderer has no UV coordinates
  anywhere — `FillTrianglesColors` carries per-vertex colors and nothing else.
  The app samples planet surfaces on the CPU, once per mesh vertex, and folds
  them into the color that vertex already had. No engine change, and every
  backend still works.

- **One encoding for "which body".** Selection and hover are a planet index,
  `selSun` for the sun, or `-1` for nothing. The sun gets a value outside the
  planet range rather than a row in the planets table. It does not orbit, so it
  has no `orbitPos` to recompute a screen position from. `drawPlanet` does not
  draw it. `bodyAt` is the one place that encoding is decoded. Arrow stepping
  walks a _rank_ rather than the encoded value, because `selSun` is deliberately
  not adjacent to Mercury in arithmetic.

## Notes on technique

`DrawContext` fills come in three forms, and this example uses all of them.

The two glows are real gradients (`FilledCircleGradient`, issue #398). Each is
one fill:

- The sun's halo keeps its **cubic** alpha falloff. Linear reads as a hard-edged
  disc, and even quadratic leaves a visible shoulder at this radius.
- A hovered planet's halo is the same shape at planet scale.

Both used to be stacks of concentric translucent discs, up to 140 of them for
the sun. The ring count was the smoothness knob. Seams showed when the count was
decreased. `haloStops` (`draw.go`) samples the opacity those stacks accumulated
to, so the appearance is the one they had. That is one fill each, with nothing
left to tune.

The planet shading is the third form: `FillTrianglesColors` (issue #400). The
caller evaluates its own shading model and hands over geometry plus one color
per vertex. A gradient cannot stand in for it — circles cannot make a crescent,
and neither can any other conic isoline. The argument is below.

The remaining approach is flat shapes in place of a gradient, deliberately.
Three translucent rings just outside each planet's silhouette feather the
polygon edge, so it reads as anti-aliased. Three is few enough that a gradient
does not pay for itself.

### Painting a star

A sun is not a bright disc. Four separate things must be true before it reads as
one.

**The color belongs at the edge, not the middle.** If the face ramps orange all
the way in, it reads as an amber lamp. The disc is near-white across most of its
face, with gold kept to a rind at the limb and to the halo around it.

**The limb is brighter than the body behind it.** The disc ramp runs bright rind
→ slightly deeper body → white core. The dip between the first two is the point:
that is what limb brightening looks like. A plain center-out ramp is a coin lit
from the front.

**The face is mottled.** Granulation is a fixed set of soft blobs. Each blob is
a small stack of nested circles, so it has a falloff instead of a hard edge.
They are drawn tier-major with lit and dark cells in separate passes. Every
circle in a pass shares one color, so the whole texture costs `2 x tiers`
batches rather than one per cell. A cell-major pass costs seventy times more.
The app places cells by `sqrt` of a uniform, so they spread over the _area_
instead of crowding the middle. It clamps each cell's far edge inside the limb,
so the texture needs no clipping.

**The rim is ragged.** A clean circular edge looks like a coin no matter how
bright it is. The corona is a fringe whose radius follows a smoothed noise ring
that drifts slowly with time. Raw uniform noise gives a spiky rim that reads as
a gear. The ring must wrap. A seam at the join shows as a notch that never
moves.

The corona's tiers are _nested_ annuli that share boundaries. Each band's inner
edge is the previous band's outer edge. The first attempt stacked them all from
one shared inner radius. Alpha accumulated there and painted a hard bright ring
inside the limb — a worse artifact than the soft edge the design intended.

### Shading a sphere the renderer cannot express

A planet is a real Lambert sphere, not an approximation of one. On a sphere lit
by a distant source, the brightness at a surface point is `N·L`. The lines of
equal brightness are the circles of constant angle from the light. The body is a
mesh of exactly those circles, one ring of vertices per intensity. The app
traces the mesh in a frame with the light as its pole and projects it to screen.
It hands the result to `FillTrianglesColors`, with the intensity as a color on
every vertex.

The light vector is built once, in screen coordinates, and carries everything
the shading needs. `diskTilt` is the sine of the camera's elevation above the
orbital plane — that is what the vertical squash in `worldToScreen` _is_. The
camera's basis is `R = (1,0,0)`, `D = (0, sin e, -cos e)`,
`V = (0, cos e, sin e)`. The projection of the world light direction onto this
basis gives all three components. The `z` component falls out as `cos` of the
phase angle, so **phase** costs no extra derivation. A planet on the near side
of its orbit has the sun beyond it and reads as a crescent. Its range is about
`[0.06, 0.94]` rather than `[0, 1]`, because a camera 29° above the plane never
sees a pure new or full phase. Most orreries skip phase entirely and paint every
planet fully lit.

Five things about this are worth knowing if you copy it.

**Circles cannot make a crescent.** The first version built the body from a
focal radial gradient. That is the `fx`/`fy` construction an SVG
`radialGradient` uses, which `gui/svg/xml_defs.go` parses. Its isophotes are
circles. If the focal point is pushed out to the limb to fake a crescent, a
compact bright blob appears on a dark disc. It reads as a second small sphere
inside the planet. The terminator is an ellipse. No amount of softening the
blob's edge turns one into the other.

**Nor can any other gradient.** The isophotes _are_ ellipses, which makes an
elliptical gradient look like the answer, and it is not. For an orthographic
unit sphere lit by `l`, the locus `N·L = k` projects to an ellipse. The ellipse
is centered at `|l₂d|·k` along the screen light direction, with semi-axis
`√(1−k²)` across the light and `√(1−k²)·|lz|` along it. The aspect ratio is
constant in `k`, so a gradient can match that part. Two things defeat it. The
radius law is `√(1−k²)`, not linear. The isoline is a point at `k = −1`. It
opens to the full limb at the terminator, and closes to a point again at
`k = 1`. The isophotes at 30° and 150° are two small disjoint ellipses on
opposite sides of the disc, neither containing the other. Gradient level sets
are nested by construction, so no gradient emits that family. Each isophote is
clipped at the silhouette, which a gradient has no notion of. That is what
`FillTrianglesColors` exists for: the app solves the shading model here and
hands it over already evaluated.

**Hidden points get pushed onto the limb.** A band's boundary circle usually
runs off the back of the sphere. The hidden part, projected radially out to
radius `r`, lands on the silhouette. That is exactly where the band's boundary
belongs and where it already is at the crossing, so nothing needs clipping.

**Where the rings go is decided by the limb, not by the ramp.** Even steps in
intensity are the obvious spacing. They make the silhouette go polygonal,
because the mesh boundary _is_ the limb. A ring only partly on the near side
ends on the silhouette. The mesh edge between two such rings is a chord of it. A
limb point's intensity is `m·cos(delta)`, with `delta` the angle around the limb
from the light's screen direction and `m` the light's screen-plane length. Even
steps in intensity are wildly uneven steps in `delta`. They grow as a square
root near `delta = 0`, until a single chord spans 20°. With the light in the
screen plane, that lands on the disc's own edge. That is why the facets showed
on planets drawn beside the sun and nowhere else.

So the rings that reach the limb are spaced evenly in `delta` instead, which
makes every limb chord the same length. Those are the rings with
`|cos(phi)| < m`. Outside that band, a ring is either wholly visible or wholly
hidden. A visible ring is a closed curve around the pole the light points at.
The visible cap gets rings spaced evenly in `phi`, and the hidden one gets none.
This is also a saving. Even spacing spent nearly half its rings on the far side,
only for `appendTri` to drop them.

**The mesh must be watertight and consistently wound.** Both follow from how a
vertex-colored batch is rasterized: `gui/backend/soft` treats it as one path and
accumulates _signed_ coverage. Adjacent triangles that merely touch antialias
independently and leak the background through as a hairline. Consecutive rings
share their vertices instead of separate strips. They use the same array as the
outer edge of one strip and the inner edge of the next. A triangle wound against
its neighbours subtracts from them and cuts a seam through the body. The limb
clamp can reorder a quad's corners. `appendTri` measures the signed area and
emits the vertices in a consistent order rather than assuming one. It drops the
zero-area case in the same step. That removes the rings that lie entirely on the
far side.

The flat-band version this replaced needed a fifth trick: every quad was grown a
fraction of a step past its own share. Opaque neighbours then overlapped instead
of merely touching. With a per-vertex ramp, that overlap shows as banding. A
shared-vertex mesh has no seam left to close.

**The color ramp needs the base color as a middle stop.** Interpolation straight
from night to light passes through the midpoint of those two. That midpoint is a
desaturated grey, and the planet loses its hue. With three stops, the disc keeps
its own color across most of the lit side — Uranus stays cyan and Mars stays
red. The unlit side stops at a fraction of the body's color rather than black. A
planet you cannot see is a planet you cannot click.

Ring and segment counts scale with pixel radius, never a constant. Moving color
onto the vertices cut them by more than half. The ramp is then continuous
however coarse the mesh is. The counts have only the _geometry_ left to resolve.
The geometry is the curvature of the elliptical rings and the chord error where
the mesh boundary meets the limb. Both fall off as the square of the spacing. A
full-system frame went from 232 triangle batches and 40.6k triangles to 79 and
about 31k.

Surface texture then gave those counts a second job and reversed that argument
in part. The app samples albedo per vertex and interpolates it between vertices.
Mesh density now bounds detail in a way intensity never did. The ceilings are 72
rings and 128 segments, matched to the 128x64 textures, where before they were
36 and 64. The rates that approach them are unchanged, so this is invisible at
ordinary zoom. A full-system frame is 79 batches and 31.6k triangles either way.
A selected Jupiter is 105 batches and 47.0k triangles, against 46.8k before. It
is only at maximum zoom, where a planet is hundreds of pixels across and both
ceilings actually bind, that the raise costs anything. It is 99.6k triangles
against 85.9k.

Off-screen bodies are still culled. Without that, the seven planets outside the
viewport at a focused zoom each build a full-resolution sphere anyway.

### Texturing a renderer that has no textures

There is no textured-triangle path in go-gui. Adding one is out of scope.
`DrawContext` offers flat fills, gradients, an axis-aligned `Image`, and
`FillTrianglesColors` — per-vertex colors with no UVs. The `RenderSvg` command
that carries arbitrary triangles explicitly zeroes its texcoords. The Metal
image pipeline derives UVs from a hardcoded quad rather than reading them. Real
UV support needs a new render command, a new sampling pipeline, and edits to six
backends.

Pre-baking a sphere image per frame is the obvious alternative, and it is a
trap. `gui.UseImage` does hand a CPU pixel buffer to the GPU, but it is
content-keyed with **no invalidation API**, by explicit design. The Metal
texture cache is a 128-entry LRU. For nine planets that spin, a per-frame key
thrashes the cache with nine texture creates and destroys every frame.

So the app samples the surface on the CPU, once per mesh vertex, and folds it
into the vertex color the mesh already carries. The accepted cost is that detail
is bounded by vertex density and Gouraud-interpolated between vertices. The
raised tessellation ceilings above pay for it.

**Sampling turned out to be nearly free, because the mesh already computed what
it needs.** Two facts do the work. The light basis stores its third axis as 2D
because its `z` is exactly zero by construction. `lightBasis.point` already
computes the depth term for its own visibility test — which _is_ the normal's
`z`. A vertex at `(cosPhi, sinPhi, ct, st)` has camera-space direction
`n = cosPhi*l + sinPhi*(ct*u + st*v)`, and three things then collapse:

1. **The body axes are pre-transformed into camera space, once.** Axial tilt and
   camera elevation are both constants. The code applies the world-to-camera
   rotation when it builds the surface, and never again. `n_world · a` becomes
   `n · A` and the world transform disappears.
2. **The ring linearizes it.** For any fixed axis `W`,
   `n·W = cosPhi*(l·W) + sinPhi*(ct*(u·W) + st*(v·W))`. That is nine dot
   products per body per frame, folded into nine scalars per ring. Per vertex,
   it reuses the `ct`/`st` that `appendRing` already had.
3. **The shading ramp folds to an affine map.** `sphereTone` mixes three colors
   all derived from one base. At a fixed intensity, it collapses to
   `channel*k + w` — two scalars per ring instead of three color operations per
   vertex.

The result is **no new transcendental per vertex**: about fifteen multiply-adds,
one polynomial `atan2`, a texel fetch, and the ramp. Spin is applied as a
longitude offset rather than as a rotation of the basis — algebraically
identical and much cheaper.

The `atan2` is a polynomial with a worst case of 0.0102 rad. That is not
sloppiness: one texel of a 128-wide texture spans 0.049 rad. The entire error
budget is about a fifth of the smallest thing the result can address.

**Noise is evaluated on the 3D direction, not on the `(u,v)` grid.** One extra
multiply, and it buys both seamlessness in longitude and freedom from polar
pinching. The direction at `u=0` and `u=1` is literally the same point. The test
for this compares the step across the wrap column against the worst step
anywhere else in the same texture. It does not use a fixed threshold. A
coastline can be a sharp edge. The seam cannot be sharper than everything else.

**Every texture's mean is normalized back onto its `Planet.Color`.** That is
what makes texturing safe to add. It is not a change to every other part of the
app. The nav dots, the labels, the tooltip, and the flat tone a body under
`flatBodyRadius` draws all still read from that one color. A planet too small to
show any texture still looks like the planet it did before. The shift is
additive, so contrast survives. It is also applied iteratively. Clamping at the
byte range pulls the mean back a little when a texture has ice caps or a dark
spot.

Rows are uniform in `sin(latitude)` rather than in latitude. Every row band then
covers the same area of the sphere, so texel density is even instead of crowding
the poles. Since the sampler already holds `sin(latitude)`, the scheme removes
an `asin` from the per-vertex path.

Orbit ellipses are drawn with `Arc`, the ellipse primitive, with the vertical
radius squashed to tilt the plane. Each ellipse's center is offset by `a·e`
because the sun sits on the focus, not the center. Without that offset, the
eccentricity is not visible at all.

Saturn's rings are drawn in three passes: the back half of the arc, then the
body, then the front half. That is what makes them pass behind it. Planets are
painted in screen-Y order, so a nearer body correctly overlaps a farther one.

Orbital periods and radii are compressed, not to scale. The real range spans 88
to 60,190 days — 684x, which nobody can watch. Periods are `realDays^0.45`. This
preserves the ordering exactly and keeps every gap visible. Orbit radii use a
milder `realAU^0.38`, because a stronger exponent packs the inner four planets
inside the sun's halo.

Rotation is compressed the same way and for the same reason. Real periods run
from 9.9 h to 5,832 h, a 590x range, so `RotS` is `|realHours|^0.45 x 1.07`.
Axial tilts are the real values in radians. Venus carries its retrograde spin as
a negative `RotS` rather than as the equivalent 177.4° tilt — identical physics,
more legible in a table.

Worth being honest about one consequence: rotation and orbit are compressed on
_separate_ scales. The ratio between a planet's day and its year is not
preserved. Mercury's real sidereal day is 0.67 of its year. Here it is 4.65. The
distortion happens to run in the direction that makes Mercury's and Venus's fact
sheets read as true on screen. The sheets claim "a day lasts longer than a
year". That is a coincidence of the scaling, not a property the model keeps.

## Provenance

This app was generated entirely by AI, from the following prompt:

> Build an interactive solar system where planets orbit a glowing sun on
> elliptical paths with a twinkling star background; animate each planet at a
> different relative speed (Mercury fastest, Neptune slowest). Hovering a planet
> makes it glow and shows a small name tooltip near the cursor. Clicking a
> planet smoothly zooms and pans to center it and opens a bottom info panel with
> the planet name and a set of stat cards (diameter, mass, distance, day length,
> gravity, temperature) plus a fun fact. Include bottom navigation dots to jump
> between planets, support left/right arrow keys, and let users zoom with
> scroll/pinch and +/− buttons. Saturn should have visible rings. Clicking empty
> space or pressing Escape returns to the full system view.
