# Subpackage analysis

June 2026. Evaluated splitting large core subsystems out of the root `gui`
package into subpackages (`gui/layout/`, `gui/animation/`, etc.).

## Current state

- Root `gui/` package: ~200 non-test .go files at top level (~400 including tests)
- 25 library packages under `gui/` (16 under `backend/`); 82 total in the
  module including examples and tools
- Compile time: 0.28s — not a problem
- File naming convention: `layout_*.go`, `render_*.go`, `view_*.go`,
  `animation_*.go` — provides grep-level discoverability
- Existing subpackages: `datagrid`, `markdown`, `svg`, `svg/css`, `highlight`,
  `audio`, `shader` — leaf subsystems that import `gui` but aren't imported by it

## Why core subsystems can't move

### Import cycles

Core types are mutually dependent. Examples:

- `Window` embeds animation lifecycle state (`windowAnimation`: the active
  `map[string]Animation`, loop state)
- Animation types (`animation_layout.go`, `animation_tween.go`) reference
  `Window`, `Layout`, `Sizing`
- Layout engine (`layout_arrange.go`) references `Shape`, `Layout`
- Render engine (`render_layout.go`) walks `Layout` trees and reads shape
  state (e.g. `Opacity`) that animations mutate

Moving `Window` to `gui/window` and animation to `gui/animation` creates:

```
gui/window → gui/animation  (Window embeds animation lifecycle state)
gui/animation → gui/window  (Animation.Update takes *Window)
```

Go forbids import cycles. The single-package design avoids this by design — it's
not an accident, it's the architecture.

### Type coupling

Every subsystem references these types:

| Type     | Referenced by                                         |
| -------- | ----------------------------------------------------- |
| `Layout` | layout, render, animation, all widgets, window, event |
| `Shape`  | layout, render, all widgets, event, a11y              |
| `Window` | all widgets, event, state, animation                  |
| `Sizing` | layout, all container widgets, theme                  |

These form the trunk of the dependency tree. Moving them out doesn't reduce
coupling — it just moves the coupling across package boundaries, which Go
doesn't allow.

## What could move (but shouldn't)

Standalone leaf types with no internal dependencies:

- `color` (`color.go`, `color_hsv.go`, `color_filter.go`) — only imports `fmt`/`math`
- `opt` (`opt.go`) — standalone generic
- `bounded` (`bounded_stack.go`, `bounded_map.go`) — standalone data structures

Moving these adds import verbosity (`gui.Color` → `color.RGBA`) for marginal
benefit. Each is <1K LoC. Not worth the churn.

## The correct pattern

The existing approach is right:

- **Core types stay in `gui/`** — flat, single package. File prefix convention
  for discoverability. No internal import boundaries.
- **Leaf subsystems become subpackages** — `datagrid/`, `markdown/`, `svg/`,
  `highlight/`. They import `gui` and are imported by user code. `gui` never
  imports them (interface-based inversion where needed).

## Decision

**No subpackage split for core.** Revisit only if:

- Compile times exceed 5s (10× current) AND
- The slow package is `gui/` (not backend or examples) AND
- Go tooling can't mitigate (build cache, Go workspaces)

The file prefix convention (`layout_*.go`, `render_*.go`, `view_*.go`,
`animation_*.go`) provides sufficient discoverability at current scale.
