Draggable slider for selecting a value within a
numeric range. Supports horizontal and vertical orientations.

## Example

```go
gui.Slider(gui.SliderCfg{
    ID:      "vol",
    IDFocus: 900,
    Value:   app.Volume,
    Min:     0, Max: 100,
    OnChange: func(v float32, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Volume = v
    },
})
```

## Vertical Slider

```go
gui.Slider(gui.SliderCfg{
    ID:       "vert",
    Vertical: true,
    Value:    app.Level,
    Min:      0, Max: 10,
    Step:     0.5,
})
```

## Key Properties

| Property   | Type    | Description                          |
|------------|---------|--------------------------------------|
| Value      | float32 | Current value                        |
| Min        | float32 | Range minimum (default 0)            |
| Max        | float32 | Range maximum (default 100)          |
| Step       | float32 | Step increment                       |
| Vertical   | bool    | Vertical orientation                 |
| RoundValue | bool    | Round to nearest integer             |
| ThumbSize  | float32 | Thumb diameter                       |
| Size       | float32 | Track thickness                      |
| Width      | float32 | Fixed width                          |
| Height     | float32 | Fixed height                         |
| IDFocus    | uint32  | Tab-order focus ID (> 0 to enable)   |
| Sizing     | Sizing  | Combined axis sizing mode            |
| Disabled   | bool    | Disable interaction                  |
| Invisible  | bool    | Hide without removing from layout    |

## Appearance

| Property     | Type         | Description                      |
|--------------|--------------|----------------------------------|
| Padding      | Opt[Padding] | Inner padding                    |
| Radius       | Opt[float32] | Track corner radius              |
| RadiusBorder | Opt[float32] | Border corner radius             |
| SizeBorder   | Opt[float32] | Border width                     |
| Color        | Color        | Track background color           |
| ColorLeft    | Color        | Filled portion color             |
| ColorThumb   | Color        | Thumb color                      |
| ColorHover   | Color        | Thumb on hover                   |
| ColorFocus   | Color        | Thumb when focused               |
| ColorClick   | Color        | Thumb on click                   |
| ColorBorder  | Color        | Border color                     |

## Events

| Callback | Signature                          | Fired when       |
|----------|------------------------------------|------------------|
| OnChange | func(float32, *Event, *Window)     | Value changes    |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
