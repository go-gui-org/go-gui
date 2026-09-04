Editable dropdown with type-ahead filtering. Typing narrows the options list.
Selecting an option commits the value.

Combobox already accepts `[]string` directly via the `Options` field.

## Usage

```go
gui.Combobox(gui.ComboboxCfg{
    ID:      "cb",
    Value:   app.Value,
    Options: []string{"Go", "Rust", "Zig"},
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Value = v
    },
})
```

## With Placeholder

```go
gui.Combobox(gui.ComboboxCfg{
    ID:          "search",
    Placeholder: "Search languages...",
    Options:     languages,
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Lang = v
    },
})
```

## Virtualization

The dropdown always scrolls, and virtualizes its rows once
`MaxDropdownHeight > 0` caps the list. Scroll state is keyed by the widget's
ID + `".dropdown"`.

```go
gui.Combobox(gui.ComboboxCfg{
    ID:                "large-cb",
    MaxDropdownHeight: 200,
    Options:           largeList,
})
```

## Key Properties

| Property          | Type     | Description                  |
| ----------------- | -------- | ---------------------------- |
| Value             | string   | Current selection            |
| Placeholder       | string   | Hint text shown when empty   |
| Options           | []string | Searchable options           |
| MaxDropdownHeight | float32  | Max dropdown pixel height    |
| MinWidth          | float32  | Minimum width                |
| MaxWidth          | float32  | Maximum width                |
| FloatZIndex       | int      | Z-order for dropdown overlay |
| Sizing            | Sizing   | Combined axis sizing mode    |
| Disabled          | bool     | Disable interaction          |

## Appearance

| Property         | Type         | Description               |
| ---------------- | ------------ | ------------------------- |
| Padding          | Opt[Padding] | Inner padding             |
| Radius           | Opt[float32] | Corner radius             |
| SizeBorder       | Opt[float32] | Border width              |
| Color            | Color        | Background color          |
| ColorBorder      | Color        | Border color              |
| ColorBorderFocus | Color        | Border color when focused |
| ColorFocus       | Color        | Background when focused   |
| ColorHighlight   | Color        | Highlighted option color  |
| ColorHover       | Color        | Option hover color        |
| TextStyle        | TextStyle    | Option text styling       |
| PlaceholderStyle | TextStyle    | Placeholder text styling  |

## Events

| Callback | Signature              | Fired when      |
| -------- | ---------------------- | --------------- |
| OnSelect | func(string, EventCtx) | Option selected |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
