Horizontal flow that wraps to the next line when full.
Items fill left to right, then break to the next row when the
container width is exceeded. Uses ContainerCfg with Wrap set
automatically by the factory function.

## Usage

```go
gui.Wrap(gui.ContainerCfg{
    Spacing: gui.SomeF(4),
    Sizing:  gui.FillFit,
    Content: items,
})
```

## Wrap with Alignment

```go
gui.Wrap(gui.ContainerCfg{
    Spacing: gui.SomeF(8),
    HAlign:  gui.HAlignCenter,
    VAlign:  gui.VAlignMiddle,
    Content: tags,
})
```

## Key Properties

| Property   | Type            | Description                          |
|------------|-----------------|--------------------------------------|
| Content    | []View          | Child views to wrap                  |
| Sizing     | Sizing          | Combined axis sizing mode            |
| Width      | float32         | Fixed width                          |
| Height     | float32         | Fixed height                         |
| MinWidth   | float32         | Minimum width                        |
| MaxWidth   | float32         | Maximum width                        |
| MinHeight  | float32         | Minimum height                       |
| MaxHeight  | float32         | Maximum height                       |
| Spacing    | Opt[float32]    | Gap between items (horizontal & row) |
| Padding    | Opt[Padding]    | Inner padding                        |
| HAlign     | HorizontalAlign | Horizontal alignment per row         |
| VAlign     | VerticalAlign   | Cross-axis alignment per row         |
| TextDir    | TextDirection   | Text/layout direction (LTR/RTL)      |
| Clip       | bool            | Clip children to bounds              |
| Disabled   | bool            | Disable interaction                  |
| Invisible  | bool            | Hide without removing from layout    |

## Appearance

| Property       | Type           | Description                      |
|----------------|----------------|----------------------------------|
| Color          | Color          | Background color                 |
| ColorBorder    | Color          | Border color                     |
| SizeBorder     | Opt[float32]   | Border width                     |
| Radius         | Opt[float32]   | Corner radius                    |
| Opacity        | float32        | Opacity (0..1)                   |
| Shadow         | *BoxShadow     | Drop shadow                      |
| Gradient       | *GradientDef   | Background gradient              |

## Events

| Callback    | Signature                          | Fired when               |
|-------------|------------------------------------|--------------------------|
| OnClick     | func(*Layout, *Event, *Window)     | Left-click               |
| OnHover     | func(*Layout, *Event, *Window)     | Mouse hover              |
| OnKeyDown   | func(*Layout, *Event, *Window)     | Key pressed              |
| OnMouseMove | func(*Layout, *Event, *Window)     | Mouse movement           |
| AmendLayout | func(*Layout, *Window)             | Post-layout amendment    |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
