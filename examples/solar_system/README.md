# Solar System

An interactive orrery. Eight planets travel tilted elliptical orbits around a
glowing sun, over a starfield that twinkles. Mercury laps the sun in about six
seconds; Neptune takes nearly two minutes.

Planets are shaded spheres lit from the sun, each labelled under its disc, and
they show phases — a planet on the near side of its orbit turns its night side
toward you and reads as a crescent. Hover a body and it glows, with a tooltip
that follows the cursor. Click one and the camera zooms and pans to it, keeps
following it as it orbits, and opens a fact sheet with six stat cards and a fun
fact. The sun is a body like any other here: it hit-tests, it has its own fact
sheet, and it takes the first navigation dot. Navigation dots along the bottom
jump straight to any body, and the left and right arrow keys step through them
with wraparound. Scroll, pinch, or the `+` and `−` controls zoom in and out.
Escape, or a click on empty space, returns to the full system.

## Run

```fish
go run ./examples/solar_system/
```

## What it demonstrates

- **`DrawCanvas` as a full application surface** — the entire simulation is one
  canvas, with `Version` bumped every tick to invalidate the tessellation cache.
  Without that bump the canvas would render frame one forever.
- **Continuous animation** via `Animate` with `Repeat`, driving a single `tick`
  that advances time, the camera, and hover in one place.
- **A camera whose target moves.** A selected planet keeps orbiting, so the
  camera's goal is re-read every tick rather than captured at click time.
  Interrupting a transition restarts the tween from wherever the camera actually
  is, so there is no snap back to the previous anchor.
- **Hit-testing drawn geometry.** Screen positions are computed once per tick
  and both the hit-test and the painting read them, which is what guarantees the
  two agree. A minimum hit radius keeps sub-pixel bodies clickable.
- **Per-callback coordinate spaces.** `OnMouseMove` and `OnClick` are
  shape-relative; `OnHover` and `OnMouseLeave` are window-absolute. The canvas
  takes no padding, so shape-relative coordinates are also content coordinates.
- **Gesture and scroll input.** `GesturePinch` reports a _cumulative_ scale, so
  the per-event delta is recovered by tracking the previous value. Scroll
  arrives in lines or in points depending on `ScrollPrecise`.
- **The float system.** The zoom controls and the info panel are ordinary
  widgets floated over the canvas. Float children are excluded from parent
  sizing, so the canvas keeps the whole window and the camera math never reacts
  to a panel appearing.
- **Batch-aware drawing.** `DrawContext` merges only _consecutive_ same-color
  triangles, so the starfield quantizes its twinkle to eight brightness levels
  and draws one level at a time — eight batches per frame instead of 220. A
  shaded sphere goes the other way: it is one mesh carrying a color per vertex,
  so the whole body is a single batch however fine its ramp.

- **One encoding for "which body".** Selection and hover are a planet index,
  `selSun` for the sun, or `-1` for nothing. The sun gets a value outside the
  planet range rather than a row in the planets table: it does not orbit, so it
  has no `orbitPos` to recompute a screen position from, and it is not drawn by
  `drawPlanet`. `bodyAt` is the one place that encoding is decoded, and arrow
  stepping walks a _rank_ rather than the encoded value, because `selSun` is
  deliberately not adjacent to Mercury in arithmetic.

## Notes on technique

`DrawContext` fills come in three forms, and this example uses all of them.

