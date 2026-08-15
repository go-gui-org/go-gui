Circular radio button for selecting one option. Typically used inside a
RadioButtonGroup, but can be used standalone.

## Usage

```go
gui.Radio(gui.RadioCfg{
    ID:       "opt-go",
    Label:    "Go",
    Selected: app.Lang == "go",
    OnClick: func(ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Lang = "go"
    },
})
```

## Key Properties

| Property  | Type         | Description                        |
| --------- | ------------ | ---------------------------------- |
| Label     | string       | Label text beside the radio circle |
| Selected  | bool         | Selection state                    |
| Size      | Opt[float32] | Circle diameter                    |
| Disabled  | bool         | Disable interaction                |
| Invisible | bool         | Hide without removing from layout  |

## Appearance

| Property      | Type         | Description                                                      |
| ------------- | ------------ | ---------------------------------------------------------------- |
| Padding       | Opt[Padding] | Inner padding                                                    |
| SizeBorder    | Opt[float32] | Border width                                                     |
| Color         | Color        | Circle color (shorthand for `Colors.Base`)                       |
| Colors        | ColorSet     | Per-state colors: Base, Hover, Click, Focus, Border, BorderFocus |
| ColorSelect   | Color        | Fill color when selected                                         |
| ColorUnselect | Color        | Fill color when unselected                                       |
| TextStyle     | TextStyle    | Label text styling                                               |

## Events

| Callback | Signature      | Fired when    |
| -------- | -------------- | ------------- |
| OnClick  | func(EventCtx) | Radio clicked |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
