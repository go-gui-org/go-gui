# Developer Experience cheat sheet

go-gui is one of the easiest GUI toolkits to use. A widget is a struct plus a
callback, and most code works the first time. This page lists the few places
where the obvious reading is misleading. Each one is subtle: the code compiles,
the app runs, and one behavior is off. The linked specs explain the reasons, and
the tooling at the bottom finds most of them in your app.

## Focus

A shape is a focus target when it has `Focusable: true` and an `ID`. Tab order
also needs `!FocusSkip` and `!Disabled`.

```go
// No ID: renders and clicks, but never joins the tab order.
Button{Label: "Save", OnClick: save}

// Inputs are focusable by default. FocusDisabled opts out.
Input{ID: "name", FocusDisabled: true} // not in the tab order
```

Most input controls are focusable by default: `Button`, `ColorPicker`,
`Combobox`, `DatePicker`, `Input`, `InputDate`, `ListBox`, `NumericInput`,
`RadioButtonGroup`, `Radio`, `Select`, `Slider`, `Switch`, `Toggle`, `Tree`.
Everything else opts in with `Focusable: true`. If a control never answers the
keyboard, the usual cause is a missing `ID`. The `requiredid` analyzer and the
`DebugMissingIDs` gate report it. See `docs/specs/focusable-default-input.md`.

## ID scoping

`Shape.ID` is a leaf. The real identity is the effective ID: the leaf joined
with `:` to the IDs of its ID-bearing ancestors. An ID-less container adds no
scope. A leaf that already contains `:` is absolute. Effective IDs are unique
per window.

```go
// Under Panel{ID:"settings"} this is "settings:name".
// Under Panel{ID:"profile"} it is "profile:name".
Input{ID: "name", ...}
```

Two panels can contain the same leaf ID. Each is a different widget. Compose
IDs with `ScopeID` or `ScopeIDN`, never by hand. Public APIs — `SetFocus`,
`FindByID`, `IsFocus`, `ScrollVerticalTo`, `Test*` — take the effective ID.

A part (a row key, a heading slug) must not contain `:`. A composite widget's
inner shape sets `Shape.focusOwner` to the owner's leaf instead of repeating
its `ID`. Duplicates are loud: `gui.Debug` reports them, and
`(*Window).TestDuplicateIDs` asserts a clean window. See
`docs/specs/widget-id-scoping.md` and
`docs/specs/widget-id-per-scope-uniqueness.md`.

## `Opt[T]` vs plain fields

`Opt[T]` distinguishes "zero" from "not set". Use it for primitives where
zero is a real choice:

- Zero is a real choice (a border width of 0): `Opt[float32]`.
- Zero is not meaningful (widths, heights, counts): plain field.
- The type knows whether it was set (`Color`, `Padding`): plain field.

`SizeBorder` is the example: `0` means "no border", so a plain `float32`
cannot tell that from "not specified". `Color` and `Padding` carry their own
set flag and stay plain.

## Colors

Use plain `Color`, never `Opt[Color]`. Build values with `RGBA`, `RGB`, or
`Hex`. `Color{}` is unset, and `ColorTransparent` is explicitly transparent.
A `ColorSet` groups the per-state colors, and an assigned flat `Color` field
wins over it.

## `AmendLayout` and floats

`AmendLayout` runs after sizing with absolute coordinates. Moving a parent
there does not move its children. To move an element with its children, use
the float fields: `FloatAnchor`, `FloatTieOff`, `FloatOffsetX`,
`FloatOffsetY`.

## Group-box titles

`ContainerCfg.Title` draws a label in the top border, like an HTML fieldset.
Set `TitleBG` to the parent's background color, so the border line disappears
behind the label.

```go
// The label sits on a patch of the mismatched color.
Column{Title: "Account", TitleBG: RGB(255, 255, 255), ...}
```

## The one-event rule

Nothing is marked handled for you. A callback that acts on an event calls
`ctx.Consume()`. One that declines lets the event travel on. `ctx.Event` is
nil in `AmendLayout` and `OnScroll`, and both `EventCtx` methods are nil-safe.
See `docs/specs/eventctx-callback-refactor.md`.

## Find it early

`gui.Debug(true)`, or `GOGUI_DEBUG=1`, checks the layout every frame. It
reports duplicate IDs, focusable shapes without IDs, and handlers that act
without consuming. Findings print once per window. In tests, use
`(*Window).TestDuplicateIDs` and `(*Window).TestUnconsumedEvents`.

`DebugUnscopedIDs` is separate and opt-in: it is not part of `DebugAll`. It
reports an ID with no ID-bearing ancestor — the widget cannot move into a
second panel as it stands. Ask for it when you plan to reuse a screen:

```go
gui.DebugCategories(gui.DebugAll | gui.DebugUnscopedIDs)
```
