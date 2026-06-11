Rotate child content by quarter turns (90° increments).
Returns the child directly when turns == 0 (no rotation). Negative
values and values > 3 are normalized (mod 4).

## Usage

```go
gui.RotatedBox(gui.RotatedBoxCfg{
    QuarterTurns: 1,
    Content:      gui.Text(gui.TextCfg{Text: "Sideways"}),
})
```

## Vertical Label

```go
gui.RotatedBox(gui.RotatedBoxCfg{
    QuarterTurns: 3,
    Content: gui.Text(gui.TextCfg{
        Text:      "Y Axis",
        TextStyle: t.B3,
    }),
})
```

## Key Properties

| Property     | Type | Description                         |
|--------------|------|-------------------------------------|
| QuarterTurns | int  | Number of 90° clockwise turns (0–3) |
| Content      | View | Single child view to rotate         |

## Behavior

| Turns | Angle | Dimensions |
|-------|-------|------------|
| 0     | 0°    | unchanged  |
| 1     | 90°   | w ↔ h swap |
| 2     | 180°  | unchanged  |
| 3     | 270°  | w ↔ h swap |

Content is clipped to the rotated bounds. Sizing is always FitFit —
the rotated box measures to its child size (swapping width/height for
odd quarter turns).
