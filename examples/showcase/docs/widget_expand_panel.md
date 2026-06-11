Collapsible section with a clickable header and expandable
body. Header shows an arrow indicator that reflects the open state.

## Usage

```go
gui.ExpandPanel(gui.ExpandPanelCfg{
    ID:   "ep",
    Open: app.Open,
    Head: gui.Text(gui.TextCfg{Text: "Details"}),
    Content: gui.Column(gui.ContainerCfg{
        Content: []gui.View{child1, child2},
    }),
    OnToggle: func(w *gui.Window) {
        gui.State[App](w).Open = !gui.State[App](w).Open
    },
})
```

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| ID        | string       | Unique identifier                    |
| Head      | View         | Header view (always visible)         |
| Content   | View         | Collapsible body content             |
| Open      | bool         | Expanded state                       |
| Sizing    | Sizing       | Combined axis sizing mode            |
| MinWidth  | float32      | Minimum width                        |
| MaxWidth  | float32      | Maximum width                        |
| MinHeight | float32      | Minimum height                       |
| MaxHeight | float32      | Maximum height                       |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Color       | Color        | Background color                     |
| ColorHover  | Color        | Header background on hover           |
| ColorClick  | Color        | Header background on click           |
| ColorBorder | Color        | Border color                         |
| Padding     | Opt[Padding] | Inner padding                        |
| SizeBorder  | Opt[float32] | Border width                         |
| Radius      | Opt[float32] | Corner radius                        |

## Events

| Callback | Signature        | Fired when                           |
|----------|------------------|--------------------------------------|
| OnToggle | func(*Window)    | Header clicked or Space pressed      |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
