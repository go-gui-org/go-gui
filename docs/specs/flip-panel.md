# FlipPanel

Issue #1020. Status: **implemented**.

## Problem

Go-Gui had no control that turns over to show a second face. The transforms
example built one by hand in about 70 lines. It had no keyboard support, put
both faces in the accessibility tree, ignored reduced motion, and used a tween,
so a second click during a turn stopped the card dead and started it again.

## Design

`FlipPanel(FlipPanelCfg) View` (`gui/view_flip_panel.go`). The app owns
`Flipped` and inverts it in `OnFlip(EventCtx)`. The panel keeps its angle in a
`StateMap` entry under its effective ID.

- **Turn.** The front face turns to the angle and the back face to the angle
  plus 180°. Both use `BackfaceHidden`, so only the face toward the viewer
  draws. The faces read the angle through `TransformFn`, so a spring with
  `AnimationRefreshRenderOnly` turns the panel with no rebuild.
- **Second toggle.** Each toggle adds a new `SpringAnimation` with the same ID.
  `AnimationAdd` carries the old spring's position and velocity into it (#1015).
- **Stacking.** The back face comes first and is marked `Shape.outOfFlow`. An
  out-of-flow child is placed at the cursor of the next in-flow child, so it
  lands on the front face. The back face also sets `Shape.fillOutOfFlow`, and
  `fillOutOfFlowMain` gives such a Fill child the parent's content box on the
  main axis too. Before, only the cross axis did this. A separate flag, not
  `outOfFlow` alone, because a slider Look's Fill and Handle parts are also out
  of flow and must keep their fit size along the track. The panel takes the size
  of the front face, or the size its own `Sizing` gives it.
- **Inert face.** The face that is turned away is inert: an unexported
  `ContainerCfg.inert`, stored on `shapeTransform`, not on `Shape`, so it costs
  no `Shape` bytes. `xfCoordsInverse` refuses the pointer for the hit walks. The
  window-point mappings refuse it when they test a hit (`xfWindowToLocal` and
  `xfLocalToWindowClip` with `clip` set), which covers press-blur, the soft
  keyboard and `TestClick`. A frame mapping (`clip` false) still maps through
  the face, because it is still drawn. The tooltip hover test maps with no clip,
  so it checks `layoutInert` itself. The tab-order and accessibility walks skip
  the subtree. The inert face is the one `Flipped` does not name, so the
  accessibility tree changes on the toggle, not halfway through the turn.
- **Focus on the inert face.** Like the HTML attribute, inert takes focus away.
  A control on the hidden face that keeps focus would still take Enter and
  Space, and it is not in the accessibility tree. Each frame,
  `flipPanelDropHiddenFocus` looks for the focused ID in the hidden face. If it
  finds it, a queued command moves focus to the panel, or clears it when the
  panel has `FocusDisabled`. The command is queued because generation holds the
  frame lock, where `SetFocus` panics.
- **Reduced motion.** No turn. A layout-refresh tween cross-fades the faces by
  scaling the opacity of every shape in each face. Opacity is set per shape at
  generation, so the fade cannot run in render-only frames.

## Rejected Approaches

- **A behavior helper like `HoverLift`** (`Flip(id, flipped, &angle, build)`).
  Less exported surface, but every caller must stack the faces and add keyboard,
  accessibility and reduced-motion support. The hand-built showcase card shows
  that callers skip them.
- **A doc recipe with no widget.** Same gaps as the helper, and the code is
  copied.
- **Stacking with `Translate(-width, 0)` on the back face**, as the hand-built
  card did. It needs both faces at one fixed width known at build time.
- **Stacking with a `Float`.** A float is moved to its own layer. It leaves the
  panel's clip and the panel's place in the tab order.
- **`OnFlip func(*Window)`**, as the issue wrote it. Every Go-Gui callback is
  `func(EventCtx)`.
