Dropdown selector with single or multi-select. Options
prefixed with "---" render as subheadings.

Select already accepts `[]string` directly via the
`Options` field — the zero-configuration path.

## Usage

```go
gui.Select(gui.SelectCfg{
    ID:       "lang",
    IDFocus:  600,
    Selected: app.Selected,
    Options:  []string{"Go", "Rust", "Zig"},
    OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = sel
    },
})
```

## Multi-Select

```go
gui.Select(gui.SelectCfg{
    ID:             "tags",
    Placeholder:    "Choose tags...",
    SelectMultiple: true,
    Options:        []string{"alpha", "beta", "stable"},
    OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Tags = sel
    },
})
```

## Key Properties

| Property       | Type     | Description                          |
|----------------|----------|--------------------------------------|
| Selected       | []string | Currently selected option(s)         |
| Options        | []string | Available choices ("---" = subhead)  |
| Placeholder    | string   | Hint text when empty                 |
| SelectMultiple | bool     | Allow multi-select                   |
| NoWrap         | bool     | Clip text in multi-select mode       |
| IDFocus        | uint32   | Tab-order focus ID (> 0 to enable)   |
| MinWidth       | float32  | Minimum width                        |
| MaxWidth       | float32  | Maximum width                        |
| FloatZIndex    | int      | Z-order for dropdown overlay         |
| Sizing         | Sizing   | Combined axis sizing mode            |
| Disabled       | bool     | Disable interaction                  |
| Invisible      | bool     | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorFocus       | Color        | Background when focused          |
| ColorSelect      | Color        | Selected item highlight          |
| TextStyle        | TextStyle    | Option text styling              |
| SubheadingStyle  | TextStyle    | Subheading text styling          |
| PlaceholderStyle | TextStyle    | Placeholder text styling         |

## Events

| Callback | Signature                            | Fired when          |
|----------|--------------------------------------|---------------------|
| OnSelect | func([]string, *Event, *Window)      | Selection changes   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
