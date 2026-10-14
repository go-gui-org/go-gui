# Visual transforms on views

Issue #1019. Status: **implemented**.

## Problem

A view cannot translate, scale, rotate, skew or turn in 3D. Only two things
rotate today. `Shape.QuarterTurns` (`RotatedBox`) turns in 90° steps and also
changes layout. The `DrawCanvas` transform (#904) changes only the canvas
drawing. Without a transform on views, a card that flips over (#1020), a card
that lifts on hover, a tilted badge and tumbling confetti cannot be built.

## Design

### API

- `gui.Transform` is a value type. Its zero value is the identity.
- Package functions build it, and methods with the same names chain:
  `gui.Rotate(8).Scale(1.05)`,
  `gui.RotateY(180).Perspective(800).BackfaceHidden()`.
- The components are fixed, and they are applied in the order of the CSS
  individual transform properties, outermost first: translate → perspective →
  rotateX → rotateY → rotate → skew → scale. They turn about an origin, which is
  a fraction of the border box. The default origin is the center.
- A method sets its component and replaces an earlier value.
- `Transform.Lerp` interpolates each component on its own, so springs and tweens
  can animate any transform into any other.
- The field is `ContainerCfg.Transform`. A widget that is not a container gets a
  transform when it is wrapped in a `Row`.
- For animation, `ContainerCfg.TransformFn func() Transform` is read at render
  time, so a spring or tween moves the view in render-only frames (#1016) with
  no view rebuild. It works like `DrawCanvasCfg.VersionFn`.
  - When it is set, it wins over `Transform`.
  - It runs under the frame lock. It must be pure: read animation state and
    return a value, nothing else.
  - The render pass stores the result in the shape, and input reads that same
    value, so a click lands where the view was drawn.

### Mechanics

- **Storage.** Each component is stored so that 0 means "no effect". Scale is
  stored as `s-1`, the origin as an offset from the center, and perspective as
  `1/d`, which is also the space CSS interpolates it in. Because of this, the
  zero value needs no `Opt` wrappers.
- **The matrix is a 3x3 homography, not a 4x4.** A view lies on z = 0, and its
  output is flattened back onto the screen (CSS `transform-style: flat`, the
  default). So the z row and z column of the 4x4 never matter.
  - The 3x3 that is left is exact for perspective.
  - Its inverse maps the pointer back with no ray cast.
  - Backends expand it into the existing MVP stack, so the shaders do not
    change.
- **Backface.** CSS shows the back face when m33 of the inverse of the 4x4 is
  negative. That value is `det(3x3) / det(4x4)`, so the test is a sign
  comparison. A mirror (a negative scale) stays a front face, as it does in CSS.
- **Shape.** The field is `xf *shapeTransform`, nil when no transform is set.
  The pointer is needed because `Shape` has 8 bytes of headroom in its size
  class.
- **Render.** A new `RenderTransformBegin/End` pair carries a pointer to the
  matrix, so `RenderCmd` does not grow.
  - The bracket opens before `renderShape`, so the view's own background is
    transformed too.
  - Backends multiply the matrix into the `mvpStack` that `RenderRotateBegin`
    already uses.
- **Clips.** A scissor rect is mapped through a transform that only translates
  and scales. Under any other transform, the clip falls back to the stencil
  path, whose mask quad already goes through the MVP.
- **Culling.** Inside a transformed view, drawing culls against nothing
  (`xfUnboundedClip`): the parent's clip is a rect in the parent's frame, and in
  the view's frame it is not a rect. For the same reason, inside a transform the
  render walk takes a shape's own clip from its bounds cut to the clip its
  parent passed (`renderShapeClip`), not from its layout clip. The layout clip
  stays cut by ancestors in the window, because hit testing reads it, and until
  phase 3 hit testing is untransformed. Each scissor is mapped out through the
  open brackets (`emitClipCmd`) and cut to the scissor in effect when the
  bracket opened, so nothing draws outside an ancestor's clip.
- **Soft.** The bracket draws into a layer in the view's own frame, and the
  layer is mapped into the parent through the inverse matrix, pixel by pixel.
  This is exact for perspective. A `RenderClip` is a window rect, so inside a
  bracket it is mapped back into the view's frame. The layer is the size of the
  window, so content that lies outside the window before the transform is lost.
- **Web.** Canvas2D has only affine transforms. A matrix with perspective is
  drawn with the affine that matches it at the center of the view's box
  (`homography.AffineAt`). `TransformBegin` carries that box in X, Y, W and H.
  Rotation and transform brackets keep the canvas transform in Go, not on the
  canvas save stack, because a clip rebuild restores that stack. A rect clip is
  a window rect, drawn with no transform. Under perspective, gui maps it to the
  window through the true matrix, but the canvas draws through the flat
  stand-in, so the rect would cut off the far side of the view. There the rect
  is mapped back into the view's frame through the true matrix and drawn under
  the stand-in. Each stencil is drawn with the transform it opened under.
- **PDF.** PDF has only affine transforms too, and draws a view as web does
  (`pdfAffineAt`). fpdf ends every clip and transform with a `Q`, which restores
  the newest `q` of either kind, so `pdfGState` keeps the clips and transforms
  in Go. On each change it ends all its `q`s and opens them again: each clip
  under its own matrix, then the matrix the content draws with. A rounded
  stencil is still drawn as a rect clip.
- **Not yet checked.** A `ColorFilter` layer inside a transform.
- **Input.** Every pointer walk (press, move, release, wheel, file drop, hover,
  mouse leave, gesture, the interaction target and the idle move check) maps the
  pointer with `xfCoordsInverse` when it enters a transformed shape, before it
  tests that shape, and restores it on the way out. The shape's own fill is
  drawn through the transform, so the map comes first, not only for the
  children, unlike a quarter turn. The walks keep their map/recurse/restore
  shape.
  - The matrix comes from two places. Layout resolves every transform after
    arrange (`layoutResolveTransforms`), so hover and the first click have a
    matrix before the first frame. The render walk resolves it again, so input
    follows a `TransformFn` that changed in a render-only frame.
  - A point on no part of the view maps to NaN (`xfNowhere`), which every hit
    test misses: a hidden back face, a view flattened to a line, or a ray that
    meets the view's plane behind the viewer. `pointerMissesSubtree` treats NaN
    as a miss, so a quarter turned child under it is skipped too.
  - Hit clips. Under a turn, a skew or perspective, no rect in the view's frame
    holds the clip its ancestors pass. So `layoutSetShapeClips` keeps that clip
    on the transform (`xf.outer`, in the parent's frame), and the clips inside
    the view start unbounded. `xfCoordsInverse` tests the pointer against
    `xf.outer` before it maps it in. Every walk gets the exact ancestor clip
    this way, also the hover, target and idle move walks, which test no ancestor
    on the way down. The clips do not depend on the matrix, so a render-only
    frame that changes it leaves none stale.
  - Rejected: cutting the ancestor clip into the view's frame as the bounding
    box of its mapped corners. The box is bigger than the clip, so the walks
    that test no ancestor hit cut-off corners, and the box goes stale when a
    render-only frame changes the matrix.
  - When a view's transform is the identity at render time, the render walk
    still opens a flat frame with no bracket (`xfFrame.flat`). Its subtree then
    takes its scissors from the render walk, not from the unbounded layout
    clips.
  - Mouse lock. The press walk keeps the inverse maps it applies, outer first
    (`viewState.xfPress`): each view transform, and each quarter turn around the
    children. `MouseLock` composes them (`mouseLockXf`), and the locked press,
    move and release are mapped through it (`lockMouseMap`). So a lock taken
    inside a view gets its events in the frame the press reached the widget in,
    and its saved shape position matches. A lock taken outside a press walk (a
    queued command) keeps window coordinates.
  - `xfLocalToWindow` maps a point out through the quarter turns and view
    transforms above it, and fails for a point outside a view's `xf.outer`.
    `shapeHitPoint` tries the clip center, then a 5x5 grid across the clip, so a
    widget that is partly cut off is still found. `TestClick` and the
    unconsumed-event sweep aim with it.
  - Code that tests a window point against a shape outside the walks maps it
    with `xfWindowToLocal` (`layoutHitsWindowPoint`): blur on press, the soft
    keyboard, the tooltip hook, the focused-scroll path and the unconsumed-event
    sweep's ancestor test. The RTF tooltip keeps its bounds in the window
    (`xfBoxToWindow`). Transforms resolve once before the `AmendLayout` hooks
    too, for the tooltip hook.
  - Drag to scroll replays a tap from the root, so it keeps the press and
    release in window coordinates for that. Its claim walks map through nested
    transforms. A `MouseLock` from inside a locked callback keeps the drag's
    frame (`lockCallback`).
  - Not yet mapped: a part drawn outside a parent that does not clip, which the
    parent's own test prunes, as for any overflowing child.
- **Bounds.** `Window.LocalToWindow` and `Window.WindowToLocal` map a point
  between a widget's box (relative to its top-left corner, as `e.MouseX` is in
  `OnClick`) and the window, through `xfLocalToWindowClip` and `xfWindowToLocal`
  with no clip test. They read the last laid-out frame.
  - The IME caret rect is mapped at render time through the open brackets
    (`renderBoxToWindow`, `xfFrame.box`). A quarter turn opens a frame there too
    (`renderQuarterTurnBegin`), so the rect of an input under a `RotatedBox` is
    turned.
  - An a11y node under a view transform or a quarter turn reports the box of its
    four mapped corners (`xfBoxToWindowOK`). A node that is not drawn (a hidden
    back face) reports an empty box. The ancestor walk runs only below a
    transform or a turn.
  - A float's anchor point on its parent is mapped out to where the parent is
    drawn (`floatAnchorPoint`), auto-flip included. The float is drawn in its
    own layer, so it stays upright. A transform that `TransformFn` moves in a
    render-only frame carries its floats with it: the render-only pass refreshes
    the matrices and re-places each anchored float from them
    (`layoutReattachFloats`), shifting its subtree and its hit clips, so hit
    testing follows what is drawn. Rejected: leaving floats frozen until the
    next layout, which lags exactly the animation the render-only frame exists
    for.
  - Drag-reorder needs no index change: the press that arms it and the locked
    moves are in one frame (see mouse lock). The ghost's offset is in the list's
    frame, so it is mapped through the transform with the anchor
    (`offsetThroughTransform`): the ghost tracks the pointer and stays upright.

- **Quarter turns (phase 7).** The render walk opens an `xfStack` frame for a
  quarter turn, as for a view transform, so a clip inside a `RotatedBox` is
  mapped out to the window like any other. It used to be emitted in the unturned
  frame, which cut a 90° turned clipping child to the wrong strip. A quarter
  turn maps a rect to a rect, so `xfMat.axisAligned` counts it as axis aligned
  and the scissor stays exact, with no stencil. A `Transform` with `Rotate(90)`
  gets the same exact scissor.
  - `rotateClipToUnrotated` maps two corners through `rotateCoordsInverse`, so a
    half turn mirrors the clip too. Layout used to pass a half turn the clip
    unturned: a parent clip that hid one end of the box hid the wrong end of the
    children for hit testing.
  - A `RenderClip` inside a `RotateBegin` is now a window rect, so each backend
    that draws a turn on its own maps it back. Soft keeps the turn as a matrix
    on its rotation bracket and maps the clip into the unturned layer; the layer
    draws unclipped and the blit cuts to the clip at the bracket's start. Web
    and PDF compose the turn into the map they use to bring a clip back under
    perspective (`xfView`/`xfH`, `view`/`h`). The GPU backends need nothing: a
    scissor is a window rect there already.
  - The input walks keep their map, recurse and restore shape. A quarter turn
    maps only the children, and the view transform maps the shape itself, so the
    two stay separate steps, both through one helper each
    (`rotateCoordsInverse`, `xfCoordsInverse`). The gesture walk's copy of the
    turn math (`rotateCentroidInverse`) is gone.

## Phasing

| Phase | Content                                                                        | Status |
| ----- | ------------------------------------------------------------------------------ | ------ |
| 1     | `Transform` type, chain methods, `Lerp`, the homography, its inverse, backface | landed |
| 2     | `ContainerCfg.Transform`, render bracket, Metal + GL + mobile, clips, goldens  | landed |
| 3     | Pointer input through the inverse, in every walk                               | landed |
| 4     | Backface culling for drawing (with 2) and input (with 3)                       | landed |
| 5     | `LocalToWindow` / `WindowToLocal`, IME, a11y, tooltips, drag, float anchors    | landed |
| 6     | Soft backend (homography blit), web (Canvas2D `setTransform`) and PDF          | landed |
| 7     | QuarterTurns input goes through the general mapping; special cases are deleted | landed |
| 8     | `examples/transforms/`, README, cheat-sheet snippet                            | landed |

## Rejected Approaches

- **A `Transform` embed in every Cfg, like `A11YCfg`.** It needs 43 shape
  builders copied by hand, which is the same drift that A11Y had. It can be
  added later on top of the container field.
- **A wrapper view `gui.Transformed(cfg, child)`.** Its extra ID scope changes
  the effective IDs of the children, which breaks `SetFocus` and `FindByID`
  callers when they add a transform.
- **Exported fields (`ScaleX float32`).** The zero value would not be the
  identity, and four or more fields would need `Opt[float32]`.
- **An ordered op list (`[]TransformOp`).** It allocates a slice per frame, and
  `Lerp` between two different lists is not defined.
- **Compositing transformed subtrees on the CPU.** MyGo built this and then
  removed it after profiling: CPU use of an animation went from 69.8% to 12.9%
  on the GPU (egoist/mygo PR #110). The GPU MVP path exists already.
- **GPU backface culling.** It needs cull state on four backends, and input
  needs the CPU answer anyway.
- **A clip mask cached between frames.** The mask is one quad per frame, which
  does not justify cache state and invalidation.
- **Animating through an exported `amendOnRender`.** The app cannot write the
  shape's unexported transform from `AmendLayout`, and the hook would let app
  code mutate layout under the frame lock.
- **Binding a transform to an animation ID.** It couples the transform to one
  animation kind. `TransformFn` reads any state.
- **Dropping the perspective terms on web.** It is not the same as dropping
  perspective before the origin is applied, so the view drifts away from where
  the GPU draws it. The affine at the box center keeps the center in place.
- **A D3D backend path.** go-gui has no D3D backend. Windows uses GL through
  WGL.
