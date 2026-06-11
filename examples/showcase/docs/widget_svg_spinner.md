One hundred six built-in animated SVG spinners drawn from
the open-source svg-spinners collection plus extended SMIL- and
CSS-driven assets. Each kind is embedded as raw SVG and rendered
through the same pipeline as `gui.Svg`, so assets recolor via
`fill=\"currentColor\"` and scale cleanly at any size. Identify a
spinner by its `SvgSpinnerKind` constant; call `SvgSpinnerCount`
or `SvgSpinnerName` to enumerate or label them.

## Usage

```go
gui.SvgSpinner(gui.SvgSpinnerCfg{
    ID:   "loading",
    Kind: gui.SvgSpinnerTailSpin,
})
```

## With Color

```go
gui.SvgSpinner(gui.SvgSpinnerCfg{
    ID:    "save",
    Kind:  gui.SvgSpinner3DotsBounce,
    Color: gui.RGB(46, 160, 67),
})
```

## Explicit Size

```go
gui.SvgSpinner(gui.SvgSpinnerCfg{
    ID:     "hero",
    Kind:   gui.SvgSpinner90Ring,
    Width:  96,
    Height: 96,
})
```

When both `Width` and `Height` are zero, the spinner
defaults to 48x48. Setting only one axis leaves the other at zero so
the containing layout or aspect ratio of the asset controls it.

## Enumerating Kinds

```go
for i := range gui.SvgSpinnerCount() {
    k := gui.SvgSpinnerKind(i)
    fmt.Println(gui.SvgSpinnerName(k)) // e.g. "90-ring"
}
```

## Key Properties

| Property  | Type           | Description                           |
|-----------|----------------|---------------------------------------|
| ID        | string         | Unique identifier (required)          |
| Kind      | SvgSpinnerKind | One of 106 built-in spinners          |
| Color     | Color          | Recolor monochrome assets             |
| Width     | float32        | Explicit width (default 48 if both 0) |
| Height    | float32        | Explicit height (default 48 if both 0)|
| Sizing    | Sizing         | Combined axis sizing mode             |
| Padding   | Opt[Padding]   | Outer padding                         |
| MinWidth  | float32        | Minimum width                         |
| MaxWidth  | float32        | Maximum width                         |
| MinHeight | float32        | Minimum height                        |
| MaxHeight | float32        | Maximum height                        |
| OnClick   | func           | Click handler (optional)              |

## Accessibility

| Property        | Type   | Description                                      |
|-----------------|--------|--------------------------------------------------|
| A11YLabel       | string | Short name announced by screen readers           |
| A11YDescription | string | Longer description (e.g. "loading, please wait") |

## Helpers

| Function          | Description                                   |
|-------------------|-----------------------------------------------|
| SvgSpinnerCount() | Returns the number of built-in kinds (106)    |
| SvgSpinnerName(k) | Returns the asset basename (e.g. "tail-spin") |

## Animation Pipeline

SMIL features:

- `<animate>`, `<animateTransform>` (rotate, translate, scale,
  TRS sandwich), `<animateMotion>` with inline path or
  `<mpath>` reference plus `rotate=\"auto\"`
- opacity, fill-opacity, stroke-opacity, per-role opacity
- primitive attribute animation (cx/cy/r/x/y/width/height/rx/ry)
- dash animations (stroke-dasharray, stroke-dashoffset)
- keyTimes, keySplines spline easing, calcMode=discrete
- syncbase begin chains, additive/accumulate="sum", from/to/by
- min/max dur clamps, restart="never"/"whenNotActive"
- `<set>` elements (freeze semantics by default)

CSS features:

- selector cascade (id, class, type), presentation-attribute fallback
- `@keyframes` with percentage stops, `animation` shorthand
- `animation-iteration-count` (including `infinite`),
  `animation-timing-function` (`steps()`, cubic-bezier),
  delays, durations
