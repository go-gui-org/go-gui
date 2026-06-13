# Subpackage analysis

June 2026. Evaluated splitting large core subsystems out of the root `gui`
package into subpackages (`gui/layout/`, `gui/animation/`, etc.).

## Current state

- Root `gui/` package: 170+ non-test .go files, ~95K LoC
- 25 total packages (most under `backend/`)
- Compile time: 0.28s — not a problem
- File naming convention: `layout_*.go`, `render_*.go`, `view_*.go`,
  `animation_*.go` — provides grep-level discoverability
- Existing subpackages: `datagrid`, `markdown`, `svg`, `svg/css`, `highlight`,
  `audio`, `shader` — leaf subsystems that import `gui` but aren't imported by it

## Why core subsystems can't move

### Import cycles

Core types are mutually dependent. Examples:

- `Layout` embeds animation state (`AnimationOffset`, `AnimationOpacity`, etc.)
- Animation functions (`animation_layout.go`) reference `Layout`, `Sizing`
- Layout engine (`layout_arrange.go`) references `Shape`, `Layout`
- Render engine (`render_layout.go`) walks `Layout` trees and reads animation state

Moving `Layout` to `gui/layout` and animation to `gui/animation` creates:
```
gui/layout → gui/animation  (Layout references animation state)
gui/animation → gui/layout  (animation functions reference Layout)
```

Go forbids import cycles. The single-package design avoids this by design — it's
not an accident, it's the architecture.

### Type coupling

Every subsystem references these types:

| Type | Referenced by |
|------|--------------|
| `Layout` | layout, render, animation, all widgets, window, event |
| `Shape` | layout, render, all widgets, event, a11y |
| `Window` | all widgets, event, state, animation |
| `Sizing` | layout, all container widgets, theme |

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
