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
  and draws one level at a time — eight batches per frame instead of 220, and
  each band of a shaded sphere costs exactly one batch.

- **One encoding for "which body".** Selection and hover are a planet index,
  `selSun` for the sun, or `-1` for nothing. The sun gets a value outside the
  planet range rather than a row in the planets table: it does not orbit, so it
  has no `orbitPos` to recompute a screen position from, and it is not drawn by
  `drawPlanet`. `bodyAt` is the one place that encoding is decoded, and arrow
  stepping walks a _rank_ rather than the encoded value, because `selSun` is
  deliberately not adjacent to Mercury in arithmetic.

## Notes on technique

`DrawContext` fills come in two forms, and this example uses both.

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

Everything else that looks like a gradient is still flat shapes standing in for
one, and deliberately so:

- The planet shading is elliptical bands in a light-aligned frame. Circles
  cannot make a crescent, so no radial gradient can express it.
- Three translucent rings just outside each planet's silhouette feather the
  polygon edge into something that reads as anti-aliased. Three is few enough
  that a gradient would not pay for itself.

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

### Shading a sphere out of flat polygons

A planet is a real Lambert sphere, not an approximation of one. On a sphere lit
by a distant source the brightness at a surface point is `N·L`, so the lines of
equal brightness are the circles of constant angle from the light. Each band of
brightness is the strip of surface between two of those circles, traced in a
frame with the light as its pole and projected to screen.

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

Four things about this are worth knowing if you copy it.

**Circles cannot make a crescent.** The first version built the body from a
focal radial gradient — the `fx`/`fy` construction an SVG `radialGradient` uses,
which `gui/svg/xml_defs.go` parses. Its isophotes are circles. Pushing the focal
point out to the limb to fake a crescent puts a compact bright blob on a dark
disc, which reads as a second small sphere sitting inside the planet. The
terminator is an ellipse; no amount of softening the blob's edge turns one into
the other.

**Hidden points get pushed onto the limb.** A band's boundary circle usually
runs off the back of the sphere. Projecting the hidden part radially out to
radius `r` lands it on the silhouette — exactly where the band's boundary
belongs, and exactly where it already is at the crossing — so nothing needs
clipping.

**Bands are spaced evenly in intensity, not in angle.** Even angular spacing
looks fine across the middle of the lit side and stripes badly at the
terminator, because `cos` is flat at the poles and steepest at 90°: the same
slice of angle is a much bigger slice of brightness there. Stepping `cos`
directly makes every band the same color distance from its neighbour and spends
angular resolution where the eye is looking. The night half still has to be
subdivided even though every band there comes out the same color: at `cos = -1`
the boundary circle has collapsed to the antisolar point, and a strip run from a
single point fans out as a wedge instead of covering the region.

**Quads that merely touch leave hairlines.** Adjacent triangles sharing an edge
do not tile cleanly — the rasterizer antialiases each one on its own, so both
sides come out at partial coverage and the background shows through as a seam.
The bands are opaque, so growing every quad a fraction of a step past its own
share costs nothing and closes it.

**The color ramp needs the base color as a middle stop.** Interpolating straight
from night to light passes through the midpoint of those two, which is a
desaturated grey, and the planet loses its hue. With three stops the disc keeps
its own color across most of the lit side — Uranus stays cyan and Mars stays
red. The unlit side bottoms out at a fraction of the body's color rather than
black, on the grounds that a planet you cannot see is a planet you cannot click.

Level and segment counts scale with pixel radius, never a constant. A fixed
count bands the moment the thing gets big — it showed up first on a planet
zoomed to fill the view, and again on the sun's halo back when that was a ring
stack, 500px across with Jupiter focused and banded at 17px per ring. The halo
is a gradient now and no longer has a count to get wrong; the shading bands
still do.

A band is one flat color and its quads are emitted consecutively, so each band
costs a single batch. The full-system view emits about 245 triangle batches a
frame and a focused planet about 270. Off-screen bodies are culled: without
that, the seven planets outside the viewport at a focused zoom each still built
a full-resolution sphere.

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
