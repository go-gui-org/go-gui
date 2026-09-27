Editable dropdown with type-ahead filtering. Typing narrows the options list.
Selecting an option commits the value.

Each option has a `Label` (shown, and matched by the typed query) and a `Value`
(what `Value` and `OnSelect` carry). For a plain string list, use `Items`: each
string is both label and value.

## Usage

```go
gui.Combobox(gui.ComboboxCfg{
    ID:    "cb",
    Value: app.Value,
    Items: []string{"Go", "Rust", "Zig"},
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Value = v
    },
})
```

## Label/Value Options

Typing "node" matches the label "TypeScript (Node.js)". Selecting it sets
`Value` to "typescript_node", and the closed field shows the label.

```go
gui.Combobox(gui.ComboboxCfg{
    ID:          "runtime",
    Placeholder: "Search runtimes...",
    Value:       app.Runtime, // for example "typescript_node"
    Options: []gui.SelectOption{
        gui.NewSelectOption("Go", "go"),
        gui.NewSelectOption("TypeScript (Node.js)", "typescript_node"),
        gui.NewSelectOption("TypeScript (Bun)", "typescript_bun"),
    },
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Runtime = v // the value
    },
})
```

A Combobox has no subheadings. It leaves out any `NewSelectSubheading` option.

## Virtualization

The dropdown always scrolls, and virtualizes its rows once
`MaxDropdownHeight > 0` caps the list. Scroll state is keyed by the widget's
ID + `".dropdown"`.

```go
gui.Combobox(gui.ComboboxCfg{
    ID:                "large-cb",
    MaxDropdownHeight: 200,
    Items:             largeList,
})
```

## Key Properties

| Property          | Type           | Description                     |
| ----------------- | -------------- | ------------------------------- |
| Value             | string         | Value of the current option     |
| Placeholder       | string         | Hint text shown when empty      |
| Items             | []string       | Label==Value options            |
| Options           | []SelectOption | Label/value options; Items wins |
| MaxDropdownHeight | float32        | Max dropdown pixel height       |
| MinWidth          | float32        | Minimum width                   |
| MaxWidth          | float32        | Maximum width                   |
| FloatZIndex       | int            | Z-order for dropdown overlay    |
| Sizing            | Sizing         | Combined axis sizing mode       |
| Disabled          | bool           | Disable interaction             |

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

| Callback | Signature              | Fired when                      |
| -------- | ---------------------- | ------------------------------- |
| OnSelect | func(string, EventCtx) | Option selected; gets its value |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
