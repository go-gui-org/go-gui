Grouped radio buttons in row or column layout with optional group-box border and
title.

## Usage

```go
gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
    Value:   app.Lang,
    Title:   "Language",
    Options: []gui.RadioOption{
        gui.NewRadioOption("Go", "go"),
        gui.NewRadioOption("Rust", "rust"),
    },
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Lang = v
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
    OnSelect: func(v string, ctx gui.EventCtx) {
        gui.State[App](ctx.Window).Size = v
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

| Property  | Type          | Description                          |
| --------- | ------------- | ------------------------------------ |
| Value     | string        | Currently selected value             |
| Items     | []string      | Simple string list (alt. to Options) |
| Options   | []RadioOption | Available choices (Label + Value)    |
| Title     | string        | Group-box label                      |
| TitleBG   | Color         | Border-eraser background for title   |
| MinWidth  | float32       | Minimum width                        |
| MinHeight | float32       | Minimum height                       |
| Sizing    | Sizing        | Combined axis sizing mode            |

## Appearance

| Property    | Type         | Description               |
| ----------- | ------------ | ------------------------- |
| Padding     | Opt[Padding] | Inner padding             |
| Spacing     | Opt[float32] | Gap between radio buttons |
| SizeBorder  | Opt[float32] | Group border width        |
| ColorBorder | Color        | Group border color        |

## Factories

| Function                    | Layout              |
| --------------------------- | ------------------- |
| RadioButtonGroupColumn(cfg) | Vertical (stacked)  |
| RadioButtonGroupRow(cfg)    | Horizontal (inline) |

## Events

| Callback | Signature              | Fired when        |
| -------- | ---------------------- | ----------------- |
| OnSelect | func(string, EventCtx) | Selection changes |

## Accessibility

| Property | Type    | Description                          |
| -------- | ------- | ------------------------------------ |
| A11YCfg  | A11YCfg | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
