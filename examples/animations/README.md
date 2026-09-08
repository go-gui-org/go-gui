# Animations

> **Framework:** animation, layout, text **Description:** Tween, spring,
> keyframe, layout, hero, and text animations in one window. Shows how each
> animation type interpolates state.

![Preview](screenshot.png)

<!-- explorer: tags=animation,layout category=animation run=go -->

---

## Run

```sh
go run ./examples/animations/
```

## What it demonstrates

Tween, spring, keyframe, layout, and hero transitions in one window. Shows how
each animation type interpolates state.

The bottom row shows the canned text animations from `TextCfg.Anim`. These need
no button: an animation declared on a `Text` registers itself the first time the
text is generated, and retires on its own when the text leaves the view tree.
The row also shows the `Custom` escape hatch, which takes progress and returns a
frame.

The gap in that row is the typewriter. It paints only the part of the string
that is due, but its box keeps the full string's width, so the labels beside it
do not move as it types.

See `main.go` for the implementation.
