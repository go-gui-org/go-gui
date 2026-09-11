# Window refresh entry point naming

Status: implemented. Issue #563.

## Problem

`(*Window).UpdateWindow` did no work. It set `refreshLayout` and woke the
backend's idle loop. `(*Window).Update` is the method that rebuilds. Three
problems followed from the name.

**The name meant the opposite elsewhere.** Win32 `UpdateWindow()` forces an
immediate paint, and `InvalidateRect` is the flag-setter. A reader with that
background expects a synchronous repaint from a method that only schedules one.

**The pair was inconsistent.** The render-only sibling was `RequestRedraw`,
which is request-shaped. The two entry points do the same kind of thing — set a
staleness flag, wake the loop — and read as different kinds of thing.

**It stuttered.** `w.UpdateWindow()` on a `*Window`.

## Decision

| Old             | New                |
| --------------- | ------------------ |
| `UpdateWindow`  | `InvalidateLayout` |
| `RequestRedraw` | `InvalidateRender` |

`Invalidate` was already the repo's verb for this idea: `InvalidateListHeights`
(`gui/list_height_registry.go`) means "mark stale, rebuild later". Reusing it
keeps one vocabulary instead of importing a fourth. The suffix names what went
stale — `InvalidateLayout` rebuilds the layout tree, `InvalidateRender` rebuilds
only the render commands from the tree already arranged.

Both old names stay as one-line forwarders with a Go `// Deprecated:` line, per
the entry-points-only deprecation rule, so the sibling repos keep compiling
across a release. Every in-repo caller moved, so no first-party code trips
SA1019. The forwarders carry `//exportaudit:keep`, because nothing outside
`gui/` references them once the examples migrate.

Folded in: `requestRenderOnly` was a one-line wrapper over
`markRenderOnlyRefresh` with a single caller (`gui/view_svg.go`), so it is gone.
Its doc claimed `RequestRedraw` was "an alias for RequestRenderOnly", a method
that never existed exported.

## Rejected Approaches

**`Request*` (`RequestLayout`, `RequestRedraw`)** — the shape the code started
with on the render side. Reads awkwardly and does not say what went stale.

**`Post*` (`PostLayout`, `PostUpdate`)** — "post" means enqueue a message
(`PostMessage`, `postDelayed`), so each call implies an added item. Nothing is
enqueued here: the flag is an idempotent bool, and ten calls in one frame
collapse to one rebuild. `PostLayout` also collides with the `Layout` type.

**`SetNeeds*` (`SetNeedsLayout`)** — the UIKit idiom, unmistakably a flag
setter, but foreign in Go and with no in-repo precedent to build on.

**`Refresh*` (`RefreshLayout`)** — matches the internals verbatim
(`refreshLayout`, `markLayoutRefresh`), but `Refresh` is as imperative as
`Update`. That was the original complaint.

**Bare `Invalidate()`** — does not distinguish a layout rebuild from a
render-only pass, and the distinction is the whole point of having two entry
points.

**`InvalidateRenderers` / `InvalidateRedraw`** — the plural breaks the pair, and
"invalidate a redraw" is backwards: a redraw is the action, not the stale thing.

## Follow-up

`UpdateView` is a separate case, tracked as #564. It does real work — it clears
the view registry and assigns `w.viewGenerator` — so it never lied the way
`UpdateWindow` did. It is simply a setter, and at the dominant call site
(`OnInit`) it performs the first assignment rather than an update. It carries
129 in-repo call sites plus every sibling's `main.go`, and no correctness hazard
behind it, so it does not ride along with this change.
