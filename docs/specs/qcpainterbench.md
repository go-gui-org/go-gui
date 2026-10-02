# qcpainterbench port

Status: implemented as `examples/qcpainterbench`. Issue #723.

## Problem

Qt published qcpainterbench, a benchmark that compares QPainter with Qt Canvas
Painter. Its six workloads exercise low-level 2D drawing, not widgets or layout.
The same workloads drawn with go-gui give a renderer benchmark that is based on
a real, current workload, and they show which canvas operations go-gui does not
have.

## Source and license

The reference is `tests/manual/qcpainterbench` in
[qt/qtcanvaspainter](https://github.com/qt/qtcanvaspainter). Its C++ and QML
files are `LicenseRef-Qt-Commercial OR GPL-3.0-only`. go-gui is not GPL, so the
port copies no Qt code. This spec records the behavior (constants, formulas and
the order of operations), and the example is written from this spec.

Two assets are also not copied:

- `circle.png` (128×128 gray and alpha). The example generates a similar 128×128
  disc in code and registers it with `gui.UseImage`.
- Roboto (OFL). The example uses the theme font. Text cost depends on the font,
  so text numbers are not directly comparable with Qt's.

## Frame

- Window 375×667, background `#404040`. Qt uses 4× MSAA and turns vsync off.
- `t` is wall-clock seconds, from 0 to 360, then it loops. A click pauses and
  resumes it.
- `w`, `h` are the canvas size. `s = min(w, h)`.
- **Render count N** draws all enabled tests N times per frame, with the same
  geometry. Only the time changes: `t += 0.3` after each pass. Cost is linear in
  N.
- The test mask is: 1 ruler, 2 circles, 4 lines, 8 bars, 16 icons, 32 flower.
  The default is 31 (the flower is off).
- Tests run in mask order in each pass.

## Workloads

Colors: white, gray `(180,180,180)`, black, color1 `(180,190,40,20)`, color2
`(255,255,255,150)`, color3 `(255,255,255,80)`.

### 1. Ruler

Called as `ruler(0, 0.02h, w, 0.05h, t)`; below, `h` is the ruler height.

- Ticks start at `x = 0.05w` and repeat every `space = 0.03w + sin(t)·0.02w`
  while `x < w`.
- Tick i has height `0.5h` if `i%10 == 0`, `0.3h` if `i%5 == 0`, else `0.2h`.
  All ticks are one stroke, `#E0E0E0`, width 1.
- Ticks at multiples of 10 get the label `i`. Ticks at other multiples of 5 get
  a label only when `space > 0.02w`. Labels are `#E0E0B0`, size `10 + 0.01w`,
  centered on `(x, y + h)`.
- At w = 375 there are 19 to 95 ticks.

### 2. Circles

Three gauges: size `50 + 0.5s` at `(w/2 − size/2, 0.1h)` with 8 rings and time
`2t`; size `20 + 0.2s` at `(0.05w, 0.55h)` with 6 rings and time `3t`; and the
same size at `(w − size − 0.05w, 0.55h)` with 3 rings and time `t`.

For a gauge of size `S` with `n` rings at time `T`:

- `bar = 0.3S/n`, `margin = 0.2·bar`, `prog = 0.6 + 0.4·sin(0.8T)`,
  `lw = bar·prog`.
- Ring radii start at `S/2 − lw` and step in by `lw + margin`.
- Each ring: a full circle stroked `(215,215,215,50)` at width `lw`. Then an arc
  from −π/2 through `2π·((n−i)/n)·prog` (clockwise on screen), stroked
  `(200−150f, 200−50f, 100+50f, 255·prog)` with `f = i/n`, width `lw`, round
  caps.
- Total 17 rings: 34 strokes per pass.

### 3. Lines

Three graphs on the baseline `y = h`, all `w` wide: height `h` with 4 points at
time `t`; `0.8h` with 6 points at `t + 10`; `0.6h` with 12 points at `t/2`.

For `n` points of graph height `H`:

- `dx = w/(n−1)`, `dot = 4 + 0.005w`.
- Point i is at `x = i·dx`, `y = h − H·0.8·(0.5 + 0.1·sin(0.2(i+1)T))`.
- The curve joins the points with cubics. The control points are `dx/2` after
  the previous point and `dx/2` before the next one, at those points' heights.
- The area under the curve is filled with a linear gradient: color1 at the
  baseline, color2 at height `H`.
- The curve is stroked gray, width `1 + 0.2·dot`.
- Each point is a circle of radius `0.8·dot`, filled white and stroked black at
  width `0.2·dot`.
- Total 22 points: per pass 3 gradient fills, 3 curve strokes, 22 dots.

### 4. Bars

Four graphs on the baseline `y = h`: 6 bars of height `0.8h` at `3t`; 10 at
`0.4h` at `t + 2`; 20 at `0.3h` at `2t + 2`; 40 at `0.2h` at `3t + 2`.

For `n` bars of height `H`:

- `dx = w/n`, `bw = 0.8dx`, `margin = dx − bw`.
- Bar i has height `H·(0.5 + 0.5·sin(0.1i + T))` and left edge
  `i·dx + margin/2`.
- Qt truncates the left edge, the baseline, the width and the height to
  integers. It then adds 0.5 to x and 1.5 to y, so the 1 px stroke lands on
  pixel centers.
- All bars of a graph are one path, filled color3 and stroked black at width 1.
- Total 76 bars per pass.

### 5. Icons and text

- `size = 16 + 0.05s`, font size `size/2`, white.
- 20 items. Item i is at `x = (w − size)·i/20`,
  `y = 0.2h + 0.1h + 0.1h·sin(0.1(i+1)t)`.
- Each item draws the 128×128 image scaled to `size×size`, then the number
  `i + 1` centered on it.
- Per pass: 20 images, 20 short texts.

### 6. Flower

- `fs = 80 + 0.6s`, box at `(w/2 − fs/2, h − fs)`, center `(cx, cy)`,
  `leaf = fs/2`.
- Point k is on the circle of radius `leaf` at angle `2π(1 − k/12) − π/2`.
- The path starts at the center. For k = 0, 2, …, 10 it adds a quadratic to
  point k+1 with control point k, then a quadratic back to the center with
  control point k+2. That makes 6 petals of 12 quadratics. Qt builds the path
  once and keeps it until the canvas is resized.
- Each frame rotates the path by `20°·sin(t)` about the center.
- Fill: a radial gradient from the center, radius `leaf`. It goes from
  `(255(0.5 + 0.5 sin 2t), 0, 255(0.5 + 0.5 sin(t + π)))` to white.
- Stroke: `#40000000`, width 4.
- Then a white dot of radius `0.1·fs` at the center, not rotated.

### State that carries over

Qt never saves or restores painter state between tests. The circles set round
caps and round joins, and the bars then set miter joins. So the curves and dots
draw with round caps and round joins, and in later passes the ruler and flower
draw with round caps and miter joins. Clipping, scale and global alpha are not
used.

## Mapping to go-gui

| Qt operation                  | go-gui                                                             | Fidelity                     |
| ----------------------------- | ------------------------------------------------------------------ | ---------------------------- |
| Ruler tick path, one stroke   | `Line` per tick                                                    | Same batch, same result      |
| `fillText` centered           | `TextWidth` + `FontHeight` + `Text`                                | Theme font, not Roboto       |
| `circle` stroke               | `Circle`                                                           | Exact                        |
| `arc` stroke, round caps      | `Arc` + two half-disc `FilledArc` caps                             | Emulated caps                |
| Cubic path, gradient fill     | Flatten in the example, strip to baseline, `FillTrianglesGradient` | Emulated path fill           |
| Cubic path, round-join stroke | `PolylineJoined` on the flattened points                           | Miter/bevel joins, not round |
| Dot circles, fill + stroke    | `FilledCircle` + `Circle`                                          | Exact                        |
| Bar rects, fill + stroke      | `FilledRect` + `Rect`; `Line` for a zero-height bar                | Exact                        |
| `drawImage`                   | `Image` with a `mem:` source                                       | Generated image              |
| Rotated quadratic path        | Flatten once, rotate points on the CPU each frame                  | Emulated rotation            |
| Quadratic path, radial fill   | Triangle fan from the center, `FillTrianglesGradient`              | Emulated path fill           |
| Quadratic path stroke         | `PolylineJoined`                                                   | Exact join type              |

The fan from the center is valid for the flower because the outline's angle
around the center only turns one way. `TestFanCoversFlower` checks this.

## Gaps

These are the go-gui features the port has to emulate. Each one has its own
issue. Each new API needs its own design under the design-before-code rule. This
port adds no API.

1. **Rotation in the canvas transform (#904).** The transform is scale and
   translate only. The example rotates 193 points on the CPU per pass. Text and
   images cannot be rotated at all.
2. **Line caps and round joins (#905).** Canvas strokes have no caps, and joins
   are miter or bevel only. The example draws half discs for the arc caps.
   `gui/svg/stroke.go` already makes butt, round and square caps and miter,
   round and bevel joins, but only for SVG. `gui/svg` imports `gui`, so
   `DrawContext` cannot call it as it is.
3. **A path builder with fill rules (#906).** There is no
   `moveTo`/`lineTo`/`quadTo`/`cubicTo`/`close`. `FilledPolygon` is a fan from
   the first point, so it is correct only for convex shapes. The example builds
   triangles by hand. That works only because both filled shapes here have a
   simple structure. `gui/svg/tessellate_scanline.go` already fills with the
   nonzero and even-odd rules, but only for SVG.
4. **A vsync-off mode (#907).** Every backend presents with vsync, and there is
   no setting to turn it off. FPS is therefore capped at the display refresh
   rate (see Measuring).

Qt also asks for 4× MSAA. That is not a gap: the Metal and GL backends already
render with 4× MSAA where the device supports it (#823).

## Measuring

The example has two modes:

- Interactive: a readout in Qt's format (`Ø 6 s average | 2 s window fps`) plus
  go-gui's mean render-build CPU time per frame.
- `-sweep 1,2,4,…`: each render count runs `-warmup` seconds unmeasured, then
  `-seconds` measured, and prints one CSV row. Columns: `count`, `tests`,
  `frames`, `seconds`, `fps`, `frame_ms`, `view_us`, `layout_us`, `render_us`.

Frames are counted in the view function. The example calls
`w.InvalidateLayout()` there every frame, because go-gui's animation ticker runs
every 16 ms and would cap the count near 62.5 FPS. So each frame also rebuilds
the small control bar. `view_us` and `layout_us` show what that costs.

With vsync on, FPS reads the refresh rate until the frame takes longer than one
refresh. So read the results in two ways:

- **Saturation count**: the largest render count that still holds the refresh
  rate. It is comparable between machines only when the refresh rates are the
  same.
- **CPU cost per pass**: `render_us` divided by the count, at counts where the
  frame is no longer capped. This is go-gui's tessellation and command-building
  cost. GPU and present time is not measured in-process.

A direct FPS comparison with Qt's numbers needs gap 4 first. Single runs are
noisy: take the median of 3 runs for each point.

## First results

Apple M5, built-in 240 Hz display, Metal backend, default tests (31),
`-seconds 3 -warmup 1`, the example's 667×667 window. Each value is the median
of 3 runs. The FPS spread between runs was under 6% up to count 256, and 10% at
count 512.

| Count | FPS   | render_us | µs per pass |
| ----- | ----- | --------- | ----------- |
| 1     | 240.0 | 372       | 372         |
| 4     | 240.0 | 692       | 173         |
| 16    | 240.2 | 1358      | 85          |
| 32    | 192.1 | 2632      | 82          |
| 64    | 100.0 | 5584      | 87          |
| 128   | 47.6  | 11592     | 91          |
| 256   | 22.6  | 24547     | 96          |
| 512   | 10.6  | 50571     | 99          |

What this shows:

- The saturation count is 16 on this machine.
- Once the frame is no longer capped, go-gui spends about 80–100 µs of CPU per
  pass to build the frame. A pass is about 80 strokes and fills, 27 texts and 20
  images.
- At 64 passes and more, render-build is only about half of the frame time (5.6
  ms of 10.0 ms at 64). The other half is outside go-gui's CPU timings: GPU
  work, present, or both. Finding which one needs a GPU-side measurement.
- The per-pass cost at low counts is higher. The work per frame is small there,
  and the CPU likely runs at a lower clock speed. Do not use those rows for the
  per-pass cost.

## Not done

- Phase 4 of the issue, a side-by-side run with QPainter and Qt Canvas Painter
  on the same machine. It is useful only after gap 4.
- Qt's "item count" (several painter items) and "animate item size" options.
