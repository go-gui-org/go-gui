Horizontal container — children flow left to right. Uses
ContainerCfg for all sizing, alignment, scrolling, floating, borders,
and event handling.

## Usage

```go
gui.Row(gui.ContainerCfg{
    Spacing: gui.SomeF(8),
    Padding: gui.SomeP(4, 8, 4, 8),
    Sizing:  gui.FillFit,
    Content: []gui.View{child1, child2},
})
```

## Scrollable Row

```go
gui.Row(gui.ContainerCfg{
    ID:         "my-row",
    Scrollable: true,
    ScrollMode: gui.ScrollModeX,
    Sizing:     gui.FillFixed,
    Height:     200,
    Content:    items,
})
```

## Group Box (Titled Border)

```go
gui.Row(gui.ContainerCfg{
    Title:       "Options",
    TitleBG:     theme.ColorPanel,
    ColorBorder: theme.ColorBorder,
    Content:     items,
})
```

## Key Properties

| Property   | Type            | Description                          |
|------------|-----------------|--------------------------------------|
| Content    | []View          | Child views                          |
| Sizing     | Sizing          | Combined axis sizing mode            |
| Width      | float32         | Fixed width                          |
| Height     | float32         | Fixed height                         |
| MinWidth   | float32         | Minimum width                        |
| MaxWidth   | float32         | Maximum width                        |
| MinHeight  | float32         | Minimum height                       |
| MaxHeight  | float32         | Maximum height                       |
| Spacing    | Opt[float32]    | Gap between children                 |
| Padding    | Opt[Padding]    | Inner padding                        |
| HAlign     | HorizontalAlign | Horizontal content alignment         |
| VAlign     | VerticalAlign   | Vertical content alignment           |
| TextDir    | TextDirection   | Text/layout direction (LTR/RTL)      |
| IDFocus    | uint32          | Tab-order focus ID (> 0 to enable)   |
| Scrollable  | bool            | Opt into the scroll system (state keyed by ID)|
| ScrollMode | ScrollMode      | Scroll axis mode                     |
| Clip       | bool            | Clip children to bounds              |
| Disabled   | bool            | Disable interaction                  |
| Invisible  | bool            | Hide without removing from layout    |
| OverDraw   | bool            | Draw over siblings                   |
| Hero       | bool            | Hero animation participant           |

## Appearance

| Property       | Type           | Description                      |
|----------------|----------------|----------------------------------|
| Color          | Color          | Background color                 |
| ColorBorder    | Color          | Border color                     |
| SizeBorder     | Opt[float32]   | Border width                     |
| Radius         | Opt[float32]   | Corner radius                    |
| Opacity        | float32        | Opacity (0..1)                   |
| BlurRadius     | float32        | Background blur radius           |
| Shadow         | *BoxShadow     | Drop shadow                      |
| Gradient       | *GradientDef   | Background gradient              |
| BorderGradient | *GradientDef   | Border gradient                  |
| Shader         | *Shader        | Custom shader                    |
| Title          | string         | Group-box label in top border    |
| TitleBG        | Color          | Background behind title text     |

## Floating

| Property      | Type        | Description                        |
|---------------|-------------|------------------------------------|
| Float         | bool        | Float above siblings               |
| FloatAutoFlip | bool        | Auto-flip when clipped             |
| FloatAnchor   | FloatAttach | Anchor attachment point            |
| FloatTieOff   | FloatAttach | Tie-off attachment point           |
| FloatOffsetX  | float32     | Horizontal float offset            |
| FloatOffsetY  | float32     | Vertical float offset              |
| FloatZIndex   | int         | Z-order for floated elements       |

## Events

| Callback    | Signature                          | Fired when               |
|-------------|------------------------------------|--------------------------|
| OnClick     | func(*Layout, *Event, *Window)     | Left-click               |
| OnAnyClick  | func(*Layout, *Event, *Window)     | Any mouse button click   |
| OnChar      | func(*Layout, *Event, *Window)     | Character input          |
| OnKeyDown   | func(*Layout, *Event, *Window)     | Key pressed              |
| OnMouseMove | func(*Layout, *Event, *Window)     | Mouse movement           |
| OnMouseUp   | func(*Layout, *Event, *Window)     | Mouse button released    |
| OnHover     | func(*Layout, *Event, *Window)     | Mouse hover              |
| OnScroll    | func(*Layout, *Window)             | Scroll position changed  |
| OnIMECommit | func(*Layout, string, *Window)     | IME text committed       |
| AmendLayout | func(*Layout, *Window)             | Post-layout amendment    |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YState       | AccessState | Accessible state override        |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
