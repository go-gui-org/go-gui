Editable dropdown with type-ahead filtering. Typing
narrows the options list; selecting an option commits the value.

Combobox already accepts `[]string` directly via the
`Options` field.

## Usage

```go
gui.Combobox(gui.ComboboxCfg{
    ID:      "cb",
    IDFocus: 800,
    Value:   app.Value,
    Options: []string{"Go", "Rust", "Zig"},
    OnSelect: func(v string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Value = v
    },
})
```

## With Placeholder

```go
gui.Combobox(gui.ComboboxCfg{
    ID:          "search",
    Placeholder: "Search languages...",
    Options:     languages,
    OnSelect: func(v string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Lang = v
    },
})
```

## Virtualization

The dropdown list virtualizes when `Scrollable: true` and
`MaxDropdownHeight > 0`. Scroll state is keyed by the widget's ID + `".dropdown"`.

```go
gui.Combobox(gui.ComboboxCfg{
    ID:                 "large-cb",
    Scrollable:         true,
    MaxDropdownHeight:  200,
    Options:            largeList,
})
```

## Key Properties

| Property          | Type     | Description                          |
|-------------------|----------|--------------------------------------|
| Value             | string   | Current selection                    |
| Placeholder       | string   | Hint text shown when empty           |
| Options           | []string | Searchable options                   |
| MaxDropdownHeight | float32  | Max dropdown pixel height            |
| IDFocus           | uint32   | Tab-order focus ID (> 0 to enable)   |
| Scrollable         | bool     | Opt into the scroll system (state keyed by ID)|
| MinWidth          | float32  | Minimum width                        |
| MaxWidth          | float32  | Maximum width                        |
| FloatZIndex       | int      | Z-order for dropdown overlay         |
| Sizing            | Sizing   | Combined axis sizing mode            |
| Disabled          | bool     | Disable interaction                  |

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
| ColorHighlight   | Color        | Highlighted option color         |
| ColorHover       | Color        | Option hover color               |
| TextStyle        | TextStyle    | Option text styling              |
| PlaceholderStyle | TextStyle    | Placeholder text styling         |

## Events

| Callback | Signature                          | Fired when          |
|----------|------------------------------------|---------------------|
| OnSelect | func(string, *Event, *Window)      | Option selected     |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
