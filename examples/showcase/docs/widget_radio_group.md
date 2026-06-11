Grouped radio buttons in row or column layout with
optional group-box border and title.

## Usage

```go
gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
    Value:   app.Lang,
    IDFocus: 510,
    Title:   "Language",
    Options: []gui.RadioOption{
        gui.NewRadioOption("Go", "go"),
        gui.NewRadioOption("Rust", "rust"),
    },
    OnSelect: func(v string, w *gui.Window) {
        gui.State[App](w).Lang = v
    },
})
```

## Horizontal Layout

```go
gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
    Value:   app.Size,
    Options: []gui.RadioOption{
        gui.NewRadioOption("S", "s"),
        gui.NewRadioOption("M", "m"),
        gui.NewRadioOption("L", "l"),
    },
    OnSelect: func(v string, w *gui.Window) {
        gui.State[App](w).Size = v
    },
})
```

## Stdlib Data Binding

Use `Items []string` when label equals value:

```go
gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
    Value:   "go",
    Items:   []string{"go", "rust", "zig"},
    OnSelect: func(v string, w *gui.Window) { ... },
})
```

When `Items` is set, `Options` is ignored.

## Key Properties

| Property  | Type            | Description                          |
|-----------|-----------------|--------------------------------------|
| Value     | string          | Currently selected value             |
| Items     | []string        | Simple string list (alt. to Options) |
| Options   | []RadioOption   | Available choices (Label + Value)    |
| Title     | string          | Group-box label                      |
| TitleBG   | Color           | Border-eraser background for title   |
| IDFocus   | uint32          | Tab-order focus ID for first radio   |
| MinWidth  | float32         | Minimum width                        |
| MinHeight | float32         | Minimum height                       |
| Sizing    | Sizing          | Combined axis sizing mode            |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Padding     | Opt[Padding] | Inner padding                        |
| Spacing     | Opt[float32] | Gap between radio buttons            |
| SizeBorder  | Opt[float32] | Group border width                   |
| ColorBorder | Color        | Group border color                   |

## Factories

| Function                      | Layout               |
|-------------------------------|----------------------|
| RadioButtonGroupColumn(cfg)   | Vertical (stacked)   |
| RadioButtonGroupRow(cfg)      | Horizontal (inline)  |

## Events

| Callback | Signature                | Fired when          |
|----------|--------------------------|---------------------|
| OnSelect | func(string, *Window)    | Selection changes   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
