# Spec: ThinkingOrbs-style semantic spinners

Status: draft (go-gui #782) Base: `main` @ `45317c61`

## Motivation

`go-gui` has generic busy affordances only. `SvgSpinner` shows asset loops.
`ProgressBar` shows an indefinite bar. AI and agent interfaces need semantic
states. "Searching" must not look the same as "writing".

Upstream proves the design space. Jakub Antalik made nine dotted 3D designs
(MIT). Haplo LLC ported them to SwiftUI (MIT). This spec ports them to `go-gui`.

## Behavior

### API

```go
gui.ThinkingOrb(gui.ThinkingOrbCfg{
    ID:     "search-status",
    Design: gui.ThinkingOrbSearching,
    Size:   gui.ThinkingOrbRegular,
    Speed:  1.0,
    Paused: false,
})
gui.ThinkingOrbLabel(gui.ThinkingOrbLabelCfg{
    ID:     "search-label",
    Text:   "Searching the web…",
    Design: gui.ThinkingOrbSearching,
})
```

`ThinkingOrbCfg` is zero-initializable. The zero `Design` is
`ThinkingOrbWorking` (safe general busy). The zero `Size` is
`ThinkingOrbRegular`. A zero `Speed` means 1.0 (sanitize like `MathSpinner`).
`Paused` freezes the orb on the current frame.

`Color` is a plain `Color`. Unset takes the theme text color. A set color wins
over the theme. `A11YLabel` and `A11YDescription` come from the embedded
`A11YCfg`. The Cfg never redeclares them. Callbacks use `func(EventCtx)`. The
orb is not focusable by default. It is status, not a control.

The label helper uses a Cfg struct (4b-lite). It carries `Text`, `Design`,
`Size`, `Speed`, `Paused`, `TextStyle`, `Color`, `Padding`, `Sizing`, and
`A11YCfg`. It has no `OnClick` and no focus. It reads as one accessibility
element (its title). The shimmer stays private. Only the label helper uses it.

### Designs

Nine cases, one per agent state:

| Design     | Reach for it when               |
| ---------- | ------------------------------- |
| Working    | general-purpose busy            |
| Searching  | web search, retrieval, lookups  |
| Solving    | reasoning, math, code           |
| Listening  | voice input, transcription      |
| Connecting | tool calls, APIs, sync          |
| Weaving    | planning, multi-step agents     |
| Composing  | writing a reply                 |
| Breathing  | idle thinking, waiting on model |
| Shaping    | design, image, layout work      |

An invalid `Design` clamps to `Working`.

### Sizes

Two tuned sizes, not one scaled design. `ThinkingOrbSmall` (20pt, inline text
and rows) has fewer and bigger dots with its own tempo. `ThinkingOrbRegular`
(64pt, hero and avatar) is the default. Arbitrary diameters stay out of scope
for v1.

### Animation clock (decision 2a)

The widget follows the `ProgressBar` and `MathSpinner` pattern. `AmendLayout`
drives a repeating `KeyframeAnimation`. The animation key is the effective ID
(`Shape.idKey`). Progress lives in the window `StateMap`. The child `DrawCanvas`
carries `Version` from the progress bits, so each tick redraws the canvas.

Multiple orbs on one screen stay in phase. They share the same clock source.

Offscreen parking for v1 is free (2a). `renderDrawCanvas` already skips shapes
outside the clip, so an offscreen orb emits no GPU work. The animation tick
still wakes the main thread. A visibility gate that skips the heartbeat is a
later optimization, not v1 work. View-bound auto-cancel still applies. An orb
that leaves the tree stops within 2s (`animViewBoundStale` in
`gui/animation_loop.go`).

Speed and pause parameters are sampled at first render. A change after the
widget is visible has no effect. Use a different widget ID to apply new
parameters. This matches the `MathSpinner` rule.

### Theme ink

Ink is monochrome and theme-aware. Dark dots sit on light schemes. Light dots
sit on dark schemes. Depth shading mirrors so near dots read strongest. The
widget reads the theme at generation time. A caller `Color` wins over the theme
default.

### Accessibility

Each design has a default VoiceOver label (for example "Searching…"). A caller
label overrides it. The label widget reads as one element. With Reduce Motion
on, the orb shows a single representative frame and the shimmer holds still.
With Increase Contrast on, shimmer text stays at full strength. The widget reads
`w.prefersReducedMotion()`. Headless captures (`SetHeadlessRender`) also pin a
still frame.

### Performance budget

One draw list per orb. No view per dot. Dots draw as filled circles, edges as
lines, far to near. Upstream heaviest frame (composing, 566 dots) costs ~65µs
compute + 0.27ms rasterize at 120Hz. The port keeps heap allocations flat across
frames.

## Engine (decision 1: Swift as reference)

The engine is a pure function:

```go
orbFrame(design ThinkingOrbDesign, size ThinkingOrbSize, t float64) (dots []orbDot, lines []orbLine)
```

`t` is seconds at normal speed. The design tuned speed applies inside. Output
values are final. Array order is draw order (lines first, then dots far to
near).

Port the formulas from the Haplo Swift package. Use the original TypeScript
engine as arbiter on disagreement. Keep evaluation order and rounding of the
reference. Do not reinterpret the math.

License: carry both MIT copyrights (Antalik + Haplo), as upstream does.

## Test plan (decision 3: both levels)

- Vector parity (3a): unit tests check every dot and line for all nine designs ×
  two sizes × fixed instants against reference vectors to 1e-4. Paused frames
  give the fixed `t`.
- Golden RenderCmd (3b): `gui/golden_test.go` cases in both `ThemeDark` and
  `ThemeLight`. The harness pins the virtual clock, so live orbs record a fixed
  frame.
- Behavior tests: paused freezes, Reduce Motion pins a still frame, invalid
  design clamps, Speed ≤ 0 means 1.0.
- Identity tests: no duplicate effective IDs, two orbs with the same leaf under
  different scopes keep separate state.
- Gates: `make prepush` green, `make export-audit` clean for the new exports.

## Showcase

Gallery row of all nine designs (both sizes). One inline-small usage in a list
row. One toolbar usage of the label helper. Follow `demo_svg_spinner.go` (grid
of cells + names).

## Rejected Approaches

- **Baked SVG assets, no engine port.** Pre-rendered SMIL loops cannot express
  per-frame 3D depth shading for 566 dots. Speed and pause exactness is lost.
  The parity requirement fails.
- **RenderOnly wall-clock orb as v1.** `AlwaysRedraw` +
  `AnimationRefreshRenderOnly` skips layout rebuilds and is the better long-term
  shape. It is the wrong first shape because wall-clock `OnDraw` breaks
  deterministic goldens and its offscreen semantics are unproven. It returns as
  a perf follow-up behind the same Cfg.
- **Public shimmer modifier.** The issue scopes shimmer to what the label helper
  needs. A public text modifier grows API surface with no caller yet.
- **Arbitrary diameter knob.** Upstream supports it, but the two tuned sizes
  cover v1 callers. Defer unless cheap.
- **Factory taking `w *Window`.** `MathSpinner` reads window state in the
  factory body, which joins the wrong scope. New code resolves identity inside
  `GenerateLayout` or `AmendLayout` via `ctx.Window`.
- **Minimal-func label.** `ThinkingOrbLabel(text, design)` matches Swift but
  breaks at the first styling caller (toolbar font, padding, a11y override). The
  Cfg struct extends without breakage.
