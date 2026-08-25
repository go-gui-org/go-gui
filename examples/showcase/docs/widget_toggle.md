Checkbox-style toggle with optional label.

## Usage

```go
gui.Toggle(gui.ToggleCfg{
    ID:       "accept",
    Label:    "Accept terms",
    Selected: app.Accepted,
    OnClick: func(ctx gui.EventCtx) {
        s := gui.State[App](ctx.Window)
        s.Accepted = !s.Accepted
    },
})
```

## Custom Check Text

```go
gui.Toggle(gui.ToggleCfg{
    ID:           "star",
    TextSelect:   "★",
    TextUnselect: "☆",
    Selected:     app.Starred,
})
```

## Key Properties

| Property     | Type    | Description                       |
| ------------ | ------- | --------------------------------- |
| Label        | string  | Label text beside the checkbox    |
| Selected     | bool    | Checked state                     |
| TextSelect   | string  | Text when selected (default "✓")  |
| TextUnselect | string  | Text when unselected              |
| MinWidth     | float32 | Minimum width                     |
| Disabled     | bool    | Disable interaction               |
| Invisible    | bool    | Hide without removing from layout |

## Appearance

| Property       | Type         | Description                                                      |
| -------------- | ------------ | ---------------------------------------------------------------- |
| Padding        | Opt[Padding] | Inner padding                                                    |
| Radius         | Opt[float32] | Corner radius                                                    |
| SizeBorder     | Opt[float32] | Border width                                                     |
| Color          | Color        | Background color (shorthand for `Colors.Base`)                   |
| Colors         | ColorSet     | Per-state colors: Base, Hover, Click, Focus, Border, BorderFocus |
| ColorSelect    | Color        | Background when selected                                         |
| TextStyle      | TextStyle    | Check mark text styling                                          |
| TextStyleLabel | TextStyle    | Label text styling                                               |

## Events

| Callback | Signature      | Fired when     |
| -------- | -------------- | -------------- |
| OnClick  | func(EventCtx) | Toggle clicked |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
