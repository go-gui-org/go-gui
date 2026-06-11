Circular radio button for selecting one option. Typically
used inside a RadioButtonGroup, but can be used standalone.

## Usage

```go
gui.Radio(gui.RadioCfg{
    ID:       "opt-go",
    IDFocus:  500,
    Label:    "Go",
    Selected: app.Lang == "go",
    OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Lang = "go"
    },
})
```

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Label     | string       | Label text beside the radio circle   |
| Selected  | bool         | Selection state                      |
| Size      | Opt[float32] | Circle diameter                      |
| IDFocus   | uint32       | Tab-order focus ID (> 0 to enable)   |
| Disabled  | bool         | Disable interaction                  |
| Invisible | bool         | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Circle background                |
| ColorHover       | Color        | Circle on hover                  |
| ColorFocus       | Color        | Circle when focused              |
| ColorClick       | Color        | Circle on click                  |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorSelect      | Color        | Fill color when selected         |
| ColorUnselect    | Color        | Fill color when unselected       |
| TextStyle        | TextStyle    | Label text styling               |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Radio clicked    |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
