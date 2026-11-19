# Subpackage analysis

June 2026. This analysis evaluated splitting large core subsystems out of the
root `gui` package into subpackages (`gui/layout/`, `gui/animation/`, and more).

## Current state

- Root `gui/` package: ~370 non-test .go files at top level
- 51 library packages under `gui/` (36 under `backend/`). 169 total in the
  module including examples and tools
- Compile time: 0.28s — not a problem
- File naming convention: `layout_*.go`, `render_*.go`, `view_*.go`,
  `animation_*.go` — provides grep-level discoverability
- Existing subpackages: `datagrid`, `markdown`, `svg`, `svg/css`, `highlight`,
  `audio`, `shader`, `appinfo`, `instance`, `backend`, `internal`, `locales`,
  `assets` — leaf subsystems that import `gui`. All but one invert the
  dependency back through interfaces, so `gui` never imports them; the exception
  is `gui/appinfo`, which `gui` imports for window metadata
  (`(*Window).AppInfo`)

## Why core subsystems can't move

### Import cycles

Core types are mutually dependent. Examples:

- `Window` embeds animation lifecycle state (`windowAnimation`: the active
  `map[string]Animation`, loop state)
- Animation types (`animation_layout.go`, `animation_tween.go`) reference
  `Window`, `Layout`, `Sizing`
- Layout engine (`layout_arrange.go`) references `Shape`, `Layout`
- Render engine (`render_layout.go`) walks `Layout` trees and reads shape state
  (for example `Opacity`) that animations mutate

Moving `Window` to `gui/window` and animation to `gui/animation` creates:

```
gui/window → gui/animation  (Window embeds animation lifecycle state)
gui/animation → gui/window  (Animation.Update takes *Window)
```

Go forbids import cycles. The single-package design avoids this by design — it
is not an accident, it is the architecture.

### Type coupling

Every subsystem references these types:

| Type     | Referenced by                                         |
| -------- | ----------------------------------------------------- |
| `Layout` | layout, render, animation, all widgets, window, event |
| `Shape`  | layout, render, all widgets, event, a11y              |
| `Window` | all widgets, event, state, animation                  |
| `Sizing` | layout, all container widgets, theme                  |

These form the trunk of the dependency tree. Moving them out does not reduce
coupling. It moves the coupling across package boundaries, which Go does not
allow.

## What could move (but shouldn't)

Standalone leaf types with no internal dependencies:

- `color` (`color.go`, `color_hsv.go`, `color_filter.go`) — only imports
  `fmt`/`math`
- `opt` (`opt.go`) — standalone generic
- `bounded` (`bounded_stack.go`, `bounded_map.go`) — standalone data structures

Moving these adds import verbosity (`gui.Color` → `color.RGBA`) for marginal
benefit. Each is <1K LoC. Not worth the churn.

## The correct pattern

The existing approach is right:

- **Core types stay in `gui/`** — flat, single package. File prefix convention
  for discoverability. No internal import boundaries.
- **Leaf subsystems become subpackages** — `datagrid/`, `markdown/`, `svg/`,
  `highlight/`. They import `gui` and are imported by user code. `gui` does not
  import them back (interface-based inversion where needed), except
  `gui/appinfo` for window metadata.

## Decision

**No subpackage split for core.** Revisit only if:

- Compile times exceed 5s (10× current) AND
- The slow package is `gui/` (not backend or examples) AND
- Go tooling cannot mitigate (build cache, Go workspaces)

The file prefix convention (`layout_*.go`, `render_*.go`, `view_*.go`,
`animation_*.go`) provides sufficient discoverability at current scale.
