Standalone visual shape — sharp, rounded, bordered, or pill.
Technically a container with no children, axis, or padding.
Supports gradients, shadows, shaders, and background blur.

## Usage

```go
gui.Rectangle(gui.RectangleCfg{
    Width:  100,
    Height: 60,
    Sizing: gui.FixedFixed,
    Color:  gui.ColorFromString("#3b82f6"),
    Radius: 8,
})
```

## Bordered

```go
gui.Rectangle(gui.RectangleCfg{
    Width:       80,
    Height:      80,
    Sizing:      gui.FixedFixed,
    ColorBorder: gui.White,
    SizeBorder:  2,
    Radius:      40,
})
```

## Key Properties

| Property       | Type         | Description                          |
|----------------|--------------|--------------------------------------|
| Color          | Color        | Fill color                           |
| ColorBorder    | Color        | Border color                         |
| SizeBorder     | float32      | Border thickness                     |
| Radius         | float32      | Corner radius                        |
| BlurRadius     | float32      | Background blur                      |
| Width          | float32      | Width                                |
| Height         | float32      | Height                               |
| MinWidth       | float32      | Minimum width                        |
| MinHeight      | float32      | Minimum height                       |
| MaxHeight      | float32      | Maximum height                       |
| Sizing         | Sizing       | Combined axis sizing mode            |
| Disabled       | bool         | Disable interaction                  |
| Invisible      | bool         | Hide without removing from layout    |

## Appearance

| Property       | Type         | Description                          |
|----------------|--------------|--------------------------------------|
| Gradient       | *GradientDef | Gradient fill                        |
| BorderGradient | *GradientDef | Gradient border                      |
| Shadow         | *BoxShadow   | Drop shadow                          |
| Shader         | *Shader      | Custom fragment shader               |
