A panel with two faces that turns over in 3D to show its back face. Click the
panel, or focus it and press Enter or Space. The app owns which face shows: set
`Flipped`, and invert it in `OnFlip`. A spring runs the turn, so a second click
during a turn sends the panel back with no jump.

## Usage

```go
gui.FlipPanel(gui.FlipPanelCfg{
    ID:      "card",
    Sizing:  gui.FixedFixed,
    Width:   160,
    Height:  100,
    Front:   front,
    Back:    back,
    Flipped: app.Flipped,
    OnFlip: func(ctx gui.EventCtx) {
        a := gui.State[App](ctx.Window)
        a.Flipped = !a.Flipped
    },
})
```

The zero `Sizing` (FitFit) takes the size of the front face. The back face
always fills the panel. Give each face `Sizing: gui.FillFill` so it covers the
panel.

## Key Properties

| Property      | Type      | Description                                        |
| ------------- | --------- | -------------------------------------------------- |
| ID            | string    | Identifier; empty takes a generated one            |
| Front         | View      | Face shown when Flipped is false                   |
| Back          | View      | Face shown when Flipped is true                    |
| Flipped       | bool      | Shows the back face; the panel turns on a change   |
| Axis          | FlipAxis  | FlipAxisY (default, side to side) or FlipAxisX     |
| Spring        | SpringCfg | Turn motion; zero settles in about 600 ms          |
| Perspective   | float32   | Viewer distance in logical pixels; zero takes 800  |
| Sizing        | Sizing    | Panel sizing; zero (FitFit) takes the front's size |
| Width         | float32   | Panel width                                        |
| Height        | float32   | Panel height                                       |
| FocusDisabled | bool      | Takes the panel out of the tab order               |

## Events

| Callback | Signature      | Fired when                                          |
| -------- | -------------- | --------------------------------------------------- |
| OnFlip   | func(EventCtx) | Panel clicked, or Enter or Space pressed with focus |

A control on a face that consumes its click (`ctx.Consume()`) keeps the click,
and the panel does not turn.

## Behavior

- The face that is turned away is inert: it takes no clicks, has no tab stops
  and is not in the accessibility tree. Focus inside it moves to the panel.
- When the system asks for reduced motion, the faces cross-fade and do not turn.

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Card"}`.
