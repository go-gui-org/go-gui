Pill-shaped on/off toggle switch with animated thumb
and optional label.

## Usage

```go
gui.Switch(gui.SwitchCfg{
    ID:       "feature",
    IDFocus:  400,
    Label:    "Enable feature",
    Selected: app.Enabled,
    OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
        s := gui.State[App](w)
        s.Enabled = !s.Enabled
    },
})
```

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Label     | string       | Label text beside the switch         |
| Selected  | bool         | On/off state                         |
| Width     | Opt[float32] | Track width                          |
| Height    | Opt[float32] | Track height                         |
| IDFocus   | uint32       | Tab-order focus ID (> 0 to enable)   |
| Disabled  | bool         | Disable interaction                  |
| Invisible | bool         | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Track background color           |
| ColorHover       | Color        | Track on hover                   |
| ColorFocus       | Color        | Track when focused               |
| ColorClick       | Color        | Track on click                   |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorSelect      | Color        | Thumb color when on              |
| ColorUnselect    | Color        | Thumb color when off             |
| TextStyle        | TextStyle    | Label text styling               |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Switch toggled   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
