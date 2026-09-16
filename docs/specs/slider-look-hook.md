# Spec: `SliderCfg.Look` hook

Issue: #664 (step 5)

Status: **implemented** on branch `feat/focus-within`, not yet released.

## Motivation

A custom slider built from the public API (`examples/custom_sliders`, #663)
wrote again what `gui.Slider` already does: the mouse lock drag with window
coordinates, the arrow keys, Home and End, and the value math. It had no mouse
wheel and no screen reader value. Its parts were floats, so every part needed an
ID to keep hover on the slider, and the width had to be fixed to place them
while the view was built.

`Look` keeps all behavior in `gui.Slider` and gives the app only the drawing.

## API

```go
type SliderCfg struct {
    // ...
    Look func(SliderLookState) SliderParts
}

type SliderLookState struct {
    InteractionState
    Pct float32 // value as a fraction of the range, 0 to 1
}

type SliderParts struct {
    Track, Fill, Handle View
}
```

## Behavior

- `Look` runs in `GenerateLayout` of an unexported view, so the effective ID is
  resolved in the right scope. The state is the same as `gui.Interactive` gives
  (`Window.interactionState`).
- The slider root is the same container as the stock one: `ID`, focus,
  `AccessRoleSlider` with the value, the press and drag handler, the keys and
  the wheel. `Width`, `Height`, `Sizing` and `Vertical` size it as before.
- **Track** is an in-flow child, centered by the root. It sets its own size,
  usually `FillFixed` (`FixedFill` when vertical).
- **Fill** and **Handle** are marked `Shape.outOfFlow` after generation. They
  take no room in the root, so the track is sized as if they were not there.
  They stay in the root's layer and are not clipped to it, so a knob shadow is
  drawn whole and hover needs no IDs on the parts.
- After arrange, the root's `AmendLayout` places them:
  - The handle center travels the track, inset by half the handle size at each
    end, so the handle never leaves the track.
  - The handle subtree is moved so its center is at the value and on the track's
    cross-axis center (`layoutShift`).
  - The fill subtree is moved to the track start, centered across it, and its
    length is set from the track start to the handle center.
- A press maps over the same inset span, so the value under the pointer is the
  value where the handle is drawn.
- A nil part is left out. With no track the root is the track. With no handle
  there is no inset.
- `Color*`, `SizeBorder`, `Radius`, `ThumbSize` and the focus ring are not used.

The fill's children are laid out for the size the fill had before its length was
set, and keep that place. A fill laid out as wide as the track, with `Clip`,
cuts its content at the value; the Material look uses this to square the end of
a round fill.

## Rejected Approaches

- **A behavior helper for a plain container** (`SliderBehavior(&cfg, ...)`): two
  drag paths to keep in step, and the app places the parts itself, with a fixed
  width or its own `AmendLayout` (#664).
- **Exporting the value math** (`SliderValueAt`, `SliderStep`): the drag, the
  window coordinates of the lock and the screen reader value stay app code.
- **Floats for Fill and Handle:** a float is hit-tested in its own layer, so an
  ID-less part hides hover from the slider unless the app gives every part an
  ID.
- **`OverDraw` for Fill and Handle:** an `OverDraw` child is clipped to its
  parent, which cuts a knob shadow or a focus ring drawn outside the slider.
- **A part for the area around the track** (go-shirei's icon, open question 1 of
  #664): the app wraps the slider or puts the content in a part; the Apple look
  puts its icon in the fill.

## Known limits

- The wheel moves the value by one unit per line, as on the stock slider, not by
  `Step`. A range of 0 to 1 reaches an end on the first turn.
