# Pixel-level golden images for the software renderer

Issue #361. Status: complete.

## Problem

`gui/golden_test.go` diffs the serialized `[]RenderCmd` stream, which catches
command-level regressions — a wrong color, a missing draw, a moved rect. It
cannot see what only exists after the command stream is rasterized: blend
order, antialiasing, rounded-corner geometry, shadow and blur shapes. Those are
exactly the bugs a CPU rasterizer introduces, so `gui/backend/soft` needed its
own pixel-level gate. `TestPixelGolden` renders text-free views through the
real pipeline via `RenderToImage` and compares the buffer against committed
PNGs in `gui/backend/soft/testdata/`.

## The stability problem, and the text-free gate

Pixel goldens are only useful if the same input produces the same bytes on
every machine that runs the suite. Three drift sources were identified; the
harness eliminates all three at once by construction:

1. **Host font catalog.** The repo bundles no text font, so a family resolves
   through the platform catalog and different machines shape different glyphs.
2. **go-glyph working copy.** The repo's go.work points at a local go-glyph
   checkout; a developer's working copy can shape differently from the version
   CI resolves. A recording made on one can never match on the other.
3. **Arch-dependent antialiasing.** `x/image/vector` and `x/image/draw`
   accumulate coverage with SIMD, which can differ by a bit or two across
   GOARCH (recordings happen on macOS arm64; CI runs amd64 Linux and Windows).

Sources 1 and 2 are text-shaped, so **every golden case must be text-free** and
the harness enforces it: the command stream is checked for any text render kind
(`RenderText`, `RenderLayout`, `RenderRTF`, `RenderLayoutTransformed`,
`RenderTextPath`, `RenderTermGrid`) or `RenderCustomShader` and the case fails
if one appears. Icons are text kinds, so a case cannot cheat with an icon
glyph. No text kinds means go-glyph never runs and sources 1 and 2 do not
exist. Source 3 is absorbed by the tolerance below. A text golden would need a
bundled test font (a repo-size decision) and would re-record on every go-glyph
bump — deferred; the direct pixel assertions in `draw_test.go` /
`draw_phase2_test.go` cover text paths platform-independently.

## Comparison and tolerance

Two numbers, calibrated for the ±1-bit SIMD coverage noise, deliberately
looser than exact bytes:

- `pixelTol = 3` — per-channel (R, G, B, A) delta before a pixel counts as
  differing. A flat-fill color change — the common styling regression — moves
  every pixel of the shape by well over 3 and is caught.
- `maxDiffFraction = 0.5%` — the share of pixels allowed to differ. A 320x240
  frame has a few hundred antialiasing edge pixels; 0.5% (384 px) sits far
  above that and far below a moved or resized shape.

The trade is the issue's own: a small real regression inside the tolerance is
missed. The tolerance is a first guess pending the first cross-arch CI run —
if the suite reds on a runner, the failure stats show the *actual* spread and
the constants are widened from evidence, never blindly.

## Recording flow

```sh
go test ./gui/backend/soft/ -run TestPixelGolden -update
```

`-update` re-encodes every reference from the current renderer output, matching
the command-golden convention (`go test ./gui/ -run TestGolden -update`).
References are committed; a deliberate rendering change re-records after
reviewing the diff, an accidental one reds the suite.

## Failure artifacts

On mismatch the test writes `actual.png`, `expected.png` and `diff.png`
(differing pixels in magenta) under `gui/backend/soft/testdata/failures/` and
reports the differing-pixel count, fraction and max channel delta. The
directory is gitignored; CI uploads it as an artifact when the test job fails,
so a red pixel test is reviewable rather than a log line.

## Case set

Each case is a text-free view built around one claim the command goldens
cannot see (see `golden_cases_test.go`): rounded corners and borders
(antialiased masks), half-alpha compositing (`ColorSwatch`), linear and radial
gradients, gradient borders, circles, drop shadows, SDF blur, the color-filter
bracket, quarter-turn rotation, inline SVG (tessellation + gradient mesh),
scaled memory images, progress fills, and the focus ring. Every case records
in `ThemeDark` and `ThemeLight`, and the window carries no `BgColor` so the
theme's background is part of what is pinned.

The stencil bracket (`ContainerCfg.clipContents`) has no public trigger — the
field is unexported — so it stays covered by `draw_phase2_test.go`'s direct
pixel assertions.

## Related

- #333 — the software rasterizer (phase 1)
- #360 — phase 2 kinds; the reason pixel goldens were worth building
