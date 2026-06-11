Animated slide-out panel. Width animates between 0 and the
configured width using either a tween or spring animation. Called as a
method on Window.

## Usage

```go
w.Sidebar(gui.SidebarCfg{
    ID:      "sb",
    Open:    app.SidebarOpen,
    Width:   250,
    Content: []gui.View{navItems},
})
```

## Spring Animation

```go
w.Sidebar(gui.SidebarCfg{
    ID:            "sb",
    Open:          app.Open,
    TweenDuration: 0,
    Spring:        gui.SpringStiff,
    Content:       content,
})
```

## Key Properties

| Property      | Type          | Description                          |
|---------------|---------------|--------------------------------------|
| ID            | string        | Unique identifier                    |
| Open          | bool          | Sidebar visibility                   |
| Width         | float32       | Panel width (default 250)            |
| Content       | []View        | Sidebar content                      |
| Sizing        | Sizing        | Combined axis sizing (default FixedFill) |
| Clip          | bool          | Clip content to bounds               |
| Disabled      | bool          | Disable interaction                  |
| Invisible     | bool          | Hide without removing from layout    |

## Appearance

| Property | Type         | Description                          |
|----------|--------------|--------------------------------------|
| Color    | Color        | Background color                     |
| Shadow   | *BoxShadow   | Drop shadow                          |
| Radius   | float32      | Corner radius                        |
| Padding  | Opt[Padding] | Inner padding                        |

## Animation

| Property      | Type          | Description                          |
|---------------|---------------|--------------------------------------|
| Spring        | SpringCfg     | Spring animation config              |
| TweenDuration | time.Duration | Tween duration (default 300ms)       |
| TweenEasing   | EasingFn      | Easing function (default InOutCubic) |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
