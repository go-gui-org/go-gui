# Spec: Cut per-frame widget-factory heap allocations

Status: reviewed 2026-07-18 (claims verified against code; Circle migration,
benchmark correction, cached-view tradeoff added)
Area: performance / GC-allocation reduction
Scope: `gui/` (examples excluded)

## Context

go-gui's per-frame pipeline (`view → generateViewLayout → layoutArrange →
renderLayout`) is already arena/pool-backed and ~0-alloc once warm (see
`gui/scratch_pools.go`). A survey of six candidate areas found the **only
genuinely un-pooled per-frame heap traffic** lives in the widget-**factory**
layer: `w.viewGenerator(w)` re-runs the user's whole builder every full-refresh
frame (any input event triggers this via `UpdateWindow`), and the factories
allocate objects the downstream pools never see.

Root cause: factories (`container`, `Row`, `Column`, `Button`, …) run **without a
`*Window`**, so they cannot reach the pools (`allocShape`, `allocEventHandlers`,
`buttonColors`) — those are only usable in `GenerateLayout`, which has `w`.

### Current per-`container()` allocations, per frame

Traced in `gui/view_container.go`:

1. `&containerView{}` — the `View` interface box (`container` :398). **Unavoidable**
   without a factory API change (interface boxing of a large struct).
2. `buildContainerShape(&cfg)` → heap `&Shape{}` (:323). **Pure waste** —
   `containerView.GenerateLayout` immediately copies it *by value* into a pooled
   shape: `w.allocShape(*cv.shape)` (:182), then discards the template.
3. `makeContainerEffects` → `&shapeEffects{}` when effects set (:263).
4. `makeContainerEvents` → `&eventHandlers{}` when any handler set (:282). Buttons
   always hit this (`ClickOnSpace/Enter` + `OnClick`).
5. `makeContainerA11Y` → `*AccessInfo` when label/desc set (:301).

`Button` adds a second box: `&buttonView{cv: *containerView}` (`view_button.go`
:206) wrapping the container — so a button is ~2 boxes + template shape +
eventHandlers per frame, plus its `Text` child. Downstream pools (`viewShapes`,
`viewEvents`, `buttonColors`) recycle only the *layout-phase copies*, not these
factory-phase originals.

## Goal

Route allocations #2–#5 through the existing per-frame pools by **deferring shape
construction from the factory into `GenerateLayout`** (where `w` exists). Expected:
a container drops from ~2–4 allocs/frame to **1** (the unavoidable box); a button
from ~4 to ~2. On a widget-dense frame (hundreds of nodes) this is a large drop in
GC object count per frame.

Non-goals: changing the public factory signatures (`Row`/`Column`/`Button` stay
`func(cfg) View`); reworking the layout/render passes (already 0-alloc); the
per-frame `make([]View, …)` in `container()`'s Scrollable branch (:390) — an
un-pooled factory alloc, but scrollable containers are rare per frame; follow-up
if it shows in profiles.

## Approach

Precedent: this is already the codebase's dominant widget pattern. `view_svg.go`
:142-148, `view_image.go` :93-100, `view_rtf.go` :142-162, `view_draw_canvas.go`
:59-82, and `termgrid.go` :138-168 all build their `Shape` inside `GenerateLayout`
via `w.allocShape` + `w.allocEventHandlers`. This spec converts containers (and
buttons) to the same deferred-build shape-in-`GenerateLayout` pattern.

### 0. Create a feature branch
Branch off the current base (`main`) before any edits:
`git switch main && git switch -c perf-widget-factory-pools` (confirm `main` is up
to date first; the working tree is clean). All work lands on this branch.

### 1. Add an effects pool (mirror the existing events pool)
- `gui/scratch_pools.go`: add `viewEffects scratchObjPool[shapeEffects]` to
  `scratchPools` (next to `viewEvents` at :136), init in `newScratchPools` (mirror
  `viewEvents` caps ~ `{retainMax: 4096, shrinkTo: 256}`), and reset in
  `resetViewPools` (:204).
- `gui/window.go`: add `allocEffects(src shapeEffects) *shapeEffects` mirroring
  `allocEventHandlers` (:459) — pooled, nil-`w` heap fallback for tests.

### 2. Defer container shape construction into `GenerateLayout`
`gui/view_container.go`:
- Change `containerView` (:170) to hold the **resolved `ContainerCfg` by value**
  (plus already-resolved `content []View`) instead of the pre-built `shape *Shape`.
  cfg carries title/titleBG/colorBorder/disabled, so drop those duplicate fields.
- `container()` (:377) keeps its cheap cfg resolution (OnAnyClick→OnClick,
  ClickButton default, Scrollable content expansion) but stops calling
  `buildContainerShape`; it stores the resolved cfg: `&containerView{cfg: cfg,
  content: content}`. Still exactly **one** heap alloc (the box).
- Convert `buildContainerShape(cfg *ContainerCfg) *Shape` into a **value-returning,
  `w`-aware** builder, e.g. `buildContainerShape(cfg *ContainerCfg, w *Window)
  Shape`, that:
  - returns a `Shape` value (no `&Shape{}`),
  - sets `events` from `w.allocEventHandlers(...)` — nil when no handlers,
  - sets `fx` from `w.allocEffects(...)` — nil when no effects,
  - `A11Y` unchanged (user pointer or `makeA11YInfo`; pooling it is a stretch goal).
- `containerView.GenerateLayout` (:181) becomes:
  `layout := Layout{Shape: w.allocShape(buildContainerShape(&cv.cfg, w))}` then the
  existing `addGroupBoxTitle` call reading fields off `cv.cfg`.
- **`Circle()` (:434-439) mutates the pre-built shape**
  (`cv.shape.shapeType = shapeCircle`) — impossible once the shape is deferred.
  Add an internal `shapeType shapeType` field to `ContainerCfg` (next to `axis`,
  :147; zero value = `shapeRectangle` path preserved in `buildContainerShape`),
  set by `Circle` before calling `container()`. Without this, step 2 does not
  compile.
- **Rewrite the `containerView` doc comment (:159-169)** — it documents the
  pre-built-shape design ("Shape is pre-built at factory time…", shallow-copy
  rationale) and becomes false after this change. New comment: cfg held by value,
  shape built per `GenerateLayout` call from pooled allocs; cached views re-run
  the build each frame (see Risks).

`makeContainerEvents`/`makeContainerEffects` stay as pure field-mappers but their
results feed the pool allocators rather than heap-escaping; keep their nil
fast-paths (no handlers/effects → nil pointer, zero alloc).

### 3. Migrate the three direct callers
`gui/view_select.go:202`, `gui/view_color_picker.go:107`,
`gui/view_combobox.go:289` all use the identical pattern
`&containerView{shape: buildContainerShape(&ccfg), content: content}` with **no
post-build shape mutation** (verified). Replace each with the deferred form
`&containerView{cfg: ccfg, content: content}`. These sites already run inside a
`GenerateLayout` with `w` in scope, so they stay correct.

`invisibleContainerView()` (:457) becomes fully stateless under cfg-by-value
(no mutable template shape; `GenerateLayout` copies into pooled shapes), so
replace the per-call construction with a package-level singleton
`var invisibleView = &containerView{cfg: …}` — deletes that box alloc too. Safe
because nothing mutates a `containerView` after construction once `Circle` moves
its `shapeType` into cfg (step 2); an invisible "circle" losing its circle
shapeType is moot (placeholder, never drawn as either).

### 4. (Same-PR, high value) collapse `buttonView` into the container path
`buttonView` (`view_button.go:79`) is a *second* box per button. Fold its six
button-color/hover fields into `containerView` (as optional fields set only by
`Button`), moving the `shapeButtonColors` pooling (:103) into
`containerView.GenerateLayout`. Removes one alloc per button per frame — buttons
are the highest-frequency interactive widget. If this entangles button focus/hover
logic, split it into a follow-up PR and land steps 1–3 first.

Details:
- `buttonView.GenerateLayout` gates the color attach on
  `layout.Shape.events != nil` (:93); once folded, every container with handlers
  would match. Add an `isButton bool` discriminator on `containerView`, set by
  `Button`.
- Cost: every `containerView` grows ~80 B (4 `Color` + 2 func ptrs) whether or
  not it is a button. Still a net win (removes a whole heap object per button);
  acknowledged under Struct growth below.
- Bonus correctness: today `buttonView.GenerateLayout` mutates
  `layout.Shape.events.AmendLayout/OnHover` (:107-108) through a pointer *shared
  with the factory template* (`allocShape` shallow-copies the Shape; `events` is
  the same heap object every frame). Idempotent today, but fragile. Per-frame
  pooled `eventHandlers` gives each frame its own copy and removes the
  cross-frame sharing.

## Files
- `gui/scratch_pools.go` — add `viewEffects` pool + reset.
- `gui/window.go` — add `allocEffects`.
- `gui/view_container.go` — core refactor (`containerView`, `container`,
  `buildContainerShape`, `GenerateLayout`, `Circle`, `invisibleContainerView`,
  rewrite :159-169 comment).
- `gui/view_select.go`, `gui/view_color_picker.go`, `gui/view_combobox.go` —
  migrate direct callers.
- `gui/view_button.go` — step 4 (buttonView fold-in).

## Verification
- **`BenchmarkViewFrame` (`gui/view_frame_bench_test.go`) already exercises the
  factory phase** — it builds the tree through public `Row`/`Column` factories
  inside `b.Loop()` (deliberately: "Shapes are allocated inside the loop") and
  resets view pools per iter. It measures the container win directly; capture it
  before/after. (`BenchmarkGenerateViewLayout` does *not* — it pre-builds a
  custom `benchView` outside the loop.)
- Add a `-benchmem` benchmark for the **uncovered widgets**: `Button` (2-box +
  buttonColors path) and containers with effects/events set (exercises the new
  `viewEffects` pool). Model on `BenchmarkViewFrame` / `list_core_alloc_bench_test.go`
  (resetViewPools per iter).
- Capture before/after `allocs/op` + `B/op` via
  `go test ./gui/ -bench='ViewFrame|Factory' -benchmem -count=5` + benchstat.
  Target: measured drop in allocs/op for container- and button-heavy trees. allocs
  are the hard gate (per CLAUDE.md); ns/op advisory — expect a small ns/op *rise*
  for cached views (see Risks) against a large alloc drop for rebuilt trees.
- `go test ./...` (headless, ~12s) and `golangci-lint run ./...` must stay green.
  Watch the `no covering tests` widgets (containerView, buttonView, select,
  color_picker) — add targeted assertions that `GenerateLayout` still produces the
  correct Shape (events/fx present when configured, nil otherwise) and that pooled
  pointers are non-nil.
- Run a widget-dense example (e.g. `go run ./examples/showcase/`) to confirm no
  visual/behavioral regression in containers, buttons, select, combobox, color
  picker, group-box titles.

## Risks / open questions
- **Struct growth**: storing `ContainerCfg` by value enlarges `containerView`. Net
  GC win still holds (1 larger object beats 3–5 small ones), but confirm the single
  box does not itself regress `B/op` beyond the alloc-count savings.
- **Cached views re-pay the cfg→Shape build**: the current design pre-builds the
  shape precisely so cached views (combobox/command-palette dropdown caches, which
  retain the View and re-call `GenerateLayout` each frame) pay only a Shape copy
  per frame (`view_container.go` :159-169 comment). Deferred build trades that for
  a full ~70-field `buildContainerShape` per frame — pure CPU, zero allocs.
  Accepted: alloc count is the stated bottleneck (CLAUDE.md perf baseline), and
  the field-copy cost is in the same ballpark as the Shape copy it replaces.
  Verify with `BenchmarkViewFrame` ns/op (advisory).
- **Per-frame defaults/validation**: `applyContainerDefaults` and
  `RequireScrollID` move from factory time into `GenerateLayout`. Behavior change:
  `DefaultContainerStyle` edits now take effect on cached views next frame
  (arguably a fix); the missing-scroll-ID panic fires at layout instead of build —
  same frame, same message.
- **Pool lifetime**: `fx`/`events` pooled in the view phase are read during render;
  pools reset at frame *start* (`resetViewPools`) and are valid through
  `buildRenderers` — the same guarantee `viewShapes`/`viewEvents` already rely on.
  Safe, but assert it in a test.
- **Step 4 scope**: fold buttonView in the same PR only if it stays clean;
  otherwise defer to a follow-up.
- Pool `AccessInfo` (`makeA11YInfo`) too? Left as a stretch goal — usually nil.

## Rejected alternatives
- **Pool the `containerView`/`buttonView` boxes themselves** via a package-level
  `sync.Pool`: factories lack `w`, and reclaiming the boxes safely requires knowing
  when `generateViewLayout` has consumed them. Higher complexity and `sync.Pool`
  overhead for a single-alloc saving. Not pursued.
- **Pass `*Window` into factories**: eliminates the boxing but breaks the entire
  public widget API (`Row`/`Column`/`Button` signatures). Out of scope.