- multi-animation per element, `@media` query gating
- `display:none` and mixed SMIL+CSS animation on the same shape

## Kinds: Rings & Circles

| Constant                     | Asset                 |
|------------------------------|-----------------------|
| SvgSpinner90Ring             | 90-ring               |
| SvgSpinner90RingWithBg       | 90-ring-with-bg       |
| SvgSpinner90RingWithGradient | 90-ring-with-gradient |
| SvgSpinner180Ring            | 180-ring              |
| SvgSpinner180RingWithBg      | 180-ring-with-bg      |
| SvgSpinner270Ring            | 270-ring              |
| SvgSpinner270RingWithBg      | 270-ring-with-bg      |
| SvgSpinnerCircles            | circles               |
| SvgSpinnerEclipse            | eclipse               |
| SvgSpinnerEclipseHalf        | eclipse-half          |
| SvgSpinnerLoader2            | loader2               |
| SvgSpinnerOval               | oval                  |
| SvgSpinnerRingResize         | ring-resize           |
| SvgSpinnerRings              | rings                 |
| SvgSpinnerSpinner            | spinner               |
| SvgSpinnerSpinnerDouble      | spinner-double        |
| SvgSpinnerSpinnerMultiple    | spinner-multiple      |
| SvgSpinnerSpinnerMultiple2   | spinner-multiple-2    |
| SvgSpinnerSpinningCircles    | spinning-circles      |
| SvgSpinnerTailSpin           | tail-spin             |

## Kinds: Dots

| Constant                    | Asset                |
|-----------------------------|----------------------|
| SvgSpinner12DotsScaleRotate | 12-dots-scale-rotate |
| SvgSpinner3DotsBounce       | 3-dots-bounce        |
| SvgSpinner3DotsFade         | 3-dots-fade          |
| SvgSpinner3DotsMove         | 3-dots-move          |
| SvgSpinner3DotsRotate       | 3-dots-rotate        |
| SvgSpinner3DotsScale        | 3-dots-scale         |
| SvgSpinner3DotsScaleMiddle  | 3-dots-scale-middle  |
| SvgSpinner4DotsGoeey        | 4-dots-goeey         |
| SvgSpinner4DotsRotate       | 4-dots-rotate        |
| SvgSpinner6DotsRotate       | 6-dots-rotate        |
| SvgSpinner6DotsScale        | 6-dots-scale         |
| SvgSpinner6DotsScaleMiddle  | 6-dots-scale-middle  |
| SvgSpinner8DotsRotate       | 8-dots-rotate        |
| SvgSpinnerBallTriangle      | ball-triangle        |
| SvgSpinnerDotRevolve        | dot-revolve          |

## Kinds: Bars

| Constant                  | Asset             |
|---------------------------|-------------------|
| SvgSpinnerBars            | bars              |
| SvgSpinnerBarsFade        | bars-fade         |
| SvgSpinnerBarsRotateFade  | bars-rotate-fade  |
| SvgSpinnerBarsScale       | bars-scale        |
| SvgSpinnerBarsScaleFade   | bars-scale-fade   |
| SvgSpinnerBarsScaleMiddle | bars-scale-middle |
| SvgSpinnerHorizontalBar   | horizontal-bar    |

## Kinds: Loaders

| Constant            | Asset       |
|---------------------|-------------|
| SvgSpinnerLoader1   | loader1     |
| SvgSpinnerLoader3   | loader3     |
| SvgSpinnerLoader4   | loader4     |
| SvgSpinnerLoader5   | loader5     |
| SvgSpinnerLoader6   | loader6     |
| SvgSpinnerLoader7   | loader7     |
| SvgSpinnerLoader8   | loader8     |
| SvgSpinnerLoader9   | loader9     |
| SvgSpinnerLoader10  | loader10    |
| SvgSpinnerLoaderWifi| loader-wifi |

## Kinds: Blocks

