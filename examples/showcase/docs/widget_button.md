Clickable button with hover, focus, and click color states.
Content is any set of child views.

## Usage

```go
gui.Button(gui.ButtonCfg{
    ID:      "submit",
    IDFocus: 100,
    Content: []gui.View{gui.Text(gui.TextCfg{Text: "Submit"})},
    OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
        e.IsHandled = true
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

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Content   | []View       | Child views inside the button        |
| IDFocus   | uint32       | Tab-order focus ID (> 0 to enable)   |
| Float     | bool         | Float above siblings                 |
| HAlign    | HorizontalAlign | Horizontal content alignment      |
| VAlign    | VerticalAlign   | Vertical content alignment        |
| Disabled  | bool         | Disable interaction                  |
| Invisible | bool         | Hide without removing from layout    |
| Sizing    | Sizing       | Combined axis sizing mode            |
| Width     | float32      | Fixed width                          |
| Height    | float32      | Fixed height                         |
| MinWidth  | float32      | Minimum width                        |
| MaxWidth  | float32      | Maximum width                        |
| MinHeight | float32      | Minimum height                       |
| MaxHeight | float32      | Maximum height                       |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorHover       | Color        | Background on hover              |
| ColorFocus       | Color        | Background when focused          |
| ColorClick       | Color        | Background on click              |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| BlurRadius       | float32      | Background blur radius           |
| Shadow           | *BoxShadow   | Drop shadow                      |
| Gradient         | *GradientDef | Background gradient              |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Button clicked   |
| OnHover  | func(*Layout, *Event, *Window)   | Mouse hover      |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YState       | AccessState | Accessible state override        |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
