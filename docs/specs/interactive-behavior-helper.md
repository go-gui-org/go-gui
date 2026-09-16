# Spec: `gui.Interactive` behavior helper

Issue: #650

Status: **implemented** on branch `feat/650-interactive`, not yet released.

## Motivation

`Window.IsHovered` and `Window.IsPressed` (#587,
`docs/specs/build-time-interaction-state.md`) let a view pick its look from
interaction state. A look written by hand had two steps of boilerplate:

1. **Deferral.** The look had to be a named type with its own `GenerateLayout`.
   A factory body runs while the parent's `Content` slice is built, before the
   widget's scope exists, and the closure adapter `viewFunc` is unexported.
2. **Identity.** Each look called `w.EffID(id)` and gave the result to each
   query, and spelled "armed" as `IsPressed(eid) && IsHovered(eid)` again.

`Interactive` removes both. It is the first behavior helper.

## API

```go
type InteractionState struct {
    Hovered bool // Window.IsHovered(effID)
    Pressed bool // Window.IsPressed(effID)
    Armed   bool // Pressed && Hovered
    Focused bool // Window.IsFocus(effID); the widget itself only
}

func Interactive(id string, build func(InteractionState) View) View
```

`Interactive` returns an unexported view. Its `GenerateLayout` resolves
`w.EffID(id)`, reads the three queries, calls `build`, and generates the view
that `build` returns. The scope of the parent is open at that time, so the
caller names only the leaf.

## Semantics

- **Root ID.** The root of the built view must carry `id`. `Interactive` does
  not set it. When the root ID is different, or `id` is empty, the state never
  turns true, and `gui.Debug` reports it under `DebugMissingIDs`.
- **Disabled.** No `Disabled` field. A disabled shape is never a hover or press
  target (`enabledIDKey`), so `Hovered` is false for it. A press held from
  before a disable toggle stays recorded until release, so a look that paints
  from `Pressed` alone still checks its own disabled flag.
- **Nil.** A nil `build`, or a `build` that returns nil, gives an empty layout.
- **Allocation.** One closure and one boxed struct per use: the same as the
  named view type it replaces.

## Rejected Approaches

- **Export `viewFunc` as `gui.Deferred(func(*Window) View) View`.** Removes the
  deferral step only. The `EffID` step stays, and forgetting it fails silently.
  An exported deferral adapter can still be useful for `State[T]` reads in eager
  factories; that is a separate issue.
- **Scope-joining accessors `IsHoveredLeaf` / `IsPressedLeaf`.** Called from a
  factory body they join the wrong scope silently. They add API without removing
  the deferral, and add a second ID convention next to "public APIs take the
  effective ID".
- **go-shirei's implicit current-container model.** Needs builder closures on
  every container and a global current node: a view-tree redesign that conflicts
  with `docs/specs/view-single-method.md`.
- **`Disabled` in `InteractionState`.** Needs a disabled input on `Interactive`,
  and the hover record already excludes disabled shapes.
- **Stamp `id` onto the returned root.** Removes the mismatch failure, but
  writes into the caller's view silently. A debug report was chosen instead.
- **An `InteractiveCfg` struct.** Room for later inputs, but no input is known
  now. A later helper can take a Cfg without breaking this signature.