| Constant                 | Asset            |
|--------------------------|------------------|
| SvgSpinnerBlocksScale    | blocks-scale     |
| SvgSpinnerBlocksShuffle2 | blocks-shuffle-2 |
| SvgSpinnerBlocksShuffle3 | blocks-shuffle-3 |
| SvgSpinnerBlocksShuffle4 | blocks-shuffle-4 |
| SvgSpinnerBlocksShuffle5 | blocks-shuffle-5 |
| SvgSpinnerBlocksWave     | blocks-wave      |

## Kinds: Pulse

| Constant                     | Asset                |
|------------------------------|----------------------|
| SvgSpinnerGooeyBalls1        | gooey-balls-1        |
| SvgSpinnerGooeyBalls2        | gooey-balls-2        |
| SvgSpinnerHeartPulse         | heart-pulse          |
| SvgSpinnerHeartPulse2        | heart-pulse-2        |
| SvgSpinnerHeartPulse3        | heart-pulse-3        |
| SvgSpinnerPuff               | puff                 |
| SvgSpinnerPulse              | pulse                |
| SvgSpinnerPulse2             | pulse2               |
| SvgSpinnerPulse3             | pulse-3              |
| SvgSpinnerPulseMultiple      | pulse-multiple       |
| SvgSpinnerPulseRing          | pulse-ring           |
| SvgSpinnerPulseRings2        | pulse-rings-2        |
| SvgSpinnerPulseRings3        | pulse-rings-3        |
| SvgSpinnerPulseRingsMultiple | pulse-rings-multiple |

## Kinds: Cogs

| Constant        | Asset |
|-----------------|-------|
| SvgSpinnerCog01 | cog01 |
| SvgSpinnerCog02 | cog02 |
| SvgSpinnerCog03 | cog03 |
| SvgSpinnerCog04 | cog04 |
| SvgSpinnerCog05 | cog05 |
| SvgSpinnerCog06 | cog06 |
| SvgSpinnerCog07 | cog07 |
| SvgSpinnerCog08 | cog08 |
| SvgSpinnerCog09 | cog09 |
| SvgSpinnerCog10 | cog10 |
| SvgSpinnerCog11 | cog11 |
| SvgSpinnerCog12 | cog12 |
| SvgSpinnerCog13 | cog13 |
| SvgSpinnerCog14 | cog14 |
| SvgSpinnerCog15 | cog15 |
| SvgSpinnerCog16 | cog16 |
| SvgSpinnerCog17 | cog17 |
| SvgSpinnerCog18 | cog18 |
| SvgSpinnerCog19 | cog19 |
| SvgSpinnerCog20 | cog20 |
| SvgSpinnerCog21 | cog21 |
| SvgSpinnerCog22 | cog22 |
| SvgSpinnerCog23 | cog23 |
| SvgSpinnerCog24 | cog24 |

## Kinds: Miscellaneous

| Constant               | Asset         |
|------------------------|---------------|
| SvgSpinnerAudio        | audio         |
| SvgSpinnerBouncingBall | bouncing-ball |
| SvgSpinnerCircleFade   | circle-fade   |
| SvgSpinnerClock        | clock         |
| SvgSpinnerGrid         | grid          |
| SvgSpinnerHearts       | hearts        |
| SvgSpinnerTadpole      | tadpole       |
| SvgSpinnerWifi         | wifi          |
| SvgSpinnerWifiFade     | wifi-fade     |
| SvgSpinnerWindToy      | wind-toy      |

## Caveats

- Assets relying on SMIL or CSS features outside the supported subset
  may render as their static first frame.
- `<set>` defaults to freeze semantics (SMIL default is remove);
  authors who want remove must set `fill=\"remove\"` explicitly.
- `SvgSpinnerCount` and enum values are generated from the embedded
  asset directory, so numeric values shift when assets are added or
  removed — always reference kinds by their named constant.
