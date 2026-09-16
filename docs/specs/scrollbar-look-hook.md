# Spec: `ScrollbarCfg.Thumb` and `Track` hooks

Issue: #664 (step 6)

Status: **implemented** on branch `feat/focus-within`, not yet released.

## Motivation

`ScrollbarCfg` had colors and radii only. go-shirei's custom-scrollbars demo
draws a pill with a grip, a raised Windows 98 thumb and a thumb with ticks, and
none of these could be built. Stage 2 of the parity work skipped the demo.

## API

```go
type ScrollbarCfg struct {
    // ...
    Thumb func(ScrollbarState) View
    Track func(ScrollbarState) View
}

type ScrollbarState struct {
    Hovered, Pressed, Vertical bool
    Width, Height              float32 // the part's size, last layout pass
}
```

The issue named the state `ThumbState{Hovered, Pressed, Vertical}`. It became
`ScrollbarState` because the `Track` hook (open question 3, answered yes) takes
it too, and it gained the part's size, which go-shirei's `Thumb(size)` also
passes.

## Behavior

- Either hook makes `scrollbar` return an unexported view that builds in
  `GenerateLayout`, where the effective ID and the hover and press state can be
  read. A bar with a hook and no `ID` gets the leaf `scrollbar-x` or
  `scrollbar-y` under the scrollable, so `IsHovered` and `IsPressed` have a
  target.
- Children of the bar: the track box, if `Track` is set, then the thumb box. A
  thumb box holds the hook view and the stock drag handler. With only `Track`
  set, the stock thumb is used.
- The geometry is the stock one (`scrollbarAmendLayout`, now given the thumb's
  child index). After it runs, each hook box's children are moved by the amount
  the box moved, and the track box is set to the bar's rectangle.
- The stock thumb hides by painting itself transparent. A hook thumb paints its
  own colors, so a hidden one gets `shapeNone`, a zero size and `Clip`. In the
  hover-only mode the hover state brings it back.
- `makeScrollbarOnHover` sets only the cursor when `Thumb` is set.

### Size from the last pass

A hook view has children, and they are laid out at generation for the size its
box has then. The box gets its size only in `AmendLayout`, after the sizing
passes. So each box is built at the size the part had in the last layout pass,
stored per bar in `StateMap` (`nsScrollbarLook`). When the amend computes a
different size it stores it and calls `InvalidateLayout`. `FrameFn` runs that
second pass in the same frame, as it does for a new hover target, so the screen
never shows the first pass. The part sizes do not depend on the hook views (the
bar is `OverDraw`, out of flow), so the second pass does not ask for a third.

`TestRender` now runs the second pass too, as `FrameFn` does. Without it a
one-shot capture (`soft.RenderToPNG`) showed each hook view at its content size.

## Rejected Approaches

- **More colors** (`ColorThumbHover`, `ColorThumbPressed`, `ColorThumbBorder`):
  cannot draw a raised thumb or a grip (#664).
- **An app-built scrollbar from exported scroll state:** every app writes the
  drag, the gutter jump and the minimum thumb size again (#664).
- **Running the sizing passes again on the moved thumb subtree:** `layoutWidths`
  and `layoutHeights` add to the sizes already on a shape, so a second run on a
  laid-out subtree doubles Fit sizes. A safe re-run needs a reset of every shape
  in the subtree first.