The two glows are real gradients (`FilledCircleGradient`, issue #398). Each is
one fill:

- The sun's halo keeps its **cubic** alpha falloff. Linear reads as a hard-edged
  disc, and even quadratic leaves a visible shoulder at this radius.
- A hovered planet's halo is the same shape at planet scale.

Both used to be stacks of concentric translucent discs — up to 140 of them for
the sun — where the ring count was the smoothness knob and seams showed when it
was turned down. `haloStops` (`draw.go`) samples the opacity those stacks
accumulated to, so the appearance is the one they had, at one fill each and with
nothing left to tune.

The planet shading is the third form: `FillTrianglesColors` (issue #400), where
the caller evaluates its own shading model and hands over geometry plus one
color per vertex. A gradient cannot stand in for it — circles cannot make a
crescent, and neither can any other conic isoline; the argument is below.

What is left is flat shapes standing in for a gradient, deliberately: three
translucent rings just outside each planet's silhouette feather the polygon edge
into something that reads as anti-aliased. Three is few enough that a gradient
would not pay for itself.

### Painting a star

A sun is not a bright disc, and four separate things have to be true before it
reads as one.

**The color belongs at the edge, not the middle.** Ramping the face orange all
the way in reads as an amber lamp. The disc is near-white across most of its
face, with gold kept to a rind at the limb and to the halo around it.

**The limb is brighter than the body behind it.** The disc ramp runs bright rind
→ slightly deeper body → white core, and the dip between the first two is the
point: that is what limb brightening looks like. A plain center-out ramp is a
coin lit from the front.

**The face is mottled.** Granulation is a fixed set of soft blobs, each a small
stack of nested circles so it has a falloff instead of a hard edge. They are
drawn tier-major with lit and dark cells in separate passes, so every circle in
a pass shares one color and the whole texture costs `2 x tiers` batches rather
than one per cell — drawing cell-major would cost seventy times more. Cells are
placed by `sqrt` of a uniform so they spread over the _area_ instead of crowding
the middle, and each one's far edge is clamped inside the limb so the texture
needs no clipping.

**The rim is ragged.** A clean circular edge looks like a coin no matter how
bright it is, so the corona is a fringe whose radius is modulated by a smoothed
noise ring that drifts slowly with time. Raw uniform noise gives a spiky rim
that reads as a gear, and the ring has to wrap — a seam at the join would show
as a notch that never moves.

The corona's tiers are _nested_ annuli sharing boundaries, each band's inner
edge being the previous band's outer edge. The first attempt stacked them all
from one shared inner radius, which accumulated alpha there and painted a hard
bright ring inside the limb — a worse artifact than the soft edge it was meant
to give.

### Shading a sphere the renderer cannot express

A planet is a real Lambert sphere, not an approximation of one. On a sphere lit
by a distant source the brightness at a surface point is `N·L`, so the lines of
equal brightness are the circles of constant angle from the light. The body is a
mesh of exactly those circles — one ring of vertices per intensity, traced in a
frame with the light as its pole, projected to screen, and handed to
`FillTrianglesColors` with the intensity carried as a color on every vertex.

The light vector is built once, in screen coordinates, and carries everything
the shading needs. `diskTilt` is the sine of the camera's elevation above the
orbital plane — that is what the vertical squash in `worldToScreen` _is_ — so
the camera's basis is `R = (1,0,0)`, `D = (0, sin e, -cos e)`,
`V = (0, cos e, sin e)`, and projecting the world light direction onto it gives
all three components. The `z` component falls out as `cos` of the phase angle,
so **phase** costs no extra derivation: a planet on the near side of its orbit
has the sun beyond it and reads as a crescent. Its range is about `[0.06, 0.94]`
rather than `[0, 1]`, because a camera 29° above the plane never sees a pure new
or full phase. Most orreries skip phase entirely and paint every planet fully
lit.

Five things about this are worth knowing if you copy it.

**Circles cannot make a crescent.** The first version built the body from a
focal radial gradient — the `fx`/`fy` construction an SVG `radialGradient` uses,
which `gui/svg/xml_defs.go` parses. Its isophotes are circles. Pushing the focal
point out to the limb to fake a crescent puts a compact bright blob on a dark
disc, which reads as a second small sphere sitting inside the planet. The
terminator is an ellipse; no amount of softening the blob's edge turns one into
the other.

**Nor can any other gradient.** The isophotes _are_ ellipses, which makes an
elliptical gradient look like the answer, and it is not. For an orthographic
unit sphere lit by `l`, the locus `N·L = k` projects to an ellipse centered at
`|l₂d|·k` along the screen light direction, with semi-axis `√(1−k²)` across the
light and `√(1−k²)·|lz|` along it. The aspect ratio is constant in `k`, so a
gradient could match that part. Two things defeat it. The radius law is
`√(1−k²)`, not linear: the isoline is a point at `k = −1`, opens to the full
limb at the terminator, and closes to a point again at `k = 1`, so the isophotes
at 30° and 150° are two small disjoint ellipses on opposite sides of the disc,
neither containing the other. Gradient level sets are nested by construction, so
no gradient emits that family. And each isophote is clipped at the silhouette,
which a gradient has no notion of. That is what `FillTrianglesColors` exists
for: the shading model is solved here and handed over already evaluated.

**Hidden points get pushed onto the limb.** A band's boundary circle usually
runs off the back of the sphere. Projecting the hidden part radially out to
radius `r` lands it on the silhouette — exactly where the band's boundary
belongs, and exactly where it already is at the crossing — so nothing needs
clipping.

**Where the rings go is decided by the limb, not by the ramp.** Even steps in
intensity are the obvious spacing and they make the silhouette go polygonal,
because the mesh boundary _is_ the limb: a ring only partly on the near side
ends on the silhouette, and the mesh edge between two such rings is a chord of
it. A limb point's intensity is `m·cos(delta)` — `delta` the angle around the
limb from the light's screen direction, `m` the light's screen-plane length — so
even steps in intensity are wildly uneven steps in `delta`, growing as a square
root near `delta = 0` until a single chord spans 20°. With the light in the
screen plane that lands on the disc's own edge, which is why the facets showed
on planets drawn beside the sun and nowhere else.

So the rings that reach the limb are spaced evenly in `delta` instead, which
makes every limb chord the same length. Those are the rings with
`|cos(phi)| < m`; outside that band a ring is either wholly visible — a closed
curve around the pole the light points at — or wholly hidden. The visible cap
gets rings spaced evenly in `phi` and the hidden one gets none, which is also a
saving: even spacing spent nearly half its rings on the far side only for
`appendTri` to drop them.

**The mesh must be watertight and consistently wound.** Both follow from how a
vertex-colored batch is rasterized: `gui/backend/soft` treats it as one path and
accumulates _signed_ coverage. Adjacent triangles that merely touch antialias
independently and leak the background through as a hairline, so consecutive
rings share their vertices rather than being drawn as separate strips — the same
array, used as the outer edge of one strip and the inner edge of the next. And a
triangle wound against its neighbours subtracts from them, cutting a seam
through the body; the limb clamp can reorder a quad's corners, so `appendTri`
measures the signed area and emits the vertices in a consistent order rather
than assuming one. It drops the zero-area case in the same step, which is what
removes the rings lying entirely on the far side.

The flat-band version this replaced needed a fifth trick: every quad was grown a
fraction of a step past its own share, so opaque neighbours overlapped instead
of merely touching. With a per-vertex ramp that overlap would itself show as
banding, and a shared-vertex mesh has no seam left to close.

**The color ramp needs the base color as a middle stop.** Interpolating straight
from night to light passes through the midpoint of those two, which is a
desaturated grey, and the planet loses its hue. With three stops the disc keeps
its own color across most of the lit side — Uranus stays cyan and Mars stays
red. The unlit side bottoms out at a fraction of the body's color rather than
black, on the grounds that a planet you cannot see is a planet you cannot click.

Ring and segment counts scale with pixel radius, never a constant, but they buy
much less than they used to. They now resolve only the _geometry_ — the
curvature of the elliptical rings and the chord error where the mesh boundary
meets the limb, both of which fall off as the square of the spacing. The ramp
itself is continuous however coarse the mesh is, so the counts came down by more
than half when the color moved onto the vertices: a full-system frame went from
232 triangle batches and 40.6k triangles to 79 and 30.2k, and a Jupiter-focused
frame from 137 and 52.6k to 89 and 42.7k.

Off-screen bodies are still culled: without that, the seven planets outside the
viewport at a focused zoom each built a full-resolution sphere anyway.

Orbit ellipses are drawn with `Arc`, the ellipse primitive, with the vertical
radius squashed to tilt the plane. Each ellipse's center is offset by `a·e`
because the sun sits on the focus, not the center — without that offset the
eccentricity would not be visible at all.

Saturn's rings are drawn in three passes: the back half of the arc, then the
body, then the front half — which is what makes them pass behind it. Planets are
painted in screen-Y order, so a nearer body correctly overlaps a farther one.

Orbital periods and radii are compressed, not to scale. The real range spans 88
to 60,190 days — 684x, which nobody can watch. Periods are `realDays^0.45`,
which preserves the ordering exactly while keeping every gap visible. Orbit
radii use a milder `realAU^0.38`, because a stronger exponent packs the inner
four planets inside the sun's halo.

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
