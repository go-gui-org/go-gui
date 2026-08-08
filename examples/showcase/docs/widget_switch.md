Pill-shaped on/off toggle switch with animated thumb and optional label.

## Usage

```go
gui.Switch(gui.SwitchCfg{
    ID:       "feature",
    Label:    "Enable feature",
    Selected: app.Enabled,
    OnClick: func(ctx gui.EventCtx) {
        s := gui.State[App](ctx.Window)
        s.Enabled = !s.Enabled
    },
})
```

## Key Properties

| Property  | Type         | Description                       |
| --------- | ------------ | --------------------------------- |
| Label     | string       | Label text beside the switch      |
| Selected  | bool         | On/off state                      |
| Width     | Opt[float32] | Track width                       |
| Height    | Opt[float32] | Track height                      |
| Disabled  | bool         | Disable interaction               |
| Invisible | bool         | Hide without removing from layout |

## Appearance

| Property      | Type         | Description                                                      |
| ------------- | ------------ | ---------------------------------------------------------------- |
| Padding       | Opt[Padding] | Inner padding                                                    |
| SizeBorder    | Opt[float32] | Border width                                                     |
| Color         | Color        | Track color (shorthand for `Colors.Base`)                        |
| Colors        | ColorSet     | Per-state colors: Base, Hover, Click, Focus, Border, BorderFocus |
| ColorSelect   | Color        | Thumb color when on                                              |
| ColorUnselect | Color        | Thumb color when off                                             |
| TextStyle     | TextStyle    | Label text styling                                               |

## Events

| Callback | Signature      | Fired when     |
| -------- | -------------- | -------------- |
| OnClick  | func(EventCtx) | Switch toggled |

## Accessibility

| Property        | Type   | Description            |
| --------------- | ------ | ---------------------- |
| A11YLabel       | string | Accessible label       |
| A11YDescription | string | Accessible description |
