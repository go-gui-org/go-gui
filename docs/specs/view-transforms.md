# Visual transforms on views

Issue #1019. Status: **planned** — phases 1 and 2 landed.

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
- **Not yet transformed.** Soft, web and PDF list the new kinds as no-ops until
  phase 6. A `ColorFilter` layer inside a transform is not checked.
- **Input.** `rotateMouseInverse` becomes a general inverse mapping. Every
  pointer walk keeps its map/recurse/restore shape.
- **Bounds.** `LocalToWindow` and `WindowToLocal` map points. The IME rect, a11y
  bounds, tooltips, drag start and float anchors use the internal form of these.
  A float stays upright at its transformed anchor.

## Phasing

| Phase | Content                                                                        | Status  |
| ----- | ------------------------------------------------------------------------------ | ------- |
| 1     | `Transform` type, chain methods, `Lerp`, the homography, its inverse, backface | landed  |
| 2     | `ContainerCfg.Transform`, render bracket, Metal + GL + mobile, clips, goldens  | landed  |
| 3     | Pointer input through the inverse, in every walk                               | pending |
| 4     | Backface culling for drawing and input                                         | pending |
| 5     | `LocalToWindow` / `WindowToLocal`, IME, a11y, tooltips, drag, float anchors    | pending |
| 6     | Soft backend (homography blit) and web (Canvas2D `setTransform`)               | pending |
| 7     | QuarterTurns input goes through the general mapping; special cases are deleted | pending |
| 8     | `examples/transforms/`, README, cheat-sheet snippet                            | pending |

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
- **A D3D backend path.** go-gui has no D3D backend. Windows uses GL through
  WGL.
