Interactive HSV color selection with SV area, hue slider,
alpha slider, hex input, and RGB/HSV channel inputs. Preserves hue
when saturation or value reaches zero.

## Usage

```go
gui.ColorPicker(gui.ColorPickerCfg{
    ID:    "cp",
    Color: app.Color,
    OnColorChange: func(c gui.Color, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Color = c
    },
})
```

## With HSV Channels

```go
gui.ColorPicker(gui.ColorPickerCfg{
    ID:      "cp-hsv",
    Color:   app.Color,
    ShowHSV: true,
    OnColorChange: func(c gui.Color, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Color = c
    },
})
```

## Key Properties

| Property | Type             | Description                        |
|----------|------------------|------------------------------------|
| Color    | Color            | Current color value                |
| ShowHSV  | bool             | Show H/S/V channel inputs          |
| IDFocus  | uint32           | Tab-order focus ID (> 0 to enable) |
| Sizing   | Sizing           | Combined axis sizing mode          |
| Width    | float32          | Fixed width                        |
| Height   | float32          | Fixed height                       |

## Appearance

| Property | Type             | Description                        |
|----------|------------------|------------------------------------|
| Style    | ColorPickerStyle | Full style override                |

ColorPickerStyle fields:

| Field            | Type    | Description                        |
|------------------|---------|------------------------------------|
| Color            | Color   | Background color                   |
| ColorHover       | Color   | Background on hover                |
| ColorBorder      | Color   | Border color                       |
| ColorBorderFocus | Color   | Border color when focused          |
| Padding          | Padding | Inner padding                      |
| SizeBorder       | float32 | Border width                       |
| Radius           | float32 | Corner radius                      |
| SVSize           | float32 | Saturation/value area size (px)    |
| SliderHeight     | float32 | Hue slider height (px)             |
| IndicatorSize    | float32 | Drag indicator diameter (px)       |

## Events

| Callback      | Signature                      | Fired when           |
|---------------|--------------------------------|----------------------|
| OnColorChange | func(Color, *Event, *Window)   | Color changed        |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |

## Components

- **SV area** -- click/drag to set saturation and value
- **Hue slider** -- vertical rainbow slider for hue selection
- **Alpha slider** -- horizontal slider (0--255)
- **Hex input** -- editable hex color string
- **RGB inputs** -- Red, Green, Blue channel inputs (0--255)
- **HSV inputs** -- Hue (0--360), Sat (0--100), Val (0--100) (when ShowHSV is true)
