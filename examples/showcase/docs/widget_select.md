Dropdown selector with single or multi-select. Each option has a `Label` (the
text shown) and a `Value` (what `Selected` and `OnSelect` carry), so app state
can hold a stable key and not the display text.

For a plain string list, use `Items`. Each string is both label and value. An
`Items` string with the prefix "---" renders as a subheading.

## Usage

```go
gui.Select(gui.SelectCfg{
    ID:       "lang",
    Selected: app.Selected,
    Items:    []string{"Go", "Rust", "Zig"},
    OnSelect: func(sel []string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Selected = sel
    },
})
```

## Multi-Select

```go
gui.Select(gui.SelectCfg{
    ID:             "tags",
    Placeholder:    "Choose tags...",
    SelectMultiple: true,
    Items:          []string{"alpha", "beta", "stable"},
    OnSelect: func(sel []string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Tags = sel
    },
})
```

## Label/Value Options

```go
gui.Select(gui.SelectCfg{
    ID:       "runtime",
    Selected: app.Runtime, // for example []string{"typescript_node"}
    Options: []gui.SelectOption{
        gui.NewSelectOption("Go", "go"),
        gui.NewSelectSubheading("TypeScript"),
        gui.NewSelectOption("TypeScript (Node.js)", "typescript_node"),
        gui.NewSelectOption("TypeScript (Bun)", "typescript_bun"),
    },
    OnSelect: func(sel []string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Runtime = sel // values
    },
})
```

## Key Properties

| Property       | Type           | Description                         |
| -------------- | -------------- | ----------------------------------- |
| Selected       | []string       | Values of the selected option(s)    |
| Items          | []string       | Label==Value list ("---" = subhead) |
| Options        | []SelectOption | Label/value choices; Items wins     |
| Placeholder    | string         | Hint text when empty                |
| SelectMultiple | bool           | Allow multi-select                  |
| NoWrap         | bool           | Clip text in multi-select mode      |
| MinWidth       | float32        | Minimum width                       |
| MaxWidth       | float32        | Maximum width                       |
| FloatZIndex    | int            | Z-order for dropdown overlay        |
| Sizing         | Sizing         | Combined axis sizing mode           |
| Disabled       | bool           | Disable interaction                 |
| Invisible      | bool           | Hide without removing from layout   |

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
| ColorSelect      | Color        | Selected item highlight   |
| TextStyle        | TextStyle    | Option text styling       |
| SubheadingStyle  | TextStyle    | Subheading text styling   |
| PlaceholderStyle | TextStyle    | Placeholder text styling  |

## Events

| Callback | Signature                | Fired when                     |
| -------- | ------------------------ | ------------------------------ |
| OnSelect | func([]string, EventCtx) | Selection changes; gets values |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
