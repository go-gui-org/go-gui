# Animated theme switch

Status: implemented. Issue #753.

## Problem

`SetTheme` and `FollowSystemAppearance` (#752) switched themes in one frame.
There was no way to fade between them. `Theme` is about 12 KB, the view phase
must not allocate per frame, and `Theme.id` keys the install fast path and three
caches, so a blended theme built each frame had to fit those rules.

## Design

Design C from the issue: **only colors fade**. Sizes, radii, fonts and `WithExt`
values take the target's values on the first frame. Blending them would re-run
layout every frame and move content while the fade runs.

- **API.** `Window.SetThemeTransition(d)` is a window setting. Every later
  `SetTheme` and every change `FollowSystemAppearance` applies fades over `d`.
  Zero (the default) keeps the one-frame switch. `Window.Theme` returns the
  target at once; only what is drawn travels.
- **Skipped** when the backend reports `PrefersReducedMotion`, and before the
  window's first frame (nothing on screen to fade from).
- **Blend.** A `reflect` walk of `Theme`, done once, records the byte offset of
  every `Color` stored by value (`themeColorOffsetTable`, `gui/theme_fade.go`).
  A fade frame walks that table with `unsafe` pointer math and lerps each
  channel. A new `Color` field in any style struct joins the table with no code
  change. A color unset on either side takes the target's value.
- **Scratch.** One `themeFade` per window, made on the first fade and reused. It
  holds `from` (a copy of what was on screen), `scratch` (target's non-color
  fields plus blended colors) and a pointer to the published target. The scratch
  is installed with `applyTheme`; it is never published through `w.theme`,
  because `themeRef` promises the pointed-to value is never written through.
- **Theme id.** Each blended frame takes a fresh `nextThemeID()`. The id-keyed
  caches (markdown, RTF layout, list heights) bake colors, so they rebuild each
  fade frame. The cost lasts only as long as the fade. At the end the target is
  installed with its own id.
- **Driver.** A `TweenAnimation` (`gui.theme.fade`, `EaseInOutCubic`) with
  `AnimationRefreshLayout`, so the view regenerates each frame and factory-time
  color reads pick up the blend. Callbacks carry a fade generation; one from a
  replaced or cancelled fade is dropped.
- **Restart.** A theme change during a fade starts from the current blend, so
  nothing jumps. A change with the transition set to zero cancels the fade.
- `FrameBackground` returns the blended background while a fade runs, so the
  clear color travels with the drawn colors.

Cost on the author's machine: 1.3 µs and 0 allocations per fade frame
(`BenchmarkThemeFadeInstall`), plus the view regeneration the tween requests.

## Known limits

- Colors behind a pointer (`*BoxShadow` elevation and focus-ring values) snap
  with the non-color fields.
- `Themed` subtrees install their own theme and do not fade.
- `WithExt` values snap at the start. A `ThemeLerper` contract for ext values is
  additive and can come later.

## Rejected Approaches

- **Cross-fade two rendered frames (A).** Doubles render work for the whole fade
  and ghosts text edges.
- **Blend every `Theme` value (B).** Sizes and fonts blending re-run layout
  every frame for little visible gain.
- **Re-run `ThemeMaker` on a blended `ThemeCfg` each frame.** Allocates (shadow
  clones, maps) and its derivations are not linear.
- **A `go:generate`d blend function.** Works, but adds a generator and a
  generate-check for what an init-time offset table does with no drift.
- **A reserved constant theme id during the fade.** The markdown and RTF caches
  would keep the first frame's colors, then jump at the end.
- **Publishing the scratch through `w.theme`.** Breaks the `themeRef` promise
  that a published value is never written.
- **`SetThemeAnimated(t, d)`.** A per-call API leaves `FollowSystemAppearance`,
  the main trigger, snapping, and would need a second API later.
- **Mandatory `Lerp` on ext values (Flutter model) or `ThemeLerper` now.**
  Mandatory breaks every `WithExt` caller. Optional `Lerp(any) any` boxes a
  value and clones the ext map each frame, which breaks the zero-allocation
  rule.
