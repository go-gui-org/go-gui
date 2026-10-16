# Spec: Stack container

Issue #1071. Status: **implemented**.

## Problem

Go-Gui had Row and Column, but no container that puts children one over the
other. Controls over a background image or canvas, a badge on an avatar or a
game HUD had two workarounds, and both were wrong:

- **Float children.** `layoutRemoveFloatingLayouts` (`gui/layout_float.go`)
  moves a float out of the tree into a top-level layer. The layer then does not
  scroll or clip with its container, does not count toward its Fit size, and
  draws over everything in the main tree, menus included.
- **Canvas.** A Canvas (`axisNone`) does not stretch a `Fill` child: the fill
  pass is size-neutral for `axisNone` on purpose (`gui/layout_sizing.go`). A
  background layer stayed 0×0.

## Decision

`gui.Stack(cfg ContainerCfg) View`, backed by a new internal axis, `axisStack`.
It adds one exported function.

| Pass     | Behavior                                                         | Code                                                          |
| -------- | ---------------------------------------------------------------- | ------------------------------------------------------------- |
| Fit      | Width takes the column branch, height the row branch: max.       | `layoutWidthsDepth`, `layoutHeightsDepth`                     |
| Fill     | Both axes take the cross-axis fill: a Fill child = content.      | `layoutFillWidthsImplDepth`, `layoutFillHeightsImplDepth`     |
| Position | No advance. Each child is aligned on both axes.                  | `layoutPositionsDepth`                                        |
| Content  | Max on both axes, through the existing non-main branch.          | `computeContentWidth`, `computeContentHeight`                 |
| Text     | A wrapped Text shrinks to its ink under a non-left Stack too.    | `wrapParentHAlign`                                            |
| Input    | No change. Pointer walks already go last child first.            | `mouseDownHandlerDepth` and the other `slices.Backward` walks |
| Spacing  | Ignored, with no diagnostic. No pass that a Stack uses reads it. | —                                                             |

The cross-axis sibling-sum cache in `layoutFillCrossAxis` assumes a parent fills
on one axis. A Stack fills on both, but it never takes that branch, because its
axis is never `mainAxisOf(axis)`.

Per-layer placement uses a `FillFill` Column wrapper with its own `HAlign` and
`VAlign`. A wrapper with no handler does not consume a press, so the press goes
on to the layers under it (`examples/stack/main_test.go`).

## Rejected Approaches

- **Document Float as the answer.** A Float exists to escape its container. A
  layer must stay inside it: scroll and clip with it, and size a Fit parent.
- **`StackCfg` with a `Layers []StackLayer{View, Align, Offset}` slice.** A
  second Cfg type breaks the rule that every container takes `ContainerCfg`,
  copies or drops its padding, color, events and ID, and allocates a slice per
  Stack per frame. Per-layer alignment is already possible with a wrapper.
- **Make Canvas stretch Fill children.** It changes Canvas for apps that place
  children by absolute X/Y, and breaks the size-neutral `axisNone` fill
  invariant that the Splitter root relies on (#262).
- **A `ZIndex` field on in-flow children.** Child order already sets the
  z-order. A second sort key would let draw order and hit-test order disagree.
- **A `gui.Debug` finding for `Spacing` on a Stack.** Decided against: Spacing
  is ignored silently. It does no harm, unlike `Wrap` with `Overflow`, which
  silently drops one of two strategies.
