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

Most input controls are focusable by default: `Button`, `ColorChannelSlider`,
`ColorPicker`, `ColorPlane`, `ColorWheel`, `Combobox`, `DatePicker`, `Input`,
`InputDate`, `ListBox`, `NumericInput`, `RadioButtonGroup`, `Radio`, `Select`,
`Slider`, `Switch`, `Toggle`, `Tree`, `VirtualList`. Everything else opts in
with `Focusable: true`. If a control never answers the keyboard, the usual cause
is a missing `ID`. The `requiredid` analyzer and the `DebugMissingIDs` gate
report it. See `docs/specs/focusable-default-input.md`.

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

Two panels can contain the same leaf ID. Each is a different widget. Compose IDs
with `ScopeID` or `ScopeIDN`, never by hand. Public APIs — `SetFocus`,
`FindByID`, `IsFocus`, `ScrollVerticalTo`, `Test*` — take the effective ID.

A part (a row key, a heading slug) must not contain `:`. A composite widget's
inner shape sets `Shape.focusOwner` to the owner's leaf instead of repeating its
`ID`. Duplicates are loud: `gui.Debug` reports them, and
`(*Window).TestDuplicateIDs` asserts a clean window. See
`docs/specs/widget-id-scoping.md` and
`docs/specs/widget-id-per-scope-uniqueness.md`.

## `Opt[T]` vs plain fields

`Opt[T]` distinguishes "zero" from "not set". Use it for primitives where zero
is a real choice:

- Zero is a real choice (a border width of 0): `Opt[float32]`.
- Zero is not meaningful (widths, heights, counts): plain field.
- The type knows whether it was set (`Color`, `Padding`): plain field.

`SizeBorder` is the example: `0` means "no border", so a plain `float32` cannot
tell that from "not specified". `Color` and `Padding` carry their own set flag
and stay plain.

## Colors

Use plain `Color`, never `Opt[Color]`. Build values with `RGBA`, `RGB`, or
`Hex`. `Color{}` is unset, and `ColorTransparent` is explicitly transparent. A
`ColorSet` groups the per-state colors, and an assigned flat `Color` field wins
over it.

## `AmendLayout` and floats

`AmendLayout` runs after sizing with absolute coordinates. Moving a parent there
does not move its children. To move an element with its children, use the float
fields: `FloatAnchor`, `FloatTieOff`, `FloatOffsetX`, `FloatOffsetY`.

## Group-box titles

`ContainerCfg.Title` draws a label in the top border, like an HTML fieldset. Set
`TitleBG` to the parent's background color, so the border line disappears behind
the label.

```go
// The label sits on a patch of the mismatched color.
Column{Title: "Account", TitleBG: RGB(255, 255, 255), ...}
```

## Layout transitions: snap a channel or a subtree

`AnimateLayout` eases X, Y, Width and Height on every ID-bearing shape.
`AnimSnap` removes a channel from that, and the zero value keeps today's
behavior, so it is an opt-out:

```go
// Slides to its new position, jumps to its new size.
Column(ContainerCfg{ID: "card", AnimSnap: gui.AnimSnapSize, ...})

// Holds a scroll viewport or grid body still while the chrome animates.
Column(ContainerCfg{ID: "grid", AnimSnap: gui.AnimSnapAll, ...})
```

The mask inherits down the tree, and only down: a snapped container snaps
everything inside it, and a child cannot escape the container's mask. The hero
transition (`Shape.Hero`) ignores `AnimSnap` — it is already an explicit
per-shape opt-in.

## Virtualized lists

`ListBox`, `Table`, `Tree` and `Combobox` virtualize automatically when the
scroll container has a bounded height (`Scrollable` plus `Height`, `MaxHeight`,
or — `ListBox` only — a height Fill sizing resolved last frame). Every row is
the same height there, which is exact because the widget owns the row shape.

For rows the app builds, of heights only the layout engine knows, use
`VirtualList`:

```go
gui.VirtualList(gui.VirtualListCfg{
    ID:        "feed",
    ItemCount: len(msgs),
    Sizing:    gui.FillFill,
    // Optional but wanted whenever items can be inserted or reordered:
    // measured heights follow the key, not the index.
    ItemKey: func(i int) string { return msgs[i].ID },
    ItemView: func(i int, width float32) gui.View {
        // width is the inner width recorded by the previous arrange.
        // It is 0 on the first frame.
        return card(msgs[i], gui.ScopeIDN("feed", "row", i), width)
    },
})
```

Set `ItemHeight` when the height is cheap to compute: heights are then exact
from the first frame and nothing is measured. Put spacing _inside_ the row — the
list's own spacing is fixed at zero, because a gap between rows is height the
model does not account for.

**Use `width` for decisions, never for a minimum.** A row that sets `MinWidth`
from it asks the list for at least the width the list just reported; the row's
own border pushes that further, the list widens, and the cycle repeats every
frame — re-wrapping and re-measuring each time, so nothing settles. Rows fill
the width they are handed through `Sizing`. `gui.Debug(true)` reports the
ratchet.

Scroll by index, not by ID or percentage — a row outside the viewport has no
shape for `FindByID` to resolve, and the content height under virtualization is
an estimate, so a percentage drifts:

```go
w.ScrollToIndex("feed", 4000)          // row at the viewport top
w.ScrollToIndexAt("feed", 4000, 0.5)   // centred
w.ScrollIndexIntoView("feed", 4000)    // nearest edge; no-op when visible
w.ScrollToEnd("feed")                  // pin to bottom, exactly
```

These work on the uniform widgets too, in each one's own index space (a frozen
table header is data index 0 but sits outside the scrollable). Call
`w.InvalidateListHeights(id)` when a row's content changed under a stable key —
nothing detects that. See `docs/specs/virtualized-variable-height-lists.md`.

## `Wrap` with Fit width

`Wrap` with a Fit width resolves as **fit-content** (issue #379): the width is
`min(single-row sum, nearest definite-width ancestor's available)`, so the
container wraps within its parent instead of rendering one unwrapped row wider
than it. A Fit chain with no Fixed/Fill width above it has no width to wrap
within and keeps the single-row sum — that combination behaves as a `Row`, not
a wrap. When the wrap should always fill its parent, use Fill width, which is
what every example in this repo does.

## The one-event rule

Nothing is marked handled for you. A callback that acts on an event calls
`ctx.Consume()`. One that declines lets the event travel on. `ctx.Event` is nil
in `AmendLayout` and `OnScroll`, and both `EventCtx` methods are nil-safe. See
`docs/specs/eventctx-callback-refactor.md`.

## Find it early

`gui.Debug(true)`, or `GOGUI_DEBUG=1`, checks the layout every frame. It reports
duplicate IDs, focusable shapes without IDs, and handlers that act without
consuming. Findings print once per window. In tests, use
`(*Window).TestDuplicateIDs` and `(*Window).TestUnconsumedEvents`.

`DebugUnscopedIDs` is separate and opt-in: it is not part of `DebugAll`. It
reports an ID with no ID-bearing ancestor — the widget cannot move into a second
panel as it stands. Ask for it when you plan to reuse a screen:

```go
gui.DebugCategories(gui.DebugAll | gui.DebugUnscopedIDs)
```
