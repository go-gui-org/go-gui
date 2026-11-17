Clickable button with hover, focus, and click color states. Content is any set
of child views.

## Usage

```go
gui.Button(gui.ButtonCfg{
    ID:      "submit",
    Content: []gui.View{gui.Text(gui.TextCfg{Text: "Submit"})},
    OnClick: func(ctx gui.EventCtx) {
        ctx.Consume()
    },
})
```

## Disabled Button

```go
gui.Button(gui.ButtonCfg{
    ID:       "noop",
    Disabled: true,
    Content:  []gui.View{gui.Text(gui.TextCfg{Text: "Disabled"})},
})
```

## Loading Button

Set `Loading` while the work that the button started runs. The button shows a
spinner in place of its content and keeps its width. A click does nothing. The
button keeps its colors and its focus, and reports the busy state to assistive
tech.

```go
gui.Button(gui.ButtonCfg{
    ID:      "save",
    Label:   "Save",
    Loading: app.Saving,
    OnClick: startSave,
})
```

## Key Properties

| Property  | Type            | Description                       |
| --------- | --------------- | --------------------------------- |
| Content   | []View          | Child views inside the button     |
| Float     | bool            | Float above siblings              |
| HAlign    | HorizontalAlign | Horizontal content alignment      |
| VAlign    | VerticalAlign   | Vertical content alignment        |
| Disabled  | bool            | Disable interaction               |
| Loading   | bool            | Spinner in place of content, busy |
| Invisible | bool            | Hide without removing from layout |
| Sizing    | Sizing          | Combined axis sizing mode         |
| Width     | float32         | Fixed width                       |
| Height    | float32         | Fixed height                      |
| MinWidth  | float32         | Minimum width                     |
| MaxWidth  | float32         | Maximum width                     |
| MinHeight | float32         | Minimum height                    |
| MaxHeight | float32         | Maximum height                    |

## Appearance

| Property   | Type         | Description                                                      |
| ---------- | ------------ | ---------------------------------------------------------------- |
| Padding    | Padding      | Inner padding                                                    |
| Radius     | Radius       | Corner radius                                                    |
| SizeBorder | Border       | Border width                                                     |
| Color      | Color        | Resting background (shorthand for `Colors.Base`)                 |
| Colors     | ColorSet     | Per-state colors: Base, Hover, Click, Focus, Border, BorderFocus |
| BlurRadius | float32      | Background blur radius                                           |
| Shadow     | *BoxShadow   | Drop shadow                                                      |
| Gradient   | *GradientDef | Background gradient                                              |

## Events

| Callback | Signature      | Fired when     |
| -------- | -------------- | -------------- |
| OnClick  | func(EventCtx) | Button clicked |
| OnHover  | func(EventCtx) | Mouse hover    |

## Accessibility

| Property  | Type        | Description                          |
| --------- | ----------- | ------------------------------------ |
| A11YRole  | AccessRole  | Accessible role override             |
| A11YState | AccessState | Accessible state override            |
| A11YCfg   | A11YCfg     | Embedded: A11YLabel, A11YDescription |

Set the pair through the embed: `A11YCfg: gui.A11YCfg{A11YLabel: "Save"}`.
