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
    // targetWithin(FocusID(), effID); the widget or an ID-bearing
    // descendant, such as an Input inside a wrapper (#664)
    FocusWithin bool
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

## Keyboard activation (#658)

A custom button matches `Button` from the keyboard when its root sets
`Focusable`, `OnClick`, `ClickOnSpace` and `ClickOnEnter`.

- **Report.** A root with `OnClick` that lacks `Focusable`, `ClickOnSpace` or
  `ClickOnEnter` works with the mouse and does nothing from the keyboard.
  `gui.Debug` reports it under `DebugMissingIDs` and names the missing fields.
- **Space presses, release clicks.** On the focused widget, Space key down
  records `viewState.keyPressTargetID` and the key up fires `OnClick`. This is
  the push-button convention of HTML, GTK and Win32. The space character between
  the two is claimed so it does not type, but it does not click. A key repeat
  changes nothing. A Ctrl, Alt or Super chord does not press.
- **Cancel.** A focus change, a window blur (`EventUnfocused`), Escape, or
  `SetView` clears the press without a click. Re-asserting focus on the same
  widget is not a focus change.
- **Enter** clicks on key down, as before, and leaves no press.
- **State.** `IsPressed` is true while the key press is held.
  `InteractionState.Armed` is true for a key press without hover: a key has no
  pointer to drag off the widget. `Button` paints its click color while the key
  press is held.
- **Separate record.** The key press is not stored in `pressTargetID`, so a
  mouse release does not end a held Space and a Space release does not end a
  held mouse press.
- **X11 auto-repeat.** Without detectable auto-repeat, X11 sends a release per
  repeat, so a held Space clicks once per repeat there. The char path did the
  same before.
- **macOS.** The release convention was not checked against `NSButton`. The same
  rule applies on all platforms.

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
- **A keyboard preset field on `ContainerCfg`** (#658). One field that turns on
  `Focusable`, `ClickOnSpace` and `ClickOnEnter` is a second spelling of three
  fields that exist, and it can be forgotten the same way. A preset that sets
  `Focusable` also works against the `FocusDisabled` opt-out convention. The
  debug report covers the failure with no new API.
- **Turn the keyboard fields on inside `Interactive`** (#658). Writes into the
  caller's view, like stamping the ID.
- **Store the key press in `pressTargetID`** (#658). A mouse release would end a
  held Space, and a Space release would end a held mouse press.
- **Enter on release, or a held Enter state** (#658). Enter clicks on key down
  on the reference platforms.
- **An `InteractiveCfg` struct.** Room for later inputs, but no input is known
  now. A later helper can take a Cfg without breaking this signature.
